package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func StatusPill(th *theme.Theme, tone model.Tone, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		ink, soft := th.StateInk(int(tone))
		h := gtx.Dp(24)
		return Surface(gtx, soft, theme.Full, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = h
			return layout.Inset{Left: 8, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						d := gtx.Dp(8)
						paint.FillShape(gtx.Ops, ink, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
						return layout.Dimensions{Size: image.Pt(d, d)}
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(Text(th, th.T.Label, ink, label)),
				)
			})
		})
	}
}
