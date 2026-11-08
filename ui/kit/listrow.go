package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type ListRow struct {
	widget.Clickable
}

type RowContent struct {
	Leading, Trailing layout.Widget
	Title, Subtitle   string
	SubtitleMono      bool
}

func (r *ListRow) DoubleClicked(gtx layout.Context) bool {
	for {
		click, ok := r.Update(gtx)
		if !ok {
			return false
		}
		if click.NumClicks >= 2 {
			return true
		}
	}
}

func (r *ListRow) Layout(gtx layout.Context, th *theme.Theme, c RowContent, selected bool) layout.Dimensions {
	h := gtx.Dp(th.RowHeight())
	w := gtx.Constraints.Max.X
	if w <= 0 {
		w = gtx.Dp(320)
	}
	dims := r.Clickable.Layout(exact(gtx, image.Pt(w, h)), func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		rect := image.Rectangle{Max: size}
		if selected {
			FillRect(gtx.Ops, rect, gtx.Dp(th.Shape.L), disabledInk(gtx, th.R.Selected))
		} else if r.Hovered() {
			FillRect(gtx.Ops, rect, gtx.Dp(th.Shape.L), disabledInk(gtx, theme.WithAlpha(th.R.Raised, 0.60)))
		}
		if r.Pressed() {
			FillRect(gtx.Ops, rect, gtx.Dp(th.Shape.L), disabledInk(gtx, theme.WithAlpha(th.R.Ink, 0.08)))
		}
		return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			leading := func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(40), gtx.Dp(40)))
				if c.Leading == nil {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}
				return layout.Center.Layout(gtx, c.Leading)
			}
			textBlock := func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{layout.Rigid(Text(th, th.T.Row, disabledInk(gtx, th.R.Ink), c.Title))}
				if c.Subtitle != "" {
					style := th.T.Caption
					if c.SubtitleMono {
						style = th.T.MonoSm
					}
					children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout), layout.Rigid(Text(th, style, disabledInk(gtx, th.R.InkDim), c.Subtitle)))
				}
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
			}
			children := []layout.FlexChild{
				layout.Rigid(leading),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.W.Layout(gtx, textBlock) }),
			}
			if c.Trailing != nil {
				children = append(children, layout.Rigid(layout.Spacer{Width: 12}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, c.Trailing)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
	if gtx.Focused(&r.Clickable) {
		FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, gtx.Dp(th.Shape.L))
	}
	return dims
}
