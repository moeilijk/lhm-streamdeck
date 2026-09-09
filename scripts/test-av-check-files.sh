#!/usr/bin/env bash
# Test for scripts/av-check.sh --list-files (issue #93): the executables to scan
# are found by content (PE or ELF magic) directly inside the .sdPlugin
# directory, whatever their names, and nothing else is picked up.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT

d="$work/com.example.test.sdPlugin"
mkdir -p "$d/images"
printf 'MZ\x90\x00pe-stub' > "$d/tool.exe"
printf '\x7fELF\x02\x01\x01linux-stub' > "$d/tool"
printf 'MZ\x90\x00another-pe' > "$d/helper.dll"
printf 'MZ\x90\x00nested-not-scanned' > "$d/images/nested.exe"
printf '{"Version":"1.0.0.0"}' > "$d/manifest.json"
printf '\x89PNG\r\n\x1a\n' > "$d/icon.png"
printf 'plain text' > "$d/README"
(cd "$work" && zip -q -r pkg.streamDeckPlugin com.example.test.sdPlugin)

expected=$'pkg.streamDeckPlugin\nhelper.dll\ntool\ntool.exe'
actual=$(bash "$HERE/av-check.sh" --list-files "$work/pkg.streamDeckPlugin" | sed "s|^$work/||" | LC_ALL=C sort)
if [ "$actual" != "$(printf '%s\n' "$expected" | LC_ALL=C sort)" ]; then
  echo "test-av-check-files: unexpected file list:" >&2
  echo "$actual" >&2
  exit 1
fi

# A package without executables is an error, not a silent "clean".
mkdir -p "$work/empty/com.example.empty.sdPlugin"
printf '{}' > "$work/empty/com.example.empty.sdPlugin/manifest.json"
(cd "$work/empty" && zip -q -r ../empty.streamDeckPlugin com.example.empty.sdPlugin)
if bash "$HERE/av-check.sh" --list-files "$work/empty.streamDeckPlugin" >/dev/null 2>&1; then
  echo "test-av-check-files: package without executables was accepted" >&2
  exit 1
fi
echo "test-av-check-files: ok"
