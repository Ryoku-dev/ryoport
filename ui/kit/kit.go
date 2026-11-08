// Package kit provides Ryoport's Gio component library.
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

func disabledInk(gtx layout.Context, c color.NRGBA) color.NRGBA {
	if !gtx.Enabled() {
		return theme.WithAlpha(c, 0.38)
	}
	return c
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }
