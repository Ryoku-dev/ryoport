package console

import (
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
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

	advanced           bool
	advancedOpen       kit.Button
	advancedDisclosure kit.Disclosure

	save   kit.Button
	cancel kit.Button

	loaded       bool
	loadedTarget model.ID
	focusFirst   bool
	kind         string
	profileID    string
	auth         string

	validation struct {
		name    consoleFieldValidation
		device  consoleFieldValidation
		baud    consoleFieldValidation
		address consoleFieldValidation
		port    consoleFieldValidation
		user    consoleFieldValidation
		domain  consoleFieldValidation
	}
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
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil || e.Nav.Draft.Kind != env.DraftConsole {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	d.load(e)
	submitted := consumeConsoleSubmit(gtx, &d.name, &d.device, &d.baud, &d.address, &d.port, &d.user, &d.domain)
	profile, errs := d.profile()
	d.observeValidation(gtx)
	visibleErrs := consoleDraftErrors{
		Kind:    errs.Kind,
		Name:    d.validation.name.visible(errs.Name),
		Device:  d.validation.device.visible(errs.Device),
		Baud:    d.validation.baud.visible(errs.Baud),
		Address: d.validation.address.visible(errs.Address),
		Port:    d.validation.port.visible(errs.Port),
		User:    d.validation.user.visible(errs.User),
		Domain:  d.validation.domain.visible(errs.Domain),
	}

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
				return d.actions(gtx, e, profile, errs, submitted)
			},
		})
	}
	form := func(gtx layout.Context) layout.Dimensions {
		return d.form(gtx, e, visibleErrs)
	}
	aside := func(gtx layout.Context) layout.Dimensions {
		previewProfile, _ := d.profile()
		rows := []layout.Widget{
			common.AsidePreview(e.Th, consoleDraftPreview(previewProfile)),
			common.AsideNote(e.Th, consoleReachabilityNote(previewProfile.Kind)),
		}
		if previewProfile.Auth == "password" && (previewProfile.Kind == "rdp" || previewProfile.Kind == "vnc") {
			rows = append(rows, common.AsideNote(e.Th, "The password stays in the system keyring and is never added to this command line."))
		}
		return common.Aside(gtx, e.Th, "How Ryoport connects", rows...)
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return common.DraftColumns(gtx, e.Th, form, aside)
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
	d.auth = ""
	d.name.SetText("")
	d.device.SetText("")
	d.baud.SetText("115200")
	d.address.SetText("")
	d.port.SetText("")
	d.user.SetText("")
	d.domain.SetText("")
	d.full.Value = false
	d.advanced = false
	d.advancedDisclosure = kit.Disclosure{}
	if target != "" {
		if stored := e.Snap.Consoles[target.Key()]; stored != nil {
			profile := stored.Profile
			d.profileID = profile.ID
			d.kind = profile.Kind
			d.name.SetText(profile.Name)
			d.auth = profile.Auth
			d.device.SetText(profile.Device)
			d.baud.SetText(strconv.Itoa(profile.Baud))
			d.address.SetText(profile.Address)
			d.port.SetText(strconv.Itoa(profile.Port))
			d.user.SetText(profile.User)
			d.domain.SetText(profile.Domain)
			d.full.Value = profile.Fullscreen
		}
	}
	d.validation.name.reset(d.name.Text())
	d.validation.device.reset(d.device.Text())
	d.validation.baud.reset(d.baud.Text())
	d.validation.address.reset(d.address.Text())
	d.validation.port.reset(d.port.Text())
	d.validation.user.reset(d.user.Text())
	d.validation.domain.reset(d.domain.Text())
	d.focusFirst = true
}

func (d *Draft) profile() (backend.Profile, consoleDraftErrors) {
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
	profile.Auth = d.auth
	return profile, errs
}

