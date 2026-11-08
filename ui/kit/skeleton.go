package kit

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func Skeleton(th *theme.Theme, lines int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, func() []layout.FlexChild {
			rows := make([]layout.FlexChild, 0, max(0, lines))
			for range max(0, lines) {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					h := gtx.Dp(th.RowHeight())
					gtx.Constraints.Min.Y = h
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							d := gtx.Dp(40)
							FillRect(gtx.Ops, image.Rect(0, 0, d, d), Radius(gtx, th.Shape.M, d), th.R.Raised)
							return layout.Dimensions{Size: image.Pt(d, d)}
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							w := gtx.Constraints.Max.X
							barH := gtx.Dp(8)
							first, second := int(float32(w)*.6), int(float32(w)*.35)
							FillRect(gtx.Ops, image.Rect(0, 0, first, barH), barH/2, th.R.Raised)
							y := barH + gtx.Dp(8)
							FillRect(gtx.Ops, image.Rect(0, y, second, y+barH), barH/2, th.R.Raised)
							return layout.Dimensions{Size: image.Pt(w, y+barH)}
						}),
					)
				}))
			}
			return rows
		}()...)
	}
}
