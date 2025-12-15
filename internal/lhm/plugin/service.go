package plugin

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
)

const (
	defaultEndpoint   = "http://127.0.0.1:8085/data.json"
	defaultPollPeriod = time.Second
)

// node mirrors the structure returned by LHM's data.json endpoint.
type node struct {
	Text     string `json:"Text"`
	Min      string `json:"Min"`
	Value    string `json:"Value"`
	Max      string `json:"Max"`
	SensorID string `json:"SensorId"`
	Type     string `json:"Type"`
	ImageURL string `json:"ImageURL"`
	Children []node `json:"Children"`
}

// reading matches the exposed hardware reading.
type reading struct {
	id      int32
	label   string
	unit    string
	typ     string
	typeI   hwsensorsservice.ReadingType
	value   float64
	min     float64
	max     float64
	average float64
}

func (r *reading) ID() int32                { return r.id }
func (r *reading) TypeI() int32             { return int32(r.typeI) }
func (r *reading) Type() string             { return r.typ }
func (r *reading) Label() string            { return r.label }
func (r *reading) Unit() string             { return r.unit }
func (r *reading) Value() float64           { return r.value }
func (r *reading) ValueMin() float64        { return r.min }
func (r *reading) ValueMax() float64        { return r.max }
func (r *reading) ValueAvg() float64        { return r.average }

type sensor struct {
	id   string
	name string
}

func (s *sensor) ID() string   { return s.id }
func (s *sensor) Name() string { return s.name }

// Service polls Libre Hardware Monitor and provides cached sensor data.
type Service struct {
	url          string
	client       *http.Client
	pollInterval time.Duration

	mu          sync.RWMutex
	pollTime    uint64
	sensors     map[string]*sensor
	sensorOrder []string
	readings    map[string][]*reading
	ready       bool
}

// StartService initializes the Libre Hardware Monitor bridge.
func StartService() *Service {
	url := os.Getenv("LHM_ENDPOINT")
	if url == "" {
		url = defaultEndpoint
	}
	return &Service{
		url:          url,
		client:       &http.Client{Timeout: 2 * time.Second},
		pollInterval: defaultPollPeriod,
	}
}

// Recv pulls the latest snapshot from Libre Hardware Monitor.
func (s *Service) Recv() error {
	defer time.Sleep(s.pollInterval)

	resp, err := s.client.Get(s.url)
	if err != nil {
		return fmt.Errorf("request LHM data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request LHM data: status %s", resp.Status)
	}

	var root node
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return fmt.Errorf("decode LHM response: %w", err)
	}

	sensors, order, readings := buildSnapshot(&root)

	s.mu.Lock()
	s.pollTime = uint64(time.Now().UnixNano())
	s.sensors = sensors
	s.sensorOrder = order
	s.readings = readings
	s.ready = true
	s.mu.Unlock()

	return nil
}

// PollTime returns the last time we updated the cache.
func (s *Service) PollTime() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return 0, fmt.Errorf("LHM data unavailable")
	}
	return s.pollTime, nil
}

// SensorsSnapshot returns the currently cached sensors.
func (s *Service) SensorsSnapshot() ([]*sensor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready || len(s.sensorOrder) == 0 {
		return nil, fmt.Errorf("LHM data unavailable")
	}
	out := make([]*sensor, 0, len(s.sensorOrder))
	for _, id := range s.sensorOrder {
		out = append(out, &sensor{
			id:   id,
			name: s.sensors[id].name,
		})
	}
	return out, nil
}

// ReadingsBySensorID returns readings associated with the provided sensor id.
func (s *Service) ReadingsBySensorID(id string) ([]*reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return nil, fmt.Errorf("LHM data unavailable")
	}
	rs, ok := s.readings[id]
	if !ok {
		return nil, fmt.Errorf("sensor %s not found", id)
	}
	out := make([]*reading, len(rs))
	copy(out, rs)
	return out, nil
}

func buildSnapshot(root *node) (map[string]*sensor, []string, map[string][]*reading) {
	sensors := make(map[string]*sensor)
	sensorOrder := make([]string, 0)
	readings := make(map[string][]*reading)

	var walk func(n *node, ancestors []*node)
	walk = func(n *node, ancestors []*node) {
		if n.SensorID != "" {
			sid := sensorIDFromReading(n.SensorID)
			if sid == "" {
				sid = n.SensorID
			}
			if _, ok := sensors[sid]; !ok {
				name := determineSensorName(ancestors)
				sensors[sid] = &sensor{id: sid, name: name}
				sensorOrder = append(sensorOrder, sid)
			}
			if r := newReading(sid, n); r != nil {
				readings[sid] = append(readings[sid], r)
			}
			return
		}
		nextAncestors := append(ancestors, n)
		for i := range n.Children {
			walk(&n.Children[i], nextAncestors)
		}
	}

	walk(root, nil)

	return sensors, sensorOrder, readings
}

func sensorIDFromReading(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	trimmed := strings.Trim(id, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return "/" + parts[0]
	case 2:
		return "/" + parts[0]
	default:
		return "/" + strings.Join(parts[:len(parts)-2], "/")
	}
}

func determineSensorName(ancestors []*node) string {
	for i := len(ancestors) - 1; i >= 0; i-- {
		if isDeviceNode(ancestors[i]) && ancestors[i].Text != "" {
			return ancestors[i].Text
		}
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if ancestors[i].Text != "" {
			return ancestors[i].Text
		}
	}
	return "Unknown Sensor"
}

func isDeviceNode(n *node) bool {
	if n == nil || len(n.Children) == 0 {
		return false
	}
	for i := range n.Children {
		if len(n.Children[i].Children) > 0 {
			return true
		}
	}
	return false
}

func newReading(sensorID string, n *node) *reading {
	val, unit := parseValue(n.Value)
	min, _ := parseValue(n.Min)
	max, _ := parseValue(n.Max)

	rt := mapReadingType(n.Type)

	return &reading{
		id:      makeReadingID(sensorID, n.SensorID),
		label:   n.Text,
		unit:    unit,
		typ:     n.Type,
		typeI:   rt,
		value:   val,
		min:     min,
		max:     max,
		average: val,
	}
}

func parseValue(v string) (float64, string) {
	v = strings.TrimSpace(v)
	if v == "" || v == "-" {
		return 0, ""
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return 0, ""
	}
	num := strings.ReplaceAll(fields[0], ",", ".")
	num = strings.TrimSpace(num)
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, strings.TrimSpace(strings.TrimPrefix(v, fields[0]))
	}
	unit := strings.TrimSpace(strings.TrimPrefix(v, fields[0]))
	return f, unit
}

func makeReadingID(sensorID, readingID string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sensorID))
	_, _ = h.Write([]byte(readingID))
	return int32(h.Sum32() & 0x7fffffff)
}

func mapReadingType(t string) hwsensorsservice.ReadingType {
	switch strings.ToLower(t) {
	case "temperature":
		return hwsensorsservice.ReadingTypeTemp
	case "voltage":
		return hwsensorsservice.ReadingTypeVolt
	case "fan":
		return hwsensorsservice.ReadingTypeFan
	case "power":
		return hwsensorsservice.ReadingTypePower
	case "clock":
		return hwsensorsservice.ReadingTypeClock
	case "current":
		return hwsensorsservice.ReadingTypeCurrent
	case "load", "control", "level":
		return hwsensorsservice.ReadingTypeUsage
	default:
		return hwsensorsservice.ReadingTypeOther
	}
}
