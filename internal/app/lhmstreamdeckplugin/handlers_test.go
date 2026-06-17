package lhmstreamdeckplugin

import (
	"image/color"
	"testing"

	"github.com/moeilijk/lhm-streamdeck/pkg/graph"
	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
)

func TestApplyReadingSettingsUsesSelectedSourceProfile(t *testing.T) {
	const (
		profileID = "remote"
		sensorUID = "/cpu"
		readingID = int32(42)
	)

	p := &Plugin{
		sources: make(map[string]*sourceRuntime),
		graphs: map[string]*graph.Graph{
			"ctx": graph.NewGraph(
				72, 72, 0, 100,
				&color.RGBA{255, 255, 255, 255},
				&color.RGBA{0, 0, 0, 255},
				&color.RGBA{255, 0, 0, 255},
			),
		},
		globalSettings: globalSettings{
			SourceProfiles: []lhmSourceProfile{
				{ID: profileID, Name: "Remote", Host: "10.0.0.2", Port: 8085},
			},
			DefaultSourceProfileID: "",
		},
	}
	p.sources[profileID] = &sourceRuntime{
		profile: lhmSourceProfile{ID: profileID, Name: "Remote", Host: "10.0.0.2", Port: 8085},
		hw: stubHardwareService{
			readingsBySensor: map[string][]hwsensorsservice.Reading{
				sensorUID: {
					stubReading{id: readingID, typ: "Load", label: "CPU Total", unit: "%"},
				},
			},
		},
	}

	settings := &actionSettings{
		SourceProfileID: profileID,
		SensorUID:       sensorUID,
		ReadingID:       readingID,
	}

	if err := p.applyReadingSettings("ctx", settings); err != nil {
		t.Fatalf("applyReadingSettings() error = %v", err)
	}
	if settings.ReadingLabel != "CPU Total" {
		t.Fatalf("ReadingLabel = %q, want %q", settings.ReadingLabel, "CPU Total")
	}
	if !settings.IsValid {
		t.Fatalf("IsValid = false, want true")
	}
}

func TestRuntimeForSourceReconcilesLoadedSourceProfile(t *testing.T) {
	const profileID = "companion_wsl"

	p := &Plugin{
		sources: make(map[string]*sourceRuntime),
		globalSettings: globalSettings{
			SourceProfiles: []lhmSourceProfile{
				{ID: profileID, Name: "Companion WSL", Host: "172.18.175.238", Port: 8085},
			},
			DefaultSourceProfileID: profileID,
		},
	}
	p.sources[profileID] = &sourceRuntime{
		profile: lhmSourceProfile{ID: profileID},
		hw:      stubHardwareService{},
	}

	rt := p.runtimeForSource(profileID)

	rt.mu.RLock()
	profile := rt.profile
	hw := rt.hw
	rt.mu.RUnlock()

	if profile.Host != "172.18.175.238" || profile.Port != 8085 {
		t.Fatalf("runtime profile = %+v, want companion HTTP endpoint", profile)
	}
	if hw != nil {
		t.Fatalf("runtime hardware service was kept after endpoint reconciliation")
	}
}

type stubHardwareService struct {
	readingsBySensor map[string][]hwsensorsservice.Reading
}

func (s stubHardwareService) PollTime() (uint64, error) {
	return 1, nil
}

func (s stubHardwareService) Sensors() ([]hwsensorsservice.Sensor, error) {
	return nil, nil
}

func (s stubHardwareService) ReadingsForSensorID(id string) ([]hwsensorsservice.Reading, error) {
	return s.readingsBySensor[id], nil
}
