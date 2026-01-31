package lhmstreamdeckplugin

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/shayne/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
	"github.com/shayne/lhm-streamdeck/pkg/streamdeck"
)

func (p *Plugin) handleSensorSelect(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	sensorid := sdpi.Value
	readings, err := p.hw.ReadingsForSensorID(sensorid)
	if err != nil {
		return fmt.Errorf("handleSensorSelect ReadingsBySensor failed: %v", err)
	}
	evreadings := []*evSendReadingsPayloadReading{}
	for _, r := range readings {
		evreadings = append(evreadings, &evSendReadingsPayloadReading{ID: r.ID(), Label: r.Label(), Prefix: r.Unit(), Unit: r.Unit()})
	}
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleReadingSelect getSettings: %v", err)
	}
	// only update settings if SensorUID is changing
	// this covers case where PI sends event when tile
	// selected in SD UI
	if settings.SensorUID != sensorid {
		settings.SensorUID = sensorid
		settings.ReadingID = 0
		settings.ReadingLabel = ""
		settings.IsValid = false
	}
	payload := evSendReadingsPayload{Readings: evreadings, Settings: &settings}
	err = p.sd.SendToPropertyInspector(event.Action, event.Context, payload)
	if err != nil {
		return fmt.Errorf("sensorsSelect SendToPropertyInspector: %v", err)
	}
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleSensorSelect SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func getDefaultMinMaxForReading(r hwsensorsservice.Reading) (int, int) {
	switch r.Unit() {
	case "%":
		return 0, 100
	case "Yes/No":
		return 0, 1
	}
	min := r.ValueMin()
	max := r.ValueMax()
	min -= min * .2
	if min <= 0 {
		min = 0.
	}
	max += max * .2
	return int(min), int(max)
}

func (p *Plugin) handleReadingSelect(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	rid64, err := strconv.ParseInt(sdpi.Value, 10, 32)
	if err != nil {
		return fmt.Errorf("handleReadingSelect Atoi failed: %s, %v", sdpi.Value, err)
	}
	rid := int32(rid64)
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleReadingSelect getSettings: %v", err)
	}

	// no action if reading didn't change
	if settings.ReadingID == rid {
		return nil
	}

	settings.ReadingID = rid

	// set default min/max
	r, err := p.getReading(settings.SensorUID, settings.ReadingID)
	if err != nil {
		return fmt.Errorf("handleReadingSelect getReading: %v", err)
	}
	settings.ReadingLabel = r.Label()

	g, ok := p.graphs[event.Context]
	if !ok {
		return fmt.Errorf("handleReadingSelect no graph for context: %s", event.Context)
	}
	defaultMin, defaultMax := getDefaultMinMaxForReading(r)
	settings.Min = defaultMin
	g.SetMin(settings.Min)
	settings.Max = defaultMax
	g.SetMax(settings.Max)
	settings.IsValid = true // set IsValid once we choose reading

	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleReadingSelect SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleSetMin(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	min, err := strconv.Atoi(sdpi.Value)
	if err != nil {
		return fmt.Errorf("handleSetMin strconv: %v", err)
	}
	g, ok := p.graphs[event.Context]
	if !ok {
		return fmt.Errorf("handleSetMax no graph for context: %s", event.Context)
	}
	g.SetMin(min)
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleSetMin getSettings: %v", err)
	}
	settings.Min = min
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleSetMin SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleSetMax(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	max, err := strconv.Atoi(sdpi.Value)
	if err != nil {
		return fmt.Errorf("handleSetMax strconv: %v", err)
	}
	g, ok := p.graphs[event.Context]
	if !ok {
		return fmt.Errorf("handleSetMax no graph for context: %s", event.Context)
	}
	g.SetMax(max)
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleSetMax getSettings: %v", err)
	}
	settings.Max = max
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleSetMax SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleSetFormat(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	format := sdpi.Value
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleSetFormat getSettings: %v", err)
	}
	settings.Format = format
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleSetFormat SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleDivisor(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	divisor := sdpi.Value
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleDivisor getSettings: %v", err)
	}
	settings.Divisor = divisor
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleDivisor SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleSetGraphUnit(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	graphUnit := sdpi.Value
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleSetGraphUnit getSettings: %v", err)
	}
	settings.GraphUnit = graphUnit
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleSetGraphUnit SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

