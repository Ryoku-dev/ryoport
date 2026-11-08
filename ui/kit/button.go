package kit

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type ButtonKind int

const (
	Gold ButtonKind = iota
	Line
	Quiet
	On
	Danger
	DangerLine
)

type buttonColors struct {
	fill, ink, outline color.NRGBA
	stroke             bool
}

func colorsFor(th *theme.Theme, kind ButtonKind) buttonColors {
	clear := color.NRGBA{}
	switch kind {
	case Line:
		return buttonColors{fill: clear, ink: th.R.GoldInk, outline: th.R.GoldInk, stroke: true}
	case Quiet:
		return buttonColors{fill: clear, ink: th.R.Fg2, outline: th.R.Vein, stroke: true}
	case On:
		return buttonColors{fill: th.R.Sel, ink: th.R.OnSel}
	case Danger:
		return buttonColors{fill: th.R.Jasper, ink: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}
	case DangerLine:
		return buttonColors{fill: clear, ink: th.R.Jasper, outline: th.R.Jasper, stroke: true}
	default:
		return buttonColors{fill: th.R.Gold, ink: th.R.OnGold}
	}
}

func normalInteractiveColors(th *theme.Theme, kind ButtonKind, hovered bool) buttonColors {
	p := colorsFor(th, kind)
	if hovered {
		if p.fill.A == 0 {
			p.fill = theme.WithAlpha(p.ink, 0.10)
		} else {
			p.fill = theme.Mix(p.fill, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, 0.10)
		}
	}
	return p
}

func interactiveColors(gtx layout.Context, th *theme.Theme, kind ButtonKind, hovered bool) buttonColors {
	p := normalInteractiveColors(th, kind, hovered && gtx.Source.Enabled())
	p.fill = disabledInk(gtx, p.fill)
	p.ink = disabledInk(gtx, p.ink)
	p.outline = disabledInk(gtx, p.outline)
	return p
}

func drawButtonSurface(gtx layout.Context, th *theme.Theme, r image.Rectangle, cut int, kind ButtonKind, hovered bool) color.NRGBA {
	p := interactiveColors(gtx, th, kind, hovered)
	FillCut(gtx.Ops, r, cut, p.fill)
	if p.stroke {
		StrokeCut(gtx.Ops, r, cut, max(1, gtx.Dp(1)), p.outline)
	}
	return p.ink
}

type Button struct {
	Click
	Small, Busy bool
	press       anim.Tween
	width       int
}

func (b *Button) Clicked(gtx layout.Context) bool {
	if !b.Busy {
		return b.Click.Clicked(gtx)
	}
	for b.Click.Clicked(gtx) {
	}
	return false
}

func (b *Button) Layout(gtx layout.Context, th *theme.Theme, kind ButtonKind, icon *widget.Icon, label string) layout.Dimensions {
	height, side, iconSize, gap, baseCut := unit.Dp(40), unit.Dp(18), unit.Dp(19), unit.Dp(8), th.Cut.Mid
	if b.Small {
		height, side, iconSize, gap, baseCut = 32, 13, 17, 6, th.Cut.Small
	}
	h := gtx.Dp(height)
	original := gtx
	interaction := gtx
	if b.Busy {
		interaction = interaction.Disabled()
	}

	var finalCut int
	dims := b.Click.Layout(interaction, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = original.Source
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)

		b.press.Duration = th.Motion.Press
		b.press.Ease = anim.EaseChisel
		if b.Pressed() && !b.Busy && original.Source.Enabled() {
			b.press.Set(gtx, 1)
		} else {
			b.press.Set(gtx, 0)
		}
		pressed := b.press.Value(gtx)
		finalCut = gtx.Dp(baseCut) + int(float32(gtx.Dp(4))*pressed+0.5)
		sink := int(float32(gtx.Dp(1))*pressed + 0.5)

		palette := normalInteractiveColors(th, kind, b.Hovered() && !b.Busy && original.Source.Enabled())
		contentGTX := gtx
		contentGTX.Constraints.Min = image.Point{}
		contentGTX.Constraints.Max.Y = h
		m := op.Record(gtx.Ops)
		content := layout.Inset{Left: side, Right: side}.Layout(contentGTX, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 3)
			if b.Busy {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Working(gtx, th, iconSize) }),
					layout.Rigid(layout.Spacer{Width: gap}.Layout),
				)
			} else if icon != nil {
				children = append(children,
					layout.Rigid(iconWidget(icon, iconSize, palette.ink)),
					layout.Rigid(layout.Spacer{Width: gap}.Layout),
				)
			}
			children = append(children, layout.Rigid(rawText(th, th.T.Label, palette.ink, label)))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		call := m.Stop()

		desiredWidth := content.Size.X
		if b.Busy && b.width > 0 {
			desiredWidth = b.width
		}
		size := constrainMax(gtx, image.Pt(desiredWidth, h))
		if !b.Busy {
			b.width = size.X
		}
		r := image.Rectangle{Max: size}
		disabled := !original.Source.Enabled()
		var layer paint.OpacityStack
		if disabled {
			layer = paint.PushOpacity(gtx.Ops, 0.38)
		}
		FillCut(gtx.Ops, r, finalCut, palette.fill)
		if palette.stroke {
			StrokeCut(gtx.Ops, r, finalCut, max(1, gtx.Dp(1)), palette.outline)
		}
		offset := op.Offset(image.Pt((size.X-content.Size.X)/2, (size.Y-content.Size.Y)/2+sink)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		if disabled {
			layer.Pop()
		}
		return layout.Dimensions{Size: size, Baseline: content.Baseline + (size.Y-content.Size.Y)/2 + sink}
	})
	if original.Focused(&b.Click.Clickable) {
		FocusRing(original, th, image.Rectangle{Max: dims.Size}, finalCut)
	}
	return dims
}
