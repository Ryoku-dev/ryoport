package common

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
)

func Mark(e *env.Env, r model.Resource, size unit.Dp, ink color.NRGBA) layout.Widget {
	path := ""
	if e != nil && e.Snap != nil {
		switch r.Kind {
		case model.KindMachine:
			if machine := e.Snap.Machines[r.ID.Key()]; machine != nil {
				path = machine.Logo
			}
			if path == "" {
				path = e.Snap.Catalog.Logos[r.Icon]
			}
		case model.KindPassthrough:
			path = e.Snap.Catalog.Logos[r.Icon]
		case model.KindServer:
			if server := e.Snap.Servers[r.ID.Key()]; server != nil && server.Probe != nil {
				path = e.Snap.Catalog.Logos[build.DistroSlug(server.Probe.Distro)]
			}
		}
	}
	if path != "" && e != nil && e.Logos != nil {
		return e.Logos.Widget(path, size, r.Name, ink)
	}
	icon := KindIcon(r.Kind)
	return func(gtx layout.Context) layout.Dimensions {
		if icon == nil {
			return layout.Dimensions{}
		}
		px := gtx.Dp(size)
		gtx.Constraints = layout.Exact(image.Pt(px, px))
		return icon.Layout(gtx, ink)
	}
}

func KindIcon(k model.Kind) *widget.Icon {
	switch k {
	case model.KindMachine:
		return icons.DesktopWindows
	case model.KindPassthrough:
		return icons.ScreenShare
	case model.KindConsole:
		return icons.SettingsEthernet
	case model.KindServer:
		return icons.Dns
	default:
		return nil
	}
}
