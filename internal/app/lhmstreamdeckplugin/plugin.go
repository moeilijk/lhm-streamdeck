package lhmstreamdeckplugin

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/hashicorp/go-plugin"
	"github.com/shayne/go-winpeg"
	"github.com/shayne/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
	"github.com/shayne/lhm-streamdeck/pkg/streamdeck"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Plugin handles information between Libre Hardware Monitor and Stream Deck
type Plugin struct {
	c      *plugin.Client
	peg    winpeg.ProcessExitGroup
	hw     hwsensorsservice.HardwareService
	sd     *streamdeck.StreamDeck
	am     *actionManager
	graphs map[string]*graph.Graph

	// Cached assets and state for performance
	placeholderImage []byte            // cached startup chip placeholder image
	cachedPollTime   uint64            // cached PollTime value
	cachedPollTimeAt time.Time         // when cachedPollTime was fetched
	lastPollTime     map[string]uint64 // last processed PollTime per context
	divisorCache     map[string]divisorCacheEntry

	// Global settings
	globalSettings   globalSettings                   // plugin-wide settings (poll interval)
	pollTimeCacheTTL time.Duration                    // cache TTL (matches poll interval)
	settingsContexts map[string]*settingsTileSettings // tracks settings action contexts with appearance
}

type sensorResult struct {
	sensors []hwsensorsservice.Sensor
	err     error
}

type divisorCacheEntry struct {
	raw   string
	value float64
}

const defaultPollInterval = time.Second
const settingsTitleFontSize = 9.0

func pollTimeCacheTTLForInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = defaultPollInterval
	}

	// Keep cache short enough so each ticker cycle fetches fresh PollTime,
	// while still allowing one shared value across actions in the same tick.
	ttl := interval / 2
	if ttl < 100*time.Millisecond {
		ttl = 100 * time.Millisecond
	}
	if ttl >= interval {
		ttl = interval - (25 * time.Millisecond)
		if ttl < 25*time.Millisecond {
			ttl = 25 * time.Millisecond
		}
	}
	return ttl
}

func (p *Plugin) sensorsWithTimeout(d time.Duration) ([]hwsensorsservice.Sensor, error) {
	if p.hw == nil {
		return nil, fmt.Errorf("LHM bridge not ready")
	}
	ch := make(chan sensorResult, 1)
	go func() {
		s, err := p.hw.Sensors()
		ch <- sensorResult{sensors: s, err: err}
	}()
	select {
	case res := <-ch:
		return res.sensors, res.err
	case <-time.After(d):
		return nil, fmt.Errorf("sensors timeout")
	}
}

func (p *Plugin) restartBridge() {
	if p.c != nil {
		p.c.Kill()
	}
	_ = p.startClient()
}

func (p *Plugin) startClient() error {
	cmd := exec.Command("./lhm-bridge.exe")

	// We're a host. Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  hwsensorsservice.Handshake,
		Plugins:          hwsensorsservice.PluginMap,
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return err
	}

	g, err := winpeg.NewProcessExitGroup()
	if err == nil {
		if attachProcessToJob(g, cmd.Process) == nil {
			p.peg = g
		}
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("lhmplugin")
	if err != nil {
		return err
	}

	p.c = client
	p.hw = raw.(hwsensorsservice.HardwareService)

	return nil
}

// NewPlugin creates an instance and initializes the plugin
func NewPlugin(port, uuid, event, info string) (*Plugin, error) {
	p := &Plugin{
		am:               newActionManager(defaultPollInterval),
		graphs:           make(map[string]*graph.Graph),
		lastPollTime:     make(map[string]uint64),
		divisorCache:     make(map[string]divisorCacheEntry),
		pollTimeCacheTTL: pollTimeCacheTTLForInterval(defaultPollInterval),
		settingsContexts: make(map[string]*settingsTileSettings),
	}

	// Cache placeholder image at startup.
	// Preferred source is settings/default tile art (startup chip).
	for _, candidate := range []string{"./settingsImage.png", "./defaultImage.png", "./launch-lhm.png"} {
		if bts, err := os.ReadFile(candidate); err == nil {
			p.placeholderImage = bts
			log.Printf("Cached settings placeholder image from %s\n", candidate)
			break
		}
	}
	if len(p.placeholderImage) == 0 {
		log.Printf("Warning: could not cache placeholder image\n")
	}

	p.startClient()
	p.sd = streamdeck.NewStreamDeck(port, uuid, event, info)
	return p, nil
}

