package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type FAB struct {
	widget.Clickable
	morph anim.Spring
}

func (f *FAB) Layout(gtx layout.Context, th *theme.Theme, icon *widget.Icon, open bool) layout.Dimensions {
	f.morph.Omega = 20
	if open {
		f.morph.Set(1)
	} else {
		f.morph.Set(0)
	}
	p := f.morph.Value(gtx)
	sz := gtx.Dp(56)
	radius := int(lerp(float32(gtx.Dp(th.Shape.L)), float32(sz/2), p) + 0.5)
	m := op.Record(gtx.Ops)
	dims := f.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		desc := "Create"
		if open {
			desc = "Close"
		}
		semantic.DescriptionOp(desc).Add(gtx.Ops)
		r := image.Rect(0, 0, sz, sz)
		bg := disabledInk(gtx, th.R.Bone)
		if f.Hovered() {
			bg = theme.Mix(bg, disabledInk(gtx, th.R.OnBone), 0.08)
		}
		if f.Pressed() {
			bg = theme.Mix(bg, disabledInk(gtx, th.R.OnBone), 0.12)
		}
		FillRect(gtx.Ops, r, radius, bg)
		is := gtx.Dp(24)
		off := op.Offset(image.Pt((sz-is)/2, (sz-is)/2)).Push(gtx.Ops)
		if icon != nil && p < 1 {
			alpha := paint.PushOpacity(gtx.Ops, 1-p)
			icon.Layout(exact(gtx, image.Pt(is, is)), disabledInk(gtx, th.R.OnBone))
			alpha.Pop()
		}
		if p > 0 {
			alpha := paint.PushOpacity(gtx.Ops, p)
			icons.Close.Layout(exact(gtx, image.Pt(is, is)), disabledInk(gtx, th.R.OnBone))
			alpha.Pop()
		}
		off.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(sz, sz))}
	})
	call := m.Stop()
	Shadow(gtx, image.Rectangle{Max: dims.Size}, radius, 8)
	call.Add(gtx.Ops)
	if gtx.Focused(&f.Clickable) {
		FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, radius)
	}
	return dims
}
