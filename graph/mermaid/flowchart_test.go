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
	// Only the direction that an edge crosses is in the report.
	if len(report) != 1 || !strings.Contains(report[0].Message, `"Lost"`) || report[0].Path != "subgraph lost" {
		t.Errorf("expected one entry for the subgraph lost, got %v", report)
	}
	if _, _, err := Graph(d, true); err == nil {
		t.Error("expected an error in the strict mode")
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
