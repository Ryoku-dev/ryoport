package machine

import (
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type usbControl struct {
	toggle kit.Toggle
	last   bool
	init   bool
}

type sharingState struct {
	usb         map[string]*usbControl
	remove      []kit.Button
	forward     kit.TextField
	add, folder kit.Button
	err         string
}

func (s *sharingState) reset() {
	s.usb = nil
	s.remove = nil
	s.forward.SetText("")
	s.err = ""
}

func (s *sharingState) usbFor(device vm.USBDevice) *usbControl {
	if s.usb == nil {
		s.usb = make(map[string]*usbControl)
	}
	control := s.usb[device.ID]
	if control == nil {
		control = &usbControl{}
		s.usb[device.ID] = control
	}
	if !control.init || device.Assigned != control.last {
		control.toggle.Value = device.Assigned
		control.last = device.Assigned
		control.init = true
	}
	return control
}

func (s *sharingState) ensureForwards(n int) {
	if len(s.remove) == n {
		return
	}
	s.remove = make([]kit.Button, n)
	for i := range n {
		s.remove[i].Small = true
	}
}

func (v *View) layoutSharing(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	if machine == nil {
		return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Sharing details are not available.")(gtx)
	}
	editable := resource.State == model.StateStopped
	v.sharing.ensureForwards(len(machine.Forwards))
	if editable && v.sharing.add.Clicked(gtx) {
		spec := strings.TrimSpace(v.sharing.forward.Text())
		if spec == "" || !strings.Contains(spec, ":") {
			v.sharing.err = "Use host:guest, for example 2222:22."
		} else {
			e.Ops.AddForward(resource.Name, spec)
			v.sharing.forward.SetText("")
			v.sharing.err = ""
		}
	}
	if machine.Dir != "" && v.sharing.folder.Clicked(gtx) {
		e.Ops.OpenFolder(resource.Name)
	}
	if editable {
		for i, forward := range machine.Forwards {
			if v.sharing.remove[i].Clicked(gtx) {
				e.Ops.RemoveForward(resource.Name, forwardSpec(forward))
			}
		}
	}

	children := []layout.FlexChild{
		layout.Rigid(common.SectionTitle(e.Th, "USB devices", nil)),
	}
	if !editable {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Stop the machine to change USB devices or port forwards.")))
	}
	if len(machine.USB) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No assignable USB devices are connected.")))
	} else {
		for _, device := range machine.USB {
			device := device
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.usbRow(gtx, e, resource.Name, device, editable)
			}))
		}
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: 34}.Layout),
		layout.Rigid(common.SectionTitle(e.Th, "Port forwards", nil)),
	)
	if len(machine.Forwards) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No ports are forwarded into this machine.")))
	} else {
		for i, forward := range machine.Forwards {
			i, forward := i, forward
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return forwardRow(gtx, e, forward, &v.sharing.remove[i], editable)
			}))
		}
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: 13}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.forwardEditor(gtx, e, editable)
		}),
		layout.Rigid(layout.Spacer{Height: 34}.Layout),
		layout.Rigid(common.SectionTitle(e.Th, "Shared folder", nil)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return hardwareRow(e, "Machine folder", machine.Dir, func(gtx layout.Context) layout.Dimensions {
				if machine.Dir == "" {
					gtx = gtx.Disabled()
				}
				return v.sharing.folder.Layout(gtx, e.Th, kit.Line, icons.FolderOpen, "Open")
			})(gtx)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (v *View) usbRow(gtx layout.Context, e *env.Env, name string, device vm.USBDevice, editable bool) layout.Dimensions {
	control := v.sharing.usbFor(device)
	before := control.toggle.Value
	dims := hardwareRow(e, device.Name, device.ID, func(gtx layout.Context) layout.Dimensions {
		if !editable {
			gtx = gtx.Disabled()
		}
		return control.toggle.Layout(gtx, e.Th, "Attached")
	})(gtx)
	if editable && control.toggle.Value != before {
		e.Ops.SetUSB(name, device.ID, control.toggle.Value)
	}
	return dims
}

func forwardRow(gtx layout.Context, e *env.Env, forward vm.PortForward, remove *kit.Button, editable bool) layout.Dimensions {
	return hardwareRow(e, forward.Host+" to "+forward.Guest, "TCP port forward", func(gtx layout.Context) layout.Dimensions {
		if !editable {
			gtx = gtx.Disabled()
		}
		return remove.Layout(gtx, e.Th, kit.Line, icons.Close, "Remove")
	})(gtx)
}

func (v *View) forwardEditor(gtx layout.Context, e *env.Env, editable bool) layout.Dimensions {
	if !editable {
		gtx = gtx.Disabled()
	}
	field := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
		v.sharing.forward.Mono = true
		return v.sharing.forward.Layout(gtx, e.Th, "Add a port forward", "2222:22", "Host port first, guest port second.", v.sharing.err)
	}
	if gtx.Constraints.Max.X < gtx.Dp(520) {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
			layout.Rigid(field),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.sharing.add.Layout(gtx, e.Th, kit.Gold, icons.Add, "Add")
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(13)}.Layout(gtx,
		layout.Flexed(1, field),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.sharing.add.Layout(gtx, e.Th, kit.Gold, icons.Add, "Add")
		}),
	)
}

func forwardSpec(forward vm.PortForward) string {
	if forward.Host == "" {
		return forward.Guest
	}
	return forward.Host + ":" + forward.Guest
}
