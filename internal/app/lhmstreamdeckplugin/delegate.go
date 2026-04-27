package lhmstreamdeckplugin

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moeilijk/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
	"github.com/moeilijk/lhm-streamdeck/pkg/streamdeck"
)

const (
	tileWidth  = 72
	tileHeight = 72
)

func boolPtr(v bool) *bool {
	return &v
}

func (p *Plugin) isSettingsAction(action, context string) bool {
	if action == "com.moeilijk.lhm.settings" {
		return true
	}
	p.mu.RLock()
	_, ok := p.settingsContexts[context]
	p.mu.RUnlock()
	return ok
}

func (p *Plugin) resolveSettingsContext(context string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.settingsContexts[context]; ok {
		return context
	}
	if len(p.settingsContexts) == 1 {
		for k := range p.settingsContexts {
			return k
		}
	}
	return context
}

func isSettingsPayload(m map[string]*json.RawMessage) bool {
	if m == nil {
		return false
	}
	for _, k := range []string{"settingsConnected", "setPollInterval", "setLhmEndpoint", "updateTileAppearance",
		"addSourceProfile", "deleteSourceProfile", "setSourceProfile", "setDefaultSourceProfile",
		"setSelectedSourceProfile", "requestSettingsStatus"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func (p *Plugin) sendSettingsStatus(action, context string, includeProfiles bool) {
	p.mu.RLock()
	currentRate := p.globalSettings.PollInterval
	profiles := make([]lhmSourceProfile, len(p.globalSettings.SourceProfiles))
	copy(profiles, p.globalSettings.SourceProfiles)
	defaultProfileID := p.globalSettings.DefaultSourceProfileID
	var selectedProfileID string
	if ts := p.settingsContexts[context]; ts != nil {
		selectedProfileID = ts.SelectedSourceProfileID
	}
	p.mu.RUnlock()

	if currentRate <= 0 {
		currentRate = int(p.am.GetInterval().Milliseconds())
	}
	if selectedProfileID == "" {
		selectedProfileID = defaultProfileID
	}

	status := "Disconnected"
	if pt, err := p.getCachedPollTimeForSource(selectedProfileID); err == nil && pt != 0 {
		status = "Connected"
	}

	statusPayload := map[string]interface{}{
		"connectionStatus": status,
		"currentRate":      currentRate,
	}
	if includeProfiles {
		statusPayload["sourceProfiles"] = profiles
		statusPayload["defaultSourceProfileId"] = defaultProfileID
		statusPayload["selectedSourceProfileId"] = selectedProfileID
	}
	if err := p.sd.SendToPropertyInspector(action, context, statusPayload); err != nil {
		log.Printf("SendToPropertyInspector settings status failed: %v\n", err)
	}
}

// OnConnected event
func (p *Plugin) OnConnected(c *websocket.Conn) {
	// Request global settings on connect
	if err := p.sd.GetGlobalSettings(); err != nil {
		log.Printf("GetGlobalSettings failed: %v\n", err)
	}
}

// OnWillAppear event
func (p *Plugin) OnWillAppear(event *streamdeck.EvWillAppear) {
	// Handle settings action separately
	if event.Action == "com.moeilijk.lhm.settings" {
		// Decode tile appearance settings from payload
		var tileSettings settingsTileSettings
		hasShowLabel := false
		hasTitle := false
		hasTitleColor := false
		hasShowTitleInGraph := false
		if event.Payload.Settings != nil {
			var rawSettings map[string]json.RawMessage
			if err := json.Unmarshal(*event.Payload.Settings, &rawSettings); err == nil {
				_, hasShowLabel = rawSettings["showLabel"]
				_, hasTitle = rawSettings["title"]
				_, hasTitleColor = rawSettings["titleColor"]
				_, hasShowTitleInGraph = rawSettings["showTitleInGraph"]
			}
			if err := json.Unmarshal(*event.Payload.Settings, &tileSettings); err != nil {
				log.Printf("OnWillAppear settings tile unmarshal: %v\n", err)
			}
		}
		p.mu.Lock()
		if existing := p.settingsContexts[event.Context]; existing != nil {
			if !hasTitle {
				tileSettings.Title = existing.Title
			}
			if !hasTitleColor {
				tileSettings.TitleColor = existing.TitleColor
			}
			if !hasShowTitleInGraph {
				tileSettings.ShowTitleInGraph = existing.ShowTitleInGraph
			}
		}
		p.mu.Unlock()
		// Set defaults if not set
		if tileSettings.TileBackground == "" {
			tileSettings.TileBackground = "#000000"
		}
		if tileSettings.TileTextColor == "" {
			tileSettings.TileTextColor = "#ffffff"
		}
		if !hasShowLabel {
			tileSettings.ShowLabel = true
		}
		if tileSettings.TitleColor == "" {
			tileSettings.TitleColor = "#b7b7b7"
		}
		if tileSettings.ShowTitleInGraph == nil {
			tileSettings.ShowTitleInGraph = boolPtr(true)
		}
		p.mu.Lock()
		p.settingsContexts[event.Context] = &tileSettings
		p.mu.Unlock()
		p.updateSettingsTile(event.Context)
		return
	}

	if event.Action == derivedAction {
		ds, _ := decodeDerivedSettings(event.Payload.Settings)
		p.mu.Lock()
		p.derivedSettings[event.Context] = &ds
		p.derivedStates[event.Context] = &derivedState{graph: initDerivedGraph(&ds)}
		p.mu.Unlock()
		return
	}

	if event.Action == compositeAction {
		cs, _ := decodeCompositeSettings(event.Payload.Settings)
		p.mu.Lock()
		p.compositeSettings[event.Context] = &cs
		p.compositeStates[event.Context] = &compositeState{graphs: initCompositeGraphs(&cs)}
		p.mu.Unlock()
		return
	}

	settings, migrated, err := decodeActionSettings(event.Payload.Settings)
	if err != nil {
		log.Println("OnWillAppear settings unmarshal", err)
	}
	tfSize := 10.5
	vfSize := 10.5
	var fgColor *color.RGBA
	var bgColor *color.RGBA
	var hlColor *color.RGBA
	var tColor *color.RGBA
	var vtColor *color.RGBA
	if settings.TitleFontSize != 0 {
		tfSize = settings.TitleFontSize
	}
	if settings.ValueFontSize != 0 {
		vfSize = settings.ValueFontSize
	}
	if settings.ForegroundColor == "" {
		fgColor = &color.RGBA{0, 81, 40, 255}
	} else {
		fgColor = hexToRGBA(settings.ForegroundColor)
	}
	if settings.BackgroundColor == "" {
		bgColor = &color.RGBA{0, 0, 0, 255}
	} else {
		bgColor = hexToRGBA(settings.BackgroundColor)
	}
	if settings.HighlightColor == "" {
		hlColor = &color.RGBA{0, 158, 0, 255}
	} else {
		hlColor = hexToRGBA(settings.HighlightColor)
	}
	if settings.TitleColor == "" {
		tColor = &color.RGBA{183, 183, 183, 255}
	} else {
		tColor = hexToRGBA(settings.TitleColor)
	}
	if settings.ValueTextColor == "" {
		vtColor = &color.RGBA{255, 255, 255, 255}
	} else {
		vtColor = hexToRGBA(settings.ValueTextColor)
	}
	drawTitle := true
	if settings.ShowTitleInGraph != nil {
		drawTitle = *settings.ShowTitleInGraph
	} else {
		settings.ShowTitleInGraph = boolPtr(drawTitle)
	}
	g := graph.NewGraph(tileWidth, tileHeight, settings.Min, settings.Max, fgColor, bgColor, hlColor)
	g.SetLabel(0, "", 19, tColor)
	g.SetLabelFontSize(0, tfSize)
	g.SetLabel(1, "", 44, vtColor)
	g.SetLabelFontSize(1, vfSize)
	g.SetLabel(2, "", 56, vtColor)
	g.SetLabelFontSize(2, vfSize)
	if settings.GraphHeightPct > 0 {
		g.SetHeightPct(settings.GraphHeightPct)
	}
	if settings.GraphLineThickness > 0 {
		g.SetLineThickness(settings.GraphLineThickness)
	}
	g.SetTextStroke(settings.TextStroke)
	if settings.TextStrokeColor != "" {
		g.SetTextStrokeColor(hexToRGBA(settings.TextStrokeColor))
	}
	if drawTitle {
		g.SetLabelText(0, settings.Title)
	}
	p.mu.Lock()
	p.graphs[event.Context] = g
	p.mu.Unlock()
	p.resetThresholdRuntimeState(event.Context, "")

	// Reset threshold state so updateTiles will re-evaluate and apply correct colors on first run
	if settings.CurrentThresholdID != "" {
		settings.CurrentThresholdID = ""
		migrated = true
	}
	// Legacy: reset old alert state
	if settings.CurrentAlertState != "" {
		settings.CurrentAlertState = ""
		migrated = true
	}

	// Ensure all enabled thresholds have operators
	for i := range settings.Thresholds {
		if settings.Thresholds[i].Enabled && settings.Thresholds[i].Operator == "" {
			settings.Thresholds[i].Operator = ">="
			migrated = true
		}
	}

	// Legacy: Set default operators for old Warning/Critical if still present
	if settings.WarningEnabled && settings.WarningOperator == "" {
		settings.WarningOperator = ">="
		migrated = true
	}
	if settings.CriticalEnabled && settings.CriticalOperator == "" {
		settings.CriticalOperator = ">="
		migrated = true
	}

	p.am.SetAction(event.Action, event.Context, &settings)
	if migrated {
		_ = p.sd.SetSettings(event.Context, &settings)
	}
}

// OnWillDisappear event
func (p *Plugin) OnWillDisappear(event *streamdeck.EvWillDisappear) {
	// Handle settings action
	if event.Action == "com.moeilijk.lhm.settings" {
		p.mu.Lock()
		delete(p.settingsContexts, event.Context)
		p.mu.Unlock()
		return
	}

	if event.Action == derivedAction {
		p.mu.Lock()
		delete(p.derivedSettings, event.Context)
		delete(p.derivedStates, event.Context)
		p.mu.Unlock()
		return
	}

	if event.Action == compositeAction {
		p.mu.Lock()
		delete(p.compositeSettings, event.Context)
		delete(p.compositeStates, event.Context)
		p.mu.Unlock()
		for i := 0; i < 4; i++ {
			p.clearThresholdRuntimeState(event.Context + "|" + strconv.Itoa(i))
		}
		return
	}

	_, _, err := decodeActionSettings(event.Payload.Settings)
	if err != nil {
		log.Println("OnWillDisappear settings unmarshal", err)
	}
	p.mu.Lock()
	delete(p.graphs, event.Context)
	delete(p.divisorCache, event.Context)
	p.mu.Unlock()
	p.clearThresholdRuntimeState(event.Context)
	p.am.RemoveAction(event.Context)
}

// OnKeyDown snoozes or resumes active threshold alerts for reading tiles.
func (p *Plugin) OnKeyDown(event *streamdeck.EvKeyDown) {
	if event.Action != "com.moeilijk.lhm.reading" {
		return
	}

	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		log.Printf("OnKeyDown getSettings: %v\n", err)
		return
	}

	if settings.CurrentThresholdID == "" {
		return
	}

	if configured := normalizeThresholdSnoozeDurations(settings.SnoozeDurations); len(configured) > 0 {
		now := time.Now()
		currentSnooze, snoozed := p.currentThresholdSnooze(event.Context, now)
		var current *thresholdSnoozeState
		if snoozed {
			current = &currentSnooze
		}

		if nextDuration, ok := nextThresholdSnoozeDuration(configured, current); ok {
			p.setThresholdSnooze(event.Context, nextDuration, now)
		} else if !p.clearThresholdSnooze(event.Context) {
			return
		}

		p.refreshAction(event.Action, event.Context)
		return
	}

	if !p.clearStickyThreshold(event.Context, settings.CurrentThresholdID) {
		return
	}

	settings.CurrentThresholdID = ""
	if err := p.sd.SetSettings(event.Context, &settings); err != nil {
		log.Printf("OnKeyDown SetSettings: %v\n", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	p.refreshAction(event.Action, event.Context)
}

// OnApplicationDidLaunch event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidLaunch(event *streamdeck.EvApplication) {}

// OnApplicationDidTerminate event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidTerminate(event *streamdeck.EvApplication) {}

// OnTitleParametersDidChange event
func (p *Plugin) OnTitleParametersDidChange(event *streamdeck.EvTitleParametersDidChange) {
	if p.isSettingsAction(event.Action, event.Context) {
		targetContext := p.resolveSettingsContext(event.Context)
		p.mu.RLock()
		existing := p.settingsContexts[targetContext]
		p.mu.RUnlock()
		var tileSettings *settingsTileSettings
		if existing == nil {
			tileSettings = &settingsTileSettings{
				TileBackground: "#000000",
				TileTextColor:  "#ffffff",
				ShowLabel:      true,
				Title:          "",
				TitleColor:     "#b7b7b7",
			}
		} else {
			cp := *existing
			tileSettings = &cp
		}

		tileSettings.Title = event.Payload.Title
		if event.Payload.TitleParameters.TitleColor != "" {
			tileSettings.TitleColor = event.Payload.TitleParameters.TitleColor
		} else if tileSettings.TitleColor == "" {
			tileSettings.TitleColor = "#b7b7b7"
		}
		drawTitle := !event.Payload.TitleParameters.ShowTitle
		tileSettings.ShowTitleInGraph = boolPtr(drawTitle)
		p.mu.Lock()
		p.settingsContexts[targetContext] = tileSettings
		p.mu.Unlock()
		if err := p.sd.SetSettings(targetContext, tileSettings); err != nil {
			log.Printf("OnTitleParametersDidChange settings SetSettings: %v\n", err)
		}
		p.updateSettingsTile(targetContext)
		return
	}

	if event.Action == derivedAction {
		p.handleDerivedTitleParametersDidChange(event)
		return
	}

	if event.Action == compositeAction {
		return // composite tile gebruikt geen SD-native titel
	}

	// Get existing settings from actionManager to preserve threshold settings
	// Do NOT decode from event payload as it may have stale/different settings
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		// If no settings in actionManager yet, decode from payload as fallback
		settings, _, err = decodeActionSettings(event.Payload.Settings)
		if err != nil {
			log.Println("OnTitleParametersDidChange settings unmarshal", err)
		}
	}
	p.mu.RLock()
	g, ok := p.graphs[event.Context]
	p.mu.RUnlock()
	if !ok {
		log.Printf("OnTitleParametersDidChange no graph for context: %s\n", event.Context)
		return
	}
	drawTitle := !event.Payload.TitleParameters.ShowTitle
	if drawTitle {
		g.SetLabelText(0, event.Payload.Title)
		if event.Payload.TitleParameters.TitleColor != "" {
			tClr := hexToRGBA(event.Payload.TitleParameters.TitleColor)
			g.SetLabelColor(0, tClr)
		}
	} else {
		g.SetLabelText(0, "")
	}
	// Only update title-related fields, preserving threshold settings
	settings.Title = event.Payload.Title
	settings.TitleColor = event.Payload.TitleParameters.TitleColor
	settings.ShowTitleInGraph = boolPtr(drawTitle)

	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		log.Printf("OnTitleParametersDidChange SetSettings: %v\n", err)
		return
	}
	p.am.SetAction(event.Action, event.Context, &settings)
}

// OnPropertyInspectorConnected event
func (p *Plugin) OnPropertyInspectorConnected(event *streamdeck.EvSendToPlugin) {
	if p.isSettingsAction(event.Action, event.Context) {
		p.sendSettingsStatus("com.moeilijk.lhm.settings", event.Context, true)
		return
	}

	if event.Action == derivedAction {
		p.handleDerivedPropertyInspectorConnected(event)
		return
	}

	if event.Action == compositeAction {
		p.handleCompositePropertyInspectorConnected(event)
		return
	}

	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		log.Println("OnPropertyInspectorConnected getSettings", err)
	}
	profileID := p.resolvedSourceProfileID(settings.SourceProfileID)
	sensors, err := p.sensorsWithTimeoutForSource(profileID, 2*time.Second)
	if err != nil {
		log.Println("OnPropertyInspectorConnected Sensors", err)
		go p.restartSource(p.runtimeForSource(profileID))
		payload := evStatus{Error: true, Message: "Libre Hardware Monitor Unavailable"}
		if err := p.sd.SendToPropertyInspector(event.Action, event.Context, payload); err != nil {
			log.Printf("OnPropertyInspectorConnected SendToPropertyInspector: %v\n", err)
		}
		settings.InErrorState = true
		if err := p.sd.SetSettings(event.Context, &settings); err != nil {
			log.Printf("OnPropertyInspectorConnected SetSettings: %v\n", err)
			return
		}
		p.am.SetAction(event.Action, event.Context, &settings)
		return
	}
	evsensors := make([]*evSendSensorsPayloadSensor, 0, len(sensors))
	for _, s := range sensors {
		evsensors = append(evsensors, sensorPayload(s.ID(), s.Name()))
	}
	payload := evSendSensorsPayload{Sensors: evsensors, Settings: &settings}
	err = p.sd.SendToPropertyInspector(event.Action, event.Context, payload)
	if err != nil {
		log.Println("OnPropertyInspectorConnected SendToPropertyInspector", err)
	}
	if err := p.sendCatalogToPropertyInspector(event.Action, event.Context, &settings, sensors); err != nil {
		log.Printf("OnPropertyInspectorConnected sendCatalogToPropertyInspector: %v\n", err)
	}
	if settings.SensorUID != "" {
		readings, rerr := p.sendReadingsToPropertyInspector(event.Action, event.Context, settings.SensorUID, &settings)
		if rerr == nil {
			if syncSettingsWithReadings(&settings, readings) {
				_ = p.sd.SetSettings(event.Context, &settings)
				p.am.SetAction(event.Action, event.Context, &settings)
			}
		}
	}
}

