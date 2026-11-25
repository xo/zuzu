package graph

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
)

func TestCompileFigure(t *testing.T) {
	t.Parallel()
	spec, err := Compile(&Graph{Nodes: []*Node{
		{ID: "a", Media: &Media{Path: "M0,0H1", Width: 30, Height: 40}},
		{ID: "b"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"iconKind":"figure"`, `"iconPath":"M0,0H1"`, `"mediaWidth":30`, `"name":"iconFigure"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
	// A graph with no figure has no mark of a figure.
	if spec, err = Compile(&Graph{Nodes: []*Node{{ID: "a", Media: &Media{Icon: "x:y"}}}}); err != nil {
		t.Fatal(err)
	}
	if b, _ = json.Marshal(spec); strings.Contains(string(b), "iconFigure") {
		t.Error("expected no mark of a figure")
	}
}

func TestCompileNoLabelAndWeight(t *testing.T) {
	t.Parallel()
	// A node with NoLabel draws no text, where another node with an empty
	// label draws its id. An edge of the weight 0 keeps it, and an edge
	// with no weight writes none.
	spec, err := Compile(&Graph{
		Nodes: []*Node{{ID: "a", NoLabel: true}, {ID: "b"}},
		Edges: []*Edge{{Source: "a", Target: "b", Weight: new(0.0)}, {Source: "b", Target: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"key":"a","label":""`, `"key":"b","label":"b"`, `"weight":0`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
	if n := strings.Count(s, `"weight":`); n != 1 {
		t.Errorf("expected one weight in the spec, got %d", n)
	}
}

// TestCompileForELK checks that Compile writes what only the engine of ELK
// reads: the order of the nodes and the clusters, the box of a node, and
// the algorithm of a cluster.
func TestCompileForELK(t *testing.T) {
	t.Parallel()
	spec, err := Compile(&Graph{
		Nodes:    []*Node{{ID: "a", Boxed: true}, {ID: "b", Cluster: "k"}},
		Clusters: []*Cluster{{ID: "k", Algorithm: "elk.box"}},
		Order:    []string{"b", "k", "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"order":["b","k","a"]`, `"boxed":true`, `"algorithm":"elk.box"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
	if n := strings.Count(s, `"boxed":`); n != 1 {
		t.Errorf("expected one box in the spec, got %d", n)
	}
}

// TestCompileColorLines checks that an edge of more than one color writes
// its colors, and that the spec draws the lines of the other colors.
func TestCompileColorLines(t *testing.T) {
	t.Parallel()
	spec, err := Compile(&Graph{
		Nodes: []*Node{{ID: "a"}, {ID: "b"}},
		Edges: []*Edge{{Source: "a", Target: "b", Strokes: []string{"red", "blue"}, Weights: []float64{0.3, -1}, Style: map[string]string{"stroke": "blue"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"strokes":["red","blue"]`, `"weights":[0.3,-1]`, `"_linePaint":"stroke"`, `"name":"layoutEdgeLines"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
}

func TestBusinessEllipse(t *testing.T) {
	t.Parallel()
	if got, ok := ShapeOf(businessEllipse); !ok || got != businessEllipse {
		t.Fatalf("ShapeOf(%q) = %q, %v", businessEllipse, got, ok)
	}
	w, h := NodeSize(businessEllipse, 80, 20, 8)
	if ew, eh := NodeSize("ellipse", 80, 20, 8); w != ew || h != eh {
		t.Errorf("expected the size of an ellipse, %gx%g, got %gx%g", ew, eh, w, h)
	}
	p := ShapePath(businessEllipse, 100, 40)
	if e := ShapePath("ellipse", 100, 40); !strings.HasPrefix(p, e) || p == e {
		t.Errorf("expected an ellipse and a line, got %s", p)
	}
	x, _ := clip(NodePlace{Width: 100, Height: 40}, businessEllipse, 100, 0)
	if x != 50 {
		t.Errorf("expected an edge to end at the tip of the ellipse, got %g", x)
	}
}

func TestShiftDash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dash []float64
		off  float64
		exp  []float64
	}{
		{[]float64{9, 5}, 0, []float64{9, 5}},
		{[]float64{9, 5}, 4, []float64{5, 5, 4, 0}},
		{[]float64{9, 5}, 900, []float64{5, 5, 4, 0}},
		{[]float64{9, 5}, -10, []float64{5, 5, 4, 0}},
		// In a gap, the dash starts at the next dash.
		{[]float64{9, 5}, 11, []float64{9, 5}},
		{[]float64{4, 2, 1, 3}, 5, []float64{1, 3, 4, 2}},
		// A dash of an odd length repeats, as in SVG.
		{[]float64{3}, 1, []float64{2, 3, 1, 0}},
		{[]float64{9}, 900, []float64{9}},
	}
	for _, test := range tests {
		var in []any
		for _, v := range test.dash {
			in = append(in, v)
		}
		var got []float64
		for _, v := range shiftDash(in, test.off) {
			f, _ := v.(float64)
			got = append(got, f)
		}
		if !slices.Equal(got, test.exp) {
			t.Errorf("%v at %v: expected %v, got %v", test.dash, test.off, test.exp, got)
		}
	}
}

func TestStyleFontStyle(t *testing.T) {
	t.Parallel()
	r := map[string]any{}
	style(r, map[string]string{"fontStyle": "italic", "strokeDash": "9,5", "strokeDashOffset": "4"})
	if r["fontStyle"] != "italic" {
		t.Errorf("expected the style italic, got %v", r["fontStyle"])
	}
	if d, _ := r["strokeDash"].([]any); len(d) != 4 || d[0] != 5.0 {
		t.Errorf("expected the dash 5,5,4,0, got %v", r["strokeDash"])
	}
}

// TestCompileBackground writes the background of a graph in the config of
// the spec as zuzu.background (D37), and a graph with no background writes
// no config.
func TestCompileBackground(t *testing.T) {
	t.Parallel()
	spec, err := Compile(&Graph{Nodes: []*Node{{ID: "a"}}, Background: "#1e1e2e"})
	if err != nil {
		t.Fatal(err)
	}
	if v := string(spec.Config[theme.ConfigBackground]); v != `"#1e1e2e"` || len(spec.Config) != 1 {
		t.Errorf("expected the config zuzu.background #1e1e2e, got %v", spec.Config)
	}
	if spec, err = Compile(&Graph{Nodes: []*Node{{ID: "a"}}}); err != nil || spec.Config != nil {
		t.Errorf("expected no config, got %v %v", spec.Config, err)
	}
}

// TestNoteStyle checks that a node that is a note draws with the style of a
// note of the theme, and a node that is not draws without it (D54).
func TestNoteStyle(t *testing.T) {
	t.Parallel()
	g := &Graph{Nodes: []*Node{{ID: "a", Label: "a"}, {ID: "n", Label: "n", Shape: "note", Note: true}}}
	spec, err := Compile(g)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(b), `"`+theme.StyleNote+`"`); got != 1 {
		t.Errorf("the style %s is on %d marks, want 1", theme.StyleNote, got)
	}
}
