package lhmstreamdeckplugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"github.com/moeilijk/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
	"github.com/moeilijk/lhm-streamdeck/pkg/streamdeck"
)

const (
	dialAction = "com.moeilijk.lhm.dial"
	dialWidth  = 200
	dialHeight = 100
)

type dialState struct {
	graphs []*graph.Graph
}

func decodeDialSettings(raw *json.RawMessage) (dialActionSettings, error) {
	var s dialActionSettings
	if raw != nil && *raw != nil {
		if err := json.Unmarshal(*raw, &s); err != nil {
			return s, err
		}
	}
	if s.ActiveIndex < 0 {
		s.ActiveIndex = 0
	}
	if len(s.Pages) > 0 {
		s.ActiveIndex %= len(s.Pages)
	}
	return s, nil
}

func newDialGraph(s *actionSettings) *graph.Graph {
	minValue := s.Min
	maxValue := s.Max
	if maxValue <= minValue {
		minValue = 0
		maxValue = 100
	}
	fg := hexToRGBA(s.ForegroundColor)
	if fg == nil {
		fg = &color.RGBA{0, 81, 40, 255}
	}
	bg := hexToRGBA(s.BackgroundColor)
	if bg == nil {
		bg = &color.RGBA{0, 0, 0, 255}
	}
	hl := hexToRGBA(s.HighlightColor)
	if hl == nil {
		hl = &color.RGBA{0, 158, 0, 255}
	}
	title := hexToRGBA(s.TitleColor)
	if title == nil {
		title = &color.RGBA{183, 183, 183, 255}
	}
	value := hexToRGBA(s.ValueTextColor)
	if value == nil {
		value = &color.RGBA{255, 255, 255, 255}
	}

	g := graph.NewGraph(dialWidth, dialHeight, minValue, maxValue, fg, bg, hl)
	g.SetLabel(0, "", 24, title)
	g.SetLabelFontSize(0, defaultDialTitleFontSize(s.TitleFontSize))
	g.SetLabel(1, "", 58, value)
	g.SetLabelFontSize(1, defaultDialValueFontSize(s.ValueFontSize))
	g.SetLabel(2, "", 82, value)
	g.SetLabelFontSize(2, 10.5)
	if s.GraphHeightPct > 0 {
		g.SetHeightPct(s.GraphHeightPct)
	}
	if s.GraphLineThickness > 0 {
		g.SetLineThickness(s.GraphLineThickness)
	}
	g.SetTextStroke(s.TextStroke)
	if s.TextStrokeColor != "" {
		g.SetTextStrokeColor(hexToRGBA(s.TextStrokeColor))
	}
	return g
}

func defaultDialTitleFontSize(v float64) float64 {
	if v > 0 {
		return v
	}
	return 14
}

func defaultDialValueFontSize(v float64) float64 {
	if v > 0 {
		return v
	}
	return 18
}

func initDialState(s *dialActionSettings) *dialState {
	state := &dialState{graphs: make([]*graph.Graph, len(s.Pages))}
	for i := range s.Pages {
		state.graphs[i] = newDialGraph(&s.Pages[i])
	}
	return state
}

func wrapDialIndex(current, ticks, count int) int {
	if count <= 0 {
		return 0
	}
	next := (current + ticks) % count
	if next < 0 {
		next += count
	}
	return next
}

