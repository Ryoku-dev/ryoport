package kit

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type RowContent struct {
	Mark         layout.Widget
	Title, Facts string
	FactsMono    bool
	Claim, Proof string
	Tone         model.Tone
	Progress     float64
	Action       layout.Widget
}

type Row struct {
	widget.Clickable
	Pane color.NRGBA

	hover     anim.Tween
	notch     anim.Tween
	tone      model.Tone
	toneReady bool

	eventAt    time.Time
	eventReady bool
	clicked    bool
	double     bool
	clickedOut bool
	doubleOut  bool
}

func (r *Row) poll(gtx layout.Context) {
	if r.eventReady && r.eventAt == gtx.Now {
		return
	}
	r.eventAt = gtx.Now
	r.eventReady = true
	r.clicked = false
	r.double = false
	r.clickedOut = false
	r.doubleOut = false
	for {
		click, ok := r.Clickable.Update(gtx)
		if !ok {
			break
		}
		r.clicked = true
		if click.NumClicks >= 2 {
			r.double = true
		}
	}
}

// Clicked and DoubleClicked share one event drain, so a double click can both
// select the row and run its primary action in the same frame.
func (r *Row) Clicked(gtx layout.Context) bool {
	r.poll(gtx)
	if r.clickedOut || !r.clicked {
		return false
	}
	r.clickedOut = true
	return true
}

func (r *Row) DoubleClicked(gtx layout.Context) bool {
	r.poll(gtx)
	if r.doubleOut || !r.double {
		return false
	}
	r.doubleOut = true
	return true
}

func (r *Row) Layout(gtx layout.Context, th *theme.Theme, c RowContent, selected, focused bool) layout.Dimensions {
	r.poll(gtx)
	r.hover.Duration = th.Motion.Hover
	r.hover.Ease = anim.EaseChisel
	r.notch.Duration = th.Motion.Notch
	r.notch.Ease = anim.EaseChisel
	if !r.toneReady {
		r.tone = c.Tone
		r.toneReady = true
		r.notch.Set(gtx, 1)
	} else if r.tone != c.Tone {
		r.tone = c.Tone
		r.notch = anim.Tween{Duration: th.Motion.Notch, Ease: anim.EaseChisel}
		r.notch.Set(gtx, 0)
		r.notch.Set(gtx, 1)
	}

	h := gtx.Dp(th.RowHeight())
	w := gtx.Constraints.Max.X
	if w < gtx.Constraints.Min.X {
		w = gtx.Constraints.Min.X
	}
	rowCtx := gtx
	rowCtx.Constraints = layout.Exact(image.Pt(w, h))
	dims := r.Clickable.Layout(rowCtx, func(gtx layout.Context) layout.Dimensions {
		hot := r.Hovered() || focused
		if hot {
			r.hover.Set(gtx, 1)
		} else {
			r.hover.Set(gtx, 0)
		}
		hover := r.hover.Value(gtx)
		if !selected {
			FillCut(gtx.Ops, image.Rect(0, 0, w, h), 0, theme.Mix(th.R.Stone2, th.R.Stone3, hover))
		}

		notchColor := th.R.Stone(c.Tone)
		if c.Tone == model.ToneIdle {
			notchColor = r.Pane
			if notchColor == (color.NRGBA{}) {
				notchColor = th.R.Stone1
			}
		}
		notchSize := int(math.Round(float64(gtx.Dp(th.Cut.Notch)) * float64(r.notch.Value(gtx))))
		if notchSize > 0 {
			Notch(gtx.Ops, image.Point{}, notchSize, notchColor)
		}

		titleInk, quietInk := th.R.Fg, th.R.Fg3
		claimInk := th.R.Stone(c.Tone)
		if c.Tone == model.ToneIdle {
			claimInk = th.R.Fg2
		}
		if selected {
			titleInk, quietInk, claimInk = th.R.OnSel, th.R.OnSel2, th.R.OnSel
		}

		body := func(gtx layout.Context) layout.Dimensions {
			mark := func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(32), gtx.Dp(32)))
				if c.Mark == nil {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}
				return layout.Center.Layout(gtx, c.Mark)
			}
			copyBlock := func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(Text(th, th.T.Row, titleInk, c.Title)),
				}
				if c.Facts != "" {
					style := th.T.Caption
					if c.FactsMono {
						style = th.T.Mono
					}
					children = append(children, layout.Rigid(Text(th, style, quietInk, c.Facts)))
				}
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
			}
			trailing := func(gtx layout.Context) layout.Dimensions {
				if hot && c.Action != nil {
					return c.Action(gtx)
				}
				gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(72))
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Label{Style: th.T.Label, Color: claimInk, MaxLines: 1, Align: text.End}.Layout(gtx, th, c.Claim)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Label{Style: th.T.MonoSm, Color: quietInk, MaxLines: 1, Align: text.End}.Layout(gtx, th, c.Proof)
					}),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(mark),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, copyBlock),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(trailing),
			)
		}
		layout.Inset{Left: 16, Right: 12}.Layout(gtx, body)

		if c.Progress >= 0 {
			frac := max(0, min(1, c.Progress))
			bladeW := int(math.Round(float64(w) * frac))
			if bladeW > 0 {
				Blade(gtx.Ops, image.Rect(0, h-max(1, gtx.Dp(2)), bladeW, h), th.R.Lapis)
			}
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	})
	return dims
}
