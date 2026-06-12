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
