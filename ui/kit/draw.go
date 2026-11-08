package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

var shadowScrim = theme.Signature(true).Scrim

func Radius(gtx layout.Context, r unit.Dp, h int) int {
	if r == theme.Full {
		return h / 2
	}
	return gtx.Dp(r)
}

func FillRect(ops *op.Ops, r image.Rectangle, radius int, c color.NRGBA) {
	if r.Empty() || c.A == 0 {
		return
	}
	paint.FillShape(ops, c, clip.UniformRRect(r, max(0, radius)).Op(ops))
}

func StrokeRect(ops *op.Ops, r image.Rectangle, radius, width int, c color.NRGBA) {
	if r.Empty() || width <= 0 || c.A == 0 {
		return
	}
	path := clip.UniformRRect(r, max(0, radius)).Path(ops)
	paint.FillShape(ops, c, clip.Stroke{Path: path, Width: float32(width)}.Op())
}

func fillCorners(ops *op.Ops, r image.Rectangle, nw, ne, se, sw int, c color.NRGBA) {
	if r.Empty() || c.A == 0 {
		return
	}
	rr := clip.RRect{Rect: r, NW: nw, NE: ne, SE: se, SW: sw}
	paint.FillShape(ops, c, rr.Op(ops))
}

func Shadow(gtx layout.Context, r image.Rectangle, radius int, elev unit.Dp) {
	e := max(0, gtx.Dp(elev))
	if e == 0 || r.Empty() {
		return
	}
	scrim := shadowScrim
	for i := 3; i >= 1; i-- {
		off := max(1, e*i/3)
		spread := max(1, e*(4-i)/4)
		rr := r.Add(image.Pt(0, off)).Inset(-spread)
		FillRect(gtx.Ops, rr, radius+spread, theme.WithAlpha(scrim, 0.04*float32(i)))
	}
}

func Surface(gtx layout.Context, c color.NRGBA, radius unit.Dp, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	FillRect(gtx.Ops, image.Rectangle{Max: dims.Size}, Radius(gtx, radius, dims.Size.Y), c)
	call.Add(gtx.Ops)
	return dims
}
