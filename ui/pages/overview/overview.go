package overview

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type boardBlock uint8

const (
	blockHeader boardBlock = iota
	blockPinned
	blockColumns
	blockNeeds
	blockRunning
	blockComputer
	blockRecent
)

type attentionItem struct {
	alert    model.Alert
	resource model.Resource
	isAlert  bool
}

type eventSlot struct {
	index int
	event model.Event
}

type recentItem struct {
	event model.Event
	key   eventSlot
}

type resourceArea uint8

const (
	areaPinned resourceArea = iota
	areaRunning
)

type resourceSlot struct {
	area resourceArea
	id   model.ID
}

type resourceLabel struct {
	name  string
	shown string
	tip   string
}

type View struct {
	list           kit.List
	resourceClick  map[resourceSlot]*kit.Click
	actionButtons  map[resourceSlot]*kit.Button
	resourceLabels map[resourceSlot]resourceLabel
	alertClicks    map[string]*kit.Click
	alertButtons   map[string]*kit.Button
	eventClicks    map[eventSlot]*kit.Click
	allActivity    kit.Click

	haveRev   bool
	rev       uint64
	pinned    []model.Resource
	running   []model.Resource
	attention []attentionItem
	recent    []recentItem
	blocks    []boardBlock
}

func New() *View {
	return &View{
		resourceClick:  make(map[resourceSlot]*kit.Click),
		actionButtons:  make(map[resourceSlot]*kit.Button),
		resourceLabels: make(map[resourceSlot]resourceLabel),
		alertClicks:    make(map[string]*kit.Click),
		alertButtons:   make(map[string]*kit.Button),
		eventClicks:    make(map[eventSlot]*kit.Click),
	}
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Th == nil || e.Snap == nil || e.Nav == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	v.refresh(e.Snap)
	v.handleClicks(gtx, e)

	return layout.Inset{Top: 30, Bottom: 34, Left: 36, Right: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(1240)))
		return centered(gtx, width, func(gtx layout.Context) layout.Dimensions {
			expanded := gtx.Constraints.Max.X >= gtx.Dp(unit.Dp(860))
			v.prepareBlocks(expanded)
			return v.list.Layout(gtx, e.Th, len(v.blocks), func(gtx layout.Context, index int) layout.Dimensions {
				return v.layoutBlock(gtx, e, v.blocks[index])
			})
		})
	})
}

func (v *View) refresh(snapshot *store.Snapshot) {
	if v.haveRev && snapshot.Rev == v.rev {
		return
	}
	v.haveRev = true
	v.rev = snapshot.Rev
	v.pinned = v.pinned[:0]
	v.running = v.running[:0]
	v.attention = v.attention[:0]
	v.recent = v.recent[:0]

	for _, alert := range snapshot.Alerts {
		v.attention = append(v.attention, attentionItem{alert: alert, isAlert: true})
	}
	for _, resource := range snapshot.Resources {
		if resource.Pinned {
			v.pinned = append(v.pinned, resource)
			v.ensureResourceLabel(resourceSlot{area: areaPinned, id: resource.ID}, resource.Name, "pinned")
		}
		if runningNow(resource) {
			v.running = append(v.running, resource)
			v.ensureResourceLabel(resourceSlot{area: areaRunning, id: resource.ID}, resource.Name, "running now")
		}
		if resource.State == model.StateNeedsAuth && !hasAlertTarget(snapshot.Alerts, resource.ID) {
			v.attention = append(v.attention, attentionItem{resource: resource})
		}
	}
	count := min(6, len(snapshot.Events))
	for index, event := range snapshot.Events[:count] {
		v.recent = append(v.recent, recentItem{event: event, key: eventSlot{index: index, event: event}})
	}
	for key := range v.eventClicks {
		if key.index >= count || snapshot.Events[key.index] != key.event {
			delete(v.eventClicks, key)
		}
	}
	for key := range v.alertClicks {
		if !v.hasAttentionKey(key) {
			delete(v.alertClicks, key)
			delete(v.alertButtons, key)
		}
	}
	for key := range v.resourceClick {
		if !v.hasResourceSlot(key) {
			delete(v.resourceClick, key)
			delete(v.actionButtons, key)
			delete(v.resourceLabels, key)
		}
	}
}

