#!/usr/bin/env bash
# Fetches the binaries of a published release, checks them against the hashes
# recorded in release-hashes.txt and scans them for known vulnerabilities with
# govulncheck in binary mode (issue #121, taken over from hwinfo-streamdeck #126).
#
# Usage: scripts/scan-release.sh [--out <dir>] [--only windows|linux] [<tag>]
#   <tag>        release to scan; default: the latest release of the repository
#   --out <dir>  keep the binaries of that release in <dir>, the Linux ones
#                under <dir>/linux
#   --only <kind>  check and scan only the Windows or only the Linux binaries
#
# Exit status: 0 clean, 3 known vulnerabilities found, 1 anything else
# (download failed, hash unknown or different, scanner failed). Only 0 means
# the binaries are fit to ship again.
#
# Environment:
#   RELEASE_REPO=o/r   GitHub repository (default moeilijk/lhm-streamdeck)
#   HASHES_FILE=path   hash list (default release-hashes.txt in the repo root)
#   GOVULNCHECK=cmd    scanner command (default: govulncheck from PATH or
#                      GOPATH/bin, else `go run` of the latest version)
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="${RELEASE_REPO:-moeilijk/lhm-streamdeck}"
hashes_file="${HASHES_FILE:-$root_dir/release-hashes.txt}"
plugin_dir="com.moeilijk.lhm.sdPlugin"
# The binaries of a release, as "<name in release-hashes.txt>:<file in the
# package>:<which package>". A release has a Windows package and, since the
# Linux build exists, a Linux package with "-linux" in its name. Releases
# before that have only the first; the Linux entries are then skipped.
binaries=(
  "lhm.exe:lhm.exe:windows"
  "lhm-bridge.exe:lhm-bridge.exe:windows"
  "linux/lhm:lhm:linux"
  "linux/lhm-bridge:lhm-bridge:linux"
  "linux/lhm-companion:lhm-companion:linux"
)

out=""
tag=""
only=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --only)
      only="$2"
      if [[ "$only" != "windows" && "$only" != "linux" ]]; then
        echo "scan-release: --only takes windows or linux" >&2
        exit 1
      fi
      shift 2
      ;;
    -*) echo "scan-release: unknown option $1" >&2; exit 1 ;;
    *) tag="$1"; shift ;;
  esac
done

if [[ -z "$tag" ]]; then
  tag="$(gh release view --repo "$repo" --json tagName --jq .tagName 2>/dev/null || true)"
  if [[ -z "$tag" ]]; then
    echo "scan-release: could not determine the latest release of $repo" >&2
    exit 1
  fi
fi

if [[ -n "${GOVULNCHECK:-}" ]]; then
  read -r -a scanner <<< "$GOVULNCHECK"
elif command -v govulncheck >/dev/null 2>&1; then
  scanner=(govulncheck)
elif [[ -x "$(go env GOPATH)/bin/govulncheck" ]]; then
  scanner=("$(go env GOPATH)/bin/govulncheck")
else
  scanner=(go run golang.org/x/vuln/cmd/govulncheck@latest)
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if ! gh release download "$tag" --repo "$repo" --pattern '*.streamDeckPlugin' --dir "$work" >/dev/null; then
  echo "scan-release: could not download the $tag release asset from $repo" >&2
  exit 1
fi
windows_package="$(find "$work" -name '*.streamDeckPlugin' ! -name '*-linux*' | head -n 1)"
linux_package="$(find "$work" -name '*-linux*.streamDeckPlugin' | head -n 1)"
if [[ -z "$windows_package" ]]; then
  echo "scan-release: release $tag has no Windows .streamDeckPlugin asset" >&2
  exit 1
fi

echo "release: $tag ($repo)"
scanned=()
for entry in "${binaries[@]}"; do
  IFS=: read -r name file kind <<< "$entry"
  if [[ -n "$only" && "$kind" != "$only" ]]; then
    continue
  fi
  package="$windows_package"
  if [[ "$kind" == "linux" ]]; then
    if [[ -z "$linux_package" ]]; then
      continue
    fi
    package="$linux_package"
  fi
  mkdir -p "$(dirname "$work/bin/$name")"
  if ! unzip -p "$package" "$plugin_dir/$file" > "$work/bin/$name" 2>/dev/null || [[ ! -s "$work/bin/$name" ]]; then
    # lhm-companion is bundled from a certain release on; a package without
    # it is not an error, a package without the plugin binaries is.
    if [[ "$file" == "lhm-companion" ]]; then
      rm -f "$work/bin/$name"
      continue
    fi
    echo "scan-release: $file not found in the $tag $kind package" >&2
    exit 1
  fi
  got="$(sha256sum "$work/bin/$name" | cut -d' ' -f1)"
  want="$(awk -v key="$tag/$name" '$2 == key { print $1 }' "$hashes_file" 2>/dev/null | head -n 1)"
  if [[ -z "$want" ]]; then
    echo "scan-release: no recorded hash for $tag/$name in ${hashes_file#"$root_dir"/}" >&2
    echo "  Without it the download cannot be told apart from a replaced asset." >&2
    exit 1
  fi
  if [[ "$got" != "$want" ]]; then
    echo "scan-release: $tag/$name does not match its recorded hash" >&2
    echo "  recorded:   $want" >&2
    echo "  downloaded: $got" >&2
    exit 1
  fi
  echo "  $name $got (matches the recorded hash)"
  scanned+=("$name")
done

if [[ ${#scanned[@]} -eq 0 ]]; then
  echo "scan-release: release $tag has no binaries to check" >&2
  exit 1
fi

vulnerable=0
for bin in "${scanned[@]}"; do
  set +e
  report="$("${scanner[@]}" -mode=binary "$work/bin/$bin" 2>&1)"
  status=$?
  set -e
  case "$status" in
    0)
      echo "  $bin: no known vulnerabilities"
      ;;
    3)
      vulnerable=1
      ids="$(grep -oE '^Vulnerability #[0-9]+: [A-Z0-9-]+' <<< "$report" | awk '{print $3}' | paste -sd' ' -)"
      echo "  $bin: $(wc -w <<< "$ids") known vulnerabilities: $ids"
      ;;
    *)
      echo "scan-release: the scanner failed on $bin (exit $status)" >&2
      echo "$report" | tail -n 5 >&2
      exit 1
      ;;
  esac
done

if [[ -n "$out" ]]; then
  mkdir -p "$out"
  for bin in "${scanned[@]}"; do
    mkdir -p "$(dirname "$out/$bin")"
    cp "$work/bin/$bin" "$out/$bin"
  done
fi

if [[ "$vulnerable" == "1" ]]; then
  echo "result: $tag ships binaries with known vulnerabilities"
  exit 3
fi
echo "result: $tag is clean"
