package dot

import (
	"testing"

	"github.com/xo/zuzu/graph"
)

// TestFontName checks that fontname gives a weight and a style, and that
// zuzu accepts the rest of the name with no issue (D32).
func TestFontName(t *testing.T) {
	t.Parallel()
	src := `digraph {
	fontname="Times-Italic"
	label="T"
	node [fontname="Helvetica-Bold"]
	a
	b [fontname="Courier"]
	c [label=<{<font face="Times-BoldItalic">x</font>}>, shape=record]
	a -> b [fontname="Helvetica-Oblique", labelfontname="Arial Light", headlabel="h"]
	subgraph cluster_x { fontname="Arial Light"; d }
}`
	g, report, err := Parse(src, true)
	if err != nil {
		t.Fatalf("the strict mode: %v", err)
	}
	if len(report) != 0 {
		t.Errorf("the report %v, want none", report)
	}
	nodes := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	if w := nodes["a"].Style["fontWeight"]; w != graph.BoldWeight {
		t.Errorf("a has the weight %q, want %q", w, graph.BoldWeight)
	}
	// b names its own font, which gives no weight, so it keeps the weight
	// of the default node, as Graphviz replaces the whole name.
	if w := nodes["b"].Style["fontWeight"]; w != "" {
		t.Errorf("b has the weight %q, want none", w)
	}
	if f := nodes["c"].Fields; len(f) != 1 || len(f[0].Fonts) != 1 || !f[0].Fonts[0].Bold || !f[0].Fonts[0].Italic {
		t.Errorf("the fields of c %+v, want one bold italic line", f)
	}
	e := g.Edges[0]
	if e.Style["fontStyle"] != "italic" || e.LabelStyle["fontWeight"] != "300" {
		t.Errorf("the edge has the style %v and the label style %v, want italic and the weight 300", e.Style, e.LabelStyle)
	}
	if g.TitleStyle["fontStyle"] != "italic" {
		t.Errorf("the title has the style %v, want italic", g.TitleStyle)
	}
	if w := g.Clusters[0].Style["fontWeight"]; w != "300" {
		t.Errorf("the cluster has the weight %q, want 300", w)
	}
}
