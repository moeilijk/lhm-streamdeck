package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSnapshotFromExample(t *testing.T) {
	examplePath := filepath.Join("..", "..", "..", "example.json")
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example.json: %v", err)
	}

	var root node
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal example.json: %v", err)
	}

	sensors, order, readings := buildSnapshot(&root)
	if len(sensors) == 0 {
		t.Fatalf("expected sensors, got none")
	}
	if len(order) != len(sensors) {
		t.Fatalf("sensor order mismatch: %d vs %d", len(order), len(sensors))
	}

	cpu, ok := sensors["/amdcpu/0"]
	if !ok {
		t.Fatalf("expected cpu sensor id")
	}
	if cpu.Name() == "" {
		t.Fatalf("cpu sensor missing name")
	}
	if cpu.Name() != "AMD Ryzen 7 9800X3D" {
		t.Fatalf("unexpected cpu sensor name: %s", cpu.Name())
	}

	cpuReadings := readings["/amdcpu/0"]
	if len(cpuReadings) == 0 {
		t.Fatalf("expected cpu readings")
	}
	found := false
	for _, r := range cpuReadings {
		if r.Label() == "CPU Total" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected CPU Total reading")
	}
}

// TestBuildSnapshotDisambiguatesDuplicateSensorID covers LibreHardwareMonitor
// bug #1441: NVIDIA exposes "GPU Memory" and "GPU Bus" with an identical
// SensorId (/gpu-nvidia/0/load/3). Both readings must still get distinct ids so
// each is independently selectable, and the first occurrence must keep its
// canonical id so already-saved button settings keep resolving.
func TestBuildSnapshotDisambiguatesDuplicateSensorID(t *testing.T) {
	root := node{
		Text: "Computer",
		Children: []node{
			{
				Text: "NVIDIA GeForce RTX 3090 Ti",
				Children: []node{
					{Text: "Load", Children: []node{
						{Text: "GPU Core", Value: "10,0 %", SensorID: "/gpu-nvidia/0/load/0", Type: "Load"},
						{Text: "GPU Memory", Value: "20,0 %", SensorID: "/gpu-nvidia/0/load/3", Type: "Load"},
						{Text: "GPU Bus", Value: "30,0 %", SensorID: "/gpu-nvidia/0/load/3", Type: "Load"},
					}},
				},
			},
		},
	}

	_, _, readings := buildSnapshot(&root)
	rs := readings["/gpu-nvidia/0"]
	if len(rs) != 3 {
		t.Fatalf("expected 3 readings, got %d", len(rs))
	}

	byID := make(map[int32]string)
	byLabel := make(map[string]int32)
	for _, r := range rs {
		if prev, ok := byID[r.id]; ok {
			t.Fatalf("duplicate reading id %d shared by %q and %q", r.id, prev, r.label)
		}
		byID[r.id] = r.label
		byLabel[r.label] = r.id
	}

	// First occurrence keeps the canonical id (backward compatible).
	canonical := makeReadingID("/gpu-nvidia/0", "/gpu-nvidia/0/load/3")
	if byLabel["GPU Memory"] != canonical {
		t.Fatalf("GPU Memory should keep canonical id %d, got %d", canonical, byLabel["GPU Memory"])
	}
	if byLabel["GPU Bus"] == canonical {
		t.Fatalf("GPU Bus must be disambiguated from the canonical id %d", canonical)
	}

	// Disambiguation must be stable across snapshots.
	_, _, readings2 := buildSnapshot(&root)
	for _, r := range readings2["/gpu-nvidia/0"] {
		if byLabel[r.label] != r.id {
			t.Fatalf("unstable id for %q: %d vs %d", r.label, byLabel[r.label], r.id)
		}
	}
}
