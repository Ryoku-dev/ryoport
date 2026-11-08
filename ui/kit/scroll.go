package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type List struct {
	widget.List
}

func (l *List) Layout(gtx layout.Context, th *theme.Theme, n int, el layout.ListElement) layout.Dimensions {
	l.Axis = layout.Vertical
	dims := l.List.Layout(gtx, n, el)
	if l.Position.Length <= dims.Size.Y || dims.Size.Y <= 0 || n <= 0 {
		return dims
	}
	track := dims.Size.Y
	thumbH := max(gtx.Dp(24), track*track/l.Position.Length)
	thumbH = min(track, thumbH)
	maxTop := track - thumbH
	denom := max(1, n-l.Position.Count)
	top := maxTop * min(l.Position.First, denom) / denom
	w := max(1, gtx.Dp(4))
	x := dims.Size.X - w - gtx.Dp(2)
	FillRect(gtx.Ops, image.Rect(x, top, x+w, top+thumbH), w/2, th.R.InkFaint)
	return dims
}
