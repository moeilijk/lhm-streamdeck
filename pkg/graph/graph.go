package graph

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"image"
	"image/color"
	"image/png"
	"sync"
)

// Label struct contains text, position and color information
type Label struct {
	text     string
	y        uint
	fontSize float64
	clr      *color.RGBA
}

// Graph is used to display a histogram of data passed to Update
type Graph struct {
	img *image.RGBA

	lvay   int
	width  int
	height int
	min    int
	max    int

	yvals []uint8

	fgColor *color.RGBA
	bgColor *color.RGBA
	hlColor *color.RGBA

	labels map[int]*Label
	drawn  bool
	redraw bool
}

// FontFaceManager builds and caches fonts based on size
type FontFaceManager struct {
	mux       sync.Mutex
	fontCache map[float64]font.Face
}

// NewFontFaceManager constructs new manager
func NewFontFaceManager() *FontFaceManager {
	return &FontFaceManager{fontCache: make(map[float64]font.Face)}
}

func (f *FontFaceManager) newFace(size float64) (font.Face, error) {
	b, err := os.ReadFile("DejaVuSans-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	tt, err := truetype.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return truetype.NewFace(tt, &truetype.Options{Size: size, DPI: 72}), nil
}

// GetFaceOfSize returns font face for given size
func (f *FontFaceManager) GetFaceOfSize(size float64) (font.Face, error) {
	f.mux.Lock()
	defer f.mux.Unlock()
	if face, ok := f.fontCache[size]; ok {
		return face, nil
	}
	face, err := f.newFace(size)
	if err != nil {
		return nil, err
	}
	f.fontCache[size] = face
	return face, nil
}

type singleshared struct {
	mu              sync.Mutex // serializes EncodePNG calls that share pngBuf/pngEnc
	fontFaceManager *FontFaceManager
	pngEnc          *png.Encoder
	pngBuf          *bytes.Buffer
}

var sharedinstance *singleshared
var once sync.Once

func shared() *singleshared {
	once.Do(func() {
		sharedinstance = &singleshared{
			pngEnc: &png.Encoder{
				CompressionLevel: png.NoCompression,
			},
			pngBuf: bytes.NewBuffer(make([]byte, 0, 15697)),
		}
		sharedinstance.fontFaceManager = NewFontFaceManager()
	})
	return sharedinstance
}

// GetSharedFontFaceManager returns the shared FontFaceManager singleton.
func GetSharedFontFaceManager() *FontFaceManager {
	return shared().fontFaceManager
}

// NewGraph initializes a new Graph for rendering
func NewGraph(width, height, min, max int, fgColor, bgColor, hlColor *color.RGBA) *Graph {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	labels := make(map[int]*Label)

	return &Graph{
		img:    img,
		lvay:   -1,
		width:  width,
		height: height,
		min:    min,
		max:    max,
		labels: labels,

		yvals: make([]uint8, 0, width),

		fgColor: fgColor,
		bgColor: bgColor,
		hlColor: hlColor,
	}
}

// SetForegroundColor sets the foreground color of the graph
func (g *Graph) SetForegroundColor(clr *color.RGBA) {
	g.fgColor = clr
	g.redraw = true
}

// SetBackgroundColor sets the background color of the graph
func (g *Graph) SetBackgroundColor(clr *color.RGBA) {
	g.bgColor = clr
	g.redraw = true
}

// SetHighlightColor sets the highlight color of the graph
func (g *Graph) SetHighlightColor(clr *color.RGBA) {
	g.hlColor = clr
	g.redraw = true
}

// SetMin sets the min value for the graph scale
func (g *Graph) SetMin(min int) {
	g.min = min
}

// SetMax sets the max value for the graph scale
func (g *Graph) SetMax(max int) {
	g.max = max
}

// SetLabel given a key, set the initial text, position and color
func (g *Graph) SetLabel(key int, text string, y uint, clr *color.RGBA) {
	l := &Label{text: text, y: y, clr: clr}
	g.labels[key] = l
}

// SetLabelText given a key, update the text for a pre-set label
func (g *Graph) SetLabelText(key int, text string) error {
	l, ok := g.labels[key]
	if !ok {
		return fmt.Errorf("Label with key (%d) does not exist", key)
	}
	l.text = text
	return nil
}

// SetLabelFontSize given a key, update the text for a pre-set label
func (g *Graph) SetLabelFontSize(key int, size float64) error {
	l, ok := g.labels[key]
	if !ok {
		return fmt.Errorf("Label with key (%d) does not exist", key)
	}
	l.fontSize = size
	return nil
}

// SetLabelColor given a key and color, sets the color of the text
func (g *Graph) SetLabelColor(key int, clr *color.RGBA) error {
	l, ok := g.labels[key]
	if !ok {
		return fmt.Errorf("Label with key (%d) does not exist", key)
	}
	l.clr = clr
	return nil
}

func (g *Graph) drawGraph(x, vay, maxx int) {
	var clr *color.RGBA
	for ; x <= maxx; x++ {
		for y := 0; y < g.height; y++ {
			if y == vay {
				clr = g.hlColor
			} else if g.lvay != -1 && vay > g.lvay && vay >= y && y >= g.lvay {
				clr = g.hlColor
			} else if g.lvay != -1 && vay < g.lvay && vay <= y && y <= g.lvay {
				clr = g.hlColor
			} else if vay > y {
				clr = g.fgColor
			} else {
				clr = g.bgColor
			}
			i := g.img.PixOffset(x, g.width-1-y)
			g.img.Pix[i+0] = clr.R
			g.img.Pix[i+1] = clr.G
			g.img.Pix[i+2] = clr.B
			g.img.Pix[i+3] = clr.A
		}
		g.lvay = vay
	}
}

// Update given a value draws the graph, shifting contents left. Call EncodePNG to get a rendered PNG
func (g *Graph) Update(value float64) {
	vay := vAsY(g.height-1, value, g.min, g.max)

	if len(g.yvals) >= g.width {
		_, a := g.yvals[0], g.yvals[1:]
		g.yvals = a
	}
	g.yvals = append(g.yvals, uint8(vay))

	if g.redraw {
		g.lvay = -1
		lyvals := len(g.yvals)
		for idx := lyvals - 1; idx >= 0; idx-- {
			x := g.width - lyvals + idx
			maxx := x
			if idx == 0 {
				x = 0
			}
			v := int(g.yvals[idx])
			g.drawGraph(x, v, maxx)
		}
		g.lvay = int(g.yvals[lyvals-1])
		g.redraw = false
	} else if g.drawn {
		// shift the graph left 1px (in-place, avoid allocations)
		stride := g.img.Stride
		for y := 0; y < g.height; y++ {
			rowStart := y * stride
			row := g.img.Pix[rowStart : rowStart+stride]
			copy(row, row[4:])
			row[stride-4] = 0
			row[stride-3] = 0
			row[stride-2] = 0
			row[stride-1] = 0
		}
		g.drawGraph(int(g.width)-1, int(vay), g.width-1)
	} else {
		g.drawGraph(0, vay, g.width-1)
		g.drawn = true
	}
}

// EncodePNG renders the current state of the graph
func (g *Graph) EncodePNG() ([]byte, error) {
	bak := append(g.img.Pix[:0:0], g.img.Pix...)
	for _, l := range g.labels {
		g.drawLabel(l)
	}
	s := shared()
	s.mu.Lock()
	err := s.pngEnc.Encode(s.pngBuf, g.img)
	if err != nil {
		s.pngBuf.Reset()
		s.mu.Unlock()
		g.img.Pix = bak
		return nil, err
	}
	bts := append([]byte(nil), s.pngBuf.Bytes()...)
	s.pngBuf.Reset()
	s.mu.Unlock()
	g.img.Pix = bak
	return bts, nil
}

func vAsY(maxY int, v float64, minV, maxV int) int {
	r := maxV - minV
	v1 := v - float64(minV)
	yf := v1 / float64(r) * float64(maxY)
	yi := int(math.Round(yf))
	return yi
}

func unfix(x fixed.Int26_6) float64 {
	const shift, mask = 6, 1<<6 - 1
	if x >= 0 {
		return float64(x>>shift) + float64(x&mask)/64
	}
	x = -x
	if x >= 0 {
		return -(float64(x>>shift) + float64(x&mask)/64)
	}
	return 0
}

var newlineRegex = regexp.MustCompile("(\n|\\\\n)+")

func (g *Graph) drawLabel(l *Label) {
	shared := shared()
	lines := newlineRegex.Split(l.text, -1)
	face, err := shared.fontFaceManager.GetFaceOfSize(l.fontSize)
	if err != nil {
		log.Printf("drawLabel font: %v", err)
		return
	}
	curY := l.y - uint(10.5-float64(face.Metrics().Height.Round()))

	for _, line := range lines {
		var lwidth float64
		for _, x := range line {
			awidth, ok := face.GlyphAdvance(rune(x))
			if ok != true {
				log.Println("drawLabel: Failed to GlyphAdvance")
				return
			}
			lwidth += unfix(awidth)
		}

		lx := (float64(g.width) / 2.) - (lwidth / 2.)
		point := fixed.Point26_6{X: fixed.Int26_6(lx * 64), Y: fixed.Int26_6(curY * 64)}

		d := &font.Drawer{
			Dst:  g.img,
			Src:  image.NewUniform(l.clr),
			Face: face,
			Dot:  point,
		}
		d.DrawString(line)
		curY += 12
	}
}
