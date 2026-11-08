package activity

import (
	"image"
	"sort"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type itemKind uint8

const (
	itemPageTitle itemKind = iota
	itemSection
	itemJob
	itemScale
	itemTimeline
	itemAlert
	itemDay
	itemEvent
	itemEmpty
)

type activityItem struct {
	kind  itemKind
	text  string
	index int
}

// View is the full Activity page.
type View struct {
	list         kit.List
	picker       kit.Picker
	filter       model.ID
	filterIDs    []model.ID
	filterLabels []string
	filterRev    uint64
	filtersBuilt bool
	items        []activityItem
	groups       []eventGroup
	spans        []timelineSpan
	jobs         []model.Job
	alerts       []model.Alert
	jobButtons   map[string]*kit.Button
	fixButtons   map[string]*kit.Button
	dataRev      uint64
	dataFilter   model.ID
	dataDay      int
	dataBuilt    bool
}

func New() *View {
	view := &View{
		jobButtons: make(map[string]*kit.Button),
		fixButtons: make(map[string]*kit.Button),
	}
	view.list.Axis = layout.Vertical
	return view
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	paint.FillShape(gtx.Ops, e.Th.R.Stone2, clip.Rect{Max: gtx.Constraints.Max}.Op())
	if !v.filtersBuilt || v.filterRev != e.Snap.Rev {
		v.filterIDs, v.filterLabels, _ = activityFilters(e.Snap, v.filter)
		v.filterRev = e.Snap.Rev
		v.filtersBuilt = true
	}
	if index, changed := v.picker.Changed(gtx); changed && index >= 0 && index < len(v.filterIDs) {
		v.filter = v.filterIDs[index]
	}
	selected := selectedFilter(v.filterIDs, v.filter)
	v.filter = v.filterIDs[selected]
	day := e.Now.Year()*400 + e.Now.YearDay()
	if !v.dataBuilt || v.dataRev != e.Snap.Rev || v.dataFilter != v.filter || v.dataDay != day {
		v.rebuild(e)
		v.dataRev, v.dataFilter, v.dataDay, v.dataBuilt = e.Snap.Rev, v.filter, day, true
	}
	v.handleActions(gtx, e)
	return layout.Inset{Top: 34, Bottom: 34, Left: 36, Right: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.list.Layout(gtx, e.Th, len(v.items), func(gtx layout.Context, index int) layout.Dimensions {
			item := v.items[index]
			switch item.kind {
			case itemPageTitle:
				return v.layoutTitle(gtx, e, v.filterLabels, selected)
			case itemSection:
				return layout.Inset{Top: 21, Bottom: 8}.Layout(gtx, kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, item.text))
			case itemJob:
				return v.layoutJob(gtx, e, v.jobs[item.index])
			case itemScale:
				return layout.Inset{Left: 180}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutTimelineScale(gtx, e.Th)
				})
			case itemTimeline:
				return v.layoutTimelineRow(gtx, e, v.spans[item.index])
			case itemAlert:
				return v.layoutAlert(gtx, e, v.alerts[item.index])
			case itemDay:
				return layout.Inset{Top: 13, Bottom: 4}.Layout(gtx, kit.Text(e.Th, e.Th.T.Group, e.Th.R.Fg3, item.text))
			case itemEvent:
				group := v.groups[item.index>>16]
				return v.layoutEvent(gtx, e, group.Events[item.index&0xffff])
			case itemEmpty:
				return layout.Inset{Top: 21}.Layout(gtx, kit.Empty(e.Th, icons.History, "No activity yet", "Work on a machine or connect to a server and its history will appear here."))
			default:
				return layout.Dimensions{}
			}
		})
	})
}

