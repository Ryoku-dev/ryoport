package kit

import (
	"image"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
)

func testContext(width, height int) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(width, height)),
	}
}

func TestCutPoints(t *testing.T) {
	got := cutPoints(image.Rect(10, 20, 110, 70), 5, 10, 15, 20)
	want := [...]f32.Point{
		f32.Pt(15, 20),
		f32.Pt(100, 20),
		f32.Pt(110, 30),
		f32.Pt(110, 55),
		f32.Pt(95, 70),
		f32.Pt(30, 70),
		f32.Pt(10, 50),
		f32.Pt(10, 25),
	}
	if got != want {
		t.Fatalf("cut points = %v, want %v", got, want)
	}
}

func TestBladePoints(t *testing.T) {
	got := bladePoints(image.Rect(0, 0, 100, 6))
	want := [...]f32.Point{
		f32.Pt(0, 0),
		f32.Pt(94, 0),
		f32.Pt(100, 6),
		f32.Pt(0, 6),
	}
	if got != want {
		t.Fatalf("blade points = %v, want %v", got, want)
	}
}

func TestOctagonPoints(t *testing.T) {
	points := OctagonPoints(f32.Pt(4, 7), 10)
	if len(points) != 8 {
		t.Fatalf("octagon has %d vertices, want 8", len(points))
	}
	if points[0].Y != -3 || points[1].Y != -3 {
		t.Fatalf("top side is at %v and %v, want apothem y -3", points[0].Y, points[1].Y)
	}
	if points[2].X != 14 || points[3].X != 14 {
		t.Fatalf("right side is at %v and %v, want apothem x 14", points[2].X, points[3].X)
	}
	wantOffset := 10 * math.Tan(math.Pi/8)
	if math.Abs(float64(points[1].X-4)-wantOffset) > 0.0001 {
		t.Fatalf("vertex offset = %v, want %v", points[1].X-4, wantOffset)
	}
}

func TestColumnCount(t *testing.T) {
	tests := []struct {
		width int
		want  int
	}{
		{width: 199, want: 1},
		{width: 200, want: 2},
		{width: 320, want: 3},
	}
	for _, test := range tests {
		if got := columnCount(test.width, 96, 8, 6); got != test.want {
			t.Errorf("columnCount(%d) = %d, want %d", test.width, got, test.want)
		}
	}
}

func TestStepperClamp(t *testing.T) {
	s := Stepper{Value: 12, Min: 2, Max: 9}
	s.clamp()
	if s.Value != 9 {
		t.Fatalf("high value clamped to %d, want 9", s.Value)
	}
	s.Value = -4
	s.clamp()
	if s.Value != 2 {
		t.Fatalf("low value clamped to %d, want 2", s.Value)
	}
	s.Min, s.Max, s.Value = 7, 3, 5
	s.clamp()
	if s.Max != 7 || s.Value != 7 {
		t.Fatalf("inverted range clamped to max=%d value=%d, want 7 and 7", s.Max, s.Value)
	}
}

func TestFiltersChanged(t *testing.T) {
	gtx := testContext(200, 40)
	var filters Filters
	filters.ensure(3)
	filters.buttons[1].Click()
	got, ok := filters.Changed(gtx)
	if !ok || got != 1 {
		t.Fatalf("Changed() = %d, %v; want 1, true", got, ok)
	}
	if _, ok := filters.Changed(gtx); ok {
		t.Fatal("Changed() reported the same change twice")
	}
}
