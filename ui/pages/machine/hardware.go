package machine

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var displayOptions = [...]string{"Window", "SPICE", "Headless"}

type hardwareState struct {
	cores, ram, disk        kit.Stepper
	display                 kit.Picker
	tpm, secureBoot         kit.Toggle
	displayIndex            int
	lastCores, lastRAM      int
	lastDisk                int
	lastDisplay             string
	lastTPM, lastSecureBoot bool
	initialized             bool
}

func (h *hardwareState) reset() { h.initialized = false }

func (h *hardwareState) sync(machine *store.Machine) {
	if machine == nil {
		return
	}
	cores := leadingInt(machine.Cores)
	ram := sizeGB(machine.RAM)
	disk := sizeGB(machine.Disk)
	display := displayConfig(machine.Display)
	if !h.initialized || cores != h.lastCores {
		h.cores.Value = max(1, cores)
		h.lastCores = cores
	}
	if !h.initialized || ram != h.lastRAM {
		h.ram.Value = max(1, ram)
		h.lastRAM = ram
	}
	if !h.initialized || disk != h.lastDisk {
		h.disk.Value = max(1, disk)
		h.lastDisk = disk
	}
	if !h.initialized || display != h.lastDisplay {
		h.displayIndex = displayOptionIndex(display)
		h.lastDisplay = display
	}
	if !h.initialized || machine.TPM != h.lastTPM {
		h.tpm.Value = machine.TPM
		h.lastTPM = machine.TPM
	}
	if !h.initialized || machine.SecureBoot != h.lastSecureBoot {
		h.secureBoot.Value = machine.SecureBoot
		h.lastSecureBoot = machine.SecureBoot
	}
	h.cores.Min, h.cores.Max, h.cores.Step = 1, 64, 1
	h.ram.Min, h.ram.Max, h.ram.Step = 1, 256, 1
	h.disk.Min, h.disk.Max, h.disk.Step = 8, 4096, 8
	h.initialized = true
}

func (v *View) layoutHardware(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	if machine == nil {
		return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Hardware details are not available.")(gtx)
	}
	v.hardware.sync(machine)
	editable := resource.State == model.StateStopped || resource.State == model.StateAbsent
	beforeTPM, beforeSecure := v.hardware.tpm.Value, v.hardware.secureBoot.Value
	controlGTX := func(gtx layout.Context) layout.Context {
		if !editable {
			return gtx.Disabled()
		}
		return gtx
	}

	children := []layout.FlexChild{
		layout.Rigid(common.SectionTitle(e.Th, "Hardware", nil)),
	}
	if !editable {
		children = append(children,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Stop the machine to change its hardware.")),
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
		)
	}
	children = append(children,
		layout.Rigid(hardwareRow(e, "Processor", "Virtual CPU cores", func(gtx layout.Context) layout.Dimensions {
			return v.hardware.cores.Layout(controlGTX(gtx), e.Th, "cores")
		})),
		layout.Rigid(hardwareRow(e, "Memory", "Memory reserved for the guest", func(gtx layout.Context) layout.Dimensions {
			return v.hardware.ram.Layout(controlGTX(gtx), e.Th, "GB")
		})),
		layout.Rigid(hardwareRow(e, "Disk size", "The configured virtual disk capacity", func(gtx layout.Context) layout.Dimensions {
			return v.hardware.disk.Layout(controlGTX(gtx), e.Th, "GB")
		})),
		layout.Rigid(hardwareRow(e, "Display", "How the machine opens", func(gtx layout.Context) layout.Dimensions {
			pickerGTX := controlGTX(gtx)
			pickerGTX.Constraints.Max.X = min(pickerGTX.Constraints.Max.X, pickerGTX.Dp(240))
			return v.hardware.display.Layout(pickerGTX, e.Th, "", displayOptions[:], v.hardware.displayIndex)
		})),
		layout.Rigid(hardwareRow(e, "TPM", "A virtual trusted platform module", func(gtx layout.Context) layout.Dimensions {
			return v.hardware.tpm.Layout(controlGTX(gtx), e.Th, "")
		})),
		layout.Rigid(hardwareRow(e, "Secure boot", "Verify the guest boot chain", func(gtx layout.Context) layout.Dimensions {
			return v.hardware.secureBoot.Layout(controlGTX(gtx), e.Th, "")
		})),
	)
	facts := []kit.Fact{
		{Label: "Guest address", Value: guestIP(machine), Mono: true},
		{Label: "Guest OS", Value: guestOS(e, machine)},
		{Label: "Uptime", Value: uptimeValue(machine), Mono: true},
		{Label: "Display", Value: displayName(machine.Display)},
		{Label: "Snapshots", Value: strconv.Itoa(len(userSnapshots(machine.Snapshots))), Mono: true},
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: 21}.Layout),
		layout.Rigid(common.SectionTitle(e.Th, "Machine facts", nil)),
		layout.Rigid(kit.Facts(e.Th, facts)),
	)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	if editable {
		v.hardware.apply(gtx, e, resource.Name, beforeTPM, beforeSecure)
	}
	return dims
}

func (h *hardwareState) apply(gtx layout.Context, e *env.Env, name string, beforeTPM, beforeSecure bool) {
	if h.cores.Changed() {
		e.Ops.SetConfig(name, "cpu_cores", strconv.Itoa(h.cores.Value))
	}
	if h.ram.Changed() {
		e.Ops.SetConfig(name, "ram", strconv.Itoa(h.ram.Value)+"G")
	}
	if h.disk.Changed() {
		e.Ops.SetConfig(name, "disk_size", strconv.Itoa(h.disk.Value)+"G")
	}
	if index, ok := h.display.Changed(gtx); ok && index >= 0 && index < len(displayOptions) {
		h.displayIndex = index
		e.Ops.SetConfig(name, "display", displayConfigValue(index))
	}
	if h.tpm.Value != beforeTPM {
		e.Ops.SetConfig(name, "tpm", onOff(h.tpm.Value))
	}
	if h.secureBoot.Value != beforeSecure {
		e.Ops.SetConfig(name, "secureboot", onOff(h.secureBoot.Value))
	}
}

func hardwareRow(e *env.Env, label, detail string, control layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, label)),
				layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, detail)),
			)
		}
		var dims layout.Dimensions
		if gtx.Constraints.Max.X < gtx.Dp(480) {
			dims = layout.Inset{Top: 13, Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Rigid(content),
					layout.Rigid(control),
				)
			})
		} else {
			dims = layout.Inset{Top: 13, Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, content),
					layout.Rigid(control),
				)
			})
		}
		paint.FillShape(gtx.Ops, e.Th.R.Vein, clip.Rect{Min: image.Pt(0, dims.Size.Y-1), Max: image.Pt(dims.Size.X, dims.Size.Y)}.Op())
		return dims
	}
}

func displayConfig(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "spice":
		return "spice"
	case "none", "headless":
		return "headless"
	default:
		return "window"
	}
}

func displayOptionIndex(value string) int {
	switch displayConfig(value) {
	case "spice":
		return 1
	case "headless":
		return 2
	default:
		return 0
	}
}

func displayConfigValue(index int) string {
	switch index {
	case 1:
		return "spice"
	case 2:
		return "none"
	default:
		return "gtk"
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
