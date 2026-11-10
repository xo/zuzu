package dataflow

import (
	"math"
	"testing"
)

func TestDotbinAndCountpattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, rows, ts, want string
	}{
		{"dotbin", `[{"v":3},{"v":1},{"v":10},{"v":1.2},{"v":3.1}]`,
			`[{"type":"dotbin","field":"v","step":1}]`,
			`[{"bin":3.05,"v":3},{"bin":1.1,"v":1},{"bin":10,"v":10},{"bin":1.1,"v":1.2},{"bin":3.05,"v":3.1}]`},
		// The word that is an index of an array comes first, as the keys
		// of an object of JavaScript do.
		{"countpattern", `[{"t":"The cat and the hat"},{"t":"cat 42"}]`,
			`[{"type":"countpattern","field":"t","case":"lower","stopwords":"(the|and)"}]`,
			`[{"count":1,"text":"42"},{"count":2,"text":"cat"},{"count":1,"text":"hat"}]`},
	}
	for _, test := range tests {
		if got := run(t, newEnv(), test.rows, test.ts); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", test.name, got, test.want)
		}
	}
}

func TestIsocontour(t *testing.T) {
	t.Parallel()
	// A peak in the middle of a grid of 3 by 3 gives one ring at 0.5,
	// halfway between the cells, after the smoothing.
	rows := rowsOf(t, `[{"width":3,"height":3,"values":[0,0,0,0,1,0,0,0,0]}]`)
	out, err := Run(newEnv(), rows, transformsOf(t, `[{"type":"isocontour","thresholds":[0.5]}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1", len(out))
	}
	c, _ := out[0]["contour"].(map[string]any)
	polys, _ := c["coordinates"].([]any)
	if len(polys) != 1 {
		t.Fatalf("got %d polygons, want 1", len(polys))
	}
	poly, _ := polys[0].([]any)
	ring, _ := poly[0].([]any)
	for _, p := range ring {
		xy, _ := p.([]any)
		x, _ := xy[0].(float64)
		y, _ := xy[1].(float64)
		if x < 1 || x > 2 || y < 1 || y > 2 {
			t.Errorf("the point %v lies outside the middle cell", xy)
		}
	}
}

func TestKDE2DSumsToOne(t *testing.T) {
	t.Parallel()
	rows := rowsOf(t, `[{"x":10,"y":10},{"x":20,"y":12},{"x":15,"y":30}]`)
	out, err := Run(newEnv(), rows, transformsOf(t, `[{"type":"kde2d","x":"x","y":"y","size":[40,40],"bandwidth":[8,8],"cellSize":2}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	grid, _ := out[0]["grid"].(map[string]any)
	values, _ := grid["values"].([]any)
	sum := 0.0
	for _, v := range values {
		f, _ := v.(float64)
		sum += f
	}
	if math.Abs(sum-1) > 1e-5 {
		t.Errorf("the density sums to %v, want 1", sum)
	}
	w, _ := grid["width"].(float64)
	h, _ := grid["height"].(float64)
	if int(w)*int(h) != len(values) {
		t.Errorf("the grid of %v by %v has %d values", grid["width"], grid["height"], len(values))
	}
}

func TestHeatmapMakesAnImage(t *testing.T) {
	t.Parallel()
	rows := rowsOf(t, `[{"width":2,"height":1,"values":[0,1]}]`)
	out, err := Run(newEnv(), rows, transformsOf(t, `[{"type":"heatmap","color":"red"}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := out[0]["image"].(string); len(s) < 30 || s[:22] != "data:image/png;base64," {
		t.Errorf("image = %.40q", s)
	}
}
