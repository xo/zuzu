package mermaid

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// edges writes the edges of a flowchart as source>target:label:head:tail:
// minlen, one by line.
func edgesOf(t *testing.T, src string) string {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, _, err := Graph(d, false)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var b strings.Builder
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "%s>%s:%s:%s:%s:%d\n", e.Source, e.Target, e.Label, e.Head, e.Tail, e.MinLen)
	}
	return b.String()
}

func TestLinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, exp string
	}{
		{"A-->B", "A>B::normal:none:1\n"},
		{"A --> B", "A>B::normal:none:1\n"},
		{"A --- B", "A>B::none:none:1\n"},
		{"A ---> B", "A>B::normal:none:2\n"},
		{"A ----- B", "A>B::none:none:3\n"},
		{"A -->|go| B", "A>B:go:normal:none:1\n"},
		{"A -- go --> B", "A>B:go:normal:none:1\n"},
		{"A -- go two --- B", "A>B:go two:none:none:1\n"},
		{"A -.-> B", "A>B::normal:none:1\n"},
		{"A -..-> B", "A>B::normal:none:2\n"},
		{"A -. go .-> B", "A>B:go:normal:none:1\n"},
		{"A ==> B", "A>B::normal:none:1\n"},
		{"A == go ==> B", "A>B:go:normal:none:1\n"},
		{"A <--> B", "A>B::normal:normal:1\n"},
		{"A --o B", "A>B::circle:none:1\n"},
		{"A --x B", "A>B::cross:none:1\n"},
		{"A o--o B", "A>B::circle:circle:1\n"},
		{"A ~~~ B", "A>B::none:none:1\n"},
		{"A --> B --> C", "A>B::normal:none:1\nB>C::normal:none:1\n"},
		{"A & B --> C & D", "A>C::normal:none:1\nA>D::normal:none:1\nB>C::normal:none:1\nB>D::normal:none:1\n"},
		{"A[x] -->|y| B(z) --> C{w}", "A>B:y:normal:none:1\nB>C::normal:none:1\n"},
		{"node-1 --> node-2", "node-1>node-2::normal:none:1\n"},
		{"A e1@--> B", "A>B::normal:none:1\n"},
		{"A-->B;B-->C", "A>B::normal:none:1\nB>C::normal:none:1\n"},
		{`A["a --> b"] --> B`, "A>B::normal:none:1\n"},
		{"A --> B[\"x;y\"]", "A>B::normal:none:1\n"},
		{"Ab --> oB", "Ab>oB::normal:none:1\n"},
		{"A --> xyz", "A>xyz::normal:none:1\n"},
		// The dotted links: a dash, if any, dots, then a dash.
		{"A .-> B", "A>B::normal:none:1\n"},
		{"A <.-> B", "A>B::normal:normal:1\n"},
		{"A .- B", "A>B::none:none:1\n"},
		{"A ..-> B", "A>B::normal:none:2\n"},
		{"A o.-o B", "A>B::circle:circle:1\n"},
		{"A -. go -.-> B", "A>B:go:normal:none:1\n"},
		{"A -. go ..- B", "A>B:go:none:none:2\n"},
		// The mark before a link counts only when it is its head.
		{"A <.- B", "A>B::none:none:1\n"},
		{"A <--- B", "A>B::none:none:2\n"},
		{"A o--> B", "A>B::normal:none:2\n"},
		{"A <-- go --> B", "A>B:go:normal:normal:1\n"},
		{"A <-- go --- B", "A>B:go:normal:none:1\n"},
		// The end of a link with text can touch the node after it.
		{"A-- This is the text! ---B", "A>B:This is the text!:none:none:1\n"},
		{"A-- go -->B", "A>B:go:normal:none:1\n"},
		{"A-- go --oB", "A>B:go:circle:none:1\n"},
		{`id1(Start) link@-- "go" -->id2(Stop)`, "id1>id2:go:normal:none:1\n"},
		// A semicolon in the text of a link, or of an entity, does not
		// end a statement.
		{"A-->|x;y|B", "A>B:x;y:normal:none:1\n"},
		{"A-- a #35; b -->B", "A>B:a # b:normal:none:1\n"},
		// A chain goes on over a line break before or after a link.
		{"A\n--> B", "A>B::normal:none:1\n"},
		{"A -->\nB", "A>B::normal:none:1\n"},
		{"A\n-- go --> B", "A>B:go:normal:none:1\n"},
		{"A --> B\n--> C", "A>B::normal:none:1\nB>C::normal:none:1\n"},
		{"A --> B\nC\n--> D", "A>B::normal:none:1\nC>D::normal:none:1\n"},
		{"A & B\n--> C", "A>C::normal:none:1\nB>C::normal:none:1\n"},
		{"A -->\nB -->\nC", "A>B::normal:none:1\nB>C::normal:none:1\n"},
	}
	for _, test := range tests {
		t.Run(test.src, func(t *testing.T) {
			t.Parallel()
			if s := edgesOf(t, "flowchart TD\n"+test.src); s != test.exp {
				t.Errorf("expected\n%s, got\n%s", test.exp, s)
			}
		})
	}
}

func TestShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, shape, label string
	}{
		{"A", "rect", "A"},
		{"A[One]", "rect", "One"},
		{"A(One)", "rounded", "One"},
		{"A([One])", "stadium", "One"},
		{"A[[One]]", "subroutine", "One"},
		{"A[(One)]", "cylinder", "One"},
		{"A((One))", "circle", "One"},
		{"A(((One)))", "doublecircle", "One"},
		{"A>One]", "odd", "One"},
		{"A{One}", "diamond", "One"},
		{"A{{One}}", "hexagon", "One"},
		{"A[/One/]", "lean-r", "One"},
		{`A[\One\]`, "lean-l", "One"},
		{`A[/One\]`, "trap-b", "One"},
		{`A[\One/]`, "trap-t", "One"},
		{`A["One (two)"]`, "rect", "One (two)"},
		{"A[One<br>two]", "rect", "One\ntwo"},
		{"A[#quot;q#quot;]", "rect", `"q"`},
		{"A[\"`**bold** text`\"]", "rect", "bold text"},
		{`A@{ shape: cyl, label: "Store" }`, "cyl", "Store"},
		{"A@{ shape: diam }", "diam", "A"},
	}
	for _, test := range tests {
		t.Run(test.src, func(t *testing.T) {
			t.Parallel()
			d, err := Read("flowchart LR\n" + test.src)
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := Graph(d, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Nodes) != 1 {
				t.Fatalf("expected 1 node, got %d", len(g.Nodes))
			}
			if n := g.Nodes[0]; n.Shape != test.shape || n.Label != test.label {
				t.Errorf("expected %s %q, got %s %q", test.shape, test.label, n.Shape, n.Label)
			}
		})
	}
}

