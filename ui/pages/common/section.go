package common

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func SectionTitle(th *theme.Theme, title string, trailing layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := kit.Text(th, th.T.Title, th.R.Fg, title)
			if trailing == nil {
				return label(gtx)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(th.Sp.S3)}.Layout(gtx,
				layout.Flexed(1, label),
				layout.Rigid(trailing),
			)
		})
	}
}