func (v *View) rebuild(e *env.Env) {
	v.items = v.items[:0]
	v.jobs = v.jobs[:0]
	for _, job := range e.Snap.Jobs {
		if job.State == model.JobRunning && v.matches(job.Target) {
			v.jobs = append(v.jobs, job)
		}
	}
	v.alerts = v.alerts[:0]
	for _, alert := range e.Snap.Alerts {
		if v.matches(alert.Target) {
			v.alerts = append(v.alerts, alert)
		}
	}
	filteredEvents := make([]model.Event, 0, len(e.Snap.Events))
	for _, event := range e.Snap.Events {
		if v.matches(event.Target) {
			filteredEvents = append(filteredEvents, event)
		}
	}
	v.groups = groupEvents(filteredEvents, e.Now)
	filteredJobs := make([]model.Job, 0, len(e.Snap.Jobs))
	for _, job := range e.Snap.Jobs {
		if v.matches(job.Target) {
			filteredJobs = append(filteredJobs, job)
		}
	}
	v.spans = jobTimeline(filteredJobs, e.Now)

	v.items = append(v.items, activityItem{kind: itemPageTitle})
	if len(v.jobs) > 0 {
		v.items = append(v.items, activityItem{kind: itemSection, text: "In progress"})
		for index := range v.jobs {
			v.items = append(v.items, activityItem{kind: itemJob, index: index})
		}
	}
	if len(v.spans) > 0 {
		v.items = append(v.items, activityItem{kind: itemSection, text: "Last hour"}, activityItem{kind: itemScale})
		for index := range v.spans {
			v.items = append(v.items, activityItem{kind: itemTimeline, index: index})
		}
	}
	if len(v.alerts) > 0 {
		v.items = append(v.items, activityItem{kind: itemSection, text: "Needs you"})
		for index := range v.alerts {
			v.items = append(v.items, activityItem{kind: itemAlert, index: index})
		}
	}
	if len(v.groups) > 0 {
		v.items = append(v.items, activityItem{kind: itemSection, text: "History"})
		for groupIndex, group := range v.groups {
			v.items = append(v.items, activityItem{kind: itemDay, text: group.Label})
			for eventIndex := range group.Events {
				v.items = append(v.items, activityItem{kind: itemEvent, index: groupIndex<<16 | eventIndex})
			}
		}
	}
	if len(v.jobs) == 0 && len(v.spans) == 0 && len(v.alerts) == 0 && len(v.groups) == 0 {
		v.items = append(v.items, activityItem{kind: itemEmpty})
	}
}

func (v *View) matches(target model.ID) bool {
	return v.filter == "" || target == v.filter
}

func (v *View) handleActions(gtx layout.Context, e *env.Env) {
	for _, job := range v.jobs {
		button := v.jobButton(job.ID)
		if button.Clicked(gtx) && job.Cancelable {
			e.Ops.Cancel(job.ID)
		}
	}
	for _, alert := range v.alerts {
		if alert.Fix == model.FixNone {
			continue
		}
		if v.fixButton(alert.ID).Clicked(gtx) {
			applyFix(e, alert)
		}
	}
}

func (v *View) layoutTitle(gtx layout.Context, e *env.Env, labels []string, selected int) layout.Dimensions {
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Activity")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.picker.Layout(gtx, e.Th, "Resource", labels, selected)
			}),
		)
	})
}

func (v *View) layoutJob(gtx layout.Context, e *env.Env, job model.Job) layout.Dimensions {
	button := v.jobButton(job.ID)
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Flexed(1, kit.Text(e.Th, e.Th.T.Row, e.Th.R.Fg, job.Title)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !job.Cancelable {
									return layout.Dimensions{}
								}
								button.Small = true
								return button.Layout(gtx, e.Th, kit.Quiet, icons.Cancel, "Cancel")
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 4, Bottom: 8}.Layout(gtx, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg2, jobDetails(job, e.Now)))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.BladeProgress(gtx, e.Th, job.Progress, job.State == model.JobRunning)
					}),
				)
			})
		})
	})
}

func (v *View) layoutTimelineRow(gtx layout.Context, e *env.Env, span timelineSpan) layout.Dimensions {
	label := span.Job.Title
	if span.Job.Phase != "" {
		label += ", " + strings.ToLower(span.Job.Phase)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(180)
			gtx.Constraints.Max.X = gtx.Dp(180)
			return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2, MaxLines: 1}.Layout(gtx, e.Th, label)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layoutTimelineBar(gtx, e.Th, span)
		}),
	)
}

func (v *View) layoutAlert(gtx layout.Context, e *env.Env, alert model.Alert) layout.Dimensions {
	tone := model.ToneAttention
	if alert.Severity == model.SeverityBad {
		tone = model.ToneBad
	}
	button := v.fixButton(alert.ID)
	label, icon, hasAction := fixAction(alert.Fix)
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 13, Bottom: 13, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(13)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Flake(gtx, e.Th, tone, 1, 12) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(kit.Text(e.Th, e.Th.T.Row, e.Th.R.Fg, alert.Title)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2}.Layout(gtx, e.Th, alert.Body)
								})
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !hasAction {
							return layout.Dimensions{}
						}
						button.Small = true
						return button.Layout(gtx, e.Th, kit.Gold, icon, label)
					}),
				)
			})
		})
	})
}

func (v *View) layoutEvent(gtx layout.Context, e *env.Env, event model.Event) layout.Dimensions {
	tone := eventTone(event.Level)
	detail := event.Detail
	if event.Target != "" && v.filter == "" {
		if detail != "" {
			detail = event.Target.Key() + "  " + detail
		} else {
			detail = event.Target.Key()
		}
	}
	return layout.Inset{Bottom: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				line := max(1, gtx.Dp(1))
				paint.FillShape(gtx.Ops, e.Th.R.Vein, clip.Rect(image.Rect(0, size.Y-line, size.X, size.Y)).Op())
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(13)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Flake(gtx, e.Th, tone, 1, 10) }),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, event.Text)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if detail == "" {
										return layout.Dimensions{}
									}
									return layout.Inset{Top: 2}.Layout(gtx, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, detail))
								}),
							)
						}),
						layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, event.At.In(e.Now.Location()).Format("15:04"))),
					)
				})
			}),
		)
	})
}

