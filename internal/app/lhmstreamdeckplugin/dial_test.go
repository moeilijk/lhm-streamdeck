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
