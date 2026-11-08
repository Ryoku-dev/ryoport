package app

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
)

type shortcutAction uint8

const (
	shortcutPalette shortcutAction = iota
	shortcutSearch
	shortcutNew
	shortcutFleet
	shortcutActivity
	shortcutSettings
	shortcutRefresh
	shortcutBack
	shortcutQuit
)

type shortcut struct {
	Name   key.Name
	Mods   key.Modifiers
	Action shortcutAction
}

var shortcuts = [...]shortcut{
	{Name: "K", Mods: key.ModShortcut, Action: shortcutPalette},
	{Name: "/", Action: shortcutSearch},
	{Name: "N", Mods: key.ModShortcut, Action: shortcutNew},
	{Name: "1", Mods: key.ModShortcut, Action: shortcutFleet},
	{Name: "2", Mods: key.ModShortcut, Action: shortcutActivity},
	{Name: "3", Mods: key.ModShortcut, Action: shortcutSettings},
	{Name: "R", Mods: key.ModShortcut, Action: shortcutRefresh},
	{Name: key.NameEscape, Action: shortcutBack},
	{Name: "Q", Mods: key.ModShortcut, Action: shortcutQuit},
}

var shortcutFilters = func() []event.Filter {
	filters := make([]event.Filter, len(shortcuts))
	for i, shortcut := range shortcuts {
		filters[i] = key.Filter{Name: shortcut.Name, Required: shortcut.Mods}
	}
	return filters
}()

var modalShortcutFilters = []event.Filter{
	key.Filter{Name: key.NameEscape},
	key.Filter{Name: "Q", Required: key.ModShortcut},
}

type keys struct{}

func (keys) Layout(gtx layout.Context, e *env.Env, shell *shell) {
	filters := shortcutFilters
	if e.Nav.Confirm != nil {
		filters = modalShortcutFilters
	}
	for {
		event, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		keyEvent, ok := event.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		action, ok := shortcutFor(keyEvent)
		if !ok {
			continue
		}
		if e.Nav.Confirm != nil && action != shortcutBack && action != shortcutQuit {
			continue
		}
		switch action {
		case shortcutPalette:
			e.Nav.Palette = true
			shell.rail.CloseNew()
		case shortcutSearch:
			if e.Class == env.Compact {
				e.Nav.Clear()
			}
			e.Nav.FocusSearch()
			e.Nav.Palette = false
			shell.rail.CloseNew()
		case shortcutNew:
			shell.rail.ToggleNew()
			e.Nav.Palette = false
		case shortcutFleet:
			goDestination(e, store.PageFleet)
			shell.rail.CloseNew()
		case shortcutActivity:
			goDestination(e, store.PageActivity)
			shell.rail.CloseNew()
		case shortcutSettings:
			goDestination(e, store.PageSettings)
			shell.rail.CloseNew()
		case shortcutRefresh:
			e.Ops.Refresh()
		case shortcutBack:
			if !shell.rail.CloseNew() {
				e.Nav.Back()
			}
		case shortcutQuit:
			shell.window.Perform(system.ActionClose)
		}
	}
}

func shortcutFor(event key.Event) (shortcutAction, bool) {
	if event.State != key.Press {
		return 0, false
	}
	for _, shortcut := range shortcuts {
		if event.Name == shortcut.Name && event.Modifiers == shortcut.Mods {
			return shortcut.Action, true
		}
	}
	return 0, false
}
