package lhmstreamdeckplugin

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/shayne/go-winpeg"
	"github.com/shayne/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
	"github.com/shayne/lhm-streamdeck/pkg/streamdeck"
)

// Plugin handles information between Libre Hardware Monitor and Stream Deck
type Plugin struct {
	c      *plugin.Client
	peg    winpeg.ProcessExitGroup
	hw     hwsensorsservice.HardwareService
	sd     *streamdeck.StreamDeck
	am     *actionManager
	graphs map[string]*graph.Graph
}

type sensorResult struct {
	sensors []hwsensorsservice.Sensor
	err     error
}

func (p *Plugin) sensorsWithTimeout(d time.Duration) ([]hwsensorsservice.Sensor, error) {
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
	if err != nil {
		return err
	}

	if err := g.AddProcess(cmd.Process); err != nil {
		return err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("lhmplugin")
	if err != nil {
		return err
	}

	p.c = client
	p.peg = g
	p.hw = raw.(hwsensorsservice.HardwareService)

	return nil
}

// NewPlugin creates an instance and initializes the plugin
func NewPlugin(port, uuid, event, info string) (*Plugin, error) {
	p := &Plugin{
		am:     newActionManager(),
		graphs: make(map[string]*graph.Graph),
	}
	p.startClient()
	p.sd = streamdeck.NewStreamDeck(port, uuid, event, info)
	return p, nil
}

// RunForever starts the plugin and waits for events, indefinitely
func (p *Plugin) RunForever() error {
	defer func() {
		p.c.Kill()
		p.peg.Dispose()
	}()

	p.sd.SetDelegate(p)
	p.am.Run(p.updateTiles)

	go func() {
		for {
			if p.c.Exited() {
				p.startClient()
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

func (p *Plugin) getReading(suid string, rid int32) (hwsensorsservice.Reading, error) {
	rbs, err := p.hw.ReadingsForSensorID(suid)
	if err != nil {
		return nil, fmt.Errorf("getReading ReadingsBySensor failed: %v", err)
	}
	for _, r := range rbs {
		if r.ID() == rid {
			return r, nil
		}
	}
	return nil, fmt.Errorf("ReadingID does not exist: %s", suid)
}

func (p *Plugin) applyDefaultFormat(v float64, t hwsensorsservice.ReadingType, u string) string {
	switch t {
	case hwsensorsservice.ReadingTypeNone:
		return fmt.Sprintf("%0.f %s", v, u)
	case hwsensorsservice.ReadingTypeTemp:
		return fmt.Sprintf("%.0f %s", v, u)
	case hwsensorsservice.ReadingTypeVolt:
		return fmt.Sprintf("%.0f %s", v, u)
	case hwsensorsservice.ReadingTypeFan:
		return fmt.Sprintf("%.0f %s", v, u)
	case hwsensorsservice.ReadingTypeCurrent:
		return fmt.Sprintf("%.0f %s", v, u)
	case hwsensorsservice.ReadingTypePower:
		return fmt.Sprintf("%0.f %s", v, u)
	case hwsensorsservice.ReadingTypeClock:
		return fmt.Sprintf("%.0f %s", v, u)
	case hwsensorsservice.ReadingTypeUsage:
		return fmt.Sprintf("%.0f%s", v, u)
	case hwsensorsservice.ReadingTypeOther:
		return fmt.Sprintf("%.0f %s", v, u)
	}
	return "Bad Format"
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
		}
		bts, err := ioutil.ReadFile("./launch-lhm.png")
		if err != nil {
			log.Printf("Failed to read launch-lhm.png: %v\n", err)
			return
		}
		err = p.sd.SetImage(data.context, bts)
		if err != nil {
			log.Printf("Failed to setImage: %v\n", err)
		}
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

	pollTime, err := p.hw.PollTime()
	if err != nil {
		log.Printf("PollTime failed: %v\n", err)
		showUnavailable()
		return
	}
	if pollTime == 0 || time.Since(time.Unix(0, int64(pollTime))) > 5*time.Second {
		showUnavailable()
		return
	}

	s := data.settings
	r, err := p.getReading(s.SensorUID, s.ReadingID)
	if err != nil {
		if s.ReadingLabel != "" {
			readings, rerr := p.hw.ReadingsForSensorID(s.SensorUID)
			if rerr == nil {
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
	if s.Divisor != "" {
		fdiv := 1.
		fdiv, err := strconv.ParseFloat(s.Divisor, 64)
		if err != nil {
			log.Printf("Failed to parse float: %s\n", s.Divisor)
			return
		}
		v = r.Value() / fdiv
	}

	// Normalize the graph value to handle unit changes (e.g., KB/s → MB/s)
	// Only applies to throughput readings (units containing "/s")
	graphValue := p.normalizeForGraph(r.Value(), r.Unit(), s.GraphUnit)
	if s.Divisor != "" {
		fdiv, _ := strconv.ParseFloat(s.Divisor, 64)
		graphValue = graphValue / fdiv
	}
	g.Update(graphValue)

	// Check threshold alerts (evaluate by priority, highest first)
	activeThreshold := p.evaluateThresholds(v, s.Thresholds)

	newThresholdID := ""
	if activeThreshold != nil {
		newThresholdID = activeThreshold.ID
	}

	// Check if forced re-evaluation or state transition
	forceUpdate := s.CurrentThresholdID == "_FORCE_REEVALUATE_"
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
