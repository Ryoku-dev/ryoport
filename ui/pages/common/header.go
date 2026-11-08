package common

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type HeaderContent struct {
	Tone    model.Tone
	Mark    layout.Widget
	Name    string
	Outline float64
	Proof   []Span
	Note    string
	Acts    layout.Widget
	Below   layout.Widget
}

type Header struct {
	field kit.Field
}

func (h *Header) Layout(gtx layout.Context, e *env.Env, c HeaderContent) layout.Dimensions {
	th := e.Th
	return h.field.Layout(gtx, th, c.Tone, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 30, Right: 36, Bottom: 26, Left: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 5)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				style := th.T.Name
				if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(640)) {
					style = th.T.Sentence
				}
				name := func(gtx layout.Context) layout.Dimensions {
					if c.Outline >= 0 {
						return kit.OutlineText(gtx, th, style, c.Name, float32(c.Outline), th.R.Fg, th.R.Stone(c.Tone))
					}
					return kit.Label{Style: style, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th, c.Name)
				}
				row := make([]layout.FlexChild, 0, 2)
				if c.Mark != nil {
					row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(unit.Dp(44))
						gtx.Constraints = layout.Exact(image.Pt(size, size))
						return c.Mark(gtx)
					}))
				}
				row = append(row, layout.Flexed(1, name))
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(th.Sp.S3)}.Layout(gtx, row...)
			}))
			if len(c.Proof) > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return Sentence(gtx, th, th.T.Proof, th.R.Fg2, c.Proof)
					})
				}))
			}
			if c.Note != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S1}.Layout(gtx, kit.Text(th, th.T.Caption, th.R.Fg3, c.Note))
				}))
			}
			if c.Acts != nil {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S4}.Layout(gtx, c.Acts)
				}))
			}
			if c.Below != nil {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S4}.Layout(gtx, c.Below)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
