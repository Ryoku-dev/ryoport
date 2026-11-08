package machine

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
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

func (v *View) layoutBecoming(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine, job model.Job) layout.Dimensions {
	steps := creationSteps(job, machine, e)
	sequence := func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(steps))
		for i := range steps {
			step := steps[i]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return v.layoutCreationStep(gtx, e, step, job)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	plan := func(gtx layout.Context) layout.Dimensions {
		return v.layoutConstruction(gtx, e, resource.Name, machine, job)
	}
	if gtx.Constraints.Max.X < gtx.Dp(660) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sequence),
			layout.Rigid(layout.Spacer{Height: 21}.Layout),
			layout.Rigid(plan),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(34)}.Layout(gtx,
		layout.Flexed(.85, sequence),
		layout.Flexed(1.15, plan),
	)
}

func (v *View) layoutCreationStep(gtx layout.Context, e *env.Env, step creationStep, job model.Job) layout.Dimensions {
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
			return kit.Flake(gtx, e.Th, model.ToneBusy, job.Progress, 13)
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
		if step.status == stepCurrent {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.BladeProgress(gtx, e.Th, job.Progress, job.State == model.JobRunning)
				}),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					left := jobProgressSummary(job)
					right := ""
					if job.Progress >= 0 {
						right = format.Percent(job.Progress)
					}
					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, left)),
						layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, right)),
					)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(13)}.Layout(gtx,
		layout.Rigid(icon),
		layout.Flexed(1, content),
	)
}

func (v *View) layoutConstruction(gtx layout.Context, e *env.Env, name string, machine *store.Machine, job model.Job) layout.Dimensions {
	height := gtx.Dp(340)
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	r := image.Rect(1, 1, size.X-1, size.Y-35)
	cut := float32(gtx.Dp(28))
	minX, minY := float32(r.Min.X), float32(r.Min.Y)
	maxX, maxY := float32(r.Max.X), float32(r.Max.Y)
	outline := [...]f32.Point{
		f32.Pt(minX+cut, minY),
		f32.Pt(maxX-cut, minY),
		f32.Pt(maxX, minY+cut),
		f32.Pt(maxX, maxY-cut),
		f32.Pt(maxX-cut, maxY),
		f32.Pt(minX+cut, maxY),
		f32.Pt(minX, maxY-cut),
		f32.Pt(minX, minY+cut),
	}
	kit.Polyline(gtx.Ops, outline[:], true, float32(max(1, gtx.Dp(1))), e.Th.R.Fg3)

	guide := theme.WithAlpha(e.Th.R.Fg3, .5)
	for i := 1; i <= 3; i++ {
		y := r.Min.Y + i*r.Dy()/4
		dashedLine(gtx, image.Pt(r.Min.X+20, y), image.Pt(r.Max.X-20, y), guide)
	}
	diagonal := [...]f32.Point{
		f32.Pt(float32(r.Min.X+42), float32(r.Max.Y-48)),
		f32.Pt(float32(r.Max.X-42), float32(r.Min.Y+48)),
	}
	kit.Polyline(gtx.Ops, diagonal[:], false, float32(max(1, gtx.Dp(1))), theme.WithAlpha(e.Th.R.Gold, .55))

	cores := e.Snap.Settings.Defaults.Cores
	ram := e.Snap.Settings.Defaults.RAMGB
	disk := e.Snap.Settings.Defaults.DiskGB
	display := constructionDisplay(e.Snap.Settings.Defaults.Display)
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
		display = constructionDisplay(machine.Display)
	}
	fill := float32(max(0, min(1, job.Progress)))
	facts := []string{
		fmt.Sprintf("%d cores", cores),
		fmt.Sprintf("%d GB memory", ram),
		fmt.Sprintf("%d GB disk", disk),
		display,
		"SSH ready",
	}
	for i, fact := range facts {
		style := e.Th.T.Title
		if i < 3 {
			style = e.Th.T.Mono
		}
		y := r.Min.Y + gtx.Dp(36) + gtx.Dp(46)*i
		off := op.Offset(image.Pt(r.Min.X+gtx.Dp(34), y)).Push(gtx.Ops)
		textGTX := gtx
		textGTX.Constraints.Min = image.Point{}
		textGTX.Constraints.Max = image.Pt(max(0, r.Dx()-gtx.Dp(68)), gtx.Dp(28))
		kit.OutlineText(textGTX, e.Th, style, fact, fill, e.Th.R.Fg, e.Th.R.Fg3)
		off.Pop()
	}

	note := "The machine takes shape after the image arrives."
	if name != "" {
		note = name + " takes shape after the image arrives."
	}
	off := op.Offset(image.Pt(0, size.Y-gtx.Dp(22))).Push(gtx.Ops)
	noteGTX := gtx
	noteGTX.Constraints.Min = image.Point{}
	noteGTX.Constraints.Max = image.Pt(size.X, gtx.Dp(22))
	kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg3, MaxLines: 1}.Layout(noteGTX, e.Th, note)
	off.Pop()
	return layout.Dimensions{Size: size}
}

func constructionDisplay(value string) string {
	display := displayName(value)
	if display == "Headless" {
		return display
	}
	return display + " display"
}

func dashedLine(gtx layout.Context, from, to image.Point, color color.NRGBA) {
	if from.Y == to.Y {
		for x := from.X; x < to.X; x += gtx.Dp(8) {
			end := min(to.X, x+gtx.Dp(3))
			paint.FillShape(gtx.Ops, color, clip.Rect{Min: image.Pt(x, from.Y), Max: image.Pt(end, from.Y+max(1, gtx.Dp(1)))}.Op())
		}
	}
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