func (v *View) hasResourceSlot(key resourceSlot) bool {
	resources := v.running
	if key.area == areaPinned {
		resources = v.pinned
	}
	for _, resource := range resources {
		if resource.ID == key.id {
			return true
		}
	}
	return false
}

func (v *View) hasAttentionKey(key string) bool {
	for _, item := range v.attention {
		if item.isAlert && item.alert.ID == key {
			return true
		}
		if !item.isAlert && "auth:"+string(item.resource.ID) == key {
			return true
		}
	}
	return false
}

func (v *View) ensureResourceLabel(key resourceSlot, name, context string) {
	if label, ok := v.resourceLabels[key]; ok && label.name == name {
		return
	}
	limit := 42
	if key.area == areaPinned {
		limit = 28
	}
	v.resourceLabels[key] = resourceLabel{name: name, shown: elideName(name, limit), tip: name + " (" + context + ")"}
}

func hasAlertTarget(alerts []model.Alert, id model.ID) bool {
	for _, alert := range alerts {
		if alert.Target == id {
			return true
		}
	}
	return false
}

func (v *View) prepareBlocks(expanded bool) {
	v.blocks = v.blocks[:0]
	v.blocks = append(v.blocks, blockHeader, blockPinned)
	if expanded {
		v.blocks = append(v.blocks, blockColumns)
		return
	}
	if len(v.attention) > 0 {
		v.blocks = append(v.blocks, blockNeeds)
	}
	v.blocks = append(v.blocks, blockRunning, blockComputer, blockRecent)
}

func (v *View) layoutBlock(gtx layout.Context, e *env.Env, block boardBlock) layout.Dimensions {
	switch block {
	case blockHeader:
		return layout.Inset{Bottom: e.Th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, e)
		})
	case blockPinned:
		return layout.Inset{Bottom: e.Th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return v.layoutPinned(gtx, e)
		})
	case blockColumns:
		return v.layoutColumns(gtx, e)
	case blockNeeds:
		return layout.Inset{Bottom: e.Th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return v.layoutNeeds(gtx, e)
		})
	case blockRunning:
		return layout.Inset{Bottom: e.Th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return v.layoutRunning(gtx, e)
		})
	case blockComputer:
		return layout.Inset{Bottom: e.Th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return v.layoutComputer(gtx, e)
		})
	case blockRecent:
		return v.layoutRecent(gtx, e)
	default:
		return layout.Dimensions{}
	}
}

func (v *View) layoutHeader(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return fleetSentence(gtx, e) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: e.Th.T.Proof, Color: e.Th.R.Fg2, MaxLines: 2}.Layout(gtx, e.Th, fleetProof(e.Snap))
		}),
	)
}

func fleetSentence(gtx layout.Context, e *env.Env) layout.Dimensions {
	spans, attention := common.FleetSentence(e.Snap)
	if attention >= 0 && attention < len(spans) {
		spans[attention].Color = e.Th.R.Amber
	}
	return common.Sentence(gtx, e.Th, e.Th.T.Sentence, e.Th.R.Fg, spans)
}

