package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Search struct {
	widget.Editor
	clear widget.Clickable
}

func (s *Search) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &s.Editor})
}

func (s *Search) Layout(gtx layout.Context, th *theme.Theme, placeholder, hint string) layout.Dimensions {
	s.SingleLine = true
	for {
		e, ok := gtx.Event(key.Filter{Focus: &s.Editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			s.SetText("")
		}
	}
	if s.clear.Clicked(gtx) {
		s.SetText("")
		s.Focus(gtx)
	}
	w := gtx.Constraints.Max.X
	if w <= 0 {
		w = gtx.Dp(280)
	}
	h := gtx.Dp(48)
	size := gtx.Constraints.Constrain(image.Pt(w, h))
	r := image.Rectangle{Max: size}
	bg := th.R.Raised
	if gtx.Focused(&s.Editor) {
		bg = theme.Mix(bg, th.R.Ink, 0.04)
	}
	FillRect(gtx.Ops, r, h/2, disabledInk(gtx, bg))
	if gtx.Focused(&s.Editor) {
		FocusRing(gtx, th, r, h/2)
	}
	is := gtx.Dp(20)
	off := op.Offset(image.Pt(gtx.Dp(14), (h-is)/2)).Push(gtx.Ops)
	icons.Search.Layout(exact(gtx, image.Pt(is, is)), disabledInk(gtx, th.R.InkDim))
	off.Pop()

	right := gtx.Dp(12)
	if s.Len() > 0 {
		right = gtx.Dp(48)
	} else if hint != "" && !gtx.Focused(&s.Editor) {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Left: 8, Right: 8, Top: 4, Bottom: 4}.Layout(gtx, Text(th, th.T.Caption, disabledInk(gtx, th.R.InkDim), hint))
		call := m.Stop()
		right = d.Size.X + gtx.Dp(12)
		hintRect := image.Rect(size.X-right, (h-d.Size.Y)/2, size.X-gtx.Dp(8), (h+d.Size.Y)/2)
		FillRect(gtx.Ops, hintRect, d.Size.Y/2, disabledInk(gtx, th.R.Selected))
		o := op.Offset(hintRect.Min).Push(gtx.Ops)
		call.Add(gtx.Ops)
		o.Pop()
	}
	innerW := max(0, size.X-gtx.Dp(46)-right)
	editorGTX := exact(gtx, image.Pt(innerW, h-gtx.Dp(16)))
	off = op.Offset(image.Pt(gtx.Dp(42), gtx.Dp(8))).Push(gtx.Ops)
	layoutEditor(editorGTX, th, &s.Editor, th.T.Body, placeholder)
	off.Pop()
	if s.Len() > 0 {
		buttonGTX := exact(gtx, image.Pt(gtx.Dp(40), gtx.Dp(40)))
		off = op.Offset(image.Pt(size.X-gtx.Dp(44), gtx.Dp(4))).Push(gtx.Ops)
		b := IconButton{Clickable: s.clear}
		b.Layout(buttonGTX, th, Standard, icons.Close, "Clear search")
		s.clear = b.Clickable
		off.Pop()
	}
	return layout.Dimensions{Size: size}
}
