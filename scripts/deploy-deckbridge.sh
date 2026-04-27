#!/usr/bin/env bash
# Deploy lhm-streamdeck plugin to DeckBridge (Linux) and restart the daemon.
# Usage: scripts/deploy-deckbridge.sh
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plugin_src="$root_dir/com.moeilijk.lhm.sdPlugin"
plugin_dst="$HOME/.config/DeckBridge/plugins/com.moeilijk.lhm.sdPlugin"
deckbridge_dir="$HOME/projects/GitHub/DeckBridge"
log_file="/tmp/deckbridge.log"

# ── build ──────────────────────────────────────────────────────────────────
echo "build: $plugin_src/lhm.exe"
(
  cd "$root_dir"
  GOOS=windows GOARCH=amd64 go build -o "$plugin_src/lhm.exe"          ./cmd/lhm_streamdeck_plugin
  GOOS=windows GOARCH=amd64 go build -o "$plugin_src/lhm-bridge.exe"   ./cmd/lhm-bridge
)

# ── stop existing DeckBridge daemon ────────────────────────────────────────
echo "kill: DeckBridge + plugin processes"
pkill -f "node.*DeckBridge" 2>/dev/null || true
pkill -f "lhm.exe"          2>/dev/null || true
sleep 1

# ── deploy plugin files ────────────────────────────────────────────────────
echo "copy: $plugin_src -> $plugin_dst"
mkdir -p "$plugin_dst"
rsync -a --delete "$plugin_src/" "$plugin_dst/"

# ── start DeckBridge daemon ─────────────────────────────────────────────────
echo "start: DeckBridge"
cd "$deckbridge_dir"
node dist/index.js >"$log_file" 2>&1 &
daemon_pid=$!

# wait for dashboard URL to appear in log (max 15s)
echo "waiting for dashboard..."
for i in $(seq 1 30); do
  if grep -q "Dashboard:" "$log_file" 2>/dev/null; then
    break
  fi
  sleep 0.5
done

dashboard_url=$(grep "Dashboard:" "$log_file" 2>/dev/null | tail -1 | sed 's/.*Dashboard: //')
if [[ -z "$dashboard_url" ]]; then
  echo "DeckBridge started (PID $daemon_pid) but dashboard URL not found yet."
  echo "Check: $log_file"
else
  echo ""
  echo "DeckBridge PID: $daemon_pid"
  echo "Dashboard:      $dashboard_url"
  echo "Log:            $log_file"
fi
