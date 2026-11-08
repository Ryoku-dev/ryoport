package create

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

// Catalog is the operating-system catalogue used to make a local machine.
type Catalog struct {
	search     kit.Search
	list       kit.List
	tiles      map[string]*widget.Clickable
	selected   string
	release    string
	edition    string
	releases   kit.Picker
	editions   kit.Picker
	create     kit.Button
	install    kit.Button
	retry      kit.Button
	pendingJob string
}

func New() *Catalog {
	return &Catalog{tiles: make(map[string]*widget.Clickable)}
}

func filterCatalog(systems []build.OS, query string) []build.OS {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return systems
	}
	filtered := make([]build.OS, 0, len(systems))
	for _, system := range systems {
		if strings.Contains(strings.ToLower(system.Name), query) || strings.Contains(strings.ToLower(system.OS), query) {
			filtered = append(filtered, system)
		}
	}
	return filtered
}

func (c *Catalog) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	c.followPending(e)
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
	return pageLayout(gtx, e, &c.list, "Make a machine", c.body(e))
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
			e.Nav.Select(job.Target)
		} else if job.State != model.JobRunning {
			c.pendingJob = ""
			e.Nav.Go(store.PageFleet)
		}
		return
	}
}

func (c *Catalog) body(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		search := func(gtx layout.Context) layout.Dimensions {
			return c.search.Layout(gtx, e.Th, "Search systems", "")
		}
		filtered := filterCatalog(e.Snap.Catalog.OS, c.search.Text())
		grid := func(gtx layout.Context) layout.Dimensions {
			if len(filtered) == 0 {
				return kit.Empty(e.Th, icons.DesktopWindows, "No systems found", "Try a name or catalogue slug.")(gtx)
			}
			return kit.Columns(gtx, 200, e.Th.Sp.S1, len(filtered), func(gtx layout.Context, i int) layout.Dimensions {
				return c.tile(gtx, e, filtered[i])
			})
		}
		children := []layout.FlexChild{
			layout.Rigid(search),
			layout.Rigid(grid),
		}
		if system, ok := c.selectedSystem(e.Snap.Catalog.OS); ok {
			children = append(children, layout.Rigid(c.details(e, system)))
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx, children...)
	}
}

func (c *Catalog) tile(gtx layout.Context, e *env.Env, system build.OS) layout.Dimensions {
	click := c.tiles[system.OS]
	if click == nil {
		click = new(widget.Clickable)
		c.tiles[system.OS] = click
	}
	if click.Clicked(gtx) {
		c.selectSystem(system)
	}
	selected := c.selected == system.OS
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
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
}

func (c *Catalog) selectSystem(system build.OS) {
	c.selected = system.OS
	c.release = build.DefaultRelease(system)
	c.edition = build.DefaultEdition(system, c.release)
}

func (c *Catalog) selectedSystem(systems []build.OS) (build.OS, bool) {
	for _, system := range systems {
		if system.OS == c.selected {
			return system, true
		}
	}
	return build.OS{}, false
}

func (c *Catalog) details(e *env.Env, system build.OS) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		releaseIndex := stringIndex(system.Releases, c.release)
		editionOptions := system.Editions[c.release]
		editionIndex := stringIndex(editionOptions, c.edition)
		pickers := func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Flexed(1, fullWidth(func(gtx layout.Context) layout.Dimensions {
					dims := c.releases.Layout(gtx, e.Th, "Release", system.Releases, releaseIndex)
					if index, changed := c.releases.Changed(gtx); changed && index >= 0 && index < len(system.Releases) {
						c.release = system.Releases[index]
						c.edition = build.DefaultEdition(system, c.release)
					}
					return dims
				})),
			}
			if len(editionOptions) > 0 {
				children = append(children, layout.Flexed(1, fullWidth(func(gtx layout.Context) layout.Dimensions {
					dims := c.editions.Layout(gtx, e.Th, "Edition", editionOptions, editionIndex)
					if index, changed := c.editions.Changed(gtx); changed && index >= 0 && index < len(editionOptions) {
						c.edition = editionOptions[index]
					}
					return dims
				})))
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx, children...)
		}
		if c.create.Clicked(gtx) && c.pendingJob == "" {
			c.pendingJob = e.Ops.Create(system.OS, c.release, c.edition)
			if c.pendingJob == "" {
				e.Nav.Go(store.PageFleet)
			}
		}
		c.create.Busy = c.pendingJob != ""
		content := inset(18, 18, 18, 18, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, system.Name)),
				layout.Rigid(pickers),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.create.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Create")
				}),
			)
		})
		return kit.Slab(gtx, e.Th.R.Stone3, e.Th.Cut.Mid, content)
	}
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
