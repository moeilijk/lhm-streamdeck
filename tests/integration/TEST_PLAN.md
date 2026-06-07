# Integration test plan

## Already covered

### test-global-thresholds.js (10 assertions)
- Global threshold auto-apply by ReadingType
- Type filter: Temp global ignored on Load tile
- Suppress: suppressed global does not fire
- Unsuppress: global fires after unsuppress
- PI visibility: lhmTypeToReadingType maps all LHM type strings correctly

### test-per-tile-thresholds.js (8 assertions)
- Per-tile threshold fires above value, not below
- Threshold isolation: tile A threshold does not affect tile B
- smoothingAlpha persisted
- updateIntervalOverrideMs persisted
- updateIntervalOverrideMs throttles evaluation (live behavior)
- graphHeightPct, graphLineThickness, textStroke, textStrokeColor persisted

### test-composite-thresholds.js (6 assertions)
- Composite slot 0 per-slot threshold fires
- Slot independence: slot 1 (no threshold) not affected while slot 0 fires
- Composite smoothingAlpha persisted
- Composite updateIntervalOverrideMs persisted

## To be built

### test-derived-thresholds.js (keys 30–31)
- Derived tile: configure slots (e.g. 2× CPU Package, formula=average)
- Per-tile threshold fires when formula value exceeds limit
- Threshold does NOT fire below limit
- smoothingAlpha, updateIntervalOverrideMs persisted on derived tile
- derived_graphHeightPct, derived_graphLineThickness, derived_textStroke persisted

### test-composite-global-suppress.js (keys 40–41)
- Global threshold auto-applies to composite slot with matching ReadingType
- Composite slot suppresses global: slot does not fire, other tile does
- Composite slot unsuppresses global: slot fires again

### test-settings-tile.js (key 50)
- connectionStatus = connected when mock server is running
- connectionStatus = disconnected when mock server is stopped
- PollInterval change persisted

### test-favorites.js (keys 60–61)
- Save a favorite from a reading tile
- Apply favorite to another reading tile: correct sensor+reading selected
- Remove favorite: gone from list

### test-source-profiles.js (key 70)
- Add source profile, set host+port
- Set as default: tiles switch to that profile
- Remove profile
