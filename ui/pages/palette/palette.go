package palette

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
	"github.com/ryoku-dev/ryoport/ui/pages/create"
)

const maxVisibleResults = 6

type Palette struct {
	search   kit.Search
	list     kit.List
	scrim    widget.Clickable
	clicks   map[string]*widget.Clickable
	selected int
	query    string
}

type entry struct {
	key      string
	label    string
	detail   string
	shortcut string
	search   string
	icon     *widget.Icon
	resource bool
	run      func()
}

func New() *Palette {
	return &Palette{clicks: make(map[string]*widget.Clickable)}
}

func (p *Palette) Open(gtx layout.Context) {
	p.search.Focus(gtx)
}

func (p *Palette) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Nav == nil || !e.Nav.Palette {
		return layout.Dimensions{}
	}

	entries := p.entries(e)
	items := make([]rankItem, len(entries))
	for i, item := range entries {
		items[i] = rankItem{Text: item.search, Resource: item.resource}
	}
	query := p.search.Text()
	results := rank(query, items)
	if query != p.query {
		p.query = query
		p.selected = 0
	}
	if len(results) == 0 {
		p.selected = 0
	} else if p.selected >= len(results) {
		p.selected = len(results) - 1
	}

	p.prepareClicks(entries)
	for _, result := range results {
		item := entries[result]
		if p.clicks[item.key].Clicked(gtx) {
			p.execute(e, item)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	if result, run, closed := p.handleKeys(gtx, e, results); closed {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	} else if run {
		p.execute(e, entries[result])
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	for p.scrim.Clicked(gtx) {
	}

	return p.overlay(gtx, e, entries, results)
}

func (p *Palette) handleKeys(gtx layout.Context, e *env.Env, results []int) (int, bool, bool) {
	focus := &p.search.Editor
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: focus, Name: key.NameUpArrow},
			key.Filter{Focus: focus, Name: key.NameDownArrow},
			key.Filter{Focus: focus, Name: key.NameReturn},
			key.Filter{Focus: focus, Name: key.NameEnter},
			key.Filter{Focus: focus, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		ke, ok := event.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case key.NameUpArrow:
			if p.selected > 0 {
				p.selected--
				p.list.ScrollTo(p.selected)
			}
		case key.NameDownArrow:
			if p.selected+1 < len(results) {
				p.selected++
				p.list.ScrollTo(p.selected)
			}
		case key.NameReturn, key.NameEnter:
			if len(results) > 0 {
				return results[p.selected], true, false
			}
		case key.NameEscape:
			e.Nav.Back()
			return 0, false, true
		}
	}
	return 0, false, false
}

func (p *Palette) overlay(gtx layout.Context, e *env.Env, entries []entry, results []int) layout.Dimensions {
	window := gtx.Constraints.Max
	margin := gtx.Dp(16)
	width := min(gtx.Dp(640), max(0, window.X-2*margin))
	padding := gtx.Dp(21)
	searchHeight := gtx.Dp(44)
	searchGap := gtx.Dp(13)
	rowHeight := gtx.Dp(56)
	rowGap := gtx.Dp(4)
	visible := min(len(results), maxVisibleResults)
	listHeight := gtx.Dp(68)
	if visible > 0 {
		listHeight = visible*rowHeight + max(0, visible-1)*rowGap
	}
	fixedHeight := 2*padding + searchHeight + searchGap
	maxHeight := max(0, window.Y-2*margin)
	listHeight = min(listHeight, max(0, maxHeight-fixedHeight))
	height := fixedHeight + listHeight

	overlay := op.Record(gtx.Ops)
	scrimGTX := gtx
	scrimGTX.Constraints = layout.Exact(window)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	p.scrim.Layout(scrimGTX, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, e.Th.R.Scrim, clip.Rect{Max: window}.Op())
		return layout.Dimensions{Size: window}
	})
	pass.Pop()

	x := (window.X - width) / 2
	y := window.Y/3 - height/2
	y = max(margin, min(y, window.Y-margin-height))
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	slabRect := image.Rect(0, 0, width, height)
	kit.FillCut(gtx.Ops, slabRect, gtx.Dp(e.Th.Cut.Large), e.Th.R.Stone4)
	kit.StrokeCut(gtx.Ops, slabRect, gtx.Dp(e.Th.Cut.Large), max(1, gtx.Dp(1)), e.Th.R.Vein)

	slabGTX := gtx
	slabGTX.Constraints = layout.Exact(slabRect.Size())
	layout.Inset{Top: 21, Bottom: 21, Left: 21, Right: 21}.Layout(slabGTX, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.search.Layout(gtx, e.Th, "Find a machine or action", "Esc")
			}),
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(results) == 0 {
					return p.empty(gtx, e)
				}
				return p.list.Layout(gtx, e.Th, len(results), func(gtx layout.Context, i int) layout.Dimensions {
					bottom := unit.Dp(0)
					if i+1 < len(results) {
						bottom = 4
					}
					item := entries[results[i]]
					return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return p.resultRow(gtx, e, item, i == p.selected)
					})
				})
			}),
		)
	})
	offset.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return layout.Dimensions{Size: window}
}

