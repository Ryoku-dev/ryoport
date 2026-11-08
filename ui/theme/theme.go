// Package theme is Ryoport's design tokens for Lithic: colour roles mapped
// from the live Ryoku palette, the stones and the gold, the type scale, the
// cut and spacing scales, and the embedded fonts. docs/design.md explains
// the choices.
package theme

import (
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
)

// Roles are the colours components ask for. The neutrals come from the
// palette; the stones and the gold are Lithic's own.
type Roles struct {
	Ground color.NRGBA // behind the panes
	Stone1 color.NRGBA // the list pane and the rail
	Stone2 color.NRGBA // the work pane and row tablets
	Stone3 color.NRGBA // hover, fields, tracks
	Stone4 color.NRGBA // menus and dialogs
	Vein   color.NRGBA // hairlines and quiet outlines

	Fg, Fg2, Fg3 color.NRGBA // type from strong to faint

	Sel, OnSel, OnSel2 color.NRGBA // bone: where you are, its ink, its quiet ink

	Gold, OnGold, GoldInk color.NRGBA // gold leaf, type on it, gold type on stone

	Jade, Lapis, Amber, Jasper color.NRGBA // alive, becoming, needs you, failed

	Scrim color.NRGBA

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

// SetStones fills the fixed stones and the gold for dark or light stone.
func (r *Roles) SetStones() {
	if r.Dark {
		r.Jade, r.Lapis, r.Amber, r.Jasper = rgb(0x7ab8a0), rgb(0x8aa2d6), rgb(0xe0954a), rgb(0xe0553a)
		r.Gold, r.OnGold, r.GoldInk = rgb(0xc9a861), rgb(0x17120a), rgb(0xd6b877)
	} else {
		r.Jade, r.Lapis, r.Amber, r.Jasper = rgb(0x2f7a64), rgb(0x3d5f99), rgb(0x94520f), rgb(0xb8321e)
		r.Gold, r.OnGold, r.GoldInk = rgb(0xc9a45a), rgb(0x1d1b18), rgb(0x85651f)
	}
}

// Stone is the ink of a tone. Idle has no stone, so it returns the vein.
func (r *Roles) Stone(t model.Tone) color.NRGBA {
	switch t {
	case model.ToneAlive:
		return r.Jade
	case model.ToneBusy:
		return r.Lapis
	case model.ToneAttention, model.TonePaused:
		return r.Amber
	case model.ToneBad:
		return r.Jasper
	}
	return r.Vein
}

// Field is the mineral field behind the header of a thing in tone t.
func (r *Roles) Field(t model.Tone) color.NRGBA {
	switch t {
	case model.ToneAlive:
		return Mix(r.Stone2, r.Jade, 0.17)
	case model.ToneBusy, model.ToneAttention, model.TonePaused, model.ToneBad:
		return Mix(r.Stone2, r.Stone(t), 0.15)
	}
	return r.Stone3
}

// Signature is Lithic's own stone, used when no Ryoku palette is around.
func Signature(dark bool) Roles {
	var r Roles
	if dark {
		r = Roles{
			Ground: rgb(0x0e0e0e), Stone1: rgb(0x141414), Stone2: rgb(0x1b1b1b),
			Stone3: rgb(0x232323), Stone4: rgb(0x2c2c2c), Vein: rgb(0x393939),
			Fg: rgb(0xe3e3e3), Fg2: rgb(0xc6c6c6), Fg3: rgb(0x8f8f8f),
			Sel: rgb(0xe3e3e3), OnSel: rgb(0x121212), OnSel2: rgb(0x474747),
			Scrim: color.NRGBA{A: 0x9e}, Dark: true,
		}
	} else {
		r = Roles{
			Ground: rgb(0xe2ddd4), Stone1: rgb(0xeae6df), Stone2: rgb(0xf4f1eb),
			Stone3: rgb(0xe9e5de), Stone4: rgb(0xfbf9f5), Vein: rgb(0xcbc4b8),
			Fg: rgb(0x1d1b18), Fg2: rgb(0x4a4640), Fg3: rgb(0x6f6a62),
			Sel: rgb(0x1d1b18), OnSel: rgb(0xf4f1eb), OnSel2: rgb(0xb9b2a7),
			Scrim: color.NRGBA{A: 0x73},
		}
	}
	r.SetStones()
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
	Name     TextStyle // the selected thing's name, the one big line
	Sentence TextStyle // the fleet sentence in the view from above
	Section  TextStyle // page titles and large section heads
	Title    TextStyle // section titles inside a pane
	Row      TextStyle // row names
	Body     TextStyle
	Proof    TextStyle // the sentence under a name
	Label    TextStyle // buttons, tabs, filters
	Caption  TextStyle // facts, secondary lines
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
		Name:     sans(font.Bold, 52, 56),
		Sentence: sans(font.Bold, 34, 40),
		Section:  sans(font.Bold, 21, 28),
		Title:    sans(font.Medium, 15, 20),
		Row:      sans(font.Medium, 15, 20),
		Body:     sans(font.Normal, 14, 20),
		Proof:    sans(font.Normal, 15, 21),
		Label:    sans(font.Medium, 13, 16),
		Caption:  sans(font.Normal, 12, 16),
		Group:    sans(font.Medium, 12, 16),
		Mono:     mono(12, 16),
		MonoSm:   mono(11, 15),
	}
}

// Cut is the size of a 45 degree corner cut for each scale of thing.
type Cut struct {
	Token unit.Dp // flakes and other small tokens
	Small unit.Dp // small buttons, filters
	Mid   unit.Dp // buttons and fields
	Large unit.Dp // panes, dialogs, menus
	Notch unit.Dp // a row's state corner
}

// Space is the spacing run.
type Space struct {
	S1, S2, S3, S4, S5, S6 unit.Dp // 4, 8, 13, 21, 34, 55
}

type Density int

const (
	Comfortable Density = iota
	Compact
)

// Motion holds the durations every animation uses; the curve is anim.EaseChisel.
type Motion struct {
	Press, Hover, Select, Field, Notch, Overlay, Break time.Duration
}

type Theme struct {
	R       Roles
	T       Type
	Cut     Cut
	Sp      Space
	Motion  Motion
	Density Density
	Shaper  *text.Shaper
}

func New(r Roles, shaper *text.Shaper) *Theme {
	return &Theme{
		R:      r,
		T:      DefaultType(),
		Cut:    Cut{Token: 3, Small: 6, Mid: 7, Large: 16, Notch: 12},
		Sp:     Space{S1: 4, S2: 8, S3: 13, S4: 21, S5: 34, S6: 55},
		Motion: Motion{Press: 90 * time.Millisecond, Hover: 160 * time.Millisecond, Select: 240 * time.Millisecond, Field: 320 * time.Millisecond, Notch: 200 * time.Millisecond, Overlay: 300 * time.Millisecond, Break: 420 * time.Millisecond},
		Shaper: shaper,
	}
}

func (t *Theme) RowHeight() unit.Dp {
	if t.Density == Compact {
		return 48
	}
	return 56
}
