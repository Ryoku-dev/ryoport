package overview

import (
	"image"
	"math"
	"sort"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/model"
)

type mapNode struct {
	ID            model.ID
	Name          string
	Proof         string
	Host          string
	RTTMs         int
	RadiusBand    float64
	RadiusBandSet bool
	NameOnly      bool
	Local         bool
	Tone          model.Tone
	Progress      float64
}

type nodePlacement struct {
	Index      int
	Point      image.Point
	Label      image.Rectangle
	LabelRight bool
}

type mapLabelPlacement struct {
	Text string
	Box  image.Rectangle
}

type sameBoxPlacement struct {
	Group boxGroup
	Box   image.Rectangle
}

type mapPlacement struct {
	Nodes  []nodePlacement
	Rings  []mapLabelPlacement
	Groups []sameBoxPlacement
}

type boxGroup struct {
	Host    string
	Indices []int
}

func latencyRadius(rtt int, oneMS, outer float64) float64 {
	return latencyRadiusForBand(quantizedLatencyBand(latencyLog(rtt)), oneMS, outer)
}

func latencyRadiusForBand(band, oneMS, outer float64) float64 {
	if outer <= oneMS {
		return oneMS
	}
	const edgeInset = 12.0
	limit := outer - edgeInset
	if limit < oneMS {
		limit = oneMS
	}
	decades := max(0, min(1, band/2))
	return oneMS + (limit-oneMS)*decades
}

func latencyLog(rtt int) float64 {
	ms := float64(max(1, rtt))
	if ms <= 2 {
		return 0
	}
	return math.Log10(ms)
}

func quantizedLatencyBand(raw float64) float64 {
	return max(0, min(2, math.Round(raw/0.25)*0.25))
}

func hystereticLatencyBand(rtt int, band, center float64, set bool) (float64, float64) {
	raw := latencyLog(rtt)
	if !set || math.Abs(raw-center) > 0.25 {
		return quantizedLatencyBand(raw), raw
	}
	return band, center
}

func placeMap(nodes []mapNode, bounds image.Rectangle, inner, oneMS, outer float64) mapPlacement {
	placed := placeNodes(nodes, bounds, inner, oneMS, outer)
	obstacles := nodeObstacles(placed)
	for _, node := range placed {
		if !node.Label.Empty() {
			obstacles = append(obstacles, node.Label)
		}
	}
	groups := placeGroupLabels(nodes, placed, bounds, obstacles)
	for _, group := range groups {
		obstacles = append(obstacles, group.Box)
	}
	tenMS := (oneMS + max(oneMS, outer-12)) / 2
	rings := placeRingLabels(bounds, []float64{inner, oneMS, tenMS, outer}, obstacles)
	return mapPlacement{Nodes: placed, Rings: rings, Groups: groups}
}

