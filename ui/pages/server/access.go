package server

import (
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type accessTab struct {
	password kit.TextField
	set      kit.Button
	clear    kit.Button
	deploy   []kit.Button
	keyName  kit.TextField
	makeKey  kit.Button
}

func (a *accessTab) Layout(gtx layout.Context, e *env.Env, server *store.Server) layout.Dimensions {
	auth := "SSH key or agent"
	if server.Auth == "password" {
		auth = "Password"
	}
	lastReachable := "Not yet"
	if server.Reach != nil && server.Reach.Up {
		lastReachable = "Just now"
	}
	facts := []kit.Fact{
		{Label: "Address", Value: server.HostName, Mono: true},
		{Label: "Port", Value: formatInt(server.Port), Mono: true},
		{Label: "User", Value: server.User},
		{Label: "Identity file", Value: server.IdentityFile, Mono: true},
		{Label: "Proxy jump", Value: server.ProxyJump, Mono: true},
		{Label: "Authentication", Value: auth},
		{Label: "Last reachable", Value: lastReachable},
	}
	if e.Snap.Host.Keyring {
		if a.set.Clicked(gtx) {
			password := a.password.Text()
			if strings.TrimSpace(password) != "" {
				e.Ops.SetPassword(server.Alias, password)
				a.password.SetText("")
			}
		}
		if a.clear.Clicked(gtx) {
			e.Ops.ClearPassword(server.Alias)
			a.password.SetText("")
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(common.SectionTitle(e.Th, "What Ryoport knows", nil)),
		layout.Rigid(kit.Facts(e.Th, facts)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if server.Probe != nil && !server.Probe.NeedAuth {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx,
				kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Nothing else is probed until Ryoport can sign in."))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx, common.SectionTitle(e.Th, "Password", nil))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !e.Snap.Host.Keyring {
				return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Saving a password needs secret-tool from libsecret.")(gtx)
			}
			return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
				a.password.Secret = true
				return a.password.Layout(gtx, e.Th, "Password", "New password", "Saved in your keyring, never on a command line.", "")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !e.Snap.Host.Keyring {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						setGTX := gtx
						if strings.TrimSpace(a.password.Text()) == "" {
							setGTX = gtx.Disabled()
						}
						return a.set.Layout(setGTX, e.Th, kit.Gold, icons.Save, "Set password")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.clear.Layout(gtx, e.Th, kit.Line, icons.Close, "Clear password")
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx, common.SectionTitle(e.Th, "SSH keys", nil))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutKeys(gtx, e, server.Alias)
		}),
	)
}

func (a *accessTab) layoutKeys(gtx layout.Context, e *env.Env, alias string) layout.Dimensions {
	if len(a.deploy) < len(e.Snap.Keys.Files) {
		a.deploy = append(a.deploy, make([]kit.Button, len(e.Snap.Keys.Files)-len(a.deploy))...)
	}
	children := make([]layout.FlexChild, 0, len(e.Snap.Keys.Agent)+len(e.Snap.Keys.Files)+3)
	for _, key := range e.Snap.Keys.Agent {
		key := key
		label := key.Comment
		if label == "" {
			label = key.Type + " agent key"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return keyLine(gtx, e, label, key.Fingerprint, nil)
		}))
	}
	for i, key := range e.Snap.Keys.Files {
		i, key := i, key
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.deploy[i].Clicked(gtx) {
				e.Ops.DeployKey(alias, key.Path)
			}
			button := func(gtx layout.Context) layout.Dimensions {
				return a.deploy[i].Layout(gtx, e.Th, kit.Line, icons.Upload, "Deploy")
			}
			return keyLine(gtx, e, key.Path, key.Fingerprint, button)
		}))
	}
	if len(e.Snap.Keys.Agent) == 0 && len(e.Snap.Keys.Files) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No SSH keys found.")))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: e.Th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if a.makeKey.Clicked(gtx) {
				name := strings.TrimSpace(a.keyName.Text())
				if name == "" {
					name = "ryoport"
				}
				e.Ops.Keygen(name, "ed25519")
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.keyName.Layout(gtx, e.Th, "New key name", "ryoport", "Makes an Ed25519 key in your SSH folder.", "")
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.makeKey.Layout(gtx, e.Th, kit.Gold, icons.Key, "Make a key")
				}),
			)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func keyLine(gtx layout.Context, e *env.Env, label, fingerprint string, action layout.Widget) layout.Dimensions {
	if action == nil {
		action = func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	return layout.Inset{Top: e.Th.Sp.S2, Bottom: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, label)),
					layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, fingerprint)),
				)
			}),
			layout.Rigid(action),
		)
	})
}
