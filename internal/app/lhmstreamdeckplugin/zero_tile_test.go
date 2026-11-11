package lhmstreamdeckplugin

import (
	"bytes"
	"image/png"
	"testing"
	"time"
)

func TestStartupTitleFadesIntoFirstReading(t *testing.T) {
	now := time.Unix(100, 0)
	z, err := waitingReadingState(&actionSettings{Title: "FPS"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(z.normalPNG)); err != nil {
		t.Fatal(err)
	}
	if z.amount != 1 || !z.hidden || !z.waiting {
		t.Fatal("startup must show only the title")
	}
	z.advance(now.Add(time.Minute))
	if z.amount != 1 {
		t.Fatal("title faded before any data arrived")
	}
	z.observe(now.Add(time.Minute), false, 60, 10)
	if z.amount != 1 || z.hidden {
		t.Fatal("first reading must start, not skip, the fade")
	}
	z.advance(now.Add(time.Minute + 750*time.Millisecond))
	if z.amount != .5 {
		t.Fatalf("mid-fade = %v", z.amount)
	}
	z.advance(now.Add(time.Minute + 1500*time.Millisecond))
	if z.amount != 0 {
		t.Fatal("graph not fully revealed")
	}
}

func TestZeroWaitResetsOnNonzero(t *testing.T) {
	now := time.Unix(100, 0)
	z := &zeroTileState{}
	z.observe(now, true, 0, 10)
	z.observe(now.Add(9*time.Second), true, 0, 10)
	if z.hidden {
		t.Fatal("hidden before timeout")
	}
	z.observe(now.Add(10*time.Second), true, 1, 10)
	z.observe(now.Add(11*time.Second), true, 0, 10)
	z.observe(now.Add(20*time.Second), true, 0, 10)
	if z.hidden {
		t.Fatal("nonzero sample did not reset timeout")
	}
	z.observe(now.Add(21*time.Second), true, 0, 10)
	if !z.hidden {
		t.Fatal("did not hide after uninterrupted zero readings")
	}
	z.observe(now.Add(22*time.Second), false, 0, 10)
	if z.hidden {
		t.Fatal("disabled setting must restore tile")
	}
}

func TestZeroFadeReversesContinuously(t *testing.T) {
	now := time.Unix(100, 0)
	z := &zeroTileState{last: now, hidden: true}
	z.advance(now.Add(750 * time.Millisecond))
	if z.amount != .5 {
		t.Fatalf("mid-fade = %v", z.amount)
	}
	z.observe(now.Add(750*time.Millisecond), true, 60, 10)
	if z.amount != .5 || z.hidden {
		t.Fatal("returning FPS jumped instead of reversing")
	}
	z.advance(now.Add(time.Second))
	if z.amount <= 0 || z.amount >= .5 {
		t.Fatal("fade did not reverse gradually")
	}
	z.advance(now.Add(2 * time.Second))
	if z.amount != 0 {
		t.Fatal("normal display not restored")
	}
}
