package graph

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"
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
