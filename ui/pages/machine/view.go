package machine

import (
	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var machineTabs = [...]string{"Overview", "Hardware", "Snapshots", "Sharing"}

type View struct {
	header common.Header
	pane   common.Pane

	acts       actState
	hardware   hardwareState
	snapshots  snapshotState
	sharing    sharingState
	net        networkState
	install    kit.Button
	machineKey string
}

func New() *View { return &View{} }

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if index, ok := v.pane.Tab(gtx); ok && index >= 0 && index < len(machineTabs) {
		e.Nav.SetTab(model.KindMachine, machineTabs[index])
	}

	id := e.Nav.Selected
	resource, ok := e.Snap.Resource(id)
	if !ok || resource.Kind != model.KindMachine {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	machine := e.Snap.Machines[id.Key()]
	job, making := e.Snap.JobFor(id)
	if v.machineKey != id.Key() {
		v.machineKey = id.Key()
		v.acts.reset()
		v.hardware.reset()
		v.snapshots.reset()
		v.sharing.reset()
		v.net.reset()
	}
	v.acts.handleClicks(gtx, e, resource, machine, job, making)

	outline := -1.0
	tone := resource.State.Tone()
	proof := proofFor(machine, resource, e.Snap.Events, e.Now)
	if making {
		tone = model.ToneBusy
		outline = max(0, min(1, job.Progress))
		proof = jobProof(job)
	}

	mark := common.Mark(e, resource, 44, e.Th.R.Fg)
	header := func(gtx layout.Context) layout.Dimensions {
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    tone,
			Mark:    mark,
			Name:    resource.Name,
			Outline: outline,
			Proof:   proof,
			Note:    startedNote(resource, e.Snap.Events, e.Now),
			Acts:    v.actsWidget(e, resource, machine, job, making),
			Below:   v.acts.belowWidget(e, resource.Name),
		})
	}

	if making {
		return v.pane.Layout(gtx, e, header, nil, 0, func(gtx layout.Context) layout.Dimensions {
			return v.layoutBecoming(gtx, e, machine, job)
		})
	}

	selected := tabIndex(e.Nav.Tab(model.KindMachine))
	body := v.tabBody(e, resource, machine, selected)
	if !e.Snap.Host.Engine.Ready {
		body = v.engineMissing(e)
	}
	return v.pane.Layout(gtx, e, header, machineTabs[:], selected, body)
}

func (v *View) tabBody(e *env.Env, resource model.Resource, machine *store.Machine, selected int) layout.Widget {
	switch selected {
	case 1:
		return func(gtx layout.Context) layout.Dimensions {
			return v.layoutHardware(gtx, e, resource, machine)
		}
	case 2:
		return func(gtx layout.Context) layout.Dimensions {
			return v.layoutSnapshots(gtx, e, resource, machine)
		}
	case 3:
		return func(gtx layout.Context) layout.Dimensions {
			return v.layoutSharing(gtx, e, resource, machine)
		}
	default:
		return func(gtx layout.Context) layout.Dimensions {
			return v.layoutOverview(gtx, e, resource, machine)
		}
	}
}

func (v *View) engineMissing(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if v.install.Clicked(gtx) {
			e.Ops.InstallEngine()
		}
		empty := kit.Empty(e.Th, icons.Settings, "The machine engine is not ready", "Install Quickemu and its display tools before starting or changing local machines.", kit.EmptyAction{
			Button: &v.install,
			Kind:   kit.Gold,
			Icon:   icons.Settings,
			Label:  "Install the engine",
		})
		return empty(gtx)
	}
}

func tabIndex(tab string) int {
	for i, candidate := range machineTabs {
		if tab == candidate {
			return i
		}
	}
	return 0
}
