package lhmstreamdeckplugin

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shayne/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
	"github.com/shayne/lhm-streamdeck/pkg/streamdeck"
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
	_, ok := p.settingsContexts[context]
	return ok
}

func (p *Plugin) resolveSettingsContext(context string) string {
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

func payloadKeys(m map[string]*json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isSettingsPayload(m map[string]*json.RawMessage) bool {
	if m == nil {
		return false
	}
	for _, k := range []string{"settingsConnected", "setPollInterval", "updateTileAppearance"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func (p *Plugin) sendSettingsStatus(action, context string) {
	status := "Disconnected"
	if p.hw != nil {
		if _, err := p.hw.PollTime(); err == nil {
			status = "Connected"
		}
	}
	currentRate := p.globalSettings.PollInterval
	if currentRate <= 0 {
		currentRate = int(p.am.GetInterval().Milliseconds())
	}
	statusPayload := map[string]interface{}{
		"connectionStatus": status,
		"currentRate":      currentRate,
	}
	if err := p.sd.SendToPropertyInspector(action, context, statusPayload); err != nil {
		log.Printf("SendToPropertyInspector settings status failed: %v\n", err)
	}
}

// OnConnected event
func (p *Plugin) OnConnected(c *websocket.Conn) {
	log.Println("OnConnected")
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
		log.Printf(
			"OnWillAppear settings context=%s bg=%s text=%s showLabel=%t hasShowLabel=%t\n",
			event.Context,
			tileSettings.TileBackground,
			tileSettings.TileTextColor,
			tileSettings.ShowLabel,
			hasShowLabel,
		)
		p.settingsContexts[event.Context] = &tileSettings
		p.updateSettingsTile(event.Context)
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
	if drawTitle {
		g.SetLabelText(0, settings.Title)
	}
	p.graphs[event.Context] = g

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
		delete(p.settingsContexts, event.Context)
		return
	}

	_, _, err := decodeActionSettings(event.Payload.Settings)
	if err != nil {
		log.Println("OnWillDisappear settings unmarshal", err)
	}
	delete(p.graphs, event.Context)
	delete(p.divisorCache, event.Context)
	p.am.RemoveAction(event.Context)
}

// OnApplicationDidLaunch event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidLaunch(event *streamdeck.EvApplication) {}

// OnApplicationDidTerminate event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidTerminate(event *streamdeck.EvApplication) {}

// OnTitleParametersDidChange event
func (p *Plugin) OnTitleParametersDidChange(event *streamdeck.EvTitleParametersDidChange) {
	if p.isSettingsAction(event.Action, event.Context) {
		targetContext := p.resolveSettingsContext(event.Context)
		tileSettings := p.settingsContexts[targetContext]
		if tileSettings == nil {
			tileSettings = &settingsTileSettings{
				TileBackground: "#000000",
				TileTextColor:  "#ffffff",
				ShowLabel:      true,
				Title:          "",
				TitleColor:     "#b7b7b7",
			}
		}

		tileSettings.Title = event.Payload.Title
		if event.Payload.TitleParameters.TitleColor != "" {
			tileSettings.TitleColor = event.Payload.TitleParameters.TitleColor
		} else if tileSettings.TitleColor == "" {
			tileSettings.TitleColor = "#b7b7b7"
		}
		drawTitle := !event.Payload.TitleParameters.ShowTitle
		tileSettings.ShowTitleInGraph = boolPtr(drawTitle)
		log.Printf(
			"OnTitleParametersDidChange settings context=%s title=%q drawTitle=%t nativeShowTitle=%t\n",
			targetContext,
			tileSettings.Title,
			drawTitle,
			event.Payload.TitleParameters.ShowTitle,
		)

		p.settingsContexts[targetContext] = tileSettings
		if err := p.sd.SetSettings(targetContext, tileSettings); err != nil {
			log.Printf("OnTitleParametersDidChange settings SetSettings: %v\n", err)
		}
		p.updateSettingsTile(targetContext)
		return
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
	g, ok := p.graphs[event.Context]
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
		log.Printf("OnPropertyInspectorConnected settings context=%s action=%s\n", event.Context, event.Action)
		p.sendSettingsStatus("com.moeilijk.lhm.settings", event.Context)
		return
	}

	log.Println("OnPropertyInspectorConnected enter", event.Context)
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		log.Println("OnPropertyInspectorConnected getSettings", err)
	}
	sensors, err := p.sensorsWithTimeout(2 * time.Second)
	if err != nil {
		log.Println("OnPropertyInspectorConnected Sensors", err)
		go p.restartBridge()
		payload := evStatus{Error: true, Message: "Libre Hardware Monitor Unavailable"}
		err := p.sd.SendToPropertyInspector(event.Action, event.Context, payload)
		settings.InErrorState = true
		err = p.sd.SetSettings(event.Context, &settings)
		if err != nil {
			log.Printf("OnPropertyInspectorConnected SetSettings: %v\n", err)
			return
		}
		p.am.SetAction(event.Action, event.Context, &settings)
		if err != nil {
			log.Println("updateTiles SendToPropertyInspector", err)
		}
		return
	}
	evsensors := make([]*evSendSensorsPayloadSensor, 0, len(sensors))
	for _, s := range sensors {
		evsensors = append(evsensors, &evSendSensorsPayloadSensor{UID: s.ID(), Name: s.Name()})
	}
	payload := evSendSensorsPayload{Sensors: evsensors, Settings: &settings}
	err = p.sd.SendToPropertyInspector(event.Action, event.Context, payload)
	if err != nil {
		log.Println("OnPropertyInspectorConnected SendToPropertyInspector", err)
	} else {
		log.Printf("OnPropertyInspectorConnected Sent sensors: %d\n", len(evsensors))
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
	if p.hw == nil {
		return nil, fmt.Errorf("LHM bridge not ready")
	}
	readings, err := p.hw.ReadingsForSensorID(sensorID)
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
		})
	}
	payload := evSendReadingsPayload{Readings: evreadings, Settings: settings}
	err = p.sd.SendToPropertyInspector(action, context, payload)
	if err != nil {
		log.Println("sendReadingsToPropertyInspector SendToPropertyInspector", err)
	} else {
		log.Printf("sendReadingsToPropertyInspector Sent readings: %d\n", len(evreadings))
	}
	return readings, nil
}

