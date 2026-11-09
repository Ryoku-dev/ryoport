package theme

import (
	"image/color"
	"math"

	"github.com/ryoku-dev/ryoport/internal/palette"
)

func FromPalette(p palette.Palette) Roles {
	r := Signature(p.Dark)
	if p.Source != "wallpaper" && p.Source != "scheme" {
		accent, ok := p.Roles["primary"]
		if !ok {
			return r
		}
		r.Gold = accent
		r.OnGold = InkOn(accent)
		r.GoldInk = contrastTone(accent, r.Stone2, p.Dark)
		return r
	}

	role := func(name string, value color.NRGBA) color.NRGBA {
		if resolved, ok := p.Roles[name]; ok {
			return resolved
		}
		return value
	}
	if !p.Forced {
		r.Fg = role("onSurface", r.Fg)
		r.Fg2 = role("onSurfaceVariant", r.Fg2)
		r.Fg3 = role("outline", r.Fg3)
		if p.Dark {
			r.Ground = role("surfaceContainerLowest", r.Ground)
			r.Stone1 = role("surface", r.Stone1)
			r.Stone2 = role("surfaceContainerLow", r.Stone2)
			r.Stone3 = role("surfaceContainer", r.Stone3)
			r.Stone4 = role("surfaceContainerHigh", r.Stone4)
			r.Vein = role("surfaceContainerHighest", r.Vein)
			if scrim, ok := p.Roles["scrim"]; ok {
				r.Scrim = WithAlpha(scrim, 0.62)
			}
		} else {
			r.Ground = role("surfaceContainerHighest", r.Ground)
			r.Stone1 = role("surfaceContainerHigh", r.Stone1)
			r.Stone2 = role("surfaceContainerLowest", r.Stone2)
			r.Stone3 = role("surfaceContainer", r.Stone3)
			r.Stone4 = role("surfaceContainerLowest", r.Stone4)
			r.Vein = role("outlineVariant", r.Vein)
			if scrim, ok := p.Roles["scrim"]; ok {
				r.Scrim = WithAlpha(scrim, 0.45)
			}
		}
	}

	r.Gold = role("primary", r.Gold)
	r.OnGold = role("onPrimary", r.OnGold)
	r.GoldInk = contrastTone(r.Gold, r.Stone2, p.Dark)
	r.Sel = role("secondaryContainer", r.Sel)
	r.OnSel = role("onSecondaryContainer", r.OnSel)
	r.OnSel2 = Mix(r.OnSel, r.Sel, 0.45)
	// The palette's error tone comes from the desktop's own mode, so a
	// forced light window could get a pale pink meant for dark stone. Tone it
	// like gold ink so a Stop or Remove always reads at 4.5:1.
	r.Jasper = contrastTone(role("error", r.Jasper), r.Stone2, p.Dark)
	return r
}

// InkOn is black or white, whichever reads better on background.
func InkOn(background color.NRGBA) color.NRGBA {
	black := rgb(0x000000)
	white := rgb(0xffffff)
	if contrastRatio(black, background) >= contrastRatio(white, background) {
		return black
	}
	return white
}

func contrastTone(ink, background color.NRGBA, lighter bool) color.NRGBA {
	if contrastRatio(ink, background) >= 4.5 {
		return ink
	}
	hue, saturation, lightness := rgbToHSL(ink)
	for step := 1; step <= 255; step++ {
		at := float64(step) / 255
		next := lightness * (1 - at)
		if lighter {
			next = lightness + (1-lightness)*at
		}
		candidate := hslToNRGBA(hue, saturation, next, ink.A)
		if contrastRatio(candidate, background) >= 4.5 {
			return candidate
		}
	}
	if lighter {
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: ink.A}
	}
	return color.NRGBA{A: ink.A}
}

func contrastRatio(a, b color.NRGBA) float64 {
	lighter := relativeLuminance(a)
	darker := relativeLuminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(c color.NRGBA) float64 {
	channel := func(value uint8) float64 {
		v := float64(value) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func rgbToHSL(c color.NRGBA) (hue, saturation, lightness float64) {
	red, green, blue := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	highest := max(red, green, blue)
	lowest := min(red, green, blue)
	lightness = (highest + lowest) / 2
	delta := highest - lowest
	if delta == 0 {
		return 0, 0, lightness
	}
	saturation = delta / (1 - math.Abs(2*lightness-1))
	switch highest {
	case red:
		hue = math.Mod((green-blue)/delta, 6)
	case green:
		hue = (blue-red)/delta + 2
	default:
		hue = (red-green)/delta + 4
	}
	hue /= 6
	if hue < 0 {
		hue++
	}
	return hue, saturation, lightness
}

func hslToNRGBA(hue, saturation, lightness float64, alpha uint8) color.NRGBA {
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue*6, 2)-1))
	offset := lightness - chroma/2
	var red, green, blue float64
	switch sector := int(hue * 6); sector {
	case 0:
		red, green = chroma, x
	case 1:
		red, green = x, chroma
	case 2:
		green, blue = chroma, x
	case 3:
		green, blue = x, chroma
	case 4:
		red, blue = x, chroma
	default:
		red, blue = chroma, x
	}
	channel := func(value float64) uint8 {
		return uint8(math.Round(min(max(value+offset, 0), 1) * 255))
	}
	return color.NRGBA{R: channel(red), G: channel(green), B: channel(blue), A: alpha}
}
