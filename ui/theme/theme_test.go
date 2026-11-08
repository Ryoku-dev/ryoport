package theme

import (
	"image/color"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/palette"
)

func TestFromPaletteMapsMaterialRoles(t *testing.T) {
	values := map[string]color.NRGBA{
		"surface":                 rgb(0x010203),
		"surfaceContainerLow":     rgb(0x111213),
		"surfaceContainer":        rgb(0x212223),
		"surfaceContainerHigh":    rgb(0x313233),
		"surfaceContainerHighest": rgb(0x414243),
		"onSurface":               rgb(0xf1f2f3),
		"onSurfaceVariant":        rgb(0xd1d2d3),
		"outlineVariant":          rgb(0x515253),
		"outline":                 rgb(0x616263),
		"inverseSurface":          rgb(0xe1e2e3),
		"inverseOnSurface":        rgb(0x090a0b),
		"primary":                 rgb(0x718293),
		"onPrimary":               rgb(0xfafafa),
		"scrim":                   rgb(0x0c0d0e),
	}
	got := FromPalette(palette.Palette{Roles: values, Dark: true, Source: "wallpaper"})
	checks := map[string]struct {
		got, want color.NRGBA
	}{
		"bg": {got.Bg, values["surface"]}, "panel": {got.Panel, values["surfaceContainerLow"]},
		"raised": {got.Raised, values["surfaceContainer"]}, "overlay": {got.Overlay, values["surfaceContainerHigh"]},
		"selected": {got.Selected, values["surfaceContainerHighest"]}, "ink": {got.Ink, values["onSurface"]},
		"inkDim": {got.InkDim, values["onSurfaceVariant"]}, "line": {got.Line, values["outlineVariant"]},
		"lineStrong": {got.LineStrong, values["outline"]}, "bone": {got.Bone, values["inverseSurface"]},
		"onBone": {got.OnBone, values["inverseOnSurface"]}, "accent": {got.Accent, values["primary"]},
		"onAccent": {got.OnAccent, values["onPrimary"]}, "scrim": {got.Scrim, WithAlpha(values["scrim"], 0.6)},
		"inkFaint": {got.InkFaint, Mix(values["surface"], values["onSurfaceVariant"], 0.62)},
	}
	for name, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %#v, want %#v", name, check.got, check.want)
		}
	}
	if !got.Dark || got.Ok == (color.NRGBA{}) || got.OkSoft == (color.NRGBA{}) {
		t.Fatalf("dark or state inks missing: %#v", got)
	}
}

func TestFromPaletteFallsBackPerKey(t *testing.T) {
	accent := rgb(0x123456)
	got := FromPalette(palette.Palette{Roles: map[string]color.NRGBA{"primary": accent}, Dark: false, Source: "scheme"})
	fallback := Signature(false)
	if got.Accent != accent {
		t.Errorf("accent = %#v, want %#v", got.Accent, accent)
	}
	if got.Bg != fallback.Bg || got.Panel != fallback.Panel || got.Ink != fallback.Ink || got.Scrim != fallback.Scrim {
		t.Errorf("missing roles did not use signature: %#v", got)
	}
	if got.InkFaint != Mix(fallback.Bg, fallback.InkDim, 0.62) {
		t.Errorf("ink faint = %#v", got.InkFaint)
	}

	empty := FromPalette(palette.Palette{Roles: map[string]color.NRGBA{}, Dark: true, Source: "signature"})
	want := Signature(true)
	if empty != want {
		t.Errorf("empty palette = %#v, want %#v", empty, want)
	}
}
