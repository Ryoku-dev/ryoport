package kit

import (
	"math"
	"testing"
)

func TestMapScrollDelta(t *testing.T) {
	tests := []struct {
		name      string
		delta     float32
		pxPerDP   float32
		x11       bool
		want      float32
		wantNotch bool
	}{
		{name: "Wayland notch", delta: 150, pxPerDP: 2, want: 240, wantNotch: true},
		{name: "Wayland two notches", delta: 300, pxPerDP: 1.5, want: 360, wantNotch: true},
		{name: "Wayland negative notch", delta: -150, pxPerDP: 2, want: -240, wantNotch: true},
		{name: "Wayland notch tolerance", delta: 150.4, pxPerDP: 2, want: 240, wantNotch: true},
		{name: "Wayland fractional touchpad", delta: 10.25, pxPerDP: 2, want: 20.5},
		{name: "Wayland rejects X11 notch", delta: 10, pxPerDP: 2, want: 20},
		{name: "X11 notch", delta: 10, pxPerDP: 1.5, x11: true, want: 180, wantNotch: true},
		{name: "X11 negative two notches", delta: -20, pxPerDP: 2, x11: true, want: -480, wantNotch: true},
		{name: "X11 fractional passthrough", delta: -3.25, pxPerDP: 2, x11: true, want: -6.5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, notch := mapScrollDelta(test.delta, test.pxPerDP, test.x11)
			if notch != test.wantNotch {
				t.Fatalf("notch = %v, want %v", notch, test.wantNotch)
			}
			if math.Abs(float64(got-test.want)) > 0.001 {
				t.Fatalf("pixels = %v, want %v", got, test.want)
			}
		})
	}
}
