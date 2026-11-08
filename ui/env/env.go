// Package env is what every page gets for a frame: the theme, the snapshot,
// the actions, and where in the window the user is.
package env

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

// Class is the window's layout class; structure changes between classes,
// never by stretching.
type Class int

const (
	Compact Class = iota
	Medium
	Expanded
)

func ClassFor(width unit.Dp) Class {
	switch {
	case width >= 1280:
		return Expanded
	case width >= 760:
		return Medium
	}
	return Compact
}

type Filter string

const (
	FilterAll      Filter = ""
	FilterMachines Filter = "machines"
	FilterServers  Filter = "servers"
	FilterConsoles Filter = "consoles"
)

// Match reports whether a kind shows under the filter. Passthrough machines
// count as machines.
func (f Filter) Match(k model.Kind) bool {
	switch f {
	case FilterMachines:
		return k == model.KindMachine || k == model.KindPassthrough
	case FilterServers:
		return k == model.KindServer
	case FilterConsoles:
		return k == model.KindConsole
	}
	return true
}

type DraftKind string

const (
	DraftNone        DraftKind = ""
	DraftServer      DraftKind = "server"
	DraftConsole     DraftKind = "console"
	DraftImport      DraftKind = "import"
	DraftInstant     DraftKind = "instant"
	DraftPassthrough DraftKind = "passthrough"
	DraftSpawn       DraftKind = "spawn"
)

// Draft is a form shown in the detail pane in place of a resource. Target is
// the resource being edited, empty for a new one; Seed prefills the form (an
// OS slug, a template name).
type Draft struct {
	Kind   DraftKind
	Target model.ID
	Seed   string
}

// Confirm is a destructive action waiting for an answer. When TypeToConfirm
// is set the action stays disabled until that text is typed.
type Confirm struct {
	Title         string
	Body          string
	Action        string
	TypeToConfirm string
	Run           func()
}

// Nav is where the user is. Pages change it; the shell lays the window out
// from it and reports it to the store.
type Nav struct {
	Page     store.Page
	Selected model.ID
	Draft    Draft
	Filter   Filter
	Query    string
	Palette  bool
	Confirm  *Confirm

	tabs          map[model.Kind]string
	focusSearch   bool
	focusPassword bool
}

// Go changes page, closing the palette and any draft.
func (n *Nav) Go(p store.Page) {
	n.Page = p
	n.Palette = false
	n.Draft = Draft{}
}

// Home returns to the view from above: the fleet page with nothing open.
func (n *Nav) Home() {
	n.Page = store.PageFleet
	n.Selected = ""
	n.Draft = Draft{}
	n.Palette = false
}

// AskPassword selects a server and asks its view to focus the password field.
func (n *Nav) AskPassword(id model.ID) {
	n.Select(id)
	n.focusPassword = true
}

// TakeFocusPassword reports and clears a pending password-field focus.
func (n *Nav) TakeFocusPassword() bool {
	f := n.focusPassword
	n.focusPassword = false
	return f
}

// Select shows a resource in the detail pane, closing any draft.
func (n *Nav) Select(id model.ID) {
	n.Page = store.PageFleet
	n.Selected = id
	n.Draft = Draft{}
	n.Palette = false
}

// Clear leaves the detail pane on the fleet overview.
func (n *Nav) Clear() {
	n.Selected = ""
	n.Draft = Draft{}
}

// Shows reports whether the work pane is showing resource id, so the list
// marks a row as where you are only while its detail is actually open.
func (n *Nav) Shows(id model.ID) bool {
	return id != "" && id == n.Selected && n.Page == store.PageFleet && n.Draft == (Draft{})
}

func (n *Nav) Open(d Draft) {
	n.Page = store.PageFleet
	n.Draft = d
	n.Palette = false
}

func (n *Nav) CloseDraft() { n.Draft = Draft{} }

// Back undoes one level: an overlay, a draft, the create page, then the
// detail pane, which leaves the overview (or, on a compact window, the list).
// It reports false when there was nothing to go back from.
func (n *Nav) Back() bool {
	switch {
	case n.Confirm != nil:
		n.Confirm = nil
	case n.Palette:
		n.Palette = false
	case n.Draft.Kind != DraftNone:
		n.Draft = Draft{}
	case n.Page != store.PageFleet:
		n.Page = store.PageFleet
	case n.Selected != "":
		n.Selected = ""
	default:
		return false
	}
	return true
}

func (n *Nav) Tab(k model.Kind) string { return n.tabs[k] }

func (n *Nav) SetTab(k model.Kind, tab string) {
	if n.tabs == nil {
		n.tabs = make(map[model.Kind]string)
	}
	n.tabs[k] = tab
}

func (n *Nav) Ask(c Confirm) { n.Confirm = &c }

// FocusSearch asks the fleet list to focus its search field on the next frame.
func (n *Nav) FocusSearch() {
	n.Page = store.PageFleet
	n.focusSearch = true
}

func (n *Nav) TakeFocusSearch() bool {
	f := n.focusSearch
	n.focusSearch = false
	return f
}

// Files opens the system file chooser. done runs on the UI goroutine.
type Files interface {
	ChooseFile(extensions []string, done func(path string, err error))
}

type Env struct {
	Th       *theme.Theme
	Snap     *store.Snapshot
	Ops      store.Ops
	Nav      *Nav
	Logos    *kit.Logos
	Placards *kit.Placards
	Class    Class
	Now      time.Time
	// Frame counts the window's frames, so a view can tell it was not drawn
	// in the previous one and drop transient state such as open menus.
	Frame uint64
	Files Files
	// Post runs f on the UI goroutine before the next frame and wakes the window.
	Post func(f func())
}

// View is the shape of every page and pane.
type View interface {
	Layout(gtx layout.Context, e *Env) layout.Dimensions
}
