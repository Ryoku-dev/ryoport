package app

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/create"
)

type railDestination struct {
	click widget.Clickable
	page  store.Page
	label string
	icon  *widget.Icon
}

type rail struct {
	newPlaque  kit.NewPlaque
	newPopover kit.Popover
	newMenu    kit.Menu
	menuItems  []kit.MenuItem
	dest       [3]railDestination
	vertical   anim.Tween
	bottom     anim.Tween
}

func (r *rail) init() {
	if r.dest[0].page != "" {
		return
	}
	r.dest = [3]railDestination{
		{page: store.PageFleet, label: "Fleet", icon: icons.Dns},
		{page: store.PageActivity, label: "Activity", icon: icons.History},
		{page: store.PageSettings, label: "Settings", icon: icons.Tune},
	}
}

func (r *rail) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	r.init()
	for i := range r.dest {
		if r.dest[i].click.Clicked(gtx) {
			goDestination(e, r.dest[i].page)
		}
	}
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	paint.Fill(gtx.Ops, e.Th.R.Stone1)

	tileWidth := gtx.Dp(unit.Dp(64))
	tileHeight := gtx.Dp(unit.Dp(64))
	if e.Class == env.Medium {
		tileWidth = gtx.Dp(unit.Dp(56))
		tileHeight = gtx.Dp(unit.Dp(56))
	}
	startY := gtx.Dp(unit.Dp(102))
	gap := gtx.Dp(unit.Dp(8))
	positions := [3]int{startY, startY + tileHeight + gap, height - gtx.Dp(unit.Dp(16)) - tileHeight}
	selected := currentDestination(e.Nav.Page)
	r.vertical.Duration = e.Th.Motion.Select
	r.vertical.Ease = anim.EaseChisel
	r.vertical.Set(gtx, float32(positions[selected]))
	y := int(r.vertical.Value(gtx) + .5)
	selectionX := (width - tileWidth) / 2
	selectionStack := op.Offset(image.Pt(selectionX, y)).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, image.Rect(0, 0, tileWidth, tileHeight), gtx.Dp(e.Th.Cut.Mid), e.Th.R.Sel)
	selectionStack.Pop()

	for i := range r.dest {
		i := i
		layoutAt(gtx, image.Pt(selectionX, positions[i]), image.Pt(tileWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, e, &r.dest[i], i == selected, e.Class == env.Expanded)
		})
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *rail) LayoutBottom(gtx layout.Context, e *env.Env) layout.Dimensions {
	r.init()
	for i := range r.dest {
		if r.dest[i].click.Clicked(gtx) {
			goDestination(e, r.dest[i].page)
		}
	}
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	paint.Fill(gtx.Ops, e.Th.R.Stone1)

	startX := gtx.Dp(unit.Dp(64))
	available := width - startX
	tileWidth := available / len(r.dest)
	tileHeight := gtx.Dp(unit.Dp(48))
	y := (height - tileHeight) / 2
	selected := currentDestination(e.Nav.Page)
	r.bottom.Duration = e.Th.Motion.Select
	r.bottom.Ease = anim.EaseChisel
	r.bottom.Set(gtx, float32(startX+selected*tileWidth))
	x := int(r.bottom.Value(gtx) + .5)
	selection := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, image.Rect(0, 0, tileWidth, tileHeight), gtx.Dp(e.Th.Cut.Small), e.Th.R.Sel)
	selection.Pop()

	for i := range r.dest {
		i := i
		layoutAt(gtx, image.Pt(startX+i*tileWidth, y), image.Pt(tileWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, e, &r.dest[i], i == selected, true)
		})
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *rail) layoutDestination(gtx layout.Context, e *env.Env, destination *railDestination, selected, labels bool) layout.Dimensions {
	return destination.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(destination.label).Add(gtx.Ops)
		if destination.click.Hovered() && !selected {
			kit.FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(e.Th.Cut.Mid), e.Th.R.Stone3)
		}
		if destination.page == store.PageSettings && (e.Snap.Update.Available || e.Snap.Update.RestartPending) {
			kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(e.Th.Cut.Notch), e.Th.R.Amber)
		}
		ink := e.Th.R.Fg2
		if selected {
			ink = e.Th.R.OnSel
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			icon := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(21))
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				return destination.icon.Layout(gtx, ink)
			})
			column := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}
			if !labels {
				return column.Layout(gtx, icon)
			}
			return column.Layout(gtx,
				icon,
				layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return (kit.Label{Style: e.Th.T.Caption, Color: ink, MaxLines: 1}).Layout(gtx, e.Th, destination.label)
				}),
			)
		})
	})
}

