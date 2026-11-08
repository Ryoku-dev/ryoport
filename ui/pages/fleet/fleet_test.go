package fleet

import (
	"reflect"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
)

func TestQueryMatches(t *testing.T) {
	resource := model.Resource{
		Name:   "Mika-Server",
		Detail: "root@203.0.113.70:2222",
		Tags:   []string{"Production", "edge"},
	}
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "empty", query: "", want: true},
		{name: "name ignores case", query: "mIkA", want: true},
		{name: "facts ignore case", query: "ROOT@203", want: true},
		{name: "tag ignores case", query: "DUCT", want: true},
		{name: "overlapping prefix", query: "server", want: true},
		{name: "missing", query: "console", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := queryMatches(resource, foldedRunes(test.query)); got != test.want {
				t.Fatalf("queryMatches(%q) = %v, want %v", test.query, got, test.want)
			}
		})
	}

	overlap := model.Resource{Name: "ababac"}
	if !queryMatches(overlap, foldedRunes("abac")) {
		t.Fatal("queryMatches did not resume at an overlapping prefix")
	}
}

func TestFilterResources(t *testing.T) {
	resources := []model.Resource{
		{ID: "machine:ubuntu", Kind: model.KindMachine, Name: "Ubuntu", Detail: "4 cores", Tags: []string{"work"}},
		{ID: "passthrough:gpu", Kind: model.KindPassthrough, Name: "win11-gpu", Detail: "Looking Glass"},
		{ID: "server:ally", Kind: model.KindServer, Name: "ally", Detail: "root@host", Tags: []string{"production"}},
		{ID: "console:router", Kind: model.KindConsole, Name: "router", Detail: "/dev/ttyUSB0"},
	}
	tests := []struct {
		name   string
		filter env.Filter
		query  string
		want   []model.ID
	}{
		{name: "all", filter: env.FilterAll, want: []model.ID{"machine:ubuntu", "passthrough:gpu", "server:ally", "console:router"}},
		{name: "machines include passthrough", filter: env.FilterMachines, want: []model.ID{"machine:ubuntu", "passthrough:gpu"}},
		{name: "server facts query", filter: env.FilterServers, query: "ROOT@H", want: []model.ID{"server:ally"}},
		{name: "console query", filter: env.FilterConsoles, query: "ttyusb", want: []model.ID{"console:router"}},
		{name: "query before kind does not leak", filter: env.FilterMachines, query: "production", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resourceIDs(filterResources(resources, test.filter, test.query))
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("filterResources() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGroupResources(t *testing.T) {
	resources := []model.Resource{
		{ID: "machine:stopped", State: model.StateStopped},
		{ID: "machine:running", State: model.StateRunning},
		{ID: "server:failed", State: model.StateError},
		{ID: "machine:creating", State: model.StateCreating},
		{ID: "machine:pinned-running", State: model.StateRunning, Pinned: true},
		{ID: "server:pinned-auth", State: model.StateNeedsAuth, Pinned: true},
	}
	tests := []struct {
		name      string
		resources []model.Resource
		wantKinds []groupKind
		wantIDs   [][]model.ID
	}{
		{
			name:      "fixed group order and pinned first",
			resources: resources,
			wantKinds: []groupKind{groupNeeds, groupRunning, groupBecoming, groupStopped},
			wantIDs: [][]model.ID{
				{"server:pinned-auth", "server:failed"},
				{"machine:pinned-running", "machine:running"},
				{"machine:creating"},
				{"machine:stopped"},
			},
		},
		{
			name:      "empty groups hidden",
			resources: resources[1:2],
			wantKinds: []groupKind{groupRunning},
			wantIDs:   [][]model.ID{{"machine:running"}},
		},
		{name: "empty fleet", wantKinds: []groupKind{}, wantIDs: [][]model.ID{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			groups := groupResources(test.resources)
			gotKinds := make([]groupKind, len(groups))
			gotIDs := make([][]model.ID, len(groups))
			for i, group := range groups {
				gotKinds[i] = group.Kind
				gotIDs[i] = resourceIDs(group.Resources)
			}
			if !reflect.DeepEqual(gotKinds, test.wantKinds) {
				t.Fatalf("group kinds = %v, want %v", gotKinds, test.wantKinds)
			}
			if !reflect.DeepEqual(gotIDs, test.wantIDs) {
				t.Fatalf("group resources = %v, want %v", gotIDs, test.wantIDs)
			}
		})
	}
}

func TestMoveSelection(t *testing.T) {
	resources := []model.Resource{{ID: "machine:a"}, {ID: "server:b"}, {ID: "console:c"}}
	tests := []struct {
		name    string
		rows    []model.Resource
		current model.ID
		delta   int
		want    model.ID
	}{
		{name: "down from none starts first", rows: resources, delta: 1, want: "machine:a"},
		{name: "up from none starts last", rows: resources, delta: -1, want: "console:c"},
		{name: "down", rows: resources, current: "machine:a", delta: 1, want: "server:b"},
		{name: "up", rows: resources, current: "console:c", delta: -1, want: "server:b"},
		{name: "top clamps", rows: resources, current: "machine:a", delta: -1, want: "machine:a"},
		{name: "bottom clamps", rows: resources, current: "console:c", delta: 1, want: "console:c"},
		{name: "missing current starts first", rows: resources, current: "server:gone", delta: 1, want: "machine:a"},
		{name: "empty", current: "server:b", delta: 1, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := moveSelection(test.rows, test.current, test.delta); got != test.want {
				t.Fatalf("moveSelection() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFleetCounts(t *testing.T) {
	resources := []model.Resource{
		{Kind: model.KindMachine},
		{Kind: model.KindPassthrough},
		{Kind: model.KindServer},
		{Kind: model.KindConsole},
	}
	if got, want := fleetCounts(resources), [4]int{4, 2, 1, 1}; got != want {
		t.Fatalf("fleetCounts() = %v, want %v", got, want)
	}
}

func resourceIDs(resources []model.Resource) []model.ID {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]model.ID, len(resources))
	for i := range resources {
		ids[i] = resources[i].ID
	}
	return ids
}