// fleetProof backs the sentence with counts. It says nothing about when
// things were checked: the board only redraws when something changes, so an
// "ago" would go stale on screen.
func fleetProof(snapshot *store.Snapshot) string {
	servers, serversUp := 0, 0
	machines, machinesStopped := 0, 0
	consoles, consolesUp := 0, 0
	for _, resource := range snapshot.Resources {
		switch resource.Kind {
		case model.KindServer:
			servers++
			if resource.State == model.StateOnline || resource.State == model.StateDegraded {
				serversUp++
			}
		case model.KindMachine, model.KindPassthrough:
			machines++
			if resource.State == model.StateStopped {
				machinesStopped++
			}
		case model.KindConsole:
			consoles++
			if resource.State == model.StateOnline {
				consolesUp++
			}
		}
	}
	var facts strings.Builder
	appendFact := func(value string) {
		if facts.Len() > 0 {
			facts.WriteString(", ")
		}
		facts.WriteString(value)
	}
	if servers > 0 {
		appendFact(countOf(serversUp, servers, "server") + " up")
	}
	if machines > 0 {
		appendFact(count(machinesStopped, "machine") + " stopped")
	}
	if consoles > 0 {
		appendFact(countOf(consolesUp, consoles, "console") + " reachable")
	}
	if facts.Len() == 0 {
		return "No machines or servers yet."
	}
	return facts.String() + "."
}

func countOf(up, total int, singular string) string {
	word := singular
	if total != 1 {
		word += "s"
	}
	return strconv.Itoa(up) + " of " + strconv.Itoa(total) + " " + word
}

func count(value int, singular string) string {
	word := singular
	if value != 1 {
		word += "s"
	}
	return strconv.Itoa(value) + " " + word
}

func (v *View) layoutPinned(gtx layout.Context, e *env.Env) layout.Dimensions {
	return section(gtx, e, "Pinned", func(gtx layout.Context) layout.Dimensions {
		if len(v.pinned) == 0 {
			return quietSlab(gtx, e, "Pin the machines you reach most. Pin is in each machine's menu.")
		}
		return kit.Columns(gtx, 240, e.Th.Sp.S3, len(v.pinned), func(gtx layout.Context, index int) layout.Dimensions {
			return v.layoutPinnedTile(gtx, e, v.pinned[index])
		})
	})
}

func (v *View) layoutPinnedTile(gtx layout.Context, e *env.Env, resource model.Resource) layout.Dimensions {
	key := resourceSlot{area: areaPinned, id: resource.ID}
	click := v.resourceClickFor(key)
	action := v.actionButtonFor(key)
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(154))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	dims := kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(e.Th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						semantic.Button.Add(gtx.Ops)
						semantic.LabelOp(resource.Name).Add(gtx.Ops)
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
							layout.Rigid(common.Mark(e, resource, 40, e.Th.R.Fg)),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								claim, proof := common.StateWords(e, resource)
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.resourceName(gtx, e, resource.Name, 28, key) }),
									layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg2, stateProof(claim, proof))),
								)
							}),
						)
					})
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label, icon := primaryPresentation(resource)
					if label == "" {
						return layout.Dimensions{}
					}
					action.Small = true
					return action.Layout(gtx, e.Th, kit.Gold, icon, label)
				}),
			)
		})
	})
	kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(e.Th.Cut.Notch), e.Th.R.Stone(resource.State.Tone()))
	return dims
}

func (v *View) layoutColumns(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S5)}.Layout(gtx,
		layout.Flexed(.6, func(gtx layout.Context) layout.Dimensions {
			if len(v.attention) == 0 {
				return v.layoutRunning(gtx, e)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutNeeds(gtx, e) }),
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutRunning(gtx, e) }),
			)
		}),
		layout.Flexed(.4, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutComputer(gtx, e) }),
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutRecent(gtx, e) }),
			)
		}),
	)
}

func (v *View) layoutNeeds(gtx layout.Context, e *env.Env) layout.Dimensions {
	if len(v.attention) == 0 {
		return layout.Dimensions{}
	}
	return section(gtx, e, "Needs you", func(gtx layout.Context) layout.Dimensions {
		return flatColumn(gtx, len(v.attention), func(gtx layout.Context, index int) layout.Dimensions {
			return v.layoutAttentionRow(gtx, e, v.attention[index])
		})
	})
}

