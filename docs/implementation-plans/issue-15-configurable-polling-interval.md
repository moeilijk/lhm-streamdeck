# Implementation Plan: Configurable Polling Interval

**Issue:** [#15 - Feature request: Make polling interval configurable (e.g., 250/500/1000ms)](https://github.com/moeilijk/lhm-streamdeck/issues/15)

**Status:** Partially implemented (see `docs/implementation-plans/issue-15-progress.md`)

---

## Summary

Add a user-configurable polling interval setting to allow faster refresh rates for sensors that change quickly (CPU/GPU power, clocks, fan RPM), while keeping the default at 1000ms for backward compatibility.

Note: the bridge now runs in an on-demand fetch model with the plugin ticker as the primary clock. The original two-tier polling section below is historical design context.

### 2026-02-08 Debug Addendum

- Settings tile appearance save path was hardened:
- PI now writes tile settings only to the active action context.
- Plugin also persists `updateTileAppearance` via `SetSettings` as a backup.

- Poll timing behavior was adjusted:
- PollTime cache TTL is now derived from interval (half interval, bounded), not equal to the full interval.
- This avoids stale PollTime reuse on the next tick (notably visible at `2000ms`).

- Settings tile rendering behavior:
- `showLabel=true` keeps placeholder background path.
- `showLabel=false` uses solid configured background.
- Fallback render path keeps the interval text visible.
- Title handling on settings tile is now image-rendered (custom title or `Refresh Rate` fallback), with native Stream Deck title disabled for this action.

---

## Current Architecture

The plugin uses a **two-tier polling architecture**:

```
LHM HTTP Server (port 8085)
        ↑ HTTP GET /data.json
LHM Bridge (lhm-bridge.exe) ← 1000ms polling (hardcoded)
        ↑ gRPC
Stream Deck Plugin (lhm.exe) ← 1000ms ticker (hardcoded)
        ↓
Stream Deck Tiles
```

### Relevant Code Locations

| Component | Location | Current Value |
|-----------|----------|---------------|
| Bridge polling interval | `internal/lhm/plugin/service.go:19` | `const defaultPollPeriod = time.Second` |
| Plugin update ticker | `internal/app/lhmstreamdeckplugin/action_manager.go:24` | `time.NewTicker(time.Second)` |
| PollTime cache TTL | `internal/app/lhmstreamdeckplugin/plugin.go:46` | `const pollTimeCacheTTL = time.Second` |
| Bridge restart check | `internal/app/lhmstreamdeckplugin/plugin.go:155` | `time.Sleep(1 * time.Second)` |

---

## Proposed Implementation

### Approach: Global Setting via Environment Variable

A global polling interval is preferred because:
1. The bridge poll interval affects all actions equally
2. Prevents race conditions from different actions requesting different intervals
3. Simpler to implement and maintain
4. Matches the issue's suggestion that "global is fine"

### Configuration Options

| Value | Use Case |
|-------|----------|
| `250ms` | High-refresh sensors (power, clocks) - highest overhead |
| `500ms` | Balanced option |
| `1000ms` | Default, current behavior - lowest overhead |

Minimum clamp: **250ms** (to prevent excessive CPU/network load)
Maximum clamp: **2000ms** (any higher defeats the purpose)

---

## Implementation Steps

### Phase 1: Bridge Polling Interval (Backend)

**File:** `internal/lhm/plugin/service.go`

1. Add environment variable support for poll interval:

```go
const (
    defaultEndpoint   = "http://127.0.0.1:8085/data.json"
    defaultPollPeriod = time.Second
    minPollPeriod     = 250 * time.Millisecond
    maxPollPeriod     = 2 * time.Second
)

func StartService() *Service {
    url := os.Getenv("LHM_ENDPOINT")
    if url == "" {
        url = defaultEndpoint
    }

    pollInterval := defaultPollPeriod
    if envInterval := os.Getenv("LHM_POLL_INTERVAL"); envInterval != "" {
        if parsed, err := time.ParseDuration(envInterval); err == nil {
            if parsed >= minPollPeriod && parsed <= maxPollPeriod {
                pollInterval = parsed
            }
        }
    }

    return &Service{
        url:          url,
        client:       &http.Client{Timeout: 2 * time.Second},
        pollInterval: pollInterval,
    }
}
```

### Phase 2: Plugin Update Ticker (Frontend)

**File:** `internal/app/lhmstreamdeckplugin/action_manager.go`

1. Make the action manager accept a configurable interval:

```go
func newActionManager(updateInterval time.Duration) *actionManager {
    if updateInterval < 250*time.Millisecond {
        updateInterval = 250 * time.Millisecond
    }
    return &actionManager{
        actions:        make(map[string]*actionData),
        lastRun:        make(map[string]time.Time),
        updateInterval: updateInterval,
    }
}

func (tm *actionManager) Run(updateTiles func(*actionData)) {
    go func() {
        ticker := time.NewTicker(tm.updateInterval)
        // ... rest unchanged
    }()
}
```

**File:** `internal/app/lhmstreamdeckplugin/plugin.go`

2. Read environment variable and pass to action manager:

```go
func NewPlugin(port, uuid, event, info string) (*Plugin, error) {
    updateInterval := time.Second
    if envInterval := os.Getenv("LHM_UPDATE_INTERVAL"); envInterval != "" {
        if parsed, err := time.ParseDuration(envInterval); err == nil {
            if parsed >= 250*time.Millisecond && parsed <= 2*time.Second {
                updateInterval = parsed
            }
        }
    }

    p := &Plugin{
        am:           newActionManager(updateInterval),
        // ... rest unchanged
    }
    // ...
}
```

### Phase 3: Synchronize Cache TTL

**File:** `internal/app/lhmstreamdeckplugin/plugin.go`

Update the poll time cache TTL to match the configured interval:

```go
// Remove the const and make it a field
type Plugin struct {
    // ... existing fields
    pollTimeCacheTTL time.Duration
}

// In NewPlugin:
p.pollTimeCacheTTL = updateInterval

// In getCachedPollTime:
if !p.cachedPollTimeAt.IsZero() && time.Since(p.cachedPollTimeAt) < p.pollTimeCacheTTL {
    return p.cachedPollTime, nil
}
```

### Phase 4: Pass Interval to Bridge Process

**File:** `internal/app/lhmstreamdeckplugin/plugin.go`

The bridge runs as a separate process, so the environment variable needs to be passed:

```go
func (p *Plugin) startClient() error {
    cmd := exec.Command("./lhm-bridge.exe")

    // Pass polling interval to bridge process
    if interval := os.Getenv("LHM_POLL_INTERVAL"); interval != "" {
        cmd.Env = append(os.Environ(), "LHM_POLL_INTERVAL="+interval)
    }

    // ... rest unchanged
}
```

### Phase 5: Settings Action with Dynamic Update

Instead of adding the setting to every sensor tile's Property Inspector, we create a dedicated **Settings tile**. This provides:
- Clean separation: global settings belong in their own action
- Visual feedback: the tile can show current interval and LHM connection status
- Better UX: user knows exactly where to configure plugin settings

#### 5.1 Add New Action to Manifest

**File:** `com.moeilijk.lhm.sdPlugin/manifest.json`

```json
{
    "Actions": [
        {
            "Icon": "actionIcon",
            "Name": "Libre Hardware Monitor",
            "UUID": "com.moeilijk.lhm.reading",
            ...
        },
        {
            "Icon": "settingsIcon",
            "Name": "LHM Settings",
            "States": [
                {
                    "Image": "settingsImage",
                    "TitleAlignment": "bottom",
                    "FontSize": 9,
                    "ShowTitle": true
                }
            ],
            "SupportedInMultiActions": false,
            "Tooltip": "Configure Libre Hardware Monitor plugin settings",
            "UUID": "com.moeilijk.lhm.settings",
            "PropertyInspectorPath": "settings_pi.html"
        }
    ]
}
```

#### 5.2 Create Settings Property Inspector

**File:** `com.moeilijk.lhm.sdPlugin/settings_pi.html`

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <link rel="stylesheet" href="css/sdpi.css" />
</head>
<body>
    <div class="sdpi-wrapper">
        <div class="sdpi-heading">Performance</div>
        <div class="sdpi-item">
            <div class="sdpi-item-label">Refresh Rate</div>
            <select class="sdpi-item-value select" id="pollInterval">
                <option value="250">250ms (Fast)</option>
                <option value="500">500ms (Balanced)</option>
                <option value="1000" selected>1000ms (Default)</option>
            </select>
        </div>
        <div class="sdpi-item">
            <details class="message info">
                <summary>Info</summary>
                <p>Lower values = faster updates but higher CPU usage.</p>
                <p>Changes apply immediately to all sensor tiles.</p>
            </details>
        </div>

        <div class="sdpi-heading">Status</div>
        <div class="sdpi-item">
            <div class="sdpi-item-label">LHM Connection</div>
            <div class="sdpi-item-value" id="connectionStatus">Checking...</div>
        </div>
    </div>
    <script src="settings_pi.js"></script>
</body>
</html>
```

**File:** `com.moeilijk.lhm.sdPlugin/settings_pi.js`

```javascript
let websocket = null;
let uuid = null;

function connectElgatoStreamDeckSocket(port, pluginUUID, event, info, actionInfo) {
    uuid = pluginUUID;
    websocket = new WebSocket(`ws://127.0.0.1:${port}`);

    websocket.onopen = function() {
        websocket.send(JSON.stringify({ event: event, uuid: uuid }));
        websocket.send(JSON.stringify({ event: "getGlobalSettings", context: uuid }));
    };

    websocket.onmessage = function(evt) {
        const data = JSON.parse(evt.data);
        if (data.event === "didReceiveGlobalSettings") {
            const interval = data.payload?.settings?.pollInterval || 1000;
            document.getElementById('pollInterval').value = interval;
        }
        if (data.event === "sendToPropertyInspector") {
            // Receive status updates from plugin
            if (data.payload?.connectionStatus) {
                document.getElementById('connectionStatus').textContent = data.payload.connectionStatus;
            }
        }
    };
}

document.getElementById('pollInterval').addEventListener('change', function(e) {
    const interval = parseInt(e.target.value);
    websocket.send(JSON.stringify({
        event: "setGlobalSettings",
        context: uuid,
        payload: { pollInterval: interval }
    }));
    // Also notify plugin to apply immediately
    websocket.send(JSON.stringify({
        event: "sendToPlugin",
        action: "com.moeilijk.lhm.settings",
        context: uuid,
        payload: { setPollInterval: interval }
    }));
});
```

#### 5.3 Add Global Settings Type

**File:** `internal/app/lhmstreamdeckplugin/types.go`

```go
type globalSettings struct {
    PollInterval int `json:"pollInterval"` // milliseconds: 250, 500, 1000
}
```

#### 5.4 Dynamic Ticker Update in Action Manager

**File:** `internal/app/lhmstreamdeckplugin/action_manager.go`

```go
type actionManager struct {
    mux            sync.RWMutex
    actions        map[string]*actionData
    lastRun        map[string]time.Time
    updateInterval time.Duration
    intervalChan   chan time.Duration // Channel to signal interval changes
}

func newActionManager(interval time.Duration) *actionManager {
    if interval < 250*time.Millisecond {
        interval = 250 * time.Millisecond
    }
    return &actionManager{
        actions:        make(map[string]*actionData),
        lastRun:        make(map[string]time.Time),
        updateInterval: interval,
        intervalChan:   make(chan time.Duration, 1),
    }
}

func (tm *actionManager) Run(updateTiles func(*actionData)) {
    go func() {
        ticker := time.NewTicker(tm.updateInterval)
        defer ticker.Stop()

        for {
            select {
            case newInterval := <-tm.intervalChan:
                ticker.Stop()
                tm.mux.Lock()
                tm.updateInterval = newInterval
                tm.mux.Unlock()
                ticker = time.NewTicker(newInterval)
                log.Printf("Ticker updated to %v", newInterval)

            case <-ticker.C:
                // ... existing update logic ...
            }
        }
    }()
}

// SetInterval dynamically updates the polling interval
func (tm *actionManager) SetInterval(d time.Duration) {
    if d < 250*time.Millisecond {
        d = 250 * time.Millisecond
    }
    if d > 2*time.Second {
        d = 2 * time.Second
    }
    select {
    case tm.intervalChan <- d:
    default:
        // Channel full, skip (previous update pending)
    }
}
```

#### 5.5 Handle Settings Action Events

**File:** `internal/app/lhmstreamdeckplugin/handlers.go`

```go
func (p *Plugin) DidReceiveGlobalSettings(event streamdeck.DidReceiveGlobalSettingsEvent) {
    var gs globalSettings
    if err := json.Unmarshal(event.Payload.Settings, &gs); err != nil {
        log.Printf("Failed to parse global settings: %v", err)
        return
    }
    p.globalSettings = gs
}

func (p *Plugin) SendToPlugin(event streamdeck.SendToPluginEvent) {
    // Handle settings action commands
    if event.Action == "com.moeilijk.lhm.settings" {
        var payload struct {
            SetPollInterval int `json:"setPollInterval"`
        }
        if err := json.Unmarshal(event.Payload, &payload); err == nil && payload.SetPollInterval > 0 {
            interval := time.Duration(payload.SetPollInterval) * time.Millisecond
            p.am.SetInterval(interval)
            p.pollTimeCacheTTL = interval

            // Restart bridge with new interval
            p.restartBridgeWithInterval(interval)

            log.Printf("Poll interval changed to %v", interval)
        }
    }
    // ... existing sendToPlugin handling ...
}

func (p *Plugin) restartBridgeWithInterval(interval time.Duration) {
    if p.c != nil {
        p.c.Kill()
    }
    // Set env var for new bridge process
    os.Setenv("LHM_POLL_INTERVAL", interval.String())
    _ = p.startClient()
}
```

#### 5.6 Settings Tile Visual Feedback

**File:** `internal/app/lhmstreamdeckplugin/plugin.go`

The settings tile can display the current interval:

```go
func (p *Plugin) updateSettingsTile(context string) {
    interval := p.am.GetInterval()
    title := fmt.Sprintf("%dms", interval.Milliseconds())

    // Create a simple status image or just set the title
    p.sd.SetTitle(context, title)
}
```

#### 5.7 Create Settings Icon

Create new icon files:
- `com.moeilijk.lhm.sdPlugin/settingsIcon.png` (72x72)
- `com.moeilijk.lhm.sdPlugin/settingsIcon@2x.png` (144x144)
- `com.moeilijk.lhm.sdPlugin/settingsImage.png` (72x72)
- `com.moeilijk.lhm.sdPlugin/settingsImage@2x.png` (144x144)

Design: A gear/cog icon with "LHM" text or the LHM logo

---

## Testing Plan

### Unit Tests

1. Test `StartService()` with various `LHM_POLL_INTERVAL` values:
   - Valid: "250ms", "500ms", "1000ms", "1s"
   - Invalid: "100ms" (below min), "5s" (above max), "invalid"
   - Missing: defaults to 1000ms

2. Test `newActionManager()` with configurable intervals

### Integration Tests

1. Set `LHM_POLL_INTERVAL=250ms` and verify faster refresh
2. Verify CPU/memory usage at different intervals
3. Test error handling when LHM is unavailable

### Manual Testing

1. Configure 250ms interval and observe Stream Deck responsiveness
2. Monitor system resources during extended use
3. Test with multiple tiles simultaneously

---

## Documentation Updates

### README.md

Add a section on configuring polling interval:

```markdown
## Performance Tuning

The plugin polls Libre Hardware Monitor at a configurable interval.
Set the `LHM_POLL_INTERVAL` environment variable before starting Stream Deck:

- `250ms` - Fast refresh (higher CPU usage)
- `500ms` - Balanced
- `1000ms` - Default (recommended)

Example (Windows):
```powershell
[Environment]::SetEnvironmentVariable("LHM_POLL_INTERVAL", "500ms", "User")
```
```

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Higher CPU usage at 250ms | Clamp minimum to 250ms; document tradeoffs |
| Network overhead to LHM | HTTP client reuses connections; minimal impact |
| Race condition with multiple intervals | Global setting prevents per-action conflicts |
| Breaking change for existing users | Default remains 1000ms |
| Bridge restart on interval change | Brief interruption (~1s), tiles show "unavailable" then recover |

---

## Estimated Changes

| File | Changes |
|------|---------|
| `com.moeilijk.lhm.sdPlugin/manifest.json` | Add new "LHM Settings" action |
| `com.moeilijk.lhm.sdPlugin/settings_pi.html` | New Property Inspector for settings tile |
| `com.moeilijk.lhm.sdPlugin/settings_pi.js` | JavaScript for settings PI |
| `com.moeilijk.lhm.sdPlugin/settingsIcon*.png` | New icon assets (4 files) |
| `internal/lhm/plugin/service.go` | Add env var parsing, min/max clamping |
| `internal/app/lhmstreamdeckplugin/action_manager.go` | Add intervalChan, SetInterval method |
| `internal/app/lhmstreamdeckplugin/plugin.go` | Add restartBridgeWithInterval, settings tile updates |
| `internal/app/lhmstreamdeckplugin/types.go` | Add globalSettings struct |
| `internal/app/lhmstreamdeckplugin/handlers.go` | Handle settings action events |
| `README.md` | Document the Settings tile |

---

## Future Enhancements (Out of Scope)

1. **Per-action polling interval** - Allow different intervals per tile
2. **Adaptive polling** - Slower refresh when values are stable
