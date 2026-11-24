#!/usr/bin/env bash
# Deploy lhm-streamdeck plugin to DeckBridge (Linux) and restart the daemon.
# Usage: scripts/deploy-deckbridge.sh
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plugin_src="$root_dir/com.moeilijk.lhm.sdPlugin"
plugin_dst="$HOME/.config/DeckBridge/plugins/com.moeilijk.lhm.sdPlugin"
deckbridge_dir="$HOME/projects/GitHub/DeckBridge"
companion_dir="$HOME/projects/GitHub/lhm-companion"
companion_bin="$companion_dir/build/lhm-companion"
companion_port=8085
settings_file="$HOME/.config/DeckBridge/settings/com.moeilijk.lhm.json"
log_file="/tmp/deckbridge.log"
companion_log="/tmp/lhm-companion.log"

# Since #77 lhm-companion is the only Linux sensor source: localhost profiles
# poll companion over HTTP (the old hwmon path and its WSL-IP workaround are
# gone), so the source profile simply points at 127.0.0.1.
host_ip="127.0.0.1"

# ── build ──────────────────────────────────────────────────────────────────
echo "build: $plugin_src/lhm.exe + lhm (linux)"
(
  cd "$root_dir"
  GOOS=windows GOARCH=amd64 go build -o "$plugin_src/lhm.exe"        ./cmd/lhm_streamdeck_plugin
  GOOS=windows GOARCH=amd64 go build -o "$plugin_src/lhm-bridge.exe" ./cmd/lhm-bridge
  GOOS=linux   GOARCH=amd64 go build -o "$plugin_src/lhm"            ./cmd/lhm_streamdeck_plugin
  GOOS=linux   GOARCH=amd64 go build -o "$plugin_src/lhm-bridge"     ./cmd/lhm-bridge
  chmod +x "$plugin_src/lhm" "$plugin_src/lhm-bridge"
)
# Bundle lhm-companion next to the plugin binary (#77): the plugin spawns it
# for local profiles when nothing is listening on the endpoint.
if [[ -d "$companion_dir" ]]; then
  (cd "$companion_dir" && GOOS=linux GOARCH=amd64 go build -o "$plugin_src/lhm-companion" ./cmd/lhm-companion)
  chmod +x "$plugin_src/lhm-companion"
fi

# ── build + start lhm-companion (Linux sensor source) ──────────────────────
# Provides CPU load, memory, network and disk readings that the empty WSL
# /sys/class/hwmon cannot. Built from the sibling lhm-companion repo.
if [[ -d "$companion_dir" ]]; then
  echo "build: lhm-companion"
  ( cd "$companion_dir" && go build -o "$companion_bin" ./cmd/lhm-companion )
else
  echo "warn: $companion_dir not found — skipping companion (no sensor data on WSL)"
fi

# ── stop existing DeckBridge daemon ────────────────────────────────────────
echo "stop: DeckBridge and everything it started"
# stop-deckbridge.sh finds the processes through /proc and also stops the
# helpers of the other plugins DeckBridge loads; it fails when a process or
# the daemon port survives, so a stale daemon can never keep serving old code.
bash "$root_dir/scripts/stop-deckbridge.sh" "$companion_bin"

if [[ -x "$companion_bin" ]]; then
  echo "start: lhm-companion on $host_ip:$companion_port"
  setsid "$companion_bin" -port "$companion_port" >"$companion_log" 2>&1 </dev/null &
  sleep 1
  if curl -s -o /dev/null "http://$host_ip:$companion_port/data.json"; then
    echo "       companion serving http://$host_ip:$companion_port/data.json"
  else
    echo "warn: companion not reachable at http://$host_ip:$companion_port/data.json (check $companion_log)"
  fi
fi

# ── deploy plugin files ────────────────────────────────────────────────────
echo "copy: $plugin_src -> $plugin_dst"
mkdir -p "$plugin_dst"
rsync -a --delete "$plugin_src/" "$plugin_dst/"

# Inject CodePathLinux into the deployed manifest so DeckBridge runs lhm natively.
# Skip when plugin_dst is a symlink pointing back at the source (e.g. development layout),
# to avoid corrupting the committed manifest.json.
plugin_src_real=$(realpath "$plugin_src")
plugin_dst_real=$(realpath "$plugin_dst" 2>/dev/null || echo "")
if [[ "$plugin_src_real" == "$plugin_dst_real" ]]; then
  echo "note: plugin_dst is symlinked to source — skipping CodePathLinux injection"
else
  python3 - "$plugin_dst/manifest.json" <<'PYEOF'
import json, sys
path = sys.argv[1]
with open(path) as f:
    m = json.load(f)
m['CodePathLinux'] = 'lhm'
with open(path, 'w') as f:
    json.dump(m, f, indent=2)
PYEOF
fi

# ── seed companion source profile into DeckBridge settings ─────────────────
# Adds (or updates) a "lhm-companion (WSL)" source pointing at localhost and
# selects it as default, so tiles have a live data source on first boot.
# Existing profiles are preserved.
if [[ -x "$companion_bin" ]]; then
  echo "settings: companion source $host_ip:$companion_port -> default"
  mkdir -p "$(dirname "$settings_file")"
  python3 - "$settings_file" "$host_ip" "$companion_port" <<'PYEOF'
import json, os, sys
path, host, port = sys.argv[1], sys.argv[2], int(sys.argv[3])
data = {}
if os.path.exists(path):
    try:
        with open(path) as f:
            data = json.load(f)
    except (json.JSONDecodeError, ValueError):
        data = {}
data.setdefault("pollInterval", 1000)
profiles = data.setdefault("sourceProfiles", [])
cid = "companion_wsl"
entry = {"id": cid, "name": "lhm-companion (WSL)", "host": host, "port": port}
for p in profiles:
    if p.get("id") == cid:
        p.update(entry)
        break
else:
    profiles.append(entry)
data["defaultSourceProfileId"] = cid
with open(path, "w") as f:
    json.dump(data, f, indent=2)
PYEOF
fi

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

# ── dials for the live dial e2e suites ──────────────────────────────────────
# `make verify` runs dial suites against the dial that is on the emulated
# Stream Deck +. Put the two dials they need in place when they are missing.
# Under the XL profile the emulated Stream Deck + has no encoders and a dial
# cannot be placed.
if [[ "${DECKBRIDGE_DEVICE_PROFILE:-}" == "streamdeck-xl" ]]; then
  echo "dials: skipped, the streamdeck-xl profile has no encoders"
elif [[ -n "$dashboard_url" ]]; then
  # Without node_modules in the repo, 'ws' comes from the DeckBridge checkout.
  NODE_PATH="${NODE_PATH:-$deckbridge_dir/node_modules}" node "$root_dir/scripts/ensure-deckbridge-dials.js" || echo "warn: dials for the live dial e2e suites are not in place"
fi
