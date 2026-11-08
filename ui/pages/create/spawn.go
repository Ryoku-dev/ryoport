package create

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

// Spawn is the draft for copying a prepared template.
type Spawn struct {
	list         kit.List
	template     kit.Picker
	name         kit.TextField
	disposable   kit.Toggle
	make         kit.Button
	cancel       kit.Button
	templateName string
	initialized  bool
	submitted    bool
}

func NewSpawn() *Spawn { return &Spawn{} }

func (p *Spawn) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.initialize(e)
	if p.cancel.Clicked(gtx) {
		e.Nav.CloseDraft()
	}
	return pageLayout(gtx, e, &p.list, "Make from a template", p.body(e))
}

func (p *Spawn) initialize(e *env.Env) {
	if p.initialized || len(e.Snap.Catalog.Templates) == 0 {
		return
	}
	p.initialized = true
	p.templateName = e.Nav.Draft.Seed
	if _, ok := findTemplate(e.Snap.Catalog.Templates, p.templateName); !ok {
		p.templateName = e.Snap.Catalog.Templates[0].Name
	}
	p.name.SetText(p.templateName + "-copy")
}

func (p *Spawn) body(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		templates := e.Snap.Catalog.Templates
		if len(templates) == 0 {
			return kit.Empty(e.Th, icons.ContentCopy, "No templates yet", "Make a template from a stopped machine first.")(gtx)
		}
		options := make([]string, len(templates))
		selected := 0
		for i, template := range templates {
			options[i] = template.Name
			if template.Name == p.templateName {
				selected = i
			}
		}
		templatePicker := func(gtx layout.Context) layout.Dimensions {
			dims := p.template.Layout(gtx, e.Th, "Template", options, selected)
			if index, changed := p.template.Changed(gtx); changed && index >= 0 && index < len(templates) {
				oldDefault := p.templateName + "-copy"
				p.templateName = templates[index].Name
				if p.name.Text() == "" || p.name.Text() == oldDefault {
					p.name.SetText(p.templateName + "-copy")
				}
			}
			return dims
		}
		nameErr := ""
		if p.submitted {
			nameErr = nameError(p.name.Text())
		}
		nameField := func(gtx layout.Context) layout.Dimensions {
			return p.name.Layout(gtx, e.Th, "Name", "template-copy", "The template stays unchanged.", nameErr)
		}
		if p.make.Clicked(gtx) {
			p.submitted = true
			if nameError(p.name.Text()) == "" && p.templateName != "" {
				name := p.name.Text()
				e.Ops.Spawn(p.templateName, name, p.disposable.Value)
				e.Nav.Select(model.NewID(model.KindMachine, name))
			}
		}
		actions := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.make.Layout(gtx, e.Th, kit.Gold, icons.ContentCopy, "Make it")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.cancel.Layout(gtx, e.Th, kit.Line, nil, "Cancel")
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(fullWidth(templatePicker)),
			layout.Rigid(nameField),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.disposable.Layout(gtx, e.Th, kit.ToggleBone, "Disposable")
			}),
			space(e.Th.Sp.S2),
			layout.Rigid(actions),
		)
	}
}

func findTemplate(templates []build.Template, name string) (build.Template, bool) {
	for _, template := range templates {
		if template.Name == name {
			return template, true
		}
	}
	return build.Template{}, false
}
