package common

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

// Draft column widths, in dp. The form keeps a reading width; the aside is
// wide enough for a config stanza or a command line without wrapping much.
const (
	draftFormMax    = 600
	draftAsideMin   = 300
	draftAsideMax   = 400
	draftWideAt     = 820
	draftAsideBelow = 600
)

// DraftColumns lays a draft's form beside an aside that previews what saving
// it will do, so a wide work pane holds the whole decision instead of a
// narrow column and empty stone. From 820 dp, about what a 1440 dp window
// leaves the work pane, the aside sits on the right of the form, both
// aligned with the header; narrower, it follows the form. A
// nil aside leaves the form alone at its reading width.
func DraftColumns(gtx layout.Context, th *theme.Theme, form, aside layout.Widget) layout.Dimensions {
	available := gtx.Constraints.Max.X
	column := func(width int, w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			return w(gtx)
		}
	}
	if aside == nil {
		return column(min(available, gtx.Dp(unit.Dp(draftFormMax))), form)(gtx)
	}
	if available < gtx.Dp(unit.Dp(draftWideAt)) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(column(min(available, gtx.Dp(unit.Dp(draftFormMax))), form)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(column(min(available, gtx.Dp(unit.Dp(draftAsideBelow))), aside)),
		)
	}
	gap := gtx.Dp(th.Sp.S5)
	asideWidth := available * 36 / 100
	asideWidth = max(gtx.Dp(unit.Dp(draftAsideMin)), min(asideWidth, gtx.Dp(unit.Dp(draftAsideMax))))
	formWidth := min(gtx.Dp(unit.Dp(draftFormMax)), available-gap-asideWidth)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(column(formWidth, form)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S5}.Layout),
		layout.Rigid(column(asideWidth, aside)),
	)
}

// Aside is the quiet slab beside a draft: a title, then the facts and
// previews the draft fills in as the form changes.
func Aside(gtx layout.Context, th *theme.Theme, title string, rows ...layout.Widget) layout.Dimensions {
	return kit.Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2*len(rows)+1)
			children = append(children, layout.Rigid(kit.Text(th, th.T.Title, th.R.Fg, title)))
			for _, row := range rows {
				if row == nil {
					continue
				}
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(row),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

// AsideFact is one line of an aside: what it is on the left, its value on
// the right in the data face.
func AsideFact(th *theme.Theme, label, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Baseline, Gap: gtx.Dp(th.Sp.S3)}.Layout(gtx,
			layout.Rigid(kit.Text(th, th.T.Body, th.R.Fg2, label)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Mono, Color: th.R.Fg, MaxLines: 2}.Layout(gtx, th, value)
				})
			}),
		)
	}
}

// AsideNote is a sentence of explanation in an aside.
func AsideNote(th *theme.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return kit.Label{Style: th.T.Caption, Color: th.R.Fg2, MaxLines: 4}.Layout(gtx, th, text)
	}
}

// AsidePreview shows literal text the draft will write or run, such as an
// ssh config stanza or a command line, line for line in the data face on a
// recessed slab.
func AsidePreview(th *theme.Theme, text string) layout.Widget {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, len(lines))
				for i, line := range lines {
					children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if line == "" {
							return layout.Dimensions{Size: image.Pt(0, gtx.Sp(th.T.MonoSm.Line))}
						}
						return kit.Label{Style: th.T.MonoSm, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th, line)
					})
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	}
}
