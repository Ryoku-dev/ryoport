package kit

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

var brandRed = color.NRGBA{R: 0xe2, G: 0x34, B: 0x2a, A: 0xff}

func BrandMark(th *theme.Theme, size unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		d := gtx.Dp(size)
		sz := gtx.Constraints.Constrain(image.Pt(d, d))
		d = min(sz.X, sz.Y)
		FillRect(gtx.Ops, image.Rect(0, 0, d, d), int(float32(d)*.22), th.R.Bone)
		cx, cy := d/2, d/2
		outer, inner := int(float32(d)*17/64), int(float32(d)*13/64)
		paint.FillShape(gtx.Ops, th.R.OnBone, clip.Ellipse{Min: image.Pt(cx-outer, cy-outer), Max: image.Pt(cx+outer, cy+outer)}.Op(gtx.Ops))
		paint.FillShape(gtx.Ops, th.R.Bone, clip.Ellipse{Min: image.Pt(cx-inner, cy-inner), Max: image.Pt(cx+inner, cy+inner)}.Op(gtx.Ops))
		dot := max(1, int(float32(d)*3/64))
		paint.FillShape(gtx.Ops, th.R.OnBone, clip.Ellipse{Min: image.Pt(cx-dot, cy-dot), Max: image.Pt(cx+dot, cy+dot)}.Op(gtx.Ops))
		tickW := max(1, int(float32(d)*3/64))
		tickA, tickB := int(float32(d)*9/64), int(float32(d)*17/64)
		FillRect(gtx.Ops, image.Rect(cx-tickW/2, tickA, cx+(tickW+1)/2, tickB), tickW/2, th.R.OnBone)
		FillRect(gtx.Ops, image.Rect(cx-tickW/2, d-tickB, cx+(tickW+1)/2, d-tickA), tickW/2, th.R.OnBone)
		FillRect(gtx.Ops, image.Rect(tickA, cy-tickW/2, tickB, cy+(tickW+1)/2), tickW/2, th.R.OnBone)
		FillRect(gtx.Ops, image.Rect(d-tickB, cy-tickW/2, d-tickA, cy+(tickW+1)/2), tickW/2, th.R.OnBone)
		scale := float32(d) / 64
		var pointer clip.Path
		pointer.Begin(gtx.Ops)
		pointer.MoveTo(f32.Pt(22*scale, 42*scale))
		pointer.LineTo(f32.Pt(30*scale, 30*scale))
		pointer.LineTo(f32.Pt(42*scale, 22*scale))
		pointer.LineTo(f32.Pt(34*scale, 34*scale))
		pointer.LineTo(f32.Pt(22*scale, 42*scale))
		path := pointer.End()
		paint.FillShape(gtx.Ops, th.R.OnBone, clip.Stroke{Path: path, Width: max(1, 2*scale)}.Op())
		paint.FillShape(gtx.Ops, brandRed, clip.Outline{Path: path}.Op())
		return layout.Dimensions{Size: sz}
	}
}
