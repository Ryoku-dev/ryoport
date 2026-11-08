package fleet

import (
	"strconv"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var filterLabels = [...]string{"All", "Machines", "Servers", "Consoles"}

type entryKind uint8

const (
	entryHeader entryKind = iota
	entryResource
)

type listEntry struct {
	kind     entryKind
	title    string
	count    int
	resource model.Resource
}

type rowState struct {
	row    kit.Row
	action kit.Button
}

// View is the fleet list pane.
type View struct {
	search  kit.Search
	filters kit.Filters
	rows    kit.RowList

	moreButton kit.IconButton
	more       kit.Popover
	moreMenu   kit.Menu
	menuItems  [1]kit.MenuItem

	createMachine kit.Button
	addServer     kit.Button

	rowStates map[model.ID]*rowState
	visible   []model.Resource
	entries   []listEntry
	cursor    model.ID
	query     string
	counts    [4]int
	rev       uint64
}

func New() *View {
	return &View{rowStates: make(map[model.ID]*rowState)}
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Th == nil || e.Snap == nil || e.Nav == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	if v.query != e.Nav.Query {
		v.query = e.Nav.Query
		v.search.SetText(v.query)
	}
	if v.rev != e.Snap.Rev {
		for id := range v.rowStates {
			if _, ok := e.Snap.Resource(id); !ok {
				delete(v.rowStates, id)
			}
		}
		v.rev = e.Snap.Rev
	}

	v.handleEvents(gtx, e)
	focusSearch := e.Nav.TakeFocusSearch()
	v.counts = fleetCounts(e.Snap.Resources)

	return layout.Inset{Top: e.Th.Sp.S4, Right: e.Th.Sp.S3, Bottom: e.Th.Sp.S3, Left: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutTools(gtx, e, focusSearch)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return v.layoutFleet(gtx, e)
			}),
		)
	})
}

func (v *View) handleEvents(gtx layout.Context, e *env.Env) {
	if index, ok := v.filters.Changed(gtx); ok {
		e.Nav.Filter = filterForIndex(index)
	}
	if v.moreButton.Clicked(gtx) {
		v.more.Open = !v.more.Open
	}
	if _, ok := v.moreMenu.Clicked(gtx); ok {
		e.Nav.Filter = env.FilterConsoles
		v.more.Open = false
	}
	if v.createMachine.Clicked(gtx) {
		e.Nav.Go(store.PageCreate)
		if e.Ops != nil {
			e.Ops.LoadCatalog(false)
		}
	}
	if v.addServer.Clicked(gtx) {
		e.Nav.Open(env.Draft{Kind: env.DraftServer})
	}

	for _, resource := range v.visible {
		state := v.rowStates[resource.ID]
		if state == nil {
			continue
		}
		action := state.action.Clicked(gtx)
		clicked := state.row.Clicked(gtx)
		double := state.row.DoubleClicked(gtx)
		if action {
			v.activate(e, resource)
			continue
		}
		if clicked {
			v.cursor = resource.ID
			e.Nav.Select(resource.ID)
		}
		if double {
			v.activate(e, resource)
		}
	}
	v.handleKeys(gtx, e)
}

func (v *View) handleKeys(gtx layout.Context, e *env.Env) {
	filters := []key.Filter{
		{Name: key.NameUpArrow},
		{Name: key.NameDownArrow},
		{Name: key.NameReturn, Optional: key.ModCtrl | key.ModCommand},
		{Name: key.NameEnter, Optional: key.ModCtrl | key.ModCommand},
		{Name: key.Name("/")},
	}
	for {
		event, ok := gtx.Event(filters[0], filters[1], filters[2], filters[3], filters[4])
		if !ok {
			return
		}
		pressed, ok := event.(key.Event)
		if !ok || pressed.State != key.Press {
			continue
		}
		switch pressed.Name {
		case key.NameUpArrow:
			v.cursor = moveSelection(v.visible, v.cursor, -1)
			v.revealCursor()
		case key.NameDownArrow:
			v.cursor = moveSelection(v.visible, v.cursor, 1)
			v.revealCursor()
		case key.NameReturn, key.NameEnter:
			id := v.cursor
			if id == "" {
				id = moveSelection(v.visible, e.Nav.Selected, 0)
				v.cursor = id
			}
			resource, found := resourceByID(v.visible, id)
			if !found {
				continue
			}
			if pressed.Modifiers.Contain(key.ModCtrl) || pressed.Modifiers.Contain(key.ModCommand) {
				e.Nav.Select(resource.ID)
			} else {
				v.activate(e, resource)
			}
		case key.Name("/"):
			e.Nav.FocusSearch()
		}
	}
}

func (v *View) revealCursor() {
	index := -1
	for i, entry := range v.entries {
		if entry.kind == entryResource && entry.resource.ID == v.cursor {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	position := &v.rows.Position
	if index < position.First {
		position.First = index
		position.Offset = 0
		return
	}
	if position.Count > 0 && index >= position.First+position.Count {
		position.First = max(0, index-position.Count+1)
		position.Offset = 0
	}
}

func (v *View) activate(e *env.Env, resource model.Resource) {
	if resource.Primary == model.VerbSetPass || resource.Kind == model.KindServer && resource.State == model.StateNeedsAuth {
		v.cursor = resource.ID
		e.Nav.Select(resource.ID)
		return
	}
	if e.Ops != nil {
		e.Ops.Primary(resource.ID)
	}
}

func (v *View) layoutTools(gtx layout.Context, e *env.Env, focusSearch bool) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if focusSearch {
				v.search.Focus(gtx)
			}
			dims := v.search.Layout(gtx, e.Th, "Search machines and servers", "Ctrl K")
			if query := v.search.Text(); query != v.query {
				v.query = query
				e.Nav.Query = query
			}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutSentence(gtx, e)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutFilters(gtx, e)
		}),
	)
}