func (r *rail) LayoutNewOverlay(gtx layout.Context, e *env.Env) {
	if r.newPlaque.Clicked(gtx) {
		r.newPopover.Open = !r.newPopover.Open
	}
	compact := e.Class != env.Expanded
	width := gtx.Dp(unit.Dp(62))
	height := gtx.Dp(unit.Dp(52))
	x := (gtx.Dp(unit.Dp(88)) - width) / 2
	y := gtx.Dp(unit.Dp(16))
	visualOffset := image.Point{}
	var items []create.Item
	if r.newPopover.Open {
		items = create.Items(e)
	}
	if e.Class == env.Medium {
		width = gtx.Dp(unit.Dp(48))
		height = gtx.Dp(unit.Dp(40))
		x = (gtx.Dp(unit.Dp(72)) - width) / 2
	}
	if e.Class == env.Compact {
		height = gtx.Dp(unit.Dp(40))
		x = gtx.Dp(unit.Dp(8))
		plaqueY := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64)) + (gtx.Dp(unit.Dp(64))-height)/2
		y = plaqueY
		if r.newPopover.Open {
			menuHeight := gtx.Dp(unit.Dp(12 + 40*len(items)))
			y = max(0, plaqueY-gtx.Dp(unit.Dp(8))-menuHeight-height)
			visualOffset.Y = plaqueY - y
		}
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	child := gtx
	child.Constraints.Min = image.Point{}
	r.layoutNew(child, e, compact, visualOffset, items)
	offset.Pop()
}

func (r *rail) layoutNew(gtx layout.Context, e *env.Env, compact bool, visualOffset image.Point, items []create.Item) layout.Dimensions {
	dims := r.newPopover.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			offset := op.Offset(visualOffset).Push(gtx.Ops)
			dims := r.newPlaque.Layout(gtx, e.Th, compact, r.newPopover.Open)
			offset.Pop()
			return dims
		},
		func(gtx layout.Context) layout.Dimensions {
			if cap(r.menuItems) < len(items) {
				r.menuItems = make([]kit.MenuItem, len(items))
			} else {
				r.menuItems = r.menuItems[:len(items)]
			}
			for i, item := range items {
				r.menuItems[i] = kit.MenuItem{Icon: item.Icon, Label: item.Label}
			}
			dims := r.newMenu.Layout(gtx, e.Th, r.menuItems)
			if selected, ok := r.newMenu.Clicked(gtx); ok && selected >= 0 && selected < len(items) {
				r.newPopover.Open = false
				items[selected].Do(e)
				gtx.Execute(op.InvalidateCmd{})
			}
			return dims
		},
	)
	return dims
}

func (r *rail) ToggleNew() {
	r.newPopover.Open = !r.newPopover.Open
}

func (r *rail) CloseNew() bool {
	wasOpen := r.newPopover.Open
	r.newPopover.Open = false
	return wasOpen
}

func goDestination(e *env.Env, page store.Page) {
	if e.Class == env.Compact && page == store.PageFleet {
		e.Nav.Clear()
	}
	e.Nav.Go(page)
}

func currentDestination(page store.Page) int {
	switch page {
	case store.PageActivity:
		return 1
	case store.PageSettings:
		return 2
	default:
		return 0
	}
}
