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
		Bg:         role("surface", fallback.Bg),
		Panel:      role("surfaceContainerLow", fallback.Panel),
		Raised:     role("surfaceContainer", fallback.Raised),
		Overlay:    role("surfaceContainerHigh", fallback.Overlay),
		Selected:   role("surfaceContainerHighest", fallback.Selected),
		Ink:        role("onSurface", fallback.Ink),
		InkDim:     role("onSurfaceVariant", fallback.InkDim),
		Line:       role("outlineVariant", fallback.Line),
		LineStrong: role("outline", fallback.LineStrong),
		Bone:       role("inverseSurface", fallback.Bone),
		OnBone:     role("inverseOnSurface", fallback.OnBone),
		Accent:     role("primary", fallback.Accent),
		OnAccent:   role("onPrimary", fallback.OnAccent),
		Scrim:      fallback.Scrim,
		Dark:       p.Dark,
	}
	r.InkFaint = Mix(r.Bg, r.InkDim, 0.62)
	if scrim, ok := p.Roles["scrim"]; ok {
		r.Scrim = WithAlpha(scrim, 0.6)
	}
	r.StateInks()
	return r
}
