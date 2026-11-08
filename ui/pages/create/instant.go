package create

import (
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

// Instant is the draft for a provisioned cloud-image machine.
type Instant struct {
	list        kit.List
	image       kit.Picker
	name        kit.TextField
	packages    kit.TextField
	tools       map[string]*kit.Toggle
	disposable  kit.Toggle
	sealed      kit.Toggle
	make        kit.Button
	cancel      kit.Button
	imageOS     string
	initialized bool
	submitted   bool
}

func NewInstant() *Instant {
	return &Instant{tools: make(map[string]*kit.Toggle)}
}

func nameError(name string) string {
	if name == "" {
		return "Enter a machine name."
	}
	if !vm.ValidName(name) {
		return "Use letters, numbers, dots, underscores or hyphens."
	}
	return ""
}

func (p *Instant) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.initialize(e)
	if p.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
	}
	return pageLayout(gtx, e, &p.list, "Instant cloud machine", p.body(e))
}

func (p *Instant) initialize(e *env.Env) {
	if p.initialized || len(e.Snap.Catalog.Cloud) == 0 {
		return
	}
	p.initialized = true
	p.imageOS = e.Snap.Catalog.Cloud[0].OS
	p.name.SetText(p.imageOS + "-instant")
	p.packages.SetText(e.Snap.Settings.Defaults.ExtraPkgs)
	for _, tool := range e.Snap.Catalog.ToolSets {
		toggle := new(kit.Toggle)
		toggle.Value = slices.Contains(e.Snap.Settings.Defaults.Tools, tool.ID)
		p.tools[tool.ID] = toggle
	}
}

func (p *Instant) body(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(e.Snap.Catalog.Cloud) == 0 {
			return kit.Empty(e.Th, icons.Cloud, "No cloud images are available", "Refresh the catalogue, then try again.")(gtx)
		}
		images := make([]string, len(e.Snap.Catalog.Cloud))
		selected := 0
		for i, image := range e.Snap.Catalog.Cloud {
			images[i] = image.Name
			if image.OS == p.imageOS {
				selected = i
			}
		}
		imagePicker := func(gtx layout.Context) layout.Dimensions {
			dims := p.image.Layout(gtx, e.Th, "Cloud image", images, selected)
			if index, changed := p.image.Changed(gtx); changed && index >= 0 && index < len(e.Snap.Catalog.Cloud) {
				oldDefault := p.imageOS + "-instant"
				p.imageOS = e.Snap.Catalog.Cloud[index].OS
				if p.name.Text() == "" || p.name.Text() == oldDefault {
					p.name.SetText(p.imageOS + "-instant")
				}
			}
			return dims
		}
		nameErr := ""
		if p.submitted {
			nameErr = nameError(p.name.Text())
		}
		nameField := func(gtx layout.Context) layout.Dimensions {
			return p.name.Layout(gtx, e.Th, "Name", "ubuntu-instant", "Used for the machine and its files.", nameErr)
		}
		toolGrid := func(gtx layout.Context) layout.Dimensions {
			return kit.Columns(gtx, 180, e.Th.Sp.S2, len(e.Snap.Catalog.ToolSets), func(gtx layout.Context, index int) layout.Dimensions {
				tool := e.Snap.Catalog.ToolSets[index]
				toggle := p.tools[tool.ID]
				if toggle == nil {
					toggle = new(kit.Toggle)
					p.tools[tool.ID] = toggle
				}
				label := tool.Label
				if tool.Heavy {
					label += " (large)"
				}
				return toggle.Layout(gtx, e.Th, label)
			})
		}
		packageField := func(gtx layout.Context) layout.Dimensions {
			return p.packages.Layout(gtx, e.Th, "Extra packages", "ripgrep fd", "Separate package names with spaces.", "")
		}
		choices := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.disposable.Layout(gtx, e.Th, "Disposable")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.sealed.Layout(gtx, e.Th, "Sealed")
				}),
			)
		}
		if p.make.Clicked(gtx) {
			p.submitted = true
			if nameError(p.name.Text()) == "" && p.imageOS != "" {
				name := p.name.Text()
				e.Ops.Instant(build.InstantOptions{
					OS: p.imageOS, Name: name, Disposable: p.disposable.Value, Sealed: p.sealed.Value,
					Tools: p.selectedTools(e.Snap.Catalog.ToolSets), Pkgs: strings.Fields(p.packages.Text()),
				})
				e.Nav.Select(model.NewID(model.KindMachine, name))
			}
		}
		actions := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.make.Layout(gtx, e.Th, kit.Gold, icons.PlayArrow, "Make it")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.cancel.Layout(gtx, e.Th, kit.Line, nil, "Cancel")
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(fullWidth(imagePicker)),
			layout.Rigid(nameField),
			space(e.Th.Sp.S2),
			layout.Rigid(kit.Text(e.Th, e.Th.T.Title, e.Th.R.Fg, "Tool sets")),
			layout.Rigid(toolGrid),
			layout.Rigid(packageField),
			layout.Rigid(choices),
			space(e.Th.Sp.S2),
			layout.Rigid(actions),
		)
	}
}

func (p *Instant) selectedTools(catalog []build.ToolSet) []string {
	selected := make([]string, 0, len(catalog))
	for _, tool := range catalog {
		if toggle := p.tools[tool.ID]; toggle != nil && toggle.Value {
			selected = append(selected, tool.ID)
		}
	}
	return selected
}
