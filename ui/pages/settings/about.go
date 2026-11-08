package settings

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/version"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

var aboutNotes = releaseNoteLines(version.Notes())

func (v *View) layoutAbout(gtx layout.Context, e *env.Env) layout.Dimensions {
	notes := aboutNotes
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(21)}.Layout(gtx,
				layout.Rigid(kit.BrandMark(e.Th, 72)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Ryoport "+version.Version)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return wrapText(gtx, e.Th, e.Th.T.Body, e.Th.R.Fg2, "Reach your machines from one quiet place.")
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 4}.Layout(gtx, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "GPL-3.0-or-later"))
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(notes) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 21}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, "Release notes")),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return column(gtx, len(notes), 6, func(gtx layout.Context, index int) layout.Dimensions {
								return wrapText(gtx, e.Th, e.Th.T.Body, e.Th.R.Fg2, "• "+notes[index])
							})
						})
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 21}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return sectionAction(gtx, func(gtx layout.Context) layout.Dimensions {
					v.openRepo.Small = true
					return v.openRepo.Layout(gtx, e.Th, kit.Line, icons.OpenInNew, "Open the repository")
				})
			})
		}),
	)
}

func releaseNoteLines(notes string) []string {
	var lines []string
	for line := range strings.Lines(notes) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
	}
	return lines
}
