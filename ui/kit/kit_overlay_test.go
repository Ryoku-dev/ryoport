package kit

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

// popoverScene draws a window-sized clickable, then a popover anchored at
// (12, 168) the way the rail draws the New menu, with one clickable item.
type popoverScene struct {
	router input.Router
	pop    Popover
	under  Click
	item   Click
}

func (s *popoverScene) frame() layout.Context {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      s.router.Source(),
		Constraints: layout.Exact(image.Pt(800, 600)),
		Now:         time.Now(),
	}
	s.under.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	offset := op.Offset(image.Pt(12, 168)).Push(ops)
	anchorCtx := gtx
	anchorCtx.Constraints.Min = image.Point{}
	s.pop.Layout(anchorCtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(64, 64)}
	}, func(gtx layout.Context) layout.Dimensions {
		return s.item.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(200, 120)}
		})
	})
	offset.Pop()
	s.router.Frame(ops)
	return gtx
}

func (s *popoverScene) press(at f32.Point) {
	s.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

func TestPopoverKeepsPressesInsideItsContent(t *testing.T) {
	var scene popoverScene
	scene.pop.Open = true
	scene.frame()
	// The item sits 8 px under the 64 px anchor: about (12, 240) to (212, 360).
	scene.press(f32.Pt(80, 300))
	gtx := scene.frame()
	if !scene.pop.Open {
		t.Fatal("a press on the popover's item closed the popover")
	}
	if !scene.item.Clicked(gtx) {
		t.Fatal("the popover's item did not receive the click")
	}
	if scene.under.Clicked(gtx) {
		t.Fatal("a press on the popover also reached the widget underneath")
	}
}

func TestPopoverOutsidePressClosesAndStops(t *testing.T) {
	var scene popoverScene
	scene.pop.Open = true
	scene.frame()
	scene.press(f32.Pt(600, 500))
	gtx := scene.frame()
	if scene.pop.Open {
		t.Fatal("a press outside the popover left it open")
	}
	if scene.under.Clicked(gtx) {
		t.Fatal("the press that closed the popover also clicked what was underneath")
	}
}

func TestDialogProgrammaticCloseCompletesAfterExit(t *testing.T) {
	var (
		dialog Dialog
		router input.Router
	)
	th := theme.New(theme.Signature(true), nil)
	frame := func(now time.Time) layout.Context {
		ops := new(op.Ops)
		gtx := layout.Context{
			Ops:         ops,
			Source:      router.Source(),
			Constraints: layout.Exact(image.Pt(800, 600)),
			Now:         now,
		}
		dialog.Layout(gtx, th, "", func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 80)}
		})
		router.Frame(ops)
		return gtx
	}

	now := time.Unix(100, 0)
	dialog.Open = true
	frame(now)
	dialog.Open = false
	closing := frame(now.Add(10 * time.Millisecond))
	if dialog.Closed(closing) {
		t.Fatal("programmatic close reported before the exit completed")
	}
	closed := frame(now.Add(250 * time.Millisecond))
	if !dialog.Closed(closed) {
		t.Fatal("programmatic close did not report after the exit completed")
	}
}

func TestPlacardExpiryOrderingAndStacking(t *testing.T) {
	base := time.Unix(100, 0)
	var placards Placards
	first := placards.Push(base, "first", "")
	second := placards.Push(base.Add(time.Second), "second", "")
	third := placards.Push(base.Add(2*time.Second), "third", "")
	fourth := placards.Push(base.Add(3*time.Second), "fourth", "")
	if got, want := placardIDs(placards.entries), []int{second, third, fourth}; !equalInts(got, want) {
		t.Fatalf("stack after fourth push = %v, want %v; oldest id was %d", got, want, first)
	}

	placards.entries = prunePlacards(placards.entries, base.Add(6500*time.Millisecond))
	if got, want := placardIDs(placards.entries), []int{third, fourth}; !equalInts(got, want) {
		t.Fatalf("stack after expiry = %v, want %v", got, want)
	}
}

func TestPopoverPlacementFlipsAndClamps(t *testing.T) {
	viewport := image.Rect(0, 0, 300, 300)
	for _, test := range []struct {
		name    string
		anchor  image.Rectangle
		content image.Point
		want    image.Point
	}{
		{
			name:    "below",
			anchor:  image.Rect(50, 40, 150, 80),
			content: image.Pt(100, 60),
			want:    image.Pt(50, 88),
		},
		{
			name:    "above when below does not fit",
			anchor:  image.Rect(60, 250, 160, 280),
			content: image.Pt(80, 100),
			want:    image.Pt(60, 142),
		},
		{
			name:    "horizontal clamp",
			anchor:  image.Rect(280, 40, 295, 80),
			content: image.Pt(80, 60),
			want:    image.Pt(220, 88),
		},
		{
			name:    "vertical clamp when neither side fits",
			anchor:  image.Rect(20, 120, 80, 150),
			content: image.Pt(100, 240),
			want:    image.Pt(20, 60),
		},
		{
			name:    "oversize content starts at viewport",
			anchor:  image.Rect(20, 20, 80, 50),
			content: image.Pt(400, 400),
			want:    image.Pt(0, 0),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := placePopover(viewport, test.anchor, test.content, 8); got != test.want {
				t.Fatalf("placePopover() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestConfirmRequiresExactTypedText(t *testing.T) {
	for _, test := range []struct {
		input, required string
		want            bool
	}{
		{input: "", required: "", want: true},
		{input: "windows-11", required: "windows-11", want: true},
		{input: "Windows-11", required: "windows-11", want: false},
		{input: "windows-11 ", required: "windows-11", want: false},
		{input: "windows-1", required: "windows-11", want: false},
	} {
		if got := confirmReady(test.input, test.required); got != test.want {
			t.Fatalf("confirmReady(%q, %q) = %v, want %v", test.input, test.required, got, test.want)
		}
	}
}

func placardIDs(entries []*placardEntry) []int {
	ids := make([]int, len(entries))
	for i, entry := range entries {
		ids[i] = entry.id
	}
	return ids
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
