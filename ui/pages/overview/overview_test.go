package overview

import (
	"testing"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
)

func TestFleetProofUsesSnapshotCounts(t *testing.T) {
	snapshot := &store.Snapshot{
		Resources: []model.Resource{
			{Kind: model.KindServer, State: model.StateOnline},
			{Kind: model.KindServer, State: model.StateOffline},
			{Kind: model.KindMachine, State: model.StateRunning},
			{Kind: model.KindMachine, State: model.StateStopped},
			{Kind: model.KindConsole, State: model.StateOnline},
		},
	}
	got := fleetProof(snapshot)
	want := "1 of 2 servers up, 1 machine stopped, 1 of 1 console reachable."
	if got != want {
		t.Fatalf("fleetProof() = %q, want %q", got, want)
	}
}

func TestElideNameStopsAtWordBoundary(t *testing.T) {
	if got := elideName("production server in west rack", 20); got != "production server…" {
		t.Fatalf("elideName() = %q", got)
	}
	if got := elideName("production-eu-west-backup", 18); got != "production-eu…" {
		t.Fatalf("elideName() at hyphen = %q", got)
	}
	const unbroken = "averylongunbrokenmachinename"
	if got := elideName(unbroken, 12); got != unbroken {
		t.Fatalf("unbroken name lost meaning: %q", got)
	}
}

func TestRunningNowIncludesReachableResources(t *testing.T) {
	tests := []struct {
		resource model.Resource
		want     bool
	}{
		{model.Resource{Kind: model.KindMachine, State: model.StateRunning}, true},
		{model.Resource{Kind: model.KindPassthrough, State: model.StateRunning}, true},
		{model.Resource{Kind: model.KindServer, State: model.StateDegraded}, true},
		{model.Resource{Kind: model.KindConsole, State: model.StateOnline}, true},
		{model.Resource{Kind: model.KindMachine, State: model.StateStopped}, false},
		{model.Resource{Kind: model.KindServer, State: model.StateNeedsAuth}, false},
	}
	for _, test := range tests {
		if got := runningNow(test.resource); got != test.want {
			t.Fatalf("runningNow(%s, %s) = %v, want %v", test.resource.Kind, test.resource.State, got, test.want)
		}
	}
}
