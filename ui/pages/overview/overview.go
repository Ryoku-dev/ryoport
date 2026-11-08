package overview

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type View struct {
	rings         anim.Tween
	ringProgress  float32
	ringActive    bool
	started       bool
	attentionList kit.List
	alertButtons  map[string]*kit.Button
	jobButtons    map[string]*kit.Button
	nodeClicks    map[model.ID]*widget.Clickable
	nodeMotion    map[model.ID]*pointMotion
}

type pointMotion struct {
	x, y          anim.Spring
	target        image.Point
	current       image.Point
	latencyBand   float64
	latencyCenter float64
	tone          model.Tone
	initialized   bool
	latencySet    bool
}

func New() *View {
	return &View{
		rings:        anim.Tween{Duration: 1200 * time.Millisecond, Ease: anim.EaseChisel},
		alertButtons: make(map[string]*kit.Button),
		jobButtons:   make(map[string]*kit.Button),
		nodeClicks:   make(map[model.ID]*widget.Clickable),
		nodeMotion:   make(map[model.ID]*pointMotion),
	}
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Th == nil || e.Snap == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	stacked := gtx.Constraints.Max.X < gtx.Dp(unit.Dp(880))
	if !v.started {
		v.rings.Set(gtx, 0)
		v.rings.Set(gtx, 1)
		v.ringActive = true
		v.started = true
	}
	progress := v.ringProgress
	if v.ringActive {
		progress = v.rings.Value(gtx)
		v.ringProgress = progress
		if progress >= 1 {
			v.ringActive = false
		}
	}
	return layout.Inset{Top: 30, Right: 36, Left: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fleetSentence(gtx, e)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if stacked {
					gap := gtx.Dp(unit.Dp(14))
					attentionMax := gtx.Constraints.Max.Y * 2 / 5
					if reserve := gtx.Constraints.Max.Y - gap - gtx.Dp(unit.Dp(360)); reserve >= 0 {
						attentionMax = min(attentionMax, reserve)
					}
					return layout.Flex{Axis: layout.Vertical, Gap: gap}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = 0
							gtx.Constraints.Max.Y = attentionMax
							return v.attentionList.Layout(gtx, e.Th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
								return v.layoutAttention(gtx, e)
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return v.layoutMap(gtx, e, progress)
						}),
					)
				}
				attentionWidth := gtx.Dp(unit.Dp(240))
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = attentionWidth
						gtx.Constraints.Max.X = attentionWidth
						return layout.Inset{Top: 46}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return v.layoutAttention(gtx, e)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return v.layoutMap(gtx, e, progress)
					}),
				)
			}),
		)
	})
}

func fleetSentence(gtx layout.Context, e *env.Env) layout.Dimensions {
	spans, attention := common.FleetSentence(e.Snap)
	if attention >= 0 && attention < len(spans) {
		spans[attention].Color = e.Th.R.Amber
	}
	return common.Sentence(gtx, e.Th, e.Th.T.Sentence, e.Th.R.Fg, spans)
}

func (v *View) layoutAttention(gtx layout.Context, e *env.Env) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(e.Snap.Alerts)+len(e.Snap.Jobs)+4)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return attentionTitle(gtx, e, "Needs you", len(e.Snap.Alerts))
	}))
	for i := range e.Snap.Alerts {
		alert := e.Snap.Alerts[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutAlert(gtx, e, alert)
		}))
	}
	jobs := runningJobs(e.Snap.Jobs)
	if len(jobs) > 0 {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: 18}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return attentionTitle(gtx, e, "Becoming", len(jobs))
			}),
		)
		for i := range jobs {
			job := jobs[i]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutJob(gtx, e, job)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func attentionTitle(gtx layout.Context, e *env.Env, title string, count int) layout.Dimensions {
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(kit.Text(e.Th, e.Th.T.Label, e.Th.R.Fg3, title)),
			layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, formatCount(count))),
		)
	})
}