const (
	hexFormat      = "#%02x%02x%02x"
	hexShortFormat = "#%1x%1x%1x"
	hexToRGBFactor = 17
)

func hexToRGBA(hex string) *color.RGBA {
	var r, g, b uint8

	if len(hex) == 4 {
		fmt.Sscanf(hex, hexShortFormat, &r, &g, &b)
		r *= hexToRGBFactor
		g *= hexToRGBFactor
		b *= hexToRGBFactor
	} else {
		fmt.Sscanf(hex, hexFormat, &r, &g, &b)
	}

	return &color.RGBA{R: r, G: g, B: b, A: 255}
}

func (p *Plugin) handleColorChange(event *streamdeck.EvSendToPlugin, key string, sdpi *evSdpiCollection) error {
	hex := sdpi.Value
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleDivisor getSettings: %v", err)
	}
	g, ok := p.graphs[event.Context]
	if !ok {
		return fmt.Errorf("handleSetMax no graph for context: %s", event.Context)
	}
	clr := hexToRGBA(hex)
	switch key {
	case "foreground":
		settings.ForegroundColor = hex
		g.SetForegroundColor(clr)
	case "background":
		settings.BackgroundColor = hex
		g.SetBackgroundColor(clr)
	case "highlight":
		settings.HighlightColor = hex
		g.SetHighlightColor(clr)
	case "valuetext":
		settings.ValueTextColor = hex
		g.SetLabelColor(1, clr)
	case "warningBackground":
		settings.WarningBackgroundColor = hex
	case "warningForeground":
		settings.WarningForegroundColor = hex
	case "warningHighlight":
		settings.WarningHighlightColor = hex
	case "warningValuetext":
		settings.WarningValueTextColor = hex
	case "criticalBackground":
		settings.CriticalBackgroundColor = hex
	case "criticalForeground":
		settings.CriticalForegroundColor = hex
	case "criticalHighlight":
		settings.CriticalHighlightColor = hex
	case "criticalValuetext":
		settings.CriticalValueTextColor = hex
	}
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleColorChange SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleSetFontSize(event *streamdeck.EvSendToPlugin, key string, sdpi *evSdpiCollection) error {
	sv := sdpi.Value
	size, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return fmt.Errorf("failed to convert value to float: %w", err)
	}

	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("getSettings failed: %w", err)
	}

	g, ok := p.graphs[event.Context]
	if !ok {
		return fmt.Errorf("no graph for context: %s", event.Context)
	}

	switch key {
	case "titleFontSize":
		settings.TitleFontSize = size
		g.SetLabelFontSize(0, size)
	case "valueFontSize":
		settings.ValueFontSize = size
		g.SetLabelFontSize(1, size)
	default:
		return fmt.Errorf("invalid key: %s", sdpi.Key)
	}

	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("SetSettings failed: %w", err)
	}

	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleWarningEnabled(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	enabled := sdpi.Checked
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleWarningEnabled getSettings: %v", err)
	}
	settings.WarningEnabled = enabled
	// Reset alert state when disabled and not in critical
	if !enabled && settings.CurrentAlertState == "warning" {
		settings.CurrentAlertState = "none"
		if g, ok := p.graphs[event.Context]; ok {
			p.applyNormalColors(g, &settings)
		}
	}
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleWarningEnabled SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleCriticalEnabled(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	enabled := sdpi.Checked
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleCriticalEnabled getSettings: %v", err)
	}
	settings.CriticalEnabled = enabled
	// Reset alert state when disabled
	if !enabled && settings.CurrentAlertState == "critical" {
		settings.CurrentAlertState = "none"
		if g, ok := p.graphs[event.Context]; ok {
			p.applyNormalColors(g, &settings)
		}
	}
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleCriticalEnabled SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleWarningValue(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	value, err := strconv.ParseFloat(sdpi.Value, 64)
	if err != nil {
		return fmt.Errorf("handleWarningValue ParseFloat: %v", err)
	}
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleWarningValue getSettings: %v", err)
	}
	settings.WarningValue = value
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleWarningValue SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleCriticalValue(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	value, err := strconv.ParseFloat(sdpi.Value, 64)
	if err != nil {
		return fmt.Errorf("handleCriticalValue ParseFloat: %v", err)
	}
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleCriticalValue getSettings: %v", err)
	}
	settings.CriticalValue = value
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleCriticalValue SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleWarningOperator(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	operator := sdpi.Value
	if !isValidOperator(operator) {
		return fmt.Errorf("handleWarningOperator invalid operator: %s", operator)
	}
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleWarningOperator getSettings: %v", err)
	}
	settings.WarningOperator = operator
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleWarningOperator SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

