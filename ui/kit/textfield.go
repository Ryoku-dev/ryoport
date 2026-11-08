package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type TextField struct {
	widget.Editor
	Secret, Mono bool
	reveal       IconButton
	revealed     bool
}

func (f *TextField) Layout(gtx layout.Context, th *theme.Theme, label, placeholder, help, errText string) layout.Dimensions {
	if f.reveal.Clicked(gtx) {
		f.revealed = !f.revealed
	}
	f.SingleLine = true
	f.Mask = 0
	if f.Secret && !f.revealed {
		f.Mask = '•'
	}
	style := th.T.Body
	if f.Mono {
		style = th.T.Mono
	}

	children := make([]layout.FlexChild, 0, 5)
	if label != "" {
		children = append(children,
			layout.Rigid(Text(th, th.T.Caption, disabledInk(gtx, th.R.Fg3), label)),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		if w <= 0 {
			w = gtx.Dp(240)
		}
		h := gtx.Dp(44)
		size := gtx.Constraints.Constrain(image.Pt(w, h))
		r := image.Rectangle{Max: size}
		cut := gtx.Dp(th.Cut.Mid)
		FillCut(gtx.Ops, r, cut, disabledInk(gtx, th.R.Stone3))
		outline := th.R.Fg2
		if errText != "" {
			outline = th.R.Jasper
		}
		if gtx.Focused(&f.Editor) || errText != "" {
			StrokeCut(gtx.Ops, r, cut, max(1, gtx.Dp(1)), disabledInk(gtx, outline))
		}

		innerWidth := max(0, size.X-gtx.Dp(28))
		if f.Secret {
			innerWidth = max(0, innerWidth-gtx.Dp(36))
		}
		editorHeight := max(0, size.Y-gtx.Dp(16))
		editorGTX := exact(gtx, image.Pt(innerWidth, editorHeight))
		offset := op.Offset(image.Pt(gtx.Dp(14), (size.Y-editorHeight)/2)).Push(gtx.Ops)
		layoutEditor(editorGTX, th, &f.Editor, style, placeholder)
		offset.Pop()

		if f.Secret {
			icon, desc := icons.Visibility, "Show value"
			if f.revealed {
				icon, desc = icons.VisibilityOff, "Hide value"
			}
			f.reveal.Small = true
			buttonSize := gtx.Dp(32)
			offset := op.Offset(image.Pt(size.X-buttonSize-gtx.Dp(6), (size.Y-buttonSize)/2)).Push(gtx.Ops)
			f.reveal.Layout(exact(gtx, image.Pt(buttonSize, buttonSize)), th, Quiet, icon, desc)
			offset.Pop()
		}
		return layout.Dimensions{Size: size}
	}))

	message, messageColor := help, th.R.Fg3
	if errText != "" {
		message, messageColor = errText, th.R.Jasper
	}
	if message != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(Text(th, th.T.Caption, disabledInk(gtx, messageColor), message)),
		)
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
}

func layoutEditor(gtx layout.Context, th *theme.Theme, editor *widget.Editor, style theme.TextStyle, placeholder string) layout.Dimensions {
	textCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Fg))
	selectionCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Sel))
	m := op.Record(gtx.Ops)
	hint := Label{Style: style, Color: disabledInk(gtx, th.R.Fg3), MaxLines: 1}.Layout(gtx, th, placeholder)
	hintCall := m.Stop()
	editor.LineHeight = style.Line
	dims := editor.Layout(gtx, th.Shaper, style.Face, style.Size, textCall, selectionCall)
	if editor.Len() == 0 {
		hintCall.Add(gtx.Ops)
		if hint.Size.X > dims.Size.X {
			dims.Size.X = hint.Size.X
		}
	}
	return dims
}
