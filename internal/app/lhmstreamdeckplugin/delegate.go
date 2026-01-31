package lhmstreamdeckplugin

import (
	"encoding/json"
	"image/color"
	"log"

	"github.com/gorilla/websocket"
	"github.com/shayne/lhm-streamdeck/pkg/graph"
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
	var settings actionSettings
	err := json.Unmarshal(*event.Payload.Settings, &settings)
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
	if drawTitle {
		g.SetLabelText(0, settings.Title)
	}
	p.graphs[event.Context] = g
	p.am.SetAction(event.Action, event.Context, &settings)
}

// OnWillDisappear event
func (p *Plugin) OnWillDisappear(event *streamdeck.EvWillDisappear) {
	var settings actionSettings
	err := json.Unmarshal(*event.Payload.Settings, &settings)
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
	var settings actionSettings
	err := json.Unmarshal(*event.Payload.Settings, &settings)
	if err != nil {
		log.Println("OnWillAppear settings unmarshal", err)
	}
	g, ok := p.graphs[event.Context]
	if !ok {
		log.Printf("handleSetMax no graph for context: %s\n", event.Context)
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
	settings.Title = event.Payload.Title
	settings.TitleColor = event.Payload.TitleParameters.TitleColor
	settings.ShowTitleInGraph = boolPtr(drawTitle)

	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		log.Printf("handleSetTitle SetSettings: %v\n", err)
		return
	}
	p.am.SetAction(event.Action, event.Context, &settings)
}

// OnPropertyInspectorConnected event
func (p *Plugin) OnPropertyInspectorConnected(event *streamdeck.EvSendToPlugin) {
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		log.Println("OnPropertyInspectorConnected getSettings", err)
	}
	sensors, err := p.hw.Sensors()
	if err != nil {
		log.Println("OnPropertyInspectorConnected Sensors", err)
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
	}
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
		default:
			log.Printf("Unknown sdpi key: %s\n", sdpi.Key)
		}
		return
	}
}
