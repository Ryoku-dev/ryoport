package kit

import (
	"image"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func FocusRing(gtx layout.Context, th *theme.Theme, r image.Rectangle, radius int) {
	gap := gtx.Dp(2)
	width := max(1, gtx.Dp(2))
	StrokeRect(gtx.Ops, r.Inset(-gap), radius+gap, width, th.R.Accent)
}
