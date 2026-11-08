package kit

import (
	"image"
	"strconv"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type filterButton struct {
	Click
	press anim.Tween
}

type Filters struct {
	buttons []filterButton
	changed int
	pending bool
}

func (f *Filters) ensure(n int) {
	for len(f.buttons) < n {
		f.buttons = append(f.buttons, filterButton{})
	}
	f.buttons = f.buttons[:n]
}

func (f *Filters) poll(gtx layout.Context) {
	for i := range f.buttons {
		if f.buttons[i].Clicked(gtx) {
			f.changed, f.pending = i, true
		}
	}
}

func (f *Filters) Changed(gtx layout.Context) (int, bool) {
	f.poll(gtx)
	if !f.pending {
		return 0, false
	}
	changed := f.changed
	f.pending = false
	return changed, true
}

func (f *Filters) Layout(gtx layout.Context, th *theme.Theme, labels []string, counts []int, selected int) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	f.ensure(len(labels))
	f.poll(gtx)
	children := make([]layout.FlexChild, 0, len(labels))
	for i, label := range labels {
		i, label := i, label
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			kind := Quiet
			if i == selected {
				kind = On
			}
			var cut int
			dims := f.buttons[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(label).Add(gtx.Ops)
				semantic.SelectedOp(i == selected).Add(gtx.Ops)
				f.buttons[i].press.Duration = th.Motion.Press
				f.buttons[i].press.Ease = anim.EaseChisel
				if f.buttons[i].Pressed() && gtx.Source.Enabled() {
					f.buttons[i].press.Set(gtx, 1)
				} else {
					f.buttons[i].press.Set(gtx, 0)
				}
				pressed := f.buttons[i].press.Value(gtx)
				cut = gtx.Dp(th.Cut.Small) + int(float32(gtx.Dp(4))*pressed+0.5)
				sink := int(float32(gtx.Dp(1))*pressed + 0.5)
				palette := interactiveColors(gtx, th, kind, f.buttons[i].Hovered())

				contentGTX := gtx
				contentGTX.Constraints.Min = image.Point{}
				contentGTX.Constraints.Max.Y = gtx.Dp(32)
				m := op.Record(gtx.Ops)
				content := layout.Inset{Left: 13, Right: 13}.Layout(contentGTX, func(gtx layout.Context) layout.Dimensions {
					parts := []layout.FlexChild{layout.Rigid(rawText(th, th.T.Label, palette.ink, label))}
					if counts != nil && i < len(counts) {
						parts = append(parts,
							layout.Rigid(layout.Spacer{Width: 6}.Layout),
							layout.Rigid(rawText(th, th.T.MonoSm, palette.ink, strconv.Itoa(counts[i]))),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, parts...)
				})
				call := m.Stop()
				size := constrainMax(gtx, image.Pt(content.Size.X, gtx.Dp(32)))
				r := image.Rectangle{Max: size}
				FillCut(gtx.Ops, r, cut, palette.fill)
				if palette.stroke {
					StrokeCut(gtx.Ops, r, cut, max(1, gtx.Dp(1)), palette.outline)
				}
				offset := op.Offset(image.Pt((size.X-content.Size.X)/2, (size.Y-content.Size.Y)/2+sink)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				offset.Pop()
				return layout.Dimensions{Size: size}
			})
			if gtx.Focused(&f.buttons[i].Click.Clickable) {
				FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, cut)
			}
			return dims
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(4)}.Layout(gtx, children...)
}
