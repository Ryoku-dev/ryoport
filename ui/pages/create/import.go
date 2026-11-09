package create

import (
	"fmt"
	"path/filepath"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

var guestLabels = []string{"Linux", "Windows", "Other"}
var guestValues = []string{"linux", "windows", "other"}

// Import is the draft for a machine installed from local ISO media.
type Import struct {
	pane           common.Pane
	header         common.Header
	iso            kit.TextField
	name           kit.TextField
	osSlug         kit.TextField
	guest          kit.Picker
	browse         kit.Button
	importISO      kit.Button
	cancel         kit.Button
	guestOS        string
	fileErr        string
	file           filePreview
	focusFirst     bool
	submitted      bool
	isoValidation  fieldValidation
	nameValidation fieldValidation
}

func NewImport() *Import {
	p := &Import{guestOS: "linux", focusFirst: true}
	p.iso.Mono = true
	p.osSlug.Mono = true
	p.osSlug.SetText("linux")
	return p
}

func isoError(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "Choose an ISO file."
	}
	if !strings.EqualFold(filepath.Ext(path), ".iso") {
		return "Choose a file ending in .iso."
	}
	return ""
}

func (p *Import) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if p.browse.Clicked(gtx) {
		if e.Files == nil {
			p.fileErr = "The file picker is not available. Enter the ISO path instead."
		} else {
			e.Files.ChooseFile([]string{".iso"}, func(path string, err error) {
				if err != nil {
					p.fileErr = fmt.Sprintf("The ISO could not be chosen: %v", err)
					return
				}
				p.fileErr = ""
				p.iso.SetText(path)
				p.file.refresh(e, path)
				if p.name.Text() == "" {
					base := filepath.Base(path)
					p.name.SetText(strings.TrimSuffix(base, filepath.Ext(base)))
				}
			})
		}
	}
	submitted := consumeDraftSubmit(gtx, &p.iso, &p.name, &p.osSlug)
	p.file.refresh(e, p.iso.Text())
	header := func(gtx layout.Context) layout.Dimensions {
		return p.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Mark:    draftMark(e, strings.TrimSpace(p.osSlug.Text()), "From an ISO", icons.Album),
			Name:    "From an ISO",
			Outline: -1,
			Proof:   []common.Span{{Text: "Make a local machine from installation media you choose."}},
			Acts: func(gtx layout.Context) layout.Dimensions {
				return p.actions(gtx, e, submitted)
			},
		})
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return common.DraftColumns(gtx, e.Th, p.body(e), p.aside(e))
	}
	return p.pane.Layout(gtx, e, header, nil, 0, body)
}

func (p *Import) body(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		p.isoValidation.update(gtx, &p.iso)
		p.nameValidation.update(gtx, &p.name)
		isoErr, machineErr := p.fileErr, ""
		if p.isoValidation.show(p.submitted) && isoErr == "" {
			isoErr = isoError(p.iso.Text())
		}
		if p.nameValidation.show(p.submitted) {
			machineErr = nameError(p.name.Text())
		}
		isoRow := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dims := p.iso.Layout(gtx, e.Th, "ISO", "/path/to/system.iso", "Choose local installation media.", isoErr)
					if p.focusFirst {
						gtx.Execute(key.FocusCmd{Tag: &p.iso.Editor})
						p.focusFirst = false
					}
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Level with the 44 dp box under the field's caption,
					// not with the help line below it.
					top := gtx.Sp(e.Th.T.Caption.Line) + gtx.Dp(6)
					return layout.Inset{Top: unit.Dp(gtx.Metric.PxToDp(top))}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return p.browse.Layout(gtx, e.Th, kit.Line, icons.Album, "Choose")
					})
				}),
			)
		}
		nameField := func(gtx layout.Context) layout.Dimensions {
			return p.name.Layout(gtx, e.Th, "Name", "my-machine", "Used for the machine and its files.", machineErr)
		}
		guestIndex := stringIndex(guestValues, p.guestOS)
		guestPicker := func(gtx layout.Context) layout.Dimensions {
			dims := p.guest.Layout(gtx, e.Th, "Guest OS", guestLabels, guestIndex)
			if index, changed := p.guest.Changed(gtx); changed && index >= 0 && index < len(guestValues) {
				old := p.guestOS
				p.guestOS = guestValues[index]
				if p.osSlug.Text() == "" || p.osSlug.Text() == old {
					p.osSlug.SetText(p.guestOS)
				}
			}
			return dims
		}
		slugField := func(gtx layout.Context) layout.Dimensions {
			return p.osSlug.Layout(gtx, e.Th, "OS slug", "linux", "Used to find the engraved logo.", "")
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(isoRow),
			layout.Rigid(nameField),
			layout.Rigid(fullWidth(guestPicker)),
			layout.Rigid(slugField),
		)
	}
}

func (p *Import) actions(gtx layout.Context, e *env.Env, submitted bool) layout.Dimensions {
	if p.importISO.Clicked(gtx) || submitted {
		p.submitted = true
		if nameError(p.name.Text()) == "" && isoError(p.iso.Text()) == "" && p.fileErr == "" {
			name := p.name.Text()
			e.Ops.Import(name, strings.TrimSpace(p.iso.Text()), p.guestOS, strings.TrimSpace(p.osSlug.Text()))
			p.submitted = false
			p.isoValidation.reset()
			p.nameValidation.reset()
			p.file = filePreview{}
			p.focusFirst = true
			e.Nav.Select(model.NewID(model.KindMachine, name))
		}
	}
	if p.cancel.Clicked(gtx) {
		p.submitted = false
		p.fileErr = ""
		p.isoValidation.reset()
		p.nameValidation.reset()
		p.file = filePreview{}
		p.focusFirst = true
		e.Nav.CloseDraft()
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.importISO.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Make it")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.cancel.Layout(gtx, e.Th, kit.Quiet, icons.Close, "Cancel")
		}),
	)
}

func (p *Import) aside(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		isoName := p.file.name
		isoSize := p.file.size
		if isoName == "" {
			isoName = "Not chosen"
			isoSize = "Not available"
		}
		if isoSize == "" {
			isoSize = "Not available"
		}
		guest := p.guestOS
		if index := stringIndex(guestValues, p.guestOS); index >= 0 && index < len(guestLabels) {
			guest = guestLabels[index]
		}
		defaults := e.Snap.Settings.Defaults
		return common.Aside(gtx, e.Th, "What happens",
			common.AsideFact(e.Th, "ISO", isoName),
			common.AsideFact(e.Th, "File size", isoSize),
			common.AsideFact(e.Th, "Guest OS", guest),
			common.AsideFact(e.Th, "Size", machineSize(defaults.Cores, defaults.RAMGB, defaults.DiskGB)),
			common.AsideFact(e.Th, "Folder", machineFolder(p.name.Text())),
			common.AsideNote(e.Th, "After creation, Start boots the machine from this ISO."),
		)
	}
}
