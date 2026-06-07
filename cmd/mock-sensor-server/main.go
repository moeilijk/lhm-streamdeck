// mock-sensor-server serves /data.json in LHM format with controllable sensor values.
// POST /set {"path":"/mockcpu/0/temperature/0","value":85.0} to change a reading.
// POST /reset to restore all defaults.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

type node struct {
	Text     string  `json:"Text"`
	Min      string  `json:"Min"`
	Value    string  `json:"Value"`
	Max      string  `json:"Max"`
	SensorId string  `json:"SensorId"`
	Type     string  `json:"Type"`
	ImageURL string  `json:"ImageURL"`
	Children []*node `json:"Children"`
}

type reading struct {
	path     string
	unit     string
	typ      string
	defVal   float64
	min      float64
	max      float64
	cur      float64
}

var (
	mu       sync.RWMutex
	readings map[string]*reading
)

func defaultReadings() map[string]*reading {
	return map[string]*reading{
		"/mockcpu/0/temperature/0": {path: "/mockcpu/0/temperature/0", unit: "C", typ: "Temperature", defVal: 45, min: 20, max: 100, cur: 45},
		"/mockcpu/0/temperature/1": {path: "/mockcpu/0/temperature/1", unit: "C", typ: "Temperature", defVal: 42, min: 20, max: 100, cur: 42},
		"/mockcpu/0/load/0":        {path: "/mockcpu/0/load/0", unit: "%", typ: "Load", defVal: 20, min: 0, max: 100, cur: 20},
		"/mockgpu/0/temperature/0": {path: "/mockgpu/0/temperature/0", unit: "C", typ: "Temperature", defVal: 55, min: 20, max: 100, cur: 55},
		"/mockgpu/0/load/0":        {path: "/mockgpu/0/load/0", unit: "%", typ: "Load", defVal: 35, min: 0, max: 100, cur: 35},
		"/mocksys/0/voltage/0":     {path: "/mocksys/0/voltage/0", unit: "V", typ: "Voltage", defVal: 1.2, min: 0.8, max: 1.6, cur: 1.2},
		"/mocksys/0/fan/0":         {path: "/mocksys/0/fan/0", unit: "RPM", typ: "Fan", defVal: 1200, min: 0, max: 3000, cur: 1200},
	}
}

func fmtVal(v float64, unit string) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if unit != "" {
		return s + " " + unit
	}
	return s
}

func buildTree() *node {
	mu.RLock()
	r := readings
	mu.RUnlock()

	get := func(path string) *reading {
		if v, ok := r[path]; ok {
			return v
		}
		return &reading{unit: "", cur: 0, min: 0, max: 0}
	}

	// Mock CPU sensor
	cpuTemp0 := get("/mockcpu/0/temperature/0")
	cpuTemp1 := get("/mockcpu/0/temperature/1")
	cpuLoad := get("/mockcpu/0/load/0")

	// Mock GPU sensor
	gpuTemp := get("/mockgpu/0/temperature/0")
	gpuLoad := get("/mockgpu/0/load/0")

	// Mock System sensor
	voltage := get("/mocksys/0/voltage/0")
	fan := get("/mocksys/0/fan/0")

	return &node{
		Text: "root", Children: []*node{
			{Text: "Mock CPU", Children: []*node{
				{Text: "Temperatures", Children: []*node{
					{Text: "CPU Package", SensorId: "/mockcpu/0/temperature/0", Type: "Temperature",
						Value: fmtVal(cpuTemp0.cur, cpuTemp0.unit), Min: fmtVal(cpuTemp0.min, cpuTemp0.unit), Max: fmtVal(cpuTemp0.max, cpuTemp0.unit), Children: []*node{}},
					{Text: "CPU Core 0", SensorId: "/mockcpu/0/temperature/1", Type: "Temperature",
						Value: fmtVal(cpuTemp1.cur, cpuTemp1.unit), Min: fmtVal(cpuTemp1.min, cpuTemp1.unit), Max: fmtVal(cpuTemp1.max, cpuTemp1.unit), Children: []*node{}},
				}},
				{Text: "Load", Children: []*node{
					{Text: "CPU Total", SensorId: "/mockcpu/0/load/0", Type: "Load",
						Value: fmtVal(cpuLoad.cur, cpuLoad.unit), Min: fmtVal(cpuLoad.min, cpuLoad.unit), Max: fmtVal(cpuLoad.max, cpuLoad.unit), Children: []*node{}},
				}},
			}},
			{Text: "Mock GPU", Children: []*node{
				{Text: "Temperatures", Children: []*node{
					{Text: "GPU Core", SensorId: "/mockgpu/0/temperature/0", Type: "Temperature",
						Value: fmtVal(gpuTemp.cur, gpuTemp.unit), Min: fmtVal(gpuTemp.min, gpuTemp.unit), Max: fmtVal(gpuTemp.max, gpuTemp.unit), Children: []*node{}},
				}},
				{Text: "Load", Children: []*node{
					{Text: "GPU Total", SensorId: "/mockgpu/0/load/0", Type: "Load",
						Value: fmtVal(gpuLoad.cur, gpuLoad.unit), Min: fmtVal(gpuLoad.min, gpuLoad.unit), Max: fmtVal(gpuLoad.max, gpuLoad.unit), Children: []*node{}},
				}},
			}},
			{Text: "Mock System", Children: []*node{
				{Text: "Voltages", Children: []*node{
					{Text: "CPU Core", SensorId: "/mocksys/0/voltage/0", Type: "Voltage",
						Value: fmtVal(voltage.cur, voltage.unit), Min: fmtVal(voltage.min, voltage.unit), Max: fmtVal(voltage.max, voltage.unit), Children: []*node{}},
				}},
				{Text: "Fans", Children: []*node{
					{Text: "CPU Fan", SensorId: "/mocksys/0/fan/0", Type: "Fan",
						Value: fmtVal(fan.cur, fan.unit), Min: fmtVal(fan.min, fan.unit), Max: fmtVal(fan.max, fan.unit), Children: []*node{}},
				}},
			}},
		},
	}
}

func handleDataJSON(w http.ResponseWriter, r *http.Request) {
	tree := buildTree()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tree)
}

func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path  string  `json:"path"`
		Value float64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mu.Lock()
	rd, ok := readings[req.Path]
	if !ok {
		mu.Unlock()
		http.Error(w, "unknown path: "+req.Path, http.StatusNotFound)
		return
	}
	rd.cur = req.Value
	mu.Unlock()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"ok":true,"path":%q,"value":%g}`, req.Path, req.Value)
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	readings = defaultReadings()
	mu.Unlock()
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"ok":true}`)
}

func handleList(w http.ResponseWriter, _ *http.Request) {
	mu.RLock()
	out := make([]map[string]interface{}, 0, len(readings))
	for path, rd := range readings {
		out = append(out, map[string]interface{}{
			"path":    path,
			"type":    rd.typ,
			"unit":    rd.unit,
			"current": rd.cur,
			"default": rd.defVal,
		})
	}
	mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func main() {
	port := flag.Int("port", 9999, "HTTP port to serve on")
	flag.Parse()

	mu.Lock()
	readings = defaultReadings()
	mu.Unlock()

	http.HandleFunc("/data.json", handleDataJSON)
	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/reset", handleReset)
	http.HandleFunc("/list", handleList)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("mock-sensor-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
