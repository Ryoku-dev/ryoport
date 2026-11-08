package server

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var serverTabs = [...]string{"Overview", "Access", "Tunnels", "Apps"}

type View struct {
	pane   common.Pane
	header common.Header

	connect        kit.SplitButton
	connectPopover kit.Popover
	connectMenu    kit.Menu
	files          kit.Button
	runCommand     kit.Button
	overflow       kit.IconButton
	overflowPop    kit.Popover
	overflowMenu   kit.Menu
	command        kit.TextField
	sendCommand    kit.Button
	commandOpen    bool

	auth      authView
	overview  overviewTab
	access    accessTab
	tunnels   tunnelsTab
	apps      appsTab
	serverKey string
	lastFrame uint64
	hasFrame  bool
}

func New() *View {
	return &View{}
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	if v.hasFrame && e.Frame != v.lastFrame && e.Frame != v.lastFrame+1 {
		v.closeTransient()
	}
	v.lastFrame = e.Frame
	v.hasFrame = true
	resource, ok := e.Snap.Resource(e.Nav.Selected)
	if !ok || resource.Kind != model.KindServer {
		v.serverKey = ""
		v.closeTransient()
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	server := e.Snap.Servers[resource.ID.Key()]
	if server == nil {
		v.serverKey = ""
		v.closeTransient()
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}

	needsAuth := resource.State == model.StateNeedsAuth || (server.Probe != nil && server.Probe.NeedAuth)
	if v.serverKey != resource.ID.Key() {
		v.serverKey = resource.ID.Key()
		v.closeTransient()
		v.command.SetText("")
		v.auth.password.SetText("")
	}
	if !needsAuth {
		v.handleActionClicks(gtx, e, resource)
	}
	selected := v.selectedTab(e)
	header := func(gtx layout.Context) layout.Dimensions {
		var acts layout.Widget
		var below layout.Widget
		if needsAuth {
			below = func(gtx layout.Context) layout.Dimensions {
				return v.auth.Layout(gtx, e, server)
			}
		} else {
			acts = func(gtx layout.Context) layout.Dimensions {
				return v.layoutActions(gtx, e, resource)
			}
			if v.commandOpen {
				below = func(gtx layout.Context) layout.Dimensions {
					return v.layoutCommand(gtx, e)
				}
			}
		}
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    headerTone(resource.State, needsAuth),
			Mark:    common.Mark(e, resource, unit.Dp(44), e.Th.R.Fg),
			Name:    resource.Name,
			Outline: -1,
			Proof:   proofSentence(server, e.Snap.Servers, resource.State, resource.Updated, e.Now),
			Note:    server.Notes,
			Acts:    acts,
			Below:   below,
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		switch selected {
		case 1:
			return v.access.Layout(gtx, e, server)
		case 2:
			return v.tunnels.Layout(gtx, e, server)
		case 3:
			return v.apps.Layout(gtx, e, server)
		default:
			return v.overview.Layout(gtx, e, resource.ID, server)
		}
	}

	dims := v.pane.Layout(gtx, e, header, serverTabs[:], selected, body)
	if tab, changed := v.pane.Tab(gtx); changed && tab >= 0 && tab < len(serverTabs) {
		e.Nav.SetTab(model.KindServer, serverTabs[tab])
	}
	return dims
}

func (v *View) selectedTab(e *env.Env) int {
	name := e.Nav.Tab(model.KindServer)
	for i, tab := range serverTabs {
		if name == tab {
			return i
		}
	}
	return 0
}

func headerTone(state model.State, needsAuth bool) model.Tone {
	if needsAuth {
		return model.ToneAttention
	}
	return state.Tone()
}

func (v *View) layoutActions(gtx layout.Context, e *env.Env, resource model.Resource) layout.Dimensions {
	connectItems := []kit.MenuItem{
		{Icon: icons.Terminal, Label: "Connect"},
		{Icon: icons.ContentCopy, Label: "Copy the ssh command"},
	}
	pinLabel := "Pin"
	if resource.Pinned {
		pinLabel = "Unpin"
	}
	overflowItems := []kit.MenuItem{
		{Icon: icons.Edit, Label: "Edit"},
		{Icon: icons.PushPin, Label: pinLabel},
		{Icon: icons.Key, Label: "Forget host key"},
		{Divider: true},
		{Icon: icons.Delete, Label: "Remove", Danger: true},
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.connectPopover.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return v.connect.Layout(gtx, e.Th, kit.Gold, icons.Terminal, "Connect", v.connectPopover.Open)
				},
				func(gtx layout.Context) layout.Dimensions {
					return v.connectMenu.Layout(gtx, e.Th, connectItems)
				},
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.files.Layout(gtx, e.Th, kit.Line, icons.FolderOpen, "Files")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.runCommand.Layout(gtx, e.Th, kit.Line, icons.Terminal, "Run a command")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.overflowPop.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return v.overflow.Layout(gtx, e.Th, kit.Line, icons.MoreHoriz, "More actions")
				},
				func(gtx layout.Context) layout.Dimensions {
					return v.overflowMenu.Layout(gtx, e.Th, overflowItems)
				},
			)
		}),
	)
}