func TestNamedShapes(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  a@{ shape: lined-document, label: "Doc" }
  b@{ shape: comment, label: "Note" }
  c@{ shape: braces }
  d@{ shape: fork, label: "hidden" }
  e@{ shape: hourglass }
  f@{ shape: fr-circ }
  h@{ shape: choice, label: "hidden" }
  i@{ shape: anchor }`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("expected no issues, got %v", report)
	}
	var got []string
	for _, n := range g.Nodes {
		got = append(got, fmt.Sprintf("%s %s %q %vx%v", n.ID, n.Shape, n.Label, n.Width, n.Height))
	}
	// A fork, an hourglass, a framed circle, a choice and an anchor draw
	// no text, at the size of Mermaid. A fork across a flowchart LR stands
	// up.
	want := `a lined-document "Doc" 0x0|b comment "Note" 0x0|c braces "c" 0x0|d fork "" 10x70|e hourglass "" 30x30|f fr-circ "" 14x14|` +
		`h choice "" 28x28|i anchor "" 2x2`
	if s := strings.Join(got, "|"); s != want {
		t.Errorf("got  %s\nwant %s", s, want)
	}
}

func TestFlowchart(t *testing.T) {
	t.Parallel()
	src := `---
title: T
config:
  look: neo
---
%%{init: {'flowchart': {'curve': 'linear'}}}%%
graph RL
  subgraph one [The one]
    a --> b
    subgraph two
      c
    end
  end
  b --> c:::hot
  classDef hot fill:#f00,color:#fff
  classDef default stroke:#00f
  style a stroke-dasharray: 5 5
  linkStyle default stroke:#0f0
  linkStyle 1 stroke:#f0f
  click a callback
`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "T" || g.Direction != "RL" {
		t.Errorf("title %q direction %q", g.Title, g.Direction)
	}
	if len(g.Clusters) != 2 || g.Clusters[0].Label != "The one" || g.Clusters[1].Parent != "one" {
		t.Errorf("clusters %+v", g.Clusters)
	}
	nodes := map[string]string{}
	for _, n := range g.Nodes {
		nodes[n.ID] = fmt.Sprintf("%s|%s|%s|%s", n.Cluster, n.Style["fill"], n.Style["stroke"], n.Style["strokeDash"])
	}
	exp := map[string]string{"a": "one||#00f|5,5", "b": "one||#00f|", "c": "two|#f00|#00f|"}
	for k, v := range exp {
		if nodes[k] != v {
			t.Errorf("node %s: expected %s, got %s", k, v, nodes[k])
		}
	}
	if g.Edges[0].Style["stroke"] != "#0f0" || g.Edges[1].Style["stroke"] != "#f0f" || g.Edges[0].Curve != "linear" {
		t.Errorf("edges %+v %+v", g.Edges[0], g.Edges[1])
	}
	if len(report) != 1 {
		t.Errorf("expected 1 entry in the report, got %v", report)
	}
	if _, _, err := Graph(d, true); err == nil {
		t.Error("expected an error in the strict mode")
	}
}

func TestSubgraphs(t *testing.T) {
	t.Parallel()
	src := `flowchart TB
  subgraph kept [Kept]
    direction LR
    a --> b
  end
  subgraph lost [Lost]
    direction rl
    c --> d
  end
  x --> kept
  x --> c
`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Clusters[0].Direction != "LR" || g.Clusters[1].Direction != "RL" {
		t.Errorf("expected the directions LR and RL, got %q and %q", g.Clusters[0].Direction, g.Clusters[1].Direction)
	}
	for _, n := range g.Nodes {
		if n.ID == "kept" {
			t.Error("the id of a subgraph that an edge names is a node")
		}
	}
	if e := g.Edges[2]; e.Source != "x" || e.Target != "kept" {
		t.Errorf("expected an edge from x to the subgraph kept, got %s to %s", e.Source, e.Target)
	}
	// A direction that an edge crosses draws as Mermaid draws it, so it
	// is not in the report.
	if len(report) != 0 {
		t.Errorf("expected an empty report, got %v", report)
	}
	if _, _, err := Graph(d, true); err != nil {
		t.Errorf("expected no error in the strict mode, got %v", err)
	}
}

func TestLinkAnimation(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  A e1@--> B
  B e2@-.-> C
  C e3@--> D
  D e4@--> E
  E e5@--> F
  F e6@--> G
  G --> H
  e1@{ animate: true }
  e2@{ animation: slow }
  e3@{ animation: fast, animate: false }
  e4@{ animation: spin }
  classDef anim stroke-dasharray: 2 2,animation: dash 25s linear infinite
  classDef moves animation: dash 25s linear infinite
  class e5 anim
  class e6 moves
  class e1 anim
  linkStyle 6 animation: dash 9s linear infinite`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	// An animated link takes the dash of Mermaid over its own. The style
	// animation keeps the dash of the style, and adds none.
	var dashes []string
	for _, e := range g.Edges {
		if _, ok := e.Style["animation"]; ok {
			t.Errorf("the style animation stays on the link %s", e.ID)
		}
		dashes = append(dashes, e.Style["strokeDash"])
	}
	if s := strings.Join(dashes, "|"); s != "9,5|9,5|9,5||2,2||" {
		t.Errorf("expected the dashes 9,5|9,5|9,5||2,2||, got %s", s)
	}
	var issues []string
	for _, i := range report {
		issues = append(issues, i.Path+": "+i.Message)
	}
	if s := strings.Join(issues, "|"); s != "line 12: the animation spin of a link" {
		t.Errorf("unexpected issues %s", s)
	}
	// The style animation on a node is an issue, at the line of its
	// classDef, and the class default is on every node.
	for _, src := range []string{
		"flowchart LR\n  A --> B\n  classDef moves animation: dash 25s linear infinite\n  class A moves",
		"flowchart LR\n  A --> B\n  classDef default animation: dash 25s linear infinite",
		"flowchart LR\n  A --> B\n  style A animation: dash 25s linear infinite",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(report) != 1 || report[0].Message != "the style animation" || report[0].Path != "line 3" {
			t.Errorf("%q: expected the issue of the style animation at line 3, got %v", src, report)
		}
		for _, n := range g.Nodes {
			if _, ok := n.Style["animation"]; ok {
				t.Errorf("%q: the style animation stays on the node %s", src, n.ID)
			}
		}
	}
}