func (v *View) layoutAttentionRow(gtx layout.Context, e *env.Env, item attentionItem) layout.Dimensions {
	if !item.isAlert {
		resource := item.resource
		alert := model.Alert{ID: "auth:" + string(resource.ID), Target: resource.ID, Severity: model.SeverityWarn, Title: resource.Name + " needs a password", Body: "Sign in to connect.", Fix: model.FixSavePassword}
		return v.layoutAlertRow(gtx, e, alert, resource.State.Tone())
	}
	tone := model.ToneAttention
	if item.alert.Severity == model.SeverityBad {
		tone = model.ToneBad
	}
	return v.layoutAlertRow(gtx, e, item.alert, tone)
}

func (v *View) layoutAlertRow(gtx layout.Context, e *env.Env, alert model.Alert, tone model.Tone) layout.Dimensions {
	click := v.alertClickFor(alert.ID)
	button := v.alertButtonFor(alert.ID)
	return rowSlab(gtx, e, tone, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				body := func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.Label{Style: e.Th.T.Title, Color: e.Th.R.Fg, MaxLines: 2}.Layout(gtx, e.Th, alert.Title)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if alert.Body == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Top: e.Th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2, MaxLines: 2}.Layout(gtx, e.Th, alert.Body)
							})
						}),
					)
				}
				if alert.Target == "" {
					return body(gtx)
				}
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					semantic.Button.Add(gtx.Ops)
					semantic.LabelOp(alert.Title).Add(gtx.Ops)
					return body(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label, icon := alertPresentation(alert)
				if label == "" {
					return layout.Dimensions{}
				}
				button.Small = true
				kind := kit.Gold
				if alert.Fix == model.FixNone {
					kind = kit.Quiet
				}
				return button.Layout(gtx, e.Th, kind, icon, label)
			}),
		)
	})
}

func (v *View) layoutRunning(gtx layout.Context, e *env.Env) layout.Dimensions {
	return section(gtx, e, "Running now", func(gtx layout.Context) layout.Dimensions {
		if len(v.running) == 0 {
			return quietSlab(gtx, e, "Nothing is running right now.")
		}
		return flatColumn(gtx, len(v.running), func(gtx layout.Context, index int) layout.Dimensions {
			return v.layoutRunningRow(gtx, e, v.running[index])
		})
	})
}

func (v *View) layoutRunningRow(gtx layout.Context, e *env.Env, resource model.Resource) layout.Dimensions {
	key := resourceSlot{area: areaRunning, id: resource.ID}
	click := v.resourceClickFor(key)
	action := v.actionButtonFor(key)
	return rowSlab(gtx, e, resource.State.Tone(), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					semantic.Button.Add(gtx.Ops)
					semantic.LabelOp(resource.Name).Add(gtx.Ops)
					return v.layoutRunningBody(gtx, e, resource)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label, icon := primaryPresentation(resource)
				if label == "" {
					return layout.Dimensions{}
				}
				action.Small = true
				return action.Layout(gtx, e.Th, kit.Gold, icon, label)
			}),
		)
	})
}

func (v *View) layoutRunningBody(gtx layout.Context, e *env.Env, resource model.Resource) layout.Dimensions {
	left := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(common.Mark(e, resource, 34, e.Th.R.Fg)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						key := resourceSlot{area: areaRunning, id: resource.ID}
						return v.resourceName(gtx, e, resource.Name, 42, key)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: e.Th.Sp.S1}.Layout(gtx, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg2, runningProof(e, resource)))
					}),
				)
			}),
		)
	}
	series := e.Snap.Series(resource.ID, model.MetricCPU)
	if series == nil || series.Len() < 2 || gtx.Constraints.Max.X < gtx.Dp(unit.Dp(410)) {
		return left(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
		layout.Flexed(1, left),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(unit.Dp(150))
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return kit.EngravedChart(gtx, e.Th, series.Values(), 0, 1, 24, nil)
		}),
	)
}

