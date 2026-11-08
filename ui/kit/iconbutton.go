package kit

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type IconKind int

const (
	Standard IconKind = iota
	TonalIcon
	FilledIcon
)

type IconButton struct {
	widget.Clickable
	Dot  color.NRGBA
	Busy bool
}

func (b *IconButton) Clicked(gtx layout.Context) bool {
	if !b.Busy {
		return b.Clickable.Clicked(gtx)
	}
	for b.Clickable.Clicked(gtx) {
	}
	return false
}

func (b *IconButton) Layout(gtx layout.Context, th *theme.Theme, kind IconKind, icon *widget.Icon, desc string) layout.Dimensions {
	const size = unit.Dp(40)
	sz := gtx.Dp(size)
	interactionGTX := gtx
	if b.Busy {
		interactionGTX = interactionGTX.Disabled()
	}
	renderGTX := gtx
	dims := b.Clickable.Layout(interactionGTX, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = renderGTX.Source
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(desc).Add(gtx.Ops)
		bg, ink := theme.WithAlpha(th.R.Bg, 0), th.R.Ink
		if kind == TonalIcon {
			bg = th.R.Raised
		} else if kind == FilledIcon {
			bg, ink = th.R.Bone, th.R.OnBone
		}
		if b.Hovered() && !b.Busy {
			if kind == Standard {
				bg = theme.WithAlpha(th.R.Ink, 0.08)
			} else {
				bg = theme.Mix(bg, ink, 0.08)
			}
		}
		if b.Pressed() && !b.Busy {
			if kind == Standard {
				bg = theme.WithAlpha(th.R.Ink, 0.12)
			} else {
				bg = theme.Mix(bg, ink, 0.12)
			}
		}
		bg, ink = disabledInk(gtx, bg), disabledInk(gtx, ink)
		r := image.Rect(0, 0, sz, sz)
		FillRect(gtx.Ops, r, sz/2, bg)
		if b.Busy {
			is := gtx.Dp(20)
			off := op.Offset(image.Pt((sz-is)/2, (sz-is)/2)).Push(gtx.Ops)
			LoadingIndicator(exact(gtx, image.Pt(is, is)), th, 20)
			off.Pop()
		} else if icon != nil {
			is := gtx.Dp(20)
			off := op.Offset(image.Pt((sz-is)/2, (sz-is)/2)).Push(gtx.Ops)
			icon.Layout(exact(gtx, image.Pt(is, is)), ink)
			off.Pop()
		}
		if b.Dot.A > 0 {
			d := gtx.Dp(8)
			dot := image.Rect(sz-d-gtx.Dp(2), gtx.Dp(2), sz-gtx.Dp(2), gtx.Dp(2)+d)
			FillRect(gtx.Ops, dot, d/2, disabledInk(gtx, b.Dot))
		}
		return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(sz, sz))}
	})
	if renderGTX.Focused(&b.Clickable) {
		FocusRing(renderGTX, th, image.Rectangle{Max: dims.Size}, dims.Size.Y/2)
	}
	return dims
}
