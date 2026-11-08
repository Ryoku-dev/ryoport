package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type ButtonKind int

const (
	Filled ButtonKind = iota
	Tonal
	Outlined
	TextButton
	Danger
)

type Button struct {
	widget.Clickable
	Small, Busy bool
	squeeze     anim.Spring
	width       int
}

func (b *Button) Clicked(gtx layout.Context) bool {
	if !b.Busy {
		return b.Clickable.Clicked(gtx)
	}
	for b.Clickable.Clicked(gtx) {
	}
	return false
}

func (b *Button) Layout(gtx layout.Context, th *theme.Theme, kind ButtonKind, icon *widget.Icon, label string) layout.Dimensions {
	height := unit.Dp(40)
	if b.Small {
		height = 32
	}
	h := gtx.Dp(height)
	interactionGTX := gtx
	if b.Busy {
		interactionGTX = interactionGTX.Disabled()
	}
	renderGTX := gtx
	radius := h / 2
	dims := b.Clickable.Layout(interactionGTX, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = renderGTX.Source
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		b.squeeze.Omega = 30
		if b.Pressed() && !b.Busy {
			b.squeeze.Set(float32(gtx.Dp(th.Shape.M)))
		} else {
			b.squeeze.Set(float32(h / 2))
		}
		radius = int(b.squeeze.Value(gtx) + 0.5)

		bg, ink := th.R.Bone, th.R.OnBone
		switch kind {
		case Tonal:
			bg, ink = th.R.Raised, th.R.Ink
		case Outlined:
			bg, ink = theme.WithAlpha(th.R.Bg, 0), th.R.Ink
		case TextButton:
			bg, ink = theme.WithAlpha(th.R.Bg, 0), th.R.Ink
		case Danger:
			ink = th.R.Bg
			if th.R.Dark {
				ink = th.R.Bone
			}
			bg = th.R.Bad
		}
		if b.Hovered() && !b.Busy {
			if kind == TextButton || kind == Outlined {
				bg = theme.WithAlpha(th.R.Ink, 0.08)
			} else {
				bg = theme.Mix(bg, ink, 0.08)
			}
		}
		if b.Pressed() && !b.Busy {
			if kind == TextButton || kind == Outlined {
				bg = theme.WithAlpha(th.R.Ink, 0.12)
			} else {
				bg = theme.Mix(bg, ink, 0.12)
			}
		}
		bg, ink = disabledInk(gtx, bg), disabledInk(gtx, ink)

		m := op.Record(gtx.Ops)
		content := layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			labelWidget := Text(th, th.T.Label, ink, label)
			if b.Busy {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return LoadingIndicator(gtx, th, 18) }),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(labelWidget),
				)
			}
			if icon != nil {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(iconWidget(icon, 18, ink)),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(labelWidget),
				)
			}
			return labelWidget(gtx)
		})
		call := m.Stop()
		desiredWidth := content.Size.X
		if b.Busy && b.width > 0 {
			desiredWidth = b.width
		}
		size := gtx.Constraints.Constrain(image.Pt(desiredWidth, h))
		if !b.Busy {
			b.width = size.X
		}
		r := image.Rectangle{Max: size}
		FillRect(gtx.Ops, r, radius, bg)
		if kind == Outlined {
			StrokeRect(gtx.Ops, r.Inset(gtx.Dp(1)), max(0, radius-gtx.Dp(1)), max(1, gtx.Dp(1)), disabledInk(gtx, th.R.LineStrong))
		}
		off := op.Offset(image.Pt((size.X-content.Size.X)/2, (size.Y-content.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: size, Baseline: content.Baseline + (size.Y-content.Size.Y)/2}
	})
	if renderGTX.Focused(&b.Clickable) {
		FocusRing(renderGTX, th, image.Rectangle{Max: dims.Size}, radius)
	}
	return dims
}
