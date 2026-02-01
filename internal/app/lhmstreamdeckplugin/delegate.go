package lhmstreamdeckplugin

import (
	"encoding/json"
	"image/color"
	"log"
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

// OnConnected event
func (p *Plugin) OnConnected(c *websocket.Conn) {
	log.Println("OnConnected")
}

// OnWillAppear event
func (p *Plugin) OnWillAppear(event *streamdeck.EvWillAppear) {
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
	_, _, err := decodeActionSettings(event.Payload.Settings)
	if err != nil {
		log.Println("OnWillAppear settings unmarshal", err)
	}
	delete(p.graphs, event.Context)
	p.am.RemoveAction(event.Context)
}

// OnApplicationDidLaunch event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidLaunch(event *streamdeck.EvApplication) {}

// OnApplicationDidTerminate event (unused for LHM bridge)
func (p *Plugin) OnApplicationDidTerminate(event *streamdeck.EvApplication) {}

// OnTitleParametersDidChange event
func (p *Plugin) OnTitleParametersDidChange(event *streamdeck.EvTitleParametersDidChange) {
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
	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*event.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
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
