package kit

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type EmptyAction struct {
	Button *Button
	Kind   ButtonKind
	Icon   *widget.Icon
	Label  string
}

func Empty(th *theme.Theme, icon *widget.Icon, title, body string, actions ...EmptyAction) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := gtx.Dp(72)
					path := ShapePath(gtx.Ops, f32.Pt(float32(d)/2, float32(d)/2), float32(d)/2, SoftSquare, SoftSquare, 1, 0)
					paint.FillShape(gtx.Ops, th.R.Raised, clip.Outline{Path: path}.Op())
					if icon != nil {
						sealCtx := gtx
						sealCtx.Constraints.Min = image.Pt(d, d)
						sealCtx.Constraints.Max = image.Pt(d, d)
						layout.Center.Layout(sealCtx, func(gtx layout.Context) layout.Dimensions {
							sz := gtx.Dp(32)
							gtx.Constraints.Min = image.Pt(sz, sz)
							gtx.Constraints.Max = image.Pt(sz, sz)
							return icon.Layout(gtx, th.R.InkDim)
						})
					}
					return layout.Dimensions{Size: image.Pt(d, d)}
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(Text(th, th.T.Title, th.R.Ink, title)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
					return Label{Style: th.T.Body, Color: th.R.InkDim, MaxLines: 0}.Layout(gtx, th, body)
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(actions)*2)
					for _, action := range actions {
						action := action
						if action.Button == nil {
							continue
						}
						children = append(children,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return action.Button.Layout(gtx, th, action.Kind, action.Icon, action.Label)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						)
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				}),
			)
		})
	}
}
