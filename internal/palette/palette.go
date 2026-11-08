// Package palette resolves the colours shared by Ryoku desktop surfaces.
package palette

import (
	"encoding/hex"
	"encoding/json"
	"image/color"
	"os"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

type Palette struct {
	Roles  map[string]color.NRGBA
	Dark   bool
	Forced bool
	Source string
}

type Mode int

const (
	Follow Mode = iota
	Dark
	Light
)

func Load(mode Mode) Palette {
	named := nestedRoles(readObject(xdg.RyokuShellSettings()), "themePalette")
	follow := true
	if theme := readObject(xdg.RyokuTheme()); theme != nil {
		if value, ok := theme["followWallpaper"].(bool); ok {
			follow = value
		}
	}

	roles := make(map[string]color.NRGBA)
	source := "signature"
	if follow || mode != Follow {
		wall := colorRoles(readObject(xdg.RyokuPalette()))
		for key, value := range wall {
			roles[key] = value
		}
		if len(wall) != 0 {
			source = "wallpaper"
		}
	}
	for key, value := range named {
		roles[key] = value
	}
	if len(named) != 0 {
		source = "scheme"
	}
	if source == "signature" {
		if accent, ok := editionColor(osReleasePath); ok {
			roles["primary"] = accent
		}
	}

	forced := mode != Follow
	dark := mode != Light
	if !forced {
		dark = true
		if surface, ok := roles["surface"]; ok {
			luminance := (0.299*float64(surface.R) + 0.587*float64(surface.G) + 0.114*float64(surface.B)) / 255
			dark = luminance <= 0.5
		}
	}
	return Palette{Roles: roles, Dark: dark, Forced: forced, Source: source}
}

func readObject(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil {
		return nil
	}
	return object
}

func nestedRoles(object map[string]any, key string) map[string]color.NRGBA {
	if object == nil {
		return nil
	}
	nested, ok := object[key].(map[string]any)
	if !ok {
		return nil
	}
	return colorRoles(nested)
}

func colorRoles(object map[string]any) map[string]color.NRGBA {
	roles := make(map[string]color.NRGBA)
	for key, raw := range object {
		value, ok := raw.(string)
		if !ok {
			continue
		}
		if parsed, ok := parseColor(value); ok {
			roles[key] = parsed
		}
	}
	return roles
}

func parseColor(value string) (color.NRGBA, bool) {
	if len(value) != 7 && len(value) != 9 {
		return color.NRGBA{}, false
	}
	if value[0] != '#' {
		return color.NRGBA{}, false
	}
	decoded, err := hex.DecodeString(value[1:])
	if err != nil {
		return color.NRGBA{}, false
	}
	out := color.NRGBA{R: decoded[0], G: decoded[1], B: decoded[2], A: 0xff}
	if len(decoded) == 4 {
		out.A = decoded[3]
	}
	return out, true
}
