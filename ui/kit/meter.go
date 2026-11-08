package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func Meter(th *theme.Theme, frac float64, c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w, h := gtx.Constraints.Max.X, gtx.Dp(8)
		size := gtx.Constraints.Constrain(image.Pt(w, h))
		FillRect(gtx.Ops, image.Rectangle{Max: size}, size.Y/2, th.R.Raised)
		filled := int(float64(size.X) * max(0, min(1, frac)))
		if filled > 0 {
			FillRect(gtx.Ops, image.Rect(0, 0, filled, size.Y), size.Y/2, c)
		}
		return layout.Dimensions{Size: size}
	}
}

func MeterInk(th *theme.Theme, frac float64) color.NRGBA {
	if frac < .7 {
		return th.R.Ok
	}
	if frac < .9 {
		return th.R.Warn
	}
	return th.R.Bad
}
