package settings

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

const repositoryURL = "https://github.com/ryoku-dev/ryoport"

var settingsSections = [...]string{
	"Appearance",
	"New machines",
	"VM engine",
	"GPU passthrough",
	"SSH keys",
	"Updates",
	"About",
}

// View is the full Settings page.
type View struct {
	list         kit.List
	sectionStrip layout.List
	sectionLinks [7]kit.Click
	sectionChips [7]kit.Button

	initialized      bool
	catalogRequested bool
	mode             int
	density          int
	modeFilters      kit.Filters
	densityMode      kit.Filters
	reduce           kit.Toggle

	cores           kit.Stepper
	ram             kit.Stepper
	disk            kit.Stepper
	display         int
	displayPick     kit.Picker
	tools           map[string]*kit.Toggle
	extra           kit.TextField
	extraSaved      string
	extraPending    string
	extraPendingRev uint64
	extraPendingSet bool
	savePackages    kit.Button

	daily kit.Toggle

	installEngine kit.Button
	openGPU       kit.Button
	makeKey       kit.Button
	checkUpdate   kit.Button
	applyUpdate   kit.Button
	relaunch      kit.Button
	openRepo      kit.Button
}

func New() *View {
	view := &View{tools: make(map[string]*kit.Toggle)}
	view.list.Axis = layout.Vertical
	view.sectionStrip.Axis = layout.Horizontal
	return view
}

func (v *View) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if !v.initialized {
		v.initialize(e.Snap.Settings)
	}
	if !v.catalogRequested && !e.Snap.Loaded[store.SourceCatalog] {
		v.catalogRequested = true
		e.Ops.LoadCatalog(false)
	}
	v.handleSelections(gtx, e)
	if v.extraPendingSet && e.Snap.Rev > v.extraPendingRev {
		if e.Snap.Settings.Defaults.ExtraPkgs == v.extraPending {
			v.extraSaved = v.extraPending
		}
		v.extraPendingSet = false
		v.savePackages.Busy = false
	}
	v.handleButtons(gtx, e)
	current := v.handleSectionNavigation(gtx, e.Class)
	paint.FillShape(gtx.Ops, e.Th.R.Stone2, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Inset{Top: 34, Bottom: 34, Left: 36, Right: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch e.Class {
		case env.Expanded:
			return v.layoutExpanded(gtx, e, current)
		case env.Medium:
			return v.layoutMedium(gtx, e, current)
		default:
			return v.layoutCompact(gtx, e)
		}
	})
}

func (v *View) handleSectionNavigation(gtx layout.Context, class env.Class) int {
	current := min(max(v.list.Position.First, 0), len(settingsSections)-1)
	switch class {
	case env.Expanded:
		for index := range settingsSections {
			if v.sectionLinks[index].Clicked(gtx) {
				v.list.ScrollTo(index)
				current = index
			}
		}
	case env.Medium:
		for index := range settingsSections {
			if v.sectionChips[index].Clicked(gtx) {
				v.list.ScrollTo(index)
				current = index
			}
		}
	}
	return current
}

func (v *View) layoutCompact(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: 21}.Layout(gtx, kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Settings"))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutList(gtx, e)
		}),
	)
}

func (v *View) layoutMedium(gtx layout.Context, e *env.Env, current int) layout.Dimensions {
	width := min(gtx.Constraints.Max.X, gtx.Dp(760))
	return centeredPane(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 13}.Layout(gtx, kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Settings"))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutSectionStrip(gtx, e, current)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return v.layoutList(gtx, e)
			}),
		)
	})
}

func (v *View) layoutExpanded(gtx layout.Context, e *env.Env, current int) layout.Dimensions {
	width := min(gtx.Constraints.Max.X, gtx.Dp(994))
	return centeredPane(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 21}.Layout(gtx, kit.Text(e.Th, e.Th.T.Section, e.Th.R.Fg, "Settings"))
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(200))
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return v.layoutSectionIndex(gtx, e, current)
						})
					}),
					layout.Rigid(layout.Spacer{Width: 34}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return v.layoutList(gtx, e)
					}),
				)
			}),
		)
	})
}

func (v *View) layoutSectionStrip(gtx layout.Context, e *env.Env, current int) layout.Dimensions {
	return layout.Inset{Bottom: 21}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.sectionStrip.Layout(gtx, len(settingsSections), func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				button := &v.sectionChips[index]
				button.Small = true
				kind := kit.Quiet
				if index == current {
					kind = kit.On
				}
				return button.Layout(gtx, e.Th, kind, nil, settingsSections[index])
			})
		})
	})
}

