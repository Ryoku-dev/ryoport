package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Toggle struct {
	widget.Bool
	slide anim.Tween
}

func (t *Toggle) Layout(gtx layout.Context, th *theme.Theme, label string) layout.Dimensions {
	for {
		event, ok := gtx.Event(key.Filter{Focus: &t.Bool, Name: key.NameSpace})
		if !ok {
			break
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press {
			t.Value = !t.Value
		}
	}

	original := gtx
	gtx.Constraints.Min = image.Point{}
	trackWidth, trackHeight := gtx.Dp(40), gtx.Dp(22)
	trackCut := gtx.Dp(4)
	dims := t.Bool.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Switch.Add(gtx.Ops)
		if label != "" {
			semantic.LabelOp(label).Add(gtx.Ops)
		}

		t.slide.Duration = th.Motion.Select
		t.slide.Ease = anim.EaseChisel
		if t.Value {
			t.slide.Set(gtx, 1)
		} else {
			t.slide.Set(gtx, 0)
		}
		position := t.slide.Value(gtx)
		disabled := !original.Source.Enabled()

		var children [3]layout.FlexChild
		count := 0
		children[count] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := constrainMax(gtx, image.Pt(trackWidth, trackHeight))
			track := image.Rectangle{Max: size}
			var layer paint.OpacityStack
			if disabled {
				layer = paint.PushOpacity(gtx.Ops, 0.38)
			}
			if t.Value {
				FillCut(gtx.Ops, track, trackCut, th.R.Sel)
			} else {
				FillCut(gtx.Ops, track, trackCut, th.R.Stone4)
				StrokeCut(gtx.Ops, track, trackCut, max(1, gtx.Dp(1)), th.R.Vein)
			}

			knobSize := min(gtx.Dp(14), min(size.X, size.Y))
			knobCut := min(gtx.Dp(2), knobSize/2)
			left := min(gtx.Dp(4), max(0, size.X-knobSize))
			right := max(left, size.X-gtx.Dp(4)-knobSize)
			x := int(lerp(float32(left), float32(right), position) + 0.5)
			y := (size.Y - knobSize) / 2
			knobColor := th.R.Fg2
			if t.Value {
				knobColor = th.R.OnSel
			}
			offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			FillCut(gtx.Ops, image.Rect(0, 0, knobSize, knobSize), knobCut, knobColor)
			offset.Pop()
			if disabled {
				layer.Pop()
			}
			return layout.Dimensions{Size: size}
		})
		count++
		if label != "" {
			children[count] = layout.Rigid(layout.Spacer{Width: 8}.Layout)
			count++
			ink := th.R.Fg
			if disabled {
				ink = th.R.Fg3
			}
			children[count] = layout.Rigid(rawText(th, th.T.Label, ink, label))
			count++
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children[:count]...)
	})
	if original.Focused(&t.Bool) {
		FocusRing(original, th, image.Rectangle{Max: dims.Size}, trackCut)
	}
	return dims
}
