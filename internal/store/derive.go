package store

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

func deriveResources(snap *Snapshot, starting map[string]bool) []model.Resource {
	resources := make([]model.Resource, 0, len(snap.Machines)+len(snap.Passthrough)+len(snap.Servers)+len(snap.Consoles)+len(snap.Jobs))
	for name, machine := range snap.Machines {
		id := model.NewID(model.KindMachine, name)
		state := machineState(machine.Machine, starting[name], hasCreateJob(snap.Jobs, id))
		resources = append(resources, model.Resource{
			ID: id, Kind: model.KindMachine, Name: name, State: state,
			Detail: machineDetail(machine.Machine), Icon: machine.OS,
			Pinned: isPinned(snap.Settings.Pinned, id), Primary: machinePrimary(state), Updated: snap.At,
			Instant: machine.Instant, Disposable: machine.Disposable, Sealed: machine.Sealed,
		})
	}
	for _, job := range snap.Jobs {
		if job.State != model.JobRunning || job.Target.Kind() != model.KindMachine || !strings.HasPrefix(job.Title, "Create ") {
			continue
		}
		name := job.Target.Key()
		if name == "" || snap.Machines[name] != nil {
			continue
		}
		resources = append(resources, model.Resource{
			ID: job.Target, Kind: model.KindMachine, Name: name, State: model.StateCreating,
			Detail: job.Phase, Pinned: isPinned(snap.Settings.Pinned, job.Target), Updated: snap.At,
		})
	}
	for name, machine := range snap.Passthrough {
		id := model.NewID(model.KindPassthrough, name)
		state := passthroughState(machine.State)
		resources = append(resources, model.Resource{
			ID: id, Kind: model.KindPassthrough, Name: name, State: state,
			Detail: fmt.Sprintf("%d cores, %s", machine.VCPUs, formatRAM(machine.RAMMB)), Icon: machine.OS,
			Pinned: isPinned(snap.Settings.Pinned, id), Primary: passthroughPrimary(state), Updated: snap.At,
		})
	}
	for alias, server := range snap.Servers {
		id := model.NewID(model.KindServer, alias)
		state := serverState(server.Reach, server.Probe)
		icon := "server"
		if server.Probe != nil {
			if slug := build.DistroSlug(server.Probe.Distro); slug != "" {
				icon = slug
			}
		}
		resources = append(resources, model.Resource{
			ID: id, Kind: model.KindServer, Name: alias, State: state,
			Detail: serverDetail(server.Host), Icon: icon, Group: server.Group,
			Tags: append([]string(nil), server.Tags...), Pinned: server.Pinned,
			Primary: serverPrimary(state), Updated: snap.At,
		})
	}
	for idKey, profile := range snap.Consoles {
		id := model.NewID(model.KindConsole, idKey)
		state := consoleState(profile.Reach)
		resources = append(resources, model.Resource{
			ID: id, Kind: model.KindConsole, Name: profile.Name, State: state,
			Detail: consoleDetail(profile), Icon: profile.Kind,
			Pinned: isPinned(snap.Settings.Pinned, id), Primary: consolePrimary(state), Updated: snap.At,
		})
	}
	sortResources(resources)
	return resources
}

func machineState(machine vm.Machine, starting, creating bool) model.State {
	switch {
	case creating:
		return model.StateCreating
	case starting:
		return model.StateStarting
	case !machine.Installed:
		return model.StateAbsent
	case machine.Running:
		return model.StateRunning
	default:
		return model.StateStopped
	}
}

func machinePrimary(state model.State) model.Verb {
	switch state {
	case model.StateRunning:
		return model.VerbOpen
	case model.StateStopped, model.StateAbsent:
		return model.VerbStart
	default:
		return model.VerbNone
	}
}

func passthroughState(state string) model.State {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return model.StateRunning
	case "paused":
		return model.StatePaused
	case "starting":
		return model.StateStarting
	case "stopping", "shutdown", "shutting down":
		return model.StateStopping
	case "error", "failed":
		return model.StateError
	default:
		return model.StateStopped
	}
}

func passthroughPrimary(state model.State) model.Verb {
	if state == model.StateRunning || state == model.StatePaused {
		return model.VerbOpen
	}
	if state == model.StateStopped || state == model.StateError {
		return model.VerbStart
	}
	return model.VerbNone
}

func serverState(reach *remote.Reach, probe *remote.Probe) model.State {
	if probe != nil && probe.NeedAuth {
		return model.StateNeedsAuth
	}
	if reach == nil {
		return model.StateUnknown
	}
	if !reach.Up || !reach.SSHUp {
		return model.StateOffline
	}
	if probe != nil && probe.FailedUnits > 0 {
		return model.StateDegraded
	}
	return model.StateOnline
}

func serverPrimary(state model.State) model.Verb {
	if state == model.StateNeedsAuth {
		return model.VerbSetPass
	}
	return model.VerbConnect
}

// consolePrimary offers Connect only when the console can answer; an unplugged
// serial adapter or an unreachable host gets no action until it comes back.
func consolePrimary(state model.State) model.Verb {
	if state == model.StateOffline {
		return model.VerbNone
	}
	return model.VerbConnect
}

