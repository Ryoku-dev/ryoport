package machine

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type stepStatus uint8

const (
	stepFuture stepStatus = iota
	stepCurrent
	stepDone
)

type creationStep struct {
	title  string
	detail string
	status stepStatus
}

func stepStates(job model.Job) [4]stepStatus {
	if job.State == model.JobDone {
		return [4]stepStatus{stepDone, stepDone, stepDone, stepDone}
	}
	phase := strings.ToLower(strings.TrimSpace(job.Phase))
	current := 0
	switch {
	case strings.Contains(phase, "download"), strings.Contains(phase, "receiv"):
		current = 1
	case strings.Contains(phase, "config"), strings.Contains(phase, "disk"), strings.Contains(phase, "writ"):
		current = 2
	case strings.Contains(phase, "finish"), strings.Contains(phase, "boot"), strings.Contains(phase, "ssh"):
		current = 3
	}
	var states [4]stepStatus
	for i := range states {
		switch {
		case i < current:
			states[i] = stepDone
		case i == current:
			states[i] = stepCurrent
		default:
			states[i] = stepFuture
		}
	}
	return states
}

func creationSteps(job model.Job, machine *store.Machine, e *env.Env) [4]creationStep {
	states := stepStates(job)
	cores := e.Snap.Settings.Defaults.Cores
	ram := e.Snap.Settings.Defaults.RAMGB
	disk := e.Snap.Settings.Defaults.DiskGB
	if machine != nil {
		if parsed := leadingInt(machine.Cores); parsed > 0 {
			cores = parsed
		}
		if parsed := sizeGB(machine.RAM); parsed > 0 {
			ram = parsed
		}
		if parsed := sizeGB(machine.Disk); parsed > 0 {
			disk = parsed
		}
	}
	imageName := strings.TrimSpace(strings.TrimPrefix(job.Title, "Create "))
	if imageName == "" {
		imageName = "Machine image"
	}
	steps := [4]creationStep{
		{title: "Find the image", detail: imageName, status: states[0]},
		{title: "Download", detail: "Receiving the machine image", status: states[1]},
		{title: "Write the config", detail: fmt.Sprintf("%d cores, %d GB, %d GB disk", cores, ram, disk), status: states[2]},
		{title: "First boot", detail: "Prepare the window and SSH key", status: states[3]},
	}
	if detail := strings.TrimSpace(job.Detail); detail != "" {
		for i := range steps {
			if steps[i].status == stepCurrent {
				steps[i].detail = detail
			}
		}
	}
	return steps
}

func (v *View) layoutBecoming(gtx layout.Context, e *env.Env, machine *store.Machine, job model.Job) layout.Dimensions {
	steps := creationSteps(job, machine, e)
	readout := jobProgressSummary(job)
	if readout == "" {
		readout = strings.TrimSpace(job.Phase)
	}
	if readout == "" {
		readout = "Preparing"
	}
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(e.Th, e.Th.T.Sentence, e.Th.R.Fg, readout)),
		layout.Rigid(layout.Spacer{Height: 13}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.BladeProgress(gtx, e.Th, job.Progress, job.State == model.JobRunning)
		}),
		layout.Rigid(layout.Spacer{Height: 34}.Layout),
		layout.Rigid(common.SectionTitle(e.Th, "Steps", nil)),
	}
	for i := range steps {
		step := steps[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutCreationStep(gtx, e, step)
			})
		}))
	}
	if len(job.Log) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 21}.Layout),
			layout.Rigid(common.SectionTitle(e.Th, "Tool output", nil)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutJobLog(gtx, e, job.Log)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (v *View) layoutCreationStep(gtx layout.Context, e *env.Env, step creationStep) layout.Dimensions {
	ink := e.Th.R.Fg3
	if step.status == stepCurrent || step.status == stepDone {
		ink = e.Th.R.Fg
	}
	icon := func(gtx layout.Context) layout.Dimensions {
		switch step.status {
		case stepDone:
			size := gtx.Dp(18)
			gtx.Constraints.Min = image.Pt(size, size)
			gtx.Constraints.Max = image.Pt(size, size)
			return icons.Check.Layout(gtx, e.Th.R.Lapis)
		case stepCurrent:
			return kit.Flake(gtx, e.Th, model.ToneBusy, 1, 13)
		default:
			return kit.Flake(gtx, e.Th, model.ToneIdle, -1, 13)
		}
	}
	content := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(e.Th, e.Th.T.Title, ink, step.title)),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, step.detail)),
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(13)}.Layout(gtx,
		layout.Rigid(icon),
		layout.Flexed(1, content),
	)
}

func layoutJobLog(gtx layout.Context, e *env.Env, lines []string) layout.Dimensions {
	return kit.Slab(gtx, e.Th.R.Stone3, e.Th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 13, Right: 16, Bottom: 13, Left: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(lines))
			for _, line := range lines {
				line := line
				children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg2, line)))
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(4)}.Layout(gtx, children...)
		})
	})
}

func leadingInt(value string) int {
	value = strings.TrimSpace(value)
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(value[:end])
	return n
}

func sizeGB(value string) int {
	bytes := sizeBytes(value)
	if bytes <= 0 {
		return 0
	}
	return int((bytes + (1 << 30) - 1) / (1 << 30))
}
