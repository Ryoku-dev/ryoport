package app

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
)

func TestWindowClass(t *testing.T) {
	tests := []struct {
		width unit.Dp
		want  env.Class
	}{
		{width: 640, want: env.Compact},
		{width: 759, want: env.Compact},
		{width: 760, want: env.Medium},
		{width: 1279, want: env.Medium},
		{width: 1280, want: env.Expanded},
		{width: 1920, want: env.Expanded},
	}
	for _, test := range tests {
		if got := windowClass(test.width); got != test.want {
			t.Errorf("windowClass(%v) = %v, want %v", test.width, got, test.want)
		}
	}
}

func TestChooseWork(t *testing.T) {
	serverID := model.NewID(model.KindServer, "ally")
	machineID := model.NewID(model.KindMachine, "debian")
	consoleID := model.NewID(model.KindConsole, "router")
	passthroughID := model.NewID(model.KindPassthrough, "gpu")
	snapshot := &store.Snapshot{Resources: []model.Resource{
		{ID: serverID, Kind: model.KindServer},
		{ID: machineID, Kind: model.KindMachine},
		{ID: consoleID, Kind: model.KindConsole},
		{ID: passthroughID, Kind: model.KindPassthrough},
	}}
	tests := []struct {
		name string
		nav  env.Nav
		want workView
	}{
		{name: "overview", nav: env.Nav{Page: store.PageFleet}, want: workOverview},
		{name: "server", nav: env.Nav{Page: store.PageFleet, Selected: serverID}, want: workServer},
		{name: "machine", nav: env.Nav{Page: store.PageFleet, Selected: machineID}, want: workMachine},
		{name: "console", nav: env.Nav{Page: store.PageFleet, Selected: consoleID}, want: workConsole},
		{name: "passthrough", nav: env.Nav{Page: store.PageFleet, Selected: passthroughID}, want: workPassthrough},
		{name: "missing selection", nav: env.Nav{Page: store.PageFleet, Selected: "server:missing"}, want: workOverview},
		{name: "catalog", nav: env.Nav{Page: store.PageCreate}, want: workCatalog},
		{name: "activity", nav: env.Nav{Page: store.PageActivity, Draft: env.Draft{Kind: env.DraftServer}}, want: workActivity},
		{name: "settings", nav: env.Nav{Page: store.PageSettings, Selected: serverID}, want: workSettings},
		{name: "server draft", nav: env.Nav{Page: store.PageCreate, Draft: env.Draft{Kind: env.DraftServer}}, want: workServerDraft},
		{name: "console draft", nav: env.Nav{Draft: env.Draft{Kind: env.DraftConsole}}, want: workConsoleDraft},
		{name: "passthrough draft", nav: env.Nav{Draft: env.Draft{Kind: env.DraftPassthrough}}, want: workPassthroughDraft},
		{name: "instant draft", nav: env.Nav{Draft: env.Draft{Kind: env.DraftInstant}}, want: workInstantDraft},
		{name: "import draft", nav: env.Nav{Draft: env.Draft{Kind: env.DraftImport}}, want: workImportDraft},
		{name: "spawn draft", nav: env.Nav{Draft: env.Draft{Kind: env.DraftSpawn}}, want: workSpawnDraft},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chooseWork(test.nav, snapshot); got != test.want {
				t.Fatalf("chooseWork() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCompactShowsWork(t *testing.T) {
	machineID := model.NewID(model.KindMachine, "debian")
	snapshot := &store.Snapshot{Resources: []model.Resource{
		{ID: machineID, Kind: model.KindMachine},
	}}
	tests := []struct {
		name string
		nav  env.Nav
		want bool
	}{
		{name: "fleet list", nav: env.Nav{Page: store.PageFleet}},
		{name: "selected resource", nav: env.Nav{Page: store.PageFleet, Selected: machineID}, want: true},
		{name: "missing resource", nav: env.Nav{Page: store.PageFleet, Selected: "machine:missing"}},
		{name: "draft", nav: env.Nav{Page: store.PageFleet, Draft: env.Draft{Kind: env.DraftInstant}}, want: true},
		{name: "create catalogue", nav: env.Nav{Page: store.PageCreate}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compactShowsWork(test.nav, snapshot); got != test.want {
				t.Fatalf("compactShowsWork() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestShortcutTable(t *testing.T) {
	tests := []struct {
		name  key.Name
		mods  key.Modifiers
		want  shortcutAction
		found bool
	}{
		{name: "K", mods: key.ModShortcut, want: shortcutPalette, found: true},
		{name: "/", want: shortcutSearch, found: true},
		{name: "N", mods: key.ModShortcut, want: shortcutNew, found: true},
		{name: "1", mods: key.ModShortcut, want: shortcutFleet, found: true},
		{name: "2", mods: key.ModShortcut, want: shortcutActivity, found: true},
		{name: "3", mods: key.ModShortcut, want: shortcutSettings, found: true},
		{name: "R", mods: key.ModShortcut, want: shortcutRefresh, found: true},
		{name: key.NameEscape, want: shortcutBack, found: true},
		{name: "Q", mods: key.ModShortcut, want: shortcutQuit, found: true},
		{name: "K", found: false},
		{name: "/", mods: key.ModShortcut, found: false},
	}
	for _, test := range tests {
		event := key.Event{Name: test.name, Modifiers: test.mods, State: key.Press}
		got, found := shortcutFor(event)
		if found != test.found || found && got != test.want {
			t.Errorf("shortcutFor(%q, %v) = (%v, %v), want (%v, %v)", test.name, test.mods, got, found, test.want, test.found)
		}
	}
	if _, found := shortcutFor(key.Event{Name: "K", Modifiers: key.ModShortcut, State: key.Release}); found {
		t.Error("key release matched a shortcut")
	}
}
