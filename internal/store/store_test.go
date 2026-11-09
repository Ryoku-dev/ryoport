package store

import (
	"context"
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
	"github.com/ryoku-dev/ryoport/internal/xdg"
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

func TestHostStatsIntervalOnlyRunsForVisibleBoard(t *testing.T) {
	board := Focus{Visible: true, Focused: true, Page: PageFleet}
	if got := interval(pollHostStats, board, false, false); got != 3*time.Second {
		t.Fatalf("focused board interval = %v, want 3s", got)
	}
	if got := interval(pollHostStats, board, false, true); got != 6*time.Second {
		t.Fatalf("focused board on battery = %v, want 6s", got)
	}
	board.Focused = false
	if got := interval(pollHostStats, board, false, false); got != 15*time.Second {
		t.Fatalf("visible board interval = %v, want 15s", got)
	}
	board.Selected = model.NewID(model.KindMachine, "lab")
	if got := interval(pollHostStats, board, false, false); got != 0 {
		t.Fatalf("selected resource interval = %v, want off", got)
	}
	board.Selected = ""
	board.Page = PageActivity
	if got := interval(pollHostStats, board, false, false); got != 0 {
		t.Fatalf("activity interval = %v, want off", got)
	}
	board.Page = PageFleet
	board.Visible = false
	if got := interval(pollHostStats, board, false, false); got != 0 {
		t.Fatalf("hidden board interval = %v, want off", got)
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

func TestRefocusPicksUpAnEngineInstalledWhileOpen(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	published := make(chan struct{}, 1)
	s := New(Options{
		Now: func() time.Time { return time.Unix(1, 0) },
		Invalidate: func() {
			select {
			case published <- struct{}{}:
			default:
			}
		},
	})
	s.mu.Lock()
	s.ctx = context.Background()
	s.host.Setup = vm.SetupAdvice{Hint: "Install quickemu."}
	s.host.Engine = s.o.VM.Caps(s.ctx)
	s.publishLocked()
	s.mu.Unlock()
	if !hasFix(s.Snapshot().Alerts, model.FixInstallEngine) {
		t.Fatal("no engine alert before the install")
	}

	for _, name := range []string{"quickemu", "quickget", "qemu-system-x86_64"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho 4.9.9\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s.SetFocus(Focus{Visible: true})
	s.SetFocus(Focus{Visible: true, Focused: true})
	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("refocusing the window never re-read the engine")
	}
	snap := s.Snapshot()
	if hasFix(snap.Alerts, model.FixInstallEngine) {
		t.Fatalf("engine alert survived the install: %#v", snap.Alerts)
	}
	if snap.Host.Engine.Version != "4.9.9" {
		t.Fatalf("engine version = %q, want 4.9.9", snap.Host.Engine.Version)
	}
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
	applyBuildEvent(&job, build.Event{Phase: build.PhaseDownload})
	applyBuildEvent(&job, build.Event{Phase: build.PhaseLog, Line: "- URL: http://builds.kolibrios.org/en_US/latest-iso.7z"})
	if job.Phase != "Downloading" || job.Detail != "From builds.kolibrios.org" {
		t.Fatalf("a tool line must not replace the stage: phase %q, detail %q", job.Phase, job.Detail)
	}
	applyBuildEvent(&job, build.Event{Phase: build.PhaseLog, Line: "   "})
	if job.Detail != "From builds.kolibrios.org" {
		t.Fatalf("a blank line cleared the detail: %q", job.Detail)
	}
	lines := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	for _, line := range lines {
		applyBuildEvent(&job, build.Event{Phase: build.PhaseLog, Line: "- " + line})
	}
	wantLog := lines[len(lines)-8:]
	if len(job.Log) != len(wantLog) {
		t.Fatalf("job log = %#v, want %#v", job.Log, wantLog)
	}
	for i := range wantLog {
		if job.Log[i] != wantLog[i] {
			t.Fatalf("job log = %#v, want %#v", job.Log, wantLog)
		}
	}
	if job.Detail != "ten" {
		t.Fatalf("latest detail = %q, want ten", job.Detail)
	}
}

func TestJobLogIsSharedUntilItChanges(t *testing.T) {
	s := New(Options{Now: func() time.Time { return time.Unix(1, 0) }})
	s.mu.Lock()
	s.jobs["job-1"] = &runningJob{job: model.Job{ID: "job-1", Log: []string{"one"}, State: model.JobRunning}}
	s.publishLocked()
	s.mu.Unlock()
	before := s.Snapshot().Jobs[0].Log

	s.mu.Lock()
	s.pushNoticeLocked(model.EventInfo, "Unrelated", "", "")
	s.publishLocked()
	s.mu.Unlock()
	shared := s.Snapshot().Jobs[0].Log
	if &before[0] != &shared[0] {
		t.Fatal("unchanged job log was cloned")
	}

	s.mu.Lock()
	running := s.jobs["job-1"]
	applyBuildEvent(&running.job, build.Event{Phase: build.PhaseLog, Line: "- two"})
	s.publishLocked()
	s.mu.Unlock()
	after := s.Snapshot().Jobs[0].Log
	if len(before) != 1 || before[0] != "one" {
		t.Fatalf("published job log mutated: %#v", before)
	}
	if len(after) != 2 || after[0] != "one" || after[1] != "two" {
		t.Fatalf("updated job log = %#v", after)
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
	s.pushMetricsLocked(model.NewID(model.KindMachine, "lab"), []metricSample{{metric: model.MetricCPU, value: 0.5}}, now)
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
	s.pushMetricsLocked(model.NewID(model.KindMachine, "lab"), []metricSample{{metric: model.MetricCPU, value: 0.75}}, now.Add(time.Second))
	s.mu.Unlock()
	if series.Len() != 1 {
		t.Fatalf("published series mutated, len = %d", series.Len())
	}
}

func TestIdenticalMachinePollPublishesOnce(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	invalidations := 0
	s := New(Options{
		Now:        func() time.Time { return now },
		Invalidate: func() { invalidations++ },
	})
	result := map[string]*Machine{
		"lab": &Machine{Detail: vm.Detail{Machine: vm.Machine{Name: "lab", Installed: true}}},
	}
	if !s.applyMachines(result) {
		t.Fatal("first machine poll was ignored")
	}
	firstRev := s.Snapshot().Rev
	if invalidations != 1 {
		t.Fatalf("first invalidations = %d, want 1", invalidations)
	}
	again := map[string]*Machine{
		"lab": &Machine{Detail: vm.Detail{Machine: vm.Machine{Name: "lab", Installed: true}}},
	}
	if s.applyMachines(again) {
		t.Fatal("identical machine poll published")
	}
	if got := s.Snapshot().Rev; got != firstRev {
		t.Fatalf("revision changed from %d to %d", firstRev, got)
	}
	if invalidations != 1 {
		t.Fatalf("invalidations = %d, want 1", invalidations)
	}
}

func TestMachineResourcePinnedFromSettings(t *testing.T) {
	snap := &Snapshot{
		At:          time.Unix(1, 0),
		Settings:    config.Settings{Pinned: []string{"machine:lab"}},
		Machines:    map[string]*Machine{"lab": {Detail: vm.Detail{Machine: vm.Machine{Name: "lab", Installed: true, Instant: true, Disposable: true, Sealed: true}}}},
		Passthrough: map[string]*Passthrough{},
		Servers:     map[string]*Server{},
		Consoles:    map[string]*Console{},
	}
	resources := deriveResources(snap, map[string]bool{})
	if len(resources) != 1 || !resources[0].Pinned || !resources[0].Instant || !resources[0].Disposable || !resources[0].Sealed {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestCachedLogoResolvesWithoutCatalog(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := os.MkdirAll(xdg.IconCache(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(xdg.IconCache(), "ubuntu.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	debianPath := filepath.Join(xdg.IconCache(), "debian.png")
	if err := os.WriteFile(debianPath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(Options{})
	s.mu.Lock()
	s.machines["lab"] = &Machine{Detail: vm.Detail{Machine: vm.Machine{Name: "lab", OS: "ubuntu", Installed: true}}}
	s.publishLocked()
	s.mu.Unlock()

	s.resolveLogos([]string{"ubuntu"})
	s.resolveLogos([]string{"debian"})

	snap := s.Snapshot()
	if got := snap.Catalog.Logos["ubuntu"]; got != path {
		t.Fatalf("cached logo = %q, want %q", got, path)
	}
	if got := snap.Catalog.Logos["debian"]; got != debianPath {
		t.Fatalf("merged cached logo = %q, want %q", got, debianPath)
	}
	if got := snap.Machines["lab"].Logo; got != path {
		t.Fatalf("machine logo = %q, want %q", got, path)
	}
}

func TestServerResourceUsesDistroSlug(t *testing.T) {
	snap := &Snapshot{
		At:          time.Unix(1, 0),
		Settings:    config.Defaults(),
		Machines:    map[string]*Machine{},
		Passthrough: map[string]*Passthrough{},
		Servers: map[string]*Server{
			"node": {
				Host:  remote.Host{Alias: "node"},
				Probe: &remote.Probe{Distro: "Debian GNU/Linux 13"},
			},
		},
		Consoles: map[string]*Console{},
	}
	resources := deriveResources(snap, nil)
	if len(resources) != 1 || resources[0].Icon != "debian" {
		t.Fatalf("server resources = %#v", resources)
	}
}
