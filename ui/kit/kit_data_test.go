package kit

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"testing"

	"gioui.org/f32"
)

func TestFlakeGeometryUsesFortyFiveDegreeSides(t *testing.T) {
	got := flakeGeometry(14, 7)
	want := [4]image.Point{{X: 7}, {X: 14}, {X: 7, Y: 7}, {Y: 7}}
	if got != want {
		t.Fatalf("flakeGeometry(14, 7) = %v, want %v", got, want)
	}
}

func TestFlakeFillWidthClampsAndRounds(t *testing.T) {
	for _, tc := range []struct {
		frac float64
		want int
	}{
		{frac: -1, want: 0},
		{frac: .25, want: 4},
		{frac: .5, want: 7},
		{frac: 2, want: 14},
	} {
		if got := flakeFillWidth(14, tc.frac); got != tc.want {
			t.Fatalf("flakeFillWidth(14, %g) = %d, want %d", tc.frac, got, tc.want)
		}
	}
}

func TestPolylineQuadCorners(t *testing.T) {
	points := [...]f32.Point{
		f32.Pt(0, 0),
		f32.Pt(10, 0),
		f32.Pt(10, 10),
	}
	want := [...]polylineQuad{
		{
			f32.Pt(0, 1),
			f32.Pt(0, -1),
			f32.Pt(10, -1),
			f32.Pt(10, 1),
		},
		{
			f32.Pt(9, 0),
			f32.Pt(11, 0),
			f32.Pt(11, 10),
			f32.Pt(9, 10),
		},
	}
	for i := range len(points) - 1 {
		got, ok := makePolylineQuad(points[i], points[i+1], 2)
		if !ok {
			t.Fatalf("segment %d was rejected", i)
		}
		if got != want[i] {
			t.Fatalf("segment %d quad = %v, want %v", i, got, want[i])
		}
	}
	join, count := polylineJoin(points[0], points[1], points[2], 2)
	if count != 4 {
		t.Fatalf("miter join has %d corners, want 4", count)
	}
	wantJoin := [4]f32.Point{
		f32.Pt(10, -1),
		f32.Pt(11, -1),
		f32.Pt(11, 0),
		f32.Pt(10, 0),
	}
	if join != wantJoin {
		t.Fatalf("miter join = %v, want %v", join, wantJoin)
	}
}

func TestChartPointsAutoScale(t *testing.T) {
	values := []float64{10, 20, 15}
	lo, hi := chartRange(values, math.NaN(), math.NaN())
	if lo != 10 || hi != 20 {
		t.Fatalf("chartRange = (%g, %g), want (10, 20)", lo, hi)
	}
	points := chartPoints(values, math.NaN(), math.NaN(), image.Pt(101, 51))
	want := [][2]float32{{0, 50}, {50, 0}, {100, 25}}
	if len(points) != len(want) {
		t.Fatalf("chartPoints returned %d points, want %d", len(points), len(want))
	}
	for i, point := range points {
		if point.X != want[i][0] || point.Y != want[i][1] {
			t.Fatalf("point %d = (%g, %g), want (%g, %g)", i, point.X, point.Y, want[i][0], want[i][1])
		}
	}
}

func TestChartRangeExpandsFlatSeries(t *testing.T) {
	lo, hi := chartRange([]float64{8, 8}, math.NaN(), math.NaN())
	if !(lo < 8 && hi > 8) {
		t.Fatalf("flat chart range = (%g, %g), want a range around 8", lo, hi)
	}
}

func TestRowOffsetsKeepVariableElements(t *testing.T) {
	offsets := fillRowOffsets(nil, []int{56, 36, 0, 48}, 4, 56)
	want := []int{0, 60, 100, 160}
	if len(offsets) != len(want) {
		t.Fatalf("offset count = %d, want %d", len(offsets), len(want))
	}
	for i := range want {
		if offsets[i] != want[i] {
			t.Fatalf("offset %d = %d, want %d", i, offsets[i], want[i])
		}
	}
}

func TestLogoCacheKeyIncludesPathAndPixelSize(t *testing.T) {
	base := logoCacheKey("/logos/fedora.svg", 32)
	if base != logoCacheKey("/logos/fedora.svg", 32) {
		t.Fatal("equal path and pixel size produced different keys")
	}
	if base == logoCacheKey("/logos/fedora.svg", 48) {
		t.Fatal("different pixel sizes produced the same key")
	}
	if base == logoCacheKey("/logos/ubuntu.svg", 32) {
		t.Fatal("different paths produced the same key")
	}
}