func (p *Plugin) handleCriticalOperator(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	operator := sdpi.Value
	if !isValidOperator(operator) {
		return fmt.Errorf("handleCriticalOperator invalid operator: %s", operator)
	}
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleCriticalOperator getSettings: %v", err)
	}
	settings.CriticalOperator = operator
	err = p.sd.SetSettings(event.Context, &settings)
	if err != nil {
		return fmt.Errorf("handleCriticalOperator SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

// isValidOperator checks if the given operator is valid
func isValidOperator(op string) bool {
	switch op {
	case ">", "<", ">=", "<=", "==":
		return true
	default:
		return false
	}
}

// evaluateThreshold checks if value triggers the threshold based on operator
func evaluateThreshold(value, threshold float64, operator string) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return false
	}
}

// applyNormalColors applies normal (non-alert) colors to the graph
func (p *Plugin) applyNormalColors(g *graph.Graph, s *actionSettings) {
	if s.ForegroundColor != "" {
		g.SetForegroundColor(hexToRGBA(s.ForegroundColor))
	} else {
		g.SetForegroundColor(&color.RGBA{0, 81, 40, 255})
	}
	if s.BackgroundColor != "" {
		g.SetBackgroundColor(hexToRGBA(s.BackgroundColor))
	} else {
		g.SetBackgroundColor(&color.RGBA{0, 0, 0, 255})
	}
	if s.HighlightColor != "" {
		g.SetHighlightColor(hexToRGBA(s.HighlightColor))
	} else {
		g.SetHighlightColor(&color.RGBA{0, 158, 0, 255})
	}
	if s.ValueTextColor != "" {
		g.SetLabelColor(1, hexToRGBA(s.ValueTextColor))
	} else {
		g.SetLabelColor(1, &color.RGBA{255, 255, 255, 255})
	}
}

// applyWarningColors applies warning colors to the graph
func (p *Plugin) applyWarningColors(g *graph.Graph, s *actionSettings) {
	if s.WarningBackgroundColor != "" {
		g.SetBackgroundColor(hexToRGBA(s.WarningBackgroundColor))
	} else {
		g.SetBackgroundColor(&color.RGBA{51, 51, 0, 255}) // dark yellow
	}
	if s.WarningForegroundColor != "" {
		g.SetForegroundColor(hexToRGBA(s.WarningForegroundColor))
	} else {
		g.SetForegroundColor(&color.RGBA{153, 153, 0, 255}) // medium yellow
	}
	if s.WarningHighlightColor != "" {
		g.SetHighlightColor(hexToRGBA(s.WarningHighlightColor))
	} else {
		g.SetHighlightColor(&color.RGBA{255, 255, 0, 255}) // bright yellow
	}
	if s.WarningValueTextColor != "" {
		g.SetLabelColor(1, hexToRGBA(s.WarningValueTextColor))
	} else {
		g.SetLabelColor(1, &color.RGBA{255, 255, 0, 255}) // yellow
	}
}

// applyCriticalColors applies critical colors to the graph
func (p *Plugin) applyCriticalColors(g *graph.Graph, s *actionSettings) {
	if s.CriticalBackgroundColor != "" {
		g.SetBackgroundColor(hexToRGBA(s.CriticalBackgroundColor))
	} else {
		g.SetBackgroundColor(&color.RGBA{102, 0, 0, 255}) // dark red
	}
	if s.CriticalForegroundColor != "" {
		g.SetForegroundColor(hexToRGBA(s.CriticalForegroundColor))
	} else {
		g.SetForegroundColor(&color.RGBA{153, 0, 0, 255}) // medium red
	}
	if s.CriticalHighlightColor != "" {
		g.SetHighlightColor(hexToRGBA(s.CriticalHighlightColor))
	} else {
		g.SetHighlightColor(&color.RGBA{255, 51, 51, 255}) // bright red
	}
	if s.CriticalValueTextColor != "" {
		g.SetLabelColor(1, hexToRGBA(s.CriticalValueTextColor))
	} else {
		g.SetLabelColor(1, &color.RGBA{255, 0, 0, 255}) // red
	}
}