func runningProof(e *env.Env, resource model.Resource) string {
	var text strings.Builder
	appendPart := func(part string) {
		if part == "" {
			return
		}
		if text.Len() > 0 {
			text.WriteString(", ")
		}
		text.WriteString(part)
	}
	claim, proof := common.StateWords(e, resource)
	appendPart(claim)
	appendPart(proof)
	switch resource.Kind {
	case model.KindMachine:
		if machine := e.Snap.Machines[resource.ID.Key()]; machine != nil && machine.Stats != nil && machine.Stats.UptimeS != nil && *machine.Stats.UptimeS > 0 {
			appendPart("up " + format.Duration(time.Duration(*machine.Stats.UptimeS)*time.Second))
		}
	case model.KindPassthrough:
		if series := e.Snap.Series(resource.ID, model.MetricCPU); series != nil {
			if cpu, _, ok := series.Last(); ok {
				appendPart(format.Percent(cpu) + " CPU")
			}
		}
	case model.KindServer:
		if server := e.Snap.Servers[resource.ID.Key()]; server != nil && server.Probe != nil && server.Probe.OK {
			appendPart("load " + strconv.FormatFloat(server.Probe.Load1, 'f', 2, 64))
			if server.Probe.UptimeS > 0 {
				appendPart("up " + format.Duration(time.Duration(server.Probe.UptimeS)*time.Second))
			}
		}
	}
	return text.String()
}

func (v *View) layoutComputer(gtx layout.Context, e *env.Env) layout.Dimensions {
	return section(gtx, e, "This computer", func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(e.Th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layoutHostMeters(gtx, e) }),
					layout.Rigid(layout.Spacer{Height: e.Th.Sp.S4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						line, tone := engineLine(e.Snap)
						return factLine(gtx, e, tone, line)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						line, tone := gpuLine(e.Snap)
						return factLine(gtx, e, tone, line)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !e.Snap.Host.OnBattery {
							return layout.Dimensions{}
						}
						return factLine(gtx, e, model.ToneAttention, "On battery")
					}),
				)
			})
		})
	})
}

func layoutHostMeters(gtx layout.Context, e *env.Env) layout.Dimensions {
	stats := e.Snap.HostStats
	if stats.SampledAt.IsZero() {
		return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Host statistics will appear while this board is open.")(gtx)
	}
	cpuValue := "Sampling"
	cpuFraction := 0.0
	if stats.CPUValid {
		cpuValue = format.Percent(stats.CPUFraction)
		cpuFraction = stats.CPUFraction
	}
	cores := strconv.Itoa(stats.Cores) + " cores"
	if stats.Cores == 1 {
		cores = "1 core"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return hostMeter(gtx, e, "CPU", cpuValue+", "+cores, cpuFraction)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			fraction := ratio(stats.MemoryUsed, stats.MemoryTotal)
			value := bytes(stats.MemoryUsed) + " used of " + bytes(stats.MemoryTotal)
			return hostMeter(gtx, e, "Memory", value, fraction)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			used := 1 - ratio(stats.VMDiskFree, stats.VMDiskTotal)
			if stats.VMDiskTotal == 0 {
				used = 0
			}
			value := bytes(stats.VMDiskFree) + " free of " + bytes(stats.VMDiskTotal)
			return hostMeter(gtx, e, "VM disk", value, used)
		}),
	)
}

// hostMeter draws how much of something is in use. Meters are not states,
// so they stay in the quiet ink and only turn amber when nearly full.
func hostMeter(gtx layout.Context, e *env.Env, label, value string, used float64) layout.Dimensions {
	ink := e.Th.R.Fg2
	if used >= 0.9 {
		ink = e.Th.R.Amber
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, label)),
				layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg2, value)),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.BladeMeter(gtx, e.Th, used, ink) }),
	)
}

