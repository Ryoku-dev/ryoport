package common

import (
	"strings"
	"testing"
	"time"

	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
)

func TestStateWords(t *testing.T) {
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	cpu := 12.4
	uptime := int64(3*time.Hour/time.Second + 12*time.Minute/time.Second)
	tests := []struct {
		name      string
		resource  model.Resource
		snapshot  *store.Snapshot
		wantClaim string
		wantProof string
	}{
		{
			name:      "online server",
			resource:  model.Resource{ID: model.NewID(model.KindServer, "ally"), Kind: model.KindServer, State: model.StateOnline},
			snapshot:  &store.Snapshot{Servers: map[string]*store.Server{"ally": {Reach: &remote.Reach{Up: true, SSHUp: true, RTTMs: 23}}}},
			wantClaim: "Online",
			wantProof: "23 ms",
		},
		{
			name:      "server needs attention",
			resource:  model.Resource{ID: model.NewID(model.KindServer, "mika"), Kind: model.KindServer, State: model.StateNeedsAuth, Updated: now.Add(-2 * time.Minute)},
			snapshot:  &store.Snapshot{Servers: map[string]*store.Server{"mika": {}}},
			wantClaim: "Needs you",
			wantProof: "2 min ago",
		},
		{
			name:      "running machine CPU",
			resource:  model.Resource{ID: model.NewID(model.KindMachine, "ubuntu"), Kind: model.KindMachine, State: model.StateRunning},
			snapshot:  &store.Snapshot{Machines: map[string]*store.Machine{"ubuntu": {Stats: &qmp.Stats{HostCPUPct: &cpu}}}},
			wantClaim: "Running",
			wantProof: "12%",
		},
		{
			name:      "running machine uptime",
			resource:  model.Resource{ID: model.NewID(model.KindMachine, "fedora"), Kind: model.KindMachine, State: model.StateRunning},
			snapshot:  &store.Snapshot{Machines: map[string]*store.Machine{"fedora": {Stats: &qmp.Stats{UptimeS: &uptime}}}},
			wantClaim: "Running",
			wantProof: "3 h 12 min",
		},
		{
			name:      "stopped machine",
			resource:  model.Resource{ID: model.NewID(model.KindMachine, "windows"), Kind: model.KindMachine, State: model.StateStopped, Updated: now.Add(-2 * time.Hour)},
			snapshot:  &store.Snapshot{},
			wantClaim: "Stopped",
			wantProof: "",
		},
		{
			name:      "machine job",
			resource:  model.Resource{ID: model.NewID(model.KindMachine, "debian"), Kind: model.KindMachine, State: model.StateCreating},
			snapshot:  &store.Snapshot{Jobs: []model.Job{{Target: model.NewID(model.KindMachine, "debian"), State: model.JobRunning, Progress: 0.25, Done: 60, Total: 240, Rate: 1}}},
			wantClaim: "25%",
			wantProof: "about 3 min left",
		},
		{
			name:      "ready console",
			resource:  model.Resource{ID: model.NewID(model.KindConsole, "router"), Kind: model.KindConsole, State: model.StateOnline},
			snapshot:  &store.Snapshot{},
			wantClaim: "Ready",
		},
		{
			name:      "stopped passthrough",
			resource:  model.Resource{ID: model.NewID(model.KindPassthrough, "gpu"), Kind: model.KindPassthrough, State: model.StateStopped},
			snapshot:  &store.Snapshot{},
			wantClaim: "Stopped",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotClaim, gotProof := StateWords(&env.Env{Snap: test.snapshot, Now: now}, test.resource)
			if gotClaim != test.wantClaim || gotProof != test.wantProof {
				t.Fatalf("StateWords() = (%q, %q), want (%q, %q)", gotClaim, gotProof, test.wantClaim, test.wantProof)
			}
		})
	}
}

func TestFleetSentence(t *testing.T) {
	machine := func(name string, state model.State) model.Resource {
		return model.Resource{ID: model.NewID(model.KindMachine, name), Kind: model.KindMachine, Name: name, State: state}
	}
	tests := []struct {
		name          string
		snapshot      *store.Snapshot
		want          string
		wantAttention int
	}{
		{
			name:          "empty fleet",
			snapshot:      &store.Snapshot{},
			want:          "Nothing here yet.",
			wantAttention: -1,
		},
		{
			name: "all up",
			snapshot: &store.Snapshot{Resources: []model.Resource{
				machine("work", model.StateRunning),
				{ID: model.NewID(model.KindPassthrough, "gpu"), Kind: model.KindPassthrough, Name: "gpu", State: model.StateRunning},
				{ID: model.NewID(model.KindServer, "ally"), Kind: model.KindServer, Name: "ally", State: model.StateOnline},
				{ID: model.NewID(model.KindConsole, "router"), Kind: model.KindConsole, Name: "router", State: model.StateOnline},
			}},
			want:          "Everything is up.",
			wantAttention: -1,
		},
		{
			name: "one needing you",
			snapshot: &store.Snapshot{
				Resources: []model.Resource{
					machine("work", model.StateRunning),
					{ID: model.NewID(model.KindServer, "mika"), Kind: model.KindServer, Name: "mika-server", State: model.StateNeedsAuth},
				},
				Alerts: []model.Alert{{Target: model.NewID(model.KindServer, "mika"), Title: "Password needed"}},
			},
			want:          "One running. mika-server needs you.",
			wantAttention: 1,
		},
		{
			name:          "counts above twelve",
			snapshot:      &store.Snapshot{Resources: append(thirteenRunning(), machine("sleeping", model.StateStopped))},
			want:          "13 running.",
			wantAttention: -1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spans, attention := FleetSentence(test.snapshot)
			if got := spanText(spans); got != test.want {
				t.Fatalf("FleetSentence() text = %q, want %q", got, test.want)
			}
			if attention != test.wantAttention {
				t.Fatalf("FleetSentence() attention = %d, want %d", attention, test.wantAttention)
			}
		})
	}
}

func TestPrimaryLabel(t *testing.T) {
	tests := []struct {
		verb      model.Verb
		wantLabel string
		wantIcon  *widget.Icon
	}{
		{model.VerbConnect, "Connect", icons.Terminal},
		{model.VerbStart, "Start", icons.PlayArrow},
		{model.VerbOpen, "Open", icons.OpenInNew},
		{model.VerbSetPass, "Enter password", icons.Key},
		{model.VerbStop, "Stop", icons.Stop},
		{model.VerbNone, "", nil},
	}
	for _, test := range tests {
		t.Run(string(test.verb), func(t *testing.T) {
			label, icon := PrimaryLabel(model.Resource{Primary: test.verb})
			if label != test.wantLabel || icon != test.wantIcon {
				t.Fatalf("PrimaryLabel() = (%q, %p), want (%q, %p)", label, icon, test.wantLabel, test.wantIcon)
			}
		})
	}
}

func thirteenRunning() []model.Resource {
	resources := make([]model.Resource, 13)
	for i := range resources {
		name := "machine-" + string(rune('a'+i))
		resources[i] = model.Resource{ID: model.NewID(model.KindMachine, name), Kind: model.KindMachine, Name: name, State: model.StateRunning}
	}
	return resources
}

func spanText(spans []Span) string {
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.Text)
	}
	return text.String()
}
