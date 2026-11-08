package kit

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

const placardLifetime = 5 * time.Second

type placardEntry struct {
	id      int
	text    string
	action  string
	expires time.Time
	button  widget.Clickable
	enter   anim.Tween
	started bool
	pending bool
}

type Placards struct {
	next    int
	entries []*placardEntry
}

func (p *Placards) Push(now time.Time, text, action string) int {
	p.next++
	p.entries = append(p.entries, &placardEntry{
		id:      p.next,
		text:    text,
		action:  action,
		expires: now.Add(placardLifetime),
	})
	if len(p.entries) > 3 {
		p.entries = p.entries[len(p.entries)-3:]
	}
	return p.next
}

func prunePlacards(entries []*placardEntry, now time.Time) []*placardEntry {
	kept := entries[:0]
	for _, entry := range entries {
		if now.Before(entry.expires) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func (p *Placards) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	p.entries = prunePlacards(p.entries, gtx.Now)
	window := gtx.Constraints.Max
	if len(p.entries) == 0 {
		return layout.Dimensions{Size: window}
	}

	earliest := p.entries[0].expires
	for _, entry := range p.entries[1:] {
		if entry.expires.Before(earliest) {
			earliest = entry.expires
		}
	}
	gtx.Execute(op.InvalidateCmd{At: earliest})
	if !anim.Reduced() {
		gtx.Execute(op.InvalidateCmd{})
	}

	placardCtx := gtx
	placardCtx.Constraints.Min = image.Point{}
	placardCtx.Constraints.Max.X = min(window.X, gtx.Dp(560))
	placardCtx.Constraints.Max.Y = window.Y
	record := op.Record(gtx.Ops)
	children := make([]layout.FlexChild, 0, len(p.entries)*2-1)
	for i, entry := range p.entries {
		entry := entry
		if entry.action != "" && entry.button.Clicked(gtx) {
			entry.pending = true
		}
		if !entry.started {
			entry.enter = anim.Tween{Duration: th.Motion.Overlay, Ease: anim.EaseChisel}
			entry.enter.Set(gtx, 0)
			entry.enter.Set(gtx, 1)
			entry.started = true
		}
		progress := entry.enter.Value(gtx)
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: 8}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := gtx.Dp(48)
			width := gtx.Constraints.Max.X
			gtx.Constraints.Min = image.Pt(width, height)
			gtx.Constraints.Max = image.Pt(width, height)
			opacity := paint.PushOpacity(gtx.Ops, progress)
			rise := int(float32(gtx.Dp(16)) * (1 - progress))
			offset := op.Offset(image.Pt(0, rise)).Push(gtx.Ops)
			r := image.Rect(0, 0, width, height)
			cut := gtx.Dp(8)
			FillCut(gtx.Ops, r, cut, th.R.Sel)
			StrokeCut(gtx.Ops, r, cut, max(1, gtx.Dp(1)), th.R.Vein)
			layout.Inset{Left: 16, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, Text(th, th.T.Body, th.R.OnSel, entry.text)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if entry.action == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return entry.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									actionHeight := gtx.Dp(32)
									labelRecord := op.Record(gtx.Ops)
									labelDims := layout.Inset{Top: 8, Bottom: 8, Left: 13, Right: 13}.Layout(gtx, Text(th, th.T.Label, th.R.OnSel, entry.action))
									labelCall := labelRecord.Stop()
									width := labelDims.Size.X
									actionRect := image.Rect(0, 0, width, actionHeight)
									if entry.button.Hovered() {
										FillCut(gtx.Ops, actionRect, gtx.Dp(th.Cut.Small), theme.WithAlpha(th.R.OnSel, .08))
									}
									StrokeCut(gtx.Ops, actionRect, gtx.Dp(th.Cut.Small), max(1, gtx.Dp(1)), th.R.OnSel)
									labelCall.Add(gtx.Ops)
									return layout.Dimensions{Size: actionRect.Max}
								})
							})
						}),
					)
				})
			})
			remaining := float64(entry.expires.Sub(gtx.Now)) / float64(placardLifetime)
			remaining = max(0, min(1, remaining))
			bladeWidth := int(float64(width) * remaining)
			if bladeWidth > 0 {
				Blade(gtx.Ops, image.Rect(0, height-max(1, gtx.Dp(2)), bladeWidth, height), th.R.OnSel)
			}
			offset.Pop()
			opacity.Pop()
			return layout.Dimensions{Size: r.Max}
		}))
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(placardCtx, children...)
	call := record.Stop()

	overlay := op.Record(gtx.Ops)
	position := image.Pt((window.X-dims.Size.X)/2, max(0, window.Y-dims.Size.Y-gtx.Dp(24)))
	offset := op.Offset(position).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	op.Defer(gtx.Ops, overlay.Stop())
	return layout.Dimensions{Size: window}
}

func (p *Placards) Action(gtx layout.Context) (int, bool) {
	for i, entry := range p.entries {
		clicked := entry.pending
		entry.pending = false
		if !clicked && entry.action != "" {
			clicked = entry.button.Clicked(gtx)
		}
		if clicked {
			id := entry.id
			p.entries = append(p.entries[:i], p.entries[i+1:]...)
			return id, true
		}
	}
	return 0, false
}