func consoleState(reach *console.Reach) model.State {
	if reach == nil {
		return model.StateUnknown
	}
	if reach.Up {
		return model.StateOnline
	}
	return model.StateOffline
}

func machineDetail(machine vm.Machine) string {
	cores := machine.Cores
	if cores == "" {
		cores = "auto"
	}
	return fmt.Sprintf("%s cores, %s", cores, formatRAMString(machine.RAM))
}

func formatRAMString(value string) string {
	text := strings.TrimSpace(strings.ToUpper(value))
	if text == "" || text == "AUTO" {
		return "auto memory"
	}
	if strings.HasSuffix(text, "G") {
		return strings.TrimSuffix(text, "G") + " GB"
	}
	if strings.HasSuffix(text, "M") {
		mb, err := strconv.Atoi(strings.TrimSuffix(text, "M"))
		if err == nil {
			return formatRAM(mb)
		}
	}
	if mb, err := strconv.Atoi(text); err == nil {
		return formatRAM(mb)
	}
	return value
}

func formatRAM(mb int) string {
	if mb > 0 && mb%1024 == 0 {
		return fmt.Sprintf("%d GB", mb/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

func serverDetail(host remote.Host) string {
	endpoint := host.HostName
	if endpoint == "" {
		endpoint = host.Alias
	}
	if host.User != "" {
		endpoint = host.User + "@" + endpoint
	}
	if host.Port != 0 && host.Port != 22 {
		endpoint += ":" + strconv.Itoa(host.Port)
	}
	return endpoint
}

func consoleDetail(profile *Console) string {
	if profile.Device != "" {
		return profile.Device
	}
	return profile.Address
}

func isPinned(pinned []string, id model.ID) bool {
	for _, value := range pinned {
		if value == string(id) {
			return true
		}
	}
	return false
}

func sortResources(resources []model.Resource) {
	rank := map[model.Kind]int{
		model.KindMachine: 0, model.KindPassthrough: 1, model.KindServer: 2, model.KindConsole: 3,
	}
	sort.SliceStable(resources, func(i, j int) bool {
		left, right := resources[i], resources[j]
		if rank[left.Kind] != rank[right.Kind] {
			return rank[left.Kind] < rank[right.Kind]
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
}

func hasCreateJob(jobs []model.Job, id model.ID) bool {
	for _, job := range jobs {
		if job.Target == id && job.State == model.JobRunning && strings.HasPrefix(job.Title, "Create ") {
			return true
		}
	}
	return false
}

func deriveAlerts(snap *Snapshot) []model.Alert {
	var alerts []model.Alert
	hostDetected := len(snap.Host.Setup.Argv) > 0 || snap.Host.Setup.Hint != ""
	if hostDetected && (!snap.Host.Engine.Quickemu || !snap.Host.Engine.Quickget || !snap.Host.Engine.QEMU) {
		alerts = append(alerts, model.Alert{ID: "engine-missing", Severity: model.SeverityBad, Title: "VM engine is missing", Body: snap.Host.Setup.Hint, Fix: model.FixInstallEngine})
	} else if hostDetected && !snap.Host.Engine.KVM {
		alerts = append(alerts, model.Alert{ID: "kvm-unavailable", Severity: model.SeverityBad, Title: "KVM is unavailable", Body: "Enable hardware virtualisation and grant access to /dev/kvm.", Fix: model.FixEnableKVM})
	}
	for alias, server := range snap.Servers {
		if serverState(server.Reach, server.Probe) == model.StateNeedsAuth {
			fix := model.FixDeployKey
			if snap.Host.Keyring {
				fix = model.FixSavePassword
			}
			alerts = append(alerts, model.Alert{ID: "server-auth:" + alias, Target: model.NewID(model.KindServer, alias), Severity: model.SeverityWarn, Title: alias + " needs a password", Body: "Save a password or deploy a key.", Fix: fix})
		}
	}
	if snap.Host.Passthrough && snap.Loaded[SourcePassthrough] && !snap.GPU.Ready {
		body := snap.GPU.Blocker
		if body == "" {
			body = "GPU passthrough is not ready."
		}
		alerts = append(alerts, model.Alert{ID: "passthrough-not-ready", Target: model.NewID(model.KindPassthrough, ""), Severity: model.SeverityWarn, Title: "GPU passthrough needs attention", Body: body, Fix: model.FixOpenHubGPU})
	}
	for _, job := range snap.Jobs {
		if job.State == model.JobFailed {
			alerts = append(alerts, model.Alert{ID: "job:" + job.ID, Target: job.Target, Severity: model.SeverityBad, Title: job.Title + " failed", Body: job.Err})
		}
	}
	if snap.Update.Available {
		alerts = append(alerts, model.Alert{ID: "update-available", Severity: model.SeverityWarn, Title: "Ryoport " + snap.Update.Latest + " is available", Body: "Open settings to review the update.", Fix: model.FixOpenSettings})
	}
	return alerts
}

func firstLine(err error) string {
	if err == nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	return line
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func rate(now time.Time, current uint64, previous counterSample, old uint64) float64 {
	seconds := now.Sub(previous.at).Seconds()
	if previous.at.IsZero() || seconds <= 0 || current < old {
		return 0
	}
	return float64(current-old) / seconds
}
