package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func AreaChart(gtx layout.Context, th *theme.Theme, vals []float64, lo, hi float64, c color.NRGBA, height unit.Dp) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(height))
	size = gtx.Constraints.Constrain(size)
	guide := theme.WithAlpha(th.R.Line, .65)
	for i := 1; i <= 3; i++ {
		y := size.Y * i / 4
		paint.FillShape(gtx.Ops, guide, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(size.X, y+1)}.Op())
	}
	return Sparkline(gtx, th, vals, lo, hi, c, size)
}
