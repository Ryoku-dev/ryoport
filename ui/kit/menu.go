package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type MenuItem struct {
	Icon             *widget.Icon
	Label, Shortcut  string
	Danger, Disabled bool
	Divider          bool
}

type Menu struct {
	clicks   []widget.Clickable
	disabled []bool
	divider  []bool
}

func (m *Menu) prepare(items []MenuItem) {
	if len(m.clicks) != len(items) {
		m.clicks = make([]widget.Clickable, len(items))
	}
	m.disabled = make([]bool, len(items))
	m.divider = make([]bool, len(items))
	for i, item := range items {
		m.disabled[i], m.divider[i] = item.Disabled, item.Divider
	}
}

func (m *Menu) Layout(gtx layout.Context, th *theme.Theme, items []MenuItem) layout.Dimensions {
	m.prepare(items)
	gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, max(gtx.Constraints.Min.X, gtx.Dp(220)))
	rec := op.Record(gtx.Ops)
	dims := layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(items))
		for i, item := range items {
			i, item := i, item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if item.Divider {
					return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						FillRect(gtx.Ops, image.Rect(0, 0, gtx.Constraints.Max.X, 1), 0, th.R.Line)
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
					})
				}
				row := func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Dp(44)
					return layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ink := th.R.Ink
						if item.Danger {
							ink = th.R.Bad
						}
						if item.Disabled {
							ink = th.R.InkFaint
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if item.Icon == nil {
									return layout.Dimensions{}
								}
								sz := gtx.Dp(20)
								gtx.Constraints.Min = image.Pt(sz, sz)
								gtx.Constraints.Max = image.Pt(sz, sz)
								return item.Icon.Layout(gtx, ink)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if item.Icon == nil {
									return layout.Dimensions{}
								}
								return layout.Spacer{Width: unit.Dp(12)}.Layout(gtx)
							}),
							layout.Flexed(1, Text(th, th.T.Label, ink, item.Label)),
							layout.Rigid(Text(th, th.T.Caption, th.R.InkFaint, item.Shortcut)),
						)
					})
				}
				if item.Disabled {
					return m.clicks[i].Layout(gtx.Disabled(), row)
				}
				return m.clicks[i].Layout(gtx, row)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := rec.Stop()
	Shadow(gtx, image.Rectangle{Max: dims.Size}, Radius(gtx, th.Shape.M, dims.Size.Y), 3)
	FillRect(gtx.Ops, image.Rectangle{Max: dims.Size}, Radius(gtx, th.Shape.M, dims.Size.Y), th.R.Overlay)
	call.Add(gtx.Ops)
	return dims
}

func (m *Menu) Clicked(gtx layout.Context) (int, bool) {
	for i := range m.clicks {
		if !m.disabled[i] && !m.divider[i] && m.clicks[i].Clicked(gtx) {
			return i, true
		}
	}
	return 0, false
}
