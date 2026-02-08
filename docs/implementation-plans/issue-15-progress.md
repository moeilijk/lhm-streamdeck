# Issue #15 Implementation Progress

**Last updated:** 2026-02-08  
**Status:** In progress (core polling model implemented; latest PI/settings-tile fixes deployed for manual verification)

## What Is Implemented

- [x] Added `LHM Settings` action (`com.moeilijk.lhm.settings`) to `manifest.json`
- [x] Added settings Property Inspector (`settings_pi.html` / `settings_pi.js`)
- [x] Added dynamic plugin ticker interval support in `action_manager.go`
- [x] Added global settings plumbing (`GetGlobalSettings`, `SetGlobalSettings`, `didReceiveGlobalSettings`)
- [x] Added settings tile rendering path in plugin code
- [x] Switched bridge to on-demand fetch model (no background polling loop)
- [x] Removed bridge restart dependency from interval changes

## Current Polling Architecture

Single clock model:

```text
LHM HTTP Server (data.json)
        ^
        | on demand fetch
LHM Bridge (cache + mutex)
        ^
        | gRPC
Plugin ticker (action manager interval: 100-30000ms)
        |
Stream Deck tiles
```

Runtime behavior:

1. Plugin ticker fires.
2. Plugin asks bridge `PollTime()`.
3. Bridge refreshes snapshot once for that tick.
4. Plugin uses `lastPollTime` to skip duplicate renders per tile.
5. Reading calls use cached snapshot for that tick.

## Key Code Changes (Current Update)

### Bridge loop removed

- `cmd/lhm-bridge/main.go`
- Removed infinite goroutine loop that called `service.Recv()` continuously.

### Bridge switched to on-demand refresh

- `internal/lhm/plugin/service.go`
- `PollTime()` now triggers refresh directly.
- `SensorsSnapshot()` and `ReadingsBySensorID()` load initial snapshot only if not ready.
- Added `fetchMu` to serialize concurrent refreshes and avoid duplicate HTTP calls.
- Removed bridge-side poll-interval env parsing from service startup path.

### Plugin interval update no longer restarts bridge

- `internal/app/lhmstreamdeckplugin/plugin.go`
- `setPollInterval()` updates action-manager interval and cache TTL, then updates settings tiles and global settings.
- Removed `restartBridgeWithInterval()`.

### Settings tile UI synchronization fixes

- `internal/app/lhmstreamdeckplugin/action_manager.go`
- `SetInterval()` now updates cached interval immediately and replaces stale queued interval updates.

- `internal/app/lhmstreamdeckplugin/plugin.go`
- Settings tile now renders current rate from `globalSettings.PollInterval` to avoid stale display values.
- `setPollInterval()` now updates `globalSettings` before tile redraw.

- `internal/app/lhmstreamdeckplugin/delegate.go`
- Settings tile defaults now force `showLabel=true` when the field is missing in older settings.
- Global settings handler now always syncs/clamps interval and redraws settings tiles.

- `com.moeilijk.lhm.sdPlugin/settings_pi.js`
- Hardened PI parsing and context fallback for `didReceiveSettings` / `didReceiveGlobalSettings`.
- Added websocket-ready guards before sending updates.

### Latest debug-driven fixes (2026-02-08)

- `com.moeilijk.lhm.sdPlugin/settings_pi.js`
- Reworked settings save flow to always use the active action context.
- Added normalized color handling and change-detection signature to avoid stale/default writes.
- Added resilient polling fallback (300ms) so color changes are still persisted if WebView misses native color events.

- `internal/app/lhmstreamdeckplugin/delegate.go`
- `updateTileAppearance` now persists through `SetSettings` from plugin side as an extra safety net.
- Global settings interval apply now uses dynamic PollTime cache TTL (`pollTimeCacheTTLForInterval`), not raw interval.

- `internal/app/lhmstreamdeckplugin/plugin.go`
- PollTime cache TTL now scales to half of interval (bounded), preventing next tick from reusing stale PollTime at slower rates.
- Settings tile fallback render now keeps showing rate text when placeholder composition fails.
- Settings tile now renders title and value in-image (no native title draw), so custom title replaces `Refresh Rate` in the same top slot.
- Settings tile now clears native Stream Deck title on every redraw to prevent duplicate/misaligned overlays.

- `com.moeilijk.lhm.sdPlugin/manifest.json`
- Settings action state now uses `ShowTitle: false` to keep title handling fully in rendered tile image.

- `scripts/fix-settings-title-alignment.ps1`
- Profile migration now enforces `ShowTitle=false` for settings action states.

## Validation Run

Executed:

- `GOCACHE=/tmp/go-build go test ./internal/lhm/plugin ./cmd/lhm-bridge`
- `GOOS=windows GOARCH=amd64 GOCACHE=/tmp/go-build go test ./cmd/lhm_streamdeck_plugin ./internal/app/lhmstreamdeckplugin`
- `GOOS=windows GOARCH=amd64 GOCACHE=/tmp/go-build go build -o com.moeilijk.lhm.sdPlugin/lhm.exe ./cmd/lhm_streamdeck_plugin`
- `GOOS=windows GOARCH=amd64 GOCACHE=/tmp/go-build go build -o com.moeilijk.lhm.sdPlugin/lhm-bridge.exe ./cmd/lhm-bridge`

Result:

- All commands passed.

## Local Deploy

Executed:

- `WIN_USER=cvdveer bash scripts/deploy-local.sh`

Result:

- Rebuilt binaries were copied to local Stream Deck plugin folder and Stream Deck restarted.

## Manual Verification In Progress

- [ ] `Show Label` checkbox updates settings tile rendering as expected.
- [ ] Settings tile color changes replace placeholder icon immediately.
- [ ] Chosen colors persist across PI close/reopen and Stream Deck restart.
- [ ] Settings tile `Current Rate` always matches selected interval.

## Next Work Items

1. Complete manual validation in Stream Deck UI.
2. Capture any remaining edge-cases found during testing and patch if needed.
