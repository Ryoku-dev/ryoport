package server

import (
	"strings"
	"time"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type tunnelsTab struct {
	close []kit.Button
	spec  kit.TextField
	open  kit.Button
}

func (t *tunnelsTab) Layout(gtx layout.Context, e *env.Env, server *store.Server) layout.Dimensions {
	if len(t.close) < len(server.Tunnels) {
		t.close = append(t.close, make([]kit.Button, len(server.Tunnels)-len(t.close))...)
	}
	children := make([]layout.FlexChild, 0, len(server.Tunnels)+4)
	children = append(children, layout.Rigid(common.SectionTitle(e.Th, "Open tunnels", nil)))
	if len(server.Tunnels) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No tunnels are open.")))
	}
	for i, tunnel := range server.Tunnels {
		i, tunnel := i, tunnel
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				opened := "Open"
				if tunnel.Opened > 0 {
					opened = "Opened " + format.Ago(time.Unix(tunnel.Opened, 0), e.Now)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, displayWord(tunnel.Kind))),
							layout.Rigid(kit.Text(e.Th, e.Th.T.Mono, e.Th.R.Fg2, tunnel.Spec)),
							layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, opened)),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return t.close[i].Layout(gtx, e.Th, kit.Line, icons.Close, "Close")
					}),
				)
			})
			if t.close[i].Clicked(gtx) {
				e.Ops.CloseTunnel(tunnel.ID)
			}
			return dims
		}))
	}

	value := strings.TrimSpace(t.spec.Text())
	errText := ""
	if value != "" {
		errText = validateTunnelSpec(value)
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx, common.SectionTitle(e.Th, "Open a tunnel", nil))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t.spec.Mono = true
			return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
				return t.spec.Layout(gtx, e.Th, "Tunnel", "L:8080:127.0.0.1:80", "Local L:port:host:port, remote R:port:host:port, or dynamic D:port.", errText)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if value == "" || errText != "" {
							gtx = gtx.Disabled()
						}
						return t.open.Layout(gtx, e.Th, kit.Gold, icons.Link, "Open")
					}),
				)
				if value != "" && errText == "" && t.open.Clicked(gtx) {
					e.Ops.OpenTunnel(server.Alias, value)
					t.spec.SetText("")
				}
				return dims
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func validateTunnelSpec(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Enter a tunnel."
	}
	if _, _, err := remote.ParseTunnelSpec(value); err != nil {
		return "Use L:port:host:port, R:port:host:port, or D:port."
	}
	return ""
}
