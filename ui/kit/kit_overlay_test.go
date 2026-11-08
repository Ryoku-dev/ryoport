package kit

import (
	"image"
	"testing"
	"time"
)

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
			want:    image.Pt(50, 80),
		},
		{
			name:    "above when below does not fit",
			anchor:  image.Rect(60, 250, 160, 280),
			content: image.Pt(80, 100),
			want:    image.Pt(60, 150),
		},
		{
			name:    "horizontal clamp",
			anchor:  image.Rect(280, 40, 295, 80),
			content: image.Pt(80, 60),
			want:    image.Pt(220, 80),
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
			if got := placePopover(viewport, test.anchor, test.content); got != test.want {
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
