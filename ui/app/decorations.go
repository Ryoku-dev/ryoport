package app

import (
	"image"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type decorations struct {
	window   *gioapp.Window
	minimize kit.IconButton
	maximize kit.IconButton
	close    kit.IconButton
}

func (d *decorations) MoveArea(ops *op.Ops) {
	system.ActionInputOp(system.ActionMove).Add(ops)
}

func (d *decorations) Layout(gtx layout.Context, th *theme.Theme, mode gioapp.WindowMode) {
	macro := op.Record(gtx.Ops)
	controlsWidth := gtx.Dp(unit.Dp(108))
	controlsHeight := gtx.Dp(unit.Dp(32))
	at := image.Pt(gtx.Constraints.Max.X-controlsWidth-gtx.Dp(unit.Dp(6)), 0)
	layoutAt(gtx, at, image.Pt(controlsWidth, controlsHeight), func(gtx layout.Context) layout.Dimensions {
		d.minimize.Small = true
		d.maximize.Small = true
		d.close.Small = true
		maximizeIcon := icons.CropSquare
		maximizeAction := system.ActionMaximize
		maximizeLabel := "Maximize"
		if mode == gioapp.Maximized {
			maximizeIcon = icons.FilterNone
			maximizeAction = system.ActionUnmaximize
			maximizeLabel = "Restore"
		}
		dims := (layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(4))}).Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.minimize.Layout(gtx, th, kit.Quiet, icons.Minimize, "Minimize")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.maximize.Layout(gtx, th, kit.Quiet, maximizeIcon, maximizeLabel)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.close.Layout(gtx, th, kit.Quiet, icons.Close, "Close")
			}),
		)
		if d.minimize.Clicked(gtx) {
			d.window.Perform(system.ActionMinimize)
		}
		if d.maximize.Clicked(gtx) {
			d.window.Perform(maximizeAction)
		}
		if d.close.Clicked(gtx) {
			d.window.Perform(system.ActionClose)
		}
		return dims
	})
	call := macro.Stop()
	op.Defer(gtx.Ops, call)
}

func (s *shell) customDecorations() bool {
	return !s.config.Decorated && s.config.Mode != gioapp.Fullscreen
}
