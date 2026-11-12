package mermaid

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// classOf reads a class diagram in the loose mode.
func classOf(t *testing.T, src string) *graph.Graph {
	t.Helper()
	g, report := stateOf(t, src)
	if len(report) > 0 {
		t.Fatalf("unexpected report %v", report)
	}
	return g
}

// node returns the node of an id, or fails.
func node(t *testing.T, g *graph.Graph, id string) *graph.Node {
	t.Helper()
	for _, n := range g.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("no node %s", id)
	return nil
}

func TestClassMembers(t *testing.T) {
	t.Parallel()
	g := classOf(t, `classDiagram
  Animal <|-- Duck
  Animal : +int age
  Animal: +isMammal()
  class Duck{
    +String beakColor
    +swim()
  }
  class Square~Shape~{
    List~int~ position
    getPoints() List~List~int~~
  }
  class Shape{
    <<interface>>
    +area()$ double
    +draw()*
    +String name$
    +int sides*
  }
  <<abstract>> Duck
  class Empty
  class Animal2["An animal"]`)
	tests := []struct {
		id    string
		texts []string
	}{
		{"Animal", []string{"Animal", "+int age", "+isMammal()"}},
		{"Duck", []string{"«abstract»\nDuck", "+String beakColor", "+swim()"}},
		{"Square", []string{"Square<Shape>", "List<int> position", "getPoints() : List<List<int>>"}},
		{"Shape", []string{"«interface»\nShape", "+String name\n+int sides", "+area() : double\n+draw()"}},
		{"Empty", []string{"Empty", "", ""}},
		{"Animal2", []string{"An animal", "", ""}},
	}
	for _, test := range tests {
		n := node(t, g, test.id)
		if n.Shape != "rect" || !n.Stack || len(n.Fields) != 3 {
			t.Fatalf("%s: expected a stacked rect of 3 fields, got %q %v %d", test.id, n.Shape, n.Stack, len(n.Fields))
		}
		for i, f := range n.Fields {
			if f.Text != test.texts[i] {
				t.Errorf("%s: field %d: expected %q, got %q", test.id, i, test.texts[i], f.Text)
			}
		}
		if n.Fields[1].Align != "l" || n.Fields[2].Align != "l" {
			t.Errorf("%s: expected the members at the left", test.id)
		}
	}
	shape := node(t, g, "Shape")
	if f := shape.Fields[0].Fonts; len(f) != 2 || f[0].Bold || !f[1].Bold {
		t.Errorf("expected a plain annotation over a bold name, got %v", f)
	}
	if f := shape.Fields[1].Fonts; len(f) != 2 || !f[0].Underline || !f[1].Italic {
		t.Errorf("expected a static and an abstract attribute, got %v", f)
	}
	if f := shape.Fields[2].Fonts; len(f) != 2 || !f[0].Underline || !f[1].Italic {
		t.Errorf("expected a static and an abstract method, got %v", f)
	}
}

func TestClassRelations(t *testing.T) {
	t.Parallel()
	g := classOf(t, `classDiagram
  direction LR
  A <|-- B : Inheritance
  C *-- D
  E o-- F
  G <-- H
  I -- J
  K <.. L
  M <|.. N
  O .. P
  Q --|> R
  S "1" --> "many" T : has
  U <|--|> V
  W *--* X
  bar ()-- foo
  foo --() baz
  Zoo--Yak`)
	if g.Direction != graph.LR {
		t.Errorf("expected LR, got %s", g.Direction)
	}
	tests := []struct {
		source, target, tail, head string
		dashed                     bool
	}{
		{"A", "B", "empty", "none", false},
		{"C", "D", "diamond", "none", false},
		{"E", "F", "odiamond", "none", false},
		{"G", "H", "vee", "none", false},
		{"I", "J", "none", "none", false},
		{"K", "L", "vee", "none", true},
		{"M", "N", "empty", "none", true},
		{"O", "P", "none", "none", true},
		{"Q", "R", "none", "empty", false},
		{"S", "T", "none", "vee", false},
		{"U", "V", "empty", "empty", false},
		{"W", "X", "diamond", "diamond", false},
		{"interface0", "foo", "odot", "none", false},
		{"foo", "interface1", "none", "odot", false},
		{"Zoo", "Yak", "none", "none", false},
	}
	if len(g.Edges) != len(tests) {
		t.Fatalf("expected %d edges, got %d", len(tests), len(g.Edges))
	}
	for i, test := range tests {
		e := g.Edges[i]
		if e.Source != test.source || e.Target != test.target || e.Tail != test.tail || e.Head != test.head {
			t.Errorf("edge %d: expected %s %s %s %s, got %s %s %s %s", i, test.source, test.target, test.tail, test.head,
				e.Source, e.Target, e.Tail, e.Head)
		}
		if dashed := e.Style["strokeDash"] != ""; dashed != test.dashed {
			t.Errorf("edge %d: expected dashed %v", i, test.dashed)
		}
	}
	if e := g.Edges[0]; e.Label != "Inheritance" {
		t.Errorf("expected the label Inheritance, got %q", e.Label)
	}
	if e := g.Edges[9]; e.TailLabel != "1" || e.HeadLabel != "many" || e.Label != "has" {
		t.Errorf("expected the cardinalities 1 and many, got %q %q %q", e.TailLabel, e.HeadLabel, e.Label)
	}
	if n := node(t, g, "interface0"); n.Label != "bar" || n.Shape != "none" || len(n.Fields) != 0 {
		t.Errorf("expected the interface bar with no box, got %q %q", n.Label, n.Shape)
	}
}

