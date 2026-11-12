package mermaid

import (
	"errors"
	"slices"
	"testing"

	"github.com/xo/zuzu/graph"
)

func TestRequirementBoxes(t *testing.T) {
	t.Parallel()
	g := classOf(t, `requirementDiagram
  REQUIREMENT test_req {
    ID: 1
    text: the test text.
    risk: High
    verifyMethod: test
  }
  functionalRequirement "Fast Login" {
    id: "REQ-1"
    text: "**Fast** login, in _two_ seconds"
  }
  interfaceRequirement r3 {
  }
  performanceRequirement r4 {
    risk: medium
  }
  physicalRequirement r5 {
    verifymethod: Demonstration
  }
  designConstraint r6 {
  }
  element "*Auth Service*" {
    type: "test suite"
    docRef: reqs/test_entity
  }
  element e2 {
    docref: "a, b"
  }
  requirement last {
  }`)
	want := []string{"test_req", "Fast Login", "r3", "r4", "r5", "r6", "last", "*Auth Service*", "e2"}
	var ids []string
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("expected the requirements before the elements, %q, got %q", want, ids)
	}
	for _, c := range []struct {
		id, head, body string
	}{
		{"test_req", "«Requirement»\ntest_req", "ID: 1\nText: the test text.\nRisk: High\nVerification: Test"},
		{"Fast Login", "«Functional Requirement»\nFast Login", "ID: REQ-1\nText: Fast login, in two seconds"},
		{"r3", "«Interface Requirement»\nr3", ""},
		{"r4", "«Performance Requirement»\nr4", "Risk: Medium"},
		{"r5", "«Physical Requirement»\nr5", "Verification: Demonstration"},
		{"r6", "«Design Constraint»\nr6", ""},
		{"*Auth Service*", "«Element»\nAuth Service", "Type: test suite\nDoc Ref: reqs/test_entity"},
		{"e2", "«Element»\ne2", "Doc Ref: a, b"},
	} {
		n := node(t, g, c.id)
		if n.Shape != "rect" || !n.Stack {
			t.Errorf("%s: expected a stacked rect, got %q %v", c.id, n.Shape, n.Stack)
		}
		if n.Fields[0].Text != c.head {
			t.Errorf("%s: expected the head %q, got %q", c.id, c.head, n.Fields[0].Text)
		}
		switch {
		case c.body == "" && len(n.Fields) != 1:
			t.Errorf("%s: expected no body, got %v", c.id, n.Fields)
		case c.body != "" && (len(n.Fields) != 2 || n.Fields[1].Text != c.body || n.Fields[1].Align != "l"):
			t.Errorf("%s: expected the body %q from the left, got %v", c.id, c.body, n.Fields)
		}
	}
	if f := node(t, g, "*Auth Service*").Fields[0].Fonts; len(f) != 2 || f[0] != (graph.Font{}) || f[1] != (graph.Font{Bold: true, Italic: true}) {
		t.Errorf("expected a plain type and a bold italic name, got %v", f)
	}
}

func TestRequirementRelations(t *testing.T) {
	t.Parallel()
	g := classOf(t, `requirementDiagram
  requirement a {
  }
  requirement b {
  }
  element "c d" {
  }
  a - contains -> b
  a - copies -> b
  a - derives -> b
  "c d" - satisfies -> a
  "c d" - verifies -> b
  a - REFINES -> b
  a - traces -> b
  a <- copies - "c d"
  b<-contains-a`)
	want := []struct{ src, dst, label, head, tail string }{
		{"a", "b", "«contains»", "none", "circleplus"},
		{"a", "b", "«copies»", "vee", "none"},
		{"a", "b", "«derives»", "vee", "none"},
		{"c d", "a", "«satisfies»", "vee", "none"},
		{"c d", "b", "«verifies»", "vee", "none"},
		{"a", "b", "«refines»", "vee", "none"},
		{"a", "b", "«traces»", "vee", "none"},
		{"c d", "a", "«copies»", "vee", "none"},
		{"a", "b", "«contains»", "none", "circleplus"},
	}
	if len(g.Edges) != len(want) {
		t.Fatalf("expected %d edges, got %d", len(want), len(g.Edges))
	}
	for i, w := range want {
		e := g.Edges[i]
		if e.Source != w.src || e.Target != w.dst || e.Label != w.label || e.Head != w.head || e.Tail != w.tail {
			t.Errorf("edge %d: expected %v, got %s -> %s %q %s %s", i, w, e.Source, e.Target, e.Label, e.Head, e.Tail)
		}
		if dash := e.Style["strokeDash"]; (dash == "") != (w.head == "none") {
			t.Errorf("edge %d: unexpected dash %q", i, dash)
		}
		if !graph.ArrowOf(e.Head) || !graph.ArrowOf(e.Tail) {
			t.Errorf("edge %d: zuzu does not draw %s or %s", i, e.Head, e.Tail)
		}
	}
}

