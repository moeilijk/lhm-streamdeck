package lhmstreamdeckplugin

import (
	"fmt"

	lhmplugin "github.com/moeilijk/lhm-streamdeck/internal/lhm/plugin"
)

// The product-specific side of "Reset on press" (see reset.go).
//
// Libre Hardware Monitor resets a sensor's MIN/MAX on a POST to its /Sensor
// endpoint, and lhm-companion answers the same request. The plugin sends it
// to the endpoint of the tile's source profile.

// sourceResetSupported reports whether the sensor source can reset MIN/MAX.
func (p *Plugin) sourceResetSupported() bool {
	return true
}

// resetSourceMinMax resets the MIN/MAX of the reading behind settings at the
// sensor source.
func (p *Plugin) resetSourceMinMax(settings actionSettings) error {
	profileID := p.resolvedSourceProfileID(settings.SourceProfileID)
	profile, ok := p.sourceProfileByID(profileID)
	if !ok {
		return fmt.Errorf("source profile %q not found", profileID)
	}
	return lhmplugin.ResetReadingMinMax(profileEndpoint(profile), settings.SensorUID, settings.ReadingID)
}

// sameResetTarget reports whether two settings point at the same reading of
// the same source. The source is the tile's source profile.
func sameResetTarget(a, b *actionSettings) bool {
	return a.SensorUID == b.SensorUID && a.ReadingID == b.ReadingID && a.SourceProfileID == b.SourceProfileID
}
