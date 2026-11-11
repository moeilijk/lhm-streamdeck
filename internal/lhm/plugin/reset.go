package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ResetReadingMinMax resolves the plugin's reading ID to the exact LHM sensor
// path using the same collision handling as polling, then resets only that sensor.
func ResetReadingMinMax(endpoint, sensorID string, readingID int32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	var root node
	err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&root)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("reading catalog: HTTP %d", response.StatusCode)
	}
	if err != nil {
		return err
	}
	_, _, readings := buildSnapshot(&root)
	var path string
	for _, r := range readings[sensorID] {
		if r.id == readingID {
			path = r.path
			break
		}
	}
	if path == "" {
		return fmt.Errorf("selected reading no longer exists")
	}
	matches := 0
	for _, r := range readings[sensorID] {
		if r.path == path {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("LHM exposes duplicate sensor paths; refusing an ambiguous reset")
	}
	target, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	target.Path = "/Sensor"
	target.RawPath = ""
	target.RawQuery = url.Values{"action": {"ResetMinMax"}, "id": {path}}.Encode()
	request, err = http.NewRequestWithContext(ctx, http.MethodPost, target.String(), nil)
	if err != nil {
		return err
	}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var result struct {
		Result  string `json:"result"`
		Message string `json:"message"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("reset: HTTP %d", response.StatusCode)
	}
	if err != nil {
		return err
	}
	if result.Result != "ok" {
		return fmt.Errorf("LHM reset failed: %s", result.Message)
	}
	return nil
}
