package lhmstreamdeckplugin

import (
	"testing"

	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
)

// No recovery-by-label (user decision, #77 follow-up): a stale ReadingID must
// NOT be silently re-mapped to a reading with the same label. Sensor-id drift
// is an LHM/companion matter; the user re-selects explicitly.
func TestSyncSettingsDoesNotRecoverByLabel(t *testing.T) {
	readings := []hwsensorsservice.Reading{
		stubReading{id: 111, typ: "Temperature", label: "Tctl", unit: "°C"},
	}
	settings := actionSettings{
		SensorUID:    "/cpu",
		ReadingID:    999, // stale id from before the source renamed its sensors
		ReadingLabel: "Tctl",
		IsValid:      true,
	}

	changed := syncSettingsWithReadings(&settings, readings)

	if settings.ReadingID != 999 {
		t.Fatalf("ReadingID rewritten to %d via label match; silent re-mapping must not happen", settings.ReadingID)
	}
	if changed {
		t.Fatalf("settings reported as changed; stale ids must stay untouched")
	}
}

// Control: a matching id still fills in missing display metadata.
func TestSyncSettingsFillsLabelFromValidID(t *testing.T) {
	readings := []hwsensorsservice.Reading{
		stubReading{id: 111, typ: "Temperature", label: "Tctl", unit: "°C"},
	}
	settings := actionSettings{SensorUID: "/cpu", ReadingID: 111}

	if !syncSettingsWithReadings(&settings, readings) {
		t.Fatalf("expected change: empty label should be filled from the matching reading")
	}
	if settings.ReadingLabel != "Tctl" || !settings.IsValid {
		t.Fatalf("label/valid not filled: %+v", settings)
	}
}
