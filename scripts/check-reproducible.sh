#!/usr/bin/env bash
# Reproducibility check for the release binaries (issue #121, taken over from
# hwinfo-streamdeck #126).
#
# Clones HEAD into two different directories, adds a commit that does not
# touch any Go input to one of them, builds the Windows and the Linux binaries
# in both with the Makefile's build flags, and fails when the bytes differ.
# That covers both things a release relies on: the binaries do not depend on
# the build path, and not on the commit they are built from.
#
# The flags come from the Makefile and never from the environment, so a stray
# variable cannot change them unnoticed. To measure what other flags do, give
# them on the make command line, where they are visible and reach this script
# through MAKEFLAGS: make reproducible GOBUILDFLAGS=
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
flags="$(make -s -C "$root_dir" --eval='print-flags: ; @echo $(GOBUILDFLAGS)' print-flags)"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/one" "$work/two/nested/deeper"
git clone -q "$root_dir" "$work/one/src"
git clone -q "$root_dir" "$work/two/nested/deeper/src"

# A commit that leaves every Go input alone must not change the binaries.
(
  cd "$work/two/nested/deeper/src"
  echo "reproducibility check" > .reproducible-check
  git add .reproducible-check
  git -c user.name=check -c user.email=check@localhost commit -q -m "unrelated commit"
)

build() {
  local src="$1" out="$2"
  (
    cd "$src"
    # shellcheck disable=SC2086
    GOOS=windows GOARCH=amd64 go build $flags -o "$out/lhm.exe" ./cmd/lhm_streamdeck_plugin
    # shellcheck disable=SC2086
    GOOS=windows GOARCH=amd64 go build $flags -o "$out/lhm-bridge.exe" ./cmd/lhm-bridge
    # shellcheck disable=SC2086
    GOOS=linux GOARCH=amd64 go build $flags -o "$out/lhm" ./cmd/lhm_streamdeck_plugin
    # shellcheck disable=SC2086
    GOOS=linux GOARCH=amd64 go build $flags -o "$out/lhm-bridge" ./cmd/lhm-bridge
  )
}

echo "build flags: ${flags:-<none>}"
build "$work/one/src" "$work/one"
build "$work/two/nested/deeper/src" "$work/two"

status=0
for bin in lhm.exe lhm-bridge.exe lhm lhm-bridge; do
  one="$(sha256sum "$work/one/$bin" | cut -d' ' -f1)"
  two="$(sha256sum "$work/two/$bin" | cut -d' ' -f1)"
  if [[ "$one" == "$two" ]]; then
    echo "$bin: identical ($one)"
  else
    echo "$bin: DIFFERENT" >&2
    echo "  $one" >&2
    echo "  $two" >&2
    status=1
  fi
done
exit $status
