package lhmstreamdeckplugin

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

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
	r, _, err := p.getReading(settings.SensorUID, settings.ReadingID)
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

// colorCache stores parsed colors to avoid repeated parsing
var colorCache = make(map[string]*color.RGBA)

func hexToRGBA(hex string) *color.RGBA {
	if c, ok := colorCache[hex]; ok {
		return c
	}

	var r, g, b uint8
	if len(hex) == 4 {
		fmt.Sscanf(hex, hexShortFormat, &r, &g, &b)
		r *= hexToRGBFactor
		g *= hexToRGBFactor
		b *= hexToRGBFactor
	} else {
		fmt.Sscanf(hex, hexFormat, &r, &g, &b)
	}

	c := &color.RGBA{R: r, G: g, B: b, A: 255}
	colorCache[hex] = c
	return c
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
	// Set default operator if not set (HTML default is ">")
	if enabled && settings.WarningOperator == "" {
		settings.WarningOperator = ">"
	}
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
	// Set default operator if not set (HTML default is ">")
	if enabled && settings.CriticalOperator == "" {
		settings.CriticalOperator = ">"
	}
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
		// For == comparison, round both to integers since displayed values are typically integers
		// e.g., displayed "52%" might actually be 52.34, user expects == 52 to match
		return int(value+0.5) == int(threshold+0.5)
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
	if s.ValueTextColor != "" {
		g.SetLabelColor(2, hexToRGBA(s.ValueTextColor))
	} else {
		g.SetLabelColor(2, &color.RGBA{255, 255, 255, 255})
	}
}

// ============================================================================
// Dynamic Threshold Handlers
// ============================================================================

// findThresholdByID returns pointer to threshold and its index
func (s *actionSettings) findThresholdByID(id string) (*Threshold, int) {
	for i := range s.Thresholds {
		if s.Thresholds[i].ID == id {
			return &s.Thresholds[i], i
		}
	}
	return nil, -1
}

