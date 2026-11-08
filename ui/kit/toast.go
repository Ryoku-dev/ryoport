package kit

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type toastEntry struct {
	id      int
	text    string
	action  string
	expires time.Time
	button  widget.Clickable
}

type Toasts struct {
	next    int
	entries []*toastEntry
}

func (t *Toasts) Push(now time.Time, text, action string) int {
	t.next++
	t.entries = append(t.entries, &toastEntry{id: t.next, text: text, action: action, expires: now.Add(5 * time.Second)})
	if len(t.entries) > 3 {
		t.entries = t.entries[len(t.entries)-3:]
	}
	return t.next
}

func (t *Toasts) prune(now time.Time) {
	kept := t.entries[:0]
	for _, entry := range t.entries {
		if now.Before(entry.expires) {
			kept = append(kept, entry)
		}
	}
	t.entries = kept
}

func (t *Toasts) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	t.prune(gtx.Now)
	window := gtx.Constraints.Max
	if len(t.entries) == 0 {
		return layout.Dimensions{Size: window}
	}
	earliest := t.entries[0].expires
	for _, entry := range t.entries[1:] {
		if entry.expires.Before(earliest) {
			earliest = entry.expires
		}
	}
	gtx.Execute(op.InvalidateCmd{At: earliest})
	rec := op.Record(gtx.Ops)
	toastCtx := gtx
	toastCtx.Constraints.Min = image.Point{}
	toastCtx.Constraints.Max.X = min(window.X, gtx.Dp(560))
	children := make([]layout.FlexChild, 0, len(t.entries)*2)
	for _, entry := range t.entries {
		entry := entry
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Surface(gtx, th.R.Bone, th.Shape.M, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(48)
				return layout.Inset{Left: 16, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, Text(th, th.T.Body, th.R.OnBone, entry.text)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if entry.action == "" {
								return layout.Dimensions{}
							}
							return entry.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, Text(th, th.T.Label, th.R.OnBone, entry.action))
							})
						}),
					)
				})
			})
		}))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(toastCtx, children...)
	call := rec.Stop()
	overlay := op.Record(gtx.Ops)
	off := op.Offset(image.Pt((window.X-dims.Size.X)/2, max(0, window.Y-dims.Size.Y-gtx.Dp(24)))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return layout.Dimensions{Size: window}
}

func (t *Toasts) Action(gtx layout.Context) (int, bool) {
	for i, entry := range t.entries {
		if entry.action != "" && entry.button.Clicked(gtx) {
			id := entry.id
			t.entries = append(t.entries[:i], t.entries[i+1:]...)
			return id, true
		}
	}
	return 0, false
}
