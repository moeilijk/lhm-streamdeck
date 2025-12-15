package plugin

import (
	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
)

// Plugin adapts the Libre Hardware Monitor service to the gRPC interface.
type Plugin struct {
	Service *Service
}

// PollTime returns the last poll time from LHM.
func (p *Plugin) PollTime() (uint64, error) {
	return p.Service.PollTime()
}

// Sensors returns the list of sensors known to LHM.
func (p *Plugin) Sensors() ([]hwsensorsservice.Sensor, error) {
	ss, err := p.Service.SensorsSnapshot()
	if err != nil {
		return nil, err
	}
	out := make([]hwsensorsservice.Sensor, 0, len(ss))
	for _, s := range ss {
		out = append(out, &sensor{id: s.id, name: s.name})
	}
	return out, nil
}

// ReadingsForSensorID returns readings from a specific sensor.
func (p *Plugin) ReadingsForSensorID(id string) ([]hwsensorsservice.Reading, error) {
	rs, err := p.Service.ReadingsBySensorID(id)
	if err != nil {
		return nil, err
	}
	out := make([]hwsensorsservice.Reading, 0, len(rs))
	for _, r := range rs {
		out = append(out, r)
	}
	return out, nil
}
