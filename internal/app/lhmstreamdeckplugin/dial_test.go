package lhmstreamdeckplugin

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	hwsensorsservice "github.com/moeilijk/lhm-streamdeck/pkg/service"
)

func TestWrapDialIndex(t *testing.T) {
	tests := []struct {
		name    string
		current int
		ticks   int
		count   int
		want    int
	}{
		{name: "clockwise one", current: 0, ticks: 1, count: 4, want: 1},
		{name: "counter clockwise wraps", current: 0, ticks: -1, count: 4, want: 3},
		{name: "multi tick clockwise wraps", current: 2, ticks: 5, count: 4, want: 3},
		{name: "multi tick counter clockwise wraps", current: 1, ticks: -6, count: 4, want: 3},
		{name: "empty pages", current: 3, ticks: 1, count: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wrapDialIndex(tt.current, tt.ticks, tt.count); got != tt.want {
				t.Fatalf("wrapDialIndex(%d, %d, %d) = %d, want %d", tt.current, tt.ticks, tt.count, got, tt.want)
			}
		})
	}
}

func TestDialOverviewIndices(t *testing.T) {
	tests := []struct {
		name   string
		active int
		count  int
		want   []int
	}{
		{name: "empty", active: 0, count: 0, want: nil},
		{name: "single", active: 0, count: 1, want: []int{0}},
		{name: "two", active: 1, count: 2, want: []int{0, 1}},
		{name: "middle", active: 2, count: 5, want: []int{1, 2, 3}},
		{name: "wrap left", active: 0, count: 5, want: []int{4, 0, 1}},
		{name: "wrap right", active: 4, count: 5, want: []int{3, 4, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dialOverviewIndices(tt.active, tt.count)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("overview indices = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestEmaSmooth(t *testing.T) {
	tests := []struct {
		name         string
		alpha, value float64
		prev, want   float64
	}{
		{name: "alpha 1 follows value", alpha: 1, value: 50, prev: 10, want: 50},
		{name: "alpha 0.5 halfway", alpha: 0.5, value: 100, prev: 0, want: 50},
		{name: "alpha 0.2 mostly prev", alpha: 0.2, value: 100, prev: 0, want: 20},
		{name: "stable when equal", alpha: 0.3, value: 42, prev: 42, want: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := emaSmooth(tt.alpha, tt.value, tt.prev)
			if got != tt.want {
				t.Fatalf("emaSmooth(%v,%v,%v) = %v, want %v", tt.alpha, tt.value, tt.prev, got, tt.want)
			}
		})
	}
}

func TestDecodeDialSettings(t *testing.T) {
	raw := json.RawMessage(`{"activeIndex":5,"pages":[{},{},{}]}`)
	s, err := decodeDialSettings(&raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.ActiveIndex != 2 {
		t.Errorf("activeIndex = %d, want 2 (5 mod 3)", s.ActiveIndex)
	}
	neg := json.RawMessage(`{"activeIndex":-1,"pages":[{}]}`)
	if s2, _ := decodeDialSettings(&neg); s2.ActiveIndex != 0 {
		t.Errorf("negative activeIndex = %d, want 0", s2.ActiveIndex)
	}
	if s3, _ := decodeDialSettings(nil); s3.ActiveIndex != 0 || len(s3.Pages) != 0 {
		t.Errorf("nil raw should decode empty, got %+v", s3)
	}
}

func TestDialColor(t *testing.T) {
	def := color.RGBA{1, 2, 3, 255}
	if c := dialColor("#ff0000", def); c == nil || c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("valid hex = %+v, want red", c)
	}
	if c := dialColor("", def); c == nil || *c != def {
		t.Errorf("empty hex = %+v, want default %+v", c, def)
	}
}

func TestDialDefaultPageColors(t *testing.T) {
	firstFG, firstHL := dialDefaultPageColors(0)
	secondFG, secondHL := dialDefaultPageColors(1)
	wrappedFG, wrappedHL := dialDefaultPageColors(len(dialPageColorPalette))

	if firstFG == "" || firstHL == "" {
		t.Fatalf("first page colors must be set")
	}
	if firstFG == secondFG || firstHL == secondHL {
		t.Fatalf("second page should get a different default color")
	}
	if wrappedFG != firstFG || wrappedHL != firstHL {
		t.Fatalf("palette wrap = (%s,%s), want first (%s,%s)", wrappedFG, wrappedHL, firstFG, firstHL)
	}
}

func TestDefaultDialFontSizes(t *testing.T) {
	if got := defaultDialTitleFontSize(0); got != 14 {
		t.Errorf("title 0 -> %v, want 14", got)
	}
	if got := defaultDialTitleFontSize(9); got != 9 {
		t.Errorf("title 9 -> %v, want 9", got)
	}
	if got := defaultDialValueFontSize(0); got != 18 {
		t.Errorf("value 0 -> %v, want 18", got)
	}
	if got := defaultDialValueFontSize(22); got != 22 {
		t.Errorf("value 22 -> %v, want 22", got)
	}
}

func TestDialSeparatorDynamicWidthAndColor(t *testing.T) {
	bg := color.RGBA{20, 120, 30, 255}
	sep := color.RGBA{200, 30, 30, 255}
	fixture := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, dialWidth, dialHeight))
		fillRect(img, img.Bounds(), bg)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode fixture: %v", err)
		}
		return buf.Bytes()
	}
	at := func(b []byte, x, y int) color.RGBA {
		im, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
	}
	y := dialHeight / 2

	// width 0 = off: edges keep the original graph pixels.
	off, err := decorateDialImage(fixture(), 0, 1, false, 0, sep)
	if err != nil {
		t.Fatalf("decorate off: %v", err)
	}
	if at(off, 0, y) != bg || at(off, dialWidth-1, y) != bg {
		t.Fatalf("width 0 must not draw a separator")
	}

	// width 5 + color: a 5px band of that color on each edge, center untouched.
	on, err := decorateDialImage(fixture(), 0, 1, false, 5, sep)
	if err != nil {
		t.Fatalf("decorate on: %v", err)
	}
	if at(on, 0, y) != sep || at(on, 4, y) != sep || at(on, dialWidth-1, y) != sep || at(on, dialWidth-5, y) != sep {
		t.Fatalf("width 5 must draw a 5px colored band on both edges")
	}
	if at(on, 5, y) != bg || at(on, dialWidth-6, y) != bg {
		t.Fatalf("separator band must be exactly 5px wide")
	}
	if at(on, dialWidth/2, y) != bg {
		t.Fatalf("center must not be overwritten by the separator")
	}
}

