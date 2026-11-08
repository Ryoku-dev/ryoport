package kit

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Metric struct {
	Title   string
	Value   string
	Unit    string
	Caption string
	Vals    []float64
	Lo      float64
	Hi      float64
	Color   color.NRGBA
}

func MetricCard(th *theme.Theme, m Metric) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return Surface(gtx, th.R.Panel, th.Shape.L, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(Text(th, th.T.Caption, th.R.InkDim, m.Title)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(Text(th, th.T.Title, th.R.Ink, m.Value)),
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(Text(th, th.T.Caption, th.R.InkDim, m.Unit)),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if m.Caption == "" {
							return layout.Dimensions{}
						}
						return Text(th, th.T.Caption, th.R.InkDim, m.Caption)(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(m.Vals) < 2 {
							gtx.Constraints.Min.Y = gtx.Dp(72)
							return layout.Center.Layout(gtx, Text(th, th.T.Caption, th.R.InkFaint, "Collecting"))
						}
						return AreaChart(gtx, th, m.Vals, m.Lo, m.Hi, m.Color, 72)
					}),
				)
			})
		})
	}
}
