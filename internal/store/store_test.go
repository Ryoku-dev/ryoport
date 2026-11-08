package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

func TestIntervalTable(t *testing.T) {
	focused := Focus{Visible: true, Focused: true}
	visible := Focus{Visible: true}
	hidden := Focus{}
	tests := []struct {
		name     string
		source   pollSource
		focus    Focus
		selected bool
		want     time.Duration
	}{
		{"machine list focused", pollMachineList, focused, false, 10 * time.Second},
		{"machine list visible", pollMachineList, visible, false, 10 * time.Second},
		{"machine list hidden", pollMachineList, hidden, false, 60 * time.Second},
		{"selected machine focused", pollMachineStats, focused, true, 2 * time.Second},
		{"selected machine visible", pollMachineStats, visible, true, 5 * time.Second},
		{"machine hidden", pollMachineStats, hidden, true, 0},
		{"sparkline focused", pollSparklines, focused, false, 5 * time.Second},
		{"sparkline visible", pollSparklines, visible, false, 10 * time.Second},
		{"sparkline hidden", pollSparklines, hidden, false, 0},
		{"reach focused", pollServerReach, focused, false, 15 * time.Second},
		{"reach visible", pollServerReach, visible, false, 30 * time.Second},
		{"reach hidden", pollServerReach, hidden, false, 0},
		{"probe focused", pollServerProbe, focused, false, 30 * time.Second},
		{"selected probe focused", pollServerProbe, focused, true, 10 * time.Second},
		{"probe visible", pollServerProbe, visible, true, 60 * time.Second},
		{"probe hidden", pollServerProbe, hidden, true, 0},
		{"apps focused", pollWebApps, focused, false, 60 * time.Second},
		{"apps visible", pollWebApps, visible, false, 0},
		{"passthrough focused", pollPassthrough, focused, false, 10 * time.Second},
		{"passthrough visible", pollPassthrough, visible, false, 30 * time.Second},
		{"passthrough hidden", pollPassthrough, hidden, false, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := interval(test.source, test.focus, test.selected, false); got != test.want {
				t.Fatalf("interval = %v, want %v", got, test.want)
			}
			batteryWant := test.want
			if batteryWant > 0 {
				batteryWant *= 2
			}
			if got := interval(test.source, test.focus, test.selected, true); got != batteryWant {
				t.Fatalf("battery interval = %v, want %v", got, batteryWant)
			}
		})
	}
}

