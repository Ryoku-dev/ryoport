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
	pending int
}

// Clicked reports one click, taking it from the latch first.
func (c *Click) Clicked(gtx layout.Context) bool {
	if c.pending > 0 {
		c.pending--
		return true
	}
	return c.Clickable.Clicked(gtx)
}

// Layout keeps this frame's clicks and asks for one more frame when it caught
// one, so a click that changes state is drawn at once rather than on the next
// pointer move.
func (c *Click) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	caught := false
	for {
		if _, ok := c.Clickable.Update(gtx); !ok {
			break
		}
		c.pending++
		caught = true
	}
	if caught {
		gtx.Execute(op.InvalidateCmd{})
	}
	return c.Clickable.Layout(gtx, w)
}
