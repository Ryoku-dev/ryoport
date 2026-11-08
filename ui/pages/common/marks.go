package common

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

// ResourceMarks returns the small machine traits shown in rows and headers.
// attention is the Disposable word, which callers colour amber.
func ResourceMarks(resource model.Resource) (spans []Span, attention int) {
	attention = -1
	appendMark := func(word string, warn bool) {
		if len(spans) > 0 {
			spans = append(spans, Span{Text: " · ", Mono: true})
		}
		if warn {
			attention = len(spans)
		}
		spans = append(spans, Span{Text: word, Mono: true})
	}
	// Disposable leads: it is the one that deletes the machine, so it must
	// survive when a narrow row drops the trailing marks.
	if resource.Disposable {
		appendMark("Disposable", true)
	}
	if resource.Instant {
		appendMark("Instant", false)
	}
	if resource.Sealed {
		appendMark("Sealed", false)
	}
	return spans, attention
}

type factRun struct {
	text  string
	style theme.TextStyle
	ink   color.NRGBA
}

// ResourceFactsWidget composes facts and machine traits as one ordered line.
// Whole trailing traits are dropped before the facts are ellipsised.
func ResourceFactsWidget(th *theme.Theme, facts string, factsMono bool, resource model.Resource, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		factStyle := th.T.Caption
		if factsMono {
			factStyle = th.T.Mono
		}
		runs := make([]factRun, 0, 4)
		if facts != "" {
			runs = append(runs, factRun{text: facts, style: factStyle, ink: ink})
		}
		if resource.Instant {
			runs = append(runs, factRun{text: " · Instant", style: th.T.MonoSm, ink: th.R.Fg3})
		}
		if resource.Disposable {
			runs = append(runs, factRun{text: " · Disposable", style: th.T.MonoSm, ink: th.R.Amber})
		}
		if resource.Sealed {
			runs = append(runs, factRun{text: " · Sealed", style: th.T.MonoSm, ink: th.R.Fg3})
		}
		return layoutFactRuns(gtx, th, runs)
	}
}

func layoutFactRuns(gtx layout.Context, th *theme.Theme, runs []factRun) layout.Dimensions {
	if len(runs) == 0 {
		return layout.Dimensions{}
	}
	maxWidth := gtx.Constraints.Max.X
	widths := make([]int, len(runs))
	total := 0
	for i := range runs {
		widths[i] = measureFactRun(gtx, th, runs[i])
		total += widths[i]
	}
	if total > maxWidth {
		ellipsis := factRun{text: "…", style: runs[len(runs)-1].style, ink: th.R.Fg3}
		ellipsisWidth := measureFactRun(gtx, th, ellipsis)
		for len(runs) > 1 && total+ellipsisWidth > maxWidth {
			last := len(runs) - 1
			total -= widths[last]
			runs = runs[:last]
			widths = widths[:last]
		}
		if total+ellipsisWidth > maxWidth {
			last := len(runs) - 1
			available := max(0, maxWidth-(total-widths[last])-ellipsisWidth)
			runs[last].text = fitFactRun(gtx, th, runs[last], available)
		}
		runs = append(runs, ellipsis)
	}

	children := make([]layout.FlexChild, 0, len(runs))
	for _, run := range runs {
		run := run
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: run.style, Color: run.ink, MaxLines: 1}.Layout(gtx, th, run.text)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

func fitFactRun(gtx layout.Context, th *theme.Theme, run factRun, width int) string {
	runes := []rune(run.text)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		candidate := run
		candidate.text = string(runes[:mid])
		if measureFactRun(gtx, th, candidate) <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return string(runes[:low])
}

func measureFactRun(gtx layout.Context, th *theme.Theme, run factRun) int {
	measure := gtx
	measure.Constraints.Min = image.Point{}
	measure.Constraints.Max.X = 1 << 20
	macro := op.Record(gtx.Ops)
	dims := kit.Label{Style: run.style, Color: run.ink, MaxLines: 1}.Layout(measure, th, run.text)
	macro.Stop()
	return dims.Size.X
}
