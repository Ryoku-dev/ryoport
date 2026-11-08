package kit

import (
	"strconv"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func GroupHeader(th *theme.Theme, title string, count int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 14, Bottom: 6, Left: 6, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, Text(th, th.T.Group, th.R.Fg3, title)),
				layout.Rigid(Text(th, th.T.MonoSm, th.R.Fg3, strconv.Itoa(count))),
			)
		})
	}
}
