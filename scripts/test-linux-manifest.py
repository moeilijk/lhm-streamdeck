#!/usr/bin/env python3
"""Tests for make-linux-manifest.py (Linux artifact manifest transform).

Fails (before the fix) when the packed Linux manifest would make OpenDeck
run lhm.exe under Wine: no OS Platform "linux" entry and/or the native
binary declared under a key OpenDeck does not read ("CodePathLinux").
"""
import importlib.util
import json
import os
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True  # keep scripts/__pycache__ out of the tree

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SCRIPT = os.path.join(HERE, "make-linux-manifest.py")
SOURCE_MANIFEST = os.path.join(REPO, "com.moeilijk.lhm.sdPlugin", "manifest.json")

spec = importlib.util.spec_from_file_location("make_linux_manifest", SCRIPT)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

failures = []


def check(name, cond, detail=""):
    if cond:
        print(f"ok   {name}")
    else:
        failures.append(name)
        print(f"FAIL {name} {detail}")


# 1. Transform of the real source manifest, via the CLI (as the Makefile runs it).
with open(SOURCE_MANIFEST) as f:
    original = json.load(f)
with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as tmp:
    json.dump(original, tmp)
    tmp_path = tmp.name
try:
    subprocess.run([sys.executable, SCRIPT, tmp_path], check=True)
    with open(tmp_path) as f:
        m = json.load(f)

    check("CodePathLin is 'lhm' (key OpenDeck reads)", m.get("CodePathLin") == "lhm", repr(m.get("CodePathLin")))
    check("no CodePathLinux key (OpenDeck ignores it)", "CodePathLinux" not in m)
    check("OS contains a linux platform entry", any(e.get("Platform") == "linux" for e in m.get("OS", [])), repr(m.get("OS")))
    check("OS keeps the windows platform entry", any(e.get("Platform") == "windows" for e in m.get("OS", [])), repr(m.get("OS")))
    check("CodePath (windows) untouched", m.get("CodePath") == original.get("CodePath"), repr(m.get("CodePath")))
    check("Version untouched", m.get("Version") == original.get("Version"))

    # 2. Idempotent: running the transform again must not duplicate entries.
    m2 = mod.transform(json.loads(json.dumps(m)))
    check("idempotent: single linux OS entry after re-run", sum(1 for e in m2.get("OS", []) if e.get("Platform") == "linux") == 1, repr(m2.get("OS")))
finally:
    os.unlink(tmp_path)

# 3. Legacy shape: a manifest with the old CodePathLinux key gets migrated.
legacy = {"CodePath": "lhm.exe", "CodePathLinux": "lhm", "OS": [{"Platform": "windows", "MinimumVersion": "10"}]}
migrated = mod.transform(legacy)
check("legacy CodePathLinux removed", "CodePathLinux" not in migrated)
check("legacy manifest gains CodePathLin", migrated.get("CodePathLin") == "lhm")
check("legacy manifest gains linux OS entry", any(e.get("Platform") == "linux" for e in migrated.get("OS", [])))

# 4. Release gate (verify-linux-package.py) on synthetic packed artifacts.
import zipfile

verifier_spec = importlib.util.spec_from_file_location("verify_linux_package", os.path.join(HERE, "verify-linux-package.py"))
verifier = importlib.util.module_from_spec(verifier_spec)
verifier_spec.loader.exec_module(verifier)


def make_package(path, manifest, with_elf=True, elf_magic=b"\x7fELF"):
    with zipfile.ZipFile(path, "w") as z:
        z.writestr("com.moeilijk.lhm.sdPlugin/manifest.json", json.dumps(manifest))
        if with_elf:
            z.writestr("com.moeilijk.lhm.sdPlugin/lhm", elf_magic + b"\x00" * 16)


good_manifest = mod.transform({"CodePath": "lhm.exe", "Version": "9.9.9.0", "OS": [{"Platform": "windows", "MinimumVersion": "10"}]})
with tempfile.TemporaryDirectory() as d:
    # Artifact filename must carry MAJOR.MINOR.PATCH matching the manifest (issue #78).
    pkg = os.path.join(d, "com.moeilijk.lhm-linux-9.9.9.streamDeckPlugin")

    make_package(pkg, good_manifest)
    check("gate passes a correct, correctly-named package", verifier.verify(pkg) == [], repr(verifier.verify(pkg)))

    # The exact 2.0.0.0 regression: windows-only OS + legacy CodePathLinux key.
    make_package(pkg, {"CodePath": "lhm.exe", "Version": "9.9.9.0", "CodePathLinux": "lhm", "OS": [{"Platform": "windows", "MinimumVersion": "10"}]})
    errs = verifier.verify(pkg)
    check("gate rejects the 2.0.0.0 regression shape", len(errs) >= 3, repr(errs))

    make_package(pkg, {**good_manifest, "OS": [{"Platform": "windows", "MinimumVersion": "10"}]})
    check("gate rejects missing linux OS entry", any("linux" in e for e in verifier.verify(pkg)))

    make_package(pkg, good_manifest, with_elf=False)
    check("gate rejects missing native binary", any("missing" in e for e in verifier.verify(pkg)))

    make_package(pkg, good_manifest, elf_magic=b"MZ\x90\x00")
    check("gate rejects a non-ELF (PE) 'lhm' binary", any("ELF" in e for e in verifier.verify(pkg)))

    legacy_name = os.path.join(d, "com.moeilijk.lhm-linux.streamDeckPlugin")
    make_package(legacy_name, good_manifest)
    check("gate rejects the legacy unversioned filename", any("filename" in e for e in verifier.verify(legacy_name)))

    mismatch = os.path.join(d, "com.moeilijk.lhm-linux-1.0.0.streamDeckPlugin")
    make_package(mismatch, good_manifest)
    check("gate rejects filename/manifest version mismatch", any("!= manifest version" in e for e in verifier.verify(mismatch)))

if failures:
    print(f"\n{len(failures)} failure(s)")
    sys.exit(1)
print("\nall checks passed")
