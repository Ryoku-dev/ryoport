package settings

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

var (
	displayLabels = []string{"Window", "SPICE", "Headless"}
	displayValues = []string{"window", "spice", "headless"}
)

func (v *View) layoutDefaults(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Processor", "The starting number of virtual CPU cores.", func(gtx layout.Context) layout.Dimensions {
				dimensions := v.cores.Layout(gtx, e.Th, "cores")
				if v.cores.Changed() {
					value := v.cores.Value
					e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Defaults.Cores = value })
				}
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Memory", "Guest memory, separate from storage.", func(gtx layout.Context) layout.Dimensions {
				dimensions := v.ram.Layout(gtx, e.Th, "GB")
				if v.ram.Changed() {
					value := v.ram.Value
					e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Defaults.RAMGB = value })
				}
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Disk", "The initial virtual disk size.", func(gtx layout.Context) layout.Dimensions {
				dimensions := v.disk.Layout(gtx, e.Th, "GB")
				if v.disk.Changed() {
					value := v.disk.Value
					e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Defaults.DiskGB = value })
				}
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Display", "How a newly made machine opens by default.", func(gtx layout.Context) layout.Dimensions {
				return v.displayPick.Layout(gtx, e.Th, "", displayLabels, v.display)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Row, e.Th.R.Fg, "Tools in cloud machines")),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if len(e.Snap.Catalog.ToolSets) == 0 {
							if !e.Snap.Loaded[store.SourceCatalog] {
								return wrapText(gtx, e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Tool choices appear after the machine catalogue loads.")
							}
							return layout.Dimensions{}
						}
						return kit.Columns(gtx, 180, 8, len(e.Snap.Catalog.ToolSets), func(gtx layout.Context, index int) layout.Dimensions {
							tool := e.Snap.Catalog.ToolSets[index]
							toggle := v.toolToggle(tool.ID, slices.Contains(e.Snap.Settings.Defaults.Tools, tool.ID))
							before := toggle.Value
							label := tool.Label
							if tool.Heavy {
								label += " (large)"
							}
							dimensions := toggle.Layout(gtx, e.Th, kit.ToggleBone, label)
							if toggle.Value != before {
								id, on := tool.ID, toggle.Value
								e.Ops.UpdateSettings(func(settings *config.Settings) {
									settings.Defaults.Tools = withTool(settings.Defaults.Tools, id, on)
								})
							}
							return dimensions
						})
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return v.extra.Layout(gtx, e.Th, "Extra packages", "package names separated by spaces", "Added to cloud machines after the selected tool sets.", "")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if v.extra.Text() == v.extraSaved {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return sectionAction(gtx, func(gtx layout.Context) layout.Dimensions {
								v.savePackages.Small = true
								return v.savePackages.Layout(gtx, e.Th, kit.Line, icons.Save, "Save packages")
							})
						})
					}),
				)
			})
		}),
	)
}

func (v *View) toolToggle(id string, initial bool) *kit.Toggle {
	toggle := v.tools[id]
	if toggle == nil {
		toggle = new(kit.Toggle)
		toggle.Value = initial
		v.tools[id] = toggle
	}
	return toggle
}

func withTool(tools []string, id string, on bool) []string {
	result := make([]string, 0, len(tools)+1)
	for _, tool := range tools {
		if tool != id {
			result = append(result, tool)
		}
	}
	if on {
		result = append(result, id)
	}
	return result
}
