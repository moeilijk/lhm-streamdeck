#!/usr/bin/env bash
# Records the SHA-256 of the binaries of a published release in
# release-hashes.txt (issue #121), so scripts/scan-release.sh can tell the
# published assets apart from replaced ones.
#
# Usage: scripts/record-release-hashes.sh <tag> [<built-tag>]
#   <tag>        the release whose packages are downloaded and hashed
#   <built-tag>  the release the binaries were built for; default <tag>
#
# Lines that are already recorded for the tag are left alone. A recorded hash
# that differs from the download stops the script: published binaries do not
# change, so that is a replaced asset or a wrong record.
#
# Environment:
#   RELEASE_REPO=o/r   GitHub repository (default moeilijk/lhm-streamdeck)
#   HASHES_FILE=path   hash list (default release-hashes.txt in the repo root)
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="${RELEASE_REPO:-moeilijk/lhm-streamdeck}"
hashes_file="${HASHES_FILE:-$root_dir/release-hashes.txt}"
plugin_dir="com.moeilijk.lhm.sdPlugin"

tag="${1:-}"
built="${2:-$tag}"
if [[ -z "$tag" ]]; then
  echo "usage: scripts/record-release-hashes.sh <tag> [<built-tag>]" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if ! gh release download "$tag" --repo "$repo" --pattern '*.streamDeckPlugin' --dir "$work" >/dev/null; then
  echo "record-release-hashes: could not download the $tag release assets from $repo" >&2
  exit 1
fi

added=0
record() {
  local package="$1" name="$2" file="$3" got have
  unzip -l "$package" "$plugin_dir/$file" >/dev/null 2>&1 || return 0
  got="$(unzip -p "$package" "$plugin_dir/$file" | sha256sum | cut -d' ' -f1)"
  have="$(awk -v key="$tag/$name" '$2 == key { print $1 }' "$hashes_file" 2>/dev/null | head -n 1)"
  if [[ -n "$have" && "$have" != "$got" ]]; then
    echo "record-release-hashes: $tag/$name is recorded with another hash" >&2
    echo "  recorded:   $have" >&2
    echo "  downloaded: $got" >&2
    exit 1
  fi
  if [[ -z "$have" ]]; then
    echo "$got  $tag/$name  built:$built" >> "$hashes_file"
    echo "recorded $tag/$name $got"
    added=$((added + 1))
  fi
}

windows_package="$(find "$work" -name '*.streamDeckPlugin' ! -name '*-linux*' | head -n 1)"
linux_package="$(find "$work" -name '*-linux*.streamDeckPlugin' | head -n 1)"
if [[ -z "$windows_package" ]]; then
  echo "record-release-hashes: release $tag has no Windows .streamDeckPlugin asset" >&2
  exit 1
fi
record "$windows_package" "lhm.exe" "lhm.exe"
record "$windows_package" "lhm-bridge.exe" "lhm-bridge.exe"
if [[ -n "$linux_package" ]]; then
  record "$linux_package" "linux/lhm" "lhm"
  record "$linux_package" "linux/lhm-bridge" "lhm-bridge"
  record "$linux_package" "linux/lhm-companion" "lhm-companion"
fi
echo "record-release-hashes: $added line(s) added for $tag"
