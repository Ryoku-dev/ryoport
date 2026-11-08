package passthrough

import (
	"fmt"
	"math"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/ryoku-dev/ryoport/internal/model"
	backend "github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var cpuTicks = []string{"0", "50%", "100%"}

// View shows one GPU-passthrough machine.
type View struct {
	pane   common.Pane
	header common.Header

	primary kit.Button
	stop    kit.Button
	gpu     kit.Button
	more    kit.IconButton

	overflow kit.Popover
	menu     kit.Menu

	blockerGPU kit.Button
}

func New() *View { return &View{} }

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	id := e.Nav.Selected
	resource, ok := e.Snap.Resource(id)
	stored := e.Snap.Passthrough[id.Key()]
	if !ok || resource.Kind != model.KindPassthrough || stored == nil {
		return v.missing(gtx, e)
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    resource.State.Tone(),
			Mark:    common.Mark(e, resource, unit.Dp(44), e.Th.R.Fg),
			Name:    resource.Name,
			Outline: -1,
			Proof:   passthroughProof(stored.Machine),
			Acts: func(gtx layout.Context) layout.Dimensions {
				return v.actions(gtx, e, resource, stored.Machine)
			},
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		if !e.Snap.GPU.Ready {
			return v.blocker(gtx, e)
		}
		return v.body(gtx, e, id, resource, stored)
	}
	return v.pane.Layout(gtx, e, header, nil, 0, body)
}

func (v *View) missing(gtx layout.Context, e *env.Env) layout.Dimensions {
	header := func(gtx layout.Context) layout.Dimensions {
		return v.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Name:    "Passthrough machine unavailable",
			Outline: -1,
			Proof:   []common.Span{{Text: "This machine is no longer in the fleet."}},
		})
	}
	body := kit.Empty(e.Th, icons.ScreenShare, "Machine unavailable", "Choose another machine from the fleet.")
	return v.pane.Layout(gtx, e, header, nil, 0, body)
}

func (v *View) actions(gtx layout.Context, e *env.Env, resource model.Resource, machine backend.Machine) layout.Dimensions {
	running := resource.State == model.StateRunning
	if v.primary.Clicked(gtx) && e.Snap.GPU.Ready {
		e.Ops.StartPassthrough(machine.Name)
	}
	if v.stop.Clicked(gtx) {
		e.Ops.StopPassthrough(machine.Name)
	}
	if v.gpu.Clicked(gtx) {
		e.Ops.OpenHubGPU()
	}
	if v.more.Clicked(gtx) {
		v.overflow.Open = !v.overflow.Open
	}
	if choice, ok := v.menu.Clicked(gtx); ok {
		v.overflow.Open = false
		switch choice {
		case 0:
			e.Nav.Ask(env.Confirm{
				Title:  "Remove " + machine.Name + "?",
				Body:   "Removing forgets this passthrough machine but keeps its disk so it can be recovered.",
				Action: "Remove and keep the disk",
				Run:    func() { e.Ops.RemovePassthrough(machine.Name, false) },
			})
		case 1:
			e.Nav.Ask(env.Confirm{
				Title:         "Delete " + machine.Name + " and its disk?",
				Body:          fmt.Sprintf("Deleting removes the passthrough machine and its %d GB disk. It cannot be brought back.", machine.DiskGB),
				Action:        "Delete " + machine.Name + " and its disk",
				TypeToConfirm: machine.Name,
				Run:           func() { e.Ops.RemovePassthrough(machine.Name, true) },
			})
		}
	}
	primaryLabel := "Start"
	primaryIcon := icons.PlayArrow
	if running {
		primaryLabel = "Open"
		primaryIcon = icons.ScreenShare
	}
	items := []kit.MenuItem{
		{Icon: icons.Delete, Label: "Remove, keep the disk", Danger: true},
		{Icon: icons.Delete, Label: "Remove and delete the disk", Danger: true},
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			buttonGTX := gtx
			if !e.Snap.GPU.Ready {
				buttonGTX = gtx.Disabled()
			}
			return v.primary.Layout(buttonGTX, e.Th, kit.Gold, primaryIcon, primaryLabel)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !running {
				return layout.Dimensions{}
			}
			return v.stop.Layout(gtx, e.Th, kit.Line, icons.Stop, "Stop")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.gpu.Layout(gtx, e.Th, kit.Line, icons.Tune, "Open GPU settings")
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

func (v *View) blocker(gtx layout.Context, e *env.Env) layout.Dimensions {
	if v.blockerGPU.Clicked(gtx) {
		e.Ops.OpenHubGPU()
	}
	message := strings.TrimSpace(e.Snap.GPU.Blocker)
	if message == "" {
		message = "Finish GPU passthrough setup in Ryoku Hub, then refresh the fleet."
	}
	return kit.Empty(e.Th, icons.Tune, "GPU passthrough needs setup", message,
		kit.EmptyAction{Button: &v.blockerGPU, Kind: kit.Gold, Icon: icons.Tune, Label: "Open GPU settings"},
	)(gtx)
}

func (v *View) body(gtx layout.Context, e *env.Env, id model.ID, resource model.Resource, stored *store.Passthrough) layout.Dimensions {
	facts := []kit.Fact{
		{Label: "Guest OS", Value: guestOS(stored.OS)},
		{Label: "Processor", Value: fmt.Sprintf("%d cores", stored.VCPUs), Mono: true},
		{Label: "Memory", Value: format.Bytes(int64(stored.RAMMB) * 1024 * 1024), Mono: true},
		{Label: "Disk", Value: fmt.Sprintf("%d GB", stored.DiskGB), Mono: true},
		{Label: "Display", Value: "Looking Glass"},
	}
	running := resource.State == model.StateRunning && stored.Stats != nil
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !running {
				return layout.Dimensions{}
			}
			series := e.Snap.Series(id, model.MetricCPU)
			var values []float64
			if series != nil {
				values = series.Values()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(common.SectionTitle(e.Th, "Processor", nil)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.EngravedChart(gtx, e.Th, values, 0, 1, unit.Dp(120), cpuTicks)
				}),
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
				layout.Rigid(common.SectionTitle(e.Th, "Memory", nil)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					used := float64(stored.Stats.MemoryKB) * 1024
					total := float64(stored.RAMMB) * 1024 * 1024
					frac := 0.0
					if total > 0 {
						frac = math.Min(1, used/total)
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.Label{Style: e.Th.T.Mono, Color: e.Th.R.Fg}.Layout(gtx, e.Th, format.Bytes(int64(used))+" of "+format.Bytes(int64(total)))
						}),
						layout.Rigid(layout.Spacer{Height: e.Th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.BladeMeter(gtx, e.Th, frac, e.Th.R.Fg2)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
			)
		}),
		layout.Rigid(common.SectionTitle(e.Th, "Machine", nil)),
		layout.Rigid(kit.Facts(e.Th, facts)),
	)
}

func passthroughProof(machine backend.Machine) []common.Span {
	return []common.Span{
		{Text: "A passthrough machine, opens in Looking Glass. "},
		{Text: fmt.Sprintf("%d", machine.VCPUs), Mono: true},
		{Text: " cores, "},
		{Text: fmt.Sprintf("%d", machine.RAMMB/1024), Mono: true},
		{Text: " GB."},
	}
}

func guestOS(os string) string {
	switch strings.ToLower(os) {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return os
	}
}
