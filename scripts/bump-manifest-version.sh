#!/usr/bin/env bash
set -euo pipefail

manifest="com.moeilijk.lhm.sdPlugin/manifest.json"
state_file=".last_patch"

python3 - <<'PY'
import pathlib
import re
import sys

path = pathlib.Path("com.moeilijk.lhm.sdPlugin/manifest.json")
state_path = pathlib.Path(".last_patch")
text = path.read_text()
pattern = r'"Version"\s*:\s*"(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)\.(?P<build>\d+)"'
match = re.search(pattern, text)
if not match:
    sys.exit("Version not found or invalid format in manifest.json")

major = match.group("major")
minor = match.group("minor")
patch = match.group("patch")
build = int(match.group("build"))
patch_key = f"{major}.{minor}.{patch}"
last_patch = state_path.read_text().strip() if state_path.exists() else ""

if patch_key != last_patch:
    new_version = f"{patch_key}.0"
    state_path.write_text(patch_key)
    new_text = re.sub(pattern, f'"Version": "{new_version}"', text, count=1)
    path.write_text(new_text)
    print(new_version)
    sys.exit(0)

new_version = f"{patch_key}.{build + 1}"
new_text = re.sub(pattern, f'"Version": "{new_version}"', text, count=1)
path.write_text(new_text)
print(new_version)
PY
