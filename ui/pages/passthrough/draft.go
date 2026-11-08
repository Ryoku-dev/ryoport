package passthrough

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoport/internal/model"
	backend "github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

// Draft creates a GPU-passthrough guest from an install ISO.
type Draft struct {
	pane   common.Pane
	header common.Header
	oses   kit.Filters

	name kit.TextField
	iso  kit.TextField
	os   string
	disk kit.Stepper
	ram  kit.Stepper

	browse kit.Button
	create kit.Button
	cancel kit.Button
	gpu    kit.Button

	fileError string
}

func NewDraft() *Draft {
	d := &Draft{os: "windows"}
	d.name.SingleLine = true
	d.iso.SingleLine = true
	d.iso.Mono = true
	d.disk = kit.Stepper{Value: 64, Min: 16, Max: 1024, Step: 8}
	d.ram = kit.Stepper{Value: 16, Min: 2, Max: 128, Step: 1}
	return d
}

func (d *Draft) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e == nil || e.Snap == nil || e.Nav == nil || e.Ops == nil {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	options := backend.CreateOptions{
		Name:   strings.TrimSpace(d.name.Text()),
		ISO:    strings.TrimSpace(d.iso.Text()),
		OS:     d.os,
		DiskGB: d.disk.Value,
		RAMMB:  d.ram.Value * 1024,
	}
	errs := validatePassthroughDraft(options)
	if d.fileError != "" {
		errs.ISO = d.fileError
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return d.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Mark:    common.Mark(e, model.Resource{Kind: model.KindPassthrough, Name: "Passthrough machine"}, 44, e.Th.R.Fg),
			Name:    "New passthrough machine",
			Outline: -1,
			Proof:   []common.Span{{Text: "Install a guest that owns the GPU and opens through Looking Glass."}},
			Acts: func(gtx layout.Context) layout.Dimensions {
				return d.actions(gtx, e, options, errs)
			},
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		if !e.Snap.GPU.Ready {
			return d.blocker(gtx, e)
		}
		return d.form(gtx, e, errs)
	}
	return d.pane.Layout(gtx, e, header, nil, 0, body)
}

func (d *Draft) actions(gtx layout.Context, e *env.Env, options backend.CreateOptions, errs passthroughDraftErrors) layout.Dimensions {
	if d.create.Clicked(gtx) && e.Snap.GPU.Ready && !errs.Any() {
		e.Ops.CreatePassthrough(options)
		e.Nav.CloseDraft()
	}
	if d.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			buttonGTX := gtx
			if !e.Snap.GPU.Ready || errs.Any() {
				buttonGTX = gtx.Disabled()
			}
			return d.create.Layout(buttonGTX, e.Th, kit.Gold, icons.Add, "Create")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.cancel.Layout(gtx, e.Th, kit.Line, icons.Close, "Cancel")
		}),
	)
}

func (d *Draft) blocker(gtx layout.Context, e *env.Env) layout.Dimensions {
	if d.gpu.Clicked(gtx) {
		e.Ops.OpenHubGPU()
	}
	message := strings.TrimSpace(e.Snap.GPU.Blocker)
	if message == "" {
		message = "Finish GPU passthrough setup in Ryoku Hub before creating this machine."
	}
	return kit.Empty(e.Th, icons.Tune, "GPU passthrough needs setup", message,
		kit.EmptyAction{Button: &d.gpu, Kind: kit.Gold, Icon: icons.Tune, Label: "Open GPU settings"},
	)(gtx)
}

func (d *Draft) form(gtx layout.Context, e *env.Env, errs passthroughDraftErrors) layout.Dimensions {
	if selected, ok := d.oses.Changed(gtx); ok {
		if selected == 0 {
			d.os = "windows"
		} else {
			d.os = "linux"
		}
	}
	if d.browse.Clicked(gtx) && e.Files != nil {
		d.fileError = ""
		e.Files.ChooseFile([]string{".iso"}, func(path string, err error) {
			if err != nil {
				d.fileError = "The ISO could not be opened. Choose it again."
				return
			}
			if path != "" {
				d.iso.SetText(path)
				d.fileError = ""
			}
		})
	}
	selectedOS := 0
	if d.os == "linux" {
		selectedOS = 1
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(common.SectionTitle(e.Th, "Install media", nil)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.name.Layout(gtx, e.Th, "Name", "win11-gpu", "Letters, digits, hyphens, and underscores.", errs.Name)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.iso.Layout(gtx, e.Th, "ISO", "/home/nero/Downloads/windows.iso", "Choose an install ISO on this computer.", errs.ISO)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.browse.Layout(gtx, e.Th, kit.Line, icons.FolderOpen, "Choose ISO")
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.oses.Layout(gtx, e.Th, []string{"Windows", "Linux"}, nil, selectedOS)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if errs.OS == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: e.Th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Jasper}.Layout(gtx, e.Th, errs.OS)
			})
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S5}.Layout),
		layout.Rigid(common.SectionTitle(e.Th, "Hardware", nil)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return stepperRow(gtx, e, "Disk", &d.disk, "GB", errs.Disk)
		}),
		layout.Rigid(layout.Spacer{Height: e.Th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return stepperRow(gtx, e, "Memory", &d.ram, "GB", errs.RAM)
		}),
	)
}

func stepperRow(gtx layout.Context, e *env.Env, label string, stepper *kit.Stepper, suffix, errText string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: e.Th.T.Body, Color: e.Th.R.Fg}.Layout(gtx, e.Th, label)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return stepper.Layout(gtx, e.Th, suffix)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if errText == "" {
				return layout.Dimensions{}
			}
			return kit.Label{Style: e.Th.T.Caption, Color: e.Th.R.Jasper}.Layout(gtx, e.Th, errText)
		}),
	)
}

type passthroughDraftErrors struct {
	Name string
	ISO  string
	OS   string
	Disk string
	RAM  string
}

func (e passthroughDraftErrors) Any() bool {
	return e.Name != "" || e.ISO != "" || e.OS != "" || e.Disk != "" || e.RAM != ""
}

func validatePassthroughDraft(options backend.CreateOptions) passthroughDraftErrors {
	var errs passthroughDraftErrors
	if options.Name == "" {
		errs.Name = "Enter a machine name."
	} else {
		for _, r := range options.Name {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				continue
			}
			errs.Name = "Use only letters, digits, hyphens, or underscores."
			break
		}
	}
	if !filepath.IsAbs(options.ISO) {
		errs.ISO = "Choose an ISO using its absolute path."
	}
	if options.OS != "windows" && options.OS != "linux" {
		errs.OS = "Choose Windows or Linux."
	}
	if options.DiskGB < 16 || options.DiskGB > 1024 {
		errs.Disk = "Disk size must be from 16 to 1024 GB."
	}
	if options.RAMMB < 2*1024 || options.RAMMB > 128*1024 || options.RAMMB%1024 != 0 {
		errs.RAM = "Memory must be a whole number of GB from 2 to 128 GB."
	}
	return errs
}
