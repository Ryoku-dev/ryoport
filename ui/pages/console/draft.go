package console

import (
	"strconv"
	"strings"

	"gioui.org/layout"
	backend "github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

// Draft edits a console profile in the work pane.
type Draft struct {
	pane   common.Pane
	header common.Header
	kinds  kit.Filters

	name    kit.TextField
	device  kit.TextField
	baud    kit.TextField
	address kit.TextField
	port    kit.TextField
	user    kit.TextField
	domain  kit.TextField
	full    kit.Toggle

	save   kit.Button
	cancel kit.Button

	loaded       bool
	loadedTarget model.ID
	kind         string
	profileID    string
}

func NewDraft() *Draft {
	d := &Draft{kind: "serial"}
	for _, field := range []*kit.TextField{&d.name, &d.device, &d.baud, &d.address, &d.port, &d.user, &d.domain} {
		field.SingleLine = true
	}
	for _, field := range []*kit.TextField{&d.device, &d.baud, &d.address, &d.port} {
		field.Mono = true
	}
	d.baud.SetText("115200")
	return d
}

func (d *Draft) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	d.load(e)
	profile, errs := parseConsoleDraft(
		d.profileID,
		d.kind,
		d.name.Text(),
		d.device.Text(),
		d.baud.Text(),
		d.address.Text(),
		d.port.Text(),
		d.user.Text(),
		d.domain.Text(),
		d.full.Value,
	)

	title := "New console"
	if e.Nav.Draft.Target != "" {
		title = profile.Name
		if title == "" {
			title = "Edit console"
		}
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return d.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Mark:    common.Mark(e, model.Resource{Kind: model.KindConsole, Name: title}, 44, e.Th.R.Fg),
			Name:    title,
			Outline: -1,
			Proof:   []common.Span{{Text: "Save a direct console connection for quick access from the fleet."}},
			Acts: func(gtx layout.Context) layout.Dimensions {
				return d.actions(gtx, e, profile, errs)
			},
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return d.form(gtx, e, errs)
	}
	return d.pane.Layout(gtx, e, header, nil, 0, body)
}

func (d *Draft) load(e *env.Env) {
	target := e.Nav.Draft.Target
	if d.loaded && target == d.loadedTarget {
		return
	}
	d.loaded = true
	d.loadedTarget = target
	d.profileID = ""
	d.kind = "serial"
	d.name.SetText("")
	d.device.SetText("")
	d.baud.SetText("115200")
	d.address.SetText("")
	d.port.SetText("")
	d.user.SetText("")
	d.domain.SetText("")
	d.full.Value = false
	if target == "" {
		return
	}
	stored := e.Snap.Consoles[target.Key()]
	if stored == nil {
		return
	}
	profile := stored.Profile
	d.profileID = profile.ID
	d.kind = profile.Kind
	d.name.SetText(profile.Name)
	d.device.SetText(profile.Device)
	d.baud.SetText(strconv.Itoa(profile.Baud))
	d.address.SetText(profile.Address)
	d.port.SetText(strconv.Itoa(profile.Port))
	d.user.SetText(profile.User)
	d.domain.SetText(profile.Domain)
	d.full.Value = profile.Fullscreen
}

func (d *Draft) actions(gtx layout.Context, e *env.Env, profile backend.Profile, errs consoleDraftErrors) layout.Dimensions {
	if d.save.Clicked(gtx) && !errs.Any() {
		if profile.ID == "" {
			profile.ID = backend.NewID()
		}
		e.Ops.SaveConsole(profile)
		e.Nav.CloseDraft()
		e.Nav.Select(model.NewID(model.KindConsole, profile.ID))
	}
	if d.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
	}
	label := "Save"
	if e.Nav.Draft.Target == "" {
		label = "Add console"
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			buttonGTX := gtx
			if errs.Any() {
				buttonGTX = gtx.Disabled()
			}
			return d.save.Layout(buttonGTX, e.Th, kit.Gold, icons.Save, label)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.cancel.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
		}),
	)
}

