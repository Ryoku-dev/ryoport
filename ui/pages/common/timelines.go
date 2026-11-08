package common

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type TimeLine struct {
	Text string
	At   time.Time
}

func TimeLines(e *env.Env, lines []TimeLine) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(lines) == 0 {
			return layout.Dimensions{}
		}
		children := make([]layout.FlexChild, 0, len(lines)*2-1)
		for i, line := range lines {
			line := line
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
						layout.Flexed(1, kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, line.Text)),
						layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, format.Ago(line.At, e.Now))),
					)
				})
			}))
			if i != len(lines)-1 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					height := max(1, gtx.Dp(unit.Dp(1)))
					size := image.Pt(gtx.Constraints.Max.X, height)
					paint.FillShape(gtx.Ops, e.Th.R.Vein, clip.Rect{Max: size}.Op())
					return layout.Dimensions{Size: size}
				}))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
