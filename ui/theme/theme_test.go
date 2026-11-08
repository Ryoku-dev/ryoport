package theme

import (
	"image/color"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/palette"
)

func TestFromPaletteMapsLithicRoles(t *testing.T) {
	values := map[string]color.NRGBA{
		"surfaceContainerLowest":  rgb(0x010203),
		"surface":                 rgb(0x111213),
		"surfaceContainerLow":     rgb(0x212223),
		"surfaceContainer":        rgb(0x313233),
		"surfaceContainerHigh":    rgb(0x414243),
		"surfaceContainerHighest": rgb(0x515253),
		"onSurface":               rgb(0xf1f2f3),
		"onSurfaceVariant":        rgb(0xd1d2d3),
		"outline":                 rgb(0x717273),
		"outlineVariant":          rgb(0x616263),
		"inverseSurface":          rgb(0xe1e2e3),
		"inverseOnSurface":        rgb(0x090a0b),
		"scrim":                   rgb(0x0c0d0e),
	}
	tests := []struct {
		name string
		dark bool
		want map[string]color.NRGBA
	}{
		{
			name: "dark",
			dark: true,
			want: map[string]color.NRGBA{
				"ground": values["surfaceContainerLowest"], "stone1": values["surface"],
				"stone2": values["surfaceContainerLow"], "stone3": values["surfaceContainer"],
				"stone4": values["surfaceContainerHigh"], "vein": values["surfaceContainerHighest"],
				"scrim": WithAlpha(values["scrim"], 0.62),
			},
		},
		{
			name: "light",
			want: map[string]color.NRGBA{
				"ground": values["surfaceContainerHighest"], "stone1": values["surfaceContainerHigh"],
				"stone2": values["surfaceContainerLowest"], "stone3": values["surfaceContainer"],
				"stone4": values["surfaceContainerLowest"], "vein": values["outlineVariant"],
				"scrim": WithAlpha(values["scrim"], 0.45),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := FromPalette(palette.Palette{Roles: values, Dark: test.dark, Source: "wallpaper"})
			actual := map[string]color.NRGBA{
				"ground": got.Ground, "stone1": got.Stone1, "stone2": got.Stone2,
				"stone3": got.Stone3, "stone4": got.Stone4, "vein": got.Vein,
				"scrim": got.Scrim,
			}
			for name, want := range test.want {
				if actual[name] != want {
					t.Errorf("%s = %#v, want %#v", name, actual[name], want)
				}
			}
			if got.Fg != values["onSurface"] || got.Fg2 != values["onSurfaceVariant"] || got.Fg3 != values["outline"] {
				t.Errorf("foreground roles = %#v, %#v, %#v", got.Fg, got.Fg2, got.Fg3)
			}
			if got.Sel != values["inverseSurface"] || got.OnSel != values["inverseOnSurface"] {
				t.Errorf("selection roles = %#v, %#v", got.Sel, got.OnSel)
			}
			if got.OnSel2 != Mix(values["inverseOnSurface"], values["inverseSurface"], 0.45) {
				t.Errorf("quiet selection ink = %#v", got.OnSel2)
			}
			fixed := Signature(test.dark)
			if got.Gold != fixed.Gold || got.Jade != fixed.Jade || got.Lapis != fixed.Lapis || got.Amber != fixed.Amber || got.Jasper != fixed.Jasper {
				t.Errorf("fixed Lithic colours were not set: %#v", got)
			}
		})
	}
}

func TestFromPaletteFallsBackPerKey(t *testing.T) {
	foreground := rgb(0x123456)
	got := FromPalette(palette.Palette{
		Roles:  map[string]color.NRGBA{"onSurface": foreground},
		Dark:   true,
		Source: "scheme",
	})
	fallback := Signature(true)
	if got.Fg != foreground {
		t.Errorf("fg = %#v, want %#v", got.Fg, foreground)
	}
	if got.Ground != fallback.Ground || got.Stone1 != fallback.Stone1 ||
		got.Stone2 != fallback.Stone2 || got.Stone3 != fallback.Stone3 ||
		got.Stone4 != fallback.Stone4 || got.Vein != fallback.Vein ||
		got.Fg2 != fallback.Fg2 || got.Fg3 != fallback.Fg3 ||
		got.Sel != fallback.Sel || got.OnSel != fallback.OnSel ||
		got.Scrim != fallback.Scrim {
		t.Errorf("missing roles did not use the signature: %#v", got)
	}
	if got.OnSel2 != Mix(fallback.OnSel, fallback.Sel, 0.45) {
		t.Errorf("quiet selection ink = %#v", got.OnSel2)
	}

	empty := FromPalette(palette.Palette{Roles: map[string]color.NRGBA{}, Dark: false, Source: "signature"})
	want := Signature(false)
	if empty != want {
		t.Errorf("empty palette = %#v, want %#v", empty, want)
	}
}
