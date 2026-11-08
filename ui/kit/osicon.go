package kit

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type logoKey struct {
	path string
	px   int
}

func logoCacheKey(path string, px int) logoKey {
	return logoKey{path: path, px: max(1, px)}
}

type logoEntry struct {
	alpha   *image.Alpha
	err     error
	loading bool
	badge   bool
	tints   map[color.NRGBA]paint.ImageOp
}

type Logos struct {
	mu         sync.Mutex
	cache      map[logoKey]*logoEntry
	invalidate func()
	shaper     *text.Shaper
}

func NewLogos(invalidate func()) *Logos {
	return &Logos{
		cache:      make(map[logoKey]*logoEntry),
		invalidate: invalidate,
		shaper:     theme.Shaper(),
	}
}

func (l *Logos) Widget(path string, size unit.Dp, fallback string, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		requested := max(1, gtx.Dp(size))
		dims := gtx.Constraints.Constrain(image.Pt(requested, requested))
		if isWindowsLogo(path) {
			return windowsMark(gtx, dims, ink)
		}
		px := max(1, min(dims.X, dims.Y))
		key := logoCacheKey(path, px)

		l.mu.Lock()
		entry, ok := l.cache[key]
		if !ok {
			entry = &logoEntry{loading: true}
			l.cache[key] = entry
			go l.load(key, entry)
		}
		loading, loadErr, badge := entry.loading, entry.err, entry.badge
		var logo paint.ImageOp
		if !loading && loadErr == nil && !badge && entry.alpha != nil {
			if entry.tints == nil {
				entry.tints = make(map[color.NRGBA]paint.ImageOp)
			}
			var tinted bool
			logo, tinted = entry.tints[ink]
			if !tinted {
				logo = paint.NewImageOp(tintAlpha(entry.alpha, ink))
				entry.tints[ink] = logo
			}
		}
		l.mu.Unlock()

		if loading {
			return layout.Dimensions{Size: dims}
		}
		if loadErr != nil || badge || logo.Size() == (image.Point{}) {
			return l.fallback(gtx, dims, fallback, ink)
		}
		offset := image.Pt((dims.X-logo.Size().X)/2, (dims.Y-logo.Size().Y)/2)
		stack := op.Offset(offset).Push(gtx.Ops)
		clipStack := clip.Rect(image.Rectangle{Max: logo.Size()}).Push(gtx.Ops)
		logo.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		clipStack.Pop()
		stack.Pop()
		return layout.Dimensions{Size: dims}
	}
}

func (l *Logos) fallback(gtx layout.Context, size image.Point, fallback string, ink color.NRGBA) layout.Dimensions {
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return layout.Dimensions{Size: size}
	}
	r, _ := utf8.DecodeRuneInString(fallback)
	letter := strings.ToUpper(string(r))
	labelCtx := gtx
	labelCtx.Constraints = layout.Exact(size)
	layout.Center.Layout(labelCtx, func(gtx layout.Context) layout.Dimensions {
		style := theme.DefaultType().Row
		return widget.Label{MaxLines: 1, Alignment: text.Middle}.Layout(gtx, l.shaper, style.Face, style.Size, letter, textMaterial(gtx.Ops, ink))
	})
	return layout.Dimensions{Size: size}
}

func (l *Logos) load(key logoKey, entry *logoEntry) {
	alpha, badge, err := loadLogoAlpha(key.path, key.px)
	badge = err == nil && badge
	l.mu.Lock()
	if current := l.cache[key]; current == entry {
		entry.alpha = alpha
		entry.err = err
		entry.loading = false
		entry.badge = badge
	}
	l.mu.Unlock()
	if l.invalidate != nil {
		l.invalidate()
	}
}

func tintAlpha(alpha *image.Alpha, ink color.NRGBA) *image.RGBA {
	tinted := image.NewRGBA(alpha.Bounds())
	draw.DrawMask(tinted, tinted.Bounds(), image.NewUniform(ink), image.Point{}, alpha, alpha.Bounds().Min, draw.Src)
	return tinted
}

func isWindowsLogo(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "windows.svg" || name == "windows.png"
}

func windowsMark(gtx layout.Context, size image.Point, ink color.NRGBA) layout.Dimensions {
	d := min(size.X, size.Y)
	if d <= 0 {
		return layout.Dimensions{Size: size}
	}
	gap := max(1, int(math.Round(float64(d)*2/24)))
	tile := max(1, (d-gap)/2)
	total := tile*2 + gap
	at := image.Pt((size.X-total)/2, (size.Y-total)/2)
	for y := range 2 {
		for x := range 2 {
			minimum := at.Add(image.Pt(x*(tile+gap), y*(tile+gap)))
			FillCut(gtx.Ops, image.Rectangle{Min: minimum, Max: minimum.Add(image.Pt(tile, tile))}, 0, ink)
		}
	}
	return layout.Dimensions{Size: size}
}

