package machine

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

type snapshotState struct {
	name            kit.TextField
	take            kit.Button
	restore, remove []kit.Button
	err             string
}

func (s *snapshotState) reset() {
	s.err = ""
	s.name.SetText("")
}

func (s *snapshotState) ensure(n int) {
	if len(s.restore) != n {
		s.restore = make([]kit.Button, n)
		s.remove = make([]kit.Button, n)
		for i := range n {
			s.restore[i].Small = true
			s.remove[i].Small = true
		}
	}
}

func (v *View) layoutSnapshots(gtx layout.Context, e *env.Env, resource model.Resource, machine *store.Machine) layout.Dimensions {
	if machine == nil {
		return kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg2, "Snapshot details are not available.")(gtx)
	}
	snapshots := userSnapshots(machine.Snapshots)
	editable := resource.State == model.StateStopped
	v.snapshots.ensure(len(snapshots))
	if editable && v.snapshots.take.Clicked(gtx) {
		tag := strings.TrimSpace(v.snapshots.name.Text())
		if tag == "" {
			v.snapshots.err = "Name the snapshot before taking it."
		} else {
			e.Ops.CreateSnapshot(resource.Name, tag)
			v.snapshots.name.SetText("")
			v.snapshots.err = ""
		}
	}
	if editable {
		for i, snapshot := range snapshots {
			tag := snapshot.Name
			if v.snapshots.restore[i].Clicked(gtx) {
				e.Nav.Ask(env.Confirm{
					Title:  "Restore " + tag + "?",
					Body:   "The machine disk will return to this snapshot. Changes made after it will be lost.",
					Action: "Restore " + tag,
					Run:    func() { e.Ops.RestoreSnapshot(resource.Name, tag) },
				})
			}
			if v.snapshots.remove[i].Clicked(gtx) {
				e.Nav.Ask(env.Confirm{
					Title:  "Delete " + tag + "?",
					Body:   "Deleting this snapshot cannot be undone. The machine itself will remain.",
					Action: "Delete " + tag,
					Run:    func() { e.Ops.DeleteSnapshot(resource.Name, tag) },
				})
			}
		}
	}

	take := func(gtx layout.Context) layout.Dimensions {
		if !editable {
			gtx = gtx.Disabled()
		}
		field := func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
			return v.snapshots.name.Layout(gtx, e.Th, "Snapshot name", "before-upgrade", "Use a name that says what you can return to.", v.snapshots.err)
		}
		if gtx.Constraints.Max.X < gtx.Dp(520) {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(field),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.snapshots.take.Layout(gtx, e.Th, kit.Gold, icons.Backup, "Take a snapshot")
				}),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(13)}.Layout(gtx,
			layout.Flexed(1, field),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.snapshots.take.Layout(gtx, e.Th, kit.Gold, icons.Backup, "Take a snapshot")
			}),
		)
	}
	children := []layout.FlexChild{
		layout.Rigid(common.SectionTitle(e.Th, "Snapshots", nil)),
	}
	if !editable {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Caption, e.Th.R.Fg3, "Stop the machine to manage its snapshots.")))
	}
	children = append(children,
		layout.Rigid(take),
		layout.Rigid(layout.Spacer{Height: 21}.Layout),
	)
	if len(snapshots) == 0 {
		children = append(children, layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg3, "No snapshots yet. Take one before a risky change.")))
	} else {
		for i, snapshot := range snapshots {
			i, snapshot := i, snapshot
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return snapshotRow(gtx, e, snapshot.Name, snapshot.Date, &v.snapshots.restore[i], &v.snapshots.remove[i], editable)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func snapshotRow(gtx layout.Context, e *env.Env, name, date string, restore, remove *kit.Button, editable bool) layout.Dimensions {
	dims := layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		copy := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Th, e.Th.T.Body, e.Th.R.Fg, name)),
				layout.Rigid(kit.Text(e.Th, e.Th.T.MonoSm, e.Th.R.Fg3, date)),
			)
		}
		actions := func(gtx layout.Context) layout.Dimensions {
			if !editable {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return restore.Layout(gtx, e.Th, kit.Line, icons.Restore, "Restore")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return remove.Layout(gtx, e.Th, kit.DangerLine, icons.Delete, "Delete")
				}),
			)
		}
		if gtx.Constraints.Max.X < gtx.Dp(460) {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(copy),
				layout.Rigid(actions),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, copy),
			layout.Rigid(actions),
		)
	})
	paint.FillShape(gtx.Ops, e.Th.R.Vein, clip.Rect{Min: image.Pt(0, dims.Size.Y-1), Max: image.Pt(dims.Size.X, dims.Size.Y)}.Op())
	return dims
}

func userSnapshots(snapshots []vm.Snapshot) []vm.Snapshot {
	for i, snapshot := range snapshots {
		if snapshot.Name != "sealed" {
			continue
		}
		filtered := make([]vm.Snapshot, 0, len(snapshots)-1)
		filtered = append(filtered, snapshots[:i]...)
		for _, remaining := range snapshots[i+1:] {
			if remaining.Name != "sealed" {
				filtered = append(filtered, remaining)
			}
		}
		return filtered
	}
	return snapshots
}