func TestDialSeparatorResolve(t *testing.T) {
	if got := dialSeparatorWidth(&dialActionSettings{}); got != 3 {
		t.Errorf("unset width -> %d, want default 3", got)
	}
	w7, w0, w15, wn := 7, 0, 15, -2
	if got := dialSeparatorWidth(&dialActionSettings{SeparatorWidth: &w7}); got != 7 {
		t.Errorf("explicit 7 -> %d", got)
	}
	if got := dialSeparatorWidth(&dialActionSettings{SeparatorWidth: &w0}); got != 0 {
		t.Errorf("explicit 0 (off) -> %d", got)
	}
	if got := dialSeparatorWidth(&dialActionSettings{SeparatorWidth: &w15}); got != 10 {
		t.Errorf("15 -> %d, want clamp 10", got)
	}
	if got := dialSeparatorWidth(&dialActionSettings{SeparatorWidth: &wn}); got != 0 {
		t.Errorf("negative -> %d, want 0", got)
	}
	if c := dialSeparatorColor(&dialActionSettings{}); c != (color.RGBA{54, 62, 70, 255}) {
		t.Errorf("default color = %+v", c)
	}
	if c := dialSeparatorColor(&dialActionSettings{SeparatorColor: "#ff0000"}); c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("parsed color = %+v, want red", c)
	}
}

