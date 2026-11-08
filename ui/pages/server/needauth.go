package server

import (
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type authView struct {
	password kit.TextField
	signIn   kit.Button
	useKey   kit.Button
}

func (a *authView) Layout(gtx layout.Context, e *env.Env, server *store.Server) layout.Dimensions {
	if server == nil {
		return layout.Dimensions{}
	}
	if !e.Snap.Host.Keyring {
		return a.layoutWithoutKeyring(gtx, e)
	}
	user := server.User
	if user == "" {
		user = "user"
	}
	host := server.HostName
	if host == "" {
		host = server.Alias
	}
	label := "Password for " + user + "@" + host
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
				a.password.Secret = true
				return a.password.Layout(gtx, e.Th, label, "Password", "Saved in your keyring, never on a command line.", "")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					signGTX := gtx
					if strings.TrimSpace(a.password.Text()) == "" {
						signGTX = gtx.Disabled()
					}
					return a.signIn.Layout(signGTX, e.Th, kit.Gold, icons.Key, "Sign in")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.useKey.Layout(gtx, e.Th, kit.Line, icons.VpnKey, "Use a key instead")
				}),
			)
			if a.signIn.Clicked(gtx) {
				password := a.password.Text()
				if strings.TrimSpace(password) != "" {
					e.Ops.SetPassword(server.Alias, password)
					e.Ops.Refresh()
					a.password.SetText("")
				}
			}
			if a.useKey.Clicked(gtx) {
				e.Nav.SetTab(model.KindServer, "Access")
			}
			return dims
		}),
	)
}

func (a *authView) layoutWithoutKeyring(gtx layout.Context, e *env.Env) layout.Dimensions {
	dims := layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
		layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Saving a password needs secret-tool from libsecret.")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.useKey.Layout(gtx, e.Th, kit.Line, icons.VpnKey, "Use a key instead")
				}),
			)
		}),
	)
	if a.useKey.Clicked(gtx) {
		e.Nav.SetTab(model.KindServer, "Access")
	}
	return dims
}