func (v *View) handleActionClicks(gtx layout.Context, e *env.Env, resource model.Resource) {
	alias := resource.ID.Key()
	if v.connect.Main.Clicked(gtx) {
		e.Ops.Connect(alias)
	}
	if v.connect.More.Clicked(gtx) {
		opening := !v.connectPopover.Open
		v.connectPopover.Open = opening
		if opening {
			v.overflowPop.Open = false
			v.commandOpen = false
		}
	}
	if item, ok := v.connectMenu.Clicked(gtx); ok {
		v.connectPopover.Open = false
		switch item {
		case 0:
			e.Ops.Connect(alias)
		case 1:
			command := "ssh " + alias
			gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(command))})
			if e.Placards != nil {
				e.Placards.Push(e.Now, "Copied "+command, "")
			}
		}
	}
	if v.files.Clicked(gtx) {
		e.Ops.OpenFiles(alias)
	}
	if v.runCommand.Clicked(gtx) {
		opening := !v.commandOpen
		v.commandOpen = opening
		if opening {
			v.connectPopover.Open = false
			v.overflowPop.Open = false
		}
	}
	if v.commandOpen && v.sendCommand.Clicked(gtx) {
		command := strings.TrimSpace(v.command.Text())
		if command != "" {
			e.Ops.RunCommand(alias, command)
			v.command.SetText("")
			v.commandOpen = false
		}
	}
	if v.overflow.Clicked(gtx) {
		opening := !v.overflowPop.Open
		v.overflowPop.Open = opening
		if opening {
			v.connectPopover.Open = false
			v.commandOpen = false
		}
	}
	if item, ok := v.overflowMenu.Clicked(gtx); ok {
		v.overflowPop.Open = false
		switch item {
		case 0:
			e.Nav.Open(env.Draft{Kind: env.DraftServer, Target: resource.ID})
		case 1:
			e.Ops.SetPinned(resource.ID, !resource.Pinned)
		case 2:
			e.Ops.ForgetHostKey(alias)
		case 4:
			removeAlias := alias
			e.Nav.Ask(env.Confirm{
				Title:  "Remove " + resource.Name + "?",
				Body:   "Ryoport will remove this server from your SSH configuration. The server itself is not changed.",
				Action: "Remove " + resource.Name,
				Run: func() {
					e.Ops.RemoveServer(removeAlias)
				},
			})
		}
	}
}

func (v *View) closeTransient() {
	v.connectPopover.Open = false
	v.overflowPop.Open = false
	v.commandOpen = false
}

func (v *View) layoutCommand(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return cappedField(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.command.Layout(gtx, e.Th, "Command", "uptime", "Runs in a terminal on the server.", "")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.sendCommand.Layout(gtx, e.Th, kit.Gold, icons.KeyboardReturn, "Run")
		}),
	)
}

func proofSentence(server *store.Server, peers map[string]*store.Server, state model.State, updated, now time.Time) []common.Span {
	if server == nil {
		return nil
	}
	var parts []string
	if state == model.StateNeedsAuth || (server.Probe != nil && server.Probe.NeedAuth) {
		if server.Reach != nil && server.Reach.Up {
			line := "Reachable"
			if server.Reach.RTTMs >= 0 {
				line += ", " + format.Latency(server.Reach.RTTMs) + " from here"
			}
			parts = append(parts, line+".")
		}
		user := server.User
		if user == "" {
			user = "the configured user"
		}
		failed := "Sign-in as " + user + " failed"
		if !updated.IsZero() {
			failed += " " + format.Ago(updated, now)
		}
		parts = append(parts, failed+".")
		return []common.Span{{Text: strings.Join(parts, " ")}}
	}
	if server.Reach != nil && server.Reach.Up {
		line := "Online"
		if server.Probe != nil && server.Probe.OK && server.Probe.UptimeS > 0 {
			line += " for " + format.Duration(time.Duration(server.Probe.UptimeS)*time.Second)
		}
		parts = append(parts, line+".")
		if server.Reach.RTTMs >= 0 {
			parts = append(parts, format.Latency(server.Reach.RTTMs)+" from here.")
		}
	} else {
		parts = append(parts, "Offline.")
	}
	if server.Probe != nil && server.Probe.FailedUnits > 0 {
		unit := "units"
		if server.Probe.FailedUnits == 1 {
			unit = "unit"
		}
		parts = append(parts, fmt.Sprintf("%d failed %s.", server.Probe.FailedUnits, unit))
	}
	distro := ""
	if server.Probe != nil {
		distro = strings.TrimSpace(server.Probe.Distro)
	}
	peer := sameHostPeer(server, peers)
	switch {
	case distro != "" && peer != "":
		parts = append(parts, distro+" on the same box as "+peer+".")
	case distro != "":
		parts = append(parts, distro+".")
	case peer != "":
		parts = append(parts, "On the same box as "+peer+".")
	}
	return []common.Span{{Text: strings.Join(parts, " ")}}
}

func sameHostPeer(server *store.Server, peers map[string]*store.Server) string {
	if server == nil || strings.TrimSpace(server.HostName) == "" {
		return ""
	}
	aliases := make([]string, 0, len(peers))
	for alias, peer := range peers {
		if peer == nil || peer == server || alias == server.Alias {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(peer.HostName), strings.TrimSpace(server.HostName)) {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	if len(aliases) == 0 {
		return ""
	}
	return aliases[0]
}

func percentValue(frac float64) string {
	return fmt.Sprintf("%.0f%%", frac*100)
}
