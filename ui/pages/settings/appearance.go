package settings

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/ui/env"
)

var (
	appearanceLabels = []string{"Follow the desktop", "Dark", "Light"}
	appearanceValues = []string{"follow", "dark", "light"}
	densityLabels    = []string{"Comfortable", "Compact"}
	densityValues    = []string{"comfortable", "compact"}
)

func (v *View) layoutAppearance(gtx layout.Context, e *env.Env) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Colour", "Follow Ryoku's palette or keep one mode.", func(gtx layout.Context) layout.Dimensions {
				return v.modeFilters.Layout(gtx, e.Th, appearanceLabels, nil, v.mode)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Density", "Compact shows more rows without changing the information.", func(gtx layout.Context) layout.Dimensions {
				return v.densityMode.Layout(gtx, e.Th, densityLabels, nil, v.density)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, "Reduce motion", "Changes place instantly instead of moving between states.", func(gtx layout.Context) layout.Dimensions {
				before := v.reduce.Value
				dimensions := v.reduce.Layout(gtx, e.Th, "")
				if v.reduce.Value != before {
					value := v.reduce.Value
					e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Appearance.ReduceMotion = value })
				}
				return dimensions
			})
		}),
	)
}
