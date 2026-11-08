package console

import (
	"fmt"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

// View shows one saved serial, Telnet, RDP, or VNC console.
type View struct {
	pane   common.Pane
	header common.Header

	open kit.Button
	edit kit.Button
	more kit.IconButton

	overflow kit.Popover
	menu     kit.Menu

	showPassword bool
	password     kit.TextField
	savePassword kit.Button
	cancelPass   kit.Button
}

func New() *View {
	v := &View{}
	v.password.SingleLine = true
	v.password.Secret = true
	return v
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	id := e.Nav.Selected
	resource, ok := e.Snap.Resource(id)
	stored := e.Snap.Consoles[id.Key()]
	if !ok || resource.Kind != model.KindConsole || stored == nil {
		return v.missing(gtx, e)
	}

	tone := model.ToneIdle
	if stored.Reach != nil && stored.Reach.Up {
		tone = model.ToneAlive
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    tone,
			Mark:    common.Mark(e, resource, unit.Dp(44), e.Th.R.Fg),
			Name:    resource.Name,
			Outline: -1,
			Proof:   consoleProof(stored.Profile, stored.Reach),
			Acts: func(gtx layout.Context) layout.Dimensions {
				return v.actions(gtx, e, resource, stored.Profile, stored.Reach)
			},
			Below: v.passwordForm(e, stored.Profile),
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return v.body(gtx, e, stored.Profile)
	}
	return v.pane.Layout(gtx, e, header, nil, 0, body)
}

func (v *View) missing(gtx layout.Context, e *env.Env) layout.Dimensions {
	header := func(gtx layout.Context) layout.Dimensions {
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Name:    "Console unavailable",
			Outline: -1,
			Proof:   []common.Span{{Text: "This console is no longer in the fleet."}},
		})
	}
	body := kit.Empty(e.Th, icons.SettingsEthernet, "Console unavailable", "Choose another console from the fleet.")
	return v.pane.Layout(gtx, e, header, nil, 0, body)
}

func (v *View) actions(gtx layout.Context, e *env.Env, resource model.Resource, profile console.Profile, reach *console.Reach) layout.Dimensions {
	blocker := consoleConnectBlocker(e.Snap.Host.Consoles, profile, reach)
	if v.open.Clicked(gtx) && blocker == "" {
		e.Ops.OpenConsole(profile.ID)
	}
	if v.edit.Clicked(gtx) {
		e.Nav.Open(env.Draft{Kind: env.DraftConsole, Target: resource.ID})
	}
	if v.more.Clicked(gtx) {
		v.overflow.Open = !v.overflow.Open
	}
	if choice, ok := v.menu.Clicked(gtx); ok {
		v.overflow.Open = false
		switch choice {
		case 0:
			e.Ops.SetPinned(resource.ID, !resource.Pinned)
		case 1:
			v.showPassword = true
		case 2:
			e.Ops.ClearConsolePassword(profile.ID)
		case 4:
			name := profile.Name
			if name == "" {
				name = strings.ToUpper(profile.Kind) + " console"
			}
			e.Nav.Ask(env.Confirm{
				Title:  "Remove " + name + "?",
				Body:   "Removing this console forgets its connection profile. The machine it reaches is not changed.",
				Action: "Remove " + name,
				Run:    func() { e.Ops.RemoveConsole(profile.ID) },
			})
		}
	}

	pinLabel := "Pin"
	if resource.Pinned {
		pinLabel = "Unpin"
	}
	items := []kit.MenuItem{
		{Icon: icons.PushPin, Label: pinLabel},
		{Icon: icons.Key, Label: "Set password"},
		{Icon: icons.LockOpen, Label: "Clear password", Disabled: profile.Auth == ""},
		{Divider: true},
		{Icon: icons.Delete, Label: "Remove", Danger: true},
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Tooltip(gtx, e.Th, blocker, func(gtx layout.Context) layout.Dimensions {
				buttonGTX := gtx
				if blocker != "" {
					buttonGTX = gtx.Disabled()
				}
				return v.open.Layout(buttonGTX, e.Th, kit.Gold, icons.OpenInNew, "Connect")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.edit.Layout(gtx, e.Th, kit.Line, icons.Edit, "Edit")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.overflow.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return v.more.Layout(gtx, e.Th, kit.Line, icons.MoreHoriz, "More actions")
				},
				func(gtx layout.Context) layout.Dimensions {
					return v.menu.Layout(gtx, e.Th, items)
				},
			)
		}),
	)
}

