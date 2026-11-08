package kit

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Stepper struct {
	Value, Min, Max, Step int
	minus, plus           widget.Clickable
	changed               bool
}

func (s *Stepper) clamp() {
	if s.Max < s.Min {
		s.Max = s.Min
	}
	s.Value = min(max(s.Value, s.Min), s.Max)
}

func (s *Stepper) Layout(gtx layout.Context, th *theme.Theme, unit string) layout.Dimensions {
	s.clamp()
	step := s.Step
	if step <= 0 {
		step = 1
	}
	if s.minus.Clicked(gtx) && s.Value > s.Min {
		s.Value = max(s.Min, s.Value-step)
		s.changed = true
	}
	if s.plus.Clicked(gtx) && s.Value < s.Max {
		s.Value = min(s.Max, s.Value+step)
		s.changed = true
	}
	minusGTX, plusGTX := gtx, gtx
	if s.Value <= s.Min {
		minusGTX = minusGTX.Disabled()
	}
	if s.Value >= s.Max {
		plusGTX = plusGTX.Disabled()
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(12)}.Layout(gtx,
		layout.Rigid(func(layout.Context) layout.Dimensions {
			b := IconButton{Clickable: s.minus}
			d := b.Layout(minusGTX, th, TonalIcon, icons.Remove, "Decrease")
			s.minus = b.Clickable
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := strconv.Itoa(s.Value)
			if unit != "" {
				value += " " + unit
			}
			return layout.Inset{Left: 4, Right: 4}.Layout(gtx, Text(th, th.T.Row, th.R.Ink, value))
		}),
		layout.Rigid(func(layout.Context) layout.Dimensions {
			b := IconButton{Clickable: s.plus}
			d := b.Layout(plusGTX, th, TonalIcon, icons.Add, "Increase")
			s.plus = b.Clickable
			return d
		}),
	)
}

func (s *Stepper) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}
