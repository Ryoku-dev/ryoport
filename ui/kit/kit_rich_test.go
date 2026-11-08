package kit

import (
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

func TestShapeRadiiNormalisedAndContinuous(t *testing.T) {
	for _, shape := range []Shape{SoftSquare, Cookie, Burst, Clover, Circle} {
		maximum := 0.0
		for i := range 4096 {
			r := shapeRadius(shape, 2*math.Pi*float64(i)/4096)
			if r <= 0 || r > 1+1e-9 {
				t.Fatalf("shape %d radius outside (0,1]: %g", shape, r)
			}
			maximum = max(maximum, r)
		}
		if math.Abs(maximum-1) > 1e-6 {
			t.Fatalf("shape %d maximum radius = %g, want 1", shape, maximum)
		}
		if math.Abs(shapeRadius(shape, 0)-shapeRadius(shape, 2*math.Pi)) > 1e-9 {
			t.Fatalf("shape %d is discontinuous at the wrap", shape)
		}
	}
}

func TestMorphEndpointsMatchShapes(t *testing.T) {
	for _, pair := range [][2]Shape{{SoftSquare, Cookie}, {Cookie, Burst}, {Burst, Clover}, {Clover, Circle}} {
		for i := range shapeSteps {
			theta := 2 * math.Pi * float64(i) / shapeSteps
			from, to := shapePoint(pair[0], theta), shapePoint(pair[1], theta)
			atFrom, atTo := morphPoint(pair[0], pair[1], theta, 0), morphPoint(pair[0], pair[1], theta, 1)
			if atFrom != from || atTo != to {
				t.Fatalf("morph %d to %d changed an endpoint at step %d", pair[0], pair[1], i)
			}
		}
	}
}

func TestMeterInkThresholds(t *testing.T) {
	roles := theme.Signature(true)
	ok, warn, bad := roles.Ok, roles.Warn, roles.Bad
	th := &theme.Theme{R: roles}
	for _, tc := range []struct {
		frac float64
		want color.NRGBA
	}{{0, ok}, {.699, ok}, {.7, warn}, {.899, warn}, {.9, bad}, {1, bad}} {
		if got := MeterInk(th, tc.frac); got != tc.want {
			t.Fatalf("MeterInk(%g) = %#v, want %#v", tc.frac, got, tc.want)
		}
	}
}

func TestToastExpiryPreservesOrdering(t *testing.T) {
	base := time.Unix(100, 0)
	var toasts Toasts
	first := toasts.Push(base, "first", "")
	second := toasts.Push(base.Add(time.Second), "second", "")
	third := toasts.Push(base.Add(2*time.Second), "third", "")
	toasts.prune(base.Add(5500 * time.Millisecond))
	if len(toasts.entries) != 2 || toasts.entries[0].id != second || toasts.entries[1].id != third {
		t.Fatalf("after first expiry got ids %v, want [%d %d] (expired %d)", toastIDs(toasts.entries), second, third, first)
	}
}

func toastIDs(entries []*toastEntry) []int {
	ids := make([]int, len(entries))
	for i, entry := range entries {
		ids[i] = entry.id
	}
	return ids
}
