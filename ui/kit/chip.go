package kit

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Chip struct {
	widget.Clickable
}

func (c *Chip) Layout(gtx layout.Context, th *theme.Theme, label string, selected bool, dot color.NRGBA) layout.Dimensions {
	h := gtx.Dp(32)
	dims := c.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		bg, ink := th.R.Raised, th.R.Ink
		if selected {
			bg, ink = th.R.Selected, th.R.Ink
		}
		if c.Hovered() {
			bg = theme.Mix(bg, ink, 0.08)
		}
		if c.Pressed() {
			bg = theme.Mix(bg, ink, 0.12)
		}
		bg, ink = disabledInk(gtx, bg), disabledInk(gtx, ink)
		m := op.Record(gtx.Ops)
		content := layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 3)
			if selected {
				children = append(children, layout.Rigid(iconWidget(icons.Check, 16, ink)), layout.Rigid(layout.Spacer{Width: 6}.Layout))
			} else if dot.A > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := gtx.Dp(8)
					FillRect(gtx.Ops, image.Rect(0, 0, d, d), d/2, disabledInk(gtx, dot))
					return layout.Dimensions{Size: image.Pt(d, d)}
				}), layout.Rigid(layout.Spacer{Width: 6}.Layout))
			}
			children = append(children, layout.Rigid(Text(th, th.T.Label, ink, label)))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		call := m.Stop()
		size := gtx.Constraints.Constrain(image.Pt(content.Size.X, h))
		r := image.Rectangle{Max: size}
		FillRect(gtx.Ops, r, gtx.Dp(th.Shape.S), bg)
		off := op.Offset(image.Pt((size.X-content.Size.X)/2, (h-content.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: size}
	})
	if gtx.Focused(&c.Clickable) {
		FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, gtx.Dp(th.Shape.S))
	}
	return dims
}