func (p *Palette) resultRow(gtx layout.Context, e *env.Env, item entry, selected bool) layout.Dimensions {
	height := gtx.Dp(56)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	click := p.clicks[item.key]
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(item.label).Add(gtx.Ops)
		semantic.DescriptionOp(item.detail).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)

		ink, quiet := e.Th.R.Fg, e.Th.R.Fg3
		fill := e.Th.R.Stone4
		if selected {
			ink, quiet = e.Th.R.OnSel, e.Th.R.OnSel2
			fill = e.Th.R.Sel
		} else if click.Hovered() {
			fill = e.Th.R.Stone3
		}

		record := op.Record(gtx.Ops)
		dims := layout.Inset{Top: 8, Bottom: 8, Left: 13, Right: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 5)
			if item.icon != nil {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(20)
						gtx.Constraints = layout.Exact(image.Pt(size, size))
						return item.icon.Layout(gtx, ink)
					}),
					layout.Rigid(layout.Spacer{Width: 13}.Layout),
				)
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.Row, ink, item.label)),
					layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, quiet, item.detail)),
				)
			}))
			if item.shortcut != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: 13}.Layout),
					layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, quiet, item.shortcut)),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
		call := record.Stop()
		kit.FillCut(gtx.Ops, image.Rectangle{Max: dims.Size}, gtx.Dp(e.Th.Cut.Mid), fill)
		call.Add(gtx.Ops)
		return dims
	})
}

func (p *Palette) empty(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Row, e.Th.R.Fg, "No matching commands")),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Try a machine name or an action.")),
		)
	})
}

func (p *Palette) execute(e *env.Env, item entry) {
	e.Nav.Palette = false
	if item.run != nil {
		item.run()
	}
}

func (p *Palette) prepareClicks(entries []entry) {
	for _, item := range entries {
		if p.clicks[item.key] == nil {
			p.clicks[item.key] = new(widget.Clickable)
		}
	}
	if len(p.clicks) <= len(entries)+32 {
		return
	}
	for key := range p.clicks {
		found := false
		for _, item := range entries {
			if item.key == key {
				found = true
				break
			}
		}
		if !found {
			delete(p.clicks, key)
		}
	}
}

