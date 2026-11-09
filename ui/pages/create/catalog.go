package create

import (
	"image"
	"sort"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

// Catalog is the operating-system catalogue used to make a local machine.
type Catalog struct {
	search        kit.Search
	list, grid    kit.List
	tiles         map[string]*kit.Click
	focus         map[string]*anim.Tween
	prefetched    map[string]struct{}
	dialog        kit.Dialog
	releases      kit.Picker
	editions      kit.Picker
	create        kit.Button
	cancel        kit.Button
	install       kit.Button
	retry         kit.Button
	selected      string
	release       string
	edition       string
	pendingJob    string
	cursor        int
	lastQuery     string
	searchFocused bool
	focusRelease  bool
}

func New() *Catalog {
	catalog := &Catalog{
		tiles:      make(map[string]*kit.Click),
		focus:      make(map[string]*anim.Tween),
		prefetched: make(map[string]struct{}),
		dialog:     kit.Dialog{Width: 720},
	}
	catalog.search.Submit = true
	return catalog
}

type rankedSystem struct {
	system build.OS
	rank   int
}

func filterCatalog(systems []build.OS, query string) []build.OS {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return systems
	}
	ranked := make([]rankedSystem, 0, len(systems))
	for _, system := range systems {
		if rank := catalogMatchRank(system, query); rank >= 0 {
			ranked = append(ranked, rankedSystem{system: system, rank: rank})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].rank < ranked[j].rank })
	filtered := make([]build.OS, len(ranked))
	for i := range ranked {
		filtered[i] = ranked[i].system
	}
	return filtered
}

func catalogMatchRank(system build.OS, query string) int {
	name := strings.ToLower(system.Name)
	slug := strings.ToLower(system.OS)
	switch {
	case strings.HasPrefix(name, query):
		return 0
	case hasWordPrefix(name, query), strings.HasPrefix(slug, query):
		return 1
	case strings.Contains(name, query), strings.Contains(slug, query):
		return 2
	default:
		return -1
	}
}

func hasWordPrefix(text, query string) bool {
	for i := range len(text) {
		if i > 0 && wordByte(text[i-1]) {
			continue
		}
		if strings.HasPrefix(text[i:], query) {
			return true
		}
	}
	return false
}

func wordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func (c *Catalog) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	c.followPending(e)
	dims := c.layoutPage(gtx, e)
	c.layoutDialog(gtx, e)
	return dims
}

func (c *Catalog) layoutPage(gtx layout.Context, e *env.Env) layout.Dimensions {
	if c.install.Clicked(gtx) {
		e.Ops.InstallEngine()
	}
	if c.retry.Clicked(gtx) {
		e.Ops.LoadCatalog(true)
	}
	if !e.Snap.Host.Engine.Ready {
		return pageLayout(gtx, e, &c.list, "Make a machine", kit.Empty(e.Th, icons.Computer,
			"The machine engine is not ready",
			"Install the local virtual-machine engine before making a machine.",
			kit.EmptyAction{Button: &c.install, Kind: kit.Gold, Icon: icons.PlayArrow, Label: "Install the engine"},
		))
	}
	if message := e.Snap.Errors[store.SourceCatalog]; message != "" {
		return pageLayout(gtx, e, &c.list, "Make a machine", kit.Empty(e.Th, icons.DesktopWindows,
			"The catalogue could not be loaded", message,
			kit.EmptyAction{Button: &c.retry, Kind: kit.Line, Label: "Try again"},
		))
	}
	if !e.Snap.Loaded[store.SourceCatalog] {
		return pageLayout(gtx, e, &c.list, "Make a machine", kit.Skeleton(e.Th, 5))
	}
	if !c.searchFocused {
		c.search.Focus(gtx)
		c.searchFocused = true
	}
	return layout.Inset{Top: 30, Right: 36, Bottom: 36, Left: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Make a machine")),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return c.layoutBody(gtx, e)
			}),
		)
	})
}

func (c *Catalog) followPending(e *env.Env) {
	if c.pendingJob == "" {
		return
	}
	for _, job := range e.Snap.Jobs {
		if job.ID != c.pendingJob {
			continue
		}
		if job.Target != "" {
			c.pendingJob = ""
			c.dialog.Open = false
			c.selected = ""
			e.Nav.Select(job.Target)
		} else if job.State != model.JobRunning {
			c.pendingJob = ""
			c.dialog.Open = false
			c.selected = ""
			e.Nav.Go(store.PageFleet)
		}
		return
	}
}

