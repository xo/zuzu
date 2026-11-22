package graph

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

// TestCompileCategory checks that a node and a cluster with a category
// draw in the color of the ordinal scale of the category scheme, with a
// fill or a stroke of the style over it, and that a graph with no
// category has no scale (D37).
func TestCompileCategory(t *testing.T) {
	t.Parallel()
	spec, err := Compile(&Graph{
		Nodes: []*Node{
			{ID: "a", Category: 1},
			{ID: "b", Category: 3, Style: map[string]string{"stroke": "#000"}},
			{ID: "c", Cluster: "s"},
		},
		Clusters: []*Cluster{{ID: "s", Category: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"scales":[{"name":"zuzu.category","type":"ordinal","domain":[0,1,2],"range":"category"},` +
			`{"name":"zuzu.categoryTint","type":"ordinal","domain":[0,1,2],"range":"zuzu.tint"}]`,
		`"fill":{"field":"category","scale":"zuzu.categoryTint"}`,
		`"stroke":{"field":"category","scale":"zuzu.category"}`,
		`"_paint":"category,stroke"`, `"stroke":{"field":"stroke"}`,
		// A cluster draws as a node of its color: a solid tint and an
		// outline (D55).
		`"category":1,"key":"s"`, `"strokeWidth":1.25}`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
	if spec, err = Compile(&Graph{Nodes: []*Node{{ID: "a"}}, Clusters: []*Cluster{{ID: "s"}}}); err != nil {
		t.Fatal(err)
	}
	if b, err = json.Marshal(spec); err != nil {
		t.Fatal(err)
	}
	if s := string(b); strings.Contains(s, "scales") || strings.Contains(s, "category") {
		t.Errorf("expected no scale and no category, got %s", s)
	}
}