func engineLine(snapshot *store.Snapshot) (string, model.Tone) {
	engine := snapshot.Host.Engine
	if engine.Ready {
		version := strings.TrimSpace(engine.Version)
		if version == "" {
			return "Quickemu ready", model.ToneAlive
		}
		if !strings.Contains(strings.ToLower(version), "quickemu") {
			version = "Quickemu " + version
		}
		return version + " ready", model.ToneAlive
	}
	if hint := strings.TrimSpace(snapshot.Host.Setup.Hint); hint != "" {
		return "VM engine not ready. " + hint, model.ToneAttention
	}
	switch {
	case !engine.Quickemu:
		return "Quickemu is missing", model.ToneAttention
	case !engine.QEMU:
		return "QEMU is missing", model.ToneAttention
	case !engine.KVM:
		return "KVM is unavailable", model.ToneAttention
	default:
		return "VM engine not ready", model.ToneAttention
	}
}

func gpuLine(snapshot *store.Snapshot) (string, model.Tone) {
	if snapshot.GPU.Ready {
		return "GPU passthrough ready", model.ToneAlive
	}
	line := "GPU passthrough unavailable"
	if blocker := strings.TrimSpace(snapshot.GPU.Blocker); blocker != "" {
		line += ". " + blocker
	}
	return line, model.ToneAttention
}

func factLine(gtx layout.Context, e *env.Env, tone model.Tone, line string) layout.Dimensions {
	return layout.Inset{Top: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: e.Th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return kit.Flake(gtx, e.Th, tone, 1, 9)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2, MaxLines: 3}.Layout(gtx, e.Th, line)
			}),
		)
	})
}

func (v *View) layoutRecent(gtx layout.Context, e *env.Env) layout.Dimensions {
	return section(gtx, e, "Recent", func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2, Left: e.Th.Sp.S3, Right: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				count := len(v.recent) + 1
				if len(v.recent) == 0 {
					count++
				}
				return flatColumn(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
					if len(v.recent) == 0 && index == 0 {
						return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S3}.Layout(gtx, kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "No activity yet."))
					}
					eventIndex := index
					if len(v.recent) == 0 {
						eventIndex--
					}
					if eventIndex >= 0 && eventIndex < len(v.recent) {
						item := v.recent[eventIndex]
						return v.layoutEvent(gtx, e, item)
					}
					return layout.Inset{Top: e.Th.Sp.S3, Bottom: e.Th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return v.allActivity.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							semantic.Button.Add(gtx.Ops)
							semantic.LabelOp("All activity").Add(gtx.Ops)
							ink := e.Th.R.Fg2
							if v.allActivity.Hovered() {
								ink = e.Th.R.Fg
							}
							return kit.Text(e.Th, e.Th.T.Label, ink, "All activity")(gtx)
						})
					})
				})
			})
		})
	})
}

func (v *View) layoutEvent(gtx layout.Context, e *env.Env, item recentItem) layout.Dimensions {
	event := item.event
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Flake(gtx, e.Th, eventTone(event.Level), 1, 9) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: e.Th.T.Body, Color: e.Th.R.Fg, MaxLines: 2}.Layout(gtx, e.Th, event.Text)
				}),
				layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, format.Ago(event.At, e.Now))),
			)
		})
	}
	var dims layout.Dimensions
	if event.Target == "" {
		dims = body(gtx)
	} else {
		click := v.eventClickFor(item.key)
		dims = click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(event.Text).Add(gtx.Ops)
			return body(gtx)
		})
	}
	drawHairline(gtx, e.Th.R.Vein, dims.Size.X)
	return dims
}

