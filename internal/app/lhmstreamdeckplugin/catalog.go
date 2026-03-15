package lhmstreamdeckplugin

import (
	"fmt"
	"log"
	"strings"

	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
)

func sensorCategory(sensorID, sensorName string) string {
	value := strings.ToLower(sensorID + " " + sensorName)

	switch {
	case strings.Contains(value, "/amdcpu"),
		strings.Contains(value, "/intelcpu"),
		strings.Contains(value, "/cpu"),
		strings.Contains(value, "ryzen"),
		strings.Contains(value, "threadripper"),
		strings.Contains(value, "core i"),
		strings.Contains(value, "xeon"):
		return "cpu"
	case strings.Contains(value, "/gpu"),
		strings.Contains(value, "nvidia"),
		strings.Contains(value, "geforce"),
		strings.Contains(value, "radeon"),
		strings.Contains(value, "rtx"),
		strings.Contains(value, "gtx"),
		strings.Contains(value, "arc"),
		strings.Contains(value, "gpu"):
		return "gpu"
	case strings.Contains(value, "/ram"),
		strings.Contains(value, "/memory"),
		strings.Contains(value, "memory"),
		strings.Contains(value, "ram"):
		return "memory"
	case strings.Contains(value, "/hdd"),
		strings.Contains(value, "/ssd"),
		strings.Contains(value, "/nvme"),
		strings.Contains(value, "/storage"),
		strings.Contains(value, "disk"),
		strings.Contains(value, "drive"),
		strings.Contains(value, "ssd"),
		strings.Contains(value, "hdd"),
		strings.Contains(value, "nvme"):
		return "disk"
	case strings.Contains(value, "/nic"),
		strings.Contains(value, "/network"),
		strings.Contains(value, "network"),
		strings.Contains(value, "ethernet"),
		strings.Contains(value, "wireless"),
		strings.Contains(value, "wifi"),
		strings.Contains(value, "wi-fi"),
		strings.Contains(value, "wlan"),
		strings.Contains(value, "lan"):
		return "network"
	case strings.Contains(value, "/lpc"),
		strings.Contains(value, "/mainboard"),
		strings.Contains(value, "motherboard"),
		strings.Contains(value, "mainboard"),
		strings.Contains(value, "chipset"),
		strings.Contains(value, "superio"),
		strings.Contains(value, "nuvoton"),
		strings.Contains(value, "ite"):
		return "motherboard"
	default:
		return "other"
	}
}

func searchText(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " ")
}

func sensorPayload(sensorID, sensorName string) *evSendSensorsPayloadSensor {
	category := sensorCategory(sensorID, sensorName)
	return &evSendSensorsPayloadSensor{
		UID:        sensorID,
		Name:       sensorName,
		Category:   category,
		SearchText: searchText(sensorName, category, sensorID),
	}
}

func readingPayload(sensorID, sensorName string, reading hwsensorsservice.Reading) *evSendReadingsPayloadReading {
	category := sensorCategory(sensorID, sensorName)
	return &evSendReadingsPayloadReading{
		ID:         reading.ID(),
		Label:      reading.Label(),
		Prefix:     reading.Unit(),
		Unit:       reading.Unit(),
		Type:       reading.Type(),
		SensorUID:  sensorID,
		SensorName: sensorName,
		Category:   category,
		SearchText: searchText(sensorName, category, reading.Type(), reading.Label(), reading.Unit()),
	}
}

func (p *Plugin) favoriteReadingsSnapshot() []favoriteReading {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.globalSettings.FavoriteReadings) == 0 {
		return nil
	}

	out := make([]favoriteReading, len(p.globalSettings.FavoriteReadings))
	copy(out, p.globalSettings.FavoriteReadings)
	return out
}

func (p *Plugin) sendCatalogToPropertyInspector(action, context string, settings *actionSettings, sensors []hwsensorsservice.Sensor) error {
	p.hwMu.RLock()
	hw := p.hw
	p.hwMu.RUnlock()
	if hw == nil {
		return fmt.Errorf("LHM bridge not ready")
	}

	catalog := &evSendCatalogPayloadCatalog{
		Sensors:   make([]*evSendSensorsPayloadSensor, 0, len(sensors)),
		Readings:  make([]*evSendReadingsPayloadReading, 0),
		Favorites: p.favoriteReadingsSnapshot(),
	}

	for _, sensor := range sensors {
		sensorID := sensor.ID()
		sensorName := sensor.Name()
		catalog.Sensors = append(catalog.Sensors, sensorPayload(sensorID, sensorName))

		readings, err := hw.ReadingsForSensorID(sensorID)
		if err != nil {
			log.Printf("sendCatalogToPropertyInspector ReadingsForSensorID sensor=%s: %v\n", sensorID, err)
			continue
		}

		for _, reading := range readings {
			catalog.Readings = append(catalog.Readings, readingPayload(sensorID, sensorName, reading))
		}
	}

	payload := evSendCatalogPayload{Catalog: catalog, Settings: settings}
	return p.sd.SendToPropertyInspector(action, context, payload)
}
