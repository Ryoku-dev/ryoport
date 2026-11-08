package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

type Popover struct {
	Open        bool
	dismiss     Click
	contentRect image.Rectangle
	focusTag    int
	outsideTag  int
	wasOpen     bool
}

var activePopover *Popover

func (p *Popover) close() {
	p.Open = false
	p.wasOpen = false
	p.contentRect = image.Rectangle{}
	if activePopover == p {
		activePopover = nil
	}
}

const overlayHostValue = "ryoport.ui.kit.overlay-host"

func overlayContentContext(gtx layout.Context) layout.Context {
	values := make(map[string]any, len(gtx.Values)+1)
	for key, value := range gtx.Values {
		values[key] = value
	}
	values[overlayHostValue] = true
	gtx.Values = values
	return gtx
}

func deferOverlay(gtx layout.Context, call op.CallOp) {
	if hosted, _ := gtx.Values[overlayHostValue].(bool); hosted {
		call.Add(gtx.Ops)
		return
	}
	op.Defer(gtx.Ops, call)
}

func placePopover(viewport, anchor image.Rectangle, content image.Point) image.Point {
	maxX := viewport.Max.X - content.X
	maxY := viewport.Max.Y - content.Y
	x := clampCoordinate(anchor.Min.X, viewport.Min.X, maxX)

	below := anchor.Max.Y
	above := anchor.Min.Y - content.Y
	y := below
	switch {
	case below+content.Y <= viewport.Max.Y:
	case above >= viewport.Min.Y:
		y = above
	default:
		y = clampCoordinate(below, viewport.Min.Y, maxY)
	}
	return image.Pt(x, clampCoordinate(y, viewport.Min.Y, maxY))
}

func clampCoordinate(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}

func (p *Popover) Layout(gtx layout.Context, anchor, content layout.Widget) layout.Dimensions {
	anchorDims := anchor(gtx)
	if !p.Open {
		if activePopover == p {
			activePopover = nil
		}
		p.wasOpen = false
		p.contentRect = image.Rectangle{}
		return anchorDims
	}

	opening := !p.wasOpen || activePopover != p
	if activePopover != nil && activePopover != p {
		activePopover.Open = false
		activePopover.wasOpen = false
		activePopover.contentRect = image.Rectangle{}
	}
	activePopover = p
	p.wasOpen = true
	event.Op(gtx.Ops, &p.focusTag)
	if opening || !gtx.Focused(&p.focusTag) {
		gtx.Execute(key.FocusCmd{Tag: &p.focusTag})
	}

	for {
		e, ok := gtx.Event(pointer.Filter{Target: &p.outsideTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		press, ok := e.(pointer.Event)
		if ok && !press.Position.Round().In(p.contentRect) {
			p.close()
			return anchorDims
		}
	}
	if p.dismiss.Clicked(gtx) {
		history := p.dismiss.History()
		if len(history) == 0 || !history[len(history)-1].Position.In(p.contentRect) {
			p.close()
			return anchorDims
		}
	}
	for {
		event, ok := gtx.Event(key.Filter{Focus: &p.focusTag, Name: key.NameEscape})
		if !ok {
			break
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press {
			p.close()
			return anchorDims
		}
	}

	window := gtx.Constraints.Max
	contentCtx := gtx
	contentCtx = overlayContentContext(contentCtx)
	contentCtx.Constraints.Min = image.Point{}
	contentCtx.Constraints.Max.X = max(0, window.X)
	contentCtx.Constraints.Max.Y = max(0, window.Y)
	contentRecord := op.Record(gtx.Ops)
	contentDims := content(contentCtx)
	contentCall := contentRecord.Stop()

	viewport := image.Rect(0, -window.Y, window.X, window.Y)
	anchorRect := image.Rectangle{Max: anchorDims.Size}
	position := placePopover(viewport, anchorRect, contentDims.Size)
	p.contentRect = image.Rectangle{Min: position, Max: position.Add(contentDims.Size)}

	overlay := op.Record(gtx.Ops)
	catcherCtx := gtx
	catcherCtx.Constraints = layout.Exact(window)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.outsideTag)
	p.dismiss.Layout(catcherCtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: window}
	})
	pass.Pop()
	offset := op.Offset(position).Push(gtx.Ops)
	contentCall.Add(gtx.Ops)
	offset.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return anchorDims
}