func (p *Plugin) sendReadingsToPropertyInspector(action, context, sensorID string, settings *actionSettings) ([]hwsensorsservice.Reading, error) {
	profileID := p.resolvedSourceProfileID(settings.SourceProfileID)
	rt := p.runtimeForSource(profileID)
	rt.mu.RLock()
	hw := rt.hw
	rt.mu.RUnlock()
	if hw == nil {
		return nil, fmt.Errorf("LHM bridge not ready")
	}
	readings, err := hw.ReadingsForSensorID(sensorID)
	if err != nil {
		log.Println("sendReadingsToPropertyInspector ReadingsForSensorID", err)
		return nil, err
	}
	evreadings := make([]*evSendReadingsPayloadReading, 0, len(readings))
	for _, r := range readings {
		evreadings = append(evreadings, &evSendReadingsPayloadReading{
			ID:     r.ID(),
			Label:  r.Label(),
			Prefix: r.Unit(),
			Unit:   r.Unit(),
			Type:   r.Type(),
		})
	}
	payload := evSendReadingsPayload{Readings: evreadings, Settings: settings}
	err = p.sd.SendToPropertyInspector(action, context, payload)
	if err != nil {
		log.Println("sendReadingsToPropertyInspector SendToPropertyInspector", err)
	}
	return readings, nil
}

