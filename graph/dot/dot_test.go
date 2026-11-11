package dot

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestHTMLLabel(t *testing.T) {
	t.Parallel()
	src := `digraph {
	node [shape=none, margin=0]
	a [label=<<table border="0" cellborder="1"><tr><td bgcolor="lightblue"><b>authors</b></td></tr><tr><td port="id" href="x">id</td></tr></table>>]
	b [label=<<font color="nocolor">plain</font>>, margin="0.5,0.25"]
	a:id:e -> b:w [label=<<b>fk</b>>]
	subgraph cluster_x { label=<Box>; b }
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	a, b := g.Nodes[0], g.Nodes[1]
	if !strings.HasPrefix(a.HTML, "<table") || a.Label != "authors\nid" || len(a.Margin) != 2 || a.Margin[0] != 0 {
		t.Errorf("node a: %q %q %v", a.HTML, a.Label, a.Margin)
	}
	if b.Label != "plain" || !slices.Equal(b.Margin, []float64{36, 18}) {
		t.Errorf("node b: %q %v", b.Label, b.Margin)
	}
	e := g.Edges[0]
	if e.SourcePort != "id:e" || e.TargetPort != "w" || e.Label != "fk" {
		t.Errorf("edge: %q %q %q", e.SourcePort, e.TargetPort, e.Label)
	}
	if g.Clusters[0].Label != "Box" {
		t.Errorf("cluster: %q", g.Clusters[0].Label)
	}
	// The href of a cell, the unknown color, and the font of the label of
	// the edge, which draws as plain text.
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Message)
	}
	if len(report) != 3 {
		t.Errorf("expected 3 issues, got %q", msgs)
	}
	if _, _, err := Parse(src, true); err == nil {
		t.Error("expected an error in the strict mode")
	}
}

func TestHTMLSyntax(t *testing.T) {
	t.Parallel()
	_, _, err := Parse(`digraph { a [label=<<table><tr><td>x</tr></table>>] }`, false)
	if !errors.Is(err, ErrSyntax) || !strings.Contains(err.Error(), "at 1:") {
		t.Errorf("expected ErrSyntax with a place, got %v", err)
	}
}
