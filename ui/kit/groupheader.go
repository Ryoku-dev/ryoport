package kit

import (
	"strconv"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func GroupHeader(th *theme.Theme, title string, count int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = min(gtx.Constraints.Max.Y, gtx.Dp(32))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, Text(th, th.T.Group, th.R.InkDim, title)),
				layout.Rigid(Text(th, th.T.Group, th.R.InkFaint, strconv.Itoa(count))),
			)
		})
	}
}