func (v *View) handleClicks(gtx layout.Context, e *env.Env) {
	for _, resource := range v.pinned {
		key := resourceSlot{area: areaPinned, id: resource.ID}
		if v.resourceClickFor(key).Clicked(gtx) {
			e.Nav.Select(resource.ID)
		}
		if v.actionButtonFor(key).Clicked(gtx) {
			activateResource(e, resource)
		}
	}
	for _, resource := range v.running {
		key := resourceSlot{area: areaRunning, id: resource.ID}
		if v.resourceClickFor(key).Clicked(gtx) {
			e.Nav.Select(resource.ID)
		}
		if v.actionButtonFor(key).Clicked(gtx) {
			activateResource(e, resource)
		}
	}
	for _, item := range v.attention {
		alert := item.alert
		if !item.isAlert {
			alert = model.Alert{ID: "auth:" + string(item.resource.ID), Target: item.resource.ID, Fix: model.FixSavePassword}
		}
		if alert.Target != "" && v.alertClickFor(alert.ID).Clicked(gtx) {
			e.Nav.Select(alert.Target)
		}
		if v.alertButtonFor(alert.ID).Clicked(gtx) {
			if alert.Fix == model.FixNone {
				if alert.Target != "" {
					e.Nav.Select(alert.Target)
				} else {
					e.Nav.Go(store.PageActivity)
				}
			} else {
				applyFix(e, alert)
			}
		}
	}
	for _, item := range v.recent {
		if item.event.Target != "" && v.eventClickFor(item.key).Clicked(gtx) {
			e.Nav.Select(item.event.Target)
		}
	}
	if v.allActivity.Clicked(gtx) {
		e.Nav.Go(store.PageActivity)
	}
}

func activateResource(e *env.Env, resource model.Resource) {
	if resource.Primary == model.VerbSetPass || resource.Kind == model.KindServer && resource.State == model.StateNeedsAuth {
		e.Nav.AskPassword(resource.ID)
		return
	}
	if e.Ops != nil {
		e.Ops.Primary(resource.ID)
	}
}

func applyFix(e *env.Env, alert model.Alert) {
	switch alert.Fix {
	case model.FixSavePassword:
		e.Nav.AskPassword(alert.Target)
	case model.FixInstallEngine:
		if e.Ops != nil {
			e.Ops.InstallEngine()
		}
	case model.FixOpenHubGPU:
		if e.Ops != nil {
			e.Ops.OpenHubGPU()
		}
	case model.FixDeployKey:
		e.Nav.Select(alert.Target)
		e.Nav.SetTab(model.KindServer, "Access")
	case model.FixOpenSettings, model.FixEnableKVM:
		e.Nav.Go(store.PageSettings)
	}
}

func alertPresentation(alert model.Alert) (string, *widget.Icon) {
	label, icon := fixPresentation(alert.Fix)
	if label == "" {
		if alert.Target != "" {
			return "Open", icons.OpenInNew
		}
		return "Review", icons.OpenInNew
	}
	return label, icon
}

func fixPresentation(fix model.Fix) (string, *widget.Icon) {
	switch fix {
	case model.FixSavePassword:
		return "Sign in", icons.Key
	case model.FixInstallEngine:
		return "Install the engine", icons.Download
	case model.FixOpenHubGPU:
		return "Open GPU settings", icons.Tune
	case model.FixDeployKey:
		return "Deploy a key", icons.VpnKey
	case model.FixOpenSettings, model.FixEnableKVM:
		return "Open settings", icons.Settings
	default:
		return "", nil
	}
}

func (v *View) resourceClickFor(key resourceSlot) *kit.Click {
	click := v.resourceClick[key]
	if click == nil {
		click = new(kit.Click)
		v.resourceClick[key] = click
	}
	return click
}

func (v *View) actionButtonFor(key resourceSlot) *kit.Button {
	button := v.actionButtons[key]
	if button == nil {
		button = new(kit.Button)
		v.actionButtons[key] = button
	}
	return button
}

func (v *View) alertClickFor(key string) *kit.Click {
	click := v.alertClicks[key]
	if click == nil {
		click = new(kit.Click)
		v.alertClicks[key] = click
	}
	return click
}

func (v *View) alertButtonFor(key string) *kit.Button {
	button := v.alertButtons[key]
	if button == nil {
		button = new(kit.Button)
		v.alertButtons[key] = button
	}
	return button
}

func (v *View) eventClickFor(key eventSlot) *kit.Click {
	click := v.eventClicks[key]
	if click == nil {
		click = new(kit.Click)
		v.eventClicks[key] = click
	}
	return click
}

func primaryPresentation(resource model.Resource) (string, *widget.Icon) {
	if resource.Primary == model.VerbSetPass {
		return "Sign in", icons.Key
	}
	return common.PrimaryLabel(resource)
}

