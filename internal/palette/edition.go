package palette

import (
	"image/color"
	"os"
	"strconv"
	"strings"
)

var osReleasePath = "/etc/os-release"

func editionColor(path string) (color.NRGBA, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return color.NRGBA{}, false
	}
	var id, idLike string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ID":
			id = osReleaseValue(value)
		case "ID_LIKE":
			idLike = osReleaseValue(value)
		}
	}
	if value, ok := edition(strings.ToLower(id)); ok {
		return value, true
	}
	for like := range strings.FieldsSeq(strings.ToLower(idLike)) {
		if value, ok := edition(like); ok {
			return value, true
		}
	}
	return color.NRGBA{}, false
}

func osReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func edition(id string) (color.NRGBA, bool) {
	var hex uint32
	switch id {
	case "arch":
		hex = 0x1793d1
	case "cachyos":
		hex = 0x00b3a1
	case "fedora":
		hex = 0x51a2da
	case "void":
		hex = 0x478061
	case "nixos":
		hex = 0x7ebae4
	case "ubuntu":
		hex = 0xe95420
	case "debian":
		hex = 0xd70a53
	case "opensuse":
		hex = 0x73ba25
	case "manjaro":
		hex = 0x35bf5c
	case "endeavouros":
		hex = 0x7f3fbf
	case "linuxmint":
		hex = 0x86be43
	case "pop":
		hex = 0x48b9c7
	case "gentoo":
		hex = 0x7a6bbd
	case "alpine":
		hex = 0x0d7fb0
	default:
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}, true
}
