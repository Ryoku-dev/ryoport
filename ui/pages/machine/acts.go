package machine

import (
	"fmt"
	"strconv"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type actID uint8

const (
	actStart actID = iota
	actOpen
	actTerminal
	actPause
	actResume
	actStop
	actSnapshots
	actHardware
	actCancel
	actOverflow
)

func availableActs(state model.State, making, cancelable bool) []actID {
	if making {
		if cancelable {
			return []actID{actCancel}
		}
		return nil
	}
	switch state {
	case model.StateStopped:
		return []actID{actStart, actSnapshots, actHardware, actOverflow}
	case model.StateAbsent:
		return []actID{actStart, actHardware, actOverflow}
	case model.StateRunning:
		return []actID{actOpen, actTerminal, actPause, actStop, actOverflow}
	case model.StatePaused:
		return []actID{actOpen, actTerminal, actResume, actStop, actOverflow}
	case model.StateStarting, model.StateStopping:
		return []actID{actStop, actOverflow}
	case model.StateError:
		return []actID{actOverflow}
	default:
		return []actID{actOverflow}
	}
}

type actState struct {
	start, open                          kit.SplitButton
	startMenu, openMenu, moreMenu        kit.Menu
	startPop, openPop, morePop           kit.Popover
	terminal, pause, stop                kit.Button
	snapshots, hardware, cancel          kit.Button
	more                                 kit.IconButton
	rename, resize, template             kit.TextField
	renameSave, renameCancel             kit.Button
	resizeSave, resizeCancel             kit.Button
	templateSave, templateCancel         kit.Button
	renameOpen, resizeOpen, templateOpen bool
	renameErr, resizeErr, templateErr    string
}

func (a *actState) reset() {
	a.startPop.Open = false
	a.openPop.Open = false
	a.morePop.Open = false
	a.renameOpen = false
	a.resizeOpen = false
	a.templateOpen = false
	a.renameErr = ""
	a.resizeErr = ""
	a.templateErr = ""
}

func (v *View) actsWidget(e *env.Env, resource model.Resource, machine *store.Machine, job model.Job, making bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		acts := availableActs(resource.State, making, job.Cancelable)
		widgets := make([]layout.Widget, 0, len(acts))
		for _, act := range acts {
			widgets = append(widgets, v.acts.widgetFor(e, resource, machine, job, act))
		}
		return layoutActs(gtx, widgets)
	}
}

func (a *actState) widgetFor(e *env.Env, resource model.Resource, machine *store.Machine, job model.Job, act actID) layout.Widget {
	switch act {
	case actStart:
		return func(gtx layout.Context) layout.Dimensions {
			items := []kit.MenuItem{
				{Icon: icons.DesktopWindows, Label: "In a window"},
				{Icon: icons.ScreenShare, Label: "With SPICE", Disabled: !e.Snap.Host.Engine.Spice},
				{Icon: icons.Terminal, Label: "Headless"},
				{Icon: icons.ContentCut, Label: "Disposable run"},
			}
			return a.startPop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if !e.Snap.Host.Engine.Ready {
					gtx = gtx.Disabled()
				}
				return a.start.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Start", a.startPop.Open)
			}, func(gtx layout.Context) layout.Dimensions {
				return a.startMenu.Layout(gtx, e.Th, items)
			})
		}
	case actOpen:
		return func(gtx layout.Context) layout.Dimensions {
			items := []kit.MenuItem{
				{Icon: icons.DesktopWindows, Label: "Open"},
				{Icon: icons.Terminal, Label: "Terminal over SSH", Disabled: machine == nil || !machine.SSHReady},
			}
			return a.openPop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.open.Layout(gtx, e.Th, kit.Gold, icons.DesktopWindows, "Open", a.openPop.Open)
			}, func(gtx layout.Context) layout.Dimensions {
				return a.openMenu.Layout(gtx, e.Th, items)
			})
		}
	case actTerminal:
		return func(gtx layout.Context) layout.Dimensions {
			if machine == nil || !machine.SSHReady {
				gtx = gtx.Disabled()
			}
			return a.terminal.Layout(gtx, e.Th, kit.Line, icons.Terminal, "Terminal")
		}
	case actPause:
		return func(gtx layout.Context) layout.Dimensions {
			return a.pause.Layout(gtx, e.Th, kit.Line, icons.Pause, "Pause")
		}
	case actResume:
		return func(gtx layout.Context) layout.Dimensions {
			return a.pause.Layout(gtx, e.Th, kit.Line, icons.PlayArrow, "Resume")
		}
	case actStop:
		return func(gtx layout.Context) layout.Dimensions {
			return a.stop.Layout(gtx, e.Th, kit.DangerLine, icons.Stop, "Stop")
		}
	case actSnapshots:
		return func(gtx layout.Context) layout.Dimensions {
			return a.snapshots.Layout(gtx, e.Th, kit.Line, icons.Backup, "Snapshots")
		}
	case actHardware:
		return func(gtx layout.Context) layout.Dimensions {
			return a.hardware.Layout(gtx, e.Th, kit.Line, icons.Settings, "Settings")
		}
	case actCancel:
		return func(gtx layout.Context) layout.Dimensions {
			return a.cancel.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
		}
	default:
		return func(gtx layout.Context) layout.Dimensions {
			items := overflowItems(resource, machine)
			return a.morePop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.more.Layout(gtx, e.Th, kit.Quiet, icons.MoreHoriz, "More actions")
			}, func(gtx layout.Context) layout.Dimensions {
				return a.moreMenu.Layout(gtx, e.Th, items)
			})
		}
	}
}

