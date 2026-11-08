package kit

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Seal struct {
	morph anim.Spring
	from  Shape
	to    Shape
	tone  model.Tone
	init  bool
}

func (s *Seal) Layout(gtx layout.Context, th *theme.Theme, size unit.Dp, tone model.Tone, content layout.Widget) layout.Dimensions {
	shape := ShapeFor(tone)
	if !s.init {
		s.from, s.to, s.tone, s.init = shape, shape, tone, true
		s.morph.Omega = 14
		s.morph.Snap(1)
	} else if tone != s.tone {
		s.from, s.to, s.tone = s.to, shape, tone
		s.morph.Snap(0)
		s.morph.Set(1)
	}
	px := gtx.Dp(size)
	gtx.Constraints.Min = image.Pt(px, px)
	gtx.Constraints.Max = image.Pt(px, px)
	progress := s.morph.Value(gtx)
	rotation := float32(0)
	if tone == model.ToneBusy {
		rotation = anim.Phase(gtx, 3*time.Second) * 2 * math.Pi
	}
	_, soft := th.StateInk(int(tone))
	if tone == model.ToneIdle {
		soft = th.R.Raised
	}
	path := ShapePath(gtx.Ops, f32.Pt(float32(px)/2, float32(px)/2), float32(px)/2, s.from, s.to, progress, rotation)
	paint.FillShape(gtx.Ops, soft, clip.Outline{Path: path}.Op())
	if content != nil {
		layout.Center.Layout(gtx, content)
	}
	return layout.Dimensions{Size: image.Pt(px, px)}
}

func SealIcon(th *theme.Theme, icon *widget.Icon, tone model.Tone, size unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if icon == nil {
			return layout.Dimensions{}
		}
		ink, _ := th.StateInk(int(tone))
		if tone == model.ToneIdle {
			ink = th.R.InkDim
		}
		px := gtx.Dp(size) * 55 / 100
		gtx.Constraints.Min = image.Pt(px, px)
		gtx.Constraints.Max = image.Pt(px, px)
		return icon.Layout(gtx, ink)
	}
}