func TestRequirementStyles(t *testing.T) {
	t.Parallel()
	g, report := stateOf(t, `---
title: Styles
---
requirementDiagram
  accTitle: Reqs
  accDescr: Two requirements
  direction LR
  # a comment
  requirement a:::hot, cool {
  }
  element b {
  }
  element c {
  }
  classDef hot fill:#f96
  classDef cool, cold stroke:#00f
  class b, c hot
  c:::cold
  style a, b color:#f00
  style zz color:#0f0`)
	if g.Direction != graph.LR || g.Title != "Styles" || g.Description != "Reqs Two requirements" {
		t.Errorf("unexpected direction %s, title %q or description %q", g.Direction, g.Title, g.Description)
	}
	for _, c := range []struct{ id, fill, stroke, color string }{
		{"a", "#f96", "#00f", "#f00"},
		{"b", "#f96", "", "#f00"},
		{"c", "#f96", "#00f", ""},
	} {
		st := node(t, g, c.id).Style
		if st["fill"] != c.fill || st["stroke"] != c.stroke || st["color"] != c.color {
			t.Errorf("%s: unexpected style %v", c.id, st)
		}
	}
	// Mermaid ignores the style of zz, which names nothing, so it is no
	// issue (D32).
	if len(report) != 0 {
		t.Errorf("expected no issue for the style of zz, got %v", report)
	}
}

func TestRequirementIssues(t *testing.T) {
	t.Parallel()
	// Mermaid ignores a class or a style before the definition, and the
	// body of a second definition, so none of them is an issue (D32).
	for _, src := range []string{
		"requirementDiagram\n  class a hot\n  requirement a {\n  }",
		"requirementDiagram\n  a:::hot\n  requirement a {\n  }",
		"requirementDiagram\n  style a fill:#f00\n  requirement a {\n  }",
		"requirementDiagram\n  requirement a {\n  }\n  requirement a {\n  id: 2\n  }",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, true)
		if err != nil || len(report) != 0 || len(g.Nodes) != 1 {
			t.Errorf("%q: expected one node and no issue, got %v and %v", src, report, err)
		}
	}
	for _, src := range []string{
		"---\nconfig:\n  requirement:\n    rect_fill: red\n---\nrequirementDiagram\n  requirement a {\n  }",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, false)
		if err != nil || len(report) != 1 || len(g.Nodes) != 1 {
			t.Errorf("%q: expected one node and one issue, got %v and %v", src, report, err)
		}
		if _, _, err := Graph(d, true); err == nil {
			t.Errorf("%q: expected an error in the strict mode", src)
		}
	}
}

func TestRequirementErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"requirementDiagram LR\n  requirement a {\n  }",
		"requirementDiagram\n  requirement a {\n  id: 1",
		"requirementDiagram\n  requirement a { id: 1\n  }",
		"requirementDiagram\n  requirement a\n  {\n  }",
		"requirementDiagram\n  requirement a {\n  type: x\n  }",
		"requirementDiagram\n  element a {\n  id: 1\n  }",
		"requirementDiagram\n  requirement a {\n  risk: huge\n  }",
		"requirementDiagram\n  requirement a {\n  verifymethod: \"test\"\n  }",
		// An unquoted text cannot hold a dash or a comma, start with a
		// keyword, hold a direction or start with a letter that is not
		// ASCII.
		"requirementDiagram\n  requirement a {\n  id: REQ-1\n  }",
		"requirementDiagram\n  requirement a {\n  text: a, b\n  }",
		"requirementDiagram\n  requirement a {\n  text: test the system\n  }",
		"requirementDiagram\n  requirement low power {\n  }",
		"requirementDiagram\n  requirement a {\n  text: the direction LR\n  }",
		"requirementDiagram\n  requirement \u00e9t\u00e9 {\n  }",
		"requirementDiagram\n  requirement \"a {\n  }",
		"requirementDiagram\n  direction TD",
		"requirementDiagram\n  title Reqs",
		"requirementDiagram\n  requirement a {\n  }\n  element a {\n  }",
		"requirementDiagram\n  requirement a {\n  }\n  a - satisfies -> b",
		"requirementDiagram\n  requirement a {\n  }\n  a - satisfies - a",
		"requirementDiagram\n  requirement a {\n  }\n  a <- satisfies -> a",
		"requirementDiagram\n  requirement a {\n  }\n  a - likes -> a",
		"requirementDiagram\n  requirement a {\n  }\n  a",
		"requirementDiagram\n  class a",
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

func TestRequirementBraceLine(t *testing.T) {
	t.Parallel()
	g := classOf(t, "requirementDiagram\n  requirement a {\n  } element b {\n  }\n  a - traces -> b")
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Errorf("expected a statement after an end brace, got %d nodes and %d edges", len(g.Nodes), len(g.Edges))
	}
}

// TestRequirementDescription checks that the description of the front
// matter comes first in the description of the graph, then accTitle, as
// in the other kinds (D32).
func TestRequirementDescription(t *testing.T) {
	t.Parallel()
	g := classOf(t, "---\ndescription: The front\n---\nrequirementDiagram\naccTitle: The title\nrequirement r {\nid: 1\n}\n")
	if want := "The front The title"; g.Description != want {
		t.Errorf("the description %q, not %q", g.Description, want)
	}
}
