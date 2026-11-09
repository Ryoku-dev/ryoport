package app

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/icons"
	"github.com/ryoku-dev/ryoport/ui/kit"
)

type workView uint8

const (
	workOverview workView = iota
	workServer
	workMachine
	workConsole
	workPassthrough
	workServerDraft
	workConsoleDraft
	workPassthroughDraft
	workInstantDraft
	workImportDraft
	workSpawnDraft
	workCatalog
	workActivity
	workSettings
)

type workNavStamp struct {
	page     store.Page
	selected model.ID
	draft    env.Draft
}

func (s *shell) prepareWorkTransition(gtx layout.Context, stamp navStamp) {
	work := workNavStamp{page: stamp.page, selected: stamp.selected, draft: stamp.draft}
	if !s.workStampReady {
		s.workStampReady = true
		s.workStamp = work
		s.workEnter.Set(gtx, 1)
		return
	}
	if s.workStamp == work {
		return
	}
	s.workStamp = work
	s.workEnter = anim.Tween{Duration: s.theme.Motion.Notch, Ease: anim.EaseChisel}
	s.workEnter.Set(gtx, 0)
	s.workEnter.Set(gtx, 1)
}

func windowClass(width unit.Dp) env.Class {
	return env.ClassFor(width)
}

func chooseWork(nav env.Nav, snapshot *store.Snapshot) workView {
	switch nav.Page {
	case store.PageActivity:
		return workActivity
	case store.PageSettings:
		return workSettings
	}
	switch nav.Draft.Kind {
	case env.DraftServer:
		return workServerDraft
	case env.DraftConsole:
		return workConsoleDraft
	case env.DraftPassthrough:
		return workPassthroughDraft
	case env.DraftInstant:
		return workInstantDraft
	case env.DraftImport:
		return workImportDraft
	case env.DraftSpawn:
		return workSpawnDraft
	}
	if nav.Page == store.PageCreate {
		return workCatalog
	}
	if snapshot == nil || nav.Selected == "" {
		return workOverview
	}
	resource, ok := snapshot.Resource(nav.Selected)
	if !ok {
		return workOverview
	}
	switch resource.Kind {
	case model.KindServer:
		return workServer
	case model.KindMachine:
		return workMachine
	case model.KindConsole:
		return workConsole
	case model.KindPassthrough:
		return workPassthrough
	default:
		return workOverview
	}
}

func (s *shell) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	if e.Class == env.Compact {
		return s.layoutCompact(gtx, e)
	}
	return s.layoutWide(gtx, e)
}

