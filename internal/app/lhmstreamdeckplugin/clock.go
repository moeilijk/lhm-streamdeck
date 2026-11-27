package lhmstreamdeckplugin

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/moeilijk/lhm-streamdeck/pkg/graph"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const clockAction = "com.moeilijk.lhm.clock"

var clockWeekdays = [...]string{"ВС", "ПН", "ВТ", "СР", "ЧТ", "ПТ", "СБ"}

var clockMonths = [...]string{"ЯНВ", "ФЕВ", "МАР", "АПР", "МАЙ", "ИЮН", "ИЮЛ", "АВГ", "СЕН", "ОКТ", "НОЯ", "ДЕК"}

// An eight-second cosine cycle has no abrupt changes at either end of the fade.
func clockColonOpacity(now time.Time) uint8 {
	phase := float64(now.UnixMilli()%8000) / 8000
	return uint8(math.Round(255 * (0.5 + 0.5*math.Cos(2*math.Pi*phase))))
}

func renderClock(now time.Time, settings clockSettings) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, tileWidth, tileHeight))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
	white := &color.RGBA{255, 255, 255, 255}
	var shadow *color.RGBA
	weekday, date := clockCalendar(now, settings.Language)
	drawCompositeCenteredText(img, weekday, 18, 12, white, shadow)
	face, err := graph.GetSharedFontFaceManager().GetFaceOfSize(20)
	if err != nil {
		return nil, err
	}
	text := now.Format("15:04")
	if settings.TwelveHour {
		text = now.Format("03:04")
		date += " " + now.Format("PM")
	}
	point := fixed.Point26_6{X: (fixed.I(tileWidth) - font.MeasureString(face, text)) / 2, Y: fixed.I(44)}
	d := &font.Drawer{Dst: img, Face: face, Dot: point}
	for _, ch := range text {
		d.Src = image.White
		if ch == ':' && !settings.StaticColon {
			a := clockColonOpacity(now)
			d.Src = image.NewUniform(color.RGBA{a, a, a, a})
		}
		d.DrawString(string(ch))
	}
	drawCompositeCenteredText(img, date, 63, 9, &color.RGBA{190, 190, 200, 255}, shadow)
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	return buf.Bytes(), err
}

// The clock runs independently of sensor polling, including when LHM is offline.
func (p *Plugin) runClocks(done <-chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			p.updateClocks(now)
		}
	}
}

type clockSettings struct {
	Language    string `json:"language"`
	TwelveHour  bool   `json:"twelveHour"`
	StaticColon bool   `json:"staticColon"`
}

type clockState struct {
	settings  clockSettings
	lastFrame string
}

func clockCalendar(now time.Time, language string) (string, string) {
	if language == "ru" {
		return clockWeekdays[now.Weekday()], now.Format("02") + " " + clockMonths[int(now.Month())-1]
	}
	return strings.ToUpper(now.Format("Mon")), strings.ToUpper(now.Format("02 Jan"))
}

func (p *Plugin) setClockSettings(context string, raw *json.RawMessage) {
	var settings clockSettings
	if raw != nil {
		if err := json.Unmarshal(*raw, &settings); err != nil {
			log.Printf("Clock settings: %v", err)
			return
		}
	}
	p.mu.Lock()
	if p.clockContexts == nil {
		p.clockContexts = make(map[string]*clockState)
	}
	p.clockContexts[context] = &clockState{settings: settings}
	p.mu.Unlock()
}

func (p *Plugin) updateClocks(now time.Time) {
	type update struct {
		context  string
		state    *clockState
		settings clockSettings
		key      string
	}
	var updates []update
	p.mu.RLock()
	for context, state := range p.clockContexts {
		key := strconv.FormatInt(now.UnixMilli()/50, 10)
		if state.settings.StaticColon {
			key = now.Format("2006-01-02T15:04-0700")
		}
		if state.lastFrame != key {
			updates = append(updates, update{context, state, state.settings, key})
		}
	}
	p.mu.RUnlock()
	frames := make(map[clockSettings][]byte)
	for _, u := range updates {
		frame := frames[u.settings]
		if frame == nil {
			var err error
			frame, err = renderClock(now, u.settings)
			if err != nil {
				log.Printf("Clock render: %v", err)
				continue
			}
			frames[u.settings] = frame
		}
		if err := p.sd.SetImage(u.context, frame); err != nil {
			log.Printf("Clock SetImage: %v", err)
			continue
		}
		p.mu.Lock()
		if p.clockContexts[u.context] == u.state {
			u.state.lastFrame = u.key
		}
		p.mu.Unlock()
	}
}