func (a *actState) handleClicks(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine, job model.Job, making bool) {
	name := resource.Name
	if e.Snap.Host.Engine.Ready && a.start.Main.Clicked(gtx) {
		e.Ops.Start(name, store.StartDisplay(e.Snap), false)
	}
	if a.start.More.Clicked(gtx) {
		a.startPop.Open = !a.startPop.Open
	}
	if index, ok := a.startMenu.Clicked(gtx); ok && e.Snap.Host.Engine.Ready {
		a.startPop.Open = false
		switch index {
		case 0:
			e.Ops.Start(name, vm.DisplayWindow, false)
		case 1:
			if e.Snap.Host.Engine.Spice {
				e.Ops.Start(name, vm.DisplaySpice, false)
			}
		case 2:
			e.Ops.Start(name, vm.DisplayHeadless, false)
		case 3:
			e.Ops.Start(name, vm.DisplayWindow, true)
		}
	}
	if a.open.Main.Clicked(gtx) {
		e.Ops.OpenViewer(name)
	}
	if a.open.More.Clicked(gtx) {
		a.openPop.Open = !a.openPop.Open
	}
	if index, ok := a.openMenu.Clicked(gtx); ok {
		a.openPop.Open = false
		if index == 0 {
			e.Ops.OpenViewer(name)
		} else if index == 1 && machine != nil && machine.SSHReady {
			e.Ops.OpenSSH(name)
		}
	}
	if machine != nil && machine.SSHReady && a.terminal.Clicked(gtx) {
		e.Ops.OpenSSH(name)
	}
	if a.pause.Clicked(gtx) {
		if resource.State == model.StatePaused {
			e.Ops.Power(name, qmp.PowerResume)
		} else {
			e.Ops.Power(name, qmp.PowerPause)
		}
	}
	if a.stop.Clicked(gtx) {
		e.Ops.Stop(name, false)
	}
	if a.snapshots.Clicked(gtx) {
		e.Nav.SetTab(model.KindMachine, "Snapshots")
	}
	if a.hardware.Clicked(gtx) {
		e.Nav.SetTab(model.KindMachine, "Hardware")
	}
	if making && a.cancel.Clicked(gtx) {
		e.Ops.Cancel(job.ID)
	}
	if a.more.Clicked(gtx) {
		a.morePop.Open = !a.morePop.Open
	}
	items := overflowItems(resource, machine)
	if index, ok := a.moreMenu.Clicked(gtx); ok && index >= 0 && index < len(items) {
		a.morePop.Open = false
		a.runOverflow(e, resource, machine, items[index].Label)
	}
}

func (a *actState) runOverflow(e *env.Env, resource model.Resource, machine *store.Machine, label string) {
	name := resource.Name
	switch label {
	case "Pin", "Unpin":
		e.Ops.SetPinned(resource.ID, !resource.Pinned)
	case "Rename":
		a.rename.SetText(name)
		a.renameErr = ""
		a.renameOpen = true
		a.resizeOpen = false
		a.templateOpen = false
	case "Resize disk":
		if machine != nil {
			a.resize.SetText(machine.Disk)
			a.resize.Mono = true
		}
		a.resizeErr = ""
		a.resizeOpen = true
		a.renameOpen = false
		a.templateOpen = false
	case "Make a template":
		a.template.SetText(name + "-template")
		a.templateErr = ""
		a.templateOpen = true
		a.renameOpen = false
		a.resizeOpen = false
	case "Seal":
		e.Ops.Seal(name)
	case "Restore seal":
		e.Ops.RestoreSeal(name)
	case "Open folder":
		e.Ops.OpenFolder(name)
	case "Force stop":
		e.Nav.Ask(env.Confirm{
			Title:  "Force stop " + name + "?",
			Body:   "Force stopping can lose unsaved work inside the machine.",
			Action: "Force stop",
			Run:    func() { e.Ops.Stop(name, true) },
		})
	case "Delete":
		snapshots := 0
		disk := "its disk"
		if machine != nil {
			snapshots = len(userSnapshots(machine.Snapshots))
			if machine.Disk != "" {
				disk = displaySize(machine.Disk) + " disk"
			} else if machine.DiskUsed > 0 {
				disk = format.Bytes(machine.DiskUsed) + " disk"
			}
		}
		body := fmt.Sprintf("Deleting removes the machine, its %d %s and its %s. It cannot be brought back.", snapshots, plural(snapshots, "snapshot", "snapshots"), disk)
		e.Nav.Ask(env.Confirm{
			Title:         "Delete " + name + "?",
			Body:          body,
			Action:        "Delete " + name,
			TypeToConfirm: name,
			Run:           func() { e.Ops.Delete(name, false) },
		})
	}
}

