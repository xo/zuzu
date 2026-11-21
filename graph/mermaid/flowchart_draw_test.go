package mermaid_test

import (
	"math"
	"testing"

	"github.com/xo/zuzu/expr"
)

// nodeSizes draws a source of Mermaid and returns the width and the height
// of each node by its id.
func nodeSizes(t *testing.T, src string) map[string][2]float64 {
	t.Helper()
	items, report, err := drawDiagram(t, src, false)
	if err != nil {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	out := map[string][2]float64{}
	for _, it := range items {
		if it.datum["kind"] != "node" {
			continue
		}
		key := expr.String(it.datum["key"])
		if _, ok := out[key]; !ok {
			out[key] = [2]float64{expr.Number(it.datum["width"]), expr.Number(it.datum["height"])}
		}
	}
	return out
}

// TestMinNodeWidthDraws checks that the layout makes the box of a label at
// least flowchart.minNodeWidth wide, 120 in the pixels of Mermaid and 120
// times 11/14 in the pixels of zuzu, and grows the shape around it, as
// mmdc of Mermaid 12.3.0 does. A rectangle is that width and its padding
// of 8 on each side, and a diamond is as high as it is wide. A label in
// characters that no font has takes the least width too, and a long label
// wraps inside it. A circle and a small circle keep their own size. With
// minNodeWidth 0, each node takes the width of its label.
func TestMinNodeWidthDraws(t *testing.T) {
	t.Parallel()
	const body = "flowchart LR\n  a[A] --> b{B} --> c((C)) --> d@{ shape: sm-circ } --> e[\u65e5\u672c\u8a9e]\n" +
		"  f[A label that is wider than the least width of a node]\n"
	least := 120.0*11/14 + 2*8
	sizes := nodeSizes(t, body)
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	if w := sizes["a"][0]; !near(w, least) {
		t.Errorf("the rectangle a is %v wide, want %v", w, least)
	}
	if b := sizes["b"]; b[0] <= least || !near(b[0], b[1]) {
		t.Errorf("the diamond b is %v, want a square wider than %v", b, least)
	}
	if w := sizes["c"][0]; w >= least/2 {
		t.Errorf("the circle c is %v wide, want the size of its label", w)
	}
	if d := sizes["d"]; d != [2]float64{14, 14} {
		t.Errorf("the small circle d is %v, want 14 by 14", d)
	}
	if w := sizes["e"][0]; !near(w, least) {
		t.Errorf("the rectangle e is %v wide, want %v", w, least)
	}
	if w := sizes["f"][0]; !near(w, least) || sizes["f"][1] <= sizes["a"][1] {
		t.Errorf("the rectangle f is %v, want %v wide with its label in more lines", sizes["f"], least)
	}
	sizes = nodeSizes(t, "---\nconfig:\n  flowchart:\n    minNodeWidth: 0\n---\n"+body)
	if w := sizes["a"][0]; w >= least/2 {
		t.Errorf("with minNodeWidth 0, the rectangle a is %v wide, want the size of its label", w)
	}
}