func (v *View) layoutSectionIndex(gtx layout.Context, e *env.Env, current int) layout.Dimensions {
	return layout.Inset{Top: 1, Right: 1, Bottom: 1, Left: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return column(gtx, len(settingsSections), 4, func(gtx layout.Context, index int) layout.Dimensions {
			link := &v.sectionLinks[index]
			return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(48))
				ink := e.Th.R.Fg2
				if index == current {
					kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(e.Th.Cut.Mid), e.Th.R.Sel)
					ink = e.Th.R.OnSel
				} else if link.Hovered() {
					kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(e.Th.Cut.Mid), e.Th.R.Stone3)
				}
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(settingsSections[index]).Add(gtx.Ops)
				semantic.SelectedOp(index == current).Add(gtx.Ops)

				labelGTX := gtx
				labelGTX.Constraints.Min = image.Point{}
				labelGTX.Constraints.Max = image.Pt(max(0, size.X-gtx.Dp(26)), size.Y)
				macro := op.Record(gtx.Ops)
				labelDims := kit.Text(e.Th, e.Th.T.Row, ink, settingsSections[index])(labelGTX)
				labelCall := macro.Stop()
				x := gtx.Dp(13)
				y := max(0, (size.Y-labelDims.Size.Y)/2)
				offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
				labelCall.Add(gtx.Ops)
				offset.Pop()
				return layout.Dimensions{Size: size, Baseline: y + labelDims.Baseline}
			})
		})
	})
}

func centeredPane(gtx layout.Context, width int, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			return body(gtx)
		}),
		layout.Flexed(1, layout.Spacer{}.Layout),
	)
}

func (v *View) layoutList(gtx layout.Context, e *env.Env) layout.Dimensions {
	return v.list.Layout(gtx, e.Th, len(settingsSections), func(gtx layout.Context, index int) layout.Dimensions {
		switch index {
		case 0:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutAppearance(gtx, e)
			})
		case 1:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutDefaults(gtx, e)
			})
		case 2:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutEngine(gtx, e)
			})
		case 3:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutGPU(gtx, e)
			})
		case 4:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutKeys(gtx, e)
			})
		case 5:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutUpdates(gtx, e)
			})
		case 6:
			return settingsSection(gtx, e.Th, settingsSections[index], func(gtx layout.Context) layout.Dimensions {
				return v.layoutAbout(gtx, e)
			})
		default:
			return layout.Dimensions{}
		}
	})
}

func (v *View) initialize(settings config.Settings) {
	v.mode = stringIndex([]string{"follow", "dark", "light"}, settings.Appearance.Mode)
	v.density = stringIndex([]string{"comfortable", "compact"}, settings.Appearance.Density)
	v.reduce.Value = settings.Appearance.ReduceMotion
	v.cores = kit.Stepper{Value: settings.Defaults.Cores, Min: 1, Max: 32, Step: 1}
	v.ram = kit.Stepper{Value: settings.Defaults.RAMGB, Min: 1, Max: 128, Step: 1}
	v.disk = kit.Stepper{Value: settings.Defaults.DiskGB, Min: 16, Max: 2048, Step: 16}
	v.display = stringIndex([]string{"window", "spice", "headless"}, settings.Defaults.Display)
	v.extra.SetText(settings.Defaults.ExtraPkgs)
	v.extraSaved = settings.Defaults.ExtraPkgs
	v.daily.Value = settings.Updates.Check
	v.initialized = true
}

func (v *View) handleSelections(gtx layout.Context, e *env.Env) {
	if selected, changed := v.modeFilters.Changed(gtx); changed && selected >= 0 && selected < len(appearanceValues) {
		v.mode = selected
		value := appearanceValues[selected]
		e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Appearance.Mode = value })
	}
	if selected, changed := v.densityMode.Changed(gtx); changed && selected >= 0 && selected < len(densityValues) {
		v.density = selected
		value := densityValues[selected]
		e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Appearance.Density = value })
	}
	if selected, changed := v.displayPick.Changed(gtx); changed && selected >= 0 && selected < len(displayValues) {
		v.display = selected
		value := displayValues[selected]
		e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Defaults.Display = value })
	}
}

func (v *View) handleButtons(gtx layout.Context, e *env.Env) {
	if v.installEngine.Clicked(gtx) {
		e.Ops.InstallEngine()
	}
	if v.openGPU.Clicked(gtx) {
		e.Ops.OpenHubGPU()
	}
	if v.makeKey.Clicked(gtx) {
		e.Ops.Keygen("ryoport", "ed25519")
	}
	if v.savePackages.Clicked(gtx) && !v.extraPendingSet {
		value := v.extra.Text()
		v.extraPending = value
		v.extraPendingRev = e.Snap.Rev
		v.extraPendingSet = true
		v.savePackages.Busy = true
		e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Defaults.ExtraPkgs = value })
	}
	if v.checkUpdate.Clicked(gtx) {
		e.Ops.CheckUpdate()
	}
	if v.applyUpdate.Clicked(gtx) && e.Snap.Update.CanInstall {
		e.Ops.ApplyUpdate()
	}
	if v.relaunch.Clicked(gtx) && e.Snap.Update.RestartPending {
		e.Ops.Relaunch()
	}
	if v.openRepo.Clicked(gtx) {
		e.Ops.OpenURL(repositoryURL)
	}
}

