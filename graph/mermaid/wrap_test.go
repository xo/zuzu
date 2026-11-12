package mermaid

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// wrapsOf reads a diagram and writes the width at which each label wraps,
// in the pixels of Mermaid: the nodes as id=wrap, and the edges as
// source>target=wrap, in order, and a b when the words break.
func wrapsOf(t *testing.T, src string) string {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return wraps(g)
}

func wraps(g *graph.Graph) string {
	var parts []string
	// zuzu scales each width of Mermaid by 11/14.
	px := func(w float64) float64 { return math.Round(w*14/11*1e6) / 1e6 }
	for _, n := range g.Nodes {
		parts = append(parts, fmt.Sprintf("%s=%g", n.ID, px(n.Wrap)))
	}
	for _, e := range g.Edges {
		parts = append(parts, fmt.Sprintf("%s>%s=%g", e.Source, e.Target, px(e.Wrap)))
	}
	if g.BreakWords {
		parts = append(parts, "b")
	}
	return strings.Join(parts, " ")
}

func TestWrap(t *testing.T) {
	t.Parallel()
	md := "flowchart LR\n  A[\"`a **b**`\"] -->|\"`c`\"| B[plain] -- d --> C"
	tests := []struct {
		name, src, exp string
	}{
		{"default", "flowchart LR\n  A[a] -->|x| B", "A=120 B=120 A>B=200"},
		{"front matter", "---\nconfig:\n  flowchart:\n    wrappingWidth: 300\n---\nflowchart LR\n  A --> B", "A=300 B=300 A>B=200"},
		{"directive", "%%{init: {'flowchart': {'wrappingWidth': 60}}}%%\nflowchart LR\n  A --> B", "A=60 B=60 A>B=200"},
		{"markdown", md, "A=120 B=120 C=120 A>B=200 B>C=200"},
		{"no markdown wrap", "---\nconfig:\n  markdownAutoWrap: false\n---\n" + md, "A=0 B=120 C=120 A>B=0 B>C=200"},
		{"svg labels", "---\nconfig:\n  htmlLabels: false\n  markdownAutoWrap: false\n---\n" + md, "A=120 B=120 C=120 A>B=200 B>C=200 b"},
		{"property label", "flowchart LR\n  A@{ shape: rect, label: \"`a b`\" }", "A=120"},
		{"state", "stateDiagram-v2\n  [*] --> A : go\n  A : a long state\n  note right of A : a note", "root_start=120 A=120 A----note=120 root_start>A=200 A>A----note=200"},
		{"state width", "---\nconfig:\n  state:\n    wrappingWidth: 80\n---\nstateDiagram-v2\n  A --> B", "A=80 B=80 A>B=200"},
		{"class", "classDiagram\n  A <|-- B : a label\n  note for A \"a note\"", "A=0 B=0 note0=0 A>B=200 note0>A=200"},
		{"er", "erDiagram\n  A ||--o{ B : has", "A=0 B=0 A>B=200"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := wrapsOf(t, test.src); got != test.exp {
				t.Errorf("expected %q, got %q", test.exp, got)
			}
		})
	}
}

func TestScaleWrap(t *testing.T) {
	t.Parallel()
	g, _, err := Graph(&Diagram{Kind: "flowchart", Header: "LR", Config: map[string]any{}, Lines: []Line{{Text: "A -->|x| B", N: 2}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	// A label of zuzu is 11 pixels and one of Mermaid is 14.
	if n, e := g.Nodes[0].Wrap, g.Edges[0].Wrap; n != 120.0*11/14 || e != 200.0*11/14 {
		t.Errorf("expected the widths 120 and 200 scaled by 11/14, got %g and %g", n, e)
	}
}

func TestWrapIssues(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"---\nconfig:\n  flowchart:\n    wrappingWidth: wide\n---\nflowchart LR\n  A",
		"---\nconfig:\n  flowchart:\n    wrappingWidth: 0\n---\nflowchart LR\n  A",
		"---\nconfig:\n  htmlLabels: maybe\n---\nflowchart LR\n  A",
		"---\nconfig:\n  state:\n    wrappingWidth: -5\n---\nstateDiagram-v2\n  A",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if _, _, err := Graph(d, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported setting in the strict mode, got %v", src, err)
		}
		g, report, err := Graph(d, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%q: expected 1 line in the report, got %v, %v", src, report, err)
			continue
		}
		if g.Nodes[0].Wrap != scaleWrap(nodeWrap) {
			t.Errorf("%q: expected the default width, got %g", src, g.Nodes[0].Wrap)
		}
	}
}