func (a *actState) belowWidget(e *env.Env, name string) layout.Widget {
	if !a.renameOpen && !a.resizeOpen && !a.templateOpen {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		if a.renameOpen {
			if a.renameSave.Clicked(gtx) {
				newName := strings.TrimSpace(a.rename.Text())
				if !vm.ValidName(newName) {
					a.renameErr = "Use letters, numbers, dots, dashes or underscores."
				} else if newName == name {
					a.renameOpen = false
				} else {
					e.Ops.Rename(name, newName)
					a.renameOpen = false
				}
			}
			if a.renameCancel.Clicked(gtx) {
				a.renameOpen = false
			}
			return inlineEditor(gtx, e, &a.rename, "New name", "machine-name", a.renameErr, &a.renameSave, "Rename", &a.renameCancel)
		}
		if a.resizeOpen {
			if a.resizeSave.Clicked(gtx) {
				size := strings.ToUpper(strings.TrimSpace(a.resize.Text()))
				if !validDiskSize(size) {
					a.resizeErr = "Use a size such as 96G or 4096M."
				} else {
					e.Ops.Resize(name, size)
					a.resizeOpen = false
				}
			}
			if a.resizeCancel.Clicked(gtx) {
				a.resizeOpen = false
			}
			return inlineEditor(gtx, e, &a.resize, "New disk size", "96G", a.resizeErr, &a.resizeSave, "Resize disk", &a.resizeCancel)
		}
		if a.templateSave.Clicked(gtx) {
			template := strings.TrimSpace(a.template.Text())
			if !vm.ValidName(template) {
				a.templateErr = "Use letters, numbers, dots, dashes or underscores."
			} else {
				e.Ops.MakeTemplate(name, template)
				a.templateOpen = false
			}
		}
		if a.templateCancel.Clicked(gtx) {
			a.templateOpen = false
		}
		return inlineEditor(gtx, e, &a.template, "Template name", "machine-template", a.templateErr, &a.templateSave, "Make template", &a.templateCancel)
	}
}

func inlineEditor(gtx layout.Context, e *env.Env, field *kit.TextField, label, placeholder, errText string, save *kit.Button, saveLabel string, cancel *kit.Button) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
			return field.Layout(gtx, e.Th, label, placeholder, "", errText)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return save.Layout(gtx, e.Th, kit.Gold, icons.Save, saveLabel)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return cancel.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
				}),
			)
		}),
	)
}

func validDiskSize(value string) bool {
	if len(value) < 2 {
		return false
	}
	suffix := value[len(value)-1]
	if suffix != 'G' && suffix != 'M' {
		return false
	}
	size, err := strconv.Atoi(value[:len(value)-1])
	return err == nil && size > 0
}

func overflowItems(resource model.Resource, machine *store.Machine) []kit.MenuItem {
	seal := "Seal"
	sealIcon := icons.Lock
	if machine != nil && machine.Sealed {
		seal = "Restore seal"
		sealIcon = icons.LockOpen
	}
	pin := "Pin"
	if resource.Pinned {
		pin = "Unpin"
	}
	state := resource.State
	inactive := state == model.StateStopped || state == model.StateAbsent || state == model.StateError || state == model.StateUnknown
	installed := machine != nil && machine.Installed
	items := []kit.MenuItem{
		{Icon: icons.Edit, Label: "Rename", Disabled: machine == nil || !inactive},
		{Icon: icons.PushPin, Label: pin},
		{Icon: icons.HardDrive, Label: "Resize disk", Disabled: state != model.StateStopped || !installed},
		{Icon: icons.ContentCopy, Label: "Make a template", Disabled: state != model.StateStopped || !installed},
		{Icon: sealIcon, Label: seal, Disabled: state != model.StateStopped || !installed},
		{Icon: icons.FolderOpen, Label: "Open folder", Disabled: machine == nil},
	}
	if state == model.StateRunning || state == model.StatePaused || state == model.StateStarting || state == model.StateStopping {
		items = append(items, kit.MenuItem{Divider: true}, kit.MenuItem{Icon: icons.StopCircle, Label: "Force stop", Danger: true})
	}
	items = append(items, kit.MenuItem{Divider: true}, kit.MenuItem{Icon: icons.Delete, Label: "Delete", Danger: true, Disabled: !inactive})
	return items
}

func layoutActs(gtx layout.Context, widgets []layout.Widget) layout.Dimensions {
	if len(widgets) == 0 {
		return layout.Dimensions{}
	}
	if gtx.Constraints.Max.X >= gtx.Dp(560) || len(widgets) < 4 {
		children := make([]layout.FlexChild, 0, len(widgets))
		for _, widget := range widgets {
			children = append(children, layout.Rigid(widget))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(10)}.Layout(gtx, children...)
	}
	cut := 2
	if len(widgets) > 4 {
		cut = 3
	}
	row := func(widgets []layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(widgets))
			for _, widget := range widgets {
				children = append(children, layout.Rigid(widget))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(10)}.Layout(gtx, children...)
		}
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(row(widgets[:cut])),
		layout.Rigid(row(widgets[cut:])),
	)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
