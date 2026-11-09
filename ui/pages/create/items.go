package create

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

// Item is one way to add something to the fleet.
type Item struct {
	Icon          *widget.Icon
	Label, Detail string
	Do            func(e *env.Env)
}

type itemKind uint8

const (
	itemCatalog itemKind = iota
	itemInstant
	itemImport
	itemSpawn
	itemPassthrough
	itemServer
	itemConsole
)

func availableItemKinds(host store.Host, hasTemplates bool) []itemKind {
	kinds := []itemKind{itemCatalog, itemInstant, itemImport}
	if hasTemplates {
		kinds = append(kinds, itemSpawn)
	}
	if host.Passthrough {
		kinds = append(kinds, itemPassthrough)
	}
	return append(kinds, itemServer, itemConsole)
}

// Items returns the actions the New menu can offer on this host.
func Items(e *env.Env) []Item {
	hasTemplates := len(e.Snap.Catalog.Templates) > 0
	kinds := availableItemKinds(e.Snap.Host, hasTemplates)
	items := make([]Item, 0, len(kinds))
	for _, kind := range kinds {
		switch kind {
		case itemCatalog:
			items = append(items, Item{
				Icon: icons.DesktopWindows, Label: "From the catalogue", Detail: "Choose an operating system and release",
				Do: func(e *env.Env) {
					e.Nav.Go(store.PageCreate)
					e.Ops.LoadCatalog(false)
				},
			})
		case itemInstant:
			items = append(items, Item{
				Icon: icons.Cloud, Label: "Instant cloud machine", Detail: "Start from a ready cloud image",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftInstant}) },
			})
		case itemImport:
			items = append(items, Item{
				Icon: icons.Album, Label: "From an ISO", Detail: "Install from local media",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftImport}) },
			})
		case itemSpawn:
			items = append(items, Item{
				Icon: icons.ContentCopy, Label: "From a template", Detail: "Copy a prepared machine",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftSpawn}) },
			})
		case itemPassthrough:
			items = append(items, Item{
				Icon: icons.Computer, Label: "Passthrough machine", Detail: "Give a guest direct access to the GPU",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftPassthrough}) },
			})
		case itemServer:
			items = append(items, Item{
				Icon: icons.Dns, Label: "Server", Detail: "Add a machine reached over SSH",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftServer}) },
			})
		case itemConsole:
			items = append(items, Item{
				Icon: icons.SettingsEthernet, Label: "Console", Detail: "Add serial, Telnet, RDP or VNC access",
				Do: func(e *env.Env) { e.Nav.Open(env.Draft{Kind: env.DraftConsole}) },
			})
		}
	}
	return items
}

func pageLayout(gtx layout.Context, e *env.Env, list *kit.List, title string, body layout.Widget) layout.Dimensions {
	return list.Layout(gtx, e.Th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Inset{Top: 30, Right: 36, Bottom: 36, Left: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S4)}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, title)),
				layout.Rigid(body),
			)
		})
	})
}

type fieldValidation struct {
	initialized bool
	focused     bool
	edited      bool
	left        bool
	text        string
}

func (v *fieldValidation) update(gtx layout.Context, field *kit.TextField) {
	text := field.Text()
	focused := gtx.Focused(&field.Editor)
	if !v.initialized {
		v.initialized = true
		v.focused = focused
		v.text = text
		return
	}
	if text != v.text {
		if v.focused || focused {
			v.edited = true
		}
		v.text = text
	}
	if v.focused && !focused && v.edited {
		v.left = true
	}
	v.focused = focused
}

func (v *fieldValidation) show(submitted bool) bool {
	return submitted || v.left
}

func (v *fieldValidation) reset() {
	*v = fieldValidation{}
}

func draftColumn(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Constraints.Max.X, gtx.Dp(640))
		gtx.Constraints.Min.X = width
		gtx.Constraints.Max.X = width
		return w(gtx)
	}
}

func fullWidth(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	}
}

func inset(top, right, bottom, left unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: top, Right: right, Bottom: bottom, Left: left}.Layout(gtx, w)
	}
}

func space(height unit.Dp) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(0, gtx.Dp(height))}
	})
}
