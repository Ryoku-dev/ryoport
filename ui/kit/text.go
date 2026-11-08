package kit

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Label struct {
	Style    theme.TextStyle
	Color    color.NRGBA
	MaxLines int
	Align    text.Alignment
}

func (l Label) Layout(gtx layout.Context, th *theme.Theme, s string) layout.Dimensions {
	truncator := ""
	if l.MaxLines > 0 {
		truncator = "…"
	}
	label := widget.Label{
		Alignment:  l.Align,
		MaxLines:   l.MaxLines,
		Truncator:  truncator,
		LineHeight: l.Style.Line,
	}
	return label.Layout(gtx, th.Shaper, l.Style.Face, l.Style.Size, s, textMaterial(gtx.Ops, disabledInk(gtx, l.Color)))
}

func Text(th *theme.Theme, style theme.TextStyle, c color.NRGBA, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return Label{Style: style, Color: c, MaxLines: 1}.Layout(gtx, th, s)
	}
}
