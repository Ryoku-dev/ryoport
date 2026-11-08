package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Picker struct {
	button  widget.Clickable
	popover Popover
	menu    Menu
}

func (p *Picker) Layout(gtx layout.Context, th *theme.Theme, label string, options []string, selected int) layout.Dimensions {
	if p.button.Clicked(gtx) {
		p.popover.Open = !p.popover.Open
	}
	value := ""
	if selected >= 0 && selected < len(options) {
		value = options[selected]
	}
	anchor := func(gtx layout.Context) layout.Dimensions {
		return p.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			h := gtx.Dp(48)
			w := max(gtx.Constraints.Min.X, gtx.Dp(160))
			w = min(w, gtx.Constraints.Max.X)
			r := image.Rect(0, 0, w, h)
			FillRect(gtx.Ops, r, Radius(gtx, th.Shape.S, h), th.R.Raised)
			StrokeRect(gtx.Ops, r, Radius(gtx, th.Shape.S, h), 1, th.R.Line)
			gtx.Constraints.Min = r.Max
			gtx.Constraints.Max = r.Max
			return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(Text(th, th.T.Caption, th.R.InkDim, label)),
							layout.Rigid(Text(th, th.T.Body, th.R.Ink, value)),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sz := gtx.Dp(20)
						gtx.Constraints.Min = image.Pt(sz, sz)
						gtx.Constraints.Max = image.Pt(sz, sz)
						return icons.ExpandMore.Layout(gtx, th.R.InkDim)
					}),
				)
			})
		})
	}
	content := func(gtx layout.Context) layout.Dimensions {
		items := make([]MenuItem, len(options))
		for i, option := range options {
			items[i] = MenuItem{Label: option}
		}
		return p.menu.Layout(gtx, th, items)
	}
	return p.popover.Layout(gtx, anchor, content)
}

func (p *Picker) Changed(gtx layout.Context) (int, bool) {
	if i, ok := p.menu.Clicked(gtx); ok {
		p.popover.Open = false
		return i, true
	}
	return 0, false
}
