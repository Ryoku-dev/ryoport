package overview

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/ui/anim"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/kit"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

func (v *View) layoutMap(gtx layout.Context, e *env.Env, reveal float32) layout.Dimensions {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	bounds := image.Rectangle{Max: size}
	short := float64(min(size.X, size.Y))
	outer := max(48, short*0.40)
	inner := max(26, outer*0.25)
	oneMS := max(inner+12, outer*0.43)
	tenMS := (oneMS + max(oneMS, outer-12)) / 2
	center := image.Pt(size.X/2, size.Y/2)

	compact := outer < float64(gtx.Dp(unit.Dp(260)))
	nodes := mapNodes(e)
	for i := range nodes {
		nodes[i].NameOnly = compact
	}
	v.stabilizeLatencyBands(nodes)
	placement := placeMap(nodes, bounds, inner, oneMS, outer)
	drawRings(gtx, e.Th, center, []float64{inner, oneMS, tenMS, outer}, reveal)
	drawRingLabels(gtx, e.Th, placement.Rings, compact)
	drawCompass(gtx, e, center)

	alpha := max(float32(0), min(float32(1), (reveal-0.70)/0.20))
	settle := int(math.Round(float64((1 - alpha) * 6)))
	layer := paint.PushOpacity(gtx.Ops, alpha)
	shift := op.Offset(image.Pt(0, settle)).Push(gtx.Ops)
	drawSameBox(gtx, e, placement.Nodes, placement.Groups)
	seen := make(map[model.ID]bool, len(nodes))
	for _, nodePlacement := range placement.Nodes {
		node := nodes[nodePlacement.Index]
		seen[node.ID] = true
		motion := v.nodeMotionFor(node.ID)
		point := resolveNodePoint(gtx, motion, nodePlacement.Point, node.Tone)
		click := v.nodeClicks[node.ID]
		if click == nil {
			click = new(widget.Clickable)
			v.nodeClicks[node.ID] = click
		}
		if click.Clicked(gtx) {
			e.Nav.Select(node.ID)
		}
		drawMapNode(gtx, e, node, nodePlacement, point, click)
	}
	shift.Pop()
	layer.Pop()
	for id := range v.nodeClicks {
		if !seen[id] {
			delete(v.nodeClicks, id)
			delete(v.nodeMotion, id)
		}
	}
	return layout.Dimensions{Size: size}
}

func animSpring() anim.Spring {
	return anim.Spring{Omega: 18, Rest: 0.5}
}

func (v *View) nodeMotionFor(id model.ID) *pointMotion {
	motion := v.nodeMotion[id]
	if motion == nil {
		motion = &pointMotion{x: animSpring(), y: animSpring()}
		v.nodeMotion[id] = motion
	}
	return motion
}

func (v *View) stabilizeLatencyBands(nodes []mapNode) {
	for i := range nodes {
		if nodes[i].Local {
			continue
		}
		motion := v.nodeMotionFor(nodes[i].ID)
		motion.latencyBand, motion.latencyCenter = hystereticLatencyBand(
			float64(nodes[i].RTTMs),
			motion.latencyBand,
			motion.latencyCenter,
			motion.latencySet,
		)
		motion.latencySet = true
		nodes[i].RadiusBand = motion.latencyBand
		nodes[i].RadiusBandSet = true
	}
}

func resolveNodePoint(gtx layout.Context, motion *pointMotion, target image.Point, tone model.Tone) image.Point {
	if !motion.initialized {
		settle := gtx.Dp(unit.Dp(6))
		motion.x.Snap(float32(target.X))
		motion.y.Snap(float32(target.Y + settle))
		motion.y.Set(float32(target.Y))
		motion.target = target
		motion.current = image.Pt(target.X, target.Y+settle)
		motion.tone = tone
		motion.initialized = true
	} else {
		stateChanged := tone != motion.tone
		if target != motion.target {
			motion.x.Snap(float32(target.X))
			motion.y.Snap(float32(target.Y))
			motion.target = target
			motion.current = target
		}
		if stateChanged {
			settle := gtx.Dp(unit.Dp(3))
			motion.y.Snap(float32(target.Y + settle))
			motion.y.Set(float32(target.Y))
			motion.current.Y = target.Y + settle
		}
		motion.tone = tone
	}
	if motion.x.Moving() {
		motion.current.X = int(math.Round(float64(motion.x.Value(gtx))))
	} else {
		motion.current.X = motion.target.X
	}
	if motion.y.Moving() {
		motion.current.Y = int(math.Round(float64(motion.y.Value(gtx))))
	} else {
		motion.current.Y = motion.target.Y
	}
	return motion.current
}