// OnSendToPlugin event
func (p *Plugin) OnSendToPlugin(event *streamdeck.EvSendToPlugin) {
	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*event.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
	}

	// Handle settings action commands
	if p.isSettingsAction(event.Action, event.Context) || isSettingsPayload(payload) {
		targetContext := p.resolveSettingsContext(event.Context)
		// Check for settingsConnected
		if _, ok := payload["settingsConnected"]; ok {
			p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			return
		}

		// Check for periodic settings status refresh
		if _, ok := payload["requestSettingsStatus"]; ok {
			p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, false)
			return
		}

		// Check for setPollInterval
		if raw, ok := payload["setPollInterval"]; ok {
			var intervalMs int
			if err := json.Unmarshal(*raw, &intervalMs); err == nil && intervalMs > 0 {
				p.setPollInterval(intervalMs)
			}
			return
		}

		// Check for setSelectedSourceProfile (which profile this settings tile monitors)
		if raw, ok := payload["setSelectedSourceProfile"]; ok {
			var profileID string
			if err := json.Unmarshal(*raw, &profileID); err == nil {
				p.mu.Lock()
				if ts := p.settingsContexts[targetContext]; ts != nil {
					ts.SelectedSourceProfileID = profileID
					if err2 := p.sd.SetSettings(targetContext, ts); err2 != nil {
						log.Printf("setSelectedSourceProfile SetSettings: %v\n", err2)
					}
				}
				p.mu.Unlock()
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			}
			return
		}

		// Check for addSourceProfile
		if _, ok := payload["addSourceProfile"]; ok {
			p.mu.Lock()
			id := fmt.Sprintf("source_%d", time.Now().UnixNano())
			newProfile := lhmSourceProfile{ID: id, Name: "New Source", Host: "127.0.0.1", Port: 8085}
			p.globalSettings.SourceProfiles = append(p.globalSettings.SourceProfiles, newProfile)
			gs := p.globalSettings
			p.mu.Unlock()
			if err := p.sd.SetGlobalSettings(gs); err != nil {
				log.Printf("addSourceProfile SetGlobalSettings: %v\n", err)
			}
			p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			return
		}

		// Check for deleteSourceProfile
		if raw, ok := payload["deleteSourceProfile"]; ok {
			var profileID string
			if err := json.Unmarshal(*raw, &profileID); err == nil {
				p.mu.Lock()
				profiles := p.globalSettings.SourceProfiles
				for i, sp := range profiles {
					if sp.ID == profileID && sp.ID != p.globalSettings.DefaultSourceProfileID {
						p.globalSettings.SourceProfiles = append(profiles[:i], profiles[i+1:]...)
						break
					}
				}
				gs := p.globalSettings
				p.mu.Unlock()
				// Kill runtime for deleted profile
				p.sourceMu.Lock()
				if rt, exists := p.sources[profileID]; exists {
					rt.mu.Lock()
					if rt.c != nil {
						rt.c.Kill()
					}
					rt.mu.Unlock()
					delete(p.sources, profileID)
				}
				p.sourceMu.Unlock()
				if err := p.sd.SetGlobalSettings(gs); err != nil {
					log.Printf("deleteSourceProfile SetGlobalSettings: %v\n", err)
				}
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			}
			return
		}

		// Check for setSourceProfile (update name/host/port of a profile)
		if raw, ok := payload["setSourceProfile"]; ok {
			var sp lhmSourceProfile
			if err := json.Unmarshal(*raw, &sp); err == nil {
				p.mu.Lock()
				changed := false
				for i := range p.globalSettings.SourceProfiles {
					if p.globalSettings.SourceProfiles[i].ID == sp.ID {
						old := p.globalSettings.SourceProfiles[i]
						p.globalSettings.SourceProfiles[i].Name = sp.Name
						p.globalSettings.SourceProfiles[i].Host = sp.Host
						p.globalSettings.SourceProfiles[i].Port = sp.Port
						changed = old.Host != sp.Host || old.Port != sp.Port
						break
					}
				}
				gs := p.globalSettings
				p.mu.Unlock()
				if err := p.sd.SetGlobalSettings(gs); err != nil {
					log.Printf("setSourceProfile SetGlobalSettings: %v\n", err)
				}
				if changed {
					p.sourceMu.RLock()
					rt := p.sources[sp.ID]
					p.sourceMu.RUnlock()
					if rt != nil {
						rt.mu.Lock()
						rt.profile.Host = sp.Host
						rt.profile.Port = sp.Port
						if rt.c != nil {
							rt.c.Kill()
						}
						rt.c = nil
						rt.hw = nil
						rt.mu.Unlock()
						p.mu.Lock()
						invalidatePollCacheForRuntime(rt)
						p.mu.Unlock()
						go p.startSourceClient(rt)
					}
				}
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			}
			return
		}

		// Check for setDefaultSourceProfile
		if raw, ok := payload["setDefaultSourceProfile"]; ok {
			var profileID string
			if err := json.Unmarshal(*raw, &profileID); err == nil {
				p.mu.Lock()
				for _, sp := range p.globalSettings.SourceProfiles {
					if sp.ID == profileID {
						p.globalSettings.DefaultSourceProfileID = profileID
						break
					}
				}
				gs := p.globalSettings
				p.mu.Unlock()
				if err := p.sd.SetGlobalSettings(gs); err != nil {
					log.Printf("setDefaultSourceProfile SetGlobalSettings: %v\n", err)
				}
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			}
			return
		}

		// Check for setLhmEndpoint
		if raw, ok := payload["setLhmEndpoint"]; ok {
			var ep struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			}
			if err := json.Unmarshal(*raw, &ep); err == nil {
				p.setLhmEndpoint(ep.Host, ep.Port)
			}
			return
		}

		// Check for updateTileAppearance
		if raw, ok := payload["updateTileAppearance"]; ok {
			var appearance settingsTileSettings
			if err := json.Unmarshal(*raw, &appearance); err == nil {
				// Update stored settings
				if appearance.TileBackground == "" {
					appearance.TileBackground = "#000000"
				}
				if appearance.TileTextColor == "" {
					appearance.TileTextColor = "#ffffff"
				}
				p.mu.RLock()
				existing := p.settingsContexts[targetContext]
				p.mu.RUnlock()
				if existing != nil {
					if appearance.Title == "" {
						appearance.Title = existing.Title
					}
					if appearance.TitleColor == "" {
						appearance.TitleColor = existing.TitleColor
					}
					if appearance.ShowTitleInGraph == nil {
						appearance.ShowTitleInGraph = existing.ShowTitleInGraph
					}
				}
				if appearance.TitleColor == "" {
					appearance.TitleColor = "#b7b7b7"
				}
				if appearance.ShowTitleInGraph == nil {
					appearance.ShowTitleInGraph = boolPtr(true)
				}
				p.mu.Lock()
				p.settingsContexts[targetContext] = &appearance
				p.mu.Unlock()
				if err := p.sd.SetSettings(targetContext, &appearance); err != nil {
					log.Printf("updateTileAppearance SetSettings failed: %v\n", err)
				}
				p.updateSettingsTile(targetContext)
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext, true)
			}
			return
		}
	}

	// Handle source profile selection for reading, composite, and derived tiles.
	if raw, ok := payload["sourceProfileId"]; ok {
		var profileID string
		if err := json.Unmarshal(*raw, &profileID); err == nil {
			if event.Action == derivedAction {
				p.mu.Lock()
				if ds, exists := p.derivedSettings[event.Context]; exists {
					ds.SourceProfileID = profileID
					_ = p.sd.SetSettings(event.Context, ds)
				}
				p.mu.Unlock()
			} else if event.Action == compositeAction {
				p.mu.Lock()
				if cs, exists := p.compositeSettings[event.Context]; exists {
					cs.SourceProfileID = profileID
					_ = p.sd.SetSettings(event.Context, cs)
				}
				p.mu.Unlock()
			} else {
				settings, err2 := p.am.getSettings(event.Context)
				if err2 == nil {
					settings.SourceProfileID = profileID
					_ = p.sd.SetSettings(event.Context, &settings)
					p.am.SetAction(event.Action, event.Context, &settings)
				}
			}
		}
		return
	}

	if event.Action == derivedAction {
		if data, ok := payload["loadDerivedPreset"]; ok {
			var preset derivedPresetPayload
			if err := json.Unmarshal(*data, &preset); err != nil {
				log.Printf("derived loadDerivedPreset unmarshal: %v", err)
				return
			}
			p.handleDerivedLoadPreset(event, preset)
			return
		}
		if data, ok := payload["sdpi_collection"]; ok {
			sdpi := evSdpiCollection{}
			if err := json.Unmarshal(*data, &sdpi); err != nil {
				log.Printf("derived sdpi unmarshal: %v", err)
				return
			}
			switch sdpi.Key {
			case "derived_formula", "derived_slotCount", "derived_format", "derived_divisor",
				"derived_graphUnit", "derived_min", "derived_max",
				"derived_foregroundColor", "derived_backgroundColor", "derived_highlightColor",
				"derived_valueTextColor", "derived_titleColor", "derived_title",
				"derived_graphHeightPct", "derived_graphLineThickness", "derived_textStroke", "derived_textStrokeColor",
				"derived_updateIntervalOverrideMs", "derived_smoothingAlpha", "titleFontSize", "valueFontSize":
				p.handleDerivedGlobalField(event, &sdpi)
			case "allSlots_sensorSelect":
				p.handleDerivedAllSlotsSensor(event, &sdpi)
			default:
				slotIdx, field := parseCompositeSlotKey(sdpi.Key) // reuse — zelfde slot{N}_{field} patroon
				if slotIdx < 0 {
					log.Printf("derived unknown sdpi key: %s", sdpi.Key)
					return
				}
				switch field {
				case "sensorSelect":
					p.handleDerivedSlotSensorSelect(event, &sdpi, slotIdx)
				case "readingSelect":
					p.handleDerivedSlotReadingSelect(event, &sdpi, slotIdx)
				case "applyFavorite":
					p.handleDerivedSlotApplyFavorite(event, &sdpi, slotIdx)
				default:
					p.handleDerivedSlotField(event, &sdpi, slotIdx, field)
				}
			}
		}
		return
	}

	if event.Action == compositeAction {
		if data, ok := payload["sdpi_collection"]; ok {
			sdpi := evSdpiCollection{}
			if err := json.Unmarshal(*data, &sdpi); err != nil {
				log.Printf("composite sdpi unmarshal: %v", err)
				return
			}
			switch sdpi.Key {
			case "composite_mode", "composite_slotCount", "updateIntervalOverrideMs", "smoothingAlpha":
				p.handleCompositeGlobalField(event, &sdpi)
			default:
				slotIdx, field := parseCompositeSlotKey(sdpi.Key)
				if slotIdx < 0 {
					log.Printf("composite unknown sdpi key: %s", sdpi.Key)
					return
				}
				switch field {
				case "sensorSelect":
					p.handleCompositeSlotSensorSelect(event, &sdpi, slotIdx)
				case "readingSelect":
					p.handleCompositeSlotReadingSelect(event, &sdpi, slotIdx)
				case "addThreshold":
					p.handleCompositeAddThreshold(event, &sdpi, slotIdx)
				case "removeThreshold":
					p.handleCompositeRemoveThreshold(event, &sdpi, slotIdx)
				case "reorderThreshold":
					p.handleCompositeReorderThreshold(event, &sdpi, slotIdx)
				case "thresholdEnabled", "thresholdName",
					"thresholdOperator", "thresholdValue", "thresholdHysteresis", "thresholdDwellMs",
					"thresholdCooldownMs", "thresholdSticky", "thresholdText", "thresholdTextColor",
					"thresholdBackgroundColor", "thresholdForegroundColor",
					"thresholdHighlightColor", "thresholdValueTextColor":
					p.handleCompositeThresholdUpdate(event, &sdpi, slotIdx)
				default:
					p.handleCompositeSlotField(event, &sdpi, slotIdx, field)
				}
			}
		}
		return
	}

	if data, ok := payload["sdpi_collection"]; ok {
		sdpi := evSdpiCollection{}
		err = json.Unmarshal(*data, &sdpi)
		if err != nil {
			log.Println("SDPI unmarshal", err)
		}
		switch sdpi.Key {
		case "sensorSelect":
			err = p.handleSensorSelect(event, &sdpi)
			if err != nil {
				log.Println("handleSensorSelect", err)
			}
		case "toggleFavoriteCurrent":
			settings, getErr := p.am.getSettings(event.Context)
			if getErr != nil {
				log.Println("toggleFavoriteCurrent getSettings", getErr)
				break
			}
			err = p.toggleFavoriteSelection(event.Action, event.Context, &settings)
			if err != nil {
				log.Println("toggleFavoriteCurrent", err)
			}
		case "applyFavorite":
			err = p.handleApplyFavorite(event, &sdpi)
			if err != nil {
				log.Println("handleApplyFavorite", err)
			}
		case "removeFavorite":
			settings, getErr := p.am.getSettings(event.Context)
			if getErr != nil {
				log.Println("removeFavorite getSettings", getErr)
				break
			}
			err = p.removeFavorite(event.Action, event.Context, &settings, sdpi.Value)
			if err != nil {
				log.Println("removeFavorite", err)
			}
		case "readingSelect":
			err = p.handleReadingSelect(event, &sdpi)
			if err != nil {
				log.Println("handleReadingSelect", err)
			}
		case "min":
			err := p.handleSetMin(event, &sdpi)
			if err != nil {
				log.Println("handleSetMin", err)
			}
		case "max":
			err := p.handleSetMax(event, &sdpi)
			if err != nil {
				log.Println("handleSetMax", err)
			}
		case "format":
			err := p.handleSetFormat(event, &sdpi)
			if err != nil {
				log.Println("handleSetFormat", err)
			}
		case "divisor":
			err := p.handleDivisor(event, &sdpi)
			if err != nil {
				log.Println("handleDivisor", err)
			}
		case "graphUnit":
			err := p.handleSetGraphUnit(event, &sdpi)
			if err != nil {
				log.Println("handleSetGraphUnit", err)
			}
		case "snoozeDurations":
			err := p.handleSnoozeDurations(event, &sdpi)
			if err != nil {
				log.Println("handleSnoozeDurations", err)
			}
		case "graphMode":
			settings, getErr := p.am.getSettings(event.Context)
			if getErr != nil {
				log.Println("graphMode getSettings", getErr)
				break
			}
			settings.GraphMode = sdpi.Value
			if err2 := p.sd.SetSettings(event.Context, &settings); err2 != nil {
				log.Println("graphMode SetSettings", err2)
				break
			}
			p.am.SetAction(event.Action, event.Context, &settings)
		case "foreground", "background", "highlight", "valuetext":
			err := p.handleColorChange(event, sdpi.Key, &sdpi)
			if err != nil {
				log.Println("handleColorChange", err)
			}
		case "titleFontSize", "valueFontSize":
			err := p.handleSetFontSize(event, sdpi.Key, &sdpi)
			if err != nil {
				log.Println("handleSetTitleFontSize", err)
			}
		case "graphHeightPct", "graphLineThickness", "textStroke", "textStrokeColor", "updateIntervalOverrideMs", "smoothingAlpha":
			err := p.handleGraphVisuals(event, &sdpi)
			if err != nil {
				log.Println("handleGraphVisuals", err)
			}
		case "warningEnabled":
			err := p.handleWarningEnabled(event, &sdpi)
			if err != nil {
				log.Println("handleWarningEnabled", err)
			}
		case "criticalEnabled":
			err := p.handleCriticalEnabled(event, &sdpi)
			if err != nil {
				log.Println("handleCriticalEnabled", err)
			}
		case "warningValue":
			err := p.handleWarningValue(event, &sdpi)
			if err != nil {
				log.Println("handleWarningValue", err)
			}
		case "criticalValue":
			err := p.handleCriticalValue(event, &sdpi)
			if err != nil {
				log.Println("handleCriticalValue", err)
			}
		case "warningOperator":
			err := p.handleWarningOperator(event, &sdpi)
			if err != nil {
				log.Println("handleWarningOperator", err)
			}
		case "criticalOperator":
			err := p.handleCriticalOperator(event, &sdpi)
			if err != nil {
				log.Println("handleCriticalOperator", err)
			}
		case "warningBackground", "warningForeground", "warningHighlight", "warningValuetext",
			"criticalBackground", "criticalForeground", "criticalHighlight", "criticalValuetext":
			err := p.handleColorChange(event, sdpi.Key, &sdpi)
			if err != nil {
				log.Println("handleColorChange (threshold)", err)
			}
		// Dynamic threshold handlers
		case "addThreshold":
			err := p.handleAddThreshold(event, &sdpi)
			if err != nil {
				log.Println("handleAddThreshold", err)
			}
		case "removeThreshold":
			err := p.handleRemoveThreshold(event, &sdpi)
			if err != nil {
				log.Println("handleRemoveThreshold", err)
			}
		case "reorderThreshold":
			err := p.handleReorderThreshold(event, &sdpi)
			if err != nil {
				log.Println("handleReorderThreshold", err)
			}
		case "thresholdEnabled", "thresholdName",
			"thresholdOperator", "thresholdValue", "thresholdHysteresis", "thresholdDwellMs",
			"thresholdCooldownMs", "thresholdSticky", "thresholdText", "thresholdTextColor",
			"thresholdBackgroundColor", "thresholdForegroundColor",
			"thresholdHighlightColor", "thresholdValueTextColor":
			err := p.handleThresholdUpdate(event, &sdpi)
			if err != nil {
				log.Println("handleThresholdUpdate", err)
			}
		default:
			log.Printf("Unknown sdpi key: %s\n", sdpi.Key)
		}
		return
	}
}

