package machine

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

func proofFor(machine *store.Machine, resource model.Resource, events []model.Event, now time.Time) []common.Span {
	if machine == nil {
		return []common.Span{{Text: "Machine details are still arriving."}}
	}
	facts := hardwarePhrase(machine)
	switch resource.State {
	case model.StateRunning:
		opening := "Running"
		if uptime := machineUptime(machine, resource.ID, events, now); uptime > 0 {
			opening += " for " + format.Duration(uptime)
		}
		opening += "."
		if facts != "" {
			opening += " " + facts + "."
		}
		spans := []common.Span{{Text: opening}}
		if machine.Stats != nil && machine.Stats.GuestIP != "" {
			spans = append(spans,
				common.Span{Text: " Guest "},
				common.Span{Text: machine.Stats.GuestIP, Mono: true},
				common.Span{Text: "."},
			)
		}
		return spans
	case model.StateStarting, model.StateStopping, model.StateCreating:
		opening := resource.State.Label() + "."
		if facts != "" {
			opening += " " + facts + "."
		}
		return []common.Span{{Text: opening}}
	case model.StatePaused:
		opening := "Paused."
		if facts != "" {
			opening += " " + facts + "."
		}
		return []common.Span{{Text: opening}}
	case model.StateAbsent:
		return []common.Span{{Text: "Ready to install. Start the machine to open its installer."}}
	case model.StateError:
		return []common.Span{{Text: "The machine could not be read. Check Activity for what happened."}}
	default:
		opening := "Stopped"
		if at, ok := transitionAt(events, resource.ID, "Stopped "+resource.Name); ok {
			opening += " since " + format.Ago(at, now)
		}
		opening += "."
		if facts != "" {
			opening += " " + facts + "."
		}
		if count := len(userSnapshots(machine.Snapshots)); count > 0 {
			opening += fmt.Sprintf(" %d %s.", count, plural(count, "snapshot", "snapshots"))
		}
		return []common.Span{{Text: opening}}
	}
}

func jobProof(job model.Job) []common.Span {
	return []common.Span{{Text: jobSummary(job)}}
}

func jobSummary(job model.Job) string {
	phase := strings.TrimSpace(job.Phase)
	if phase == "" {
		phase = "Preparing"
	}
	if progress := jobProgressSummary(job); progress != "" {
		return phase + ", " + progress
	}
	if detail := strings.TrimSpace(job.Detail); detail != "" {
		return phase + ". " + detail
	}
	return phase
}

func jobProgressSummary(job model.Job) string {
	summary := ""
	if job.Total > 0 {
		summary = format.Progress(job.Done, job.Total)
	}
	if job.Rate > 0 {
		if summary != "" {
			summary += " at "
		}
		summary += format.Rate(job.Rate)
		if job.Total > job.Done {
			left := time.Duration(float64(job.Total-job.Done) / job.Rate * float64(time.Second))
			summary += ", " + format.Left(left)
		}
	}
	return summary
}

func machineUptime(machine *store.Machine, id model.ID, events []model.Event, now time.Time) time.Duration {
	if machine == nil {
		return 0
	}
	if machine.Stats != nil && machine.Stats.UptimeS != nil && *machine.Stats.UptimeS > 0 {
		return time.Duration(*machine.Stats.UptimeS) * time.Second
	}
	if at, ok := transitionAt(events, id, "Started "+machine.Name); ok {
		return max(time.Duration(0), now.Sub(at))
	}
	return 0
}

func transitionAt(events []model.Event, id model.ID, text string) (time.Time, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Target == id && events[i].Text == text {
			return events[i].At, true
		}
	}
	return time.Time{}, false
}
func startedNote(resource model.Resource, events []model.Event, now time.Time) string {
	if resource.State != model.StateRunning && resource.State != model.StatePaused && resource.State != model.StateStarting {
		return ""
	}
	if at, ok := transitionAt(events, resource.ID, "Started "+resource.Name); ok {
		return "◷ Started " + format.Ago(at, now) + " from Ryoport."
	}
	return ""
}