func (v *View) layoutAlert(gtx layout.Context, e *env.Env, alert model.Alert) layout.Dimensions {
	drawHairline(gtx, e.Th.R.Vein)
	var button *kit.Button
	if alert.Fix != model.FixNone {
		button = v.alertButtons[alert.ID]
		if button == nil {
			button = &kit.Button{Small: true}
			v.alertButtons[alert.ID] = button
		}
		if button.Clicked(gtx) {
			applyFix(e, alert)
		}
	}
	tone := model.ToneAttention
	if alert.Severity == model.SeverityBad {
		tone = model.ToneBad
	}
	return layout.Inset{Top: 13, Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return alertFlakeName(gtx, e, tone, alert.Title)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Fg2, MaxLines: 2}.Layout(gtx, e.Th, alert.Body)
			}),
		}
		if alert.Fix != model.FixNone {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label, icon := fixPresentation(alert.Fix)
				return button.Layout(gtx, e.Th, kit.Gold, icon, label)
			}))
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
	})
}

func (v *View) layoutJob(gtx layout.Context, e *env.Env, job model.Job) layout.Dimensions {
	drawHairline(gtx, e.Th.R.Vein)
	var button *kit.Button
	if job.Cancelable {
		button = v.jobButtons[job.ID]
		if button == nil {
			button = &kit.Button{Small: true}
			v.jobButtons[job.ID] = button
		}
		if button.Clicked(gtx) {
			e.Ops.Cancel(job.ID)
		}
	}
	frac := max(0, min(1, job.Progress))
	return layout.Inset{Top: 13, Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return flakeName(gtx, e, model.ToneBusy, frac, job.Title)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, jobPhase(job))),
					layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, jobAmount(job))),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.BladeProgress(gtx, e.Th, job.Progress, job.State == model.JobRunning)
			}),
		}
		if left := jobLeft(job); left != "" {
			children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, left)))
		}
		if job.Cancelable {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return button.Layout(gtx, e.Th, kit.Line, icons.Cancel, "Cancel")
			}))
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
	})
}

func alertFlakeName(gtx layout.Context, e *env.Env, tone model.Tone, name string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(9))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Flake(gtx, e.Th, tone, -1, unit.Dp(9))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: e.Th.T.Title, Color: e.Th.R.Fg, MaxLines: 2}.Layout(gtx, e.Th, name)
		}),
	)
}

func flakeName(gtx layout.Context, e *env.Env, tone model.Tone, progress float64, name string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(9))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Flake(gtx, e.Th, tone, progress, unit.Dp(9))
		}),
		layout.Rigid(kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, name)),
	)
}

func runningJobs(jobs []model.Job) []model.Job {
	out := make([]model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.State == model.JobRunning {
			out = append(out, job)
		}
	}
	return out
}

func jobPhase(job model.Job) string {
	if job.Progress >= 0 {
		return job.Phase + " " + format.Percent(job.Progress)
	}
	return job.Phase
}

func jobAmount(job model.Job) string {
	if job.Total > 0 {
		return format.Progress(job.Done, job.Total)
	}
	return ""
}

func jobLeft(job model.Job) string {
	if job.Total <= 0 || job.Rate <= 0 || job.Done >= job.Total {
		return ""
	}
	seconds := float64(job.Total-job.Done) / job.Rate
	return format.Left(time.Duration(seconds * float64(time.Second)))
}

func applyFix(e *env.Env, alert model.Alert) {
	switch alert.Fix {
	case model.FixSavePassword:
		e.Nav.Select(alert.Target)
	case model.FixInstallEngine:
		e.Ops.InstallEngine()
	case model.FixOpenHubGPU:
		e.Ops.OpenHubGPU()
	case model.FixDeployKey:
		e.Nav.Select(alert.Target)
		e.Nav.SetTab(model.KindServer, "Access")
	case model.FixOpenSettings, model.FixEnableKVM:
		e.Nav.Go(store.PageSettings)
	}
}

func fixPresentation(fix model.Fix) (string, *widget.Icon) {
	switch fix {
	case model.FixSavePassword:
		return "Enter password", icons.Key
	case model.FixInstallEngine:
		return "Install the engine", icons.Download
	case model.FixOpenHubGPU:
		return "Open GPU settings", icons.Tune
	case model.FixDeployKey:
		return "Deploy a key", icons.VpnKey
	case model.FixOpenSettings, model.FixEnableKVM:
		return "Open settings", icons.Settings
	default:
		return "Fix it", icons.Settings
	}
}

func formatCount(count int) string {
	if count < 0 {
		count = 0
	}
	return strconv.Itoa(count)
}

func drawHairline(gtx layout.Context, c color.NRGBA) {
	height := max(1, gtx.Dp(unit.Dp(1)))
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, height)).Op())
}
