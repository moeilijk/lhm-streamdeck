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

### test-derived-thresholds.js (7 assertions)
- Global threshold (readingType="") fires on derived tile at high average
- Global threshold not active at low average
- Global threshold also fires on reference reading tile
- Derived tile suppress: suppressed global does not fire
- Global still fires on unsuppressed tile while derived is suppressed
- Unsuppress restores derived tile firing
- smoothingAlpha, updateIntervalOverrideMs, graphHeightPct, graphLineThickness, textStroke all persisted

### test-composite-global-suppress.js (5 assertions)
- Global Temp threshold auto-applies to composite slot 0
- Composite slot 0 not active at 45°C
- Composite slot 0 suppress: global does not fire
- Global still fires on unsuppressed reading tile while composite is suppressed
- Composite slot 0 unsuppress: global fires again

### test-settings-tile.js (5 assertions)
- connectionStatus = Connected with mock server running
- currentRate is a positive number
- sourceProfiles is an array
- setPollInterval(2000): currentRate reflects change
- pollInterval=2000 persisted to global settings file

### test-favorites.js (6 assertions)
- Toggle favorite saves it (count=1, id correct, readingLabel correct)
- Favorite visible in tile B catalog
- Apply favorite sets sensorUid on tile B
- Remove favorite: favorites list empty after remove

### test-source-profiles.js (7 assertions)
- Add profile: count +1, default name "New Source"
- Update profile: name/host/port persisted
- Set default: defaultSourceProfileId updated + persisted
- Delete profile: removed from list, count back to initial
