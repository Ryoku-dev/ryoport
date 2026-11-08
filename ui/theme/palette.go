package theme

import (
	"image/color"

	"github.com/ryoku-dev/ryoport/internal/palette"
)

func FromPalette(p palette.Palette) Roles {
	fallback := Signature(p.Dark)
	if len(p.Roles) == 0 {
		return fallback
	}
	role := func(name string, value color.NRGBA) color.NRGBA {
		if resolved, ok := p.Roles[name]; ok {
			return resolved
		}
		return value
	}

	r := Roles{
		Fg:    role("onSurface", fallback.Fg),
		Fg2:   role("onSurfaceVariant", fallback.Fg2),
		Fg3:   role("outline", fallback.Fg3),
		Sel:   role("inverseSurface", fallback.Sel),
		OnSel: role("inverseOnSurface", fallback.OnSel),
		Scrim: fallback.Scrim,
		Dark:  p.Dark,
	}
	if p.Dark {
		r.Ground = role("surfaceContainerLowest", fallback.Ground)
		r.Stone1 = role("surface", fallback.Stone1)
		r.Stone2 = role("surfaceContainerLow", fallback.Stone2)
		r.Stone3 = role("surfaceContainer", fallback.Stone3)
		r.Stone4 = role("surfaceContainerHigh", fallback.Stone4)
		r.Vein = role("surfaceContainerHighest", fallback.Vein)
		if scrim, ok := p.Roles["scrim"]; ok {
			r.Scrim = WithAlpha(scrim, 0.62)
		}
	} else {
		r.Ground = role("surfaceContainerHighest", fallback.Ground)
		r.Stone1 = role("surfaceContainerHigh", fallback.Stone1)
		r.Stone2 = role("surfaceContainerLowest", fallback.Stone2)
		r.Stone3 = role("surfaceContainer", fallback.Stone3)
		r.Stone4 = role("surfaceContainerLowest", fallback.Stone4)
		r.Vein = role("outlineVariant", fallback.Vein)
		if scrim, ok := p.Roles["scrim"]; ok {
			r.Scrim = WithAlpha(scrim, 0.45)
		}
	}
	r.OnSel2 = Mix(r.OnSel, r.Sel, 0.45)
	r.SetStones()
	return r
}
