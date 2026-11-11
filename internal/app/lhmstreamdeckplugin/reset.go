package lhmstreamdeckplugin

import (
	lhmplugin "github.com/moeilijk/lhm-streamdeck/internal/lhm/plugin"
	"log"
	"time"
)

func (p *Plugin) resetReadingScale(context string, settings actionSettings) {
	profileID := p.resolvedSourceProfileID(settings.SourceProfileID)
	profile, ok := p.sourceProfileByID(profileID)
	if !ok {
		return
	}
	p.mu.Lock()
	if p.scaleResetBusy[context] {
		p.mu.Unlock()
		return
	}
	if p.scaleResetBusy == nil {
		p.scaleResetBusy = make(map[string]bool)
	}
	p.scaleResetBusy[context] = true
	p.mu.Unlock()
	go func() {
		defer func() { p.mu.Lock(); delete(p.scaleResetBusy, context); p.mu.Unlock() }()
		if err := lhmplugin.ResetReadingMinMax(profileEndpoint(profile), settings.SensorUID, settings.ReadingID); err != nil {
			log.Printf("Reset MIN/MAX for %s: %v", settings.ReadingLabel, err)
			return
		}
		current, err := p.am.getSettings(context)
		if err != nil || current.SensorUID != settings.SensorUID || current.ReadingID != settings.ReadingID || current.SourceProfileID != settings.SourceProfileID {
			return
		}
		p.mu.Lock()
		if p.scaleResetAfter == nil {
			p.scaleResetAfter = make(map[string]uint64)
		}
		p.scaleResetAfter[context] = uint64(time.Now().UnixNano())
		p.mu.Unlock()
		log.Printf("Reset MIN/MAX for %s", settings.ReadingLabel)
	}()
}