func TestLinkProps(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  A e1@--> B
  A e2@--> C
  A e3@--> D
  e1@{ curve: linear }
  e2@{ curve: natural }
  linkStyle 1 interpolate step
  e3@{ curve: bumpY, animate: true }
  classDef dash stroke-dasharray: 9 5,stroke-dashoffset: 4
  class e1 dash`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	// The id of a link is no node.
	if len(g.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(g.Nodes))
	}
	// A later curve wins, from a property or from a linkStyle.
	var curves []string
	for _, e := range g.Edges {
		curves = append(curves, e.Curve)
	}
	if s := strings.Join(curves, ","); s != "linear,step," {
		t.Errorf("expected the curves linear,step and none, got %s", s)
	}
	if st := g.Edges[0].Style; st["strokeDash"] != "9,5" || st["strokeDashOffset"] != "4" {
		t.Errorf("expected the class on the link e1, got %v", st)
	}
	// animate draws the link dashed, as one still frame of Mermaid.
	if st := g.Edges[2].Style; st["strokeDash"] != animatedDash {
		t.Errorf("expected the link e3 dashed, got %v", st)
	}
	var issues []string
	for _, i := range report {
		issues = append(issues, i.Message)
	}
	if s := strings.Join(issues, "|"); s != "the curve bumpY" {
		t.Errorf("unexpected issues %s", s)
	}
	// The id of a link in a group names the edge from the last node of
	// the group before it to the first node after it. The other edges
	// take the ids of Mermaid.
	d, err = Read(`flowchart LR
  A & B e1@--> C & D
  A --> C
  L_A_C_0@{ curve: linear }
  L_A_C_2@{ curve: step }
  e1@{ curve: natural }`)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err = Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	curves = nil
	for _, e := range g.Edges {
		curves = append(curves, e.Source+e.Target+":"+e.Curve)
	}
	if s := strings.Join(curves, ","); s != "AC:linear,AD:,BC:natural,BD:,AC:step" || len(g.Nodes) != 4 {
		t.Errorf("unexpected curves %s, or nodes %d", s, len(g.Nodes))
	}
}

func TestCollapsed(t *testing.T) {
	t.Parallel()
	src := `flowchart TD
  Start --> inner
  Start --> B
  subgraph outer [Outer]
    A --> B
    subgraph inner [Inner]
      C
    end
    B --> C
  end
  C --> End
  B --> End
  outer@{ view: collapsed }
  inner@{ view: collapsed }
  x@{ view: collapsed }`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatal(err)
	}
	var nodes []string
	for _, n := range g.Nodes {
		nodes = append(nodes, fmt.Sprintf("%s %s %q", n.ID, n.Shape, n.Label))
	}
	if s := strings.Join(nodes, "|"); s != `Start rect "Start"|End rect "End"|outer collapsedGroup "Outer"|x rect "x"` {
		t.Errorf("unexpected nodes %s", s)
	}
	if len(g.Clusters) != 0 {
		t.Errorf("expected no clusters, got %d", len(g.Clusters))
	}
	// An edge into the collapsed subgraph goes to its node, and an edge
	// inside it does not draw.
	var edges []string
	for _, e := range g.Edges {
		edges = append(edges, e.Source+">"+e.Target)
	}
	if s := strings.Join(edges, ","); s != "Start>outer,Start>outer,outer>End,outer>End" {
		t.Errorf("unexpected edges %s", s)
	}
	if len(report) != 1 || report[0].Message != "the view collapsed of x, which is not a subgraph" {
		t.Errorf("unexpected report %v", report)
	}
}

func TestStyles(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  A --> B
  style A font-style:italic,border:1px solid red,background:#bbb
  style B font-style:oblique`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("expected no issues, got %v", report)
	}
	for _, n := range g.Nodes {
		if n.Style["fontStyle"] != "italic" || n.Style["border"] != "" || n.Style["background"] != "" {
			t.Errorf("node %s: unexpected style %v", n.ID, n.Style)
		}
	}
	d, _ = Read("flowchart LR\nA\nstyle A font-style:wavy")
	if _, _, err := Graph(d, true); err == nil {
		t.Error("expected an error for the font-style wavy in the strict mode")
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"flowchart XX\nA",
		"flowchart TD\nsubgraph a\nA",
		"flowchart TD\nend",
		"flowchart TD\nA --> ",
		"flowchart TD\nA --> B\nlinkStyle 3 stroke:#f00",
		"flowchart TD\nsubgraph a\ndirection XY\nA\nend",
		"flowchart TD\nsubgraph a\nend\nB --> a",
		"flowchart TD\nA -->;B",
		"flowchart TD\nA -->\nend",
		"flowchart TD\nA -. go --> B",
		"flowchart TD\naccDescr {\nnever ends\nA",
	} {
		d, err := Read(src)
		if err == nil {
			_, _, err = Graph(d, false)
		}
		if !errors.Is(err, ErrSyntax) {
			t.Errorf("%q: expected ErrSyntax, got %v", src, err)
		}
	}
	d, _ := Read("sequenceDiagram\nA->>B: hi")
	if _, _, err := Graph(d, false); !errors.Is(err, ErrKind) {
		t.Errorf("expected ErrKind, got %v", err)
	}
}

func TestLinkKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, dash string
	}{
		{"A .-> B", "3,3"},
		{"A -. go -.-> B", "3,3"},
		{"A --> B", ""},
		// Mermaid draws a link with text whose marks at its ends do not
		// agree as -->.
		{"A <-. go .- B", ""},
	}
	for _, test := range tests {
		d, err := Read("flowchart LR\n" + test.src)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := Graph(d, true)
		if err != nil {
			t.Fatalf("%q: %v", test.src, err)
		}
		if dash := g.Edges[0].Style["strokeDash"]; dash != test.dash {
			t.Errorf("%q: the dash %q, expected %q", test.src, dash, test.dash)
		}
	}
}

func TestFlowchartAccessibility(t *testing.T) {
	t.Parallel()
	src := "graph LR\n  accTitle: Big Decisions\n  accDescr {\n    The process\n    of a decision\n  }\n  accDescr: in short\n  A --> B"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	if want := "Big Decisions The process of a decision in short"; g.Description != want {
		t.Errorf("the description %q, expected %q", g.Description, want)
	}
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Errorf("%d nodes and %d edges, expected 2 and 1", len(g.Nodes), len(g.Edges))
	}
}

func TestHeaderStatements(t *testing.T) {
	t.Parallel()
	d, err := Read(`graph TB;subgraph "number as labels";1;end;`)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.Direction != "TB" || len(g.Nodes) != 1 || g.Nodes[0].ID != "1" || len(g.Clusters) != 1 ||
		g.Clusters[0].Label != "number as labels" || g.Nodes[0].Cluster != g.Clusters[0].ID {
		t.Errorf("the direction %s, the nodes %v and the clusters %v", g.Direction, g.Nodes, g.Clusters)
	}
}

func TestEdgeClass(t *testing.T) {
	t.Parallel()
	src := "graph LR\n  id1 link@-- go --> id2\n  classDef c stroke:#f66,color:white\n  class link c\n  linkStyle 0 stroke:#0f0"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 {
		t.Errorf("%d nodes, expected 2", len(g.Nodes))
	}
	// linkStyle goes over the class, as in Mermaid.
	if st := g.Edges[0].Style; st["stroke"] != "#0f0" || st["color"] != "white" {
		t.Errorf("the style of the edge %v", st)
	}
}