func (c *Catalog) layoutBody(gtx layout.Context, e *env.Env) layout.Dimensions {
	return c.gridColumn(gtx, e)
}

func (c *Catalog) gridColumn(gtx layout.Context, e *env.Env) layout.Dimensions {
	before := filterCatalog(e.Snap.Catalog.OS, c.search.Text())
	columns := catalogColumns(gtx, len(before))
	c.handleKeys(gtx, e, before, columns)
	c.consumeSearchSubmit(gtx, e)
	searchGTX := gtx
	searchGTX.Constraints.Min.Y = 0
	record := op.Record(gtx.Ops)
	search := c.search.Layout(searchGTX, e.Th, "Search systems", "")
	searchCall := record.Stop()
	query := c.search.Text()
	filtered := filterCatalog(e.Snap.Catalog.OS, query)
	columns = catalogColumns(gtx, len(filtered))
	if query != c.lastQuery {
		c.cursor = 0
		c.grid.ScrollTo(0)
		c.lastQuery = query
	}
	if len(filtered) > 0 {
		c.cursor = min(c.cursor, len(filtered)-1)
	} else {
		c.cursor = 0
	}
	count := strconv.Itoa(len(filtered)) + " match"
	if strings.TrimSpace(query) == "" {
		noun := " systems"
		if len(filtered) == 1 {
			noun = " system"
		}
		count = strconv.Itoa(len(filtered)) + noun
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			searchCall.Add(gtx.Ops)
			return search
		}),
		layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, count)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return c.layoutGrid(gtx, e, filtered, columns)
		}),
	)
}

func catalogColumns(gtx layout.Context, n int) int {
	gap := gtx.Dp(4)
	available := max(1, gtx.Constraints.Max.X-2*gtx.Dp(4))
	width := max(1, gtx.Dp(200))
	columns := max(1, (available+gap)/(width+gap))
	if n > 0 {
		columns = min(columns, n)
	}
	return columns
}

func (c *Catalog) handleKeys(gtx layout.Context, e *env.Env, systems []build.OS, columns int) {
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: &c.search.Editor, Name: key.NameLeftArrow},
			key.Filter{Focus: &c.search.Editor, Name: key.NameRightArrow},
			key.Filter{Focus: &c.search.Editor, Name: key.NameUpArrow},
			key.Filter{Focus: &c.search.Editor, Name: key.NameDownArrow},
			key.Filter{Focus: &c.search.Editor, Name: key.NameReturn},
			key.Filter{Focus: &c.search.Editor, Name: key.NameEnter},
			key.Filter{Focus: &c.search.Editor, Name: key.NameEscape},
		)
		if !ok {
			return
		}
		press, ok := event.(key.Event)
		if !ok || press.State != key.Press {
			continue
		}
		switch press.Name {
		case key.NameEscape:
			if c.search.Text() != "" {
				c.search.SetText("")
				c.cursor = 0
				c.grid.ScrollTo(0)
			} else {
				e.Nav.Go(store.PageFleet)
			}
		case key.NameReturn, key.NameEnter:
			if len(systems) > 0 {
				c.cursor = min(c.cursor, len(systems)-1)
				c.selectSystem(systems[c.cursor])
			}
		case key.NameLeftArrow:
			c.moveCursor(-1, systems, columns)
		case key.NameRightArrow:
			c.moveCursor(1, systems, columns)
		case key.NameUpArrow:
			c.moveCursor(-columns, systems, columns)
		case key.NameDownArrow:
			c.moveCursor(columns, systems, columns)
		}
	}
}

func (c *Catalog) consumeSearchSubmit(gtx layout.Context, e *env.Env) {
	for {
		event, ok := c.search.Update(gtx)
		if !ok {
			return
		}
		if _, submitted := event.(widget.SubmitEvent); !submitted {
			continue
		}
		systems := filterCatalog(e.Snap.Catalog.OS, c.search.Text())
		if len(systems) == 0 {
			continue
		}
		c.cursor = min(c.cursor, len(systems)-1)
		c.selectSystem(systems[c.cursor])
	}
}

func (c *Catalog) moveCursor(delta int, systems []build.OS, columns int) {
	if len(systems) == 0 {
		c.cursor = 0
		return
	}
	c.cursor = max(0, min(len(systems)-1, c.cursor+delta))
	row := c.cursor / max(1, columns)
	if c.grid.Position.Count == 0 || row < c.grid.Position.First || row >= c.grid.Position.First+c.grid.Position.Count {
		c.grid.ScrollTo(row)
	}
}