func (v *View) layoutEngine(gtx layout.Context, e *env.Env) layout.Dimensions {
	engine := e.Snap.Host.Engine
	status := "Ready"
	tone := model.ToneAlive
	detail := engine.Version
	if !engine.Ready {
		status = "Not ready"
		tone = model.ToneAttention
		detail = e.Snap.Host.Setup.Hint
		if detail == "" {
			detail = "Quickemu, QEMU and KVM are required."
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Quickemu", detail, func(gtx layout.Context) layout.Dimensions {
				return kit.Claim(e.Th, tone, status)(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if engine.Ready {
				return layout.Dimensions{}
			}
			return sectionAction(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.installEngine.Layout(gtx, e.Th, kit.Gold, icons.InstallDesktop, "Install the engine")
			})
		}),
	)
}

func (v *View) layoutGPU(gtx layout.Context, e *env.Env) layout.Dimensions {
	status := "Unavailable"
	tone := model.ToneAttention
	detail := e.Snap.GPU.Blocker
	if e.Snap.GPU.Ready {
		status = "Ready"
		tone = model.ToneAlive
		detail = "GPU isolation and the passthrough guest are ready."
	} else if detail == "" {
		detail = "Set up GPU passthrough in Ryoku Hub before making a passthrough machine."
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, status, detail, func(gtx layout.Context) layout.Dimensions {
				return kit.Flake(gtx, e.Th, tone, 1, 10)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return sectionAction(gtx, func(gtx layout.Context) layout.Dimensions {
				v.openGPU.Small = true
				return v.openGPU.Layout(gtx, e.Th, kit.Line, icons.Hub, "Open GPU settings")
			})
		}),
	)
}

func (v *View) layoutKeys(gtx layout.Context, e *env.Env) layout.Dimensions {
	count := len(e.Snap.Keys.Files) + len(e.Snap.Keys.Agent)
	actionTop := unit.Dp(13)
	if count > 0 {
		actionTop = 0
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if count == 0 {
				return wrapText(gtx, e.Th, e.Th.T.Body, e.Th.R.Fg2, "No SSH keys found. Make an Ed25519 key to deploy to your servers.")
			}
			return column(gtx, count, 0, func(gtx layout.Context, index int) layout.Dimensions {
				if index < len(e.Snap.Keys.Files) {
					key := e.Snap.Keys.Files[index]
					return settingRow(gtx, e.Th, key.Comment, key.Path, kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, key.Fingerprint))
				}
				key := e.Snap.Keys.Agent[index-len(e.Snap.Keys.Files)]
				title := key.Comment
				if title == "" {
					title = "Key in the SSH agent"
				}
				return settingRow(gtx, e.Th, title, key.Type, kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, key.Fingerprint))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: actionTop}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return sectionAction(gtx, func(gtx layout.Context) layout.Dimensions {
					v.makeKey.Small = true
					return v.makeKey.Layout(gtx, e.Th, kit.Line, icons.Key, "Make a key")
				})
			})
		}),
	)
}

func settingsSection(gtx layout.Context, th *theme.Theme, title string, body layout.Widget) layout.Dimensions {
	return layout.Inset{Bottom: 21}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(13)}.Layout(gtx,
						layout.Rigid(kit.Text(th, th.T.Section, th.R.Fg, title)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(1)))
							paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect{Max: size}.Op())
							return layout.Dimensions{Size: size}
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(21).Layout(gtx, body)
				})
			}),
		)
	})
}

func sectionAction(gtx layout.Context, action layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layout.Rigid(action))
}

func settingRow(gtx layout.Context, th *theme.Theme, title, detail string, control layout.Widget) layout.Dimensions {
	return layout.Inset{Bottom: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(21)}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if title == "" {
							return layout.Dimensions{}
						}
						return wrapText(gtx, th, th.T.Row, th.R.Fg, title)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if detail == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return wrapText(gtx, th, th.T.Caption, th.R.Fg3, detail)
						})
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if control == nil {
					return layout.Dimensions{}
				}
				gtx.Constraints.Min.X = 0
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(320))
				return control(gtx)
			}),
		)
	})
}

func wrapText(gtx layout.Context, th *theme.Theme, style theme.TextStyle, color color.NRGBA, text string) layout.Dimensions {
	return kit.Label{Style: style, Color: color}.Layout(gtx, th, text)
}

func column(gtx layout.Context, count int, gap unit.Dp, cell func(layout.Context, int) layout.Dimensions) layout.Dimensions {
	y := 0
	width := gtx.Constraints.Min.X
	gapPixels := gtx.Dp(gap)
	for index := range count {
		childGTX := gtx
		childGTX.Constraints.Min.Y = 0
		macro := op.Record(gtx.Ops)
		dimensions := cell(childGTX, index)
		call := macro.Stop()
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		y += dimensions.Size.Y
		if index+1 < count {
			y += gapPixels
		}
		width = max(width, dimensions.Size.X)
	}
	return layout.Dimensions{Size: image.Pt(width, y)}
}

func stringIndex(options []string, value string) int {
	for index, option := range options {
		if option == value {
			return index
		}
	}
	return 0
}
