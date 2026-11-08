package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Fact struct {
	Label string
	Value string
	Mono  bool
}

func Facts(th *theme.Theme, facts []Fact, minCol unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(facts) == 0 {
			return layout.Dimensions{}
		}
		minWidth := max(1, gtx.Dp(minCol))
		cols := max(1, gtx.Constraints.Max.X/minWidth)
		cols = min(cols, len(facts))
		rows := (len(facts) + cols - 1) / cols
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, func() []layout.FlexChild {
			children := make([]layout.FlexChild, 0, rows)
			for row := range rows {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, func() []layout.FlexChild {
						cells := make([]layout.FlexChild, 0, cols)
						for col := range cols {
							idx := row*cols + col
							cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								if idx >= len(facts) {
									return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
								}
								f := facts[idx]
								value := f.Value
								if value == "" {
									value = "-"
								}
								style := th.T.Body
								if f.Mono {
									style = th.T.Mono
								}
								return layout.Inset{Right: th.Sp.S4, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(Text(th, th.T.Caption, th.R.InkDim, f.Label)),
										layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
										layout.Rigid(Text(th, style, th.R.Ink, value)),
									)
								})
							}))
						}
						return cells
					}()...)
				}))
			}
			return children
		}()...)
	}
}
