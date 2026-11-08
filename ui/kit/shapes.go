package kit

import (
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoport/internal/model"
)

type Shape int

const (
	SoftSquare Shape = iota
	Cookie
	Burst
	Clover
	Circle
)

const shapeSteps = 96

func shapeRadius(shape Shape, theta float64) float64 {
	switch shape {
	case SoftSquare:
		c, s := math.Abs(math.Cos(theta)), math.Abs(math.Sin(theta))
		return 1 / math.Pow((math.Pow(c, 4)+math.Pow(s, 4))*2, .25)
	case Cookie:
		return .86 + .14*math.Cos(8*theta)
	case Burst:
		v := .5 + .5*math.Cos(10*theta)
		return .70 + .30*v*v
	case Clover:
		return .78 + .22*math.Cos(4*theta)
	default:
		return 1
	}
}

func shapePoint(shape Shape, theta float64) f32.Point {
	r := shapeRadius(shape, theta)
	return f32.Pt(float32(r*math.Cos(theta)), float32(r*math.Sin(theta)))
}

func morphPoint(from, to Shape, theta float64, t float32) f32.Point {
	a, b := shapeRadius(from, theta), shapeRadius(to, theta)
	r := a + (b-a)*float64(t)
	return f32.Pt(float32(r*math.Cos(theta)), float32(r*math.Sin(theta)))
}

func ShapePath(ops *op.Ops, center f32.Point, radius float32, from, to Shape, t, rotation float32) clip.PathSpec {
	t = max(0, min(1, t))
	var p clip.Path
	p.Begin(ops)
	for i := range shapeSteps + 1 {
		theta := 2*math.Pi*float64(i)/shapeSteps + float64(rotation)
		pt := morphPoint(from, to, theta, t)
		pt.X = center.X + pt.X*radius
		pt.Y = center.Y + pt.Y*radius
		if i == 0 {
			p.MoveTo(pt)
		} else {
			p.LineTo(pt)
		}
	}
	return p.End()
}

func ShapeFor(t model.Tone) Shape {
	switch t {
	case model.ToneAlive:
		return Cookie
	case model.ToneBusy:
		return Burst
	case model.ToneAttention:
		return Clover
	case model.TonePaused:
		return Circle
	default:
		return SoftSquare
	}
}
