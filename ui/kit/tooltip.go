package kit

import (
	"image"
	"sync"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type tooltipState struct {
	tag     struct{}
	hovered bool
	since   time.Time
}

type tooltipKey struct {
	ops *op.Ops
	tip string
}

var tooltipStates = struct {
	sync.Mutex
	m map[tooltipKey]*tooltipState
}{m: make(map[tooltipKey]*tooltipState)}

func Tooltip(gtx layout.Context, th *theme.Theme, tip string, w layout.Widget) layout.Dimensions {
	key := tooltipKey{ops: gtx.Ops, tip: tip}
	tooltipStates.Lock()
	state := tooltipStates.m[key]
	if state == nil {
		state = &tooltipState{}
		tooltipStates.m[key] = state
	}
	tooltipStates.Unlock()
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &state.tag, Kinds: pointer.Enter | pointer.Leave})
		if !ok {
			break
		}
		pe := e.(pointer.Event)
		if pe.Kind == pointer.Enter {
			state.hovered, state.since = true, gtx.Now
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(600 * time.Millisecond)})
		} else if pe.Kind == pointer.Leave {
			state.hovered = false
		}
	}
	dims := w(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &state.tag)
	area.Pop()
	pass.Pop()
	if !state.hovered || gtx.Now.Sub(state.since) < 600*time.Millisecond || tip == "" {
		return dims
	}
	rec := op.Record(gtx.Ops)
	tipCtx := gtx
	tipCtx.Constraints.Min = image.Point{}
	tipCtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(320))
	tipDims := Surface(tipCtx, th.R.Overlay, th.Shape.S, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, Text(th, th.T.Caption, th.R.Ink, tip))
	})
	call := rec.Stop()
	overlay := op.Record(gtx.Ops)
	x := max(0, min(gtx.Constraints.Max.X-tipDims.Size.X, (dims.Size.X-tipDims.Size.X)/2))
	y := dims.Size.Y + gtx.Dp(8)
	if y+tipDims.Size.Y > gtx.Constraints.Max.Y {
		y = -tipDims.Size.Y - gtx.Dp(8)
	}
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return dims
}
