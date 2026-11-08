//go:build tools

// Pins the modules the packages pull in, so parallel work on the tree never
// edits go.mod at the same time.
package tools

import (
	_ "gioui.org/x/explorer"
	_ "gioui.org/x/notify"
	_ "github.com/godbus/dbus/v5"
	_ "github.com/srwiley/oksvg"
	_ "github.com/srwiley/rasterx"
	_ "golang.org/x/exp/shiny/iconvg"
	_ "golang.org/x/sys/unix"
)