func (c *Catalog) layoutGrid(gtx layout.Context, e *env.Env, systems []build.OS, columns int) layout.Dimensions {
	if len(systems) == 0 {
		return kit.Empty(e.Th, icons.DesktopWindows, "No systems found", "Try a name or catalogue slug.")(gtx)
	}
	rows := (len(systems) + columns - 1) / columns
	gap := gtx.Dp(e.Th.Sp.S1)
	return c.grid.Layout(gtx, e.Th, rows, func(gtx layout.Context, row int) layout.Dimensions {
		top := e.Th.Sp.S1
		if row != 0 {
			top = 0
		}
		bottom := e.Th.Sp.S1
		if row == rows-1 {
			bottom = 4
		}
		return layout.Inset{Top: top, Right: 4, Bottom: bottom, Left: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			tileWidth := min(gtx.Dp(260), max(1, (gtx.Constraints.Max.X-gap*(columns-1))/columns))
			children := make([]layout.FlexChild, 0, columns)
			for column := range columns {
				index := row*columns + column
				if index >= len(systems) {
					break
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = tileWidth
					gtx.Constraints.Max.X = tileWidth
					return c.tile(gtx, e, systems[index], index)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gap}.Layout(gtx, children...)
		})
	})
}

func (c *Catalog) tile(gtx layout.Context, e *env.Env, system build.OS, index int) layout.Dimensions {
	click := c.tiles[system.OS]
	if click == nil {
		click = new(kit.Click)
		c.tiles[system.OS] = click
	}
	if click.Clicked(gtx) {
		c.cursor = index
		c.selectSystem(system)
	}
	keyboard := index == c.cursor
	selected := c.selected == system.OS
	if click.Hovered() || keyboard {
		c.prefetchLogo(gtx, e, system)
	}
	background := e.Th.R.Stone3
	nameInk := e.Th.R.Fg
	logoInk := e.Th.R.Fg2
	releaseInk := e.Th.R.Fg3
	if click.Hovered() {
		background = e.Th.R.Stone4
	}
	if selected {
		background = e.Th.R.Sel
		nameInk = e.Th.R.OnSel
		logoInk = e.Th.R.OnSel
		releaseInk = e.Th.R.OnSel
	}
	dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(128)
		return kit.Slab(gtx, background, e.Th.Cut.Mid, inset(13, 13, 13, 13, func(gtx layout.Context) layout.Dimensions {
			logo := e.Logos.Widget(e.Snap.Catalog.Logos[system.OS], 40, system.Name, logoInk)
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					return logo(gtx)
				}),
				layout.Rigid(kit.Text(e.Th, e.Th.T.Row, nameInk, system.Name)),
				layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, releaseInk, build.DefaultRelease(system))),
			)
		}))
	})
	focus := c.focus[system.OS]
	if focus == nil {
		focus = &anim.Tween{Duration: e.Th.Motion.Hover * 3 / 4, Ease: anim.EaseChisel}
		focus.Set(gtx, 0)
		c.focus[system.OS] = focus
	}
	if keyboard {
		focus.Duration = e.Th.Motion.Hover * 3 / 4
		focus.Set(gtx, 1)
	} else {
		focus.Duration = e.Th.Motion.Hover * 5 / 8
		focus.Set(gtx, 0)
	}
	kit.FocusRingAlpha(gtx, e.Th, image.Rectangle{Max: dims.Size}, gtx.Dp(e.Th.Cut.Mid), focus.Value(gtx))
	return dims
}

func (c *Catalog) selectSystem(system build.OS) {
	c.selected = system.OS
	c.release = build.DefaultRelease(system)
	c.edition = build.DefaultEdition(system, c.release)
	c.dialog.Open = true
	c.focusRelease = true
}

func (c *Catalog) selectedSystem(systems []build.OS) (build.OS, bool) {
	for _, system := range systems {
		if system.OS == c.selected {
			return system, true
		}
	}
	return build.OS{}, false
}

