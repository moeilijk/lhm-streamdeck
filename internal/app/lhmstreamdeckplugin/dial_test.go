package lhmstreamdeckplugin

import "testing"

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
		name             string
		alpha, value     float64
		prev, want       float64
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