func (v *View) layoutSentence(gtx layout.Context, e *env.Env) layout.Dimensions {
	spans, attention := common.FleetSentence(e.Snap)
	if attention >= 0 && attention < len(spans) {
		spans[attention].Color = e.Th.R.Amber
	}
	return layout.Inset{Left: e.Th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return common.Sentence(gtx, e.Th, e.Th.T.Label, e.Th.R.Fg2, spans)
	})
}

func (v *View) layoutFilters(gtx layout.Context, e *env.Env) layout.Dimensions {
	selected := filterIndex(e.Nav.Filter)
	if e.Class == env.Expanded {
		return v.filters.Layout(gtx, e.Th, filterLabels[:], v.counts[:], selected)
	}

	visibleSelected := selected
	if selected == 3 {
		visibleSelected = -1
	}
	v.moreButton.Small = true
	v.menuItems[0] = kit.MenuItem{Label: "Consoles " + strconv.Itoa(v.counts[3])}
	kind := kit.Quiet
	if selected == 3 {
		kind = kit.On
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S1)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.filters.Layout(gtx, e.Th, filterLabels[:3], v.counts[:3], visibleSelected)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.more.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return v.moreButton.Layout(gtx, e.Th, kind, icons.MoreHoriz, "More filters")
				},
				func(gtx layout.Context) layout.Dimensions {
					return v.moreMenu.Layout(gtx, e.Th, v.menuItems[:])
				},
			)
		}),
	)
}

func (v *View) layoutFleet(gtx layout.Context, e *env.Env) layout.Dimensions {
	if !e.Snap.Loaded[store.SourceMachines] && !e.Snap.Loaded[store.SourceServers] {
		v.visible = v.visible[:0]
		v.entries = v.entries[:0]
		return kit.Skeleton(e.Th, 6)(gtx)
	}
	if len(e.Snap.Resources) == 0 {
		v.visible = v.visible[:0]
		v.entries = v.entries[:0]
		return kit.Empty(e.Th, icons.DesktopWindows, "No machines yet", "Make one from the catalogue or add a server.",
			kit.EmptyAction{Button: &v.createMachine, Kind: kit.Gold, Icon: icons.Add, Label: "Create a machine"},
			kit.EmptyAction{Button: &v.addServer, Kind: kit.Line, Icon: icons.Dns, Label: "Add a server"},
		)(gtx)
	}

	filtered := filterResources(e.Snap.Resources, e.Nav.Filter, e.Nav.Query)
	if len(filtered) == 0 {
		v.visible = v.visible[:0]
		v.entries = v.entries[:0]
		return kit.Empty(e.Th, icons.Search, "No matches", "Try another search or filter.")(gtx)
	}

	groups := groupResources(filtered)
	v.visible = v.visible[:0]
	v.entries = v.entries[:0]
	for _, group := range groups {
		v.entries = append(v.entries, listEntry{kind: entryHeader, title: group.Title, count: len(group.Resources)})
		for _, resource := range group.Resources {
			v.visible = append(v.visible, resource)
			v.entries = append(v.entries, listEntry{kind: entryResource, resource: resource})
			if v.rowStates[resource.ID] == nil {
				v.rowStates[resource.ID] = &rowState{}
			}
		}
	}

	selected := -1
	for i, entry := range v.entries {
		if entry.kind == entryResource && e.Nav.Shows(entry.resource.ID) {
			selected = i
			break
		}
	}
	return v.rows.Layout(gtx, e.Th, len(v.entries), selected, func(gtx layout.Context, index int) layout.Dimensions {
		return v.layoutEntry(gtx, e, v.entries[index])
	})
}

func (v *View) layoutEntry(gtx layout.Context, e *env.Env, entry listEntry) layout.Dimensions {
	switch entry.kind {
	case entryHeader:
		return kit.GroupHeader(e.Th, entry.title, entry.count)(gtx)
	}

	resource := entry.resource
	state := v.rowStates[resource.ID]
	selected := e.Nav.Shows(resource.ID)
	focused := resource.ID == v.cursor
	ink := e.Th.R.Fg2
	if selected {
		ink = e.Th.R.OnSel
	}
	claim, proof := common.StateWords(e, resource)
	progress := -1.0
	if job, ok := e.Snap.JobFor(resource.ID); ok && job.Progress >= 0 {
		progress = job.Progress
	}
	label, icon := common.PrimaryLabel(resource)
	var action layout.Widget
	if label != "" {
		action = func(gtx layout.Context) layout.Dimensions {
			state.action.Small = true
			return state.action.Layout(gtx, e.Th, kit.Gold, icon, label)
		}
	}
	return state.row.Layout(gtx, e.Th, kit.RowContent{
		Mark:      common.Mark(e, resource, 32, ink),
		Title:     resource.Name,
		Facts:     resource.Detail,
		FactsMono: resource.Kind == model.KindServer || resource.Kind == model.KindConsole,
		Claim:     claim,
		Proof:     proof,
		Tone:      resource.State.Tone(),
		Progress:  progress,
		Action:    action,
	}, selected, focused)
}

func filterIndex(filter env.Filter) int {
	switch filter {
	case env.FilterMachines:
		return 1
	case env.FilterServers:
		return 2
	case env.FilterConsoles:
		return 3
	default:
		return 0
	}
}

func filterForIndex(index int) env.Filter {
	switch index {
	case 1:
		return env.FilterMachines
	case 2:
		return env.FilterServers
	case 3:
		return env.FilterConsoles
	default:
		return env.FilterAll
	}
}
