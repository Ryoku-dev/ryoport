package kit

import (
	"image"
	"math"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

// List is a vertical widget.List with Lithic's thin scroll thumb. Wheel and
// touchpad scrolling are Gio's own: on Wayland one wheel notch moves about
// 150 px and a touchpad follows the fingers.
type List struct {
	widget.List
}

func scrollThumb(viewport, content, first, offset, count int) (top, height int, ok bool) {
	if viewport <= 0 || content <= viewport || count <= 0 {
		return 0, 0, false
	}
	height = max(12, viewport*viewport/content)
	height = min(viewport, height)
	travel := viewport - height
	average := float64(content) / float64(count)
	scrolled := float64(first)*average + float64(offset)
	maxScroll := float64(content - viewport)
	if maxScroll > 0 {
		top = int(math.Round(float64(travel) * max(0, min(1, scrolled/maxScroll))))
	}
	return top, height, true
}

func (l *List) Layout(gtx layout.Context, th *theme.Theme, n int, el layout.ListElement) layout.Dimensions {
	l.Axis = layout.Vertical
	dims := l.List.Layout(gtx, n, el)
	top, thumbH, ok := scrollThumb(dims.Size.Y, l.Position.Length, l.Position.First, l.Position.Offset, n)
	if !ok {
		return dims
	}
	width := max(1, gtx.Dp(3))
	x := dims.Size.X - width
	cut := min(width/2, thumbH/2)
	FillCut(gtx.Ops, image.Rect(x, top, x+width, top+thumbH), cut, th.R.Vein)
	return dims
}
