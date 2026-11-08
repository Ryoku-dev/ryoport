package kit

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type logoKey struct {
	path string
	px   int
}

type logoEntry struct {
	img     image.Image
	err     error
	loading bool
}

type Logos struct {
	mu         sync.Mutex
	cache      map[logoKey]*logoEntry
	invalidate func()
}

func NewLogos(invalidate func()) *Logos {
	return &Logos{cache: make(map[logoKey]*logoEntry), invalidate: invalidate}
}

func (l *Logos) Widget(path string, size unit.Dp, fallback string, th *theme.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		px := max(1, gtx.Dp(size))
		key := logoKey{path: path, px: px}
		l.mu.Lock()
		entry, ok := l.cache[key]
		if !ok {
			entry = &logoEntry{loading: true}
			l.cache[key] = entry
			go l.load(key, entry)
		}
		img, err, loading := entry.img, entry.err, entry.loading
		l.mu.Unlock()
		gtx.Constraints.Min = image.Pt(px, px)
		gtx.Constraints.Max = image.Pt(px, px)
		if loading {
			return layout.Dimensions{Size: image.Pt(px, px)}
		}
		if err != nil || img == nil {
			letter := "?"
			if s := strings.TrimSpace(fallback); s != "" {
				letter = strings.ToUpper(string([]rune(s)[0]))
			}
			return layout.Center.Layout(gtx, Text(th, th.T.Row, th.R.InkDim, letter))
		}
		scale := float32(1)
		if gtx.Metric.PxPerDp > 0 {
			scale = 1 / gtx.Metric.PxPerDp
		}
		im := widget.Image{Src: paint.NewImageOp(img), Fit: widget.Contain, Position: layout.Center, Scale: scale}
		return im.Layout(gtx)
	}
}

func (l *Logos) load(key logoKey, entry *logoEntry) {
	img, err := loadLogo(key.path, key.px)
	l.mu.Lock()
	entry.img, entry.err, entry.loading = img, err, false
	l.mu.Unlock()
	if l.invalidate != nil {
		l.invalidate()
	}
}

func loadLogo(path string, px int) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		icon, err := oksvg.ReadIconStream(f)
		if err != nil {
			return nil, err
		}
		img := image.NewRGBA(image.Rect(0, 0, px, px))
		icon.SetTarget(0, 0, float64(px), float64(px))
		scanner := rasterx.NewScannerGV(px, px, img, img.Bounds())
		icon.Draw(rasterx.NewDasher(px, px, scanner), 1)
		return img, nil
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst, nil
}
