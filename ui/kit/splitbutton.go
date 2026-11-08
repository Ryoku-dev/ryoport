package kit

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type SplitButton struct {
	Main, More widget.Clickable
	turn       anim.Spring
}

func (s *SplitButton) Layout(gtx layout.Context, th *theme.Theme, icon *widget.Icon, label string, open bool) layout.Dimensions {
	s.turn.Omega = 22
	if open {
		s.turn.Set(1)
	} else {
		s.turn.Set(0)
	}
	turn := s.turn.Value(gtx)
	h := gtx.Dp(40)
	gap := gtx.Dp(2)
	outer := h / 2
	inner := gtx.Dp(th.Shape.XS)
	moreW := gtx.Dp(48)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := s.Main.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(label).Add(gtx.Ops)
				m := op.Record(gtx.Ops)
				content := layout.Inset{Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, 3)
					if icon != nil {
						children = append(children, layout.Rigid(iconWidget(icon, 18, disabledInk(gtx, th.R.OnBone))), layout.Rigid(layout.Spacer{Width: 8}.Layout))
					}
					children = append(children, layout.Rigid(Text(th, th.T.Label, disabledInk(gtx, th.R.OnBone), label)))
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})
				call := m.Stop()
				size := gtx.Constraints.Constrain(image.Pt(content.Size.X, h))
				r := image.Rectangle{Max: size}
				bg := th.R.Bone
				if s.Main.Hovered() {
					bg = theme.Mix(bg, th.R.OnBone, 0.08)
				}
				if s.Main.Pressed() {
					bg = theme.Mix(bg, th.R.OnBone, 0.12)
				}
				fillCorners(gtx.Ops, r, outer, inner, inner, outer, disabledInk(gtx, bg))
				off := op.Offset(image.Pt((size.X-content.Size.X)/2, (h-content.Size.Y)/2)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				off.Pop()
				return layout.Dimensions{Size: size}
			})
			if gtx.Focused(&s.Main) {
				FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, outer)
			}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = moreW, moreW
			dims := s.More.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.DescriptionOp("More actions").Add(gtx.Ops)
				r := image.Rect(0, 0, moreW, h)
				bg := th.R.Bone
				if s.More.Hovered() {
					bg = theme.Mix(bg, th.R.OnBone, 0.08)
				}
				if s.More.Pressed() {
					bg = theme.Mix(bg, th.R.OnBone, 0.12)
				}
				fillCorners(gtx.Ops, r, inner, outer, outer, inner, disabledInk(gtx, bg))
				is := gtx.Dp(20)
				center := f32.Pt(float32(moreW)/2, float32(h)/2)
				rot := op.Affine(f32.AffineId().Rotate(center, float32(math.Pi)*turn)).Push(gtx.Ops)
				off := op.Offset(image.Pt((moreW-is)/2, (h-is)/2)).Push(gtx.Ops)
				icons.ExpandMore.Layout(exact(gtx, image.Pt(is, is)), disabledInk(gtx, th.R.OnBone))
				off.Pop()
				rot.Pop()
				return layout.Dimensions{Size: image.Pt(moreW, h)}
			})
			if gtx.Focused(&s.More) {
				FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, outer)
			}
			return dims
		}),
	)
}