// OnDidReceiveSettings handles action settings updates persisted by Stream Deck.
func (p *Plugin) OnDidReceiveSettings(event *streamdeck.EvDidReceiveSettings) {
	if !p.isSettingsAction(event.Action, event.Context) {
		return
	}
	if event.Payload.Settings == nil {
		return
	}

	var rawSettings map[string]json.RawMessage
	hasShowLabel := false
	hasTitle := false
	hasTitleColor := false
	hasShowTitleInGraph := false
	if err := json.Unmarshal(*event.Payload.Settings, &rawSettings); err == nil {
		_, hasShowLabel = rawSettings["showLabel"]
		_, hasTitle = rawSettings["title"]
		_, hasTitleColor = rawSettings["titleColor"]
		_, hasShowTitleInGraph = rawSettings["showTitleInGraph"]
	}

	var tileSettings settingsTileSettings
	if err := json.Unmarshal(*event.Payload.Settings, &tileSettings); err != nil {
		log.Printf("OnDidReceiveSettings settings tile unmarshal: %v\n", err)
		return
	}

	if tileSettings.TileBackground == "" {
		tileSettings.TileBackground = "#000000"
	}
	if tileSettings.TileTextColor == "" {
		tileSettings.TileTextColor = "#ffffff"
	}
	if !hasShowLabel {
		tileSettings.ShowLabel = true
	}
	p.mu.RLock()
	existing := p.settingsContexts[event.Context]
	p.mu.RUnlock()
	if existing != nil {
		if !hasTitle {
			tileSettings.Title = existing.Title
		}
		if !hasTitleColor {
			tileSettings.TitleColor = existing.TitleColor
		}
		if !hasShowTitleInGraph {
			tileSettings.ShowTitleInGraph = existing.ShowTitleInGraph
		}
	}
	if tileSettings.TitleColor == "" {
		tileSettings.TitleColor = "#b7b7b7"
	}
	if tileSettings.ShowTitleInGraph == nil {
		tileSettings.ShowTitleInGraph = boolPtr(true)
	}

	p.mu.Lock()
	p.settingsContexts[event.Context] = &tileSettings
	p.mu.Unlock()
	// Repair partial payloads (e.g. PI sending only color/showLabel) so title fields persist.
	if !hasShowLabel || !hasTitle || !hasTitleColor || !hasShowTitleInGraph {
		if err := p.sd.SetSettings(event.Context, &tileSettings); err != nil {
			log.Printf("OnDidReceiveSettings repair SetSettings failed: %v\n", err)
		}
	}
	p.updateSettingsTile(event.Context)
	p.sendSettingsStatus("com.moeilijk.lhm.settings", event.Context, true)
}