func hardwarePhrase(machine *store.Machine) string {
	if machine == nil {
		return ""
	}
	cores := strings.TrimSpace(machine.Cores)
	ram := displaySize(machine.RAM)
	parts := make([]string, 0, 2)
	if cores != "" && cores != "auto" {
		parts = append(parts, cores+" "+pluralString(cores, "core", "cores"))
	}
	if ram != "" && ram != "auto" {
		parts = append(parts, ram)
	}
	return strings.Join(parts, ", ")
}

func displaySize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "auto") {
		return value
	}
	upper := strings.ToUpper(value)
	for _, suffix := range []string{"GIB", "GB", "G", "MIB", "MB", "M", "TIB", "TB", "T"} {
		if !strings.HasSuffix(upper, suffix) {
			continue
		}
		number := strings.TrimSpace(value[:len(value)-len(suffix)])
		unit := suffix
		switch suffix {
		case "GIB", "GB", "G":
			unit = "GB"
		case "MIB", "MB", "M":
			unit = "MB"
		case "TIB", "TB", "T":
			unit = "TB"
		}
		return number + " " + unit
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value + " MB"
	}
	return value
}

func pluralString(value, one, many string) string {
	if value == "1" {
		return one
	}
	return many
}

func (v *View) layoutOverview(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	if machine == nil {
		return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Machine details are still arriving.")(gtx)
	}
	if resource.State != model.StateRunning && resource.State != model.StatePaused {
		return v.layoutStoppedOverview(gtx, e, resource, machine)
	}

	v.net.update(machine, e.Snap.At)
	return layoutOverviewColumns(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return v.layoutReadings(gtx, e, resource, machine)
		},
		func(gtx layout.Context) layout.Dimensions {
			return v.layoutOverviewSharing(gtx, e, resource, machine)
		},
	)
}

func (v *View) layoutReadings(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	cpu := seriesValues(e.Snap.Series(resource.ID, model.MetricCPU))
	memory := lastValue(e.Snap.Series(resource.ID, model.MetricMem))
	memoryTotal := sizeBytes(machine.RAM)
	memoryUsed := int64(float64(memoryTotal) * memory)
	diskTotal := sizeBytes(machine.Disk)
	diskFrac := 0.0
	if diskTotal > 0 {
		diskFrac = float64(machine.DiskUsed) / float64(diskTotal)
	}
	network := format.Rate(v.net.inbound) + " in, " + format.Rate(v.net.outbound) + " out"
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(section(e, "CPU over the last 10 minutes", percentReading(cpu), true, func(gtx layout.Context) layout.Dimensions {
			return chartWithGrid(gtx, e, cpu)
		})),
		layout.Rigid(section(e, "Memory", format.Bytes(memoryUsed)+" of "+displaySize(machine.RAM), false, func(gtx layout.Context) layout.Dimensions {
			return kit.BladeMeter(gtx, e.Th, memory, e.Th.R.Fg2)
		})),
		layout.Rigid(section(e, "Disk", format.Bytes(machine.DiskUsed)+" of "+displaySize(machine.Disk), false, func(gtx layout.Context) layout.Dimensions {
			return kit.BladeMeter(gtx, e.Th, diskFrac, e.Th.R.Fg2)
		})),
		layout.Rigid(section(e, "Network", network, false, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))}
		})),
	)
}

func (v *View) layoutStoppedOverview(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	v.overviewSnapshots.Small = true
	if v.overviewSnapshots.Clicked(gtx) {
		e.Nav.SetTab(model.KindMachine, "Snapshots")
	}
	startDisplay := displayName(string(store.StartDisplay(e.Snap)))
	configuredDisplay := displayName(e.Snap.Settings.Defaults.Display)
	intro := "Start opens this machine in a window."
	switch startDisplay {
	case "SPICE":
		intro = "Start opens this machine with SPICE."
	case "Headless":
		intro = "Start runs this machine without a display."
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, intro)),
		layout.Rigid(layout.Spacer{Height: 21}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutOverviewColumns(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return v.layoutStoppedReadings(gtx, e, resource, machine, configuredDisplay, startDisplay)
				},
				func(gtx layout.Context) layout.Dimensions {
					return v.layoutOverviewSharing(gtx, e, resource, machine)
				},
			)
		}),
	)
}

