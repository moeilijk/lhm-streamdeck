#!/usr/bin/env python3
"""Release gate for the packed Linux .streamDeckPlugin artifact.

Verifies the OpenDeck contract on the artifact itself (not on our tooling),
so the Linux release can never again ship a package OpenDeck runs under Wine
(issues #45/#74). OpenDeck (src-tauri/src/plugins/mod.rs) requires:
  1. an OS entry with Platform == "linux" — otherwise the "windows" entry
     routes the plugin to Wine;
  2. the native binary declared under "CodePathLin" (not "CodePathLinux").
The native binary must also actually be an ELF executable in the package.

Usage: verify-linux-package.py <path/to/com.moeilijk.lhm-linux.streamDeckPlugin>
"""
import json
import sys
import zipfile

PLUGIN_DIR = "com.moeilijk.lhm.sdPlugin/"


def verify(path):
    errors = []
    with zipfile.ZipFile(path) as z:
        try:
            manifest = json.loads(z.read(PLUGIN_DIR + "manifest.json"))
        except KeyError:
            return [f"{PLUGIN_DIR}manifest.json missing from package"]

        if manifest.get("CodePathLin") != "lhm":
            errors.append(f"CodePathLin must be 'lhm', got {manifest.get('CodePathLin')!r}")
        if "CodePathLinux" in manifest:
            errors.append("legacy key CodePathLinux present (OpenDeck does not read it)")
        if not any(e.get("Platform") == "linux" for e in manifest.get("OS", [])):
            errors.append(f"OS array has no Platform 'linux' entry: {manifest.get('OS')!r}")

        try:
            if z.read(PLUGIN_DIR + "lhm")[:4] != b"\x7fELF":
                errors.append("packaged 'lhm' is not an ELF binary")
        except KeyError:
            errors.append("native binary 'lhm' missing from package")
    return errors


def main():
    if len(sys.argv) != 2:
        sys.stderr.write(__doc__)
        return 2
    errors = verify(sys.argv[1])
    for e in errors:
        print(f"FAIL linux-package: {e}")
    if errors:
        return 1
    print(f"ok   linux-package: {sys.argv[1]} satisfies the OpenDeck contract")
    return 0


if __name__ == "__main__":
    sys.exit(main())
