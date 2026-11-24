#!/usr/bin/env bash
# Rebuilds the binaries of a published release from its tag and compares them
# with release-hashes.txt (issue #121, taken over from hwinfo-streamdeck #126).
# The binaries do not carry their commit, so this is what ties a shipped hash
# to the source it was built from. It also closes the gap that
# record-release-hashes.sh leaves: that script records what a download holds,
# this one proves the recorded hash follows from the tag.
#
# Only the Windows binaries are rebuilt. They are pure Go, so the rebuild
# needs no pinned environment: the toolchain comes from the tag's go.mod and
# the flags from the tag's Makefile.
#
# Usage: scripts/verify-release.sh [<tag>]     default: the latest v* tag
#
# Exit status: 0 the rebuild matches the record, 1 it does not (or the check
# could not run), 2 the release predates the reproducible build flags and
# cannot be rebuilt this way.
#
# Not covered: the Linux binaries. They link against the C library of the
# machine that builds them, so a rebuild elsewhere gives other bytes, and
# lhm-companion comes from its own repository.
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hashes_file="${HASHES_FILE:-$root_dir/release-hashes.txt}"
# "<name in release-hashes.txt>:<GOOS>:<package to build>"
binaries=(
  "lhm.exe:windows:./cmd/lhm_streamdeck_plugin"
  "lhm-bridge.exe:windows:./cmd/lhm-bridge"
)

tag="${1:-$(git -C "$root_dir" describe --tags --abbrev=0 --match 'v*')}"

recorded() { awk -v key="$tag/$1" '$2 == key { print $1 }' "$hashes_file" | head -n 1; }
built="$(awk -v key="$tag/lhm.exe" '$2 == key { sub(/^built:/, "", $3); print $3 }' "$hashes_file" | head -n 1)"
if [[ -z "$(recorded lhm.exe)" || -z "$built" ]]; then
  echo "verify-release: $tag is not in ${hashes_file#"$root_dir"/}" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
git clone -q "$root_dir" "$work/src"
if ! git -C "$work/src" checkout -q "refs/tags/$built" 2>/dev/null; then
  echo "verify-release: tag $built not found" >&2
  exit 1
fi

echo "release: $tag, binaries built for $built"
flags="$(make -s -C "$work/src" --eval='print-flags: ; @echo $(GOBUILDFLAGS)' print-flags 2>/dev/null || true)"
if [[ "$flags" != *-trimpath* || "$flags" != *-buildvcs=false* ]]; then
  echo "result: $built predates the reproducible build flags and cannot be rebuilt for comparison"
  exit 2
fi
echo "  build flags of $built: $flags"

status=0
for entry in "${binaries[@]}"; do
  IFS=: read -r name goos pkg <<< "$entry"
  want="$(recorded "$name")"
  if [[ -z "$want" ]]; then
    echo "  $name: not in the record for $tag, skipped"
    continue
  fi
  mkdir -p "$(dirname "$work/out/$name")"
  # shellcheck disable=SC2086
  (cd "$work/src" && GOOS="$goos" GOARCH=amd64 go build $flags -o "$work/out/$name" "$pkg")
  got="$(sha256sum "$work/out/$name" | cut -d' ' -f1)"
  if [[ "$got" == "$want" ]]; then
    echo "  $name: the rebuild matches the recorded hash"
  else
    echo "  $name: the rebuild DIFFERS from the recorded hash" >&2
    echo "    recorded: $want" >&2
    echo "    rebuilt:  $got" >&2
    status=1
  fi
done
if [[ "$status" == "0" ]]; then
  echo "result: $tag is what its source builds"
else
  echo "result: $tag does not match a rebuild of $built"
fi
exit $status
