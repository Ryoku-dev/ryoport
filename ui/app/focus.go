package app

import (
	gioapp "gioui.org/app"

	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
)

func (s *shell) Config(config gioapp.Config) {
	s.config = config
	s.publishFocus(s.store, focusFor(s.nav, s.store.Snapshot(), config.Mode != gioapp.Minimized, config.Focused))
	s.window.Invalidate()
}

func (s *shell) reportFocus(e *env.Env) {
	s.publishFocus(e.Ops, focusFor(*e.Nav, e.Snap, s.config.Mode != gioapp.Minimized, s.config.Focused))
}

func (s *shell) publishFocus(ops store.Ops, focus store.Focus) {
	if s.haveFocus && focus == s.lastFocus {
		return
	}
	s.haveFocus = true
	s.lastFocus = focus
	ops.SetFocus(focus)
}

func focusFor(nav env.Nav, snapshot *store.Snapshot, visible, focused bool) store.Focus {
	focus := store.Focus{
		Visible:  visible,
		Focused:  focused,
		Page:     nav.Page,
		Selected: nav.Selected,
	}
	if snapshot == nil || nav.Selected == "" {
		return focus
	}
	resource, ok := snapshot.Resource(nav.Selected)
	if ok {
		focus.Tab = nav.Tab(resource.Kind)
	}
	return focus
}
