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
