#!/usr/bin/env bash
# Decides which lhm.exe and lhm-bridge.exe a release ships and records the
# hashes of what ships (issue #121, taken over from hwinfo-streamdeck #126).
#
# Windows reputation services judge unsigned files by hash, so a release
# should not ship new hashes without a reason. When the project's own Go code
# behind the two Windows binaries has not changed since the previous release,
# this script replaces the freshly built binaries in the plugin directory with
# the ones from that release, byte for byte. The Linux binaries are always the
# fresh build: nothing judges them by hash.
#
# Only the Windows binaries are recorded and compared here. The Linux binaries
# link against the C library of the machine that builds them, so their bytes
# differ between this machine and the release workflow. Their hashes are
# recorded from the published package, with scripts/record-release-hashes.sh.
#
# Security comes first. The old binaries are only reused when they match the
# hashes recorded for that release in release-hashes.txt and govulncheck finds
# no known vulnerability in them. A dependency or toolchain update that fixes
# something the old binaries contain therefore ships with this release, and an
# asset that was replaced after publication is never carried forward.
#
# Whatever ships is recorded in release-hashes.txt for the version in the
# manifest; commit that file with the release. When that version is already
# tagged (the release workflow builds from the tag), nothing is recorded:
# the binaries are compared with the record instead, and a difference stops
# the release.
#
# Usage: scripts/release-binaries.sh [windows|linux]
#   windows  decide and record lhm.exe and lhm-bridge.exe (default)
#   linux    put the Windows binaries of the decision back, because the Linux
#            package contains them too, and print the hashes of the Linux
#            binaries that were just built
#
# Environment:
#   REBUILD_BINARIES=1   keep the freshly built binaries, whatever changed
#   PREV_TAG=vX.Y.Z      compare against this release instead of the latest tag
#   RELEASE_REPO=o/r     GitHub repository to fetch the previous release from
#   HASHES_FILE=path     hash list (default release-hashes.txt in the repo root)
#   SDPLUGINDIR=path     plugin directory holding the binaries
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

plugin_dir="${SDPLUGINDIR:-$root_dir/com.moeilijk.lhm.sdPlugin}"
hashes_file="${HASHES_FILE:-$root_dir/release-hashes.txt}"
mode="${1:-windows}"
windows_binaries=(lhm.exe lhm-bridge.exe)
linux_binaries=(lhm lhm-bridge lhm-companion)

if [[ "$mode" != "windows" && "$mode" != "linux" ]]; then
  echo "usage: scripts/release-binaries.sh [windows|linux]" >&2
  exit 1
fi

version="$(python3 "$root_dir/scripts/manifest-version.py")"
tag="v$version"
tagged=0
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  tagged=1
fi

# record_or_check writes "<hash>  <tag>/<name>  built:<built>" for a file, or,
# for a version that is already tagged, checks the file against that line.
mismatch=0
record_or_check() {
  local name="$1" file="$2" built="$3" got want tmp
  got="$(sha256sum "$file" | cut -d' ' -f1)"
  echo "  $name $got"
  if [[ "$tagged" == "1" ]]; then
    want="$(awk -v key="$tag/$name" '$2 == key { print $1 }' "$hashes_file" 2>/dev/null | head -n 1)"
    if [[ -z "$want" ]]; then
      echo "release-binaries: $tag is tagged but $tag/$name is not in ${hashes_file#"$root_dir"/}" >&2
      mismatch=1
    elif [[ "$want" != "$got" ]]; then
      echo "release-binaries: $tag/$name differs from the hash recorded for the release" >&2
      echo "  recorded: $want" >&2
      echo "  built:    $got" >&2
      mismatch=1
    fi
    return
  fi
  tmp="$(mktemp)"
  awk -v key="$tag/$name" '$2 != key' "$hashes_file" > "$tmp" || true
  echo "$got  $tag/$name  built:$built" >> "$tmp"
  cat "$tmp" > "$hashes_file"
  rm -f "$tmp"
}

finish() {
  if [[ "$mismatch" == "1" ]]; then
    echo "release-binaries: the build does not match what was recorded for $tag" >&2
    echo "  Run make release before the release commit and commit release-hashes.txt." >&2
    exit 1
  fi
  if [[ "$tagged" == "1" ]]; then
    echo "  $tag is already tagged: the binaries match the record"
  else
    echo "  recorded for $tag in ${hashes_file#"$root_dir"/}"
  fi
}

