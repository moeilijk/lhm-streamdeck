package lhmstreamdeckplugin

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"time"

	"github.com/moeilijk/lhm-streamdeck/pkg/graph"
	"golang.org/x/image/font"
)

// Animation uses the cosmetic 20 Hz ticker, never adding samples to history.
type zeroTileState struct {
	since, last   time.Time
	amount        float64
	hidden, dirty bool
	normal, idle  image.Image
	normalPNG     []byte
	identity      string
	waiting       bool
}

func (z *zeroTileState) observe(now time.Time, enabled bool, value float64, delay int) {
	if delay < 1 {
		delay = 10
	}
	if !enabled || value != 0 {
		z.since = time.Time{}
		z.hidden = false
		return
	}
	if z.since.IsZero() {
		z.since = now
	}
	z.hidden = now.Sub(z.since) >= time.Duration(delay)*time.Second
}

func (z *zeroTileState) advance(now time.Time) bool {
	if z.last.IsZero() {
		z.last = now
	}
	step := math.Max(0, now.Sub(z.last).Seconds()) / 1.5
	z.last = now
	previous := z.amount
	if z.hidden {
		z.amount = math.Min(1, z.amount+step)
	} else {
		z.amount = math.Max(0, z.amount-step)
	}
	return previous != z.amount
}

func (p *Plugin) setReadingImage(context string, normal []byte, s *actionSettings, value float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	z := p.zeroTiles[context]
	if !s.HideWhenZero && z == nil {
		return p.sd.SetImage(context, normal)
	}
	identity := s.SourceProfileID + "/" + s.SensorUID + "/" + strconv.FormatInt(int64(s.ReadingID), 10) + "/" + s.ReadingLabel
	if z == nil || z.identity != identity {
		if p.zeroTiles == nil {
			p.zeroTiles = make(map[string]*zeroTileState)
		}
		z = &zeroTileState{identity: identity, last: time.Now()}
		p.zeroTiles[context] = z
	}
	decoded, err := png.Decode(bytes.NewReader(normal))
	if err != nil {
		return err
	}
	z.normal, z.normalPNG = decoded, normal
	z.idle = readingTitleImage(s)
	z.waiting = false
	z.observe(time.Now(), s.HideWhenZero && s.CurrentThresholdID == "", value, s.ZeroHideDelaySeconds)
	z.dirty = true
	return nil
}

func readingTitleImage(s *actionSettings) *image.RGBA {
	idle := image.NewRGBA(image.Rect(0, 0, tileWidth, tileHeight))
	bg := hexToRGBA(s.BackgroundColor)
	if bg == nil {
		bg = &color.RGBA{0, 0, 0, 255}
	}
	fg := hexToRGBA(s.TitleColor)
	if fg == nil {
		fg = &color.RGBA{255, 255, 255, 255}
	}
	draw.Draw(idle, idle.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	title := s.Title
	if title == "" {
		title = s.ReadingLabel
	}
	if title == "" {
		title = "LHM"
	}
	size := 18.0
	baseline := 42
	// Fit the title inside six-pixel side margins and center its ink vertically.
	for {
		face, err := graph.GetSharedFontFaceManager().GetFaceOfSize(size)
		if err != nil {
			break
		}
		bounds, advance := font.BoundString(face, title)
		if advance.Ceil() <= tileWidth-12 || size <= 1 {
			baseline = tileHeight/2 - (bounds.Min.Y+bounds.Max.Y).Round()/2
			break
		}
		size -= .5
	}
	drawCompositeCenteredText(idle, title, baseline, size, fg, nil)
	return idle
}

// Seed the first frame before polling starts; retain it as the fade's origin.
func waitingReadingState(s *actionSettings, now time.Time) (*zeroTileState, error) {
	idle := readingTitleImage(s)
	var b bytes.Buffer
	if err := png.Encode(&b, idle); err != nil {
		return nil, err
	}
	return &zeroTileState{
		normal: idle, idle: idle, normalPNG: b.Bytes(), amount: 1,
		hidden: true, waiting: true, last: now,
		identity: s.SourceProfileID + "/" + s.SensorUID + "/" + strconv.FormatInt(int64(s.ReadingID), 10) + "/" + s.ReadingLabel,
	}, nil
}

func (p *Plugin) showReadingTitle(context string, s *actionSettings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if z := p.zeroTiles[context]; z != nil && z.waiting {
		return nil
	}
	z, err := waitingReadingState(s, time.Now())
	if err != nil {
		return err
	}
	if p.zeroTiles == nil {
		p.zeroTiles = make(map[string]*zeroTileState)
	}
	p.zeroTiles[context] = z
	return p.sd.SetImage(context, z.normalPNG)
}

func (p *Plugin) updateZeroTiles(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for context, z := range p.zeroTiles {
		changed := z.advance(now)
		if !changed && !z.dirty {
			continue
		}
		frame := z.normalPNG
		if z.amount > 0 {
			img := image.NewRGBA(z.normal.Bounds())
			// Smoothstep makes the fade settle gently at both ends.
			a := z.amount * z.amount * (3 - 2*z.amount)
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					n := color.RGBAModel.Convert(z.normal.At(x, y)).(color.RGBA)
					h := color.RGBAModel.Convert(z.idle.At(x, y)).(color.RGBA)
					mix := func(v, w uint8) uint8 { return uint8(math.Round(float64(v)*(1-a) + float64(w)*a)) }
					img.SetRGBA(x, y, color.RGBA{mix(n.R, h.R), mix(n.G, h.G), mix(n.B, h.B), 255})
				}
			}
			var b bytes.Buffer
			if err := png.Encode(&b, img); err != nil {
				continue
			}
			frame = b.Bytes()
		}
		if err := p.sd.SetImage(context, frame); err == nil {
			z.dirty = false
		}
	}
}

func (p *Plugin) runIdleTiles(done <-chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			p.updateZeroTiles(now)
		}
	}
}
