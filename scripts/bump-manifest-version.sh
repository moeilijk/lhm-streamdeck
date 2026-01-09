#!/usr/bin/env bash
set -euo pipefail

manifest="com.moeilijk.lhm.sdPlugin/manifest.json"

python3 - <<'PY'
import pathlib
import re
import sys

path = pathlib.Path("com.moeilijk.lhm.sdPlugin/manifest.json")
text = path.read_text()
pattern = r'"Version"\s*:\s*"(\d+\.\d+\.\d+\.)(\d+)"'
match = re.search(pattern, text)
if not match:
    sys.exit("Version not found or invalid format in manifest.json")

prefix, build = match.group(1), int(match.group(2))
new_version = f"{prefix}{build + 1}"
new_text = re.sub(pattern, f'"Version": "{new_version}"', text, count=1)
path.write_text(new_text)
print(new_version)
PY
