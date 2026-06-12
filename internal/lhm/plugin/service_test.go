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

func TestBuildSnapshotGroupsCompanionStyleSensorIDs(t *testing.T) {
	root := node{
		Text: "Computer",
		Children: []node{
			{
				Text: "Memory",
				Children: []node{
					{Text: "Load", Children: []node{
						{Text: "Memory", Value: "32,3 %", SensorID: "/memory/load/0/0", Type: "Load"},
						{Text: "Swap", Value: "0,0 %", SensorID: "/memory/load/1/0", Type: "Load"},
					}},
					{Text: "Data", Children: []node{
						{Text: "Used Memory", Value: "4,9 GB", SensorID: "/memory/data/0/0", Type: "Data"},
					}},
				},
			},
			{
				Text: "Network",
				Children: []node{
					{Text: "eth0", Children: []node{
						{Text: "Throughput", Children: []node{
							{Text: "Receive", Value: "1,0 B/s", SensorID: "/network/eth0/throughput/0/0", Type: "Throughput"},
						}},
						{Text: "Data", Children: []node{
							{Text: "Received Total", Value: "1,0 B", SensorID: "/network/eth0/data/0/0", Type: "Data"},
						}},
					}},
				},
			},
			{
				Text: "NVIDIA GeForce RTX 4070 Ti SUPER",
				Children: []node{
					{Text: "Temperatures", Children: []node{
						{Text: "GPU Core", Value: "35,0 °C", SensorID: "/nvidia/0/temperature/0/0", Type: "Temperature"},
					}},
					{Text: "Load", Children: []node{
						{Text: "GPU Core", Value: "10,0 %", SensorID: "/nvidia/0/load/0/0", Type: "Load"},
					}},
				},
			},
		},
	}

	sensors, order, readings := buildSnapshot(&root)
	if len(order) != 3 {
		t.Fatalf("expected grouped sensors, got order=%v sensors=%d", order, len(sensors))
	}
	for _, id := range []string{"/memory", "/network/eth0", "/nvidia/0"} {
		if _, ok := sensors[id]; !ok {
			t.Fatalf("expected grouped sensor %s in %v", id, order)
		}
		if len(readings[id]) == 0 {
			t.Fatalf("expected readings for grouped sensor %s", id)
		}
	}
	if got := sensors["/memory"].Name(); got != "Memory" {
		t.Fatalf("memory name = %q, want Memory", got)
	}
	if got := sensors["/network/eth0"].Name(); got != "eth0" {
		t.Fatalf("network name = %q, want eth0", got)
	}
	if got := sensors["/nvidia/0"].Name(); got != "NVIDIA GeForce RTX 4070 Ti SUPER" {
		t.Fatalf("gpu name = %q", got)
	}
}
