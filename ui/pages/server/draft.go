package server

import (
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type Draft struct {
	pane   common.Pane
	header common.Header

	target      model.ID
	initialized bool
	original    remote.HostInput

	alias        kit.TextField
	host         kit.TextField
	user         kit.TextField
	port         kit.TextField
	auth         kit.Filters
	authSelected int

	advanced     bool
	advancedOpen kit.Button
	identity     kit.TextField
	proxy        kit.TextField
	group        kit.TextField
	tags         kit.TextField
	notes        kit.TextField
	pveURL       kit.TextField
	pveToken     kit.TextField
	pveInsecure  bool

	save   kit.Button
	cancel kit.Button
}

type validationInput struct {
	Alias, Host, Port, Tunnel string
}

type validationErrors struct {
	Alias, Host, Port, Tunnel string
}

func (v validationErrors) Any() bool {
	return v.Alias != "" || v.Host != "" || v.Port != "" || v.Tunnel != ""
}

func NewDraft() *Draft {
	return &Draft{}
}

func (d *Draft) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil || e.Nav.Draft.Kind != env.DraftServer {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	d.initialize(e)
	values := validationInput{Alias: d.alias.Text(), Host: d.host.Text(), Port: d.port.Text()}
	errors := validateDraft(values)
	editing := d.target != ""
	name := strings.TrimSpace(d.alias.Text())
	if name == "" {
		name = "New server"
		if editing {
			name = "Edit server"
		}
	}
	proof := "Add a server you can reach over SSH."
	if editing {
		proof = "Change how Ryoport reaches this server."
	}
	resource := model.Resource{Kind: model.KindServer, Name: name, Icon: "server", State: model.StateStopped}
	header := func(gtx layout.Context) layout.Dimensions {
		return d.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Mark:    common.Mark(e, resource, unit.Dp(44), e.Th.R.Fg),
			Name:    name,
			Outline: -1,
			Proof:   []common.Span{{Text: proof}},
			Acts: func(gtx layout.Context) layout.Dimensions {
				return d.layoutActions(gtx, e, errors)
			},
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return d.layoutForm(gtx, e, errors)
	}
	return d.pane.Layout(gtx, e, header, nil, 0, body)
}

func (d *Draft) initialize(e *env.Env) {
	target := e.Nav.Draft.Target
	if d.initialized && d.target == target {
		return
	}
	d.target = target
	d.initialized = true
	d.original = remote.HostInput{Port: 22}
	d.advanced = false
	d.authSelected = 0
	if target != "" {
		if server := e.Snap.Servers[target.Key()]; server != nil {
			d.original = remote.HostInput{
				Alias: server.Alias, HostName: server.HostName, User: server.User, Port: server.Port,
				IdentityFile: server.IdentityFile, ProxyJump: server.ProxyJump, Group: server.Group,
				Tags: append([]string(nil), server.Tags...), Notes: server.Notes, Pinned: server.Pinned,
				Watch: append([]string(nil), server.Watch...), Apps: append([]remote.App(nil), server.Host.Apps...),
				Pve: server.Host.Pve, Auth: server.Auth,
			}
			if server.Auth == "password" {
				d.authSelected = 1
			}
			if server.Pve != nil {
				d.pveInsecure = server.Pve.Insecure
			}
		}
	}
	if d.original.Port == 0 {
		d.original.Port = 22
	}
	d.alias.SetText(d.original.Alias)
	d.host.SetText(d.original.HostName)
	d.user.SetText(d.original.User)
	d.port.SetText(strconv.Itoa(d.original.Port))
	d.identity.SetText(d.original.IdentityFile)
	d.proxy.SetText(d.original.ProxyJump)
	d.group.SetText(d.original.Group)
	d.tags.SetText(strings.Join(d.original.Tags, ", "))
	d.notes.SetText(d.original.Notes)
	if d.original.Pve != nil {
		d.pveURL.SetText(d.original.Pve.URL)
		d.pveToken.SetText(d.original.Pve.Token)
	} else {
		d.pveURL.SetText("")
		d.pveToken.SetText("")
		d.pveInsecure = false
	}
}

func (d *Draft) layoutActions(gtx layout.Context, e *env.Env, errors validationErrors) layout.Dimensions {
	dims := layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			saveGTX := gtx
			if errors.Any() {
				saveGTX = gtx.Disabled()
			}
			return d.save.Layout(saveGTX, e.Th, kit.Gold, icons.Save, "Save")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.cancel.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
		}),
	)
	if !errors.Any() && d.save.Clicked(gtx) {
		input := d.hostInput()
		previous := ""
		if d.target != "" {
			previous = d.target.Key()
		}
		e.Ops.SaveServer(input, previous)
		e.Nav.CloseDraft()
		d.initialized = false
	}
	if d.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
		d.initialized = false
	}
	return dims
}

