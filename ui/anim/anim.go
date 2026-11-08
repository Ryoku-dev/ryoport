// Package anim moves values over time and asks Gio for the next frame only
// while something is still moving, so a settled window draws nothing.
package anim

import (
	"math"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

var reduce atomic.Bool

// SetReduce turns every animation into an instant jump.
func SetReduce(on bool) { reduce.Store(on) }

func Reduced() bool { return reduce.Load() }

// Spring is a critically damped spring toward Target. Omega sets the speed:
// it settles in about 4/Omega seconds (18 is roughly 220 ms).
type Spring struct {
	Omega  float32
	pos    float32
	vel    float32
	target float32
	last   time.Time
	init   bool
}

// Set moves the target. The first call places the spring there directly.
func (s *Spring) Set(target float32) {
	if !s.init {
		s.pos, s.target, s.init = target, target, true
		return
	}
	s.target = target
}

// Snap jumps to v and stops.
func (s *Spring) Snap(v float32) {
	s.pos, s.target, s.vel, s.init = v, v, 0, true
}

func (s *Spring) Target() float32 { return s.target }

func (s *Spring) Moving() bool {
	return math.Abs(float64(s.pos-s.target)) > 0.001 || math.Abs(float64(s.vel)) > 0.001
}

// Value advances the spring to gtx.Now and returns its position.
func (s *Spring) Value(gtx layout.Context) float32 {
	if !s.init {
		return s.target
	}
	now := gtx.Now
	if reduce.Load() || s.last.IsZero() {
		if reduce.Load() {
			s.pos, s.vel = s.target, 0
		}
		s.last = now
		if s.Moving() {
			gtx.Execute(op.InvalidateCmd{})
		}
		return s.pos
	}
	dt := float32(now.Sub(s.last).Seconds())
	s.last = now
	if dt > 0.05 {
		dt = 0.05
	}
	w := s.Omega
	if w <= 0 {
		w = 18
	}
	x0 := s.pos - s.target
	e := float32(math.Exp(float64(-w * dt)))
	s.pos = s.target + (x0+(s.vel+w*x0)*dt)*e
	s.vel = (s.vel - w*(s.vel+w*x0)*dt) * e
	if s.Moving() {
		gtx.Execute(op.InvalidateCmd{})
	} else {
		s.pos, s.vel = s.target, 0
	}
	return s.pos
}

// Tween runs from one value to another over a fixed duration.
type Tween struct {
	Duration time.Duration
	Ease     func(float32) float32
	from     float32
	to       float32
	start    time.Time
	init     bool
}

// Set starts a run from the current value to v.
func (t *Tween) Set(gtx layout.Context, v float32) {
	if !t.init {
		t.from, t.to, t.init = v, v, true
		return
	}
	if v == t.to {
		return
	}
	t.from, t.to, t.start = t.Value(gtx), v, gtx.Now
}

func (t *Tween) Value(gtx layout.Context) float32 {
	if !t.init || reduce.Load() || t.Duration <= 0 || t.start.IsZero() {
		return t.to
	}
	p := float32(gtx.Now.Sub(t.start)) / float32(t.Duration)
	if p >= 1 {
		return t.to
	}
	if p < 0 {
		p = 0
	}
	gtx.Execute(op.InvalidateCmd{})
	ease := t.Ease
	if ease == nil {
		ease = EaseOutCubic
	}
	return t.from + (t.to-t.from)*ease(p)
}

// Phase returns where a looping animation of the given period is, 0 to 1,
// and keeps frames coming. Call it only while the thing should move.
func Phase(gtx layout.Context, period time.Duration) float32 {
	if reduce.Load() || period <= 0 {
		return 0
	}
	gtx.Execute(op.InvalidateCmd{})
	return float32(gtx.Now.UnixNano()%int64(period)) / float32(period)
}

func EaseOutCubic(p float32) float32 {
	q := 1 - p
	return 1 - q*q*q
}

// EaseEmphasized is Material's emphasized decelerate curve, near enough.
func EaseEmphasized(p float32) float32 {
	q := 1 - p
	return 1 - q*q*q*q*q
}

func EaseInOut(p float32) float32 {
	if p < 0.5 {
		return 4 * p * p * p
	}
	q := -2*p + 2
	return 1 - q*q*q/2
}