func TestDrawDialPageIndicatorUsesDotsForSmallPageCounts(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, dialWidth, dialHeight))
	fillRect(img, img.Bounds(), color.RGBA{0, 0, 0, 255})

	drawDialPageIndicator(img, 1, 3)

	activePixel := color.RGBAModel.Convert(img.At(dialWidth/2, dialHeight-6)).(color.RGBA)
	if activePixel == (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("active indicator pixel was not drawn")
	}
}

func TestDialPageContext(t *testing.T) {
	if got := dialPageContext("abc", 2); got != "abc|dial|page|2" {
		t.Errorf("dialPageContext = %q, want abc|dial|page|2", got)
	}
	if dialPageContext("x", 0) == dialPageContext("x", 1) {
		t.Errorf("different page indices must produce different contexts")
	}
}

func TestDialGraphScale(t *testing.T) {
	tests := []struct {
		name             string
		min, max         int
		wantMin, wantMax int
	}{
		{name: "explicit", min: 10, max: 90, wantMin: 10, wantMax: 90},
		{name: "unset sentinel", min: 0, max: 0, wantMin: 0, wantMax: 100},
		{name: "inverted falls back", min: 50, max: 20, wantMin: 0, wantMax: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := dialGraphScale(&actionSettings{Min: tt.min, Max: tt.max})
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Fatalf("scale = (%d,%d), want (%d,%d)", gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBuildDialGraphsPreservesHistory(t *testing.T) {
	old := &dialActionSettings{Pages: []actionSettings{
		{SensorUID: "cpu", ReadingID: 1, Min: 0, Max: 100},
		{SensorUID: "gpu", ReadingID: 2, Min: 0, Max: 100},
	}}
	oldState := initDialState(old)

	// Same readings, only a scale edit on page 0: both graphs must be reused so
	// plotted history survives.
	scaleEdit := &dialActionSettings{Pages: []actionSettings{
		{SensorUID: "cpu", ReadingID: 1, Min: 0, Max: 50},
		{SensorUID: "gpu", ReadingID: 2, Min: 0, Max: 100},
	}}
	got := buildDialGraphs(old, oldState, scaleEdit)
	if got[0] != oldState.graphs[0] {
		t.Errorf("page 0 graph rebuilt on scale edit; history lost")
	}
	if got[1] != oldState.graphs[1] {
		t.Errorf("page 1 graph rebuilt despite no change")
	}

	// Reading changed on page 1: that page rebuilds, page 0 still reused.
	readingChange := &dialActionSettings{Pages: []actionSettings{
		{SensorUID: "cpu", ReadingID: 1, Min: 0, Max: 100},
		{SensorUID: "gpu", ReadingID: 9, Min: 0, Max: 100},
	}}
	got2 := buildDialGraphs(old, oldState, readingChange)
	if got2[0] != oldState.graphs[0] {
		t.Errorf("page 0 graph rebuilt despite same reading")
	}
	if got2[1] == oldState.graphs[1] {
		t.Errorf("page 1 graph reused despite reading change")
	}

	// Appended page: existing reused, new one built.
	appended := &dialActionSettings{Pages: []actionSettings{
		{SensorUID: "cpu", ReadingID: 1, Min: 0, Max: 100},
		{SensorUID: "gpu", ReadingID: 2, Min: 0, Max: 100},
		{SensorUID: "ram", ReadingID: 3, Min: 0, Max: 100},
	}}
	got3 := buildDialGraphs(old, oldState, appended)
	if len(got3) != 3 {
		t.Fatalf("graphs len = %d, want 3", len(got3))
	}
	if got3[0] != oldState.graphs[0] || got3[1] != oldState.graphs[1] {
		t.Errorf("existing graphs not preserved when appending a page")
	}
	if got3[2] == nil {
		t.Errorf("appended page graph not built")
	}
}

func TestUpdateDialPageKeepsSnoozeWhenThresholdDrops(t *testing.T) {
	const (
		ctx       = "dial-ctx"
		pageCtx   = ctx + "|dial|page|0"
		sensorUID = "/cpu"
		readingID = int32(42)
	)

	p := &Plugin{
		sources:          make(map[string]*sourceRuntime),
		thresholdStates:  make(map[string]map[string]*thresholdRuntimeState),
		thresholdSnoozes: make(map[string]*thresholdSnoozeState),
		thresholdDirty:   make(map[string]bool),
		lastPollTime:     make(map[string]uint64),
		lastRenderTime:   make(map[string]time.Time),
		smoothedValues:   make(map[string]float64),
		divisorCache:     make(map[string]divisorCacheEntry),
		pollTimeCacheTTL: time.Second,
	}
	p.sources[""] = &sourceRuntime{
		hw: stubHardwareService{
			readingsBySensor: map[string][]hwsensorsservice.Reading{
				sensorUID: {
					stubReading{id: readingID, typ: "Load", label: "CPU Total", unit: "%"},
				},
			},
		},
	}

	settings := &dialActionSettings{Pages: []actionSettings{{
		SensorUID:          sensorUID,
		ReadingID:          readingID,
		ReadingLabel:       "CPU Total",
		IsValid:            true,
		Min:                0,
		Max:                100,
		ForegroundColor:    "#005128",
		BackgroundColor:    "#000000",
		HighlightColor:     "#009e00",
		ValueTextColor:     "#ffffff",
		TitleColor:         "#b7b7b7",
		CurrentThresholdID: "t1",
		SnoozeDurations:    []int{0},
		Thresholds:         []Threshold{{ID: "t1", Enabled: true, Operator: ">=", Value: 10}},
	}}}
	state := initDialState(settings)
	now := time.Unix(1200, 0)
	p.setThresholdSnooze(pageCtx, 0, now)

	_, changed := p.updateDialPage(ctx, settings, state, 0, true, now.Add(time.Second))
	if !changed {
		t.Fatalf("expected settings change when current threshold id is cleared")
	}
	if _, ok := p.currentThresholdSnooze(pageCtx, now.Add(time.Second)); !ok {
		t.Fatalf("expected dial page snooze to remain active after threshold drops")
	}
	if settings.Pages[0].CurrentThresholdID != "" {
		t.Fatalf("expected current threshold id to clear after threshold drops")
	}
}

func TestHandleDialPageTouchStartsSnoozeForConfiguredThresholdPage(t *testing.T) {
	p := &Plugin{
		thresholdSnoozes: make(map[string]*thresholdSnoozeState),
		thresholdDirty:   make(map[string]bool),
	}
	now := time.Unix(1300, 0)
	page := &actionSettings{
		SnoozeDurations: []int{0},
		Thresholds:      []Threshold{{ID: "t1", Enabled: true, Operator: ">=", Value: 10}},
	}

	if !p.handleDialPageTouch("dial-page", page, now) {
		t.Fatalf("expected touch to start snooze on configured threshold page")
	}
	if _, ok := p.currentThresholdSnooze("dial-page", now); !ok {
		t.Fatalf("expected snooze to be active after touch")
	}
	if !p.handleDialPageTouch("dial-page", page, now.Add(time.Second)) {
		t.Fatalf("expected second touch to clear active snooze")
	}
	if _, ok := p.currentThresholdSnooze("dial-page", now.Add(time.Second)); ok {
		t.Fatalf("expected snooze to be cleared by second touch")
	}
}

func TestHandleDialPageTouchIgnoresPageWithoutThresholds(t *testing.T) {
	p := &Plugin{
		thresholdSnoozes: make(map[string]*thresholdSnoozeState),
		thresholdDirty:   make(map[string]bool),
	}
	page := &actionSettings{SnoozeDurations: []int{0}}

	if p.handleDialPageTouch("dial-page", page, time.Unix(1400, 0)) {
		t.Fatalf("expected touch without thresholds or active alert to do nothing")
	}
}
