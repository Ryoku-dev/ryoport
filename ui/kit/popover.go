package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

type Popover struct {
	Open        bool
	dismiss     widget.Clickable
	contentRect image.Rectangle
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
		p.contentRect = image.Rectangle{}
		return anchorDims
	}

	if p.dismiss.Clicked(gtx) {
		history := p.dismiss.History()
		if len(history) == 0 || !history[len(history)-1].Position.In(p.contentRect) {
			p.Open = false
			p.contentRect = image.Rectangle{}
			return anchorDims
		}
	}
	for {
		event, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press {
			p.Open = false
			p.contentRect = image.Rectangle{}
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
