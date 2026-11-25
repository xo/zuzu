package graph

import (
	"math"
	"testing"
)

func TestSpreadRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []float64
		half float64
		want []float64
	}{
		{"apart", []float64{-10, 0, 10}, 40, []float64{-10, 0, 10}},
		{"two close", []float64{0, 2}, 40, []float64{-3, 5}},
		{"three at one point", []float64{5, 5, 5}, 40, []float64{-3, 5, 13}},
		// A run that pushes into its neighbor joins it.
		{"join", []float64{0, 1, 9}, 40, []float64{-4.667, 3.333, 11.333}},
		// A run near a corner stays inside the side.
		{"corner", []float64{19, 20}, 20, []float64{8, 16}},
		// A run wider than the side narrows its gaps.
		{"narrow", []float64{0, 0, 0}, 6, []float64{-3, 0, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := append([]float64(nil), tt.in...)
			moved := spreadRun(got, tt.half)
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 0.001 {
					t.Fatalf("spreadRun(%v) = %v, want %v", tt.in, got, tt.want)
				}
			}
			if want := tt.name != "apart"; moved != want {
				t.Errorf("spreadRun moved %v, want %v", moved, want)
			}
		})
	}
}

func TestSpreadEnds(t *testing.T) {
	t.Parallel()
	// Two edges come into the top of n from the right, one steeper than
	// the other, and their ends are 1 apart. They move 8 apart and keep
	// their order, and each stays on the top of n. The edge at a port
	// stays.
	in := &LayoutInput{
		Nodes: []LayoutNode{{ID: "n", Shape: "rect"}, {ID: "p"}, {ID: "q"}, {ID: "r"}},
		Edges: []LayoutEdge{
			{Source: "p", Target: "n"}, {Source: "n", Target: "q"},
			{Source: "r", Target: "n", TargetPort: []float64{0, -10}},
		},
	}
	out := &Layout{
		Nodes: []NodePlace{{X: 0, Y: 0, Width: 60, Height: 20}, {X: 40, Y: -100}, {X: 50, Y: -80}, {X: 0, Y: -50}},
		Edges: []EdgePlace{
			{Points: [][2]float64{{40, -100}, {4, -10}}},
			{Points: [][2]float64{{5, -10}, {50, -80}}},
			{Points: [][2]float64{{0, -50}, {4.5, -10}}},
		},
	}
	ids := map[string]int{"n": 0, "p": 1, "q": 2, "r": 3}
	spreadEnds(in, out, ids)
	a, b := out.Edges[0].Points[1], out.Edges[1].Points[0]
	if a[1] != -10 || b[1] != -10 || b[0]-a[0] < endGap-1e-9 || a[0] >= 4 || b[0] <= 5 {
		t.Errorf("the ends are %v and %v, want them %v apart on the top, in order", a, b, endGap)
	}
	if e := out.Edges[0]; e.EndX != a[0] || e.EndY != a[1] {
		t.Errorf("the end of the edge is %v, %v, want %v", e.EndX, e.EndY, a)
	}
	if p := out.Edges[2].Points[1]; p != [2]float64{4.5, -10} {
		t.Errorf("the end at a port moved to %v", p)
	}
}
