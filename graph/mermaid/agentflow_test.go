package mermaid

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// agentOf reads an agentflow diagram with no issue, or fails.
func agentOf(t *testing.T, src string) *graph.Graph {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, report, err := Graph(d, true)
	if err != nil || len(report) > 0 {
		t.Fatalf("graph: %v %v", err, report)
	}
	return g
}

// clusterOf returns the cluster of an id, or fails.
func clusterOf(t *testing.T, g *graph.Graph, id string) *graph.Cluster {
	t.Helper()
	for _, c := range g.Clusters {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no cluster %s", id)
	return nil
}

func TestAgentShapes(t *testing.T) {
	t.Parallel()
	g := agentOf(t, `agentflow-beta LR
  a["A"]@{ shape: task }
  b["B"]@{ shape: tool, returns: "String", connectorRef: "x.y" }
  c@{ shape: input, value: "Stockholm", description: "a city" }
  d@{ shape: decision }
  e@{ shape: refdoc }
  f@{ shape: action }
  g@{ shape: cyl }
  h@{ shape: lined-document }
  i[[I]]
  j{J}
  k{{K}}
  l[/L/]
  m((M))
  n([N])
  o(O)
  p[P]
  q
  r@{ label: "Arr", shape: subproc }
  s@{ }
  t@{
  }`)
	if g.Direction != graph.LR {
		t.Errorf("expected LR, got %s", g.Direction)
	}
	for id, want := range map[string]string{
		"a": "rounded", "b": "subroutine", "c": "lean-r", "d": "diamond", "e": "lin-doc", "f": "hexagon",
		"g": "rounded", "h": "lin-doc", "i": "subroutine", "j": "diamond", "k": "hexagon", "l": "lean-r",
		"m": "rounded", "n": "rounded", "o": "rounded", "p": "rounded", "q": "rounded", "r": "subroutine",
		"s": "rounded", "t": "rounded",
	} {
		if n := node(t, g, id); n.Shape != want {
			t.Errorf("%s: expected the shape %s, got %s", id, want, n.Shape)
		}
	}
	if n := node(t, g, "r"); n.Label != "Arr" {
		t.Errorf("expected the label Arr, got %q", n.Label)
	}
	// The label of a node is at least agentflow.minNodeWidth wide, and a
	// diamond grows to fit its label.
	if n := node(t, g, "a"); n.MinWidth != agentWidth(agentMinWidth) {
		t.Errorf("expected the least width %v, got %v", agentWidth(agentMinWidth), n.MinWidth)
	}
	if n := node(t, g, "d"); n.MinWidth != 0 {
		t.Errorf("expected no least width on a diamond, got %v", n.MinWidth)
	}
}

func TestAgentLinks(t *testing.T) {
	t.Parallel()
	got := edgesOf(t, `agentflow-beta
  a --> b
  b --x c
  c -.- d
  a -- yes --> c
  a -- "no -- never" --x d
  b -->|low| d
  a ---> d
  a -..- b
  d -.-|ref| a
  a e1@--> c
  e1@{ animate: true, instruction: "go" }`)
	want := `a>b::normal:none:1
b>c::cross:none:1
c>d::none:none:1
a>c:yes:normal:none:1
a>d:no -- never:cross:none:1
b>d:low:normal:none:1
a>d::normal:none:2
a>b::none:none:2
d>a:ref:none:none:1
a>c::normal:none:1
`
	if got != want {
		t.Errorf("expected\n%s\ngot\n%s", want, got)
	}
	g := agentOf(t, "agentflow-beta\n  a -.- b\n  a e1@--> b\n  e1@{ animate: true }")
	if g.Edges[0].Style["strokeDash"] != "3,3" {
		t.Errorf("expected a dotted link, got %v", g.Edges[0].Style)
	}
	if g.Edges[1].Style["strokeDash"] != animatedDash {
		t.Errorf("expected an animated link, got %v", g.Edges[1].Style)
	}
}

func TestAgentFlows(t *testing.T) {
	t.Parallel()
	g := agentOf(t, `agentflow-beta TB
  global
    corpus["Shared corpus"]
  end
  connector llm["LLM API"]
  llm@{ protocol: "http", shape: nosuch }
  flow team["Coffee Team"]
    city["city"]
    flow researcher["Researcher"]
      direction LR
      city --> research --> corpus
    end
    researcher@{ model: "m" }
    flow designer
      render
    end
    researcher --> designer
    connector crm
  end
  flow
    review
  end
  render --> review
  flow team
    extra
  end`)
	for id, want := range map[string]string{
		"corpus": "", "llm": "", "city": "researcher", "research": "researcher", "render": "designer",
		"review": "subGraph3", "crm": "team", "extra": "team",
	} {
		if n := node(t, g, id); n.Cluster != want {
			t.Errorf("%s: expected the flow %q, got %q", id, want, n.Cluster)
		}
	}
	for id, want := range map[string][2]string{
		"team": {"", "Coffee Team"}, "researcher": {"team", "Researcher"}, "designer": {"team", ""}, "subGraph3": {"", ""},
	} {
		c := clusterOf(t, g, id)
		if c.Parent != want[0] || c.Label != want[1] {
			t.Errorf("%s: expected the parent %q and the title %q, got %q and %q", id, want[0], want[1], c.Parent, c.Label)
		}
	}
	if len(g.Clusters) != 4 {
		t.Errorf("expected 4 flows, got %d", len(g.Clusters))
	}
	if c := clusterOf(t, g, "researcher"); c.Direction != graph.LR {
		t.Errorf("expected the direction LR, got %q", c.Direction)
	}
	if n := node(t, g, "llm"); n.Label != "LLM API" || n.Shape != "rounded" {
		t.Errorf("expected the connector LLM API, got %q %q", n.Label, n.Shape)
	}
	if n := node(t, g, "crm"); n.Label != "crm" {
		t.Errorf("expected the connector crm, got %q", n.Label)
	}
	// The edge to a flow ends at the flow.
	for _, n := range g.Nodes {
		if n.ID == "researcher" || n.ID == "designer" {
			t.Errorf("expected no node of the flow %s", n.ID)
		}
	}
}

func TestAgentCollapsed(t *testing.T) {
	t.Parallel()
	g := agentOf(t, `agentflow-beta
  flow intake["Intake"]
    receive
  end
  flow processing["Processing"]@{ model: "m" }
    validate --> enrich
  end
  processing@{ view: "collapsed" }
  flow f2["Two"]@{ view: collapsed }
    flow inner
      x
    end
  end
  receive --> validate
  enrich --> x`)
	if n := node(t, g, "processing"); n.Shape != "collapsedGroup" || n.Label != "Processing" || n.MinWidth != 0 {
		t.Errorf("expected the collapsed flow Processing, got %q %q %v", n.Shape, n.Label, n.MinWidth)
	}
	if n := node(t, g, "f2"); n.Shape != "collapsedGroup" || n.Label != "Two" {
		t.Errorf("expected the collapsed flow Two, got %q %q", n.Shape, n.Label)
	}
	got := edgesOf(t, `agentflow-beta
  flow intake["Intake"]
    receive
  end
  flow processing["Processing"]
    validate --> enrich
  end
  processing@{ view: "collapsed" }
  receive --> validate
  enrich --> receive`)
	if want := "receive>processing::normal:none:1\nprocessing>receive::normal:none:1\n"; got != want {
		t.Errorf("expected\n%s\ngot\n%s", want, got)
	}
	for _, c := range g.Clusters {
		if c.ID != "intake" {
			t.Errorf("expected only the flow intake, got %s", c.ID)
		}
	}
}

func TestAgentConfig(t *testing.T) {
	t.Parallel()
	g := agentOf(t, `---
config:
  agentflow:
    nodeSpacing: 100
    rankSpacing: 25
    minNodeWidth: 0
    wrappingWidth: 140
    useMaxWidth: false
  flowchart:
    curve: linear
---
agentflow-beta
  a --> b`)
	if g.NodeSep != 48 || g.RankSep != 20 {
		t.Errorf("expected the spaces 48 and 20, got %v and %v", g.NodeSep, g.RankSep)
	}
	if n := node(t, g, "a"); n.MinWidth != 0 || n.Wrap != scaleWrap(140) {
		t.Errorf("expected no least width and a wrap of %v, got %v and %v", scaleWrap(140), n.MinWidth, n.Wrap)
	}
	if g.Edges[0].Curve != "linear" {
		t.Errorf("expected a linear curve, got %q", g.Edges[0].Curve)
	}
	for _, src := range []string{
		"---\nconfig:\n  agentflow:\n    minNodeWidth: wide\n---\nagentflow-beta\n  a",
		"---\nconfig:\n  agentflow:\n    theme: dark\n---\nagentflow-beta\n  a",
		"---\nconfig:\n  flowchart:\n    padding: 4\n---\nagentflow-beta\n  a",
		"agentflow-beta\n  a@{ view: collapsed }",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, report, err := Graph(d, false); err != nil || len(report) != 1 {
			t.Errorf("%q: expected one issue, got %v and %v", src, report, err)
		}
	}
}

func TestAgentErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		// Mermaid reads no other link.
		"agentflow-beta\n  a --- b",
		"agentflow-beta\n  a ==> b",
		"agentflow-beta\n  a -.-> b",
		"agentflow-beta\n  a --o b",
		"agentflow-beta\n  a <--> b",
		"agentflow-beta\n  a ~~~ b",
		"agentflow-beta\n  a -. text .- b",
		"agentflow-beta\n  a -- text b",
		// An agentflow diagram has no subgraph and no title.
		"agentflow-beta\n  subgraph x\n  a\n  end",
		"agentflow-beta\n  title Hello",
		"agentflow-beta foo\n  a",
		// The shapes are in lower case, with no underscore, and Mermaid
		// has them.
		"agentflow-beta\n  a@{ shape: nosuch }",
		"agentflow-beta\n  a@{ shape: Task }",
		"agentflow-beta\n  a@{ shape: lean_right }",
		"agentflow-beta\n  a@{ shape: connector }",
		"agentflow-beta\n  a@{\n    shape: input,\n    value: x\n  }",
		"agentflow-beta\n  a@{ shape: task, value: \"x }",
		// A flow or a block global ends once.
		"agentflow-beta\n  flow x\n  a",
		"agentflow-beta\n  global\n  a",
		"agentflow-beta\n  a\n  end",
		"agentflow-beta\n  global x\n  end",
		"agentflow-beta\n  connector",
		"agentflow-beta\n  flow x[\"X\"] y\n  end",
	} {
		d, err := Read(src)
		if err == nil {
			_, _, err = Graph(d, false)
		}
		if !errors.Is(err, ErrSyntax) {
			t.Errorf("%q: expected ErrSyntax, got %v", src, err)
		}
	}
}

func TestAgentCycle(t *testing.T) {
	t.Parallel()
	d, err := Read(`agentflow-beta
  flow a
    flow b
      x --> a
    end
  end`)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, false)
	if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "holds it") {
		t.Fatalf("expected one issue of a loop, got %v and %v", report, err)
	}
	if a, b := clusterOf(t, g, "a"), clusterOf(t, g, "b"); a.Parent != "" || b.Parent != "a" {
		t.Errorf("expected the flow b in the flow a, as Mermaid, got %q and %q", a.Parent, b.Parent)
	}
}