func (v *View) jobButton(id string) *kit.Button {
	button := v.jobButtons[id]
	if button == nil {
		button = new(kit.Button)
		v.jobButtons[id] = button
	}
	return button
}

func (v *View) fixButton(id string) *kit.Button {
	button := v.fixButtons[id]
	if button == nil {
		button = new(kit.Button)
		v.fixButtons[id] = button
	}
	return button
}

func activityFilters(snapshot *store.Snapshot, selected model.ID) ([]model.ID, []string, int) {
	type option struct {
		id    model.ID
		label string
	}
	seen := make(map[model.ID]bool)
	options := make([]option, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		seen[resource.ID] = true
		options = append(options, option{id: resource.ID, label: resource.Name})
	}
	for _, event := range snapshot.Events {
		if event.Target != "" && !seen[event.Target] {
			seen[event.Target] = true
			options = append(options, option{id: event.Target, label: event.Target.Key()})
		}
	}
	for _, job := range snapshot.Jobs {
		if job.Target != "" && !seen[job.Target] {
			seen[job.Target] = true
			options = append(options, option{id: job.Target, label: job.Target.Key()})
		}
	}
	for _, alert := range snapshot.Alerts {
		if alert.Target != "" && !seen[alert.Target] {
			seen[alert.Target] = true
			options = append(options, option{id: alert.Target, label: alert.Target.Key()})
		}
	}
	sort.SliceStable(options, func(i, j int) bool { return strings.ToLower(options[i].label) < strings.ToLower(options[j].label) })
	ids := make([]model.ID, 1, len(options)+1)
	labels := make([]string, 1, len(options)+1)
	labels[0] = "All resources"
	selectedIndex := 0
	for _, option := range options {
		ids = append(ids, option.id)
		labels = append(labels, option.label)
		if option.id == selected {
			selectedIndex = len(ids) - 1
		}
	}
	return ids, labels, selectedIndex
}

func selectedFilter(ids []model.ID, selected model.ID) int {
	for index, id := range ids {
		if id == selected {
			return index
		}
	}
	return 0
}

func eventTone(level model.EventLevel) model.Tone {
	switch level {
	case model.EventOK:
		return model.ToneAlive
	case model.EventWarn:
		return model.ToneAttention
	case model.EventError:
		return model.ToneBad
	default:
		return model.ToneIdle
	}
}

func fixAction(fix model.Fix) (string, *widget.Icon, bool) {
	switch fix {
	case model.FixSavePassword:
		return "Enter password", icons.Key, true
	case model.FixDeployKey:
		return "Deploy a key", icons.VpnKey, true
	case model.FixInstallEngine:
		return "Install the engine", icons.InstallDesktop, true
	case model.FixEnableKVM:
		return "Review engine", icons.DeveloperBoard, true
	case model.FixOpenHubGPU:
		return "Open GPU settings", icons.Hub, true
	case model.FixOpenSettings:
		return "Open settings", icons.Settings, true
	default:
		return "", nil, false
	}
}

func applyFix(e *env.Env, alert model.Alert) {
	switch alert.Fix {
	case model.FixSavePassword:
		e.Nav.Select(alert.Target)
	case model.FixDeployKey:
		e.Nav.Select(alert.Target)
		e.Nav.SetTab(model.KindServer, "Access")
	case model.FixInstallEngine:
		e.Ops.InstallEngine()
	case model.FixOpenHubGPU:
		e.Ops.OpenHubGPU()
	case model.FixEnableKVM, model.FixOpenSettings:
		e.Nav.Go(store.PageSettings)
	}
}

func jobDetails(job model.Job, now time.Time) string {
	parts := make([]string, 0, 3)
	if job.Phase != "" {
		parts = append(parts, job.Phase)
	}
	if job.Total > 0 {
		progress := format.Progress(job.Done, job.Total)
		if job.Rate > 0 {
			progress += " at " + format.Rate(job.Rate)
		}
		parts = append(parts, progress)
		if remaining := job.Total - job.Done; remaining > 0 && job.Rate > 0 {
			parts = append(parts, format.Left(time.Duration(float64(remaining)/job.Rate*float64(time.Second))))
		}
	} else if job.Progress >= 0 {
		parts = append(parts, format.Percent(job.Progress))
	} else if !job.Started.IsZero() {
		parts = append(parts, format.Duration(now.Sub(job.Started)))
	}
	return strings.Join(parts, ", ")
}
