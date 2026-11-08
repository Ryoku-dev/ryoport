package overview

import (
	"fmt"
	"image"
	"math"
	"testing"
)

func TestLatencyRadiusIsMonotonicAndInsideOuterRing(t *testing.T) {
	const oneMS, outer = 120.0, 280.0
	last := 0.0
	for _, rtt := range []int{0, 1, 2, 5, 10, 25, 50, 100, 500} {
		radius := latencyRadius(rtt, oneMS, outer)
		if radius < last {
			t.Fatalf("radius for %d ms went backwards: %.2f after %.2f", rtt, radius, last)
		}
		if radius >= outer {
			t.Fatalf("radius for %d ms reached the outer ring: %.2f", rtt, radius)
		}
		last = radius
	}
}

func TestLatencyRadiusQuantizesNearbyReadings(t *testing.T) {
	const oneMS, outer = 120.0, 280.0
	for _, pair := range [][2]int{{1, 2}, {3, 4}} {
		a, b := latencyRadius(pair[0], oneMS, outer), latencyRadius(pair[1], oneMS, outer)
		if a != b {
			t.Fatalf("%d ms and %d ms should share a radius band: %.2f and %.2f", pair[0], pair[1], a, b)
		}
	}
}

func TestLatencyBandHysteresis(t *testing.T) {
	var band, center float64
	var set bool
	var initial float64
	for i, rtt := range []int{41, 59, 44, 57} {
		band, center = hystereticLatencyBand(rtt, band, center, set)
		set = true
		if i == 0 {
			initial = band
		} else if band != initial {
			t.Fatalf("%d ms moved from jitter band %.2f to %.2f", rtt, initial, band)
		}
	}

	start, startCenter := hystereticLatencyBand(41, 0, 0, false)
	moved, _ := hystereticLatencyBand(300, start, startCenter, true)
	if moved == start {
		t.Fatalf("41 ms to 300 ms stayed in band %.2f", start)
	}
}

func TestCompactPlacementUsesNameOnlyLabels(t *testing.T) {
	nodes := []mapNode{{
		ID:       "machine:vm",
		Name:     "vm",
		Proof:    "a proof that is much wider than the name",
		NameOnly: true,
		Local:    true,
	}}
	placement := placeMap(nodes, image.Rect(0, 0, 400, 360), 40, 72, 144)
	label := placement.Nodes[0].Label
	if label.Empty() {
		t.Fatal("compact node label was not placed")
	}
	if label.Dy() != 16 {
		t.Fatalf("compact label height = %d, want 16", label.Dy())
	}
	if label.Dx() != 48 {
		t.Fatalf("compact label width = %d, want name-only width 48", label.Dx())
	}
}

func TestPlacementLabelsDoNotOverlapForFleet(t *testing.T) {
	nodes := realFleetNodes()
	placement := placeMap(nodes, image.Rect(0, 0, 690, 700), 70, 120, 280)
	for _, node := range placement.Nodes {
		if node.Label.Empty() {
			t.Fatalf("%s has no label in the real fleet layout", nodes[node.Index].Name)
		}
	}
	assertPlacementClear(t, placement)
	center := image.Pt(345, 350)
	for _, p := range placement.Nodes {
		distance := math.Hypot(float64(p.Point.X-center.X), float64(p.Point.Y-center.Y))
		if distance >= 280 {
			t.Fatalf("%s is outside the map: %.2f", nodes[p.Index].Name, distance)
		}
	}
	radii := []float64{70, 120, (120 + 268) / 2, 280}
	for i, ring := range placement.Rings {
		label := image.Pt((ring.Box.Min.X+ring.Box.Max.X)/2, (ring.Box.Min.Y+ring.Box.Max.Y)/2)
		distance := math.Hypot(float64(label.X-center.X), float64(label.Y-center.Y))
		if delta := math.Abs(distance - radii[i]); delta > 25 {
			t.Fatalf("ring label %q is %.2f px from its ring", ring.Text, delta)
		}
	}
	if len(placement.Groups) != 1 {
		t.Fatalf("same-box labels = %d, want 1", len(placement.Groups))
	}
	group := placement.Groups[0]
	a, b := placement.Nodes[group.Group.Indices[0]].Point, placement.Nodes[group.Group.Indices[1]].Point
	pair := image.Pt((a.X+b.X)/2, (a.Y+b.Y)/2)
	label := image.Pt((group.Box.Min.X+group.Box.Max.X)/2, (group.Box.Min.Y+group.Box.Max.Y)/2)
	if (label.X-center.X)*(pair.X-center.X)+(label.Y-center.Y)*(pair.Y-center.Y) <= 0 {
		t.Fatalf("same-box label is not outward: pair %v, label %v", pair, label)
	}
}