func layoutOverviewColumns(gtx layout.Context, left, right layout.Widget) layout.Dimensions {
	if gtx.Constraints.Max.X < gtx.Dp(660) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(left),
			layout.Rigid(layout.Spacer{Height: 21}.Layout),
			layout.Rigid(right),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(34)}.Layout(gtx,
		layout.Flexed(1.1, left),
		layout.Flexed(.9, right),
	)
}

func (v *View) layoutStoppedReadings(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine, display, startDisplay string) layout.Dimensions {
	diskReading, diskDetail := stoppedDiskFact(machine)
	snapshots := userSnapshots(machine.Snapshots)
	snapshotReading := fmt.Sprintf("%d %s", len(snapshots), plural(len(snapshots), "snapshot", "snapshots"))
	snapshotDetail := "No snapshots yet."
	if len(snapshots) > 0 {
		snapshotDetail = "Newest: " + snapshots[0].Name
		if at, err := time.Parse(time.RFC3339, snapshots[0].Date); err == nil {
			snapshotDetail += ", " + format.Ago(at, e.Now)
		}
		snapshotDetail += "."
	}
	bootDetail := "Boots from its virtual disk."
	if !machine.Installed {
		bootDetail = "Uses its configured install media before the virtual disk is created."
	}
	if display != startDisplay {
		bootDetail = display + " is configured, but this host will use a window. " + bootDetail
	}
	snapshotBody := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, snapshotDetail)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.overviewSnapshots.Layout(gtx, e.Th, kit.Quiet, icons.Backup, "Open snapshots")
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(section(e, "Last run", lastRunReading(e.Snap.Events, resource, e.Now), false, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "It is not using memory or processor time now."))),
		layout.Rigid(section(e, "Disk", diskReading, false, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, diskDetail))),
		layout.Rigid(section(e, "Snapshots", snapshotReading, false, snapshotBody)),
		layout.Rigid(section(e, "Starts with", display, false, kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, bootDetail))),
	)
}

func lastRunReading(events []model.Event, resource model.Resource, now time.Time) string {
	var latest time.Time
	started := "Started " + resource.Name
	stopped := "Stopped " + resource.Name
	for _, event := range events {
		if event.Target != resource.ID || (event.Text != started && event.Text != stopped) {
			continue
		}
		if latest.IsZero() || event.At.After(latest) {
			latest = event.At
		}
	}
	if latest.IsZero() {
		return "Never started"
	}
	return format.Ago(latest, now)
}

func stoppedDiskFact(machine *store.Machine) (string, string) {
	capacity := displaySize(machine.Disk)
	if !machine.Installed {
		if capacity == "" {
			return "Not created", "No disk image exists on this computer."
		}
		return "Not created", capacity + " is configured for the virtual disk."
	}
	used := format.Bytes(max(int64(0), machine.DiskUsed))
	if capacity == "" {
		return used, "Space used by this machine on the computer."
	}
	return used + " of " + capacity, "Machine folder space used on the host against virtual disk capacity."
}

func section(e *env.Env, title, reading string, prominent bool, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		trailing := layout.Widget(nil)
		if reading != "" {
			style := e.Th.T.Mono
			if prominent {
				style = e.Th.T.Sentence
				style.Face = e.Th.T.Mono.Face
			}
			trailing = kit.Text(e.Th, style, e.Th.R.Fg, reading)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(common.SectionTitle(e.Th, title, trailing)),
			layout.Rigid(body),
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
		)
	}
}

type networkState struct {
	at                time.Time
	rx, tx            uint64
	inbound, outbound float64
	initialized       bool
}

func (n *networkState) reset() { *n = networkState{} }

