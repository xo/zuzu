package mermaid

import (
	"math"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// vennCircleWant is a circle and the place of its label as mmdc 12.3.0
// draws them, in the pixels of Mermaid: the center, the radius, and the
// point of the label, which Mermaid rounds down.
type vennCircleWant struct {
	id             string
	x, y, r        float64
	labelX, labelY float64
}

func TestVennPlacement(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src string
		want      []vennCircleWant
	}{
		{
			"three", "venn-beta\n  set A\n  set B\n  set C\n  union A, B\n  union B, C\n  union A, C\n  union A, B, C\n",
			[]vennCircleWant{
				{"A", 314.261, 299.251, 135.749, 268, 325},
				{"B", 485.739, 299.251, 135.749, 531, 325},
				{"C", 399.999, 150.749, 135.749, 400, 98},
			},
		},
		{
			"sizes of two", "venn-beta\n  set A:30\n  set B:10\n  union A, B:5\n",
			[]vennCircleWant{
				{"A", 345.451, 225, 210, 278, 225},
				{"B", 543.306, 225, 121.244, 610, 225},
			},
		},
		{
			"sizes of three", "venn-beta\n  set A:20\n  set B:15\n  set C:10\n  union A, B:5\n  union B, C:3\n  union A, C:2\n  union A, B, C:1\n",
			[]vennCircleWant{
				{"A", 323.269, 278.286, 156.714, 264, 278},
				{"B", 497.727, 278.286, 135.718, 556, 284},
				{"C", 456.161, 125.814, 110.814, 444, 83},
			},
		},
		{
			"a subset", "venn-beta\n  set A:10\n  set B:4\n  union A,B:4\n",
			[]vennCircleWant{
				{"A", 400, 225, 210, 267, 225},
				{"B", 477.184, 225, 132.816, 477, 225},
			},
		},
		{
			"one union of three", "venn-beta\n  title Innovation\n  set Desirable\n  set Feasible\n  set Viable\n  union Desirable,Feasible,Viable[\"Innovation\"]\n",
			[]vennCircleWant{
				{"Desirable", 319.161, 283.008, 127.992, 276, 307},
				{"Feasible", 480.839, 283.008, 127.992, 523, 307},
				{"Viable", 400, 142.992, 127.992, 399, 93},
				{"Desirable,Feasible,Viable", 0, 0, 0, 399, 236},
			},
		},
		{
			"apart", "venn-beta\n  title Programming Languages\n  set \"JavaScript\"\n  set \"Python\"\n  set \"TypeScript\"\n  union \"JavaScript\", \"TypeScript\"\n",
			[]vennCircleWant{
				{"JavaScript", 159.335, 213, 144.335, 106, 212},
				{"Python", 640.665, 213, 144.335, 640, 213},
				{"TypeScript", 342.556, 213, 144.335, 395, 213},
			},
		},
		{
			"four", "venn-beta\n  set A\n  set B\n  set C\n  set D\n  union A,B\n  union B,C\n  union C,D\n  union A,D\n  union A,C\n",
			[]vennCircleWant{
				{"A", 356.995, 299.487, 135.513, 326, 352},
				{"B", 529.016, 299.487, 135.513, 574, 325},
				{"C", 443.005, 150.513, 135.513, 473, 97},
				{"D", 270.984, 150.513, 135.513, 225, 124},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, err := Read(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			v := &venn{c: &chart{d: d, issues: &graph.Issues{Strict: true}}, styles: map[string]map[string]string{},
				styleLines: map[string]int{}, known: map[string]bool{}, width: 800, height: 450, padding: 8}
			if err := v.parse(); err != nil {
				t.Fatal(err)
			}
			p := vennPlace(v, v.hasTitle)
			for _, w := range tc.want {
				if !strings.Contains(w.id, ",") {
					c := p.drawn.circles[w.id]
					if c == nil {
						t.Fatalf("no circle %s", w.id)
					}
					// The four sets lay out with a random start in Mermaid,
					// which it keeps when it fits better, so they match to a
					// pixel.
					tol := 0.01
					if tc.name == "four" {
						tol = 1
					}
					if math.Abs(c.x-w.x) > tol || math.Abs(c.y-w.y) > tol || math.Abs(c.r-w.r) > tol {
						t.Errorf("%s: the circle (%.3f, %.3f) %.3f, want (%.3f, %.3f) %.3f", w.id, c.x, c.y, c.r, w.x, w.y, w.r)
					}
				}
				lc, ok := p.labels[w.id]
				if !ok {
					t.Fatalf("no label of %s", w.id)
				}
				if math.Abs(math.Floor(lc.x)-w.labelX) > 1 || math.Abs(math.Floor(lc.y)-w.labelY) > 1 {
					t.Errorf("%s: the label at (%v, %v), want (%v, %v)", w.id, math.Floor(lc.x), math.Floor(lc.y), w.labelX, w.labelY)
				}
			}
		})
	}
}

func TestVennGrid(t *testing.T) {
	t.Parallel()
	src := "venn-beta\n  set A[\"Frontend\"]\n    text A1[\"React\"]\n    text A2[\"Design Systems\"]\n" +
		"  set B[\"Backend\"]\n    text B1[\"API\"]\n  union A,B[\"Shared\"]\n    text AB1[\"OpenAPI\"]\n"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	v := &venn{c: &chart{d: d, issues: &graph.Issues{Strict: true}}, styles: map[string]map[string]string{},
		styleLines: map[string]int{}, known: map[string]bool{}, width: 800, height: 450, padding: 8}
	if err := v.parse(); err != nil {
		t.Fatal(err)
	}
	p := vennPlace(v, false)
	// The boxes of the texts as mmdc 12.3.0 draws them: the left and the
	// top of the box of each text, and its width, which is 0.9 of its
	// cell.
	want := map[string][][3]float64{
		"A":   {{58.698, 138.24, 117.76}, {189.542, 138.24, 117.76}},
		"B":   {{499.24, 138.24, 235.52}},
		"A,B": {{332.225, 188.225, 135.55}},
	}
	for _, a := range p.data {
		key := vennKey(a.sets, ",")
		boxes := want[key]
		x0, y0, cw, ch, cols := vennGrid(a, len(boxes), p.texts[key], p.placed, 0.5)
		for i, b := range boxes {
			x := x0 + cw*(float64(i%cols)+0.5)
			y := y0 + ch*(float64(i/cols)+0.5)
			w, h := cw*0.9, ch*0.9
			if math.Abs(x-w/2-b[0]) > 0.01 || math.Abs(y-h/2-b[1]) > 0.01 || math.Abs(w-b[2]) > 0.01 {
				t.Errorf("%s %d: the box at (%.3f, %.3f), %.3f wide, want (%.3f, %.3f), %.3f wide", key, i, x-w/2, y-h/2, w, b[0], b[1], b[2])
			}
		}
	}
}
