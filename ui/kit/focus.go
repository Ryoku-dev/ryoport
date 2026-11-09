package kit

import (
	"image"
	"time"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func transitionValue(gtx layout.Context, tween *anim.Tween, on bool, in, out time.Duration) float32 {
	tween.Ease = anim.EaseChisel
	if on {
		tween.Set(gtx, 1)
		tween.Duration = in
	} else {
		tween.Set(gtx, 0)
		tween.Duration = out
	}
	return tween.Value(gtx)
}

func hoverValue(gtx layout.Context, th *theme.Theme, tween *anim.Tween, hovered bool) float32 {
	return transitionValue(gtx, tween, hovered && gtx.Source.Enabled(), th.Motion.Hover, th.Motion.Hover*5/8)
}

func focusValue(gtx layout.Context, th *theme.Theme, tween *anim.Tween, focused bool) float32 {
	return transitionValue(gtx, tween, focused, th.Motion.Hover*3/4, th.Motion.Hover*5/8)
}

func FocusRingAlpha(gtx layout.Context, th *theme.Theme, r image.Rectangle, cut int, alpha float32) {
	if alpha <= 0 {
		return
	}
	gap := gtx.Dp(2)
	width := max(1, gtx.Dp(2))
	StrokeCut(gtx.Ops, r.Inset(-gap), cut+gap, width, theme.WithAlpha(th.R.Fg2, min(alpha, 1)))
}

func FocusRing(gtx layout.Context, th *theme.Theme, r image.Rectangle, cut int) {
	FocusRingAlpha(gtx, th, r, cut, 1)
}