func (d *Draft) form(gtx layout.Context, e *env.Env, errs consoleDraftErrors) layout.Dimensions {
	labels := []string{"Serial", "Telnet", "RDP", "VNC"}
	if changed, ok := d.kinds.Changed(gtx); ok {
		d.kind = []string{"serial", "telnet", "rdp", "vnc"}[changed]
		switch d.kind {
		case "telnet":
			d.port.SetText("23")
		case "rdp":
			d.port.SetText("3389")
		case "vnc":
			d.port.SetText("5900")
		}
	}
	selected := kindIndex(d.kind)
	children := []layout.FlexChild{
		layout.Rigid(common.SectionTitle(e.Th, "Connection", nil)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.kinds.Layout(gtx, e.Th, labels, nil, selected)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.name.Layout(gtx, e.Th, "Name", "Router console", "Shown in the fleet.", errs.Name)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
	}
	if errs.Kind != "" {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Jasper}.Layout(gtx, e.Th, errs.Kind)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
		)
	}
	switch d.kind {
	case "serial":
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.device.Layout(gtx, e.Th, "Device", "/dev/ttyUSB0", "Use the device path reported by the system.", errs.Device)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.baud.Layout(gtx, e.Th, "Baud", "115200", "30 to 4000000.", errs.Baud)
			}),
		)
	default:
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.address.Layout(gtx, e.Th, "Address", "10.0.0.5", "A host name or IP address without a scheme.", errs.Address)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.port.Layout(gtx, e.Th, "Port", defaultPort(d.kind), "1 to 65535.", errs.Port)
			}),
		)
	}
	if d.kind == "rdp" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.user.Layout(gtx, e.Th, "User", "alice", "Optional account name.", errs.User)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.domain.Layout(gtx, e.Th, "Domain", "WORKGROUP", "Optional Windows domain.", errs.Domain)
			}),
		)
	}
	if d.kind == "rdp" || d.kind == "vnc" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.full.Layout(gtx, e.Th, "Open fullscreen")
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

type consoleDraftErrors struct {
	Kind    string
	Name    string
	Device  string
	Baud    string
	Address string
	Port    string
	User    string
	Domain  string
}

func (e consoleDraftErrors) Any() bool {
	return e.Kind != "" || e.Name != "" || e.Device != "" || e.Baud != "" || e.Address != "" || e.Port != "" || e.User != "" || e.Domain != ""
}

func parseConsoleDraft(id, kind, name, device, baud, address, port, user, domain string, fullscreen bool) (backend.Profile, consoleDraftErrors) {
	profile := backend.Profile{
		ID:   id,
		Kind: kind,
		Name: strings.TrimSpace(name),
	}
	var errs consoleDraftErrors
	if kind != "serial" && kind != "telnet" && kind != "rdp" && kind != "vnc" {
		errs.Kind = "Choose serial, Telnet, RDP, or VNC."
	}
	if profile.Name == "" {
		errs.Name = "Enter a name."
	} else if hasControl(profile.Name) {
		errs.Name = "The name cannot contain control characters."
	}
	checkField := func(value string, target *string) {
		if hasControl(value) {
			*target = "Remove control characters."
		} else if strings.HasPrefix(value, "-") {
			*target = "The value cannot start with a hyphen."
		}
	}
	if kind == "serial" {
		profile.Device = strings.TrimSpace(device)
		checkField(profile.Device, &errs.Device)
		if !strings.HasPrefix(profile.Device, "/dev/") || strings.ContainsAny(profile.Device, " \t") {
			errs.Device = "Use a /dev/ path without spaces."
		}
		value, err := strconv.Atoi(strings.TrimSpace(baud))
		if err != nil || value < 30 || value > 4000000 {
			errs.Baud = "Enter a baud rate from 30 to 4000000."
		} else {
			profile.Baud = value
		}
	} else if kind == "telnet" || kind == "rdp" || kind == "vnc" {
		profile.Fullscreen = (kind == "rdp" || kind == "vnc") && fullscreen
		profile.Address = strings.TrimSpace(address)
		checkField(profile.Address, &errs.Address)
		if kind == "rdp" {
			profile.User = strings.TrimSpace(user)
			profile.Domain = strings.TrimSpace(domain)
			checkField(profile.User, &errs.User)
			checkField(profile.Domain, &errs.Domain)
		}
		if !validConsoleHost(profile.Address) {
			errs.Address = "Use only letters, digits, dots, hyphens, or underscores."
		}
		value, err := strconv.Atoi(strings.TrimSpace(port))
		if err != nil || value < 1 || value > 65535 {
			errs.Port = "Enter a port from 1 to 65535."
		} else {
			profile.Port = value
		}
	}
	return profile, errs
}

func validConsoleHost(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func hasControl(value string) bool {
	return strings.ContainsAny(value, "\x00\r\n")
}

func kindIndex(kind string) int {
	switch kind {
	case "telnet":
		return 1
	case "rdp":
		return 2
	case "vnc":
		return 3
	default:
		return 0
	}
}

func defaultPort(kind string) string {
	switch kind {
	case "telnet":
		return "23"
	case "rdp":
		return "3389"
	case "vnc":
		return "5900"
	default:
		return ""
	}
}
