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
		children := make([]layout.FlexChild, 0, 3)
		if label != "" {
			children = append(children,
				layout.Rigid(Text(th, th.T.Caption, th.R.Fg3, label)),
				layout.Rigid(layout.Spacer{Height: 4}.Layout),
			)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				height := gtx.Dp(44)
				width := max(gtx.Constraints.Min.X, gtx.Dp(160))
				width = min(width, gtx.Constraints.Max.X)
				r := image.Rect(0, 0, width, height)
				fill := th.R.Stone3
				if p.button.Hovered() {
					fill = theme.Mix(fill, th.R.Fg, .06)
				}
				FillCut(gtx.Ops, r, gtx.Dp(th.Cut.Mid), fill)
				gtx.Constraints = layout.Exact(r.Max)
				return layout.Inset{Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, Text(th, th.T.Body, th.R.Fg, value)),
							layout.Rigid(layout.Spacer{Width: 10}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								size := gtx.Dp(20)
								gtx.Constraints = layout.Exact(image.Pt(size, size))
								return icons.ExpandMore.Layout(gtx, th.R.Fg2)
							}),
						)
					})
				})
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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
	if selected, ok := p.menu.Clicked(gtx); ok {
		p.popover.Open = false
		return selected, true
	}
	return 0, false
}
