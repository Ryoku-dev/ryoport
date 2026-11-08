package common

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/x/richtext"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

// Sentence lays out a wrapping sentence while preserving mono data runs.
func Sentence(gtx layout.Context, th *theme.Theme, style theme.TextStyle, c color.NRGBA, spans []Span) layout.Dimensions {
	if len(spans) == 0 {
		return layout.Dimensions{}
	}
	styles := make([]richtext.SpanStyle, len(spans))
	for i, span := range spans {
		face := style.Face
		if span.Mono {
			face = th.T.Mono.Face
		}
		ink := c
		if span.Color.A != 0 {
			ink = span.Color
		}
		styles[i] = richtext.SpanStyle{
			Font:    face,
			Size:    style.Size,
			Color:   ink,
			Content: span.Text,
		}
	}
	var state richtext.InteractiveText
	text := richtext.Text(&state, th.Shaper, styles...)
	text.LineHeight = style.Line
	return text.Layout(gtx)
}
