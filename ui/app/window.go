package app

import (
	"context"
	"sync"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoport/internal/cli"
	"github.com/ryoku-dev/ryoport/internal/instance"
	"github.com/ryoku-dev/ryoport/internal/model"
	internalpalette "github.com/ryoku-dev/ryoport/internal/palette"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/activity"
	"github.com/ryoku-dev/ryoport/ui/pages/console"
	"github.com/ryoku-dev/ryoport/ui/pages/create"
	"github.com/ryoku-dev/ryoport/ui/pages/fleet"
	"github.com/ryoku-dev/ryoport/ui/pages/machine"
	"github.com/ryoku-dev/ryoport/ui/pages/overview"
	palettepage "github.com/ryoku-dev/ryoport/ui/pages/palette"
	passthroughpage "github.com/ryoku-dev/ryoport/ui/pages/passthrough"
	"github.com/ryoku-dev/ryoport/ui/pages/server"
	"github.com/ryoku-dev/ryoport/ui/pages/settings"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type shell struct {
	window *gioapp.Window
	store  *store.Store
	post   func(func())

	theme *theme.Theme
	files env.Files
	nav   env.Nav
	logos *kit.Logos

	rail        rail
	keys        keys
	decorations decorations
	overlays    overlays
	compactBack kit.Button

	fleet            *fleet.View
	overview         *overview.View
	server           *server.View
	serverDraft      *server.Draft
	machine          *machine.View
	console          *console.View
	consoleDraft     *console.Draft
	passthrough      *passthroughpage.View
	passthroughDraft *passthroughpage.Draft
	catalog          *create.Catalog
	instant          *create.Instant
	importISO        *create.Import
	spawn            *create.Spawn
	activity         *activity.View
	settings         *settings.View
	palette          *palettepage.Palette

	config         gioapp.Config
	lastFocus      store.Focus
	haveFocus      bool
	appearanceMode string
	density        string
	reduceMotion   bool
	haveMotion     bool
}

func newShell(window *gioapp.Window, state *store.Store, post func(func()), th *theme.Theme) *shell {
	s := &shell{
		window:           window,
		store:            state,
		post:             post,
		theme:            th,
		fleet:            fleet.New(),
		overview:         overview.New(),
		server:           server.New(),
		serverDraft:      server.NewDraft(),
		machine:          machine.New(),
		console:          console.New(),
		consoleDraft:     console.NewDraft(),
		passthrough:      passthroughpage.New(),
		passthroughDraft: passthroughpage.NewDraft(),
		catalog:          create.New(),
		instant:          create.NewInstant(),
		importISO:        create.NewImport(),
		spawn:            create.NewSpawn(),
		activity:         activity.New(),
		settings:         settings.New(),
		palette:          palettepage.New(),
	}
	s.nav.Page = store.PageFleet
	s.logos = kit.NewLogos(window.Invalidate)
	s.decorations.window = window
	return s
}

func (s *shell) route(selected model.ID, page store.Page) {
	if page == "" {
		page = store.PageFleet
	}
	s.nav.Page = page
	s.nav.Selected = selected
	s.nav.Draft = env.Draft{}
	s.nav.Palette = false
	s.nav.Confirm = nil
	if page == store.PageCreate {
		s.store.LoadCatalog(false)
	}
}

func (s *shell) listen(ctx context.Context, messages <-chan instance.Message, post func(func())) {
	if messages == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					return
				}
				selected, page := cli.ParseWindow(message.Args)
				post(func() {
					s.route(selected, page)
					s.window.Perform(system.ActionRaise)
				})
			}
		}
	}()
}

func (s *shell) setPalette(p internalpalette.Palette) {
	density := s.theme.Density
	s.theme = theme.New(theme.FromPalette(p), theme.Shaper())
	s.theme.Density = density
}

func (s *shell) Frame(frame gioapp.FrameEvent, ops *op.Ops, snapshot *store.Snapshot) {
	gtx := gioapp.NewContext(ops, frame)
	if snapshot == nil {
		return
	}
	s.applySettings(snapshot)

	e := &env.Env{
		Th:       s.theme,
		Snap:     snapshot,
		Ops:      s.store,
		Nav:      &s.nav,
		Logos:    s.logos,
		Placards: &s.overlays.placards,
		Class:    windowClass(frame.Metric.PxToDp(gtx.Constraints.Max.X)),
		Now:      frame.Now,
		Files:    s.files,
		Post:     s.post,
	}

	s.keys.Layout(gtx, e, s)
	beforeNavigation := navigationStamp(s.nav)
	paint.Fill(gtx.Ops, e.Th.R.Ground)
	s.Layout(gtx, e)
	s.overlays.Layout(gtx, e, s)
	if navigationStamp(s.nav) != beforeNavigation {
		gtx.Execute(op.InvalidateCmd{})
	}
	s.reportFocus(e)
}

type navStamp struct {
	page     store.Page
	selected model.ID
	draft    env.Draft
	filter   env.Filter
	query    string
	palette  bool
	confirm  *env.Confirm
}

func navigationStamp(nav env.Nav) navStamp {
	return navStamp{
		page: nav.Page, selected: nav.Selected, draft: nav.Draft,
		filter: nav.Filter, query: nav.Query, palette: nav.Palette, confirm: nav.Confirm,
	}
}

func (s *shell) applySettings(snapshot *store.Snapshot) {
	mode := snapshot.Settings.Appearance.Mode
	if mode != s.appearanceMode {
		s.appearanceMode = mode
		s.reloadPalette(mode)
	}
	density := snapshot.Settings.Appearance.Density
	if density != s.density {
		s.density = density
		if density == "compact" {
			s.theme.Density = theme.Compact
		} else {
			s.theme.Density = theme.Comfortable
		}
	}
	reduce := snapshot.Settings.Appearance.ReduceMotion
	if !s.haveMotion || reduce != s.reduceMotion {
		s.haveMotion = true
		s.reduceMotion = reduce
		anim.SetReduce(reduce)
	}
}

func (s *shell) reloadPalette(mode string) {
	go func() {
		p := internalpalette.Load(paletteModeValue(mode))
		s.post(func() {
			if s.appearanceMode == mode {
				s.setPalette(p)
			}
		})
	}()
}

type postQueue struct {
	mu         sync.Mutex
	pending    []func()
	invalidate func()
}

func newPostQueue(invalidate func()) *postQueue {
	return &postQueue{invalidate: invalidate}
}

func (q *postQueue) Post(f func()) {
	if f == nil {
		return
	}
	q.mu.Lock()
	q.pending = append(q.pending, f)
	q.mu.Unlock()
	q.invalidate()
}

func (q *postQueue) Drain() {
	q.mu.Lock()
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	for _, f := range pending {
		f()
	}
}
