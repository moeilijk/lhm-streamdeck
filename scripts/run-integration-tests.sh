#!/usr/bin/env bash
# Run integration tests against DeckBridge + mock sensor server.
# Usage: scripts/run-integration-tests.sh [--keep-alive]
#
# Steps:
#   1. Build plugin + mock-sensor-server
#   2. Start mock-sensor-server on :9999
#   3. Deploy plugin to DeckBridge and restart DeckBridge
#   4. Run tests/integration/test-global-thresholds.js
#   5. Tear down unless --keep-alive is passed
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."
LOG_FILE="/tmp/deckbridge.log"
MOCK_PID_FILE="/tmp/mock-sensor-server.pid"
KEEP_ALIVE=false

for arg in "$@"; do
  [[ "$arg" == "--keep-alive" ]] && KEEP_ALIVE=true
done

# ── cleanup on exit ────────────────────────────────────────────────────────────
cleanup() {
  echo ""
  echo "── cleanup ──"
  if [[ -f "$MOCK_PID_FILE" ]]; then
    kill "$(cat "$MOCK_PID_FILE")" 2>/dev/null || true
    rm -f "$MOCK_PID_FILE"
    echo "mock-sensor-server stopped"
  fi
  if [[ "$KEEP_ALIVE" == "false" ]]; then
    pkill -f "node.*DeckBridge" 2>/dev/null || true
    echo "DeckBridge stopped"
  else
    echo "DeckBridge kept alive (--keep-alive)"
  fi
}
trap cleanup EXIT

# ── 1. Build ───────────────────────────────────────────────────────────────────
echo "── build ──"
(
  cd "$ROOT"
  GOOS=linux GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm ./cmd/lhm_streamdeck_plugin
  GOOS=linux GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm-bridge ./cmd/lhm-bridge
  GOOS=windows GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm.exe ./cmd/lhm_streamdeck_plugin
  GOOS=windows GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm-bridge.exe ./cmd/lhm-bridge
  go build -o /tmp/mock-sensor-server ./cmd/mock-sensor-server
)
echo "build OK"

# ── 2. Start mock sensor server ────────────────────────────────────────────────
echo "── mock sensor server ──"
/tmp/mock-sensor-server -port 9999 >/tmp/mock-sensor-server.log 2>&1 &
echo $! > "$MOCK_PID_FILE"
echo "mock-sensor-server started (PID $(cat $MOCK_PID_FILE))"
sleep 0.5

# verify it's up
if ! curl -sf http://127.0.0.1:9999/list >/dev/null 2>&1; then
  echo "ERROR: mock-sensor-server not responding on :9999"
  exit 1
fi
echo "mock-sensor-server: OK"

# ── 3. Deploy + start DeckBridge ──────────────────────────────────────────────
echo "── DeckBridge deploy ──"
bash "$SCRIPT_DIR/deploy-deckbridge.sh"
echo "DeckBridge deploy: OK"

# ── 4. Run tests ───────────────────────────────────────────────────────────────
echo ""
echo "── integration tests ──"
cd "$ROOT"

OVERALL_EXIT=0

for TEST_FILE in \
  tests/integration/test-global-thresholds.js \
  tests/integration/test-per-tile-thresholds.js \
  tests/integration/test-composite-thresholds.js \
  tests/integration/test-derived-thresholds.js \
  tests/integration/test-composite-global-suppress.js \
  tests/integration/test-settings-tile.js \
  tests/integration/test-favorites.js \
  tests/integration/test-source-profiles.js; do
  echo ""
  echo "── $TEST_FILE ──"
  node "$TEST_FILE"
  FILE_EXIT=$?
  if [[ $FILE_EXIT -ne 0 ]]; then
    OVERALL_EXIT=$FILE_EXIT
    echo "FAILED: $TEST_FILE (exit $FILE_EXIT)"
  fi
done

echo ""
if [[ $OVERALL_EXIT -eq 0 ]]; then
  echo "ALL TESTS PASSED"
else
  echo "SOME TESTS FAILED"
fi

exit $OVERALL_EXIT