// colouredArt reports art painted in more than one colour, such as a mark on
// its own disc. Its alpha is a solid shape, so it cannot be engraved as a mask.
func colouredArt(canvas *image.RGBA) bool {
	var bins [512]int
	opaque := 0
	for y := canvas.Rect.Min.Y; y < canvas.Rect.Max.Y; y++ {
		for x := canvas.Rect.Min.X; x < canvas.Rect.Max.X; x++ {
			c := canvas.RGBAAt(x, y)
			if c.A != 255 {
				continue
			}
			bins[int(c.R>>5)<<6|int(c.G>>5)<<3|int(c.B>>5)]++
			opaque++
		}
	}
	first, second := 0, 0
	for _, n := range bins {
		if n > first {
			first, second = n, first
		} else if n > second {
			second = n
		}
	}
	return opaque > 0 && second*100 >= opaque*8
}

func isSVGCommand(c byte) bool {
	return strings.IndexByte("MmZzLlHhVvCcSsQqTtAa", c) >= 0
}

func scanSVGNumber(value string, start int) (int, bool) {
	i := start
	if i < len(value) && (value[i] == '+' || value[i] == '-') {
		i++
	}
	digits := 0
	for i < len(value) && value[i] >= '0' && value[i] <= '9' {
		i++
		digits++
	}
	if i < len(value) && value[i] == '.' {
		i++
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return start, false
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
		exponent := i
		i++
		if i < len(value) && (value[i] == '+' || value[i] == '-') {
			i++
		}
		firstDigit := i
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
		if i == firstDigit {
			i = exponent
		}
	}
	return i, true
}

// normalizePathData rewrites minified path data with every token spaced out.
// Arc flags are single characters, so "0 01-.171" holds three tokens, and each
// arc in a run gets its own command letter because oksvg reads only the first
// arc of a run.
func normalizePathData(value string) string {
	var normalized strings.Builder
	normalized.Grow(len(value) + len(value)/4)
	write := func(token string) {
		if normalized.Len() > 0 {
			normalized.WriteByte(' ')
		}
		normalized.WriteString(token)
	}
	var command byte
	param := 0
	for i := 0; i < len(value); {
		c := value[i]
		if isSVGCommand(c) {
			command, param = c, 0
			write(value[i : i+1])
			i++
			continue
		}
		arc := command == 'a' || command == 'A'
		if arc && (param%7 == 3 || param%7 == 4) && (c == '0' || c == '1') {
			write(value[i : i+1])
			param++
			i++
			continue
		}
		if end, ok := scanSVGNumber(value, i); ok {
			if arc && param > 0 && param%7 == 0 {
				write(string(command))
			}
			write(value[i:end])
			param++
			i = end
			continue
		}
		i++
	}
	return normalized.String()
}

func normalizeSVGPaths(reader io.Reader) ([]byte, error) {
	decoder := xml.NewDecoder(reader)
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "path" {
			for i := range start.Attr {
				if start.Attr[i].Name.Local == "d" {
					start.Attr[i].Value = normalizePathData(start.Attr[i].Value)
				}
			}
			token = start
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func loadLogoAlpha(path string, px int) (*image.Alpha, bool, error) {
	if px <= 0 {
		return nil, false, fmt.Errorf("logo size must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	canvas := image.NewRGBA(image.Rect(0, 0, px, px))
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		normalized, err := normalizeSVGPaths(file)
		if err != nil {
			return nil, false, err
		}
		icon, err := oksvg.ReadIconStream(bytes.NewReader(normalized))
		if err != nil {
			return nil, false, err
		}
		if icon.ViewBox.W <= 0 || icon.ViewBox.H <= 0 {
			return nil, false, fmt.Errorf("SVG has no drawable view box")
		}
		scale := math.Min(float64(px)/icon.ViewBox.W, float64(px)/icon.ViewBox.H)
		width, height := icon.ViewBox.W*scale, icon.ViewBox.H*scale
		icon.SetTarget((float64(px)-width)/2, (float64(px)-height)/2, width, height)
		scanner := rasterx.NewScannerGV(px, px, canvas, canvas.Bounds())
		icon.Draw(rasterx.NewDasher(px, px, scanner), 1)
	} else {
		source, err := png.Decode(file)
		if err != nil {
			return nil, false, err
		}
		bounds := source.Bounds()
		if bounds.Empty() {
			return nil, false, fmt.Errorf("PNG has no pixels")
		}
		scale := math.Min(float64(px)/float64(bounds.Dx()), float64(px)/float64(bounds.Dy()))
		width := max(1, int(math.Round(float64(bounds.Dx())*scale)))
		height := max(1, int(math.Round(float64(bounds.Dy())*scale)))
		destination := image.Rect((px-width)/2, (px-height)/2, (px-width)/2+width, (px-height)/2+height)
		xdraw.CatmullRom.Scale(canvas, destination, source, bounds, draw.Over, nil)
	}

	alpha := image.NewAlpha(canvas.Bounds())
	for y := range px {
		for x := range px {
			alpha.SetAlpha(x, y, color.Alpha{A: canvas.RGBAAt(x, y).A})
		}
	}
	return alpha, colouredArt(canvas), nil
}
