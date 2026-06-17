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

echo "check: dial_pi.js syntax"
node --check com.moeilijk.lhm.sdPlugin/dial_pi.js

echo "test: settings PI functional script"
node scripts/test-settings-pi.js

echo "test: reading PI functional script"
node scripts/test-reading-pi.js

echo "test: dial PI functional script"
node scripts/test-dial-pi.js

if curl -fsS "${DECKBRIDGE_URL:-http://127.0.0.1:34075}/api/state" >/dev/null 2>&1; then
  echo "test: DeckBridge live dial e2e flow"
  node scripts/test-deckbridge-dial-live.js
  if command -v powershell.exe >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:9998/data.json" >/dev/null 2>&1; then
    echo "test: DeckBridge browser bulk e2e flow"
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test-deckbridge-bulk-browser-e2e.ps1
  else
    echo "skip: DeckBridge browser bulk e2e flow (PowerShell or Bulk E2E source not reachable)"
  fi
else
  echo "skip: DeckBridge live dial e2e flow (DeckBridge not reachable)"
fi

echo "test: Go targets (windows)"
GOOS=windows GOARCH=amd64 GOCACHE=/tmp/go-build go test \
  ./cmd/lhm_streamdeck_plugin \
  ./internal/app/lhmstreamdeckplugin \
  ./pkg/streamdeck \
  ./internal/lhm/plugin \
  ./cmd/lhm-bridge

echo "verify-settings-pi: ok"
