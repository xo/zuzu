package dot

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRankInherit(t *testing.T) {
	t.Parallel()
	// rank=same in the graph is the rank of each subgraph that starts after
	// it, as in dot. A subgraph before it and a subgraph that sets its own
	// rank keep theirs.
	src := `graph {
	newrank=true
	{a b}
	rank=same
	{c -- d}
	subgraph s { e; f }
	{rank=min; g; h}
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	rank := map[string]string{}
	for _, n := range g.Nodes {
		rank[n.ID] = n.Rank
	}
	if rank["a"] != "" || rank["b"] != "" {
		t.Errorf("a and b have the ranks %q and %q, want none", rank["a"], rank["b"])
	}
	if rank["c"] == "" || rank["c"] != rank["d"] || rank["e"] == "" || rank["e"] != rank["f"] || rank["c"] == rank["e"] {
		t.Errorf("the ranks %v, want c and d in one group, and e and f in another", rank)
	}
	if rank["g"] != "" {
		t.Errorf("g has the rank %q, want none", rank["g"])
	}
	if len(report) != 1 || !strings.Contains(report[0].Error(), "the rank min") {
		t.Errorf("expected one issue for rank=min, got %v", report)
	}
}

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
	if e.SourcePort != "id:e" || e.TargetPort != "w" || e.Label != "fk" || e.HTML != "<b>fk</b>" {
		t.Errorf("edge: %q %q %q %q", e.SourcePort, e.TargetPort, e.Label, e.HTML)
	}
	if c := g.Clusters[0]; c.Label != "Box" || c.HTML != "Box" {
		t.Errorf("cluster: %q %q", c.Label, c.HTML)
	}
	// The href of a cell and the unknown color.
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Message)
	}
	if len(report) != 2 {
		t.Errorf("expected 2 issues, got %q", msgs)
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

func TestHTMLGraphLabel(t *testing.T) {
	t.Parallel()
	g, report, err := Parse(`digraph { label=<<b>Title</b>>; subgraph cluster_a { label=<<i>A</i>>; x } }`, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "Title" || g.TitleHTML != "<b>Title</b>" {
		t.Errorf("title: %q %q", g.Title, g.TitleHTML)
	}
	if c := g.Clusters[0]; c.Label != "A" || c.HTML != "<i>A</i>" {
		t.Errorf("cluster: %q %q", c.Label, c.HTML)
	}
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
}

func TestEndLabels(t *testing.T) {
	t.Parallel()
	src := `digraph {
	a [xlabel="ext"]
	b [xlabel=<<b>B</b>>]
	a -> b [headlabel="1", taillabel=<<i>*</i>>, xlabel="x", labelfontcolor=red, labelfontsize=9]
	a -> c [headlabel="h", labelangle=10]
	b -> c [headlabel="h", labeldistance=0]
	c -> a [headlabel="h"]
}`
	g, report, err := Parse(src, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	a, b := g.Nodes[0], g.Nodes[1]
	if a.XLabel != "ext" || a.XHTML != "" || b.XLabel != "B" || b.XHTML != "<b>B</b>" {
		t.Errorf("xlabels: %q %q %q %q", a.XLabel, a.XHTML, b.XLabel, b.XHTML)
	}
	e := g.Edges[0]
	if e.HeadLabel != "1" || e.HeadHTML != "" || e.TailLabel != "*" || e.TailHTML != "<i>*</i>" || e.XLabel != "x" {
		t.Errorf("labels: %q %q %q %q %q", e.HeadLabel, e.HeadHTML, e.TailLabel, e.TailHTML, e.XLabel)
	}
	if e.LabelStyle["color"] != "red" || e.LabelStyle["fontSize"] != "9" {
		t.Errorf("label style: %v", e.LabelStyle)
	}
	// labelangle alone fixes the place, with the distance of dot, and
	// labeldistance=0 puts the label on the end. With neither, the layout
	// places the label.
	for i, want := range [][2]float64{{0, 0}, {1, 10}, {0.01, -25}, {0, 0}} {
		if e := g.Edges[i]; e.LabelDistance != want[0] || e.LabelAngle != want[1] {
			t.Errorf("edge %d: the distance %v and the angle %v, want %v", i, e.LabelDistance, e.LabelAngle, want)
		}
	}
}

func TestArrows(t *testing.T) {
	t.Parallel()
	src := `digraph {
	a -> b [arrowhead=crowodot, arrowsize=2]
	a -> c [dir=both, arrowtail=lteeoldiamond, arrowhead=invempty]
	a -> d [dir=back, arrowtail=obox]
	a -> e [arrowhead=nope]
	a -> f [arrowhead=normalnormalnormalnormalnormal]
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range g.Edges {
		got = append(got, e.Head+" "+e.Tail)
	}
	want := []string{"crowodot none", "invempty lteeoldiamond", "none obox", "nope none", "normalnormalnormalnormalnormal none"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if g.Edges[0].ArrowSize != 2 {
		t.Errorf("the arrow size %v", g.Edges[0].ArrowSize)
	}
	if len(report) != 2 {
		t.Errorf("expected 2 issues, for nope and the five shapes, got %v", report)
	}
	if _, _, err := Parse(src, true); err == nil {
		t.Error("expected an error in the strict mode")
	}
}

func TestPolygon(t *testing.T) {
	t.Parallel()
	src := `digraph {
	node [shape=polygon, sides=4, skew="0.0", orientation="0.0"]
	a [sides=7, skew=0.25, distortion=-0.5, orientation=30]
	b [regular=true]
	c [sides=2]
	d [sides=x]
	e [shape=Mdiamond]
	f [shape=box, skew=0.5, orientation=20]
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range g.Nodes {
		got = append(got, n.Shape)
	}
	want := []string{"polygon 7 0.25 -0.5 30", "polygon 4 0 0 0 regular", "ellipse", "polygon 4 0 0 0", "Mdiamond", "box"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	// The sides x, and the orientation of the box. A box draws without a
	// skew, as in Graphviz.
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Message)
	}
	if len(msgs) != 2 || msgs[0] != "the sides x" || msgs[1] != "the attribute orientation of a node of the shape box" {
		t.Errorf("got the issues %q", msgs)
	}
}
