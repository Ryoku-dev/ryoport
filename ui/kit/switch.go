package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Switch struct {
	widget.Bool
	slide anim.Spring
}

func (s *Switch) Layout(gtx layout.Context, th *theme.Theme, desc string) layout.Dimensions {
	w, h := gtx.Dp(52), gtx.Dp(32)
	dims := s.Bool.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		s.slide.Omega = 24
		if s.Value {
			s.slide.Set(1)
		} else {
			s.slide.Set(0)
		}
		p := s.slide.Value(gtx)
		semantic.Switch.Add(gtx.Ops)
		semantic.DescriptionOp(desc).Add(gtx.Ops)
		r := image.Rect(0, 0, w, h)
		track, thumb := th.R.Raised, th.R.InkDim
		if s.Value {
			track, thumb = th.R.Bone, th.R.OnBone
		}
		if s.Hovered() {
			track = theme.Mix(track, thumb, 0.08)
		}
		if s.Pressed() {
			track = theme.Mix(track, thumb, 0.12)
		}
		track, thumb = disabledInk(gtx, track), disabledInk(gtx, thumb)
		FillRect(gtx.Ops, r, h/2, track)
		if !s.Value {
			StrokeRect(gtx.Ops, r.Inset(gtx.Dp(1)), h/2-gtx.Dp(1), max(1, gtx.Dp(2)), disabledInk(gtx, th.R.LineStrong))
		}
		thumbSize := int(lerp(float32(gtx.Dp(16)), float32(gtx.Dp(24)), p) + 0.5)
		if s.Pressed() {
			thumbSize += gtx.Dp(2)
		}
		cx := int(lerp(float32(gtx.Dp(10)), float32(w-gtx.Dp(14)), p) + 0.5)
		thumbRect := image.Rect(cx-thumbSize/2, (h-thumbSize)/2, cx+(thumbSize+1)/2, (h+thumbSize+1)/2)
		FillRect(gtx.Ops, thumbRect, thumbSize/2, thumb)
		if p > 0.55 {
			is := min(gtx.Dp(16), thumbSize-gtx.Dp(4))
			if is > 0 {
				off := op.Offset(image.Pt(cx-is/2, (h-is)/2)).Push(gtx.Ops)
				icons.Check.Layout(exact(gtx, image.Pt(is, is)), disabledInk(gtx, th.R.Bone))
				off.Pop()
			}
		}
		return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(w, h))}
	})
	if gtx.Focused(&s.Bool) {
		FocusRing(gtx, th, image.Rectangle{Max: dims.Size}, h/2)
	}
	return dims
}
