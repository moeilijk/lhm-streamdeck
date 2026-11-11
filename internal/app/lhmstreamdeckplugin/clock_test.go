package lhmstreamdeckplugin

import (
	"bytes"
	"image/png"
	"testing"
	"time"
)

func TestClockShowsLocalMinute(t *testing.T) {
	now := time.Date(2026, 11, 6, 23, 59, 0, 0, time.FixedZone("UTC+3", 3*3600))
	frame, err := renderClock(now, clockSettings{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(frame))
	if err != nil || img.Bounds().Dx() != 72 || img.Bounds().Dy() != 72 {
		t.Fatalf("invalid clock image: %v", err)
	}
	whitePixels := 0
	for y := 20; y < 55; y++ {
		for x := 0; x < 72; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r > 0x8000 {
				whitePixels++
			}
		}
	}
	if whitePixels < 100 {
		t.Fatal("clock text missing")
	}
	same, _ := renderClock(now.Add(8*time.Second), clockSettings{})
	if !bytes.Equal(frame, same) {
		t.Fatal("a full animation cycle should restore the same frame")
	}
	next, _ := renderClock(now.Add(time.Minute), clockSettings{})
	if bytes.Equal(frame, next) {
		t.Fatal("clock did not change at midnight")
	}
	utc, _ := renderClock(now.UTC(), clockSettings{})
	if bytes.Equal(frame, utc) {
		t.Fatal("clock ignored local timezone")
	}
}

func TestClockColonFadeAndWeekday(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	if clockColonOpacity(now) != 255 || clockColonOpacity(now.Add(4*time.Second)) != 0 {
		t.Fatal("colon should fade fully in and out")
	}
	previous := clockColonOpacity(now)
	for step := 1; step <= 80; step++ {
		current := clockColonOpacity(now.Add(time.Duration(step) * 50 * time.Millisecond))
		if current > previous || int(previous)-int(current) > 11 {
			t.Fatal("colon fade should be gradual and monotonic")
		}
		previous = current
	}
	a, _ := renderClock(now, clockSettings{})
	b, _ := renderClock(now.Add(4*time.Second), clockSettings{})
	c, _ := renderClock(now.Add(24*time.Hour), clockSettings{})
	if bytes.Equal(a, b) {
		t.Fatal("colon animation missing")
	}
	if bytes.Equal(a, c) {
		t.Fatal("weekday did not change")
	}
	if clockWeekdays[now.Weekday()] != "ПТ" {
		t.Fatal("incorrect local weekday")
	}
}

func TestClockOptions(t *testing.T) {
	now := time.Date(2026, 11, 6, 23, 59, 0, 0, time.UTC)
	day, date := clockCalendar(now, "en")
	if day != "FRI" || date != "06 NOV" {
		t.Fatalf("English date: %s %s", day, date)
	}
	a, err := renderClock(now, clockSettings{StaticColon: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := renderClock(now.Add(4*time.Second), clockSettings{StaticColon: true})
	if !bytes.Equal(a, b) {
		t.Fatal("static colon animated")
	}
	c, _ := renderClock(now, clockSettings{TwelveHour: true, StaticColon: true})
	if bytes.Equal(a, c) {
		t.Fatal("12-hour format did not change time")
	}
	d, _ := renderClock(now, clockSettings{Language: "ru", StaticColon: true})
	if bytes.Equal(a, d) {
		t.Fatal("language did not change calendar")
	}
}