// OnDidReceiveGlobalSettings handles global settings from Stream Deck
func (p *Plugin) OnDidReceiveGlobalSettings(event *streamdeck.EvDidReceiveGlobalSettings) {
	if event.Payload.Settings == nil {
		return
	}

	var gs globalSettings
	if err := json.Unmarshal(*event.Payload.Settings, &gs); err != nil {
		log.Printf("OnDidReceiveGlobalSettings unmarshal failed: %v\n", err)
		return
	}

	if gs.PollInterval <= 0 {
		gs.PollInterval = int(defaultPollInterval.Milliseconds())
	}
	if gs.PollInterval < 250 {
		gs.PollInterval = 250
	}
	if gs.PollInterval > 10000 {
		gs.PollInterval = 10000
	}

	p.mu.Lock()
	intervalChanged := gs.PollInterval != p.globalSettings.PollInterval

	// Compute which profiles changed endpoint before updating globalSettings.
	type profileDelta struct {
		rt      *sourceRuntime
		profile lhmSourceProfile
	}
	var endpointChanges []profileDelta
	for _, newProf := range gs.SourceProfiles {
		for _, oldProf := range p.globalSettings.SourceProfiles {
			if oldProf.ID == newProf.ID {
				if oldProf.Host != newProf.Host || oldProf.Port != newProf.Port {
					p.sourceMu.RLock()
					rt := p.sources[newProf.ID]
					p.sourceMu.RUnlock()
					if rt != nil {
						endpointChanges = append(endpointChanges, profileDelta{rt: rt, profile: newProf})
					}
				}
				break
			}
		}
	}

	p.globalSettings = gs
	migrated := p.migrateSourceProfiles()
	if intervalChanged {
		p.pollTimeCacheTTL = pollTimeCacheTTLForInterval(time.Duration(gs.PollInterval) * time.Millisecond)
	}
	for _, d := range endpointChanges {
		invalidatePollCacheForRuntime(d.rt)
	}
	p.mu.Unlock()

	if migrated {
		if err := p.sd.SetGlobalSettings(p.globalSettings); err != nil {
			log.Printf("SetGlobalSettings migration persist failed: %v\n", err)
		}
	}

	if intervalChanged {
		interval := time.Duration(gs.PollInterval) * time.Millisecond
		p.am.SetInterval(interval)
	}

	// Restart bridges whose endpoint changed.
	for _, d := range endpointChanges {
		rt := d.rt
		rt.mu.Lock()
		rt.profile = d.profile
		if rt.c != nil {
			rt.c.Kill()
		}
		rt.c = nil
		rt.hw = nil
		rt.mu.Unlock()
		log.Printf("LHM endpoint changed for source %s, restarting bridge\n", d.profile.ID)
		go p.startSourceClient(rt)
	}

	// Ensure a runtime exists and is started for each profile.
	p.mu.RLock()
	profiles := make([]lhmSourceProfile, len(p.globalSettings.SourceProfiles))
	copy(profiles, p.globalSettings.SourceProfiles)
	p.mu.RUnlock()

	for _, prof := range profiles {
		rt := p.runtimeForSource(prof.ID)
		rt.mu.RLock()
		needsStart := rt.c == nil
		rt.mu.RUnlock()
		if needsStart {
			go func(r *sourceRuntime) {
				if err := p.startSourceClient(r); err != nil {
					log.Printf("startSourceClient %s failed: %v\n", r.profile.ID, err)
				}
			}(rt)
		}
	}

	p.updateAllSettingsTiles()
}
