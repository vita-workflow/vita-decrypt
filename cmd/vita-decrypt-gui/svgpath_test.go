package main

import (
	"errors"
	"math"
	"strings"
	"testing"

	"gioui.org/f32"
)

// parsePath parses data into finished subpaths.
func parsePath(data string) ([]subpath, error) {
	var builder pathBuilder
	err := builder.parse(data)
	builder.finish()
	return builder.subpaths, err
}

// ends lists where each curve of subpaths ends, starting each subpath with its start point.
func ends(subpaths []subpath) [][]f32.Point {
	var all [][]f32.Point
	for _, path := range subpaths {
		points := []f32.Point{path.start}
		for _, curve := range path.curves {
			points = append(points, curve.end)
		}
		all = append(all, points)
	}
	return all
}

func near(first, second f32.Point) bool {
	const tolerance = 1e-3
	return math.Abs(float64(first.X-second.X)) < tolerance && math.Abs(float64(first.Y-second.Y)) < tolerance
}

func sameEnds(got, want [][]f32.Point) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}
		for j := range got[i] {
			if !near(got[i][j], want[i][j]) {
				return false
			}
		}
	}
	return true
}

func TestPathBuilderParse(t *testing.T) {
	point := f32.Pt
	tests := []struct {
		name string
		data string
		want [][]f32.Point
	}{
		{"empty", "", nil},
		{"lines", "M1 2 L3 4", [][]f32.Point{{point(1, 2), point(3, 4)}}},
		{"implicit lines after a move", "M1 2 3 4 5 6", [][]f32.Point{{point(1, 2), point(3, 4), point(5, 6)}}},
		{"relative", "m1 2 l3 4 h1 v-1", [][]f32.Point{{point(1, 2), point(4, 6), point(5, 6), point(5, 5)}}},
		{"absolute horizontal and vertical", "M1 1H5V7", [][]f32.Point{{point(1, 1), point(5, 1), point(5, 7)}}},
		{
			"close returns to the start", "M0 0 L4 0 L4 4 Z",
			[][]f32.Point{{point(0, 0), point(4, 0), point(4, 4), point(0, 0)}},
		},
		{"close at the start adds no line", "M0 0 L4 0 L0 0 z", [][]f32.Point{{point(0, 0), point(4, 0), point(0, 0)}}},
		{"close with no subpath", "Z", nil},
		{
			"line after close starts at the old start", "M1 1 L2 2 Z L5 5",
			[][]f32.Point{{point(1, 1), point(2, 2), point(1, 1)}, {point(1, 1), point(5, 5)}},
		},
		{"two subpaths", "M0 0 L1 1 M5 5 L6 6", [][]f32.Point{{point(0, 0), point(1, 1)}, {point(5, 5), point(6, 6)}}},
		{"cubic", "M0 0 C1 1 2 2 3 0", [][]f32.Point{{point(0, 0), point(3, 0)}}},
		{
			"smooth cubic", "M0 0 C0 1 2 1 2 0 S4 -1 4 0 s2 1 2 0",
			[][]f32.Point{{point(0, 0), point(2, 0), point(4, 0), point(6, 0)}},
		},
		{"quadratic", "M0 0 Q1 1 2 0 T4 0 t2 0", [][]f32.Point{{point(0, 0), point(2, 0), point(4, 0), point(6, 0)}}},
		{"arc in quarters", "M0 0 A1 1 0 0 1 2 0", [][]f32.Point{{point(0, 0), point(1, -1), point(2, 0)}}},
		{"large arc the other way", "M0 0 a1 1 0 1 0 2 0", [][]f32.Point{{point(0, 0), point(1, 1), point(2, 0)}}},
		{"flat arc is a line", "M0 0 A0 1 0 0 1 2 0", [][]f32.Point{{point(0, 0), point(2, 0)}}},
		{"compact numbers", "M.5.5-1-1e1", [][]f32.Point{{point(0.5, 0.5), point(-1, -10)}}},
		{"exponent with sign", "M1e+1 2E-1", [][]f32.Point{{point(10, 0.2)}}},
		{"compact arc flags", "M0 0a1 1 0 012 0", [][]f32.Point{{point(0, 0), point(1, -1), point(2, 0)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subpaths, err := parsePath(test.data)
			if err != nil {
				t.Fatal(err)
			}
			if got := ends(subpaths); !sameEnds(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestPathBuilderParse_ScalesUpArcsTooSmallToReachTheirEnd(t *testing.T) {
	subpaths, err := parsePath("M0 0 A0.5 0.25 30 0 1 4 0")
	if err != nil {
		t.Fatal(err)
	}
	curves := subpaths[0].curves
	if len(curves) < 2 || !near(curves[len(curves)-1].end, f32.Pt(4, 0)) {
		t.Errorf("the arc is %d curves ending at %v, want a half ellipse ending at (4, 0)", len(curves),
			curves[len(curves)-1].end)
	}
}

func TestPathBuilderParse_ClosedFlag(t *testing.T) {
	subpaths, err := parsePath("M0 0 L1 0 Z M5 5 L6 6")
	if err != nil {
		t.Fatal(err)
	}
	if !subpaths[0].isClosed || subpaths[1].isClosed {
		t.Errorf("closed flags %v %v, want true false", subpaths[0].isClosed, subpaths[1].isClosed)
	}
}

func TestPathBuilderParse_Errors(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		{"no command", "1 2", "starts without a command"},
		{"unsupported command", "M0 0 X1", "unsupported path command X"},
		{"missing number", "M0", "bad number"},
		{"bad number", "M0 0 L1 -", "bad number"},
		{"bad arc flag", "M0 0 A1 1 0 2 1 2 0", "bad arc flag"},
		{"missing arc flag", "M0 0 A1 1 0 1", "bad arc flag"},
		{"missing arc radius", "M0 0 A", "bad number"},
		{"missing arc y radius", "M0 0 A1", "bad number"},
		{"missing arc rotation", "M0 0 A1 1", "bad number"},
		{"missing arc sweep", "M0 0 A1 1 0 0 ", "bad arc flag"},
		{"missing arc end", "M0 0 A1 1 0 0 1 2", "bad number"},
		{"missing cubic control", "M0 0 C1", "bad number"},
		{"missing cubic second control", "M0 0 C1 1", "bad number"},
		{"missing cubic end", "M0 0 C1 1 2 2", "bad number"},
		{"missing quadratic control", "M0 0 Q", "bad number"},
		{"missing quadratic end", "M0 0 Q1 1", "bad number"},
		{"missing horizontal", "M0 0 H", "bad number"},
		{"missing vertical", "M0 0 V", "bad number"},
		{"missing line", "M0 0 L", "bad number"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parsePath(test.data)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %v, want one mentioning %q", err, test.want)
			}
		})
	}
	if _, err := parsePath("M0 0 Z 1"); !errors.Is(err, errNoProgress) {
		t.Errorf("numbers after Z gave %v, want errNoProgress", err)
	}
}

func TestParseNumbers(t *testing.T) {
	tests := []struct {
		list string
		want []float32
	}{
		{"", nil},
		{"1 2,3", []float32{1, 2, 3}},
		{" 4,5 6 x 7", []float32{4, 5, 6}},
		{"-1.5-2", []float32{-1.5, -2}},
	}
	for _, test := range tests {
		got := parseNumbers(test.list)
		if len(got) != len(test.want) {
			t.Errorf("parseNumbers(%q) = %v, want %v", test.list, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("parseNumbers(%q) = %v, want %v", test.list, got, test.want)
			}
		}
	}
}

func FuzzParsePath(f *testing.F) {
	seeds := []string{"M1 2 L3 4 Z", "m0 0c1 1 2 2 3 3s1 1 2 2", "M0 0 a1 1 0 1 0 2 0", "M0 0 Q1 1 2 0 T4 0"}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data string) {
		var builder pathBuilder
		err := builder.parse(data)
		builder.finish()
		if err != nil {
			return
		}
		for _, path := range builder.subpaths {
			if path.isClosed && len(path.curves) > 0 && !samePoint(path.curves[len(path.curves)-1].end, path.start) {
				t.Errorf("closed subpath of %q ends at %v, not its start %v", data,
					path.curves[len(path.curves)-1].end, path.start)
			}
		}
		// Parsing is deterministic.
		again, _ := parsePath(data)
		if len(again) != len(builder.subpaths) {
			t.Errorf("parsing %q twice gave %d and %d subpaths", data, len(builder.subpaths), len(again))
		}
	})
}

// samePoint compares points exactly, treating NaN coordinates as equal to each other.
func samePoint(first, second f32.Point) bool {
	return sameCoordinate(first.X, second.X) && sameCoordinate(first.Y, second.Y)
}

func sameCoordinate(first, second float32) bool {
	return first == second || (math.IsNaN(float64(first)) && math.IsNaN(float64(second)))
}
