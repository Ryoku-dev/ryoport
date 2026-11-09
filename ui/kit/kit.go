// Package kit provides Ryoport's Lithic Gio components.
package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func exact(gtx layout.Context, size image.Point) layout.Context {
	size.X = min(max(0, size.X), gtx.Constraints.Max.X)
	size.Y = min(max(0, size.Y), gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)
	return gtx
}

func constrainMax(gtx layout.Context, size image.Point) image.Point {
	size.X = min(max(0, size.X), gtx.Constraints.Max.X)
	size.Y = min(max(0, size.Y), gtx.Constraints.Max.Y)
	return size
}

func iconWidget(icon *widget.Icon, size unit.Dp, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if icon == nil {
			return layout.Dimensions{}
		}
		sz := gtx.Dp(size)
		return icon.Layout(exact(gtx, image.Pt(sz, sz)), ink)
	}
}

func textMaterial(ops *op.Ops, c color.NRGBA) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: c}.Add(ops)
	return m.Stop()
}

// disabledAlpha keeps a disabled control readable on light stone and on a
// mineral field while still reading as unavailable; Material's 38 percent
// left grey type on a pale field below 2:1.
const disabledAlpha = 0.5

func disabledInk(gtx layout.Context, c color.NRGBA) color.NRGBA {
	if !gtx.Source.Enabled() {
		return theme.WithAlpha(c, disabledAlpha)
	}
	return c
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}
