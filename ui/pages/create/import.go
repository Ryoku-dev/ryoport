package create

import (
	"fmt"
	"path/filepath"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

var guestLabels = []string{"Linux", "Windows", "Other"}
var guestValues = []string{"linux", "windows", "other"}

// Import is the draft for a machine installed from local ISO media.
type Import struct {
	list           kit.List
	iso            kit.TextField
	name           kit.TextField
	osSlug         kit.TextField
	guest          kit.Picker
	browse         kit.Button
	importISO      kit.Button
	cancel         kit.Button
	guestOS        string
	fileErr        string
	submitted      bool
	isoValidation  fieldValidation
	nameValidation fieldValidation
}

func NewImport() *Import {
	p := &Import{guestOS: "linux"}
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
				if p.name.Text() == "" {
					base := filepath.Base(path)
					p.name.SetText(strings.TrimSuffix(base, filepath.Ext(base)))
				}
			})
		}
	}
	if p.cancel.Clicked(gtx) {
		p.submitted = false
		p.fileErr = ""
		p.isoValidation.reset()
		p.nameValidation.reset()
		e.Nav.CloseDraft()
	}
	return pageLayout(gtx, e, &p.list, "Import an ISO", draftColumn(p.body(e)))
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
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.iso.Layout(gtx, e.Th, "ISO", "/path/to/system.iso", "Choose local installation media.", isoErr)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.browse.Layout(gtx, e.Th, kit.Line, icons.Album, "Choose")
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
		if p.importISO.Clicked(gtx) {
			p.submitted = true
			if nameError(p.name.Text()) == "" && isoError(p.iso.Text()) == "" && p.fileErr == "" {
				name := p.name.Text()
				e.Ops.Import(name, strings.TrimSpace(p.iso.Text()), p.guestOS, strings.TrimSpace(p.osSlug.Text()))
				p.submitted = false
				p.isoValidation.reset()
				p.nameValidation.reset()
				e.Nav.Select(model.NewID(model.KindMachine, name))
			}
		}
		actions := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.importISO.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Import")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.cancel.Layout(gtx, e.Th, kit.Line, nil, "Cancel")
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(isoRow),
			layout.Rigid(nameField),
			layout.Rigid(fullWidth(guestPicker)),
			layout.Rigid(slugField),
			space(e.Th.Sp.S2),
			layout.Rigid(actions),
		)
	}
}
