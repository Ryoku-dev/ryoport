package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/theme"
)

type Confirm struct {
	Sheet   Sheet
	cancel  Button
	confirm Button
	field   TextField
}

func (c *Confirm) Layout(gtx layout.Context, th *theme.Theme, title, body, action, typeToConfirm string) (confirmed bool) {
	enabled := typeToConfirm == "" || c.field.Text() == typeToConfirm
	if c.cancel.Clicked(gtx) {
		c.Sheet.dismiss()
	}
	if enabled && c.confirm.Clicked(gtx) {
		confirmed = true
		c.Sheet.dismiss()
		c.field.SetText("")
	}
	bodyWidget := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(Text(th, th.T.Body, th.R.InkDim, body))}
		if typeToConfirm != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(Text(th, th.T.Caption, th.R.InkDim, "Type "+typeToConfirm+" to confirm")),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.field.Layout(gtx, th, "Confirmation", typeToConfirm, "", "")
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return c.cancel.Layout(gtx, th, TextButton, nil, "Cancel")
	}
	danger := func(gtx layout.Context) layout.Dimensions {
		if !enabled {
			gtx = gtx.Disabled()
		}
		return c.confirm.Layout(gtx, th, Danger, nil, action)
	}
	c.Sheet.Layout(gtx, th, Dialog, title, bodyWidget, cancel, danger)
	if c.Sheet.Closed(gtx) {
		c.field.SetText("")
	}
	return confirmed
}
