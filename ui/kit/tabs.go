package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Tabs struct {
	tabs     []widget.Clickable
	x, width anim.Spring
	changed  int
	pending  bool
}

func (t *Tabs) ensure(n int) {
	for len(t.tabs) < n {
		t.tabs = append(t.tabs, widget.Clickable{})
	}
	t.tabs = t.tabs[:n]
}

func (t *Tabs) poll(gtx layout.Context) {
	for i := range t.tabs {
		if t.tabs[i].Clicked(gtx) {
			t.changed, t.pending = i, true
		}
		for {
			e, ok := gtx.Event(
				key.Filter{Focus: &t.tabs[i], Name: key.NameLeftArrow},
				key.Filter{Focus: &t.tabs[i], Name: key.NameRightArrow},
			)
			if !ok {
				break
			}
			ke, ok := e.(key.Event)
			if !ok || ke.State != key.Press || len(t.tabs) == 0 {
				continue
			}
			next := i
			if ke.Name == key.NameLeftArrow {
				next = max(0, i-1)
			} else {
				next = min(len(t.tabs)-1, i+1)
			}
			if next != i {
				t.changed, t.pending = next, true
				gtx.Execute(key.FocusCmd{Tag: &t.tabs[next]})
			}
		}
	}
}

func (t *Tabs) Changed(gtx layout.Context) (int, bool) {
	t.poll(gtx)
	if !t.pending {
		return 0, false
	}
	i := t.changed
	t.pending = false
	return i, true
}

func (t *Tabs) Layout(gtx layout.Context, th *theme.Theme, labels []string, selected int) layout.Dimensions {
	t.ensure(len(labels))
	t.poll(gtx)
	h := gtx.Dp(36)
	widths := make([]int, len(labels))
	total := 0
	for i, label := range labels {
		m := op.Record(gtx.Ops)
		d := Label{Style: th.T.Label, Color: th.R.Ink, MaxLines: 1}.Layout(gtx, th, label)
		m.Stop()
		widths[i] = d.Size.X + gtx.Dp(24)
		total += widths[i]
	}
	if len(labels) == 0 {
		return layout.Dimensions{}
	}
	selected = min(max(selected, 0), len(labels)-1)
	targetX := 0
	for i := range selected {
		targetX += widths[i]
	}
	t.x.Omega, t.width.Omega = 22, 22
	t.x.Set(float32(targetX))
	t.width.Set(float32(widths[selected]))
	indicatorX := int(t.x.Value(gtx) + 0.5)
	indicatorW := int(t.width.Value(gtx) + 0.5)
	FillRect(gtx.Ops, image.Rect(indicatorX, 0, indicatorX+indicatorW, h), h/2, disabledInk(gtx, th.R.Selected))

	x := 0
	for i, label := range labels {
		i, label := i, label
		w := widths[i]
		off := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		tabGTX := exact(gtx, image.Pt(w, h))
		dims := t.tabs[i].Layout(tabGTX, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.SelectedOp(i == selected).Add(gtx.Ops)
			if t.tabs[i].Hovered() && i != selected {
				FillRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, disabledInk(gtx, theme.WithAlpha(th.R.Ink, 0.08)))
			}
			if t.tabs[i].Pressed() {
				FillRect(gtx.Ops, image.Rect(0, 0, w, h), h/2, disabledInk(gtx, theme.WithAlpha(th.R.Ink, 0.12)))
			}
			return layout.Center.Layout(gtx, Text(th, th.T.Label, disabledInk(gtx, th.R.Ink), label))
		})
		if gtx.Focused(&t.tabs[i]) {
			FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, h/2)
		}
		off.Pop()
		x += w
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(total, h))}
}