func mapNodes(e *env.Env) []mapNode {
	nodes := make([]mapNode, 0, len(e.Snap.Resources))
	for _, resource := range e.Snap.Resources {
		claim, proof := common.StateWords(e, resource)
		unknownStopped := resource.Kind == model.KindMachine && resource.State == model.StateStopped && resource.Updated.IsZero()
		if unknownStopped {
			proof = ""
		} else if proof == "" {
			proof = claim
		}
		node := mapNode{
			ID:       resource.ID,
			Name:     resource.Name,
			Proof:    proof,
			Local:    resource.Kind != model.KindServer,
			Tone:     resource.State.Tone(),
			Progress: -1,
		}
		if job, ok := e.Snap.JobFor(resource.ID); ok {
			node.Tone = model.ToneBusy
			node.Progress = job.Progress
			if job.Progress >= 0 {
				node.Proof = format.Percent(job.Progress)
			}
		}
		if resource.Kind == model.KindServer {
			node.RTTMs = 100
			if server := e.Snap.Servers[resource.ID.Key()]; server != nil {
				node.Host = server.HostName
				if server.Reach != nil {
					node.RTTMs = server.Reach.RTTMs
				}
			}
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func drawRings(gtx layout.Context, th *theme.Theme, center image.Point, radii []float64, reveal float32) {
	reveal = max(float32(0), min(float32(1), reveal))
	limit := int(float32(gtx.Constraints.Max.X) * reveal)
	mask := clip.Rect(image.Rect(0, 0, limit, gtx.Constraints.Max.Y)).Push(gtx.Ops)
	width := float32(max(1, gtx.Dp(unit.Dp(1))))
	dash, gap := float32(gtx.Dp(unit.Dp(5))), float32(gtx.Dp(unit.Dp(4)))
	for _, radius := range radii {
		points := kit.OctagonPoints(layout.FPt(center), float32(radius))
		kit.Polyline(gtx.Ops, points[:], true, width, th.R.Vein)
		kit.Dashes(gtx.Ops, points[:], true, width, dash, gap, th.R.Fg3)
	}
	mask.Pop()
}

func drawRingLabels(gtx layout.Context, th *theme.Theme, labels []mapLabelPlacement, compact bool) {
	if compact && len(labels) > 0 {
		labels = labels[len(labels)-1:]
	}
	for _, label := range labels {
		drawMapLabel(gtx, th, label.Box.Min, label.Text, text.Start, th.T.MonoSm, th.R.Fg2, label.Box.Dx())
	}
}

func drawCompass(gtx layout.Context, e *env.Env, center image.Point) {
	size := gtx.Dp(unit.Dp(30))
	stack := op.Offset(image.Pt(center.X-size/2, center.Y-size/2)).Push(gtx.Ops)
	compassGTX := gtx
	compassGTX.Constraints = layout.Exact(image.Pt(size, size))
	kit.BrandMark(e.Th, unit.Dp(30))(compassGTX)
	stack.Pop()
}

func drawSameBox(gtx layout.Context, e *env.Env, placed []nodePlacement, groups []sameBoxPlacement) {
	width := float32(max(1, gtx.Dp(unit.Dp(1.5))))
	for _, group := range groups {
		points := make([]f32.Point, 0, len(group.Group.Indices))
		for _, index := range group.Group.Indices {
			points = append(points, layout.FPt(placed[index].Point))
		}
		kit.Polyline(gtx.Ops, points, false, width, e.Th.R.Fg3)
		drawMapLabel(gtx, e.Th, group.Box.Min, "same box", text.Middle, e.Th.T.MonoSm, e.Th.R.Fg3, group.Box.Dx())
	}
}

func drawMapNode(gtx layout.Context, e *env.Env, node mapNode, placement nodePlacement, point image.Point, click *widget.Clickable) {
	halo := e.Th.R.Vein
	if click.Hovered() {
		halo = e.Th.R.Fg2
	}
	haloPoints := kit.OctagonPoints(layout.FPt(point), float32(gtx.Dp(unit.Dp(10))))
	kit.Polyline(gtx.Ops, haloPoints[:], true, float32(max(1, gtx.Dp(unit.Dp(1)))), halo)

	flakeWidth, flakeHeight := gtx.Dp(unit.Dp(14)), gtx.Dp(unit.Dp(10))
	flakeStack := op.Offset(image.Pt(point.X-flakeWidth/2, point.Y-flakeHeight/2)).Push(gtx.Ops)
	flakeGTX := gtx
	flakeGTX.Constraints = layout.Exact(image.Pt(flakeWidth, flakeHeight))
	kit.Flake(flakeGTX, e.Th, node.Tone, node.Progress, unit.Dp(9))
	if node.Tone == model.ToneIdle {
		cut := min(flakeHeight, flakeWidth/2)
		outline := [4]f32.Point{
			f32.Pt(float32(cut), 0),
			f32.Pt(float32(flakeWidth), 0),
			f32.Pt(float32(flakeWidth-cut), float32(flakeHeight)),
			f32.Pt(0, float32(flakeHeight)),
		}
		kit.Polyline(gtx.Ops, outline[:], true, float32(max(1, gtx.Dp(unit.Dp(1.5)))), e.Th.R.Fg3)
	}
	flakeStack.Pop()

	align := text.Start
	if !placement.LabelRight {
		align = text.End
	}
	drawMapLabel(gtx, e.Th, placement.Label.Min, node.Name, align, e.Th.T.Label, e.Th.R.Fg, placement.Label.Dx())
	if !node.NameOnly {
		proofAt := image.Pt(placement.Label.Min.X, placement.Label.Min.Y+gtx.Dp(unit.Dp(16)))
		drawMapLabel(gtx, e.Th, proofAt, node.Proof, align, e.Th.T.MonoSm, e.Th.R.Fg2, placement.Label.Dx())
	}

	hit := placement.Label.Union(image.Rect(point.X-12, point.Y-12, point.X+12, point.Y+12))
	hitStack := op.Offset(hit.Min).Push(gtx.Ops)
	hitGTX := gtx
	hitGTX.Constraints = layout.Exact(hit.Size())
	click.Layout(hitGTX, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	hitStack.Pop()
	if (placement.Label.Empty() || node.NameOnly) && click.Hovered() {
		drawNodeTooltip(gtx, e.Th, node, point)
	}
}

func drawNodeTooltip(gtx layout.Context, th *theme.Theme, node mapNode, point image.Point) {
	full := node
	full.NameOnly = false
	width := min(gtx.Dp(unit.Dp(180)), max(gtx.Dp(unit.Dp(92)), labelWidth(full, gtx.Dp(unit.Dp(180)))))
	height, gap := gtx.Dp(unit.Dp(40)), gtx.Dp(unit.Dp(14))
	x := point.X + gap
	if x+width > gtx.Constraints.Max.X-4 {
		x = point.X - gap - width
	}
	x = max(4, min(x, gtx.Constraints.Max.X-width-4))
	y := max(4, min(point.Y-height/2, gtx.Constraints.Max.Y-height-4))
	box := image.Rect(x, y, x+width, y+height)
	kit.FillCut(gtx.Ops, box, gtx.Dp(unit.Dp(4)), th.R.Stone4)
	kit.StrokeCut(gtx.Ops, box, gtx.Dp(unit.Dp(4)), max(1, gtx.Dp(unit.Dp(1))), th.R.Vein)
	drawMapLabel(gtx, th, image.Pt(x+6, y+4), node.Name, text.Start, th.T.Label, th.R.Fg, width-12)
	drawMapLabel(gtx, th, image.Pt(x+6, y+20), node.Proof, text.Start, th.T.MonoSm, th.R.Fg2, width-12)
}

func drawMapLabel(gtx layout.Context, th *theme.Theme, at image.Point, value string, align text.Alignment, style theme.TextStyle, ink color.NRGBA, width int) {
	if width <= 0 {
		return
	}
	stack := op.Offset(at).Push(gtx.Ops)
	labelGTX := gtx
	labelGTX.Constraints = layout.Exact(image.Pt(width, gtx.Dp(unit.Dp(16))))
	kit.Label{Style: style, Color: ink, MaxLines: 1, Align: align}.Layout(labelGTX, th, value)
	stack.Pop()
}
