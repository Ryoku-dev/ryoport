package kit

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func Badge(th *theme.Theme, count int, c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if count <= 0 {
			d := gtx.Dp(8)
			paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
			return layout.Dimensions{Size: image.Pt(d, d)}
		}
		label := fmt.Sprint(count)
		if count > 99 {
			label = "99+"
		}
		return Surface(gtx, c, theme.Full, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(18)
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, Text(th, th.T.Group, th.R.OnAccent, label))
				})
		})
	}
}
