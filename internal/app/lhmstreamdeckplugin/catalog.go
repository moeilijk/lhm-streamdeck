package lhmstreamdeckplugin

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

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

	catalog.Presets = buildPresets(catalog.Readings)

	payload := evSendCatalogPayload{Catalog: catalog, Settings: settings}
	return p.sd.SendToPropertyInspector(action, context, payload)
}

type presetSpec struct {
	ID    string
	Name  string
	Score func(*evSendReadingsPayloadReading) int
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func scoreCPUTemperature(reading *evSendReadingsPayloadReading) int {
	if reading == nil {
		return 0
	}

	label := strings.ToLower(reading.Label)
	unit := strings.ToLower(reading.Unit)
	score := 0

	if reading.Category == "cpu" {
		score += 30
	}
	if strings.EqualFold(reading.Type, "Temperature") {
		score += 40
	}
	if containsAny(label, "package", "tctl/tdie", "cpu package", "core max") {
		score += 40
	}
	if containsAny(label, "core", "ccd") {
		score -= 10
	}
	if unit == "°c" || unit == "c" {
		score += 10
	}

	return score
}

func scoreGPULoad(reading *evSendReadingsPayloadReading) int {
	if reading == nil {
		return 0
	}

	label := strings.ToLower(reading.Label)
	unit := strings.ToLower(reading.Unit)
	score := 0

	if reading.Category == "gpu" {
		score += 35
	}
	if strings.EqualFold(reading.Type, "Load") {
		score += 35
	}
	if containsAny(label, "gpu core", "gpu total", "gpu") {
		score += 25
	}
	if containsAny(label, "memory", "fan", "video engine", "copy", "decode", "encode") {
		score -= 20
	}
	if unit == "%" {
		score += 10
	}

	return score
}

func scoreMemoryUsed(reading *evSendReadingsPayloadReading) int {
	if reading == nil {
		return 0
	}

	label := strings.ToLower(reading.Label)
	unit := strings.ToLower(reading.Unit)
	score := 0

	if reading.Category == "memory" {
		score += 35
	}
	if strings.EqualFold(reading.Type, "Data") {
		score += 30
	}
	if containsAny(label, "used", "in use", "memory used") {
		score += 35
	}
	if containsAny(label, "available", "free") {
		score -= 25
	}
	if containsAny(unit, "gb", "mb", "kb", "b") {
		score += 10
	}

	return score
}

func scoreNetworkThroughput(reading *evSendReadingsPayloadReading) int {
	if reading == nil {
		return 0
	}

	label := strings.ToLower(reading.Label)
	unit := strings.ToLower(reading.Unit)
	score := 0

	if reading.Category == "network" {
		score += 30
	}
	if strings.EqualFold(reading.Type, "Throughput") {
		score += 25
	}
	if containsAny(label, "throughput", "speed", "total", "download", "upload", "received", "sent") {
		score += 25
	}
	if containsAny(label, "errors", "drops") {
		score -= 20
	}
	if strings.Contains(unit, "/s") {
		score += 20
	}

	return score
}

func buildPresets(readings []*evSendReadingsPayloadReading) []catalogPreset {
	specs := []presetSpec{
		{ID: "cpu_temperature", Name: "CPU Temperature", Score: scoreCPUTemperature},
		{ID: "gpu_load", Name: "GPU Load", Score: scoreGPULoad},
		{ID: "memory_used", Name: "Memory Used", Score: scoreMemoryUsed},
		{ID: "network_throughput", Name: "Network Throughput", Score: scoreNetworkThroughput},
	}

	const minPresetScore = 50

	presets := make([]catalogPreset, 0, len(specs))
	for _, spec := range specs {
		var best *evSendReadingsPayloadReading
		bestScore := 0

		for _, reading := range readings {
			score := spec.Score(reading)
			if score <= bestScore {
				continue
			}
			best = reading
			bestScore = score
		}

		if best == nil || bestScore < minPresetScore {
			continue
		}

		presets = append(presets, catalogPreset{
			ID:           spec.ID,
			Name:         spec.Name,
			SensorUID:    best.SensorUID,
			SensorName:   best.SensorName,
			ReadingID:    best.ID,
			ReadingLabel: best.Label,
			Category:     best.Category,
		})
	}

	return presets
}

func favoriteID(sensorUID string, readingID int32) string {
	return fmt.Sprintf("%s|%d", sensorUID, readingID)
}

func findSensorName(sensors []hwsensorsservice.Sensor, sensorID string) string {
	for _, sensor := range sensors {
		if sensor.ID() == sensorID {
			return sensor.Name()
		}
	}
	return sensorID
}

func favoriteFromSelection(sensorName string, settings *actionSettings, reading hwsensorsservice.Reading) favoriteReading {
	category := sensorCategory(settings.SensorUID, sensorName)
	return favoriteReading{
		ID:           favoriteID(settings.SensorUID, settings.ReadingID),
		SensorUID:    settings.SensorUID,
		SensorName:   sensorName,
		ReadingID:    settings.ReadingID,
		ReadingLabel: reading.Label(),
		ReadingUnit:  reading.Unit(),
		Category:     category,
	}
}

func findFavoriteByID(favorites []favoriteReading, id string) (favoriteReading, int, bool) {
	for i, favorite := range favorites {
		if favorite.ID == id {
			return favorite, i, true
		}
	}
	return favoriteReading{}, -1, false
}

func sortFavorites(favorites []favoriteReading) {
	sort.SliceStable(favorites, func(i, j int) bool {
		left := favorites[i]
		right := favorites[j]

		if left.Category != right.Category {
			return left.Category < right.Category
		}
		if left.SensorName != right.SensorName {
			return left.SensorName < right.SensorName
		}
		if left.ReadingLabel != right.ReadingLabel {
			return left.ReadingLabel < right.ReadingLabel
		}
		return left.ID < right.ID
	})
}

func (p *Plugin) persistGlobalSettings() error {
	p.mu.RLock()
	gs := p.globalSettings
	p.mu.RUnlock()
	return p.sd.SetGlobalSettings(gs)
}

func (p *Plugin) toggleFavoriteSelection(action, context string, settings *actionSettings) error {
	if settings == nil || settings.SensorUID == "" || settings.ReadingID == 0 || !settings.IsValid {
		return fmt.Errorf("favorite requires a valid sensor and reading selection")
	}

	sensors, err := p.sensorsWithTimeout(2 * time.Second)
	if err != nil {
		return err
	}

	reading, _, err := p.getReading(settings.SensorUID, settings.ReadingID)
	if err != nil {
		return err
	}

	favorite := favoriteFromSelection(findSensorName(sensors, settings.SensorUID), settings, reading)

	p.mu.Lock()
	_, index, exists := findFavoriteByID(p.globalSettings.FavoriteReadings, favorite.ID)
	if exists {
		p.globalSettings.FavoriteReadings = append(
			p.globalSettings.FavoriteReadings[:index],
			p.globalSettings.FavoriteReadings[index+1:]...,
		)
	} else {
		p.globalSettings.FavoriteReadings = append(p.globalSettings.FavoriteReadings, favorite)
		sortFavorites(p.globalSettings.FavoriteReadings)
	}
	p.mu.Unlock()

	if err := p.persistGlobalSettings(); err != nil {
		return err
	}

	return p.sendCatalogToPropertyInspector(action, context, settings, sensors)
}

func (p *Plugin) removeFavorite(action, context string, settings *actionSettings, favoriteID string) error {
	if favoriteID == "" {
		return fmt.Errorf("favorite id is required")
	}

	p.mu.Lock()
	_, index, exists := findFavoriteByID(p.globalSettings.FavoriteReadings, favoriteID)
	if !exists {
		p.mu.Unlock()
		return nil
	}
	p.globalSettings.FavoriteReadings = append(
		p.globalSettings.FavoriteReadings[:index],
		p.globalSettings.FavoriteReadings[index+1:]...,
	)
	p.mu.Unlock()

	if err := p.persistGlobalSettings(); err != nil {
		return err
	}

	sensors, err := p.sensorsWithTimeout(2 * time.Second)
	if err != nil {
		return err
	}

	return p.sendCatalogToPropertyInspector(action, context, settings, sensors)
}
