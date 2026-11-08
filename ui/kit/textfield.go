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
	reveal       widget.Clickable
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
			layout.Rigid(Text(th, th.T.Caption, disabledInk(gtx, th.R.InkDim), label)),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		if w <= 0 {
			w = gtx.Dp(240)
		}
		h := gtx.Dp(48)
		size := gtx.Constraints.Constrain(image.Pt(w, h))
		r := image.Rectangle{Max: size}
		bg := th.R.Raised
		if gtx.Focused(&f.Editor) {
			bg = theme.Mix(bg, th.R.Ink, 0.04)
		}
		FillRect(gtx.Ops, r, gtx.Dp(th.Shape.M), disabledInk(gtx, bg))
		outline := th.R.LineStrong
		width := 0
		if gtx.Focused(&f.Editor) {
			width = max(1, gtx.Dp(2))
		}
		if errText != "" {
			outline, width = th.R.Bad, max(1, gtx.Dp(2))
		}
		if width > 0 {
			StrokeRect(gtx.Ops, r.Inset(width/2), max(0, gtx.Dp(th.Shape.M)-width/2), width, disabledInk(gtx, outline))
		}
		inner := size.X - gtx.Dp(24)
		if f.Secret {
			inner -= gtx.Dp(40)
		}
		editorGTX := gtx
		editorGTX.Constraints = layout.Exact(image.Pt(max(0, inner), h-gtx.Dp(16)))
		off := op.Offset(image.Pt(gtx.Dp(12), gtx.Dp(8))).Push(gtx.Ops)
		layoutEditor(editorGTX, th, &f.Editor, style, placeholder)
		off.Pop()
		if f.Secret {
			icon := icons.Visibility
			desc := "Show value"
			if f.revealed {
				icon, desc = icons.VisibilityOff, "Hide value"
			}
			buttonGTX := exact(gtx, image.Pt(gtx.Dp(40), gtx.Dp(40)))
			off := op.Offset(image.Pt(size.X-gtx.Dp(44), gtx.Dp(4))).Push(gtx.Ops)
			b := IconButton{Clickable: f.reveal}
			b.Layout(buttonGTX, th, Standard, icon, desc)
			f.reveal = b.Clickable
			off.Pop()
		}
		return layout.Dimensions{Size: size}
	}))
	message := help
	messageColor := th.R.InkDim
	if errText != "" {
		message, messageColor = errText, th.R.Bad
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
	textCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Ink))
	selectionCall := textMaterial(gtx.Ops, disabledInk(gtx, th.R.Selected))
	m := op.Record(gtx.Ops)
	hint := Label{Style: style, Color: disabledInk(gtx, th.R.InkFaint), MaxLines: 1}.Layout(gtx, th, placeholder)
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