func TestHostPeersKeepPointsWhenRoundTripsSwap(t *testing.T) {
	firstNodes := realFleetNodes()
	secondNodes := append([]mapNode(nil), firstNodes...)
	for i := range secondNodes {
		switch secondNodes[i].Name {
		case "ally":
			secondNodes[i].RTTMs = 2
		case "prowl":
			secondNodes[i].RTTMs = 1
		}
	}
	bounds := image.Rect(0, 0, 690, 700)
	first := placeMap(firstNodes, bounds, 70, 120, 280)
	second := placeMap(secondNodes, bounds, 70, 120, 280)
	for i := range first.Nodes {
		if first.Nodes[i].Point != second.Nodes[i].Point {
			t.Fatalf("%s moved when host-peer round trips swapped: %v to %v", firstNodes[i].Name, first.Nodes[i].Point, second.Nodes[i].Point)
		}
	}
}

func realFleetNodes() []mapNode {
	return []mapNode{
		{ID: "server:localhost", Name: "localhost", Proof: "under 1 ms", Host: "127.0.0.1", RTTMs: 0},
		{ID: "server:ally", Name: "ally", Proof: "1 ms", Host: "192.168.110.8", RTTMs: 1},
		{ID: "server:prowl", Name: "prowl", Proof: "1 ms", Host: "192.168.110.8", RTTMs: 2},
		{ID: "server:mika-server", Name: "mika-server", Proof: "41 ms", Host: "203.0.113.70", RTTMs: 41},
		{ID: "machine:ubuntu-24.04", Name: "ubuntu-24.04", Proof: "Running", Local: true},
		{ID: "machine:fedora-43", Name: "fedora-43", Local: true},
		{ID: "machine:windows-11", Name: "windows-11", Local: true},
		{ID: "passthrough:win11-gpu", Name: "win11-gpu", Proof: "Looking Glass", Local: true},
		{ID: "console:router", Name: "router", Proof: "Offline", Local: true},
	}
}

func TestPlacementLabelsDoNotOverlapForThirtyNodes(t *testing.T) {
	nodes := make([]mapNode, 30)
	for i := range nodes {
		nodes[i] = mapNode{
			Name:  fmt.Sprintf("machine-%02d-with-a-name", i),
			Proof: fmt.Sprintf("%d ms", i*4+1),
			Host:  fmt.Sprintf("host-%02d.example", i),
			RTTMs: i*4 + 1,
		}
	}
	placement := placeMap(nodes, image.Rect(0, 0, 690, 700), 70, 120, 280)
	assertPlacementClear(t, placement)
}

func TestSameBoxGroupsByHostName(t *testing.T) {
	nodes := []mapNode{
		{Name: "ally", Host: "Box.Example."},
		{Name: "prowl", Host: "box.example"},
		{Name: "mika", Host: "elsewhere.example"},
		{Name: "local", Local: true},
	}
	groups := sameBoxGroups(nodes)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].Host != "box.example" {
		t.Fatalf("host = %q, want box.example", groups[0].Host)
	}
	if len(groups[0].Indices) != 2 || nodes[groups[0].Indices[0]].Name != "ally" || nodes[groups[0].Indices[1]].Name != "prowl" {
		t.Fatalf("group members = %v, want ally and prowl", groups[0].Indices)
	}
}

func assertPlacementClear(t *testing.T, placement mapPlacement) {
	t.Helper()
	labels := make([]image.Rectangle, 0, len(placement.Nodes)+len(placement.Rings)+len(placement.Groups))
	for _, node := range placement.Nodes {
		if !node.Label.Empty() {
			labels = append(labels, node.Label)
		}
	}
	for _, ring := range placement.Rings {
		if !ring.Box.Empty() {
			labels = append(labels, ring.Box)
		}
	}
	for _, group := range placement.Groups {
		if !group.Box.Empty() {
			labels = append(labels, group.Box)
		}
	}
	for i := range labels {
		for j := i + 1; j < len(labels); j++ {
			if labels[i].Overlaps(labels[j]) {
				t.Fatalf("labels %d and %d overlap: %v and %v", i, j, labels[i], labels[j])
			}
		}
		for j, node := range placement.Nodes {
			obstacle := image.Rect(node.Point.X-12, node.Point.Y-12, node.Point.X+12, node.Point.Y+12)
			if labels[i].Overlaps(obstacle) {
				t.Fatalf("label %d overlaps node %d: %v and %v", i, j, labels[i], obstacle)
			}
		}
	}
}