func (p *Palette) entries(e *env.Env) []entry {
	resourceCount := 0
	if e.Snap != nil {
		resourceCount = len(e.Snap.Resources)
	}
	entries := make([]entry, 0, resourceCount*2+12)
	if e.Snap != nil {
		for _, resource := range e.Snap.Resources {
			r := resource
			kind := kindLabel(r.Kind)
			detail := kind
			if r.Detail != "" {
				detail += ", " + r.Detail
			}
			terms := strings.Join(append([]string{r.Name, detail, r.Group, string(r.Kind)}, r.Tags...), " ")
			entries = append(entries, entry{
				key:      "resource:" + string(r.ID),
				label:    r.Name,
				detail:   detail,
				search:   terms,
				icon:     common.KindIcon(r.Kind),
				resource: true,
				run:      func() { e.Nav.Select(r.ID) },
			})
			if r.Primary != model.VerbNone {
				label, icon := common.PrimaryLabel(r)
				entries = append(entries, entry{
					key:      "primary:" + string(r.ID),
					label:    label + " " + r.Name,
					detail:   "Run the primary action for this " + kind,
					search:   label + " " + terms,
					icon:     icon,
					resource: true,
					run:      func() { e.Ops.Primary(r.ID) },
				})
			}
		}
	}

	for i, item := range create.Items(e) {
		item := item
		entries = append(entries, entry{
			key:    "new:" + item.Label + ":" + strconv.Itoa(i),
			label:  item.Label,
			detail: item.Detail,
			search: "new create make " + item.Label + " " + item.Detail,
			icon:   item.Icon,
			run:    func() { item.Do(e) },
		})
	}
	entries = append(entries,
		entry{
			key: "page:activity", label: "Go to Activity", detail: "Show jobs, alerts and recent events", shortcut: "Ctrl 2",
			search: "activity jobs alerts history events", icon: icons.History, run: func() { e.Nav.Go(store.PageActivity) },
		},
		entry{
			key: "page:settings", label: "Go to Settings", detail: "Change Ryoport and machine defaults", shortcut: "Ctrl 3",
			search: "settings preferences defaults", icon: icons.Settings, run: func() { e.Nav.Go(store.PageSettings) },
		},
		entry{
			key: "action:refresh", label: "Refresh now", detail: "Refresh machines, servers and consoles", shortcut: "Ctrl R",
			search: "refresh reload update fleet", icon: icons.Refresh, run: e.Ops.Refresh,
		},
		entry{
			key: "action:check-update", label: "Check for updates", detail: "Look for a newer Ryoport release",
			search: "check updates release version", icon: icons.Update, run: e.Ops.CheckUpdate,
		},
	)

	if e.Snap != nil && e.Nav.Selected != "" {
		if selected, ok := e.Snap.Resource(e.Nav.Selected); ok {
			entries = append(entries, selectedEntries(e, selected)...)
		}
	}
	return entries
}

func selectedEntries(e *env.Env, r model.Resource) []entry {
	selected := make([]entry, 0, 4)
	add := func(key, label, detail string, icon *widget.Icon, run func()) {
		selected = append(selected, entry{
			key:    "selected:" + string(r.ID) + ":" + key,
			label:  label,
			detail: detail,
			search: label + " " + detail + " " + r.Name,
			icon:   icon,
			run:    run,
		})
	}
	switch r.Kind {
	case model.KindMachine:
		if r.State == model.StateRunning || r.State == model.StatePaused {
			add("terminal", "Open terminal on "+r.Name, "Connect to the guest over SSH", icons.Terminal, func() { e.Ops.OpenSSH(r.ID.Key()) })
			add("stop", "Stop "+r.Name, "Shut down this machine", icons.Stop, func() { e.Ops.Stop(r.ID.Key(), false) })
		}
		add("snapshots", "Show snapshots for "+r.Name, "Open the machine's snapshots", icons.Backup, func() {
			e.Nav.Select(r.ID)
			e.Nav.SetTab(r.Kind, "Snapshots")
		})
		add("hardware", "Show settings for "+r.Name, "Open the machine's hardware settings", icons.Settings, func() {
			e.Nav.Select(r.ID)
			e.Nav.SetTab(r.Kind, "Hardware")
		})
	case model.KindServer:
		add("files", "Open files on "+r.Name, "Browse this server's files", icons.FolderOpen, func() { e.Ops.OpenFiles(r.ID.Key()) })
		add("access", "Show access for "+r.Name, "Open keys and connection details", icons.Key, func() {
			e.Nav.Select(r.ID)
			e.Nav.SetTab(r.Kind, "Access")
		})
		add("tunnels", "Show tunnels for "+r.Name, "Open this server's tunnels", icons.SwapHoriz, func() {
			e.Nav.Select(r.ID)
			e.Nav.SetTab(r.Kind, "Tunnels")
		})
	case model.KindPassthrough:
		if r.State == model.StateRunning || r.State == model.StateStarting {
			add("stop", "Stop "+r.Name, "Shut down the passthrough guest", icons.Stop, func() { e.Ops.StopPassthrough(r.ID.Key()) })
		}
		add("gpu", "Open GPU settings", "Review passthrough readiness in Ryoku Hub", icons.Settings, e.Ops.OpenHubGPU)
	case model.KindConsole:
		add("edit", "Edit "+r.Name, "Change this console's connection", icons.Edit, func() {
			e.Nav.Open(env.Draft{Kind: env.DraftConsole, Target: r.ID})
		})
	}
	return selected
}

func kindLabel(kind model.Kind) string {
	switch kind {
	case model.KindMachine:
		return "machine"
	case model.KindPassthrough:
		return "passthrough machine"
	case model.KindServer:
		return "server"
	case model.KindConsole:
		return "console"
	default:
		return "resource"
	}
}
