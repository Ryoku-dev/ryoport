package app

import (
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type overlays struct {
	confirm       kit.Confirm
	activeConfirm *env.Confirm
	placards      kit.Placards
	undo          map[int]string
	lastNotice    uint64
	paletteOpen   bool
}

func (o *overlays) Layout(gtx layout.Context, e *env.Env, shell *shell) {
	shell.rail.LayoutNewOverlay(gtx, e)
	if e.Nav.Palette && !o.paletteOpen {
		shell.palette.Open(gtx)
	}
	o.paletteOpen = e.Nav.Palette
	shell.palette.Layout(gtx, e)

	o.layoutConfirm(gtx, e)
	o.pushNotices(e)
	o.placards.Layout(gtx, e.Th)
	if id, ok := o.placards.Action(gtx); ok {
		if token := o.undo[id]; token != "" {
			e.Ops.Undo(token)
			delete(o.undo, id)
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (o *overlays) layoutConfirm(gtx layout.Context, e *env.Env) {
	requested := e.Nav.Confirm
	if requested != nil && o.activeConfirm != requested {
		o.activeConfirm = requested
		o.confirm.Dialog.Open = true
	}
	pending := o.activeConfirm
	if pending == nil {
		return
	}
	if requested == nil {
		o.confirm.Dialog.Open = false
	}
	if o.confirm.Layout(gtx, e.Th, pending.Title, pending.Body, pending.Action, pending.TypeToConfirm) {
		run := pending.Run
		e.Nav.Confirm = nil
		if run != nil {
			run()
		}
	}
	if !o.confirm.Dialog.Open && e.Nav.Confirm == pending {
		e.Nav.Confirm = nil
	}
	if o.confirm.Closed() {
		o.activeConfirm = nil
	}
}

func (o *overlays) pushNotices(e *env.Env) {
	if o.undo == nil {
		o.undo = make(map[int]string)
	}
	previous := o.lastNotice
	latest := previous
	for _, notice := range e.Snap.Notices {
		if notice.ID <= previous {
			continue
		}
		action := ""
		if notice.Undo != "" {
			action = "Undo"
		}
		id := o.placards.Push(e.Now, notice.Text, action)
		if notice.Undo != "" {
			o.undo[id] = notice.Undo
		}
		for old := range o.undo {
			if old < id-3 {
				delete(o.undo, old)
			}
		}
		if notice.ID > latest {
			latest = notice.ID
		}
	}
	o.lastNotice = latest
}