func placeNodes(nodes []mapNode, bounds image.Rectangle, inner, oneMS, outer float64) []nodePlacement {
	placed := make([]nodePlacement, len(nodes))
	for i := range placed {
		placed[i].Index = i
	}
	center := image.Pt((bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2)

	var local []int
	remoteUnits := make(map[string][]int)
	for i, node := range nodes {
		if node.Local {
			local = append(local, i)
			continue
		}
		key := normalizedHost(node.Host)
		if key == "" {
			key = "\x00" + node.Name
		}
		remoteUnits[key] = append(remoteUnits[key], i)
	}
	sort.Slice(local, func(i, j int) bool { return stableNodeKey(nodes[local[i]]) < stableNodeKey(nodes[local[j]]) })
	for i, index := range local {
		angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(max(1, len(local)))
		placed[index].Point = radialPoint(center, inner, angle)
	}

	keys := make([]string, 0, len(remoteUnits))
	for key := range remoteUnits {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	unitStep := 2 * math.Pi / float64(max(1, len(keys)))
	for unit, key := range keys {
		indices := remoteUnits[key]
		sort.Slice(indices, func(i, j int) bool { return stableNodeKey(nodes[indices[i]]) < stableNodeKey(nodes[indices[j]]) })
		base := -math.Pi/2 + (float64(unit)+0.5)*unitStep
		spread := min(0.14, unitStep*0.28)
		for i, index := range indices {
			offset := (float64(i) - float64(len(indices)-1)/2) * spread
			radius := latencyRadius(nodes[index].RTTMs, oneMS, outer)
			if nodes[index].RadiusBandSet {
				radius = latencyRadiusForBand(nodes[index].RadiusBand, oneMS, outer)
			}
			placed[index].Point = radialPoint(center, radius, base+offset)
		}
	}

	const (
		labelGap  = 3
		margin    = 8
		centerGap = 12
	)
	for i := range placed {
		point := placed[i].Point
		width := labelWidth(nodes[i], max(48, bounds.Dx()/2-centerGap-margin*2))
		height := 31
		if nodes[i].NameOnly {
			height = 16
		}
		right := point.X >= center.X
		var x int
		if right {
			x = max(point.X+15, center.X+centerGap)
			x = min(x, bounds.Max.X-margin-width)
		} else {
			x = min(point.X-15-width, center.X-centerGap-width)
			x = max(x, bounds.Min.X+margin)
		}
		y := point.Y - height/2
		y = max(bounds.Min.Y+margin, min(y, bounds.Max.Y-margin-height))
		placed[i].Label = image.Rect(x, y, x+width, y+height)
		placed[i].LabelRight = right
	}
	nodesOnly := nodeObstacles(placed)
	packLabels(placed, true, center.X+centerGap, bounds.Max.X-margin, bounds.Min.Y+margin, bounds.Max.Y-margin, labelGap, nodesOnly)
	packLabels(placed, false, bounds.Min.X+margin, center.X-centerGap, bounds.Min.Y+margin, bounds.Max.Y-margin, labelGap, nodesOnly)
	return placed
}

func stableNodeKey(node mapNode) string {
	if node.ID != "" {
		return string(node.ID)
	}
	return node.Name
}

func radialPoint(center image.Point, radius, angle float64) image.Point {
	return image.Pt(
		center.X+int(math.Round(math.Cos(angle)*radius)),
		center.Y+int(math.Round(math.Sin(angle)*radius)),
	)
}

func labelWidth(node mapNode, maximum int) int {
	name := 8 + len([]rune(node.Name))*7
	width := max(48, name)
	if !node.NameOnly {
		proof := 8 + len([]rune(node.Proof))*6
		width = max(width, proof)
	}
	return min(maximum, width)
}

func packLabels(placed []nodePlacement, right bool, left, rightEdge, top, bottom, gap int, fixed []image.Rectangle) {
	indices := make([]int, 0, len(placed))
	for i := range placed {
		if placed[i].LabelRight == right {
			indices = append(indices, i)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		a, b := placed[indices[i]].Label, placed[indices[j]].Label
		if a.Min.Y == b.Min.Y {
			return placed[indices[i]].Index < placed[indices[j]].Index
		}
		return a.Min.Y < b.Min.Y
	})
	obstacles := append([]image.Rectangle(nil), fixed...)
	for _, index := range indices {
		desired := placed[index].Label
		free, ok := nearestFreeBox(desired, left, rightEdge, top, bottom, gap, obstacles)
		if !ok {
			placed[index].Label = image.Rectangle{}
			continue
		}
		placed[index].Label = free
		obstacles = append(obstacles, free)
	}
}

func nearestFreeBox(desired image.Rectangle, left, right, top, bottom, gap int, obstacles []image.Rectangle) (image.Rectangle, bool) {
	xs := candidatePositions(desired.Min.X, left, right, desired.Dx(), gap, obstacles, true)
	ys := candidatePositions(desired.Min.Y, top, bottom, desired.Dy(), gap, obstacles, false)
	for _, x := range xs {
		for _, y := range ys {
			candidate := image.Rect(x, y, x+desired.Dx(), y+desired.Dy())
			if !overlapsAny(candidate, gap, obstacles) {
				return candidate, true
			}
		}
	}
	return desired, false
}

func candidatePositions(preferred, minimum, maximum, size, gap int, obstacles []image.Rectangle, horizontal bool) []int {
	limit := maximum - size
	preferred = max(minimum, min(preferred, limit))
	seen := map[int]bool{}
	positions := make([]int, 0, len(obstacles)*2+3)
	add := func(value int) {
		value = max(minimum, min(value, limit))
		if !seen[value] {
			seen[value] = true
			positions = append(positions, value)
		}
	}
	add(preferred)
	add(minimum)
	add(limit)
	for _, obstacle := range obstacles {
		if horizontal {
			add(obstacle.Min.X - gap - size)
			add(obstacle.Max.X + gap)
		} else {
			add(obstacle.Min.Y - gap - size)
			add(obstacle.Max.Y + gap)
		}
	}
	sort.SliceStable(positions, func(i, j int) bool {
		di, dj := absInt(positions[i]-preferred), absInt(positions[j]-preferred)
		if di == dj {
			return positions[i] < positions[j]
		}
		return di < dj
	})
	return positions
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func nodeObstacles(placed []nodePlacement) []image.Rectangle {
	obstacles := make([]image.Rectangle, len(placed))
	for i, node := range placed {
		obstacles[i] = image.Rect(node.Point.X-12, node.Point.Y-12, node.Point.X+12, node.Point.Y+12)
	}
	return obstacles
}

func overlapsAny(box image.Rectangle, gap int, obstacles []image.Rectangle) bool {
	for _, obstacle := range obstacles {
		if box.Overlaps(obstacle.Inset(-gap)) {
			return true
		}
	}
	return false
}

func placeGroupLabels(nodes []mapNode, placed []nodePlacement, bounds image.Rectangle, obstacles []image.Rectangle) []sameBoxPlacement {
	center := image.Pt((bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2)
	groups := sameBoxGroups(nodes)
	out := make([]sameBoxPlacement, 0, len(groups))
	for _, group := range groups {
		var x, y int
		for _, index := range group.Indices {
			x += placed[index].Point.X
			y += placed[index].Point.Y
		}
		anchor := image.Pt(x/len(group.Indices), y/len(group.Indices))
		dx, dy := float64(anchor.X-center.X), float64(anchor.Y-center.Y)
		length := math.Hypot(dx, dy)
		if length == 0 {
			dx, dy, length = 1, 0, 1
		}
		box := findOutwardBox(anchor, dx/length, dy/length, image.Pt(70, 16), bounds, obstacles)
		out = append(out, sameBoxPlacement{Group: group, Box: box})
		obstacles = append(obstacles, box)
	}
	return out
}

func findOutwardBox(anchor image.Point, ux, uy float64, size image.Point, bounds image.Rectangle, obstacles []image.Rectangle) image.Rectangle {
	for _, distance := range []float64{28, 44, 60, 76} {
		for _, tangent := range []float64{0, 20, -20, 40, -40} {
			cx := float64(anchor.X) + ux*distance - uy*tangent
			cy := float64(anchor.Y) + uy*distance + ux*tangent
			box := centeredBox(image.Pt(int(math.Round(cx)), int(math.Round(cy))), size).Intersect(bounds)
			if box.Size() == size && !overlapsAny(box, 3, obstacles) {
				return box
			}
		}
	}
	return findFreeBox(centeredBox(anchor, size), bounds, obstacles)
}

func placeRingLabels(bounds image.Rectangle, radii []float64, obstacles []image.Rectangle) []mapLabelPlacement {
	center := image.Pt((bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2)
	texts := []string{"this computer", "1 ms", "10 ms", "100 ms"}
	out := make([]mapLabelPlacement, len(texts))
	angles := make([]float64, 0, 72)
	for _, angle := range []float64{-math.Pi / 4, math.Pi / 4, 3 * math.Pi / 4, -3 * math.Pi / 4, 0, math.Pi / 2, math.Pi, -math.Pi / 2} {
		angles = append(angles, angle)
	}
	for i := range 64 {
		angles = append(angles, -math.Pi/2+2*math.Pi*float64(i)/64)
	}
	for i := len(texts) - 1; i >= 0; i-- {
		width := 42
		if i == 0 {
			width = 92
		}
		size := image.Pt(width, 16)
		var box image.Rectangle
		var fallback image.Rectangle
		found := false
		for _, offset := range []float64{14, 20, 24} {
			for _, angle := range angles {
				candidate := centeredBox(radialPoint(center, radii[i]+offset, angle), size)
				if !containsRect(bounds.Inset(4), candidate) {
					continue
				}
				if fallback.Empty() {
					fallback = candidate
				}
				if !overlapsAny(candidate, 3, obstacles) {
					box, found = candidate, true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			box = fallback
			if box.Empty() {
				box = centeredBox(radialPoint(center, radii[i]+14, 0), size).Intersect(bounds)
			}
		}
		out[i] = mapLabelPlacement{Text: texts[i], Box: box}
		obstacles = append(obstacles, box)
	}
	return out
}

func findFreeBox(desired, bounds image.Rectangle, obstacles []image.Rectangle) image.Rectangle {
	for y := bounds.Min.Y + 4; y+desired.Dy() <= bounds.Max.Y-4; y += 4 {
		for x := bounds.Min.X + 4; x+desired.Dx() <= bounds.Max.X-4; x += 4 {
			candidate := image.Rect(x, y, x+desired.Dx(), y+desired.Dy())
			if !overlapsAny(candidate, 3, obstacles) {
				return candidate
			}
		}
	}
	return desired.Intersect(bounds)
}

func centeredBox(center, size image.Point) image.Rectangle {
	minimum := image.Pt(center.X-size.X/2, center.Y-size.Y/2)
	return image.Rectangle{Min: minimum, Max: minimum.Add(size)}
}

func containsRect(outer, inner image.Rectangle) bool {
	return inner.Min.X >= outer.Min.X && inner.Min.Y >= outer.Min.Y &&
		inner.Max.X <= outer.Max.X && inner.Max.Y <= outer.Max.Y
}

func sameBoxGroups(nodes []mapNode) []boxGroup {
	byHost := make(map[string][]int)
	for i, node := range nodes {
		if host := normalizedHost(node.Host); host != "" {
			byHost[host] = append(byHost[host], i)
		}
	}
	groups := make([]boxGroup, 0, len(byHost))
	for host, indices := range byHost {
		if len(indices) < 2 {
			continue
		}
		sort.Slice(indices, func(i, j int) bool { return nodes[indices[i]].Name < nodes[indices[j]].Name })
		groups = append(groups, boxGroup{Host: host, Indices: indices})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Host < groups[j].Host })
	return groups
}

func normalizedHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}
