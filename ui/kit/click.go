package kit

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// Click is a clickable area that keeps the clicks its Layout drains until
// Clicked takes them. Gio's Clickable.Layout throws away any click nobody
// asked for before it ran, so a page that draws a button first and asks
// Clicked afterwards would never see a press. With Click either order works.
type Click struct {
	widget.Clickable
	pending       int
	pendingDouble int
}

// Update reports one click, taking it from the latch first.
func (c *Click) Update(gtx layout.Context) (widget.Click, bool) {
	if c.pending > 0 {
		c.pending--
		click := widget.Click{NumClicks: 1}
		if c.pendingDouble > 0 {
			c.pendingDouble--
			click.NumClicks = 2
		}
		return click, true
	}
	return c.Clickable.Update(gtx)
}

// Clicked reports one click, taking it from the latch first.
func (c *Click) Clicked(gtx layout.Context) bool {
	_, ok := c.Update(gtx)
	return ok
}

// Layout keeps this frame's clicks and asks for one more frame when it caught
// one, so a click that changes state is drawn at once rather than on the next
// pointer move.
func (c *Click) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	caught := false
	for {
		click, ok := c.Clickable.Update(gtx)
		if !ok {
			break
		}
		c.pending++
		if click.NumClicks >= 2 {
			c.pendingDouble++
		}
		caught = true
	}
	if caught {
		gtx.Execute(op.InvalidateCmd{})
	}
	return c.Clickable.Layout(gtx, w)
}