// OnSendToPlugin event
func (p *Plugin) OnSendToPlugin(event *streamdeck.EvSendToPlugin) {
	log.Printf("OnSendToPlugin action=%s context=%s\n", event.Action, event.Context)

	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*event.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
	}

	// Handle settings action commands
	if p.isSettingsAction(event.Action, event.Context) || isSettingsPayload(payload) {
		targetContext := p.resolveSettingsContext(event.Context)
		log.Printf(
			"OnSendToPlugin settings action=%s context=%s target=%s keys=%v\n",
			event.Action,
			event.Context,
			targetContext,
			payloadKeys(payload),
		)
		// Check for settingsConnected
		if _, ok := payload["settingsConnected"]; ok {
			log.Println("Settings PI connected")
			p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext)
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

		// Check for updateTileAppearance
		if raw, ok := payload["updateTileAppearance"]; ok {
			var appearance settingsTileSettings
			if err := json.Unmarshal(*raw, &appearance); err == nil {
				log.Printf(
					"updateTileAppearance context=%s target=%s bg=%s text=%s showLabel=%t\n",
					event.Context,
					targetContext,
					appearance.TileBackground,
					appearance.TileTextColor,
					appearance.ShowLabel,
				)
				// Update stored settings
				if appearance.TileBackground == "" {
					appearance.TileBackground = "#000000"
				}
				if appearance.TileTextColor == "" {
					appearance.TileTextColor = "#ffffff"
				}
				if existing := p.settingsContexts[targetContext]; existing != nil {
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
				p.settingsContexts[targetContext] = &appearance
				if err := p.sd.SetSettings(targetContext, &appearance); err != nil {
					log.Printf("updateTileAppearance SetSettings failed: %v\n", err)
				}
				p.updateSettingsTile(targetContext)
				p.sendSettingsStatus("com.moeilijk.lhm.settings", targetContext)
			}
			return
		}
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
			"thresholdOperator", "thresholdValue", "thresholdText", "thresholdTextColor",
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
	log.Printf("OnDidReceiveSettings raw context=%s payload=%s\n", event.Context, string(*event.Payload.Settings))

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
	if tileSettings.TitleColor == "" {
		tileSettings.TitleColor = "#b7b7b7"
	}
	if tileSettings.ShowTitleInGraph == nil {
		tileSettings.ShowTitleInGraph = boolPtr(true)
	}

	p.settingsContexts[event.Context] = &tileSettings
	log.Printf(
		"OnDidReceiveSettings context=%s bg=%s text=%s showLabel=%t\n",
		event.Context,
		tileSettings.TileBackground,
		tileSettings.TileTextColor,
		tileSettings.ShowLabel,
	)
	// Repair partial payloads (e.g. PI sending only color/showLabel) so title fields persist.
	if !hasShowLabel || !hasTitle || !hasTitleColor || !hasShowTitleInGraph {
		if err := p.sd.SetSettings(event.Context, &tileSettings); err != nil {
			log.Printf("OnDidReceiveSettings repair SetSettings failed: %v\n", err)
		}
	}
	p.updateSettingsTile(event.Context)
	p.sendSettingsStatus("com.moeilijk.lhm.settings", event.Context)
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

	log.Printf("Received global settings: pollInterval=%d\n", gs.PollInterval)

	if gs.PollInterval <= 0 {
		// First time, no settings saved yet.
		p.globalSettings.PollInterval = int(defaultPollInterval.Milliseconds())
		return
	}

	if gs.PollInterval < 250 {
		gs.PollInterval = 250
	}
	if gs.PollInterval > 2000 {
		gs.PollInterval = 2000
	}

	// Keep the cached global settings in sync even when value did not change.
	changed := gs.PollInterval != p.globalSettings.PollInterval
	p.globalSettings = gs

	if changed {
		interval := time.Duration(gs.PollInterval) * time.Millisecond
		p.am.SetInterval(interval)
		p.pollTimeCacheTTL = pollTimeCacheTTLForInterval(interval)
		log.Printf("Applied global settings: interval=%v\n", interval)
	}
	p.updateAllSettingsTiles()
}
