package machine

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

func TestProofForRunningMachine(t *testing.T) {
	uptime := int64(3*60*60 + 12*60)
	machine := &store.Machine{
		Detail: vm.Detail{Machine: vm.Machine{Cores: "4", RAM: "8G"}},
		Stats:  &qmp.Stats{UptimeS: &uptime, GuestIP: "10.0.2.15"},
	}
	resource := model.Resource{
		ID:    model.NewID(model.KindMachine, "ubuntu-24.04"),
		Kind:  model.KindMachine,
		Name:  "ubuntu-24.04",
		State: model.StateRunning,
	}
	spans := proofFor(machine, resource, nil, time.Time{})
	if got, want := spanText(spans), "Running for 3 h 12 min. 4 cores, 8 GB. Guest 10.0.2.15."; got != want {
		t.Fatalf("proof = %q, want %q", got, want)
	}
	if len(spans) != 4 || !spans[2].Mono || spans[2].Text != "10.0.2.15" {
		t.Fatalf("guest address is not the mono proof span: %#v", spans)
	}
}

func TestProofForRunningMachineUsesStartEvent(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	id := model.NewID(model.KindMachine, "debian")
	machine := &store.Machine{Detail: vm.Detail{Machine: vm.Machine{Name: "debian", Cores: "2", RAM: "4G"}}}
	resource := model.Resource{ID: id, Kind: model.KindMachine, Name: "debian", State: model.StateRunning}
	events := []model.Event{{At: now.Add(-2 * time.Hour), Target: id, Text: "Started debian"}}
	if got, want := spanText(proofFor(machine, resource, events, now)), "Running for 2 h. 2 cores, 4 GB."; got != want {
		t.Fatalf("proof = %q, want %q", got, want)
	}
	if got, want := startedNote(resource, events, now), "◷ Started 2 h ago from Ryoport."; got != want {
		t.Fatalf("started note = %q, want %q", got, want)
	}
}

func TestProofForStoppedMachine(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	id := model.NewID(model.KindMachine, "windows-11")
	machine := &store.Machine{Detail: vm.Detail{
		Machine:   vm.Machine{Cores: "8", RAM: "16G"},
		Snapshots: []vm.Snapshot{{Name: "clean"}, {Name: "tools"}, {Name: "work"}},
	}}
	resource := model.Resource{ID: id, Kind: model.KindMachine, Name: "windows-11", State: model.StateStopped}
	events := []model.Event{{At: time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC), Target: id, Text: "Stopped windows-11"}}
	if got, want := spanText(proofFor(machine, resource, events, now)), "Stopped since Tuesday. 8 cores, 16 GB. 3 snapshots."; got != want {
		t.Fatalf("proof = %q, want %q", got, want)
	}
}

func TestLastRunReadingUsesLatestTargetTransition(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	resource := model.Resource{
		ID:   model.NewID(model.KindMachine, "debian"),
		Name: "debian",
	}
	events := []model.Event{
		{At: now.Add(-2 * time.Hour), Target: resource.ID, Text: "Started debian"},
		{At: now.Add(-10 * time.Minute), Target: model.NewID(model.KindMachine, "other"), Text: "Stopped other"},
		{At: now.Add(-45 * time.Minute), Target: resource.ID, Text: "Stopped debian"},
	}
	if got, want := lastRunReading(events, resource, now), "45 min ago"; got != want {
		t.Fatalf("last run = %q, want %q", got, want)
	}
	if got, want := lastRunReading(nil, resource, now), "Never started"; got != want {
		t.Fatalf("unknown last run = %q, want %q", got, want)
	}
}

func TestStepStatesFollowJobPhase(t *testing.T) {
	tests := []struct {
		name string
		job  model.Job
		want [4]stepStatus
	}{
		{name: "resolve", job: model.Job{Phase: "Resolving", State: model.JobRunning}, want: [4]stepStatus{stepCurrent, stepFuture, stepFuture, stepFuture}},
		{name: "download", job: model.Job{Phase: "Downloading", State: model.JobRunning}, want: [4]stepStatus{stepDone, stepCurrent, stepFuture, stepFuture}},
		{name: "config", job: model.Job{Phase: "Writing config", State: model.JobRunning}, want: [4]stepStatus{stepDone, stepDone, stepCurrent, stepFuture}},
		{name: "first boot", job: model.Job{Phase: "Finishing", State: model.JobRunning}, want: [4]stepStatus{stepDone, stepDone, stepDone, stepCurrent}},
		{name: "done", job: model.Job{Phase: "Done", State: model.JobDone}, want: [4]stepStatus{stepDone, stepDone, stepDone, stepDone}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := stepStates(test.job); got != test.want {
				t.Fatalf("step states = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestAvailableActsByMachineState(t *testing.T) {
	tests := []struct {
		name       string
		state      model.State
		making     bool
		cancelable bool
		want       []actID
	}{
		{name: "stopped", state: model.StateStopped, want: []actID{actStart, actSnapshots, actHardware, actOverflow}},
		{name: "running", state: model.StateRunning, want: []actID{actOpen, actTerminal, actPause, actStop, actOverflow}},
		{name: "paused", state: model.StatePaused, want: []actID{actOpen, actTerminal, actResume, actStop, actOverflow}},
		{name: "starting", state: model.StateStarting, want: []actID{actStop, actOverflow}},
		{name: "stopping", state: model.StateStopping, want: []actID{actStop, actOverflow}},
		{name: "absent", state: model.StateAbsent, want: []actID{actStart, actHardware, actOverflow}},
		{name: "error", state: model.StateError, want: []actID{actOverflow}},
		{name: "unknown", state: model.StateUnknown, want: []actID{actOverflow}},
		{name: "creating", state: model.StateCreating, making: true, cancelable: true, want: []actID{actCancel}},
		{name: "uncancelable create", state: model.StateCreating, making: true, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := availableActs(test.state, test.making, test.cancelable); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("acts = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUserSnapshotsHideReservedSeal(t *testing.T) {
	snapshots := []vm.Snapshot{{Name: "clean"}, {Name: "sealed"}, {Name: "work"}}
	got := userSnapshots(snapshots)
	if len(got) != 2 || got[0].Name != "clean" || got[1].Name != "work" {
		t.Fatalf("user snapshots = %#v", got)
	}
}

func spanText(spans []common.Span) string {
	var b strings.Builder
	for _, span := range spans {
		b.WriteString(span.Text)
	}
	return b.String()
}
