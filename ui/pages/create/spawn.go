package create

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

// Spawn is the draft for copying a prepared template.
type Spawn struct {
	pane           common.Pane
	header         common.Header
	template       kit.Picker
	name           kit.TextField
	disposable     kit.Toggle
	make           kit.Button
	cancel         kit.Button
	templateName   string
	templateFile   filePreview
	initialized    bool
	focusFirst     bool
	submitted      bool
	nameValidation fieldValidation
}

func NewSpawn() *Spawn { return &Spawn{} }

func (p *Spawn) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.initialize(e)
	submitted := consumeDraftSubmit(gtx, &p.name)
	template, _ := findTemplate(e.Snap.Catalog.Templates, p.templateName)
	p.templateFile.refresh(e, template.Base)
	header := func(gtx layout.Context) layout.Dimensions {
		return p.header.Layout(gtx, e, common.HeaderContent{
			Tone:    model.ToneIdle,
			Mark:    draftMark(e, template.OS, template.Name, icons.ContentCopy),
			Name:    "From a template",
			Outline: -1,
			Proof:   []common.Span{{Text: "Make a thin copy of a prepared machine."}},
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
	p.focusFirst = true
}

func (p *Spawn) body(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		p.nameValidation.update(gtx, &p.name)
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
			if p.focusFirst {
				p.template.Focus(gtx)
				p.focusFirst = false
			}
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
		if p.nameValidation.show(p.submitted) {
			nameErr = nameError(p.name.Text())
		}
		nameField := func(gtx layout.Context) layout.Dimensions {
			return p.name.Layout(gtx, e.Th, "Name", "template-copy", "The template stays unchanged.", nameErr)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(e.Th.Sp.S3)}.Layout(gtx,
			layout.Rigid(fullWidth(templatePicker)),
			layout.Rigid(nameField),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.disposable.Layout(gtx, e.Th, "Disposable")
			}),
		)
	}
}

func (p *Spawn) actions(gtx layout.Context, e *env.Env, submitted bool) layout.Dimensions {
	if p.make.Clicked(gtx) || submitted {
		p.submitted = true
		if nameError(p.name.Text()) == "" && p.templateName != "" {
			name := p.name.Text()
			e.Ops.Spawn(p.templateName, name, p.disposable.Value)
			p.submitted = false
			p.nameValidation.reset()
			p.initialized = false
			p.templateFile = filePreview{}
			e.Nav.Select(model.NewID(model.KindMachine, name))
		}
	}
	if p.cancel.Clicked(gtx) {
		p.submitted = false
		p.nameValidation.reset()
		p.initialized = false
		p.templateFile = filePreview{}
		e.Nav.CloseDraft()
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(e.Th.Sp.S2)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.make.Layout(gtx, e.Th, kit.Gold, icons.ContentCopy, "Make it")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.cancel.Layout(gtx, e.Th, kit.Quiet, icons.Close, "Cancel")
		}),
	)
}

func (p *Spawn) aside(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		template, _ := findTemplate(e.Snap.Catalog.Templates, p.templateName)
		p.templateFile.refresh(e, template.Base)
		templateName := template.Name
		if templateName == "" {
			templateName = "None"
		}
		templateOS := template.OS
		for _, image := range e.Snap.Catalog.Cloud {
			if image.OS == template.OS {
				templateOS = image.Name
				break
			}
		}
		if templateOS == template.OS {
			for _, system := range e.Snap.Catalog.OS {
				if system.OS == template.OS {
					templateOS = system.Name
					break
				}
			}
		}
		if templateOS == "" {
			templateOS = "Unknown"
		}
		templateSize := p.templateFile.size
		if templateSize == "" {
			templateSize = "Unavailable"
		}
		disposable := "No"
		if p.disposable.Value {
			disposable = "Yes"
		}
		return common.Aside(gtx, e.Th, "What you will get",
			common.AsideFact(e.Th, "Template", templateName),
			common.AsideFact(e.Th, "OS", templateOS),
			common.AsideFact(e.Th, "Size", templateSize),
			common.AsideFact(e.Th, "Disposable", disposable),
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