func (v *View) passwordForm(e *env.Env, profile console.Profile) layout.Widget {
	if !v.showPassword {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		if v.savePassword.Clicked(gtx) && v.password.Text() != "" {
			e.Ops.SetConsolePassword(profile.ID, v.password.Text())
			v.password.SetText("")
			v.showPassword = false
		}
		if v.cancelPass.Clicked(gtx) {
			v.password.SetText("")
			v.showPassword = false
		}
		return layout.Inset{Top: e.Th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.password.Layout(gtx, e.Th, "Password", "Enter the console password", "Stored in the keyring.", "")
				}),
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							buttonGTX := gtx
							if v.password.Text() == "" {
								buttonGTX = gtx.Disabled()
							}
							return v.savePassword.Layout(buttonGTX, e.Th, kit.Gold, icons.Save, "Save password")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.cancelPass.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
						}),
					)
				}),
			)
		})
	}
}

func (v *View) body(gtx layout.Context, e *env.Env, profile console.Profile) layout.Dimensions {
	facts := consoleFacts(profile)
	available := consoleAvailable(e.Snap.Host.Consoles, profile.Kind)
	advice := e.Snap.Host.Consoles.Advice[profile.Kind]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(common.SectionTitle(e.Th, "Profile", nil)),
		layout.Rigid(kit.Facts(e.Th, facts)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if available {
				return layout.Dimensions{}
			}
			if advice == "" {
				advice = "Install a compatible client, then refresh the fleet."
			}
			return layout.Inset{Top: e.Th.Sp.S5}.Layout(gtx,
				kit.Empty(e.Th, icons.SettingsEthernet, "No "+consoleKindName(profile.Kind)+" client", advice),
			)
		}),
	)
}

func consoleProof(profile console.Profile, reach *console.Reach) []common.Span {
	if profile.Kind == "serial" {
		spans := []common.Span{
			{Text: "Serial on "},
			{Text: profile.Device, Mono: true},
			{Text: " at "},
			{Text: strconv.Itoa(profile.Baud), Mono: true},
		}
		if serialNotPluggedIn(profile, reach) {
			spans = append(spans, common.Span{Text: ". Not plugged in."})
		}
		return spans
	}
	spans := []common.Span{
		{Text: consoleKindName(profile.Kind) + " to "},
		{Text: profile.Address, Mono: true},
	}
	if reach != nil && reach.Up {
		spans = append(spans, common.Span{Text: ", "}, common.Span{Text: format.Latency(reach.RTTMs), Mono: true})
	}
	return spans
}

func consoleFacts(profile console.Profile) []kit.Fact {
	facts := []kit.Fact{{Label: "Kind", Value: consoleKindName(profile.Kind)}}
	if profile.Kind == "serial" {
		return append(facts,
			kit.Fact{Label: "Device", Value: profile.Device, Mono: true},
			kit.Fact{Label: "Baud", Value: strconv.Itoa(profile.Baud), Mono: true},
		)
	}
	facts = append(facts,
		kit.Fact{Label: "Address", Value: profile.Address, Mono: true},
		kit.Fact{Label: "Port", Value: strconv.Itoa(profile.Port), Mono: true},
	)
	if profile.Kind == "rdp" {
		facts = append(facts,
			kit.Fact{Label: "User", Value: profile.User},
			kit.Fact{Label: "Domain", Value: profile.Domain},
		)
	}
	if profile.Kind == "rdp" || profile.Kind == "vnc" {
		facts = append(facts, kit.Fact{Label: "Window", Value: fullscreenWord(profile.Fullscreen)})
	}
	return facts
}

func consoleAvailable(caps console.Caps, kind string) bool {
	switch kind {
	case "serial":
		return caps.Serial
	case "telnet":
		return caps.Telnet
	case "rdp":
		return caps.RDP
	case "vnc":
		return caps.VNC
	default:
		return false
	}
}

func consoleConnectBlocker(caps console.Caps, profile console.Profile, reach *console.Reach) string {
	if serialNotPluggedIn(profile, reach) {
		return "Not plugged in."
	}
	if !consoleAvailable(caps, profile.Kind) {
		if advice := strings.TrimSpace(caps.Advice[profile.Kind]); advice != "" {
			return advice
		}
		return "No " + consoleKindName(profile.Kind) + " client is installed."
	}
	return ""
}

func serialNotPluggedIn(profile console.Profile, reach *console.Reach) bool {
	return profile.Kind == "serial" && reach != nil && !reach.Up && strings.HasPrefix(reach.Error, "device not present:")
}

func consoleKindName(kind string) string {
	switch kind {
	case "rdp":
		return "RDP"
	case "vnc":
		return "VNC"
	case "telnet":
		return "Telnet"
	case "serial":
		return "Serial"
	default:
		return fmt.Sprintf("%s console", kind)
	}
}

func fullscreenWord(fullscreen bool) string {
	if fullscreen {
		return "Fullscreen"
	}
	return "Windowed"
}