func TestClassNamespacesNotes(t *testing.T) {
	t.Parallel()
	g := classOf(t, `classDiagram
  class Lid
  namespace Shapes {
    class Triangle
    class Lid
    class Rectangle {
      double width
    }
  }
  note "A floating note"
  note for Triangle "Three sides<br>and three corners"
  accTitle: Shapes
  accDescr: Shapes in a namespace`)
	for id, want := range map[string]string{"Lid": "Shapes", "Triangle": "Shapes", "Rectangle": "Shapes", "note0": "", "note1": ""} {
		if n := node(t, g, id); n.Cluster != want {
			t.Errorf("%s: expected the cluster %q, got %q", id, want, n.Cluster)
		}
	}
	if len(g.Clusters) != 1 || g.Clusters[0].Label != "Shapes" {
		t.Errorf("expected the cluster Shapes, got %v", g.Clusters)
	}
	if n := node(t, g, "note1"); n.Shape != "note" || n.Label != "Three sides\nand three corners" {
		t.Errorf("expected a note, got %q %q", n.Shape, n.Label)
	}
	if len(g.Edges) != 1 || g.Edges[0].Source != "note1" || g.Edges[0].Target != "Triangle" || g.Edges[0].Head != "none" {
		t.Errorf("expected a line from the note to Triangle, got %v", g.Edges)
	}
	if g.Description != "Shapes Shapes in a namespace" {
		t.Errorf("unexpected description %q", g.Description)
	}
}

func TestClassStyles(t *testing.T) {
	t.Parallel()
	g, report := stateOf(t, `classDiagram-v2
  class Animal:::hot
  class Mineral
  Animal --> Plant
  classDef hot fill:#f96
  classDef pale fill:#eee
  classDef default stroke:#333
  cssClass "Mineral,Plant" pale
  style Animal stroke:#c00
  click Animal href "https://example.com"
  link Mineral "https://example.com"
  callback Plant "fn"`)
	if n := node(t, g, "Animal"); n.Style["fill"] != "#f96" || n.Style["stroke"] != "#c00" {
		t.Errorf("Animal: unexpected style %v", n.Style)
	}
	for _, id := range []string{"Mineral", "Plant"} {
		if n := node(t, g, id); n.Style["fill"] != "#eee" || n.Style["stroke"] != "#333" {
			t.Errorf("%s: unexpected style %v", id, n.Style)
		}
	}
	var msgs []string
	for _, r := range report {
		msgs = append(msgs, r.Message)
	}
	if got := strings.Join(msgs, ", "); got != "the callback fn, since no script runs" {
		t.Errorf("unexpected report %q", got)
	}
	for _, id := range []string{"Animal", "Mineral"} {
		if n := node(t, g, id); n.Href != "https://example.com" {
			t.Errorf("%s: the link %q, want https://example.com", id, n.Href)
		}
	}
	// A click on a name that is not a class draws nothing in Mermaid, so
	// it is no issue (D32).
	d, _ := Read("classDiagram\n  class B\n  click A href \"x\"")
	if g, report, err := Graph(d, true); err != nil || len(report) != 0 || len(g.Nodes) != 1 {
		t.Errorf("a click on no class: the error %v, the report %v and %d nodes", err, report, len(g.Nodes))
	}
}

func TestClassText(t *testing.T) {
	t.Parallel()
	g := classOf(t, "---\nconfig:\n  class:\n    hideEmptyMembersBox: true\n---\nclassDiagram\n"+
		"  class A[\"This has\n  a newline!\"] {\n  +String x\n  }\n"+
		"  class `Du\n  ck`\n  `Du\n  ck` <|-- `Car Class`\n"+
		"  class C5[\"With {Brackets}\"]\n"+
		"  class D {\n  +List&lt;int&gt; items\n  }\n"+
		"  class E { +int one }")
	if n := node(t, g, "A"); n.Fields[0].Text != "This has\na newline!" || len(n.Fields) != 3 {
		t.Errorf("A: unexpected name %q", n.Fields[0].Text)
	}
	if n := node(t, g, "Du\nck"); len(n.Fields) != 1 || len(n.Fields[0].Fonts) != 2 || !n.Fields[0].Fonts[1].Bold {
		t.Errorf("expected an empty class drawn as its bold name alone, got %v", n.Fields)
	}
	if len(g.Edges) != 1 || g.Edges[0].Target != "Car Class" {
		t.Errorf("expected an edge to Car Class, got %v", g.Edges)
	}
	if n := node(t, g, "C5"); n.Label != "With {Brackets}" {
		t.Errorf("C5: unexpected label %q", n.Label)
	}
	if n := node(t, g, "D"); n.Fields[1].Text != "+List<int> items" {
		t.Errorf("D: unexpected member %q", n.Fields[1].Text)
	}
	if n := node(t, g, "E"); n.Fields[1].Text != "+int one" {
		t.Errorf("E: unexpected member %q", n.Fields[1].Text)
	}
}

func TestClassErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"classDiagram LR\n  A",
		"classDiagram\n  class A {\n  +x",
		"classDiagram\n  namespace N {\n  class A",
		"classDiagram\n  }",
		"classDiagram\n  direction XY",
		"classDiagram\n  A -- ",
		"classDiagram\n  note for A",
		"classDiagram\n  A B C",
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

func TestGeneric(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"List~int~":        "List<int>",
		"List~List~int~~":  "List<List<int>>",
		"Map~K, V~ x":      "Map<K, V> x",
		"Map~K,V~":         "Map<K,V>",
		"a ~ b":            "a ~ b",
		"Square~Shape~":    "Square<Shape>",
		"f(List~int~ xs)":  "f(List<int> xs)",
		"no generic types": "no generic types",
	} {
		if got := generic(in); got != want {
			t.Errorf("%q: expected %q, got %q", in, want, got)
		}
	}
}
