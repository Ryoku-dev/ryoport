package kit

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func testContext(width, height int) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(width, height)),
	}
}

func TestRadius(t *testing.T) {
	gtx := testContext(100, 40)
	if got := Radius(gtx, theme.Full, 40); got != 20 {
		t.Fatalf("full radius = %d, want 20", got)
	}
	if got := Radius(gtx, unit.Dp(12), 40); got != 12 {
		t.Fatalf("fixed radius = %d, want 12", got)
	}
}

func TestColumnsChoosesFittingCount(t *testing.T) {
	tests := []struct {
		width int
		want  int
	}{
		{width: 199, want: 1},
		{width: 200, want: 2},
		{width: 320, want: 3},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("width_%d", test.width), func(t *testing.T) {
			gtx := testContext(test.width, 200)
			firstWidth := 0
			Columns(gtx, 96, 8, 6, func(gtx layout.Context, i int) layout.Dimensions {
				if i == 0 {
					firstWidth = gtx.Constraints.Min.X
				}
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 20)}
			})
			got := (test.width + 8) / (firstWidth + 8)
			if got != test.want {
				t.Fatalf("first row has %d columns, want %d", got, test.want)
			}
		})
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
}

func TestButtonGroupChange(t *testing.T) {
	gtx := testContext(200, 40)
	var g ButtonGroup
	g.ensure(3)
	g.buttons[1].Click()
	got, ok := g.Changed(gtx)
	if !ok || got != 1 {
		t.Fatalf("Changed() = %d, %v; want 1, true", got, ok)
	}
}
