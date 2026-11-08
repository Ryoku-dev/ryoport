package settings

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/version"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type updateState struct {
	Title  string
	Detail string
}

func updateStateWords(info update.Info) updateState {
	current := strings.TrimPrefix(info.Current, "v")
	if current == "" {
		current = version.Version
	}
	latest := strings.TrimPrefix(info.Latest, "v")
	if info.RestartPending {
		if latest == "" {
			latest = current
		}
		return updateState{Title: "Restart to finish", Detail: "Ryoport " + latest + " is installed and will be used after the restart."}
	}
	if info.Available {
		title := "Ryoport " + latest + " is available"
		if latest == "" {
			title = "An update is available"
		}
		if info.CanInstall {
			return updateState{Title: title, Detail: "You have Ryoport " + current + ". The update can be installed here."}
		}
		reason := strings.TrimSpace(info.Reason)
		switch info.Install {
		case update.InstallDistro:
			if reason == "" {
				reason = "Use your distribution's package manager to update Ryoport."
			}
			return updateState{Title: title, Detail: "Installed by your distribution. " + reason}
		case update.InstallSource:
			if reason == "" {
				reason = "Pull the latest source and rebuild Ryoport."
			}
			return updateState{Title: title, Detail: "Installed from source. " + reason}
		default:
			if reason == "" {
				reason = "This installation cannot update itself."
			}
			return updateState{Title: title, Detail: reason}
		}
	}
	if latest != "" {
		return updateState{Title: "Ryoport is up to date", Detail: "You have Ryoport " + current + "."}
	}
	return updateState{Title: "Ryoport " + current, Detail: "Check for a newer release when you are ready."}
}

func (v *View) layoutUpdates(gtx layout.Context, e *env.Env) layout.Dimensions {
	state := updateStateWords(e.Snap.Update)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, e.Th, state.Title, state.Detail, nil)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						v.checkUpdate.Small = true
						return v.checkUpdate.Layout(gtx, e.Th, kit.Gold, icons.Refresh, "Check now")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !e.Snap.Update.Available || !e.Snap.Update.CanInstall || e.Snap.Update.RestartPending {
							return layout.Dimensions{}
						}
						v.applyUpdate.Small = true
						return v.applyUpdate.Layout(gtx, e.Th, kit.Gold, icons.Download, "Install the update")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !e.Snap.Update.RestartPending {
							return layout.Dimensions{}
						}
						v.relaunch.Small = true
						return v.relaunch.Layout(gtx, e.Th, kit.Line, icons.RestartAlt, "Restart Ryoport")
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return settingRow(gtx, e.Th, "Check daily", "Checks release metadata in the background while Ryoport is open.", func(gtx layout.Context) layout.Dimensions {
					before := v.daily.Value
					dimensions := v.daily.Layout(gtx, e.Th, kit.ToggleBone, "Check daily")
					if v.daily.Value != before {
						value := v.daily.Value
						e.Ops.UpdateSettings(func(settings *config.Settings) { settings.Updates.Check = value })
					}
					return dimensions
				})
			})
		}),
	)
}