func (c *Catalog) layoutDialog(gtx layout.Context, e *env.Env) {
	system, ok := c.selectedSystem(e.Snap.Catalog.OS)
	if !ok {
		c.dialog.Open = false
		c.dialog.Layout(gtx, e.Th, "", func(layout.Context) layout.Dimensions {
			return layout.Dimensions{}
		})
		if c.dialog.Closed(gtx) {
			c.focusRelease = false
			c.search.Focus(gtx)
			c.searchFocused = true
		}
		return
	}
	if c.focusRelease && c.dialog.Open {
		c.releases.Focus(gtx)
		c.focusRelease = false
	}
	releaseIndex := stringIndex(system.Releases, c.release)
	editionOptions := system.Editions[c.release]
	editionIndex := stringIndex(editionOptions, c.edition)
	if c.cancel.Clicked(gtx) {
		c.dialog.Open = false
	}
	if c.create.Clicked(gtx) && c.pendingJob == "" {
		c.pendingJob = e.Ops.Create(system.OS, c.release, c.edition)
		if c.pendingJob == "" {
			c.dialog.Open = false
			c.selected = ""
			e.Nav.Go(store.PageFleet)
		}
	}
	c.create.Busy = c.pendingJob != ""

	body := func(gtx layout.Context) layout.Dimensions {
		logo := e.Logos.Widget(e.Snap.Catalog.Logos[system.OS], 64, system.Name, e.Th.R.Fg2)
		header := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					return logo(gtx)
				}),
				layout.Flexed(1, kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, system.Name)),
			)
		}
		facts := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
				layout.Flexed(1, kit.Facts(e.Th, []kit.Fact{
					{Label: "Releases", Value: strconv.Itoa(len(system.Releases)), Mono: true},
					{Label: "Editions", Value: catalogueEditions(system)},
				})),
				layout.Flexed(1, kit.Facts(e.Th, []kit.Fact{
					{Label: "CPU", Value: strconv.Itoa(e.Snap.Settings.Defaults.Cores) + " cores", Mono: true},
					{Label: "Memory", Value: strconv.Itoa(e.Snap.Settings.Defaults.RAMGB) + " GB", Mono: true},
					{Label: "Disk", Value: strconv.Itoa(e.Snap.Settings.Defaults.DiskGB) + " GB", Mono: true},
				})),
			)
		}
		releasePicker := func(gtx layout.Context) layout.Dimensions {
			dims := c.releases.Layout(gtx, e.Th, "Release", system.Releases, releaseIndex)
			if index, changed := c.releases.Changed(gtx); changed && index >= 0 && index < len(system.Releases) {
				c.release = system.Releases[index]
				c.edition = build.DefaultEdition(system, c.release)
			}
			return dims
		}
		editionPicker := func(gtx layout.Context) layout.Dimensions {
			dims := c.editions.Layout(gtx, e.Th, "Edition", editionOptions, editionIndex)
			if index, changed := c.editions.Changed(gtx); changed && index >= 0 && index < len(editionOptions) {
				c.edition = editionOptions[index]
			}
			return dims
		}
		pickers := func(gtx layout.Context) layout.Dimensions {
			if len(editionOptions) == 0 {
				return fullWidth(releasePicker)(gtx)
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
				layout.Flexed(1, fullWidth(releasePicker)),
				layout.Flexed(1, fullWidth(editionPicker)),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
			layout.Rigid(header),
			layout.Rigid(facts),
			layout.Rigid(pickers),
		)
	}
	create := func(gtx layout.Context) layout.Dimensions {
		return c.create.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Create")
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return c.cancel.Layout(gtx, e.Th, kit.Line, nil, "Cancel")
	}
	c.dialog.Layout(gtx, e.Th, "", body, create, cancel)
	if c.dialog.Closed(gtx) {
		c.selected = ""
		c.focusRelease = false
		c.search.Focus(gtx)
		c.searchFocused = true
	}
}

func (c *Catalog) prefetchLogo(gtx layout.Context, e *env.Env, system build.OS) {
	path := e.Snap.Catalog.Logos[system.OS]
	pixels := max(1, gtx.Dp(64))
	key := path + "\x00" + strconv.Itoa(pixels)
	if _, ok := c.prefetched[key]; ok {
		return
	}
	c.prefetched[key] = struct{}{}
	var operations op.Ops
	prefetch := gtx
	prefetch.Ops = &operations
	prefetch.Constraints = layout.Exact(image.Pt(pixels, pixels))
	e.Logos.Widget(path, 64, system.Name, e.Th.R.Fg2)(prefetch)
}

func catalogueEditions(system build.OS) string {
	seen := make(map[string]bool)
	var editions []string
	for _, release := range system.Releases {
		for _, edition := range system.Editions[release] {
			if edition != "" && !seen[edition] {
				seen[edition] = true
				editions = append(editions, edition)
			}
		}
	}
	if len(editions) == 0 {
		return "None listed"
	}
	return strings.Join(editions, ", ")
}

func stringIndex(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	if len(values) > 0 {
		return 0
	}
	return -1
}
