package lhmstreamdeckplugin

import (
	"encoding/json"
	"hash/fnv"
	"strconv"
	"strings"

	hwsensorsservice "github.com/shayne/lhm-streamdeck/pkg/service"
)

func decodeActionSettings(raw *json.RawMessage) (actionSettings, bool, error) {
	var settings actionSettings
	if raw == nil || *raw == nil {
		return settings, false, nil
	}
	if err := json.Unmarshal(*raw, &settings); err == nil {
		return settings, false, nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(*raw, &rawMap); err != nil {
		return settings, false, err
	}

	migrated := false
	legacyReading := ""
	if v, ok := rawMap["readingId"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			if _, err := strconv.ParseInt(s, 10, 32); err != nil {
				legacyReading = s
			}
		}
	}

	legacySensor := ""
	if v, ok := rawMap["sensorUid"]; ok {
		_ = json.Unmarshal(v, &legacySensor)
	}

	if legacyReading != "" || strings.Contains(legacySensor, ":") {
		sensorID, readingPath := legacySensorReading(legacySensor, legacyReading)
		if sensorID != "" && readingPath != "" {
			migrated = true
			id := makeReadingIDCompat(sensorID, readingPath)
			rawMap["sensorUid"] = mustJSON(strconv.Quote(sensorID))
			rawMap["readingId"] = mustJSON(strconv.Quote(strconv.FormatInt(int64(id), 10)))
			rawMap["isValid"] = mustJSON("true")
		}
	}

	b, err := json.Marshal(rawMap)
	if err != nil {
		return settings, migrated, err
	}
	if err := json.Unmarshal(b, &settings); err != nil {
		return settings, migrated, err
	}
	return settings, migrated, nil
}

func mustJSON(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

func legacySensorReading(sensorUID, readingID string) (string, string) {
	readingPath := strings.TrimSpace(readingID)
	if readingPath == "" {
		if idx := strings.LastIndex(sensorUID, ":"); idx != -1 && idx+1 < len(sensorUID) {
			readingPath = strings.TrimSpace(sensorUID[idx+1:])
		}
	}
	if readingPath == "" {
		return "", ""
	}
	sensorID := sensorIDFromReadingCompat(readingPath)
	if sensorID == "" {
		return "", ""
	}
	return sensorID, readingPath
}

func sensorIDFromReadingCompat(id string) string {
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

func makeReadingIDCompat(sensorID, readingID string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sensorID))
	_, _ = h.Write([]byte(readingID))
	return int32(h.Sum32() & 0x7fffffff)
}

func syncSettingsWithReadings(settings *actionSettings, readings []hwsensorsservice.Reading) bool {
	changed := false
	if settings.ReadingID != 0 {
		for _, r := range readings {
			if r.ID() == settings.ReadingID {
				if settings.ReadingLabel == "" {
					settings.ReadingLabel = r.Label()
					changed = true
				}
				if !settings.IsValid {
					settings.IsValid = true
					changed = true
				}
				return changed
			}
		}
	}
	if settings.ReadingLabel != "" {
		for _, r := range readings {
			if r.Label() == settings.ReadingLabel {
				if settings.ReadingID != r.ID() {
					settings.ReadingID = r.ID()
					changed = true
				}
				if !settings.IsValid {
					settings.IsValid = true
					changed = true
				}
				return changed
			}
		}
	}
	return changed
}
