package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

type Popover struct {
	Open        bool
	dismiss     widget.Clickable
	contentRect image.Rectangle
}

func (p *Popover) Layout(gtx layout.Context, anchor layout.Widget, content layout.Widget) layout.Dimensions {
	anchorDims := anchor(gtx)
	if !p.Open {
		return anchorDims
	}
	if p.dismiss.Clicked(gtx) {
		history := p.dismiss.History()
		if len(history) == 0 {
			p.Open = false
			return anchorDims
		}
		pos := history[len(history)-1].Position
		if !image.Pt(int(pos.X), int(pos.Y)).In(p.contentRect) {
			p.Open = false
			return anchorDims
		}
	}
	for {
		e, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := e.(key.Event); ok && e.State == key.Press {
			p.Open = false
			return anchorDims
		}
	}
	contentCtx := gtx
	contentCtx.Constraints.Min = image.Point{}
	contentCtx.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y)
	contentRec := op.Record(gtx.Ops)
	contentDims := content(contentCtx)
	contentCall := contentRec.Stop()
	x := 0
	y := anchorDims.Size.Y
	if y+contentDims.Size.Y > gtx.Constraints.Max.Y {
		y = -contentDims.Size.Y
	}
	x = max(0, min(x, gtx.Constraints.Max.X-contentDims.Size.X))
	p.contentRect = image.Rect(x, y, x+contentDims.Size.X, y+contentDims.Size.Y)
	overlay := op.Record(gtx.Ops)
	backCtx := gtx
	backCtx.Constraints.Min = gtx.Constraints.Max
	pass := pointer.PassOp{}.Push(gtx.Ops)
	p.dismiss.Layout(backCtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	pass.Pop()
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	contentCall.Add(gtx.Ops)
	off.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return anchorDims
}
