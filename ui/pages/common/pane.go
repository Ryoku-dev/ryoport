package common

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type Pane struct {
	Tabs kit.Tabs
	List kit.List
}

func (p *Pane) Layout(gtx layout.Context, e *env.Env, header layout.Widget, tabs []string, selected int, body layout.Widget) layout.Dimensions {
	th := e.Th
	children := make([]layout.FlexChild, 0, 3)
	if header != nil {
		children = append(children, layout.Rigid(header))
	}
	if len(tabs) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S2, Right: th.Sp.S5, Left: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.Tabs.Layout(gtx, th, tabs, selected)
			})
		}))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		if body == nil {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		return p.List.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S4, Right: th.Sp.S5, Bottom: th.Sp.S4, Left: th.Sp.S5}.Layout(gtx, body)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *Pane) Tab(gtx layout.Context) (int, bool) {
	return p.Tabs.Changed(gtx)
}
