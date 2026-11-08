package kit

import (
	"image"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type SheetKind int

const (
	Dialog SheetKind = iota
	Side
)

type Sheet struct {
	Open      bool
	close     IconButton
	scrim     widget.Clickable
	body      layout.List
	enter     anim.Tween
	wasOpen   bool
	closed    bool
	sheetRect image.Rectangle
}

func (s *Sheet) dismiss() {
	s.Open = false
	s.closed = true
}

func (s *Sheet) Layout(gtx layout.Context, th *theme.Theme, kind SheetKind, title string, body layout.Widget, actions ...layout.Widget) layout.Dimensions {
	window := gtx.Constraints.Max
	if s.Open && s.close.Clicked(gtx) {
		s.dismiss()
	}
	if s.Open && s.scrim.Clicked(gtx) {
		history := s.scrim.History()
		if len(history) == 0 {
			s.dismiss()
		} else {
			pos := history[len(history)-1].Position
			if !image.Pt(int(pos.X), int(pos.Y)).In(s.sheetRect) {
				s.dismiss()
			}
		}
	}
	if s.Open {
		for {
			e, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if e, ok := e.(key.Event); ok && e.State == key.Press {
				s.dismiss()
				break
			}
		}
	}
	if !s.Open {
		s.wasOpen = false
		return layout.Dimensions{Size: window}
	}
	if !s.wasOpen {
		s.enter = anim.Tween{Duration: 220 * time.Millisecond, Ease: anim.EaseEmphasized}
		s.enter.Set(gtx, 0)
		s.enter.Set(gtx, 1)
		s.wasOpen = true
	}
	progress := s.enter.Value(gtx)
	overlay := op.Record(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	scrimCtx := gtx
	scrimCtx.Constraints.Min = window
	pass := pointer.PassOp{}.Push(gtx.Ops)
	s.scrim.Layout(scrimCtx, func(gtx layout.Context) layout.Dimensions {
		paint.Fill(gtx.Ops, th.R.Scrim)
		return layout.Dimensions{Size: window}
	})
	pass.Pop()
	width := min(window.X, gtx.Dp(560))
	height := min(window.Y, gtx.Dp(680))
	x, y := (window.X-width)/2, (window.Y-height)/2
	if kind == Side {
		width, height = min(window.X, gtx.Dp(440)), window.Y
		x, y = window.X-width, 0
	}
	y += int(float32(gtx.Dp(16)) * (1 - progress))
	s.sheetRect = image.Rect(x, y, x+width, y+height)
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	r := image.Rect(0, 0, width, height)
	Shadow(gtx, r, gtx.Dp(th.Shape.XL), 4)
	if kind == Side {
		paint.FillShape(gtx.Ops, th.R.Overlay, clip.RRect{Rect: r, NW: gtx.Dp(24), SW: gtx.Dp(24)}.Op(gtx.Ops))
	} else {
		FillRect(gtx.Ops, r, gtx.Dp(24), th.R.Overlay)
	}
	sheetCtx := gtx
	sheetCtx.Constraints.Min = r.Max
	sheetCtx.Constraints.Max = r.Max
	layout.Inset{Top: 20, Bottom: 16, Left: 24, Right: 24}.Layout(sheetCtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, Text(th, th.T.Title, th.R.Ink, title)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return s.close.Layout(gtx, th, Standard, icons.Close, "Close")
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				s.body.Axis = layout.Vertical
				return s.body.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions { return body(gtx) })
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(actions))
				for _, action := range actions {
					children = append(children, layout.Rigid(action))
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
				}
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				})
			}),
		)
	})
	off.Pop()
	opacity.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return layout.Dimensions{Size: window}
}

func (s *Sheet) Closed(layout.Context) bool {
	closed := s.closed
	s.closed = false
	return closed
}