func pngDataURL(b []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

func (p *Plugin) sendDialCanvas(ctx string, b []byte) {
	payload := map[string]interface{}{
		"full-canvas": pngDataURL(b),
		"title":       "",
	}
	if err := p.sd.SetFeedback(ctx, payload); err != nil {
		log.Printf("dial setFeedback: %v", err)
	}
}

func (p *Plugin) showDialMessage(ctx, title, value string) {
	s := actionSettings{
		Title:           title,
		Min:             0,
		Max:             100,
		ForegroundColor: "#003322",
		BackgroundColor: "#000000",
		HighlightColor:  "#00a36c",
		TitleColor:      "#b7b7b7",
		ValueTextColor:  "#ffffff",
	}
	g := newDialGraph(&s)
	_ = g.SetLabelText(0, title)
	_ = g.SetLabelText(1, value)
	g.Update(0)
	b, err := g.EncodePNG()
	if err != nil {
		log.Printf("dial message encode: %v", err)
		return
	}
	p.sendDialCanvas(ctx, b)
}

func (p *Plugin) updateDialFeedback(ctx string) {
	p.mu.RLock()
	settings := p.dialSettings[ctx]
	state := p.dialStates[ctx]
	p.mu.RUnlock()
	if settings == nil || state == nil {
		return
	}
	if len(settings.Pages) == 0 {
		p.showDialMessage(ctx, "LHM Dial", "Configure pages")
		return
	}
	if settings.ActiveIndex < 0 || settings.ActiveIndex >= len(settings.Pages) {
		settings.ActiveIndex = wrapDialIndex(settings.ActiveIndex, 0, len(settings.Pages))
	}
	page := &settings.Pages[settings.ActiveIndex]
	if settings.SourceProfileID != "" && page.SourceProfileID == "" {
		page.SourceProfileID = settings.SourceProfileID
	}
	if !page.IsValid {
		p.showDialMessage(ctx, "LHM Dial", fmt.Sprintf("Page %d empty", settings.ActiveIndex+1))
		return
	}
	if settings.ActiveIndex >= len(state.graphs) || state.graphs[settings.ActiveIndex] == nil {
		return
	}
	g := state.graphs[settings.ActiveIndex]

	profileID := p.resolvedSourceProfileID(page.SourceProfileID)
	pollTime, err := p.getCachedPollTimeForSource(profileID)
	if err != nil || pollTime == 0 {
		p.showDialMessage(ctx, "LHM Dial", "LHM unavailable")
		return
	}

	r, readings, err := p.getReadingForSource(profileID, page.SensorUID, page.ReadingID)
	if err != nil && page.ReadingLabel != "" {
		for _, candidate := range readings {
			if candidate.Label() == page.ReadingLabel {
				page.ReadingID = candidate.ID()
				r = candidate
				err = nil
				_ = p.sd.SetSettings(ctx, settings)
				break
			}
		}
	}
	if err != nil {
		p.showDialMessage(ctx, "LHM Dial", "Reading missing")
		return
	}

	title := page.Title
	if title == "" {
		title = r.Label()
	}
	if page.ShowTitleInGraph == nil || *page.ShowTitleInGraph {
		_ = g.SetLabelText(0, title)
	} else {
		_ = g.SetLabelText(0, "")
	}

	v := r.Value()
	divisor, err := p.getCachedDivisor(ctx+"|dial", page.Divisor)
	if err != nil {
		log.Printf("dial divisor: %v", err)
		return
	}
	if divisor != 1 {
		v /= divisor
	}

	graphValue := r.Value()
	if page.GraphUnit != "" && strings.Contains(r.Unit(), "/s") {
		graphValue = p.normalizeForGraph(r.Value(), r.Unit(), page.GraphUnit)
	}
	if divisor != 1 {
		graphValue /= divisor
	}

	displayValue := v
	displayUnit := r.Unit()
	if page.GraphUnit != "" && strings.Contains(r.Unit(), "/s") {
		displayValue = p.normalizeForGraph(v, r.Unit(), page.GraphUnit)
		displayUnit = page.GraphUnit + "/s"
	}
	valueTextNoUnit, displayText := p.formatDisplayValue(displayValue, displayUnit, page.Format, hwsensorsservice.ReadingType(r.TypeI()))

	now := time.Now()
	thresholds := p.resolveThresholdsForEval(page.Thresholds, page.SuppressedGlobalIDs, hwsensorsservice.ReadingType(r.TypeI()))
	activeThreshold := p.evaluateThresholds(ctx, v, thresholds, now)
	newThresholdID := ""
	alertText := ""
	if activeThreshold != nil {
		newThresholdID = activeThreshold.ID
		if activeThreshold.Text != "" {
			alertText = p.applyThresholdText(activeThreshold.Text, valueTextNoUnit, displayUnit)
		}
	}

	snoozeState, snoozed, _ := p.currentThresholdSnoozeState(ctx, now)
	if activeThreshold == nil {
		p.clearThresholdSnooze(ctx)
		snoozed = false
		snoozeState = thresholdSnoozeState{}
	}
	if newThresholdID != page.CurrentThresholdID {
		if activeThreshold != nil && !snoozed {
			p.applyThresholdColors(g, activeThreshold)
		} else {
			p.applyNormalColors(g, page)
		}
		page.CurrentThresholdID = newThresholdID
		_ = p.sd.SetSettings(ctx, settings)
	}

	renderDisplayText, renderAlertText, renderGraphValue, freezeGraph := p.resolveThresholdDisplay(
		ctx,
		activeThreshold,
		v,
		graphValue,
		displayText,
		alertText,
	)
	if snoozed {
		renderDisplayText = displayText
		renderAlertText = thresholdSnoozeText(snoozeState, now)
		renderGraphValue = graphValue
		freezeGraph = false
	}

	switch page.GraphMode {
	case "text":
		g.Clear()
	default:
		if !freezeGraph {
			g.Update(renderGraphValue)
		}
	}
	if page.GraphMode == "graph" {
		_ = g.SetLabelText(0, "")
		_ = g.SetLabelText(1, "")
		_ = g.SetLabelText(2, "")
	} else {
		_ = g.SetLabelText(1, renderDisplayText)
		if renderAlertText != "" {
			_ = g.SetLabelText(2, renderAlertText)
		} else if len(settings.Pages) > 1 {
			_ = g.SetLabelText(2, fmt.Sprintf("%d/%d", settings.ActiveIndex+1, len(settings.Pages)))
		} else {
			_ = g.SetLabelText(2, "")
		}
	}

	b, err := g.EncodePNG()
	if err != nil {
		log.Printf("dial encode: %v", err)
		return
	}
	p.sendDialCanvas(ctx, b)
}

func (p *Plugin) updateDialTick() {
	p.mu.RLock()
	contexts := make([]string, 0, len(p.dialSettings))
	for ctx := range p.dialSettings {
		contexts = append(contexts, ctx)
	}
	p.mu.RUnlock()
	for _, ctx := range contexts {
		p.updateDialFeedback(ctx)
	}
}

func (p *Plugin) handleDialWillAppear(event *streamdeck.EvWillAppear) {
	settings, err := decodeDialSettings(event.Payload.Settings)
	if err != nil {
		log.Printf("dial settings unmarshal: %v", err)
	}
	state := initDialState(&settings)
	p.mu.Lock()
	p.dialSettings[event.Context] = &settings
	p.dialStates[event.Context] = state
	p.mu.Unlock()
	if err := p.sd.SetFeedbackLayout(event.Context, "$A0"); err != nil {
		log.Printf("dial setFeedbackLayout: %v", err)
	}
	p.updateDialFeedback(event.Context)
}

func (p *Plugin) handleDialWillDisappear(event *streamdeck.EvWillDisappear) {
	p.mu.Lock()
	delete(p.dialSettings, event.Context)
	delete(p.dialStates, event.Context)
	p.mu.Unlock()
}

func (p *Plugin) handleDialPropertyInspectorConnected(event *streamdeck.EvSendToPlugin) {
	p.mu.RLock()
	settings := p.dialSettings[event.Context]
	var settingsCopy dialActionSettings
	if settings != nil {
		settingsCopy = *settings
	}
	p.mu.RUnlock()

	profileID := p.resolvedSourceProfileID(settingsCopy.SourceProfileID)
	sensors, err := p.sensorsWithTimeoutForSource(profileID, 2*time.Second)
	if err != nil {
		log.Printf("dial PI sensors: %v", err)
		go p.restartSource(p.runtimeForSource(profileID))
		_ = p.sd.SendToPropertyInspector(event.Action, event.Context, evStatus{Error: true, Message: "Libre Hardware Monitor Unavailable"})
		return
	}
	_ = p.sd.SendToPropertyInspector(event.Action, event.Context, map[string]interface{}{"error": false, "message": "show_ui"})
	catalogSettings := actionSettings{SourceProfileID: settingsCopy.SourceProfileID}
	if err := p.sendCatalogToPropertyInspector(event.Action, event.Context, &catalogSettings, sensors); err != nil {
		log.Printf("dial PI catalog: %v", err)
	}
	_ = p.sd.SendToPropertyInspector(event.Action, event.Context, map[string]interface{}{"dialSettings": settingsCopy})
}

func (p *Plugin) handleDialSendToPlugin(event *streamdeck.EvSendToPlugin, payload map[string]*json.RawMessage) bool {
	if event.Action != dialAction {
		return false
	}
	if raw, ok := payload["dialSetSettings"]; ok {
		var settings dialActionSettings
		if err := json.Unmarshal(*raw, &settings); err != nil {
			log.Printf("dialSetSettings unmarshal: %v", err)
			return true
		}
		if len(settings.Pages) > 0 {
			settings.ActiveIndex = wrapDialIndex(settings.ActiveIndex, 0, len(settings.Pages))
		} else {
			settings.ActiveIndex = 0
		}
		p.mu.Lock()
		p.dialSettings[event.Context] = &settings
		p.dialStates[event.Context] = initDialState(&settings)
		p.mu.Unlock()
		if err := p.sd.SetSettings(event.Context, &settings); err != nil {
			log.Printf("dialSetSettings SetSettings: %v", err)
		}
		p.updateDialFeedback(event.Context)
		return true
	}
	if _, ok := payload["requestDialCatalog"]; ok {
		p.handleDialPropertyInspectorConnected(event)
		return true
	}
	return true
}

func (p *Plugin) OnDialDown(event *streamdeck.EvDialDown) {}

func (p *Plugin) OnDialUp(event *streamdeck.EvDialUp) {}

func (p *Plugin) OnTouchTap(event *streamdeck.EvTouchTap) {
	if event.Action != dialAction || event.Payload.Controller != "Encoder" {
		return
	}

	p.mu.RLock()
	settings := p.dialSettings[event.Context]
	p.mu.RUnlock()
	if settings == nil || len(settings.Pages) == 0 {
		return
	}
	if settings.ActiveIndex < 0 || settings.ActiveIndex >= len(settings.Pages) {
		settings.ActiveIndex = wrapDialIndex(settings.ActiveIndex, 0, len(settings.Pages))
	}
	page := &settings.Pages[settings.ActiveIndex]
	if page.CurrentThresholdID == "" {
		return
	}

	if configured := normalizeThresholdSnoozeDurations(page.SnoozeDurations); len(configured) > 0 {
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
		p.updateDialFeedback(event.Context)
		return
	}

	if !p.clearStickyThreshold(event.Context, page.CurrentThresholdID) {
		return
	}
	page.CurrentThresholdID = ""
	if err := p.sd.SetSettings(event.Context, settings); err != nil {
		log.Printf("dial touch SetSettings: %v", err)
	}
	p.updateDialFeedback(event.Context)
}

func (p *Plugin) OnDialRotate(event *streamdeck.EvDialRotate) {
	if event.Action != dialAction || event.Payload.Controller != "Encoder" {
		return
	}
	if event.Payload.Ticks == 0 {
		return
	}

	p.mu.Lock()
	settings := p.dialSettings[event.Context]
	if settings == nil {
		p.mu.Unlock()
		return
	}
	settings.ActiveIndex = wrapDialIndex(settings.ActiveIndex, event.Payload.Ticks, len(settings.Pages))
	settingsCopy := *settings
	p.mu.Unlock()

	if err := p.sd.SetSettings(event.Context, &settingsCopy); err != nil {
		log.Printf("dial SetSettings: %v", err)
	}
	p.updateDialFeedback(event.Context)
}
