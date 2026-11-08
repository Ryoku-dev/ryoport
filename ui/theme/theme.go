// Package theme is Ryoport's design tokens: colour roles mapped from the
// live Ryoku palette, the type scale, the shape and spacing scales, and the
// embedded fonts. docs/design.md explains the choices.
package theme

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
)

// Roles are the colours components ask for. They never name a hue: the
// palette decides those, and only the state inks are fixed.
type Roles struct {
	Bg       color.NRGBA // the window
	Panel    color.NRGBA // list pane, cards
	Raised   color.NRGBA // hover, idle seals
	Overlay  color.NRGBA // menus, sheets, the palette
	Selected color.NRGBA // the selected row

	Ink      color.NRGBA
	InkDim   color.NRGBA
	InkFaint color.NRGBA

	Line       color.NRGBA // dividers and table rules
	LineStrong color.NRGBA // field outlines on focus

	Bone   color.NRGBA // the primary action, selection, the rail indicator
	OnBone color.NRGBA

	Accent   color.NRGBA // focus ring, chart lines, links
	OnAccent color.NRGBA

	Scrim color.NRGBA

	Ok, Warn, Bad, Busy                 color.NRGBA // state inks
	OkSoft, WarnSoft, BadSoft, BusySoft color.NRGBA // their containers, for seals and pills

	Dark bool
}

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

// WithAlpha returns c at alpha a (0..1) of its own alpha.
func WithAlpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * a)
	return c
}

// Mix blends a toward b by t (0..1), alpha included.
func Mix(a, b color.NRGBA, t float32) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t + 0.5) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: l(a.A, b.A)}
}

// StateInks fills the fixed state colours for dark or light paper, and their
// soft containers against the panel.
func (r *Roles) StateInks() {
	if r.Dark {
		r.Ok, r.Warn, r.Bad, r.Busy = rgb(0x7fb59a), rgb(0xd9a65b), rgb(0xe2342a), rgb(0x8ea6c9)
	} else {
		r.Ok, r.Warn, r.Bad, r.Busy = rgb(0x2f7a57), rgb(0x8f5f14), rgb(0xc0281f), rgb(0x3d5f8f)
	}
	soft := float32(0.20)
	if !r.Dark {
		soft = 0.16
	}
	r.OkSoft = Mix(r.Panel, r.Ok, soft)
	r.WarnSoft = Mix(r.Panel, r.Warn, soft)
	r.BadSoft = Mix(r.Panel, r.Bad, soft)
	r.BusySoft = Mix(r.Panel, r.Busy, soft)
}

// Signature is Ryoku's own palette, used when no live palette is around:
// black paper and bone ink, or the same inverted for light.
func Signature(dark bool) Roles {
	var r Roles
	if dark {
		r = Roles{
			Bg: rgb(0x000000), Panel: rgb(0x0b0b0a), Raised: rgb(0x151514),
			Overlay: rgb(0x1b1a19), Selected: rgb(0x22211f),
			Ink: rgb(0xcdc4ba), InkDim: rgb(0xb0a9a0), InkFaint: rgb(0x7a756e),
			Line: rgb(0x2c2a27), LineStrong: rgb(0x5e5a54),
			Bone: rgb(0xcdc4ba), OnBone: rgb(0x000000),
			Accent: rgb(0xcdc4ba), OnAccent: rgb(0x000000),
			Scrim: color.NRGBA{A: 0x99}, Dark: true,
		}
	} else {
		r = Roles{
			Bg: rgb(0xf4f1ec), Panel: rgb(0xece8e1), Raised: rgb(0xe3ded6),
			Overlay: rgb(0xf8f6f2), Selected: rgb(0xd9d3ca),
			Ink: rgb(0x1d1b18), InkDim: rgb(0x4a4640), InkFaint: rgb(0x7d776f),
			Line: rgb(0xd2ccc3), LineStrong: rgb(0x8f897f),
			Bone: rgb(0x1d1b18), OnBone: rgb(0xf4f1ec),
			Accent: rgb(0x1d1b18), OnAccent: rgb(0xf4f1ec),
			Scrim: color.NRGBA{A: 0x66},
		}
	}
	r.StateInks()
	return r
}

type TextStyle struct {
	Face font.Font
	Size unit.Sp
	Line unit.Sp
}

// Type is the type scale. Space Grotesk carries everything a person reads,
// Space Mono only literal data like addresses, ports and paths.
type Type struct {
	Display  TextStyle // the harbour sentence
	Headline TextStyle // the detail title
	Title    TextStyle // card and sheet titles
	Row      TextStyle // row names
	Body     TextStyle
	Label    TextStyle // buttons, tabs, chips
	Caption  TextStyle // row facts, secondary text
	Group    TextStyle // list group headers
	Mono     TextStyle
	MonoSm   TextStyle
}

const (
	Sans = "Space Grotesk"
	Mono = "Space Mono"
)

func DefaultType() Type {
	sans := func(w font.Weight, size, line float32) TextStyle {
		return TextStyle{Face: font.Font{Typeface: Sans, Weight: w}, Size: unit.Sp(size), Line: unit.Sp(line)}
	}
	mono := func(size, line float32) TextStyle {
		return TextStyle{Face: font.Font{Typeface: Mono}, Size: unit.Sp(size), Line: unit.Sp(line)}
	}
	return Type{
		Display:  sans(font.Medium, 40, 46),
		Headline: sans(font.Medium, 26, 32),
		Title:    sans(font.Medium, 18, 24),
		Row:      sans(font.Medium, 15, 20),
		Body:     sans(font.Normal, 14, 20),
		Label:    sans(font.Medium, 13, 16),
		Caption:  sans(font.Normal, 12, 16),
		Group:    sans(font.Medium, 11, 14),
		Mono:     mono(13, 18),
		MonoSm:   mono(12, 16),
	}
}

// Shape is the corner scale. Full asks for half the height.
type Shape struct{ XS, S, M, L, XL unit.Dp }

const Full unit.Dp = -1

func DefaultShape() Shape { return Shape{XS: 4, S: 8, M: 12, L: 16, XL: 24} }

// Space is Ryoku's spacing scale.
type Space struct{ S1, S2, S3, S4, S5, S6, S7 unit.Dp }

func DefaultSpace() Space { return Space{4, 8, 12, 16, 24, 32, 48} }

type Density int

const (
	Comfortable Density = iota
	Compact
)

type Theme struct {
	R       Roles
	T       Type
	Shape   Shape
	Sp      Space
	Density Density
	// Shaper holds the embedded fonts. It belongs to the window goroutine.
	Shaper *text.Shaper
}

// New builds a theme around roles and a shaper from Shaper().
func New(r Roles, shaper *text.Shaper) *Theme {
	return &Theme{R: r, T: DefaultType(), Shape: DefaultShape(), Sp: DefaultSpace(), Shaper: shaper}
}

func (th *Theme) RowHeight() unit.Dp {
	if th.Density == Compact {
		return 52
	}
	return 64
}

// StateInk returns the ink and container for a tone index from model.Tone,
// passed as an int so this package stays free of the model.
func (th *Theme) StateInk(tone int) (ink, soft color.NRGBA) {
	switch tone {
	case 1: // alive
		return th.R.Ok, th.R.OkSoft
	case 2: // busy
		return th.R.Busy, th.R.BusySoft
	case 3, 5: // attention, paused
		return th.R.Warn, th.R.WarnSoft
	case 4: // bad
		return th.R.Bad, th.R.BadSoft
	}
	return th.R.InkDim, th.R.Raised
}
