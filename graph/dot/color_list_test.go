package dot

import (
	"slices"
	"testing"
)

// TestEdgeColorList checks that an edge with a list of colors draws a line
// in each color: side by side with no weight, with the arrowheads in the
// first color, and one after another with a weight, with the arrowheads in
// the last color.
func TestEdgeColorList(t *testing.T) {
	t.Parallel()
	g, report, err := Parse(`digraph {
	a -> b [color="red:blue"]
	a -> c [color="red;0.3:/blues9/9"]
	a -> d [color="red:nocolor"]
	a -> e [color="red;2:blue"]
	a -> f [color="red:blue", style=invis]
}`, false)
	if err != nil {
		t.Fatal(err)
	}
	e := g.Edges
	if !slices.Equal(e[0].Strokes, []string{"red", "blue"}) || e[0].Weights != nil || e[0].Style["stroke"] != "red" {
		t.Errorf("a -> b: %v %v %v", e[0].Strokes, e[0].Weights, e[0].Style)
	}
	if !slices.Equal(e[1].Strokes, []string{"red", "#08306b"}) || !slices.Equal(e[1].Weights, []float64{0.3, -1}) || e[1].Style["stroke"] != "#08306b" {
		t.Errorf("a -> c: %v %v %v", e[1].Strokes, e[1].Weights, e[1].Style)
	}
	for _, i := range []int{2, 3, 4} {
		if e[i].Strokes != nil {
			t.Errorf("edge %d: expected one color, got %v", i, e[i].Strokes)
		}
	}
	if len(report) != 2 {
		t.Errorf("expected the color and the weight as issues, got %v", report)
	}
}
