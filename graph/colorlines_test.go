package graph

import (
	"math"
	"slices"
	"testing"
)

// TestColorLinesSideBySide checks that the lines of colors with no weight
// lie side by side, gap apart, the first at the right of the route, and
// stop at the arrowheads.
func TestColorLinesSideBySide(t *testing.T) {
	t.Parallel()
	// A route down from 0, 0 to 0, 100: its right is toward -x.
	paths := ColorLines("M0,0L0,100", 3, nil, 2, 0, 10)
	want := []string{"M-2,0L-2,90", "M0,0L0,90", "M2,0L2,90"}
	if !slices.Equal(paths, want) {
		t.Errorf("expected %q, got %q", want, paths)
	}
	// A curve reads as a line through points along it.
	paths = ColorLines("M0,0C0,50 100,50 100,100", 2, nil, 2, 0, 0)
	if pts := flatten(paths[0]); len(pts) != 17 || math.Abs(pts[0][0]+1) > 0.02 {
		t.Errorf("expected 17 points from about -1, 0, got %v", pts)
	}
	if ColorLines("", 2, nil, 2, 0, 0) != nil {
		t.Error("expected no lines for no route")
	}
}

// TestColorLinesInSeries checks that colors with weights draw one after
// another along the route, the colors with no weight sharing the rest.
func TestColorLinesInSeries(t *testing.T) {
	t.Parallel()
	paths := ColorLines("M0,0L0,100", 3, []float64{0.5, -1, 0.25}, 2, 10, 10)
	want := []string{"M0,0L0,50", "M0,50L0,75", "M0,75L0,100"}
	if !slices.Equal(paths, want) {
		t.Errorf("expected %q, got %q", want, paths)
	}
	for _, tc := range []struct {
		weights []float64
		want    []float64
	}{
		{[]float64{0.3, -1}, []float64{0.3, 0.7}},
		{[]float64{0.3, 0.2}, []float64{0.3, 0.7}},
		{[]float64{-1, -1, 0.5}, []float64{0.25, 0.25, 0.5}},
		{[]float64{0.8, 0.8, -1}, []float64{0.8, 0.8, 0}},
	} {
		if got := shares(tc.weights, len(tc.weights)); !slices.Equal(got, tc.want) {
			t.Errorf("%v: expected the shares %v, got %v", tc.weights, tc.want, got)
		}
	}
}

func TestArrowLength(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]float64{"normal": 10, "": 10, "none": 0, "invempty": 10, "normalnormal": 20} {
		if got := ArrowLength(kind, 100); math.Abs(got-want) > 1e-9 {
			t.Errorf("%q: expected %g, got %g", kind, want, got)
		}
	}
}
