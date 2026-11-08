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

	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func LoadingIndicator(gtx layout.Context, th *theme.Theme, size unit.Dp) layout.Dimensions {
	px := gtx.Dp(size)
	sz := gtx.Constraints.Constrain(image.Pt(px, px))
	shapes := [...]Shape{Cookie, Burst, Clover, Circle}
	phase := anim.Phase(gtx, 1600*time.Millisecond)
	step := phase * float32(len(shapes))
	i := int(step) % len(shapes)
	t := anim.EaseInOut(step - float32(int(step)))
	path := ShapePath(gtx.Ops, f32.Pt(float32(sz.X)/2, float32(sz.Y)/2), float32(min(sz.X, sz.Y))/2, shapes[i], shapes[(i+1)%len(shapes)], t, phase*2*math.Pi)
	paint.FillShape(gtx.Ops, th.R.Accent, clip.Outline{Path: path}.Op())
	return layout.Dimensions{Size: sz}
}
