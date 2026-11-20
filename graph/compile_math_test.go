package graph

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

// TestCompileMath checks the spec of a graph with math (D36): the
// parameters of the layout, and a mark of text and a mark of type path
// with the role math for each part with a label that draws math. A graph
// with no math, or whose labels with math all draw their source, has
// none of them.
func TestCompileMath(t *testing.T) {
	t.Parallel()
	g := &Graph{
		Math: true, MathSource: []string{"$$bad$$", "$$bad$$"},
		Nodes: []*Node{
			{ID: "a", Label: "Area $$x^2$$"},
			{ID: "b", Fields: []Field{{Text: "plain\n+int $$y$$ n"}}},
		},
		Edges:    []*Edge{{Source: "a", Target: "b", Label: "$$bad$$"}},
		Clusters: []*Cluster{{ID: "c", Label: "Set $$S$$"}},
	}
	spec, err := Compile(g)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"math":true`, `"mathSource":["$$bad$$"]`, `"role":"math"`,
		`datum.kind === \"mathPath\" && datum.part === \"node\"`,
		`datum.kind === \"mathText\" && datum.part === \"cluster\"`,
		`"text":{"field":"tex"}`, `"stroke":{"value":null}`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
	// The label of the edge draws its source, so no mark draws math of an
	// edge.
	if strings.Contains(s, `datum.part === \"edge\"`) {
		t.Error("expected no mark of math of an edge")
	}
	for _, g := range []*Graph{
		{Nodes: []*Node{{ID: "a", Label: "Area $$x^2$$"}}},
		{Math: true, MathSource: []string{"$$x$$"}, Nodes: []*Node{{ID: "a", Label: "$$x$$"}}},
	} {
		spec, err := Compile(g)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"role":"math"`) || strings.Contains(string(b), "mathText") {
			t.Errorf("expected no mark of math, got %s", b)
		}
	}
}
