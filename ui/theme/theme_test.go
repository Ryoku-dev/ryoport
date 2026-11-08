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
		"primary":                 rgb(0x6750a4),
		"onPrimary":               rgb(0xffffff),
		"secondaryContainer":      rgb(0xe8def8),
		"onSecondaryContainer":    rgb(0x1d192b),
		"error":                   rgb(0xba1a1a),
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
			if got.Sel != values["secondaryContainer"] || got.OnSel != values["onSecondaryContainer"] {
				t.Errorf("selection roles = %#v, %#v", got.Sel, got.OnSel)
			}
			if got.OnSel2 != Mix(values["onSecondaryContainer"], values["secondaryContainer"], 0.45) {
				t.Errorf("quiet selection ink = %#v", got.OnSel2)
			}
			fixed := Signature(test.dark)
			if got.Gold != values["primary"] || got.OnGold != values["onPrimary"] ||
				got.Jasper != values["error"] || got.Jade != fixed.Jade ||
				got.Lapis != fixed.Lapis || got.Amber != fixed.Amber {
				t.Errorf("matugen accents were not mapped: %#v", got)
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

func TestContrastToneReachesAccessibleContrast(t *testing.T) {
	tests := []struct {
		name       string
		ink, stone color.NRGBA
		lighter    bool
	}{
		{"dark stone", rgb(0x3d3d3d), rgb(0x1b1b1b), true},
		{"light stone", rgb(0xd0d0d0), rgb(0xf4f1eb), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := contrastTone(test.ink, test.stone, test.lighter)
			if ratio := contrastRatio(got, test.stone); ratio < 4.5 {
				t.Fatalf("contrast = %.2f for %#v", ratio, got)
			}
			if test.lighter && relativeLuminance(got) <= relativeLuminance(test.ink) {
				t.Errorf("dark-mode ink did not get lighter: %#v", got)
			}
			if !test.lighter && relativeLuminance(got) >= relativeLuminance(test.ink) {
				t.Errorf("light-mode ink did not get darker: %#v", got)
			}
		})
	}
}

func TestFromPaletteForcedModeKeepsSignatureNeutralsAndRetonesAccents(t *testing.T) {
	tests := []struct {
		name    string
		dark    bool
		primary color.NRGBA
	}{
		{"dark", true, rgb(0x3d3d3d)},
		{"light", false, rgb(0xd0d0d0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := rgb(0x76558f)
			got := FromPalette(palette.Palette{
				Roles: map[string]color.NRGBA{
					"surface":              rgb(0xff00ff),
					"primary":              test.primary,
					"onPrimary":            rgb(0xffffff),
					"secondaryContainer":   selection,
					"onSecondaryContainer": rgb(0xffffff),
				},
				Dark:   test.dark,
				Forced: true,
				Source: "wallpaper",
			})
			signature := Signature(test.dark)
			if got.Ground != signature.Ground || got.Stone1 != signature.Stone1 ||
				got.Stone2 != signature.Stone2 || got.Fg != signature.Fg {
				t.Errorf("forced neutrals did not stay on the signature: %#v", got)
			}
			if got.Gold != test.primary || got.Sel != selection {
				t.Errorf("forced accents were dropped: %#v", got)
			}
			if got.GoldInk == test.primary || contrastRatio(got.GoldInk, got.Stone2) < 4.5 {
				t.Errorf("forced gold ink was not re-toned: %#v", got.GoldInk)
			}
		})
	}
}

func TestFromPaletteUsesEditionAccentOutsideRyoku(t *testing.T) {
	accent := rgb(0x1793d1)
	got := FromPalette(palette.Palette{
		Roles:  map[string]color.NRGBA{"primary": accent},
		Dark:   true,
		Source: "signature",
	})
	signature := Signature(true)
	if got.Gold != accent || got.OnGold != highestContrastInk(accent) {
		t.Errorf("edition gold = %#v on %#v", got.Gold, got.OnGold)
	}
	if got.Sel != signature.Sel || got.OnSel != signature.OnSel ||
		got.Jade != signature.Jade || got.Jasper != signature.Jasper {
		t.Errorf("edition accent changed selection or states: %#v", got)
	}
	if contrastRatio(got.GoldInk, got.Stone2) < 4.5 {
		t.Errorf("edition gold ink contrast = %.2f", contrastRatio(got.GoldInk, got.Stone2))
	}
}