# report_windows prints and records the two Windows binaries that are in the
# plugin directory now. Fresh binaries were built for this version; reused
# ones keep the release they were built for.
report_windows() {
  local built="${1:-$tag}"
  for bin in "${windows_binaries[@]}"; do
    record_or_check "$bin" "$plugin_dir/$bin" "$built"
  done
}

# decide_windows leaves the right lhm.exe and lhm-bridge.exe in the plugin
# directory and sets $windows_built to the release they were built for.
windows_built="$tag"
decide_windows() {
  if [[ "${REBUILD_BINARIES:-}" == "1" ]]; then
    echo "binaries: rebuilt (REBUILD_BINARIES=1)"
    return
  fi

  local prev
  prev="${PREV_TAG:-$(git describe --tags --abbrev=0 --match 'v*' --exclude "$tag" 2>/dev/null || true)}"
  if [[ -z "$prev" ]]; then
    echo "binaries: rebuilt (no previous release tag)"
    return
  fi

  # Directories of the project's own packages that are compiled into the two
  # Windows binaries. Test files are not compiled in and do not count.
  local dirs changed
  mapfile -t dirs < <(
    GOOS=windows GOARCH=amd64 go list -deps \
      -f '{{if .Module}}{{if .Module.Main}}{{.Dir}}{{end}}{{end}}' \
      ./cmd/lhm_streamdeck_plugin ./cmd/lhm-bridge |
      sed "s#^$root_dir/##" | sort -u
  )
  if [[ ${#dirs[@]} -eq 0 ]]; then
    echo "release-binaries: could not list the packages behind the binaries" >&2
    exit 1
  fi

  # Committed changes since the previous release plus anything uncommitted,
  # because the release is built from the working tree.
  changed="$(
    {
      git diff --name-only "$prev" HEAD -- "${dirs[@]}"
      git status --porcelain -- "${dirs[@]}" | cut -c4-
    } | grep -v '_test\.go$' | sort -u || true
  )"
  if [[ -n "$changed" ]]; then
    echo "binaries: rebuilt (own Go code changed since $prev)"
    echo "$changed" | sed 's/^/  changed: /' | head -n 20
    return
  fi

  # Fetch the previous Windows binaries, check them against their recorded
  # hashes and scan them. Only a clean result (exit 0) allows them to ship
  # again.
  local work scan_status
  work="$(mktemp -d)"
  set +e
  bash "$root_dir/scripts/scan-release.sh" --only windows --out "$work" "$prev" 2>&1 | sed 's/^/  /'
  scan_status=${PIPESTATUS[0]}
  set -e

  case "$scan_status" in
    0) ;;
    3)
      rm -rf "$work"
      echo "binaries: rebuilt ($prev has known vulnerabilities; the fresh build ships instead)"
      return
      ;;
    *)
      rm -rf "$work"
      echo "release-binaries: the $prev binaries could not be verified, so they are not reused" >&2
      echo "  The own Go code is unchanged since $prev. Fix the cause above," >&2
      echo "  or set REBUILD_BINARIES=1 to ship new binaries on purpose." >&2
      exit 1
      ;;
  esac

  for bin in "${windows_binaries[@]}"; do
    cp "$work/$bin" "$plugin_dir/$bin"
  done
  rm -rf "$work"
  echo "binaries: reused from $prev (own Go code unchanged, hashes verified, no known vulnerabilities)"
  windows_built="$(awk -v key="$prev/lhm.exe" '$2 == key { sub(/^built:/, "", $3); print $3 }' "$hashes_file" | head -n 1)"
  [[ -n "$windows_built" ]] || windows_built="$prev"
}

decide_windows
report_windows "$windows_built"

if [[ "$mode" == "linux" ]]; then
  for file in "${linux_binaries[@]}"; do
    if [[ ! -f "$plugin_dir/$file" ]]; then
      echo "release-binaries: $file is missing from the plugin directory" >&2
      exit 1
    fi
    echo "  linux/$file $(sha256sum "$plugin_dir/$file" | cut -d' ' -f1) (this build, not recorded)"
  done
fi

finish
