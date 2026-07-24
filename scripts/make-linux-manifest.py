#!/usr/bin/env python3
"""Rewrite a copy of manifest.json for the Linux .streamDeckPlugin artifact.

OpenDeck (the Linux Stream Deck host) selects the native code path only when
both of these hold (src-tauri/src/plugins/mod.rs):
  1. the manifest's OS array contains an entry with Platform == "linux";
     otherwise a "windows" entry makes it run CodePath (lhm.exe) under Wine,
     which crash-loops on the go-plugin mTLS handshake (issue #45, #74);
  2. the native binary is declared under the key "CodePathLin"
     (serde alias in OpenDeck's manifest parser) — not "CodePathLinux".

Usage: make-linux-manifest.py <manifest.json>   (rewrites the file in place)
"""
import json
import sys


def transform(manifest: dict) -> dict:
    manifest["CodePathLin"] = "lhm"
    manifest.pop("CodePathLinux", None)
    os_list = manifest.setdefault("OS", [])
    if not any(entry.get("Platform") == "linux" for entry in os_list):
        os_list.append({"Platform": "linux", "MinimumVersion": "0"})
    return manifest


def main() -> int:
    if len(sys.argv) != 2:
        sys.stderr.write(__doc__)
        return 2
    path = sys.argv[1]
    with open(path) as f:
        manifest = json.load(f)
    with open(path, "w") as f:
        f.write(json.dumps(transform(manifest), indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