func (d *Draft) layoutForm(gtx layout.Context, e *env.Env, errors validationErrors) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(common.SectionTitle(e.Th, "Connection", nil)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
				return d.alias.Layout(gtx, e.Th, "Alias", "ally", "The short name used by SSH and Ryoport.", errors.Alias)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fieldInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
				return d.host.Layout(gtx, e.Th, "Host", "192.168.1.10 or server.example.com", "A host name or address.", errors.Host)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fieldInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
				return d.user.Layout(gtx, e.Th, "User", "root", "Leave empty to use your local user.", "")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d.port.Mono = true
			return fieldInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
				return d.port.Layout(gtx, e.Th, "Port", "22", "SSH usually listens on port 22.", errors.Port)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Authentication")),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims := d.auth.Layout(gtx, e.Th, []string{"Key", "Password"}, nil, d.authSelected)
						if selected, changed := d.auth.Changed(gtx); changed {
							d.authSelected = selected
						}
						return dims
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: e.Th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Advanced"
				icon := icons.ExpandMore
				if d.advanced {
					label = "Hide advanced"
					icon = icons.ExpandLess
				}
				dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return d.advancedOpen.Layout(gtx, e.Th, kit.Quiet, icon, label)
					}),
				)
				if d.advancedOpen.Clicked(gtx) {
					d.advanced = !d.advanced
				}
				return dims
			})
		}),
	}
	if d.advanced {
		children = append(children, d.advancedFields(e)...)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Draft) advancedFields(e *env.Env) []layout.FlexChild {
	field := func(widget layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fieldInset(gtx, e, widget)
		})
	}
	return []layout.FlexChild{
		field(func(gtx layout.Context) layout.Dimensions {
			d.identity.Mono = true
			return d.identity.Layout(gtx, e.Th, "Identity file", "~/.ssh/id_ed25519", "The private key SSH should use.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			d.proxy.Mono = true
			return d.proxy.Layout(gtx, e.Th, "Proxy jump", "bastion", "An SSH alias used to reach this server.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			return d.group.Layout(gtx, e.Th, "Group", "Home lab", "Groups keep related servers together.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			return d.tags.Layout(gtx, e.Th, "Tags", "gpu, build", "Separate tags with commas.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			return d.notes.Layout(gtx, e.Th, "Notes", "What this server is for", "Shown under the server name.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			d.pveURL.Mono = true
			return d.pveURL.Layout(gtx, e.Th, "Proxmox URL", "https://host:8006", "Leave empty when this is not a Proxmox host.", "")
		}),
		field(func(gtx layout.Context) layout.Dimensions {
			d.pveToken.Secret = true
			return d.pveToken.Layout(gtx, e.Th, "Proxmox token", "user@realm!token=secret", "An API token used to list and control guests.", "")
		}),
	}
}

func fieldInset(gtx layout.Context, e *env.Env, widget layout.Widget) layout.Dimensions {
	return layout.Inset{Top: e.Th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return cappedField(gtx, widget)
	})
}

func (d *Draft) hostInput() remote.HostInput {
	port, _ := strconv.Atoi(strings.TrimSpace(d.port.Text()))
	input := d.original
	input.Alias = strings.TrimSpace(d.alias.Text())
	input.HostName = strings.TrimSpace(d.host.Text())
	input.User = strings.TrimSpace(d.user.Text())
	input.Port = port
	input.IdentityFile = strings.TrimSpace(d.identity.Text())
	input.ProxyJump = strings.TrimSpace(d.proxy.Text())
	input.Group = strings.TrimSpace(d.group.Text())
	input.Tags = splitTags(d.tags.Text())
	input.Notes = strings.TrimSpace(d.notes.Text())
	input.Auth = "key"
	if d.authSelected == 1 {
		input.Auth = "password"
	}
	url := strings.TrimSpace(d.pveURL.Text())
	token := strings.TrimSpace(d.pveToken.Text())
	if url == "" && token == "" {
		input.Pve = nil
	} else {
		input.Pve = &remote.PVE{URL: url, Token: token, Insecure: d.pveInsecure}
	}
	return input
}

func validateDraft(input validationInput) validationErrors {
	var errors validationErrors
	alias := strings.TrimSpace(input.Alias)
	switch {
	case alias == "":
		errors.Alias = "Enter an alias."
	case !remote.ValidAlias(alias):
		errors.Alias = "Use letters, numbers, dots, underscores, or hyphens."
	}
	host := strings.TrimSpace(input.Host)
	switch {
	case host == "":
		errors.Host = "Enter a host name or address."
	case strings.ContainsAny(host, " \t\r\n") || strings.HasPrefix(host, "-"):
		errors.Host = "Host names cannot contain spaces or start with a hyphen."
	}
	port, err := strconv.Atoi(strings.TrimSpace(input.Port))
	if err != nil || port < 1 || port > 65535 {
		errors.Port = "Port must be from 1 to 65535."
	}
	if strings.TrimSpace(input.Tunnel) != "" {
		errors.Tunnel = validateTunnelSpec(input.Tunnel)
	}
	return errors
}

func splitTags(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
