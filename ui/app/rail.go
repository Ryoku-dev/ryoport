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
	click kit.Click
	page  store.Page
	label string
	icon  *widget.Icon
}

type rail struct {
	home       kit.Click
	newItem    kit.Click
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
	r.pollNavigation(gtx, e)
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	paint.Fill(gtx.Ops, e.Th.R.Stone1)

	tileWidth := gtx.Dp(unit.Dp(64))
	tileHeight := gtx.Dp(unit.Dp(64))
	startY := gtx.Dp(unit.Dp(96))
	if e.Class == env.Medium {
		tileWidth = gtx.Dp(unit.Dp(56))
		tileHeight = gtx.Dp(unit.Dp(56))
		startY = gtx.Dp(unit.Dp(88))
	}
	gap := gtx.Dp(unit.Dp(8))
	newY := startY + tileHeight + gap
	positions := [3]int{startY, newY + tileHeight + gap, height - gtx.Dp(unit.Dp(16)) - tileHeight}
	selected := currentDestination(e.Nav.Page)
	r.vertical.Duration = e.Th.Motion.Select
	r.vertical.Ease = anim.EaseChisel
	r.vertical.Set(gtx, float32(positions[selected]))
	y := int(r.vertical.Value(gtx) + .5)
	selectionX := (width - tileWidth) / 2
	selectionStack := op.Offset(image.Pt(selectionX, y)).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, image.Rect(0, 0, tileWidth, tileHeight), gtx.Dp(e.Th.Cut.Mid), e.Th.R.Sel)
	selectionStack.Pop()

	homeSize := gtx.Dp(unit.Dp(56))
	if e.Class == env.Medium {
		homeSize = gtx.Dp(unit.Dp(48))
	}
	layoutAt(gtx, image.Pt((width-homeSize)/2, gtx.Dp(unit.Dp(16))), image.Pt(homeSize, homeSize), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, e, false)
	})
	for i := range r.dest {
		i := i
		layoutAt(gtx, image.Pt(selectionX, positions[i]), image.Pt(tileWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, e, &r.dest[i], i == selected, e.Class == env.Expanded)
		})
	}
	r.pollNavigation(gtx, e)
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *rail) LayoutBottom(gtx layout.Context, e *env.Env) layout.Dimensions {
	r.init()
	r.pollNavigation(gtx, e)
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	paint.Fill(gtx.Ops, e.Th.R.Stone1)

	tileWidth := width / 5
	tileHeight := gtx.Dp(unit.Dp(48))
	y := (height - tileHeight) / 2
	positions := [3]int{tileWidth, 3 * tileWidth, 4 * tileWidth}
	selected := currentDestination(e.Nav.Page)
	r.bottom.Duration = e.Th.Motion.Select
	r.bottom.Ease = anim.EaseChisel
	r.bottom.Set(gtx, float32(positions[selected]))
	x := int(r.bottom.Value(gtx) + .5)
	selection := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, image.Rect(0, 0, tileWidth, tileHeight), gtx.Dp(e.Th.Cut.Small), e.Th.R.Sel)
	selection.Pop()

	layoutAt(gtx, image.Pt(0, y), image.Pt(tileWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, e, true)
	})
	for i := range r.dest {
		i := i
		layoutAt(gtx, image.Pt(positions[i], y), image.Pt(tileWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, e, &r.dest[i], i == selected, true)
		})
	}
	r.pollNavigation(gtx, e)
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *rail) pollNavigation(gtx layout.Context, e *env.Env) {
	if r.home.Clicked(gtx) {
		e.Nav.Home()
		r.CloseNew()
	}
	for i := range r.dest {
		if r.dest[i].click.Clicked(gtx) {
			goDestination(e, r.dest[i].page)
			r.CloseNew()
		}
	}
}

func (r *rail) layoutHome(gtx layout.Context, e *env.Env, label bool) layout.Dimensions {
	return r.home.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("Home").Add(gtx.Ops)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := unit.Dp(40)
			if label {
				size = unit.Dp(21)
			}
			mark := layout.Rigid(kit.BrandMark(e.Th, size))
			column := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}
			if !label {
				return column.Layout(gtx, mark)
			}
			return column.Layout(gtx,
				mark,
				layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return (kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2, MaxLines: 1}).Layout(gtx, e.Th, "Home")
				}),
			)
		})
	})
}

func (r *rail) layoutDestination(gtx layout.Context, e *env.Env, destination *railDestination, selected, labels bool) layout.Dimensions {
	notch := destination.page == store.PageSettings && (e.Snap.Update.Available || e.Snap.Update.RestartPending)
	return r.layoutItem(gtx, e, &destination.click, destination.icon, destination.label, selected, labels, notch)
}

func (r *rail) layoutItem(gtx layout.Context, e *env.Env, click *kit.Click, icon *widget.Icon, label string, selected, labels, notch bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		if click.Hovered() && !selected {
			kit.FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(e.Th.Cut.Mid), e.Th.R.Stone3)
		}
		if notch {
			kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(e.Th.Cut.Notch), e.Th.R.Amber)
		}
		ink := e.Th.R.Fg2
		if selected {
			ink = e.Th.R.OnSel
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			iconWidget := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(21))
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				return icon.Layout(gtx, ink)
			})
			column := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}
			if !labels {
				return column.Layout(gtx, iconWidget)
			}
			return column.Layout(gtx,
				iconWidget,
				layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return (kit.Label{Style: e.Th.T.Caption, Color: ink, MaxLines: 1}).Layout(gtx, e.Th, label)
				}),
			)
		})
	})
}

func (r *rail) LayoutNewOverlay(gtx layout.Context, e *env.Env) {
	if r.newItem.Clicked(gtx) {
		r.ToggleNew()
	}
	openBeforeLayout := r.newPopover.Open
	width := gtx.Dp(unit.Dp(64))
	height := gtx.Dp(unit.Dp(64))
	x := (gtx.Dp(unit.Dp(88)) - width) / 2
	y := gtx.Dp(unit.Dp(168))
	labels := true
	visualOffset := image.Point{}
	if e.Class == env.Medium {
		width = gtx.Dp(unit.Dp(56))
		height = gtx.Dp(unit.Dp(56))
		x = (gtx.Dp(unit.Dp(72)) - width) / 2
		y = gtx.Dp(unit.Dp(152))
		labels = false
	}
	var items []create.Item
	if r.newPopover.Open {
		items = create.Items(e)
	}
	if e.Class == env.Compact {
		width = gtx.Constraints.Max.X / 5
		height = gtx.Dp(unit.Dp(48))
		x = 2 * width
		itemY := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64)) + (gtx.Dp(unit.Dp(64))-height)/2
		y = itemY
		if r.newPopover.Open {
			menuHeight := gtx.Dp(unit.Dp(12 + 40*len(items)))
			y = max(0, itemY-gtx.Dp(unit.Dp(8))-menuHeight-height)
			visualOffset.Y = itemY - y
		}
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	child := gtx
	child.Constraints.Min = image.Point{}
	r.layoutNew(child, e, image.Pt(width, height), labels, visualOffset, items)
	offset.Pop()
	if r.newItem.Clicked(gtx) {
		r.newPopover.Open = !openBeforeLayout
	}
}

func (r *rail) layoutNew(gtx layout.Context, e *env.Env, itemSize image.Point, labels bool, visualOffset image.Point, items []create.Item) layout.Dimensions {
	dims := r.newPopover.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(itemSize)
			offset := op.Offset(visualOffset).Push(gtx.Ops)
			dims := r.layoutItem(gtx, e, &r.newItem, icons.Add, "New", false, labels, false)
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
