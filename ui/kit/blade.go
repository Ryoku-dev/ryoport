package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func BladeMeter(gtx layout.Context, th *theme.Theme, frac float64, c color.NRGBA) layout.Dimensions {
	height := max(1, gtx.Dp(6))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	FillCut(gtx.Ops, image.Rectangle{Max: size}, 0, th.R.Stone3)
	filled := int(float64(size.X) * max(0, min(1, frac)))
	if filled > 0 {
		Blade(gtx.Ops, image.Rect(0, 0, filled, size.Y), c)
	}
	return layout.Dimensions{Size: size}
}

func BladeProgress(gtx layout.Context, th *theme.Theme, frac float64, active bool) layout.Dimensions {
	trackH := max(1, gtx.Dp(6))
	markH := max(trackH, gtx.Dp(10))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, markH))
	trackY := (size.Y - min(size.Y, trackH)) / 2
	track := image.Rect(0, trackY, size.X, trackY+min(size.Y, trackH))
	FillCut(gtx.Ops, track, 0, th.R.Stone3)

	start, end := 0, 0
	if frac < 0 {
		if active && size.X > 0 {
			phase := anim.Phase(gtx, 4*th.Motion.Field)
			segment := max(1, size.X/4)
			start = int(float32(size.X+segment)*phase) - segment
			end = start + segment
		}
	} else {
		end = int(float64(size.X) * max(0, min(1, frac)))
	}
	if end > start {
		stack := clip.Rect(track).Push(gtx.Ops)
		Blade(gtx.Ops, image.Rect(start, track.Min.Y, end, track.Max.Y), th.R.Lapis)
		stack.Pop()
	}
	if active && end > 0 && size.X > 0 {
		markW := min(size.X, max(1, gtx.Dp(2)))
		x := max(0, min(size.X-markW, end-markW/2))
		FillCut(gtx.Ops, image.Rect(x, 0, x+markW, size.Y), 0, th.R.Gold)
	}
	return layout.Dimensions{Size: size}
}