func (s *shell) layoutWide(gtx layout.Context, e *env.Env) layout.Dimensions {
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	railWidth := gtx.Dp(unit.Dp(72))
	listWidth := gtx.Dp(unit.Dp(320))
	if e.Class == env.Expanded {
		railWidth = gtx.Dp(unit.Dp(88))
		listWidth = gtx.Dp(unit.Dp(384))
	}
	gap := gtx.Dp(unit.Dp(8))
	top := gtx.Dp(unit.Dp(10))
	paneHeight := height - top

	layoutAt(gtx, image.Point{}, image.Pt(railWidth, height), func(gtx layout.Context) layout.Dimensions {
		return s.rail.Layout(gtx, e)
	})

	contentX := railWidth + gap
	if e.Nav.Page == store.PageActivity || e.Nav.Page == store.PageSettings {
		layoutAt(gtx, image.Pt(contentX, top), image.Pt(width-contentX, paneHeight), func(gtx layout.Context) layout.Dimensions {
			return s.layoutPane(gtx, e, true, s.fullPage(e))
		})
		return layout.Dimensions{Size: image.Pt(width, height)}
	}

	layoutAt(gtx, image.Pt(contentX, top), image.Pt(listWidth, paneHeight), func(gtx layout.Context) layout.Dimensions {
		return s.layoutPane(gtx, e, false, func(gtx layout.Context) layout.Dimensions {
			return s.fleet.Layout(gtx, e)
		})
	})
	workX := contentX + listWidth + gap
	layoutAt(gtx, image.Pt(workX, top), image.Pt(width-workX, paneHeight), func(gtx layout.Context) layout.Dimensions {
		return s.layoutPane(gtx, e, true, s.workPage(e))
	})
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (s *shell) layoutCompact(gtx layout.Context, e *env.Env) layout.Dimensions {
	width := gtx.Constraints.Max.X
	height := gtx.Constraints.Max.Y
	bottomHeight := gtx.Dp(unit.Dp(64))
	gap := gtx.Dp(unit.Dp(8))
	top := gtx.Dp(unit.Dp(10))
	paneHeight := height - bottomHeight - gap - top
	if paneHeight < 0 {
		paneHeight = 0
	}

	work := e.Nav.Page == store.PageActivity || e.Nav.Page == store.PageSettings || compactShowsWork(*e.Nav, e.Snap)
	body := func(gtx layout.Context) layout.Dimensions { return s.fleet.Layout(gtx, e) }
	if e.Nav.Page == store.PageActivity || e.Nav.Page == store.PageSettings {
		body = s.fullPage(e)
	} else if work {
		body = s.compactWorkPage(e)
	}
	layoutAt(gtx, image.Pt(0, top), image.Pt(width, paneHeight), func(gtx layout.Context) layout.Dimensions {
		return s.layoutPane(gtx, e, work, body)
	})
	layoutAt(gtx, image.Pt(0, height-bottomHeight), image.Pt(width, bottomHeight), func(gtx layout.Context) layout.Dimensions {
		return s.rail.LayoutBottom(gtx, e)
	})
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func compactShowsWork(nav env.Nav, snapshot *store.Snapshot) bool {
	if nav.Draft.Kind != env.DraftNone || nav.Page == store.PageCreate {
		return true
	}
	if snapshot == nil || nav.Selected == "" {
		return false
	}
	_, ok := snapshot.Resource(nav.Selected)
	return ok
}

func (s *shell) fullPage(e *env.Env) layout.Widget {
	return s.workPage(e)
}

func (s *shell) workPage(e *env.Env) layout.Widget {
	choice := chooseWork(*e.Nav, e.Snap)
	return func(gtx layout.Context) layout.Dimensions {
		switch choice {
		case workActivity:
			return s.activity.Layout(gtx, e)
		case workSettings:
			return s.settings.Layout(gtx, e)
		case workServer:
			return s.server.Layout(gtx, e)
		case workMachine:
			return s.machine.Layout(gtx, e)
		case workConsole:
			return s.console.Layout(gtx, e)
		case workPassthrough:
			return s.passthrough.Layout(gtx, e)
		case workServerDraft:
			return s.serverDraft.Layout(gtx, e)
		case workConsoleDraft:
			return s.consoleDraft.Layout(gtx, e)
		case workPassthroughDraft:
			return s.passthroughDraft.Layout(gtx, e)
		case workInstantDraft:
			return s.instant.Layout(gtx, e)
		case workImportDraft:
			return s.importISO.Layout(gtx, e)
		case workSpawnDraft:
			return s.spawn.Layout(gtx, e)
		case workCatalog:
			return s.catalog.Layout(gtx, e)
		default:
			return s.overview.Layout(gtx, e)
		}
	}
}

func (s *shell) compactWorkPage(e *env.Env) layout.Widget {
	page := s.workPage(e)
	return func(gtx layout.Context) layout.Dimensions {
		if s.compactBack.Clicked(gtx) {
			e.Nav.Back()
			gtx.Execute(op.InvalidateCmd{})
		}
		return (layout.Flex{Axis: layout.Vertical}).Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					s.compactBack.Small = true
					dims := s.compactBack.Layout(gtx, e.Th, kit.Quiet, icons.ArrowBack, "Back")
					return dims
				})
			}),
			layout.Flexed(1, page),
		)
	}
}

func (s *shell) layoutPane(gtx layout.Context, e *env.Env, work bool, body layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	path := kit.CutPath(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(e.Th.Cut.Large), gtx.Dp(e.Th.Cut.Large), 0, 0)
	stack := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	plane := e.Th.R.Stone1
	if work {
		plane = e.Th.R.Stone2
	}
	paint.Fill(gtx.Ops, plane)

	strip := 0
	if s.customDecorations() {
		strip = gtx.Dp(unit.Dp(32))
		move := clip.Rect{Max: image.Pt(size.X, strip)}.Push(gtx.Ops)
		s.decorations.MoveArea(gtx.Ops)
		move.Pop()
		if work || e.Class == env.Compact {
			s.decorations.Layout(gtx, e.Th, s.config.Mode)
		}
	}
	bodySize := image.Pt(size.X, size.Y-strip)
	if bodySize.Y < 0 {
		bodySize.Y = 0
	}
	body = s.transitionWorkPane(gtx, work, body)
	layoutAt(gtx, image.Pt(0, strip), bodySize, body)
	stack.Pop()
	return layout.Dimensions{Size: size}
}

func (s *shell) transitionWorkPane(gtx layout.Context, work bool, body layout.Widget) layout.Widget {
	if !work {
		return body
	}
	progress := s.workEnter.Value(gtx)
	if progress >= 1 {
		return body
	}
	return func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dims := body(gtx)
		call := record.Stop()
		opacity := paint.PushOpacity(gtx.Ops, progress)
		rise := int(float32(gtx.Dp(8)) * (1 - progress))
		offset := op.Offset(image.Pt(0, rise)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		opacity.Pop()
		return dims
	}
}

func layoutAt(gtx layout.Context, at, size image.Point, widget layout.Widget) layout.Dimensions {
	if size.X < 0 {
		size.X = 0
	}
	if size.Y < 0 {
		size.Y = 0
	}
	stack := op.Offset(at).Push(gtx.Ops)
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	child := gtx
	child.Constraints = layout.Exact(size)
	widget(child)
	area.Pop()
	stack.Pop()
	return layout.Dimensions{Size: size}
}
