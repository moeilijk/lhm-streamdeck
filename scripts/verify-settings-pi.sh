#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root_dir"

echo "check: settings_pi.js syntax"
node --check com.moeilijk.lhm.sdPlugin/settings_pi.js

echo "check: pi_utils.js syntax"
node --check com.moeilijk.lhm.sdPlugin/pi_utils.js

echo "check: index_pi.js syntax"
node --check com.moeilijk.lhm.sdPlugin/index_pi.js

echo "check: composite_pi.js syntax"
node --check com.moeilijk.lhm.sdPlugin/composite_pi.js

echo "check: derived_pi.js syntax"
node --check com.moeilijk.lhm.sdPlugin/derived_pi.js

echo "test: settings PI functional script"
node scripts/test-settings-pi.js

echo "test: reading PI functional script"
node scripts/test-reading-pi.js

echo "test: Go targets (windows)"
GOOS=windows GOARCH=amd64 GOCACHE=/tmp/go-build go test \
  ./cmd/lhm_streamdeck_plugin \
  ./internal/app/lhmstreamdeckplugin \
  ./pkg/streamdeck \
  ./internal/lhm/plugin \
  ./cmd/lhm-bridge

echo "verify-settings-pi: ok"