func TestServerState(t *testing.T) {
	tests := []struct {
		name  string
		reach *remote.Reach
		probe *remote.Probe
		want  model.State
	}{
		{"unknown", nil, nil, model.StateUnknown},
		{"offline ping", &remote.Reach{}, nil, model.StateOffline},
		{"offline ssh", &remote.Reach{Up: true}, nil, model.StateOffline},
		{"online", &remote.Reach{Up: true, SSHUp: true}, &remote.Probe{OK: true}, model.StateOnline},
		{"needs auth", &remote.Reach{}, &remote.Probe{NeedAuth: true}, model.StateNeedsAuth},
		{"degraded", &remote.Reach{Up: true, SSHUp: true}, &remote.Probe{OK: true, FailedUnits: 2}, model.StateDegraded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := serverState(test.reach, test.probe); got != test.want {
				t.Fatalf("state = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPrimaryVerbChoice(t *testing.T) {
	if got := machinePrimary(model.StateStopped); got != model.VerbStart {
		t.Fatalf("stopped machine verb = %q", got)
	}
	if got := machinePrimary(model.StateRunning); got != model.VerbOpen {
		t.Fatalf("running machine verb = %q", got)
	}
	if got := passthroughPrimary(model.StateRunning); got != model.VerbOpen {
		t.Fatalf("running passthrough verb = %q", got)
	}
	if got := serverPrimary(model.StateNeedsAuth); got != model.VerbSetPass {
		t.Fatalf("auth server verb = %q", got)
	}
	if got := serverPrimary(model.StateOffline); got != model.VerbConnect {
		t.Fatalf("offline server verb = %q", got)
	}
}

func TestResourceSortingAndPinnedRouting(t *testing.T) {
	resources := []model.Resource{
		{Kind: model.KindConsole, Name: "zeta"},
		{Kind: model.KindServer, Name: "beta"},
		{Kind: model.KindMachine, Name: "Zulu"},
		{Kind: model.KindPassthrough, Name: "win"},
		{Kind: model.KindMachine, Name: "alpha"},
		{Kind: model.KindServer, Name: "Alpha"},
	}
	sortResources(resources)
	want := []string{"alpha", "Zulu", "win", "Alpha", "beta", "zeta"}
	for i, name := range want {
		if resources[i].Name != name {
			t.Fatalf("resource %d = %q, want %q", i, resources[i].Name, name)
		}
	}
	if got := pinnedOwner(model.NewID(model.KindServer, "ally")); got != "remote" {
		t.Fatalf("server pinned owner = %q", got)
	}
	for _, kind := range []model.Kind{model.KindMachine, model.KindPassthrough, model.KindConsole} {
		if got := pinnedOwner(model.NewID(kind, "x")); got != "settings" {
			t.Fatalf("%s pinned owner = %q", kind, got)
		}
	}
}

func TestAlertDerivation(t *testing.T) {
	snap := &Snapshot{
		Host: Host{
			Engine:  vm.Caps{Quickemu: true, Quickget: true, QEMU: true, KVM: true, Ready: true},
			Setup:   vm.SetupAdvice{Hint: "Install the VM engine."},
			Keyring: true, Passthrough: true,
		},
		Servers: map[string]*Server{
			"ally": {Host: remote.Host{Alias: "ally"}, Probe: &remote.Probe{NeedAuth: true}},
		},
		GPU:    passthrough.Readiness{Blocker: "IOMMU is disabled"},
		Jobs:   []model.Job{{ID: "job-1", Title: "Create lab", State: model.JobFailed, Err: "download failed"}},
		Update: update.Info{Available: true, Latest: "1.2.3"},
		Loaded: map[Source]bool{SourcePassthrough: true},
	}
	alerts := deriveAlerts(snap)
	fixes := make(map[model.Fix]bool)
	for _, alert := range alerts {
		fixes[alert.Fix] = true
	}
	for _, fix := range []model.Fix{model.FixSavePassword, model.FixOpenHubGPU, model.FixOpenSettings} {
		if !fixes[fix] {
			t.Fatalf("missing alert fix %q in %#v", fix, alerts)
		}
	}
	failed := false
	for _, alert := range alerts {
		if alert.ID == "job:job-1" && alert.Body == "download failed" {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("missing failed job alert in %#v", alerts)
	}

	snap.Host.Engine.KVM = false
	alerts = deriveAlerts(snap)
	if !hasFix(alerts, model.FixEnableKVM) {
		t.Fatalf("missing KVM alert in %#v", alerts)
	}
	snap.Host.Engine.Quickemu = false
	alerts = deriveAlerts(snap)
	if !hasFix(alerts, model.FixInstallEngine) {
		t.Fatalf("missing engine alert in %#v", alerts)
	}
}

func hasFix(alerts []model.Alert, fix model.Fix) bool {
	for _, alert := range alerts {
		if alert.Fix == fix {
			return true
		}
	}
	return false
}

func TestJobMappingFromBuildEvents(t *testing.T) {
	job := model.Job{Progress: -1}
	applyBuildEvent(&job, build.Event{Phase: build.PhaseResolve})
	if job.Phase != "Resolving" {
		t.Fatalf("resolve phase = %q", job.Phase)
	}
	applyBuildEvent(&job, build.Event{Phase: build.PhaseDownload, Progress: &fetch.Progress{Recv: 25, Total: 100, BPS: 12.5}})
	if job.Phase != "Downloading" || job.Progress != 0.25 || job.Done != 25 || job.Total != 100 || job.Rate != 12.5 {
		t.Fatalf("download mapping = %#v", job)
	}
	applyBuildEvent(&job, build.Event{Phase: build.PhaseConfig})
	if job.Phase != "Writing config" {
		t.Fatalf("config phase = %q", job.Phase)
	}
	unknown := model.Job{Progress: -1}
	applyBuildEvent(&unknown, build.Event{Phase: build.PhaseLog, Line: "Preparing disk"})
	if unknown.Phase != "Preparing disk" {
		t.Fatalf("log phase = %q", unknown.Phase)
	}
}

func TestUndoWindow(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	host := remote.HostInput{Alias: "ally"}
	entries := map[string]undoEntry{"valid": {expires: now.Add(10 * time.Second), host: &host}}
	entry, ok := takeUndo(entries, "valid", now.Add(9*time.Second))
	if !ok || entry.host == nil || entry.host.Alias != "ally" {
		t.Fatalf("valid undo = %#v, %v", entry, ok)
	}
	if _, ok := entries["valid"]; ok {
		t.Fatal("used undo token remained")
	}
	entries["expired"] = undoEntry{expires: now.Add(10 * time.Second), host: &host}
	if _, ok := takeUndo(entries, "expired", now.Add(10*time.Second)); ok {
		t.Fatal("expired undo token was accepted")
	}
	if _, ok := entries["expired"]; ok {
		t.Fatal("expired undo token remained")
	}
}

func TestNoticeTrimming(t *testing.T) {
	s := New(Options{Now: func() time.Time { return time.Unix(1, 0) }})
	for i := range 5 {
		s.pushNoticeLocked(model.EventInfo, string(rune('a'+i)), "", "")
	}
	if len(s.notices) != 3 {
		t.Fatalf("notice count = %d", len(s.notices))
	}
	if s.notices[0].Text != "c" || s.notices[2].Text != "e" {
		t.Fatalf("notices = %#v", s.notices)
	}
}

func TestBatteryDetection(t *testing.T) {
	root := t.TempDir()
	oldRoot := powerSupplyRoot
	powerSupplyRoot = root
	t.Cleanup(func() { powerSupplyRoot = oldRoot })
	writeSupply := func(name, kind, online string) {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "type"), []byte(kind+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if online != "" {
			if err := os.WriteFile(filepath.Join(dir, "online"), []byte(online+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeSupply("BAT0", "Battery", "")
	if !onBattery(powerSupplyRoot) {
		t.Fatal("battery without mains was not detected")
	}
	writeSupply("AC", "Mains", "1")
	if onBattery(powerSupplyRoot) {
		t.Fatal("online mains still reported battery")
	}
	if err := os.WriteFile(filepath.Join(root, "AC", "online"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !onBattery(powerSupplyRoot) {
		t.Fatal("offline mains did not report battery")
	}
	if onBattery(filepath.Join(root, "missing")) {
		t.Fatal("missing sysfs root reported battery")
	}
}

func TestPublishedSnapshotsAreImmutable(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := New(Options{Now: func() time.Time { return now }})
	before := s.Snapshot()
	s.mu.Lock()
	s.settings.Pinned = append(s.settings.Pinned, "machine:lab")
	s.machines["lab"] = &Machine{Detail: vm.Detail{Machine: vm.Machine{Name: "lab", Installed: true}}}
	s.metrics[model.NewID(model.KindMachine, "lab")] = map[model.Metric]*model.Series{model.MetricCPU: model.NewSeries(150)}
	s.metrics[model.NewID(model.KindMachine, "lab")][model.MetricCPU].Push(0.5, now)
	s.publishLocked()
	s.mu.Unlock()
	after := s.Snapshot()
	if len(before.Settings.Pinned) != 0 || len(before.Machines) != 0 || len(before.Metrics) != 0 {
		t.Fatalf("old snapshot mutated: %#v", before)
	}
	series := after.Series(model.NewID(model.KindMachine, "lab"), model.MetricCPU)
	if series == nil || series.Len() != 1 {
		t.Fatalf("new snapshot series = %#v", series)
	}
	s.mu.Lock()
	s.pushNoticeLocked(model.EventInfo, "Unrelated change", "", "")
	s.publishLocked()
	s.mu.Unlock()
	if got := s.Snapshot().Machines["lab"]; got != after.Machines["lab"] {
		t.Fatal("unchanged machine was not shared between snapshots")
	}
	s.mu.Lock()
	s.metrics[model.NewID(model.KindMachine, "lab")][model.MetricCPU].Push(0.75, now.Add(time.Second))
	s.mu.Unlock()
	if series.Len() != 1 {
		t.Fatalf("published series mutated, len = %d", series.Len())
	}
}

func TestMachineResourcePinnedFromSettings(t *testing.T) {
	snap := &Snapshot{
		At:          time.Unix(1, 0),
		Settings:    config.Settings{Pinned: []string{"machine:lab"}},
		Machines:    map[string]*Machine{"lab": {Detail: vm.Detail{Machine: vm.Machine{Name: "lab", Installed: true}}}},
		Passthrough: map[string]*Passthrough{},
		Servers:     map[string]*Server{},
		Consoles:    map[string]*Console{},
	}
	resources := deriveResources(snap, map[string]bool{})
	if len(resources) != 1 || !resources[0].Pinned {
		t.Fatalf("resources = %#v", resources)
	}
}