func TestNormalizeCompactSimpleIconPaths(t *testing.T) {
	const arch = "M11.39.605C10.376 3.092 9.764 4.72 8.635 7.132c.693.734 1.543 1.589 2.923 2.554-1.484-.61-2.496-1.224-3.252-1.86C6.86 10.842 4.596 15.138 0 23.395c3.612-2.085 6.412-3.37 9.021-3.862a6.61 6.61 0 01-.171-1.547l.003-.115c.058-2.315 1.261-4.095 2.687-3.973 1.426.12 2.534 2.096 2.478 4.409a6.52 6.52 0 01-.146 1.243c2.58.505 5.352 1.787 8.914 3.844-.702-1.293-1.33-2.459-1.929-3.57-.943-.73-1.926-1.682-3.933-2.713 1.38.359 2.367.772 3.137 1.234-6.09-11.334-6.582-12.84-8.67-17.74zM22.898 21.36v-.623h-.234v-.084h.562v.084h-.234v.623h.331v-.707h.142l.167.5.034.107a2.26 2.26 0 01.038-.114l.17-.493H24v.707h-.091v-.593l-.206.593h-.084l-.205-.602v.602h-.091"
	const ubuntu = "M17.61.455a3.41 3.41 0 0 0-3.41 3.41 3.41 3.41 0 0 0 3.41 3.41 3.41 3.41 0 0 0 3.41-3.41 3.41 3.41 0 0 0-3.41-3.41zM12.92.8C8.923.777 5.137 2.941 3.148 6.451a4.5 4.5 0 0 1 .26-.007 4.92 4.92 0 0 1 2.585.737A8.316 8.316 0 0 1 12.688 3.6 4.944 4.944 0 0 1 13.723.834 11.008 11.008 0 0 0 12.92.8zm9.226 4.994a4.915 4.915 0 0 1-1.918 2.246 8.36 8.36 0 0 1-.273 8.303 4.89 4.89 0 0 1 1.632 2.54 11.156 11.156 0 0 0 .559-13.089zM3.41 7.932A3.41 3.41 0 0 0 0 11.342a3.41 3.41 0 0 0 3.41 3.409 3.41 3.41 0 0 0 3.41-3.41 3.41 3.41 0 0 0-3.41-3.41zm2.027 7.866a4.908 4.908 0 0 1-2.915.358 11.1 11.1 0 0 0 7.991 6.698 11.234 11.234 0 0 0 2.422.249 4.879 4.879 0 0 1-.999-2.85 8.484 8.484 0 0 1-.836-.136 8.304 8.304 0 0 1-5.663-4.32zm11.405.928a3.41 3.41 0 0 0-3.41 3.41 3.41 3.41 0 0 0 3.41 3.41 3.41 3.41 0 0 0 3.41-3.41 3.41 3.41 0 0 0-3.41-3.41z"

	for _, tc := range []struct {
		name      string
		path      string
		fragments []string
	}{
		{
			name: "arch",
			path: arch,
			fragments: []string{
				"M 11.39 .605 C 10.376 3.092",
				"c .693 .734 1.543 1.589",
				"a 6.61 6.61 0 0 1 -.171 -1.547",
				"z M 22.898 21.36 v -.623",
			},
		},
		{
			name: "ubuntu",
			path: ubuntu,
			fragments: []string{
				"M 17.61 .455 a 3.41 3.41 0 0 0 -3.41",
				"z M 12.92 .8 C 8.923 .777",
				"z M 3.41 7.932 A 3.41 3.41",
				"z m 11.405 .928 a 3.41 3.41",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizePathData(tc.path)
			for _, fragment := range tc.fragments {
				if !strings.Contains(got, fragment) {
					t.Fatalf("normalized path does not contain %q:\n%s", fragment, got)
				}
			}
		})
	}
}

func TestColouredArt(t *testing.T) {
	paint := func(fills ...color.RGBA) *image.RGBA {
		canvas := image.NewRGBA(image.Rect(0, 0, 10, 10))
		for i, fill := range fills {
			draw.Draw(canvas, image.Rect(0, i*10/len(fills), 10, (i+1)*10/len(fills)), image.NewUniform(fill), image.Point{}, draw.Src)
		}
		return canvas
	}
	bone := color.RGBA{230, 220, 203, 255}
	if colouredArt(paint(bone)) {
		t.Fatal("a single fill counts as coloured art")
	}
	edge := paint(bone)
	edge.SetRGBA(0, 0, color.RGBA{115, 110, 101, 128})
	if colouredArt(edge) {
		t.Fatal("antialiased edges count as a second colour")
	}
	if !colouredArt(paint(color.RGBA{255, 255, 255, 255}, color.RGBA{0, 120, 212, 255})) {
		t.Fatal("a mark on its own disc is not coloured art")
	}
}

func TestWindowsLogoPath(t *testing.T) {
	for _, path := range []string{"/logos/windows.svg", "/logos/windows.png", "/logos/WINDOWS.SVG"} {
		if !isWindowsLogo(path) {
			t.Fatalf("isWindowsLogo(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"/logos/windows-11.svg", "/logos/windows.svg.old", "/logos/ubuntu.svg"} {
		if isWindowsLogo(path) {
			t.Fatalf("isWindowsLogo(%q) = true, want false", path)
		}
	}
}
