package lhmstreamdeckplugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moeilijk/lhm-streamdeck/pkg/streamdeck"
)

// Stream Deck replaces the stored global settings on every setGlobalSettings
// (measured on Stream Deck 7.6.1, from the plugin and from a Property
// Inspector) and hands the plugin only what was written. These tests pin that
// a partial object never erases the rest, and that every write by the plugin
// carries the complete settings, Derived presets included.

const fullGlobalSettings = `{
 "pollInterval": 2000,
 "sourceProfiles": [
  {"id":"default","name":"This PC","host":"127.0.0.1","port":8085},
  {"id":"source_1","name":"PC 2","host":"192.168.1.20","port":8085}
 ],
 "defaultSourceProfileId": "default",
 "favoriteReadings": [{"id":"f1","sourceProfileId":"source_1","sensorUid":"/gpu-nvidia/0","sensorName":"GPU","readingId":"3","readingLabel":"GPU Core","readingUnit":"°C"}],
 "globalThresholds": [{"id":"g1","name":"Hot","enabled":true,"operator":">","value":80}],
 "derivedPresets": [{"name":"old","formula":"sum","slotCount":2,"slots":[]}]
}`

// gsRecorder is a Stream Deck stand-in that keeps the payload of every
// setGlobalSettings the plugin sends.
type gsRecorder struct {
	t      *testing.T
	sd     *streamdeck.StreamDeck
	mu     sync.Mutex
	writes []map[string]json.RawMessage
	titles []string
	seen   chan struct{}
	syncs  int
}

