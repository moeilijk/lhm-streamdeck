# Issue #15 Implementation Progress

**Last updated:** Implementation complete, ready for testing

## Build Status

Build successful with `GOOS=windows go build ./...`

## All Steps Completed

- [x] **manifest.json** - Added new "LHM Settings" action with `com.moeilijk.lhm.settings` UUID
- [x] **settings_pi.html** - Created Property Inspector with poll interval dropdown and status display
- [x] **settings_pi.js** - Created JavaScript handler for global settings get/set
- [x] **Settings icon images** - Created placeholder PNG files (copies of existing icons)
- [x] **service.go** - Added environment variable parsing for `LHM_POLL_INTERVAL` with min/max clamping
- [x] **action_manager.go** - Added dynamic interval support with channel-based ticker reset
- [x] **types.go** - Added `globalSettings` struct
- [x] **streamdeck/types.go** - Added `EvDidReceiveGlobalSettings` type
- [x] **streamdeck/streamdeck.go** - Added `GetGlobalSettings`, `SetGlobalSettings`, `OnWillDisappear`, `OnDidReceiveGlobalSettings`
- [x] **delegate.go** - Added handlers for settings action, global settings events
- [x] **plugin.go** - Added `setPollInterval`, `restartBridgeWithInterval`, `updateSettingsTile` methods

## Files Modified

| File | Changes |
|------|---------|
| `com.moeilijk.lhm.sdPlugin/manifest.json` | Added "LHM Settings" action |
| `com.moeilijk.lhm.sdPlugin/settings_pi.html` | New file - Settings Property Inspector |
| `com.moeilijk.lhm.sdPlugin/settings_pi.js` | New file - Settings PI JavaScript |
| `com.moeilijk.lhm.sdPlugin/settingsIcon*.png` | New files - Placeholder icons (4 files) |
| `internal/lhm/plugin/service.go` | Added min/max poll period, env var parsing |
| `internal/app/lhmstreamdeckplugin/action_manager.go` | Added intervalChan, SetInterval, GetInterval |
| `internal/app/lhmstreamdeckplugin/types.go` | Added globalSettings struct |
| `internal/app/lhmstreamdeckplugin/delegate.go` | Added settings action handlers, OnDidReceiveGlobalSettings |
| `internal/app/lhmstreamdeckplugin/plugin.go` | Added globalSettings field, setPollInterval, updateSettingsTile |
| `pkg/streamdeck/types.go` | Added EvDidReceiveGlobalSettings |
| `pkg/streamdeck/streamdeck.go` | Added GetGlobalSettings, SetGlobalSettings, event handlers |

## Testing Instructions

1. Build the plugin: `GOOS=windows GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm.exe ./cmd/lhm_streamdeck_plugin`
2. Build the bridge: `GOOS=windows GOARCH=amd64 go build -o com.moeilijk.lhm.sdPlugin/lhm-bridge.exe ./cmd/lhm-bridge`
3. Copy the plugin to Stream Deck plugins folder
4. Restart Stream Deck
5. Add "LHM Settings" tile to deck
6. Open Property Inspector and change Refresh Rate
7. Verify all sensor tiles update at new rate

## How It Works

1. **Settings Tile**: User drags "LHM Settings" tile to deck
2. **Property Inspector**: Shows Refresh Rate dropdown (250ms, 500ms, 1000ms)
3. **On Change**: PI sends `setGlobalSettings` and `sendToPlugin` with new interval
4. **Plugin**: Calls `setPollInterval()` which:
   - Updates action manager ticker dynamically
   - Restarts bridge with new `LHM_POLL_INTERVAL` env var
   - Updates all settings tiles to show current rate
   - Saves to Stream Deck global settings
5. **On Startup**: Plugin requests global settings and applies saved interval