// RunForever starts the plugin and waits for events, indefinitely
func (p *Plugin) RunForever() error {
	defer func() {
		if p.c != nil {
			p.c.Kill()
		}
		if p.peg != 0 {
			_ = p.peg.Dispose()
		}
	}()

	p.sd.SetDelegate(p)
	p.am.Run(p.updateTiles)

	go func() {
		for {
			if p.c == nil {
				if err := p.startClient(); err != nil {
					log.Printf("startClient failed: %v\n", err)
				}
			} else if p.c.Exited() {
				if err := p.startClient(); err != nil {
					log.Printf("restartClient failed: %v\n", err)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	err := p.sd.Connect()
	if err != nil {
		return fmt.Errorf("StreamDeck Connect: %v", err)
	}
	defer p.sd.Close()
	p.sd.ListenAndWait()
	return nil
}

func (p *Plugin) getReading(suid string, rid int32) (hwsensorsservice.Reading, []hwsensorsservice.Reading, error) {
	if p.hw == nil {
		return nil, nil, fmt.Errorf("LHM bridge not ready")
	}
	rbs, err := p.hw.ReadingsForSensorID(suid)
	if err != nil {
		return nil, nil, fmt.Errorf("getReading ReadingsBySensor failed: %v", err)
	}
	for _, r := range rbs {
		if r.ID() == rid {
			return r, rbs, nil
		}
	}
	return nil, rbs, fmt.Errorf("ReadingID does not exist: %s", suid)
}

func (p *Plugin) applyDefaultFormatValueOnly(v float64, t hwsensorsservice.ReadingType) string {
	switch t {
	case hwsensorsservice.ReadingTypeNone:
		return fmt.Sprintf("%0.f", v)
	case hwsensorsservice.ReadingTypeTemp:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypeVolt:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypeFan:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypeCurrent:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypePower:
		return fmt.Sprintf("%0.f", v)
	case hwsensorsservice.ReadingTypeClock:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypeUsage:
		return fmt.Sprintf("%.0f", v)
	case hwsensorsservice.ReadingTypeOther:
		return fmt.Sprintf("%.0f", v)
	}
	return "Bad Format"
}

// normalizeForGraph converts data size values to the target unit for consistent graph scaling.
// This prevents jumps when LHM switches units (e.g., 1000 KB/s → 1 MB/s).
// targetUnit can be: "B", "KB", "MB", "GB", "TB" or empty (no normalization).
func (p *Plugin) normalizeForGraph(value float64, sourceUnit string, targetUnit string) float64 {
	if targetUnit == "" {
		return value // no normalization
	}

	// Convert source value to bytes first
	sourceLower := strings.ToLower(sourceUnit)
	var bytes float64
	switch {
	case strings.HasPrefix(sourceLower, "tb") || strings.HasPrefix(sourceLower, "tib"):
		bytes = value * 1024 * 1024 * 1024 * 1024
	case strings.HasPrefix(sourceLower, "gb") || strings.HasPrefix(sourceLower, "gib"):
		bytes = value * 1024 * 1024 * 1024
	case strings.HasPrefix(sourceLower, "mb") || strings.HasPrefix(sourceLower, "mib"):
		bytes = value * 1024 * 1024
	case strings.HasPrefix(sourceLower, "kb") || strings.HasPrefix(sourceLower, "kib"):
		bytes = value * 1024
	case strings.HasPrefix(sourceLower, "b/") || sourceLower == "b":
		bytes = value
	default:
		return value // not a data size unit, no conversion
	}

	// Convert bytes to target unit
	switch strings.ToUpper(targetUnit) {
	case "TB":
		return bytes / (1024 * 1024 * 1024 * 1024)
	case "GB":
		return bytes / (1024 * 1024 * 1024)
	case "MB":
		return bytes / (1024 * 1024)
	case "KB":
		return bytes / 1024
	case "B":
		return bytes
	default:
		return value
	}
}

func (p *Plugin) getCachedPollTime() (uint64, error) {
	if p.hw == nil {
		return 0, fmt.Errorf("LHM bridge not ready")
	}
	cacheTTL := p.pollTimeCacheTTL
	if cacheTTL == 0 {
		cacheTTL = defaultPollInterval
	}
	if !p.cachedPollTimeAt.IsZero() && time.Since(p.cachedPollTimeAt) < cacheTTL {
		return p.cachedPollTime, nil
	}

	pollTime, err := p.hw.PollTime()
	if err != nil {
		p.cachedPollTime = 0
		p.cachedPollTimeAt = time.Now()
		return 0, err
	}

	p.cachedPollTime = pollTime
	p.cachedPollTimeAt = time.Now()
	return pollTime, nil
}

func (p *Plugin) getCachedDivisor(context, raw string) (float64, error) {
	if raw == "" {
		return 1, nil
	}
	if entry, ok := p.divisorCache[context]; ok && entry.raw == raw {
		return entry.value, nil
	}
	fdiv, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	p.divisorCache[context] = divisorCacheEntry{raw: raw, value: fdiv}
	return fdiv, nil
}

func (p *Plugin) updateTiles(data *actionData) {
	if data.action != "com.moeilijk.lhm.reading" {
		log.Printf("Unknown action updateTiles: %s\n", data.action)
		return
	}

	g, ok := p.graphs[data.context]
	if !ok {
		log.Printf("Graph not found for context: %s\n", data.context)
		return
	}

	showUnavailable := func() {
		if !data.settings.InErrorState {
			payload := evStatus{Error: true, Message: "Libre Hardware Monitor Unavailable"}
			err := p.sd.SendToPropertyInspector("com.moeilijk.lhm.reading", data.context, payload)
			if err != nil {
				log.Println("updateTiles SendToPropertyInspector", err)
			}
			data.settings.InErrorState = true
			p.sd.SetSettings(data.context, &data.settings)

			// Only set image on state transition (optimization #2)
			if len(p.placeholderImage) > 0 {
				if err := p.sd.SetImage(data.context, p.placeholderImage); err != nil {
					log.Printf("Failed to setImage: %v\n", err)
				}
			}
		}
		// Clear lastPollTime so we re-render when LHM comes back
		delete(p.lastPollTime, data.context)
	}

	// show ui on property inspector if in error state
	if data.settings.InErrorState {
		payload := evStatus{Error: false, Message: "show_ui"}
		err := p.sd.SendToPropertyInspector("com.moeilijk.lhm.reading", data.context, payload)
		if err != nil {
			log.Println("updateTiles SendToPropertyInspector", err)
		}
		data.settings.InErrorState = false
		p.sd.SetSettings(data.context, &data.settings)
	}

	s := data.settings
	forceUpdate := s.CurrentThresholdID == "_FORCE_REEVALUATE_"

	pollTime, err := p.getCachedPollTime()
	if err != nil {
		log.Printf("PollTime failed: %v\n", err)
		showUnavailable()
		return
	}
	if pollTime == 0 || time.Since(time.Unix(0, int64(pollTime))) > 5*time.Second {
		showUnavailable()
		return
	}

	if !forceUpdate {
		if last, ok := p.lastPollTime[data.context]; ok && last == pollTime {
			return
		}
	}
	r, readings, err := p.getReading(s.SensorUID, s.ReadingID)
	if err != nil {
		if s.ReadingLabel != "" {
			for _, candidate := range readings {
				if candidate.Label() == s.ReadingLabel {
					s.ReadingID = candidate.ID()
					r = candidate
					err = nil
					_ = p.sd.SetSettings(data.context, s)
					p.am.SetAction(data.action, data.context, s)
					break
				}
			}
		}
		if err != nil {
			log.Printf("getReading failed: %v\n", err)
			showUnavailable()
			return
		}
	}
	if s.ShowTitleInGraph != nil && *s.ShowTitleInGraph && s.Title == "" {
		g.SetLabelText(0, r.Label())
	}

	v := r.Value()
	divisor, err := p.getCachedDivisor(data.context, s.Divisor)
	if err != nil {
		log.Printf("Failed to parse float: %s\n", s.Divisor)
		return
	}
	if divisor != 1 {
		v = r.Value() / divisor
	}

	// Normalize the graph value to handle unit changes (e.g., KB/s → MB/s)
	// Only applies to throughput readings (units containing "/s")
	graphValue := r.Value()
	if s.GraphUnit != "" && strings.Contains(r.Unit(), "/s") {
		graphValue = p.normalizeForGraph(r.Value(), r.Unit(), s.GraphUnit)
	}
	if divisor != 1 {
		graphValue = graphValue / divisor
	}
	g.Update(graphValue)

	// Check threshold alerts (evaluate by priority, highest first)
	activeThreshold := p.evaluateThresholds(v, s.Thresholds)

	newThresholdID := ""
	if activeThreshold != nil {
		newThresholdID = activeThreshold.ID
	}

	// Check if forced re-evaluation or state transition
	if forceUpdate || newThresholdID != s.CurrentThresholdID {
		if activeThreshold != nil {
			p.applyThresholdColors(g, activeThreshold)
		} else {
			p.applyNormalColors(g, s)
		}
		s.CurrentThresholdID = newThresholdID
		p.am.SetAction(data.action, data.context, s)
		_ = p.sd.SetSettings(data.context, s)
	}

	// Determine display value and unit
	displayValue := v
	displayUnit := r.Unit()
	if s.GraphUnit != "" && strings.Contains(r.Unit(), "/s") {
		// Convert display value to match GraphUnit
		displayValue = p.normalizeForGraph(v, r.Unit(), s.GraphUnit)
		displayUnit = s.GraphUnit + "/s"
	}

	valueTextNoUnit := ""
	displayText := ""
	if f := s.Format; f != "" {
		valueTextNoUnit = fmt.Sprintf(f, displayValue)
		displayText = valueTextNoUnit
	} else {
		valueTextNoUnit = p.applyDefaultFormatValueOnly(displayValue, hwsensorsservice.ReadingType(r.TypeI()))
		if displayUnit != "" {
			if displayUnit == "%" {
				displayText = valueTextNoUnit + displayUnit
			} else {
				displayText = valueTextNoUnit + " " + displayUnit
			}
		} else {
			displayText = valueTextNoUnit
		}
	}

	g.SetLabelText(1, displayText)
	if activeThreshold != nil && activeThreshold.Text != "" {
		alertText := p.applyThresholdText(activeThreshold.Text, valueTextNoUnit, displayUnit)
		g.SetLabelText(2, alertText)
	} else {
		g.SetLabelText(2, "")
	}

	b, err := g.EncodePNG()
	if err != nil {
		log.Printf("Failed to encode graph: %v\n", err)
		return
	}

	err = p.sd.SetImage(data.context, b)
	if err != nil {
		log.Printf("Failed to setImage: %v\n", err)
		return
	}

	p.lastPollTime[data.context] = pollTime
}

func (p *Plugin) applyThresholdText(template, valueTextNoUnit, unit string) string {
	out := strings.ReplaceAll(template, "{value}", valueTextNoUnit)
	out = strings.ReplaceAll(out, "{unit}", unit)
	return out
}

// evaluateThresholds checks all thresholds top-to-bottom, last matching wins
func (p *Plugin) evaluateThresholds(value float64, thresholds []Threshold) *Threshold {
	if len(thresholds) == 0 {
		return nil
	}

	var active *Threshold
	for i := range thresholds {
		t := &thresholds[i]
		if t.Enabled && t.Operator != "" && evaluateThreshold(value, t.Value, t.Operator) {
			active = t
		}
	}
	return active
}

// updateSettingsTile updates the settings tile with current interval and appearance
func (p *Plugin) updateSettingsTile(context string) {
	intervalMs := p.globalSettings.PollInterval
	if intervalMs <= 0 {
		intervalMs = int(p.am.GetInterval().Milliseconds())
	}
	tileSettings := p.settingsContexts[context]
	if tileSettings == nil {
		tileSettings = &settingsTileSettings{
			TileBackground:   "#000000",
			TileTextColor:    "#ffffff",
			ShowLabel:        true,
			Title:            "",
			TitleColor:       "#b7b7b7",
			ShowTitleInGraph: boolPtr(true),
		}
	}
	log.Printf(
		"updateSettingsTile context=%s rate=%d bg=%s text=%s showLabel=%t\n",
		context,
		intervalMs,
		tileSettings.TileBackground,
		tileSettings.TileTextColor,
		tileSettings.ShowLabel,
	)

	// Parse colors
	bgColor := hexToRGBA(tileSettings.TileBackground)
	if bgColor == nil {
		bgColor = &color.RGBA{0, 0, 0, 255}
	}
	textColor := hexToRGBA(tileSettings.TileTextColor)
	if textColor == nil {
		textColor = &color.RGBA{255, 255, 255, 255}
	}
	titleColor := hexToRGBA(tileSettings.TitleColor)
	if titleColor == nil {
		titleColor = &color.RGBA{183, 183, 183, 255}
	}
	drawTitle := true
	if tileSettings.ShowTitleInGraph != nil {
		drawTitle = *tileSettings.ShowTitleInGraph
	}
	renderedTitle := strings.TrimSpace(tileSettings.Title)
	if renderedTitle == "" {
		renderedTitle = "Refresh Rate"
	}

	// The settings tile uses in-image text for title/value.
	// Keep native title empty to avoid duplicate/misaligned text.
	if err := p.sd.SetTitle(context, ""); err != nil {
		log.Printf("updateSettingsTile SetTitle failed: %v\n", err)
	}

	// For settings tile, ShowLabel toggles placeholder background on/off.
	// true  -> startup placeholder background + current interval
	// false -> user-selected solid background + current interval
	if tileSettings.ShowLabel {
		if img, err := p.renderSettingsPlaceholderTile(intervalMs, renderedTitle, drawTitle, titleColor, textColor); err == nil {
			if err := p.sd.SetImage(context, img); err != nil {
				log.Printf("updateSettingsTile SetImage failed: %v\n", err)
			}
			return
		}
		log.Printf("updateSettingsTile placeholder render failed, falling back to solid background\n")
	}

	// Create a graph just for rendering the tile
	g := graph.NewGraph(tileWidth, tileHeight, 0, 100, bgColor, bgColor, bgColor)

	// Render title + value in the image, aligned like graph tiles.
	titleText := ""
	if drawTitle {
		titleText = renderedTitle
	}
	g.SetLabel(0, titleText, 19, titleColor)
	g.SetLabelFontSize(0, settingsTitleFontSize)
	g.SetLabel(1, fmt.Sprintf("%dms", intervalMs), 44, textColor)
	g.SetLabelFontSize(1, 10.5)

	// Render and set image
	g.Update(0) // Initialize the graph
	b, err := g.EncodePNG()
	if err != nil {
		log.Printf("updateSettingsTile EncodePNG failed: %v\n", err)
		return
	}
	if err := p.sd.SetImage(context, b); err != nil {
		log.Printf("updateSettingsTile SetImage failed: %v\n", err)
	}

}

func (p *Plugin) renderSettingsPlaceholderTile(intervalMs int, title string, drawTitle bool, titleColor, textColor *color.RGBA) ([]byte, error) {
	if len(p.placeholderImage) == 0 {
		return nil, fmt.Errorf("placeholder image not cached")
	}

	base, err := png.Decode(bytes.NewReader(p.placeholderImage))
	if err != nil {
		return nil, fmt.Errorf("decode placeholder image: %w", err)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, tileWidth, tileHeight))
	draw.Draw(canvas, canvas.Bounds(), base, image.Point{}, draw.Src)

	fontBytes, err := os.ReadFile("DejaVuSans-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	tt, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}

	faceValue := truetype.NewFace(tt, &truetype.Options{Size: 10.5, DPI: 72})
	defer func() {
		if c, ok := faceValue.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}()
	faceTitle := truetype.NewFace(tt, &truetype.Options{Size: settingsTitleFontSize, DPI: 72})
	defer func() {
		if c, ok := faceTitle.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}()

	if drawTitle {
		drawCenteredText(canvas, faceTitle, titleColor, title, 19)
	}
	drawCenteredText(canvas, faceValue, textColor, fmt.Sprintf("%dms", intervalMs), 44)

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("encode placeholder tile: %w", err)
	}
	return out.Bytes(), nil
}

func drawCenteredText(dst *image.RGBA, face font.Face, clr *color.RGBA, text string, baselineY int) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(clr),
		Face: face,
	}
	textWidth := d.MeasureString(text).Round()
	x := (dst.Bounds().Dx() - textWidth) / 2
	if x < 0 {
		x = 0
	}
	d.Dot = fixed.P(x, baselineY)
	d.DrawString(text)
}

// updateAllSettingsTiles updates all settings tiles
func (p *Plugin) updateAllSettingsTiles() {
	for context := range p.settingsContexts {
		p.updateSettingsTile(context)
	}
}

// setPollInterval changes the polling interval dynamically
func (p *Plugin) setPollInterval(intervalMs int) {
	if intervalMs < 100 {
		intervalMs = 100
	}
	if intervalMs > 30000 {
		intervalMs = 30000
	}

	interval := time.Duration(intervalMs) * time.Millisecond

	// Update action manager ticker
	p.am.SetInterval(interval)

	// Update cache TTL
	p.pollTimeCacheTTL = pollTimeCacheTTLForInterval(interval)

	// Save to global settings first so UI/tile render use the latest value immediately.
	p.globalSettings.PollInterval = intervalMs

	// Update all settings tiles
	p.updateAllSettingsTiles()

	// Persist global settings
	if err := p.sd.SetGlobalSettings(p.globalSettings); err != nil {
		log.Printf("SetGlobalSettings failed: %v\n", err)
	}

	log.Printf("Poll interval changed to %v\n", interval)
}
