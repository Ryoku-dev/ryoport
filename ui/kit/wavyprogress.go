package kit

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func WavyProgress(gtx layout.Context, th *theme.Theme, frac float64, active bool) layout.Dimensions {
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, gtx.Dp(10)))
	phase := float32(0)
	if active || frac < 0 {
		phase = anim.Phase(gtx, 1400*time.Millisecond)
	}
	amp := float32(gtx.Dp(2.5))
	waveLen := float32(max(1, gtx.Dp(22)))
	cy := float32(size.Y) / 2
	start, done := float32(0), float32(size.X)*float32(max(0, min(1, frac)))
	if frac < 0 {
		segment := max(float32(gtx.Dp(44)), float32(size.X)*.28)
		start = (float32(size.X)+segment)*phase - segment
		done = start + segment
		start, done = max(0, start), min(float32(size.X), done)
	}
	if done > start {
		var p clip.Path
		p.Begin(gtx.Ops)
		for x := start; x <= done; x++ {
			y := cy + amp*float32(math.Sin(float64((x/waveLen+phase)*2*math.Pi)))
			if x == start {
				p.MoveTo(f32.Pt(x, y))
			} else {
				p.LineTo(f32.Pt(x, y))
			}
		}
		paint.FillShape(gtx.Ops, th.R.Accent, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(3))}.Op())
		capD := max(1, gtx.Dp(3))
		for _, x := range []float32{start, done} {
			y := cy + amp*float32(math.Sin(float64((x/waveLen+phase)*2*math.Pi)))
			r := capD / 2
			paint.FillShape(gtx.Ops, th.R.Accent, clip.Ellipse{
				Min: image.Pt(int(x)-r, int(y)-r),
				Max: image.Pt(int(x)+(capD+1)/2, int(y)+(capD+1)/2),
			}.Op(gtx.Ops))
		}
	}
	if frac >= 0 {
		gap := float32(gtx.Dp(4))
		trackStart := min(float32(size.X), done+gap)
		if trackStart < float32(size.X) {
			y := int(cy) - gtx.Dp(3)/2
			FillRect(gtx.Ops, image.Rect(int(trackStart), y, size.X, y+gtx.Dp(3)), gtx.Dp(2), th.R.Raised)
		}
		d := gtx.Dp(8)
		paint.FillShape(gtx.Ops, th.R.Raised, clip.Ellipse{Min: image.Pt(size.X-d, int(cy)-d/2), Max: image.Pt(size.X, int(cy)+(d+1)/2)}.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: size}
}