// handleAddThreshold adds a new threshold to the settings
func (p *Plugin) handleAddThreshold(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleAddThreshold getSettings: %v", err)
	}

	// New thresholds get priority 0 (lowest, appears at bottom since list is sorted highest first)
	name := sdpi.Value
	if name == "" {
		name = "New"
	}

	bg := defaultColor(settings.BackgroundColor, "#000000")
	fg := defaultColor(settings.ForegroundColor, "#005128")
	hl := defaultColor(settings.HighlightColor, "#009e00")
	vt := defaultColor(settings.ValueTextColor, "#ffffff")

	newThreshold := Threshold{
		ID:              fmt.Sprintf("threshold_%d", time.Now().UnixNano()),
		Name:            name,
		Text:            "",
		TextColor:       vt,
		Enabled:         true,
		Operator:        ">=",
		Value:           0,
		BackgroundColor: bg,
		ForegroundColor: fg,
		HighlightColor:  hl,
		ValueTextColor:  vt,
	}

	settings.Thresholds = append(settings.Thresholds, newThreshold)

	if err := p.sd.SetSettings(event.Context, &settings); err != nil {
		return fmt.Errorf("handleAddThreshold SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)

	// Send updated thresholds to PI
	return p.sendThresholdsToPI(event.Action, event.Context, &settings)
}

// handleRemoveThreshold removes a threshold from the settings
func (p *Plugin) handleRemoveThreshold(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleRemoveThreshold getSettings: %v", err)
	}

	thresholdID := sdpi.ThresholdID
	_, idx := settings.findThresholdByID(thresholdID)
	if idx == -1 {
		return fmt.Errorf("threshold not found: %s", thresholdID)
	}

	// Remove threshold from slice
	settings.Thresholds = append(settings.Thresholds[:idx], settings.Thresholds[idx+1:]...)

	// Clear current threshold if it was the removed one
	if settings.CurrentThresholdID == thresholdID {
		settings.CurrentThresholdID = ""
		if g, ok := p.graphs[event.Context]; ok {
			p.applyNormalColors(g, &settings)
		}
	}

	if err := p.sd.SetSettings(event.Context, &settings); err != nil {
		return fmt.Errorf("handleRemoveThreshold SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)

	return p.sendThresholdsToPI(event.Action, event.Context, &settings)
}

// handleReorderThreshold moves a threshold up/down in priority order
func (p *Plugin) handleReorderThreshold(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleReorderThreshold getSettings: %v", err)
	}

	if len(settings.Thresholds) < 2 {
		return nil
	}

	direction := sdpi.Value
	if direction != "up" && direction != "down" {
		return fmt.Errorf("handleReorderThreshold invalid direction: %s", direction)
	}

	pos := -1
	for i := range settings.Thresholds {
		if settings.Thresholds[i].ID == sdpi.ThresholdID {
			pos = i
			break
		}
	}
	if pos == -1 {
		return fmt.Errorf("threshold not found: %s", sdpi.ThresholdID)
	}

	switch direction {
	case "up":
		if pos == 0 {
			return nil
		}
		settings.Thresholds[pos-1], settings.Thresholds[pos] = settings.Thresholds[pos], settings.Thresholds[pos-1]
	case "down":
		if pos == len(settings.Thresholds)-1 {
			return nil
		}
		settings.Thresholds[pos], settings.Thresholds[pos+1] = settings.Thresholds[pos+1], settings.Thresholds[pos]
	}

	settings.CurrentThresholdID = "_FORCE_REEVALUATE_"

	if err := p.sd.SetSettings(event.Context, &settings); err != nil {
		return fmt.Errorf("handleReorderThreshold SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)

	return p.sendThresholdsToPI(event.Action, event.Context, &settings)
}

// handleThresholdUpdate updates a specific field of a threshold
func (p *Plugin) handleThresholdUpdate(event *streamdeck.EvSendToPlugin, sdpi *evSdpiCollection) error {
	settings, err := p.am.getSettings(event.Context)
	if err != nil {
		return fmt.Errorf("handleThresholdUpdate getSettings: %v", err)
	}

	threshold, _ := settings.findThresholdByID(sdpi.ThresholdID)
	if threshold == nil {
		return fmt.Errorf("threshold not found: %s", sdpi.ThresholdID)
	}

	needsReEvaluation := false
	needsColorUpdate := false

	// Update based on key
	switch sdpi.Key {
	case "thresholdEnabled":
		threshold.Enabled = sdpi.Checked
		needsReEvaluation = true
	case "thresholdName":
		threshold.Name = sdpi.Value
	case "thresholdOperator":
		if isValidOperator(sdpi.Value) {
			threshold.Operator = sdpi.Value
			needsReEvaluation = true
		}
	case "thresholdValue":
		value, _ := strconv.ParseFloat(sdpi.Value, 64)
		threshold.Value = value
		needsReEvaluation = true
	case "thresholdText":
		threshold.Text = sdpi.Value
	case "thresholdTextColor":
		threshold.TextColor = sdpi.Value
		needsColorUpdate = settings.CurrentThresholdID == threshold.ID
	case "thresholdBackgroundColor":
		threshold.BackgroundColor = sdpi.Value
		needsColorUpdate = settings.CurrentThresholdID == threshold.ID
	case "thresholdForegroundColor":
		threshold.ForegroundColor = sdpi.Value
		needsColorUpdate = settings.CurrentThresholdID == threshold.ID
	case "thresholdHighlightColor":
		threshold.HighlightColor = sdpi.Value
		needsColorUpdate = settings.CurrentThresholdID == threshold.ID
	case "thresholdValueTextColor":
		threshold.ValueTextColor = sdpi.Value
		needsColorUpdate = settings.CurrentThresholdID == threshold.ID
	}

	// Re-evaluate thresholds and apply colors immediately if needed
	if needsReEvaluation {
		// Mark as needing re-evaluation by using special marker
		settings.CurrentThresholdID = "_FORCE_REEVALUATE_"
	}

	// If this threshold is currently active and its colors changed, apply immediately
	if needsColorUpdate {
		if g, ok := p.graphs[event.Context]; ok {
			p.applyThresholdColors(g, threshold)
		}
	}

	if err := p.sd.SetSettings(event.Context, &settings); err != nil {
		return fmt.Errorf("handleThresholdUpdate SetSettings: %v", err)
	}
	p.am.SetAction(event.Action, event.Context, &settings)
	return nil
}

// applyThresholdColors applies colors from a specific threshold to the graph
func (p *Plugin) applyThresholdColors(g *graph.Graph, t *Threshold) {
	if t.BackgroundColor != "" {
		g.SetBackgroundColor(hexToRGBA(t.BackgroundColor))
	}
	if t.ForegroundColor != "" {
		g.SetForegroundColor(hexToRGBA(t.ForegroundColor))
	}
	if t.HighlightColor != "" {
		g.SetHighlightColor(hexToRGBA(t.HighlightColor))
	}
	if t.ValueTextColor != "" {
		g.SetLabelColor(1, hexToRGBA(t.ValueTextColor))
	}
	if t.TextColor != "" {
		g.SetLabelColor(2, hexToRGBA(t.TextColor))
	}
}

// sendThresholdsToPI sends the current thresholds to the Property Inspector
func (p *Plugin) sendThresholdsToPI(action, context string, settings *actionSettings) error {
	payload := map[string]interface{}{
		"thresholds": settings.Thresholds,
		"settings":   settings,
	}
	return p.sd.SendToPropertyInspector(action, context, payload)
}
