package server

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type appsTab struct {
	open  []kit.Button
	guest []kit.Button
}

func (a *appsTab) Layout(gtx layout.Context, e *env.Env, server *store.Server) layout.Dimensions {
	if len(a.open) < len(server.Apps) {
		a.open = append(a.open, make([]kit.Button, len(server.Apps)-len(a.open))...)
	}
	if len(a.guest) < len(server.Guests) {
		a.guest = append(a.guest, make([]kit.Button, len(server.Guests)-len(a.guest))...)
	}
	children := make([]layout.FlexChild, 0, len(server.Apps)+len(server.Guests)+4)
	children = append(children, layout.Rigid(common.SectionTitle(e.Th, "Web apps", nil)))
	if len(server.Apps) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No web apps are configured.")))
	}
	for i, app := range server.Apps {
		i, app := i, app
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.open[i].Clicked(gtx) {
				e.Ops.OpenURL(app.URL)
			}
			return appLine(gtx, e, app, func(gtx layout.Context) layout.Dimensions {
				return a.open[i].Layout(gtx, e.Th, kit.Line, icons.OpenInNew, "Open")
			})
		}))
	}
	if server.Pve != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx, common.SectionTitle(e.Th, "Proxmox guests", nil))
		}))
		if len(server.Guests) == 0 {
			children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No Proxmox guests were found.")))
		}
		for i, guest := range server.Guests {
			i, guest := i, guest
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				running := strings.EqualFold(guest.Status, "running")
				label := "Start"
				icon := icons.PlayArrow
				action := "start"
				if running {
					label = "Stop"
					icon = icons.Stop
					action = "stop"
				}
				if a.guest[i].Clicked(gtx) {
					e.Ops.PVEAction(server.Alias, guest.Node, guest.Type, guest.VMID, action)
				}
				return guestLine(gtx, e, guest, func(gtx layout.Context) layout.Dimensions {
					return a.guest[i].Layout(gtx, e.Th, kit.Line, icon, label)
				})
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func appLine(gtx layout.Context, e *env.Env, app remote.AppCheck, action layout.Widget) layout.Dimensions {
	proof := displayWord(app.State)
	if app.Ms > 0 {
		proof += ", " + format.Latency(int(app.Ms))
	}
	return dataActionLine(gtx, e, app.Name, proof, serviceStatusTone(app.State), action)
}

func guestLine(gtx layout.Context, e *env.Env, guest remote.Guest, action layout.Widget) layout.Dimensions {
	tone := serviceStatusTone(guest.Status)
	proof := fmt.Sprintf("%s, %s, VM %d", displayWord(guest.Status), format.Bytes(guest.Mem), guest.VMID)
	return dataActionLine(gtx, e, guest.Name, proof, tone, action)
}

func dataActionLine(gtx layout.Context, e *env.Env, name, proof string, tone model.Tone, action layout.Widget) layout.Dimensions {
	return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Flake(gtx, e.Th, tone, -1, unit.Dp(10))
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, name)),
					layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, proof)),
				)
			}),
			layout.Rigid(action),
		)
	})
}

func displayWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