func newGSRecorder(t *testing.T) *gsRecorder {
	t.Helper()
	rec := &gsRecorder{t: t, seen: make(chan struct{}, 1024)}
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ev struct {
				Event   string          `json:"event"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(data, &ev) != nil {
				continue
			}
			rec.mu.Lock()
			switch ev.Event {
			case "setGlobalSettings":
				var m map[string]json.RawMessage
				_ = json.Unmarshal(ev.Payload, &m)
				rec.writes = append(rec.writes, m)
			case "setTitle":
				var p struct {
					Title string `json:"title"`
				}
				_ = json.Unmarshal(ev.Payload, &p)
				rec.titles = append(rec.titles, p.Title)
			}
			rec.mu.Unlock()
			rec.seen <- struct{}{}
		}
	}))
	t.Cleanup(srv.Close)
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	rec.sd = streamdeck.NewStreamDeck(port, "test", "registerPlugin", "")
	if err := rec.sd.Connect(); err != nil {
		t.Fatalf("connect to recorder: %v", err)
	}
	t.Cleanup(rec.sd.Close)
	return rec
}

// take returns the global settings writes since the previous call, after
// syncing on a marker sent through the same connection.
func (rec *gsRecorder) take() []map[string]json.RawMessage {
	rec.t.Helper()
	rec.syncs++
	marker := fmt.Sprintf("gs-sync-%d", rec.syncs)
	if err := rec.sd.SetTitle("sync", marker); err != nil {
		rec.t.Fatalf("send marker: %v", err)
	}
	deadline := time.After(5 * time.Second)
	for {
		rec.mu.Lock()
		for _, title := range rec.titles {
			if title == marker {
				out := rec.writes
				rec.writes = nil
				rec.titles = nil
				rec.mu.Unlock()
				return out
			}
		}
		rec.mu.Unlock()
		select {
		case <-rec.seen:
		case <-deadline:
			rec.t.Fatal("recorder did not receive the marker")
		}
	}
}

// newGSPlugin returns a plugin whose sources are already running, so
// receiving global settings starts no bridge.
func newGSPlugin(t *testing.T) (*Plugin, *gsRecorder) {
	rec := newGSRecorder(t)
	p := &Plugin{
		sd: rec.sd,
		am: newActionManager(2 * time.Second),
		sources: map[string]*sourceRuntime{
			"default":  {hw: &tileHW{}, profile: lhmSourceProfile{ID: "default", Name: "This PC", Host: "127.0.0.1", Port: 8085}},
			"source_1": {hw: &tileHW{}, profile: lhmSourceProfile{ID: "source_1", Name: "PC 2", Host: "192.168.1.20", Port: 8085}},
		},
		globalSettings:    globalSettings{PollInterval: 2000},
		lastPollTime:      map[string]uint64{},
		lastRenderTime:    map[string]time.Time{},
		compositeSettings: map[string]*compositeActionSettings{},
		derivedSettings:   map[string]*derivedActionSettings{},
	}
	receiveGlobalSettings(p, fullGlobalSettings)
	rec.take()
	return p, rec
}

func receiveGlobalSettings(p *Plugin, raw string) {
	msg := json.RawMessage(raw)
	p.OnDidReceiveGlobalSettings(&streamdeck.EvDidReceiveGlobalSettings{
		Payload: streamdeck.EvDidReceiveGlobalSettingsPayload{Settings: &msg},
	})
}

func assertCompleteSettings(t *testing.T, gs globalSettings, wantPresets string) {
	t.Helper()
	if len(gs.SourceProfiles) != 2 || gs.SourceProfiles[1].Host != "192.168.1.20" {
		t.Errorf("source profiles = %+v, want both profiles", gs.SourceProfiles)
	}
	if gs.DefaultSourceProfileID != "default" {
		t.Errorf("default profile = %q, want default", gs.DefaultSourceProfileID)
	}
	if len(gs.FavoriteReadings) != 1 || gs.FavoriteReadings[0].ID != "f1" {
		t.Errorf("favorites = %+v, want f1", gs.FavoriteReadings)
	}
	if len(gs.GlobalThresholds) != 1 || gs.GlobalThresholds[0].ID != "g1" {
		t.Errorf("global thresholds = %+v, want g1", gs.GlobalThresholds)
	}
	if !strings.Contains(string(gs.DerivedPresets), wantPresets) {
		t.Errorf("derived presets = %s, want %s in it", gs.DerivedPresets, wantPresets)
	}
}

func assertWrittenComplete(t *testing.T, writes []map[string]json.RawMessage, wantPresets string) {
	t.Helper()
	if len(writes) == 0 {
		t.Fatal("plugin wrote no global settings")
	}
	last := writes[len(writes)-1]
	var gs globalSettings
	buf, _ := json.Marshal(last)
	if err := json.Unmarshal(buf, &gs); err != nil {
		t.Fatalf("written settings: %v", err)
	}
	assertCompleteSettings(t, gs, wantPresets)
}

// A Property Inspector of an older version writes only the presets. The
// plugin keeps everything else and writes the complete settings back, so
// the stored copy is whole again.
func TestPartialPresetWriteKeepsTheRest(t *testing.T) {
	p, rec := newGSPlugin(t)
	receiveGlobalSettings(p, `{"derivedPresets":[{"name":"old"},{"name":"new","formula":"sum","slotCount":2,"slots":[]}]}`)

	assertCompleteSettings(t, p.globalSettings, `"new"`)
	if p.globalSettings.PollInterval != 2000 {
		t.Errorf("poll interval = %d, want 2000 kept", p.globalSettings.PollInterval)
	}
	assertWrittenComplete(t, rec.take(), `"new"`)
}

// A Property Inspector of an older version writes only the refresh rate.
func TestPartialPollIntervalWriteKeepsTheRest(t *testing.T) {
	p, rec := newGSPlugin(t)
	receiveGlobalSettings(p, `{"pollInterval":500}`)

	if p.globalSettings.PollInterval != 500 {
		t.Errorf("poll interval = %d, want 500", p.globalSettings.PollInterval)
	}
	assertCompleteSettings(t, p.globalSettings, `"old"`)
	assertWrittenComplete(t, rec.take(), `"old"`)
}

// A complete object is taken as is and not written back.
func TestCompleteGlobalSettingsAreNotWrittenBack(t *testing.T) {
	p, rec := newGSPlugin(t)
	receiveGlobalSettings(p, fullGlobalSettings)
	assertCompleteSettings(t, p.globalSettings, `"old"`)
	if writes := rec.take(); len(writes) != 0 {
		t.Errorf("plugin wrote %d times for a complete object, want 0", len(writes))
	}
}

// Every write by the plugin carries the Derived presets: saving a favorite or
// adding a global threshold must not drop them.
func TestPluginWriteKeepsDerivedPresets(t *testing.T) {
	p, rec := newGSPlugin(t)
	if err := p.persistGlobalSettings(); err != nil {
		t.Fatal(err)
	}
	assertWrittenComplete(t, rec.take(), `"old"`)

	p.handleGlobalAddThreshold("Warm")
	writes := rec.take()
	if len(writes) == 0 {
		t.Fatal("adding a global threshold wrote nothing")
	}
	if !strings.Contains(string(writes[len(writes)-1]["derivedPresets"]), `"old"`) {
		t.Errorf("threshold write lost the presets: %s", writes[len(writes)-1]["derivedPresets"])
	}
}

// The Derived Property Inspector sends its preset list to the plugin, which
// writes it together with everything else.
func TestSaveDerivedPresetsWritesComplete(t *testing.T) {
	p, rec := newGSPlugin(t)
	if err := p.saveDerivedPresets(json.RawMessage(`[{"name":"old"},{"name":"mine","formula":"max","slotCount":3,"slots":[]}]`)); err != nil {
		t.Fatal(err)
	}
	assertCompleteSettings(t, p.globalSettings, `"mine"`)
	assertWrittenComplete(t, rec.take(), `"mine"`)

	if err := p.saveDerivedPresets(json.RawMessage(`{"name":"not a list"}`)); err == nil {
		t.Error("a non-list was accepted as presets")
	}
	if writes := rec.take(); len(writes) != 0 {
		t.Errorf("a rejected preset list was written")
	}
	if !strings.Contains(string(p.globalSettings.DerivedPresets), `"mine"`) {
		t.Errorf("a rejected preset list replaced the stored one: %s", p.globalSettings.DerivedPresets)
	}
}

func TestMergeGlobalSettings(t *testing.T) {
	current := globalSettings{PollInterval: 1000, DefaultSourceProfileID: "default", SourceProfiles: []lhmSourceProfile{{ID: "default", Host: "127.0.0.1", Port: 8085}}}

	merged, incomplete, err := mergeGlobalSettings(current, json.RawMessage(`{"sourceProfiles":[{"id":"default","host":"10.0.0.2","port":8085}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !incomplete {
		t.Error("an object without pollInterval and default profile should count as incomplete")
	}
	if merged.SourceProfiles[0].Host != "10.0.0.2" || merged.PollInterval != 1000 || merged.DefaultSourceProfileID != "default" {
		t.Errorf("merged = %+v", merged)
	}
	if current.SourceProfiles[0].Host != "127.0.0.1" {
		t.Errorf("merge changed the current settings: %+v", current.SourceProfiles)
	}

	if _, _, err := mergeGlobalSettings(current, json.RawMessage(`[1,2]`)); err == nil {
		t.Error("a non-object was accepted")
	}
}