func (d *Draft) actions(gtx layout.Context, e *env.Env, profile backend.Profile, errs consoleDraftErrors, submitted bool) layout.Dimensions {
	saveClicked := d.save.Clicked(gtx)
	// Save uses full validation and stays disabled; field errors wait until an edited field loses focus.
	if !errs.Any() && (saveClicked || submitted) {
		if profile.ID == "" {
			profile.ID = backend.NewID()
		}
		e.Ops.SaveConsole(profile)
		e.Nav.CloseDraft()
		e.Nav.Select(model.NewID(model.KindConsole, profile.ID))
		d.loaded = false
	}
	if d.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
		d.loaded = false
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
		d.advanced = false
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
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(common.SectionTitle(e.Th, "Connection", nil)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.kinds.Layout(gtx, e.Th, labels, nil, selected)
			}),
			layout.Rigid(layout.Spacer{Height: e.Th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims := layoutConsoleField(gtx, &d.validation.name, &d.name, errs.Name, func(errText string) layout.Dimensions {
					return d.name.Layout(gtx, e.Th, "Name", "Router console", "Shown in the fleet.", errText)
				})
				if d.focusFirst {
					gtx.Execute(key.FocusCmd{Tag: &d.name.Editor})
					d.focusFirst = false
				}
				return dims
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
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return consoleFieldPair(gtx, e,
					func(gtx layout.Context) layout.Dimensions {
						return layoutConsoleField(gtx, &d.validation.device, &d.device, errs.Device, func(errText string) layout.Dimensions {
							return d.device.Layout(gtx, e.Th, "Device", "/dev/ttyUSB0", "Use the device path reported by the system.", errText)
						})
					},
					func(gtx layout.Context) layout.Dimensions {
						return layoutConsoleField(gtx, &d.validation.baud, &d.baud, errs.Baud, func(errText string) layout.Dimensions {
							return d.baud.Layout(gtx, e.Th, "Baud", "115200", "30 to 4000000.", errText)
						})
					},
				)
			}))
		default:
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return consoleFieldPair(gtx, e,
					func(gtx layout.Context) layout.Dimensions {
						return layoutConsoleField(gtx, &d.validation.address, &d.address, errs.Address, func(errText string) layout.Dimensions {
							return d.address.Layout(gtx, e.Th, "Address", "10.0.0.5", "A host name or IP address without a scheme.", errText)
						})
					},
					func(gtx layout.Context) layout.Dimensions {
						return layoutConsoleField(gtx, &d.validation.port, &d.port, errs.Port, func(errText string) layout.Dimensions {
							return d.port.Layout(gtx, e.Th, "Port", defaultPort(d.kind), "1 to 65535.", errText)
						})
					},
				)
			}))
		}
		if d.kind == "rdp" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
				layout.Rigid(common.SectionTitle(e.Th, "Authentication", nil)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return consoleFieldPair(gtx, e,
						func(gtx layout.Context) layout.Dimensions {
							return layoutConsoleField(gtx, &d.validation.user, &d.user, errs.User, func(errText string) layout.Dimensions {
								return d.user.Layout(gtx, e.Th, "User", "alice", "Optional account name.", errText)
							})
						},
						func(gtx layout.Context) layout.Dimensions {
							return layoutConsoleField(gtx, &d.validation.domain, &d.domain, errs.Domain, func(errText string) layout.Dimensions {
								return d.domain.Layout(gtx, e.Th, "Domain", "WORKGROUP", "Optional Windows domain.", errText)
							})
						},
					)
				}),
			)
		}
		if d.kind == "rdp" || d.kind == "vnc" {
			advancedAction := func(gtx layout.Context) layout.Dimensions {
				if d.advancedOpen.Clicked(gtx) {
					d.advanced = !d.advanced
				}
				label := "Show"
				icon := icons.ExpandMore
				if d.advanced {
					label = "Hide"
					icon = icons.ExpandLess
				}
				d.advancedOpen.Small = true
				return d.advancedOpen.Layout(gtx, e.Th, kit.Quiet, icon, label)
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
				layout.Rigid(common.SectionTitle(e.Th, "Advanced", advancedAction)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.advancedDisclosure.Layout(gtx, e.Th, d.advanced, func(gtx layout.Context) layout.Dimensions {
						return d.full.Layout(gtx, e.Th, "Open fullscreen")
					})
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}(gtx)
}

func consoleFieldPair(gtx layout.Context, e *env.Env, first, second layout.Widget) layout.Dimensions {
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520)) {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(first),
			layout.Rigid(second),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
		layout.Flexed(1, first),
		layout.Flexed(1, second),
	)
}

func consoleDraftPreview(profile backend.Profile) string {
	binary := ""
	switch profile.Kind {
	case "serial":
		binary = "picocom"
	case "telnet":
		binary = "telnet"
	case "rdp":
		binary = "xfreerdp3"
	case "vnc":
		binary = "vncviewer"
	}
	argv := backend.CommandArgv(binary, profile)
	var line strings.Builder
	for i, arg := range argv {
		if i > 0 {
			line.WriteByte(' ')
		}
		if arg == "" || strings.ContainsAny(arg, " \t\r\n\"'\\$`;&|()<>*?![]{}") {
			line.WriteString(strconv.Quote(arg))
		} else {
			line.WriteString(arg)
		}
	}
	return line.String()
}

func consoleReachabilityNote(kind string) string {
	if kind == "serial" {
		return "The fleet checks that this device exists and can be read and written."
	}
	return "The fleet checks whether it can open a TCP connection to this address and port."
}

func layoutConsoleField(gtx layout.Context, state *consoleFieldValidation, field *kit.TextField, errText string, widget func(string) layout.Dimensions) layout.Dimensions {
	state.observeField(gtx, field)
	dims := widget(state.visible(errText))
	state.observeField(gtx, field)
	return dims
}

type consoleFieldValidation struct {
	last                  string
	edited, focused, show bool
}

func (v *consoleFieldValidation) reset(value string) {
	*v = consoleFieldValidation{last: value}
}

func (v *consoleFieldValidation) observe(value string, focused bool) bool {
	wasVisible := v.show
	if value != v.last {
		v.last = value
		v.edited = true
	}
	if v.focused && !focused && v.edited {
		v.show = true
	}
	v.focused = focused
	return !wasVisible && v.show
}

func (v *consoleFieldValidation) observeField(gtx layout.Context, field *kit.TextField) {
	if v.observe(field.Text(), gtx.Focused(&field.Editor)) {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (v consoleFieldValidation) visible(errText string) string {
	if v.show {
		return errText
	}
	return ""
}

func (d *Draft) observeValidation(gtx layout.Context) {
	d.validation.name.observeField(gtx, &d.name)
	d.validation.device.observeField(gtx, &d.device)
	d.validation.baud.observeField(gtx, &d.baud)
	d.validation.address.observeField(gtx, &d.address)
	d.validation.port.observeField(gtx, &d.port)
	d.validation.user.observeField(gtx, &d.user)
	d.validation.domain.observeField(gtx, &d.domain)
}

func consumeConsoleSubmit(gtx layout.Context, fields ...*kit.TextField) bool {
	submitted := false
	for _, field := range fields {
		field.Submit = true
		for {
			event, ok := field.Update(gtx)
			if !ok {
				break
			}
			if _, ok := event.(widget.SubmitEvent); ok {
				submitted = true
			}
		}
	}
	return submitted
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
