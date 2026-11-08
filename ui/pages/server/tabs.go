package server

import (
	"math"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type overviewTab struct{}

func (o *overviewTab) Layout(gtx layout.Context, e *env.Env, id model.ID, server *store.Server) layout.Dimensions {
	if server.Probe == nil {
		return kit.Skeleton(e.Th, 5)(gtx)
	}
	if server.Probe.NeedAuth {
		return o.layoutKnown(gtx, e, server)
	}
	probe := server.Probe
	measures := func(gtx layout.Context) layout.Dimensions { return o.layoutMeasures(gtx, e, id, probe) }
	services := func(gtx layout.Context) layout.Dimensions { return o.layoutServices(gtx, e, id, probe) }
	gap := gtx.Dp(e.Th.Sp.S5)
	if gtx.Constraints.Max.X < 2*gtx.Dp(unit.Dp(280))+gap {
		return layout.Flex{Axis: layout.Vertical, Gap: gap}.Layout(gtx, layout.Rigid(measures), layout.Rigid(services))
	}
	return layout.Flex{Gap: gap}.Layout(gtx, layout.Flexed(1, measures), layout.Flexed(1, services))
}

func (o *overviewTab) layoutKnown(gtx layout.Context, e *env.Env, server *store.Server) layout.Dimensions {
	lastReachable := "Not yet"
	if server.Reach != nil && server.Reach.Up {
		lastReachable = "Just now"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(common.SectionTitle(e.Th, "What Ryoport knows", nil)),
		layout.Rigid(kit.Facts(e.Th, []kit.Fact{
			{Label: "Address", Value: server.HostName, Mono: true},
			{Label: "Port", Value: formatInt(server.Port), Mono: true},
			{Label: "User", Value: server.User},
			{Label: "Authentication", Value: "Password"},
			{Label: "Last reachable", Value: lastReachable},
		})),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx,
				kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Nothing else is probed until Ryoport can sign in."))
		}),
	)
}

func (o *overviewTab) layoutMeasures(gtx layout.Context, e *env.Env, id model.ID, probe *remote.Probe) layout.Dimensions {
	var values []float64
	if series := e.Snap.Series(id, model.MetricCPU); series != nil {
		values = series.Values()
	}
	current := probe.CPUPercent / 100
	if len(values) > 0 {
		current = values[len(values)-1]
	}
	memory := float64(0)
	if probe.MemTotalKb > 0 {
		memory = float64(probe.MemTotalKb-probe.MemAvailKb) / float64(probe.MemTotalKb)
	}
	disk := float64(probe.DiskPct) / 100
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(common.SectionTitle(e.Th, "Load over the last 10 minutes",
			kit.Text(e.Th, e.Th.T.Mono, e.Th.R.Fg, percentValue(current)))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.EngravedChart(gtx, e.Th, values, math.NaN(), math.NaN(), unit.Dp(184), []string{"10 min", "5 min", "now"})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return metricBlade(gtx, e, "Memory",
					format.Bytes((probe.MemTotalKb-probe.MemAvailKb)*1024)+" of "+format.Bytes(probe.MemTotalKb*1024),
					memory)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return metricBlade(gtx, e, "Disk",
					format.Bytes(probe.DiskUsedKb*1024)+" of "+format.Bytes(probe.DiskTotalKb*1024),
					disk)
			})
		}),
	)
}

func (o *overviewTab) layoutServices(gtx layout.Context, e *env.Env, id model.ID, probe *remote.Probe) layout.Dimensions {
	names := make([]string, 0, len(probe.Services))
	for name := range probe.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	children := make([]layout.FlexChild, 0, len(names)+5)
	children = append(children, layout.Rigid(common.SectionTitle(e.Th, "Services", nil)))
	if len(names) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No watched services.")))
	} else {
		for _, name := range names {
			name, status := name, probe.Services[name]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return serviceLine(gtx, e, name, status, serviceStatusTone(status))
			}))
		}
	}
	if probe.FailedUnits > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "failed unit"
				if probe.FailedUnits != 1 {
					label = "failed units"
				}
				return serviceLine(gtx, e, "System", formatInt(probe.FailedUnits)+" "+label, model.ToneBad)
			})
		}))
	}

	lines := eventLines(e.Snap.Events, id)
	history := common.TimeLines(e, lines)
	if len(lines) == 0 {
		history = kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Nothing changed since you last looked.")
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx, common.SectionTitle(e.Th, "Since you last looked", nil))
		}),
		layout.Rigid(history),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func metricBlade(gtx layout.Context, e *env.Env, label, value string, frac float64) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, label)),
				layout.Rigid(kit.Text(e.Th, e.Th.T.Mono, e.Th.R.Fg, value)),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.BladeMeter(gtx, e.Th, frac, e.Th.R.Fg2)
		}),
	)
}

func serviceLine(gtx layout.Context, e *env.Env, name, status string, tone model.Tone) layout.Dimensions {
	return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Flake(gtx, e.Th, tone, -1, unit.Dp(10))
			}),
			layout.Flexed(1, kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, name)),
			layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, status)),
		)
	})
}

func serviceStatusTone(status string) model.Tone {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "running", "online", "ok", "up":
		return model.ToneAlive
	case "failed", "error", "offline", "down":
		return model.ToneBad
	case "warn", "warning", "degraded":
		return model.ToneAttention
	case "paused":
		return model.TonePaused
	}
	return model.ToneIdle
}

func eventLines(events []model.Event, id model.ID) []common.TimeLine {
	lines := make([]common.TimeLine, 0, 5)
	for _, event := range events {
		if len(lines) == 5 {
			break
		}
		if event.Target != id {
			continue
		}
		lines = append(lines, common.TimeLine{Text: event.Text, At: event.At})
	}
	return lines
}

func formatInt(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func cappedField(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(520)))
	return widget(gtx)
}
