package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Field struct {
	tone  model.Tone
	from  color.NRGBA
	to    color.NRGBA
	blend anim.Tween
	ready bool
}

func (f *Field) Layout(gtx layout.Context, th *theme.Theme, tone model.Tone, w layout.Widget) layout.Dimensions {
	target := th.R.Field(tone)
	f.blend.Duration = th.Motion.Field
	f.blend.Ease = anim.EaseChisel
	if !f.ready {
		f.tone = tone
		f.from, f.to = target, target
		f.ready = true
		f.blend.Set(gtx, 1)
	} else if f.tone != tone {
		f.from = theme.Mix(f.from, f.to, f.blend.Value(gtx))
		f.to = target
		f.tone = tone
		f.blend = anim.Tween{Duration: th.Motion.Field, Ease: anim.EaseChisel}
		f.blend.Set(gtx, 0)
		f.blend.Set(gtx, 1)
	} else if target != f.to {
		// The palette changed under a steady state: follow it at once rather
		// than keep painting the old scheme's stone.
		f.from, f.to = target, target
	}
	ink := theme.Mix(f.from, f.to, f.blend.Value(gtx))
	child := op.Record(gtx.Ops)
	if gtx.Constraints.Max.X > 0 {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	}
	var dims layout.Dimensions
	if w != nil {
		dims = w(gtx)
	} else {
		dims.Size = gtx.Constraints.Min
	}
	call := child.Stop()
	FillCut(gtx.Ops, image.Rectangle{Max: dims.Size}, 0, ink)
	call.Add(gtx.Ops)
	return dims
}
