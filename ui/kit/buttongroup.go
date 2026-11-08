package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type ButtonGroup struct {
	buttons []widget.Clickable
	morphs  []anim.Spring
	changed int
	pending bool
}

func (g *ButtonGroup) ensure(n int) {
	for len(g.buttons) < n {
		g.buttons = append(g.buttons, widget.Clickable{})
		g.morphs = append(g.morphs, anim.Spring{Omega: 22})
	}
	g.buttons = g.buttons[:n]
	g.morphs = g.morphs[:n]
}

func (g *ButtonGroup) poll(gtx layout.Context) {
	for i := range g.buttons {
		if g.buttons[i].Clicked(gtx) {
			g.changed, g.pending = i, true
		}
	}
}

func (g *ButtonGroup) Changed(gtx layout.Context) (int, bool) {
	g.poll(gtx)
	if !g.pending {
		return 0, false
	}
	i := g.changed
	g.pending = false
	return i, true
}

func (g *ButtonGroup) Layout(gtx layout.Context, th *theme.Theme, labels []string, selected int) layout.Dimensions {
	g.ensure(len(labels))
	g.poll(gtx)
	h := gtx.Dp(36)
	gap := gtx.Dp(2)
	outer := h / 2
	inner := gtx.Dp(th.Shape.XS)
	children := make([]layout.FlexChild, len(labels))
	for i := range labels {
		i := i
		target := float32(0)
		if i == selected {
			target = 1
		}
		g.morphs[i].Set(target)
		p := g.morphs[i].Value(gtx)
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := g.buttons[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.RadioButton.Add(gtx.Ops)
				semantic.LabelOp(labels[i]).Add(gtx.Ops)
				semantic.SelectedOp(i == selected).Add(gtx.Ops)
				bg, ink := th.R.Raised, th.R.Ink
				if i == selected {
					bg, ink = th.R.Bone, th.R.OnBone
				}
				if g.buttons[i].Hovered() {
					bg = theme.Mix(bg, ink, 0.08)
				}
				if g.buttons[i].Pressed() {
					bg = theme.Mix(bg, ink, 0.12)
				}
				m := op.Record(gtx.Ops)
				content := layout.Inset{Left: 14, Right: 14}.Layout(gtx, Text(th, th.T.Label, disabledInk(gtx, ink), labels[i]))
				call := m.Stop()
				size := gtx.Constraints.Constrain(image.Pt(content.Size.X, h))
				r := image.Rectangle{Max: size}
				baseNW, baseNE, baseSE, baseSW := inner, inner, inner, inner
				if i == 0 {
					baseNW, baseSW = outer, outer
				}
				if i == len(labels)-1 {
					baseNE, baseSE = outer, outer
				}
				corner := func(base int) int { return int(lerp(float32(base), float32(outer), p) + 0.5) }
				fillCorners(gtx.Ops, r, corner(baseNW), corner(baseNE), corner(baseSE), corner(baseSW), disabledInk(gtx, bg))
				off := op.Offset(image.Pt((size.X-content.Size.X)/2, (h-content.Size.Y)/2)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				off.Pop()
				return layout.Dimensions{Size: size}
			})
			if gtx.Focused(&g.buttons[i]) {
				FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, outer)
			}
			return dims
		})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(gtx, children...)
}