func (n *networkState) update(machine *store.Machine, at time.Time) {
	if machine == nil || machine.Stats == nil || machine.Stats.NetRXBytes == nil || machine.Stats.NetTXBytes == nil || at.IsZero() {
		return
	}
	rx, tx := *machine.Stats.NetRXBytes, *machine.Stats.NetTXBytes
	if !n.initialized {
		n.at, n.rx, n.tx, n.initialized = at, rx, tx, true
		return
	}
	if !at.After(n.at) || (rx == n.rx && tx == n.tx) {
		return
	}
	if rx < n.rx || tx < n.tx {
		n.at, n.rx, n.tx = at, rx, tx
		n.inbound, n.outbound = 0, 0
		return
	}
	seconds := at.Sub(n.at).Seconds()
	n.inbound = float64(rx-n.rx) / seconds
	n.outbound = float64(tx-n.tx) / seconds
	n.at, n.rx, n.tx = at, rx, tx
}

func chartWithGrid(gtx layout.Context, e *env.Env, values []float64) layout.Dimensions {
	chart := func(gtx layout.Context) layout.Dimensions {
		return kit.EngravedChart(gtx, e.Th, values, 0, 1, 138, []string{"10 min ago", "5 min", "now"})
	}
	grid := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		plotHeight := min(size.Y, gtx.Dp(138))
		dot := max(1, gtx.Dp(1))
		for row := 1; row <= 3; row++ {
			y := row * plotHeight / 4
			for x := 0; x < size.X; x += gtx.Dp(8) {
				paint.FillShape(gtx.Ops, e.Th.R.Vein, clip.Rect(image.Rect(x, y, min(size.X, x+gtx.Dp(2)), y+dot)).Op())
			}
		}
		return layout.Dimensions{Size: size}
	}
	return layout.Stack{}.Layout(gtx, layout.Expanded(grid), layout.Stacked(chart))
}

func seriesValues(series *model.Series) []float64 {
	if series == nil {
		return nil
	}
	return series.Values()
}

func percentReading(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	return format.Percent(values[len(values)-1])
}

func lastValue(series *model.Series) float64 {
	if series == nil {
		return 0
	}
	value, _, ok := series.Last()
	if !ok {
		return 0
	}
	return value
}

func guestIP(machine *store.Machine) string {
	if machine != nil && machine.Stats != nil {
		return machine.Stats.GuestIP
	}
	return ""
}

func guestOS(e *env.Env, machine *store.Machine) string {
	if machine == nil {
		return ""
	}
	slug := machine.OS
	if slug == "" && machine.Stats != nil {
		slug = machine.Stats.GuestOS
	}
	for _, system := range e.Snap.Catalog.OS {
		if !strings.EqualFold(system.OS, slug) {
			continue
		}
		for _, release := range system.Releases {
			if strings.Contains(strings.ToLower(machine.Name), strings.ToLower(release)) {
				return strings.TrimSpace(system.Name + " " + release)
			}
		}
		if system.Name != "" {
			return system.Name
		}
		break
	}
	if machine.Stats != nil && machine.Stats.GuestOS != "" {
		return machine.Stats.GuestOS
	}
	return slug
}

func uptimeValue(machine *store.Machine) string {
	if machine != nil && machine.Stats != nil && machine.Stats.UptimeS != nil {
		return format.Duration(time.Duration(*machine.Stats.UptimeS) * time.Second)
	}
	return ""
}

func displayName(display string) string {
	switch strings.ToLower(display) {
	case "spice":
		return "SPICE"
	case "none", "headless":
		return "Headless"
	default:
		return "Window"
	}
}

func sizeBytes(value string) int64 {
	value = strings.TrimSpace(strings.ToUpper(value))
	multipliers := []struct {
		suffix string
		value  float64
	}{
		{"TIB", 1 << 40}, {"TB", 1 << 40}, {"T", 1 << 40},
		{"GIB", 1 << 30}, {"GB", 1 << 30}, {"G", 1 << 30},
		{"MIB", 1 << 20}, {"MB", 1 << 20}, {"M", 1 << 20},
	}
	for _, multiplier := range multipliers {
		if strings.HasSuffix(value, multiplier.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, multiplier.suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0
			}
			return int64(parsed * multiplier.value)
		}
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed * (1 << 20)
}
