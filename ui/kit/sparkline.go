package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func chartRange(vals []float64, lo, hi float64) (float64, float64) {
	if math.IsNaN(lo) {
		lo = math.Inf(1)
		for _, v := range vals {
			if !math.IsNaN(v) {
				lo = min(lo, v)
			}
		}
	}
	if math.IsNaN(hi) {
		hi = math.Inf(-1)
		for _, v := range vals {
			if !math.IsNaN(v) {
				hi = max(hi, v)
			}
		}
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		lo, hi = 0, 1
	}
	if hi <= lo {
		pad := math.Max(1, math.Abs(lo)*.05)
		lo, hi = lo-pad, hi+pad
	}
	return lo, hi
}

func chartPoints(vals []float64, lo, hi float64, size image.Point) []f32.Point {
	pts := make([]f32.Point, 0, len(vals))
	for i, v := range vals {
		if math.IsNaN(v) {
			continue
		}
		x := float32(i) * float32(max(0, size.X-1)) / float32(max(1, len(vals)-1))
		y := float32(size.Y-1) * (1 - float32((v-lo)/(hi-lo)))
		pts = append(pts, f32.Pt(x, max(0, min(float32(size.Y-1), y))))
	}
	return pts
}

func strokePoints(ops layout.Context, pts []f32.Point, width float32, c color.NRGBA) {
	if len(pts) < 2 {
		return
	}
	var p clip.Path
	p.Begin(ops.Ops)
	p.MoveTo(pts[0])
	for _, pt := range pts[1:] {
		p.LineTo(pt)
	}
	paint.FillShape(ops.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func Sparkline(gtx layout.Context, th *theme.Theme, vals []float64, lo, hi float64, c color.NRGBA, size image.Point) layout.Dimensions {
	size = gtx.Constraints.Constrain(size)
	if len(vals) < 2 {
		y := float32(size.Y) / 2
		step, dash := max(2, gtx.Dp(6)), max(1, gtx.Dp(3))
		for x := 0; x < size.X; x += step {
			paint.FillShape(gtx.Ops, th.R.Line, clip.Rect{Min: image.Pt(x, int(y)), Max: image.Pt(min(size.X, x+dash), int(y)+1)}.Op())
		}
		return layout.Dimensions{Size: size}
	}
	lo, hi = chartRange(vals, lo, hi)
	pts := chartPoints(vals, lo, hi, size)
	if len(pts) < 2 {
		return layout.Dimensions{Size: size}
	}
	var area clip.Path
	area.Begin(gtx.Ops)
	area.MoveTo(f32.Pt(pts[0].X, float32(size.Y)))
	area.LineTo(pts[0])
	for _, pt := range pts[1:] {
		area.LineTo(pt)
	}
	area.LineTo(f32.Pt(pts[len(pts)-1].X, float32(size.Y)))
	area.LineTo(f32.Pt(pts[0].X, float32(size.Y)))
	paint.FillShape(gtx.Ops, theme.WithAlpha(c, .14), clip.Outline{Path: area.End()}.Op())
	strokePoints(gtx, pts, float32(gtx.Dp(1.5)), c)
	d := gtx.Dp(6)
	last := pts[len(pts)-1]
	defer clip.Ellipse{Min: image.Pt(int(last.X)-d/2, int(last.Y)-d/2), Max: image.Pt(int(last.X)+(d+1)/2, int(last.Y)+(d+1)/2)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, c)
	return layout.Dimensions{Size: size}
}