func runningNow(resource model.Resource) bool {
	switch resource.Kind {
	case model.KindMachine, model.KindPassthrough:
		return resource.State == model.StateRunning
	case model.KindServer:
		return resource.State == model.StateOnline || resource.State == model.StateDegraded
	case model.KindConsole:
		return resource.State == model.StateOnline
	default:
		return false
	}
}

func section(gtx layout.Context, e *env.Env, title string, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: e.Th.Sp.S2}.Layout(gtx, kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, title))
		}),
		layout.Rigid(body),
	)
}

func quietSlab(gtx layout.Context, e *env.Env, text string) layout.Dimensions {
	return kit.Slab(gtx, e.Th.R.Stone1, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: e.Th.Sp.S4, Bottom: e.Th.Sp.S4, Left: e.Th.Sp.S4, Right: e.Th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: e.Th.T.Body, Color: e.Th.R.Fg2, MaxLines: 3}.Layout(gtx, e.Th, text)
		})
	})
}

func rowSlab(gtx layout.Context, e *env.Env, tone model.Tone, body layout.Widget) layout.Dimensions {
	dims := kit.Slab(gtx, e.Th.R.Stone1, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: e.Th.Sp.S3, Bottom: e.Th.Sp.S3, Left: e.Th.Sp.S4, Right: e.Th.Sp.S3}.Layout(gtx, body)
	})
	kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(e.Th.Cut.Notch), e.Th.R.Stone(tone))
	return dims
}

func flatColumn(gtx layout.Context, count int, row func(layout.Context, int) layout.Dimensions) layout.Dimensions {
	y := 0
	width := gtx.Constraints.Min.X
	gap := gtx.Dp(unit.Dp(4))
	for index := range count {
		child := gtx
		child.Constraints.Min.Y = 0
		child.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-y)
		macro := op.Record(gtx.Ops)
		dims := row(child, index)
		call := macro.Stop()
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		y += dims.Size.Y
		if index+1 < count {
			y += gap
		}
		width = max(width, dims.Size.X)
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, y))}
}

func (v *View) resourceName(gtx layout.Context, e *env.Env, name string, maxRunes int, key resourceSlot) layout.Dimensions {
	shown, tip := name, name
	if label, ok := v.resourceLabels[key]; ok {
		shown, tip = label.shown, label.tip
	} else {
		shown = elideName(name, maxRunes)
	}
	return kit.Tooltip(gtx, e.Th, tip, func(gtx layout.Context) layout.Dimensions {
		return kit.Label{Style: e.Th.T.Title, Color: e.Th.R.Fg, MaxLines: 2}.Layout(gtx, e.Th, shown)
	})
}

func elideName(name string, limit int) string {
	if limit < 2 || utf8.RuneCountInString(name) <= limit {
		return name
	}
	runes := []rune(name)
	cut := -1
	for index := min(limit-1, len(runes)-1); index > 0; index-- {
		if unicode.IsSpace(runes[index]) || strings.ContainsRune("-_/.", runes[index]) {
			cut = index
			break
		}
	}
	if cut <= 0 {
		return name
	}
	return strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace) + "…"
}

func stateProof(claim, proof string) string {
	if proof == "" {
		return claim
	}
	return claim + ", " + proof
}

func ratio(value, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return min(1, max(0, float64(value)/float64(total)))
}

func bytes(value uint64) string {
	const maxInt64 = uint64(1<<63 - 1)
	if value > maxInt64 {
		value = maxInt64
	}
	return format.Bytes(int64(value))
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

func centered(gtx layout.Context, width int, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			return body(gtx)
		}),
		layout.Flexed(1, layout.Spacer{}.Layout),
	)
}

func drawHairline(gtx layout.Context, c color.NRGBA, width int) {
	height := max(1, gtx.Dp(unit.Dp(1)))
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(0, 0, width, height)).Op())
}
