package activity

import (
	"image"
	"math"
	"sort"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

const timelineWindow = time.Hour

type eventGroup struct {
	Label  string
	Events []model.Event
}

type timelineSpan struct {
	Job   model.Job
	Start float64
	End   float64
}

func groupEvents(events []model.Event, now time.Time) []eventGroup {
	if len(events) == 0 {
		return nil
	}
	ordered := append([]model.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At.After(ordered[j].At) })
	groups := make([]eventGroup, 0, 8)
	for _, event := range ordered {
		label := dayLabel(event.At, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, eventGroup{Label: label})
		}
		group := &groups[len(groups)-1]
		group.Events = append(group.Events, event)
	}
	return groups
}

func dayLabel(at, now time.Time) string {
	location := now.Location()
	at = at.In(location)
	now = now.In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	days := int(today.Sub(day) / (24 * time.Hour))
	switch {
	case days == 0:
		return "Today"
	case days == 1:
		return "Yesterday"
	case days > 1 && days < 7:
		return day.Weekday().String()
	default:
		return day.Format("2 January 2006")
	}
}

func jobTimeline(jobs []model.Job, now time.Time) []timelineSpan {
	windowStart := now.Add(-timelineWindow)
	spans := make([]timelineSpan, 0, len(jobs))
	for _, job := range jobs {
		start := job.Started
		if start.IsZero() {
			continue
		}
		end := job.Finished
		if end.IsZero() {
			end = now
		}
		if end.Before(start) {
			end = start
		}
		if end.Before(windowStart) || start.After(now) {
			continue
		}
		startFraction := float64(now.Sub(start)) / float64(timelineWindow)
		endFraction := float64(now.Sub(end)) / float64(timelineWindow)
		startX := clamp01(1 - startFraction)
		endX := clamp01(1 - endFraction)
		if endX < startX {
			startX, endX = endX, startX
		}
		spans = append(spans, timelineSpan{Job: job, Start: startX, End: endX})
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].Start == spans[j].Start {
			return spans[i].Job.ID < spans[j].Job.ID
		}
		return spans[i].Start < spans[j].Start
	})
	return spans
}

func clamp01(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func layoutTimelineScale(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	width := gtx.Constraints.Max.X
	height := gtx.Dp(24)
	lineY := gtx.Dp(4)
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, lineY, width, lineY+gtx.Dp(1))).Op())
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(width, height)}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(kit.Text(th, th.T.MonoSm, th.R.Fg3, "60 min ago")),
				layout.Rigid(kit.Text(th, th.T.MonoSm, th.R.Fg3, "Now")),
			)
		}),
	)
}

func layoutTimelineBar(gtx layout.Context, th *theme.Theme, span timelineSpan) layout.Dimensions {
	width := gtx.Constraints.Max.X
	height := gtx.Dp(24)
	trackY := gtx.Dp(11)
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, trackY, width, trackY+gtx.Dp(1))).Op())
	start := int(math.Round(float64(width) * span.Start))
	end := int(math.Round(float64(width) * span.End))
	minimum := gtx.Dp(4)
	if end-start < minimum {
		end = min(width, start+minimum)
		start = max(0, end-minimum)
	}
	if end > start {
		kit.Blade(gtx.Ops, image.Rect(start, gtx.Dp(7), end, gtx.Dp(15)), th.R.Stone(jobTone(span.Job)))
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func jobTone(job model.Job) model.Tone {
	switch job.State {
	case model.JobRunning:
		return model.ToneBusy
	case model.JobDone:
		return model.ToneAlive
	case model.JobFailed:
		return model.ToneBad
	case model.JobCancelled:
		return model.ToneAttention
	default:
		return model.ToneIdle
	}
}
