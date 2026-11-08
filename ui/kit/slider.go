package kit

import (
	"image"
	"strconv"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Slider struct {
	widget.Float
}

func (s *Slider) Layout(gtx layout.Context, th *theme.Theme, minValue, maxValue float32) layout.Dimensions {
	if maxValue < minValue {
		minValue, maxValue = maxValue, minValue
	}
	s.Value = min(max(s.Value, 0), 1)
	w := gtx.Constraints.Max.X
	if w <= 0 {
		w = gtx.Dp(160)
	}
	h := gtx.Dp(40)
	trackH := gtx.Dp(16)
	handleW := max(1, gtx.Dp(4))
	gap := gtx.Dp(6)
	gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(image.Pt(w, h)))
	s.Update(gtx)
	center := int(float32(gtx.Constraints.Min.X-handleW)*s.Value) + handleW/2
	y := (gtx.Constraints.Min.Y - trackH) / 2
	activeEnd := max(0, center-gap)
	inactiveStart := min(gtx.Constraints.Min.X, center+gap)
	active := image.Rect(0, y, activeEnd, y+trackH)
	inactive := image.Rect(inactiveStart, y, gtx.Constraints.Min.X, y+trackH)
	FillRect(gtx.Ops, inactive, trackH/2, disabledInk(gtx, th.R.Raised))
	FillRect(gtx.Ops, active, trackH/2, disabledInk(gtx, th.R.Bone))
	handle := image.Rect(center-handleW/2, 0, center+(handleW+1)/2, gtx.Constraints.Min.Y)
	FillRect(gtx.Ops, handle, handleW/2, disabledInk(gtx, th.R.Bone))
	semantic.DescriptionOp(sliderDescription(minValue, maxValue, s.Value)).Add(gtx.Ops)
	return s.Float.Layout(gtx, layout.Horizontal, 8)
}

func sliderDescription(minValue, maxValue, value float32) string {
	return strconv.FormatFloat(float64(minValue+(maxValue-minValue)*value), 'g', -1, 32)
}
