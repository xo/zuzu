package mermaid

import (
	"errors"
	"slices"
	"testing"

	"github.com/xo/zuzu/graph"
)

func TestERAttributes(t *testing.T) {
	t.Parallel()
	g := classOf(t, `erDiagram
  CUSTOMER ||--o{ ORDER : places
  CUSTOMER {
    string name PK "the name"
    string id PK, FK
    string(99) note "a #quot;quoted#quot; comment"
    int age
  }
  p[Person] {
    string firstName
  }
  a["Customer Account"]
  "Entity with space" ||--|| p : names
  ITEM { string sku }`)
	c := node(t, g, "CUSTOMER")
	if c.Shape != "rect" || !c.Stack || len(c.Fields) != 5 {
		t.Fatalf("expected a stacked rect of 5 fields, got %q %v %d", c.Shape, c.Stack, len(c.Fields))
	}
	if f := c.Fields[0]; f.Text != "CUSTOMER" || len(f.Fonts) != 1 || !f.Fonts[0].Bold {
		t.Errorf("expected the bold name, got %v", f)
	}
	rows := [][]string{
		{"string", "name", "PK", "the name"},
		{"string", "id", "PK, FK", ""},
		{"string(99)", "note", "", `a "quoted" comment`},
		{"int", "age", "", ""},
	}
	for i, want := range rows {
		f := c.Fields[i+1]
		if !slices.Equal(f.Cells, want) || f.Port != want[1] {
			t.Errorf("row %d: expected %q, got %q with the port %q", i, want, f.Cells, f.Port)
		}
	}
	if n := node(t, g, "ORDER"); len(n.Fields) != 0 || n.Label != "ORDER" {
		t.Errorf("expected ORDER with no fields, got %v", n.Fields)
	}
	if n := node(t, g, "p"); n.Label != "Person" || n.Fields[0].Text != "Person" {
		t.Errorf("expected the alias Person, got %q", n.Label)
	}
	if n := node(t, g, "a"); n.Label != "Customer Account" {
		t.Errorf("expected the alias Customer Account, got %q", n.Label)
	}
	if n := node(t, g, "Entity with space"); n.Label != "Entity with space" {
		t.Errorf("unexpected label %q", n.Label)
	}
	if n := node(t, g, "ITEM"); len(n.Fields) != 2 || n.Fields[1].Cells[1] != "sku" {
		t.Errorf("expected ITEM with the attribute sku, got %v", n.Fields)
	}
}

func TestERRelationships(t *testing.T) {
	t.Parallel()
	g := classOf(t, `erDiagram
  A |o--o| B : "zero or one"
  A ||--|| C : one
  A }o--o{ D : many
  A }|--|{ E : more
  F |o..o{ G : dashed
  M only one to zero or more C : makes
  C one or more optionally to one or zero D : has
  D 1+ to 0+ E : holds
  E many(1) optionally to only one F : by
  F 1 to many(0) G : in
  G one or many to zero or many H : ""
  H |o.-|| I : posts
  I }|-.o{ J : on
  J o{--|| K : flipped
  K 1--1 L
  P u--o{ Q : parent
  A ||--|| u : named`)
	tests := []struct {
		source, target, tail, head string
		dashed                     bool
	}{
		{"A", "B", "teeodot", "teeodot", false},
		{"A", "C", "teetee", "teetee", false},
		{"A", "D", "crowodot", "crowodot", false},
		{"A", "E", "crowtee", "crowtee", false},
		{"F", "G", "teeodot", "crowodot", true},
		{"M", "C", "teetee", "crowodot", false},
		{"C", "D", "crowtee", "teeodot", true},
		{"D", "E", "crowtee", "crowodot", false},
		{"E", "F", "crowtee", "teetee", true},
		{"F", "G", "teetee", "crowodot", false},
		{"G", "H", "crowtee", "crowodot", false},
		{"H", "I", "teeodot", "teetee", true},
		{"I", "J", "crowtee", "crowodot", true},
		{"J", "K", "crowodot", "teetee", false},
		{"K", "L", "teetee", "teetee", false},
		{"P", "Q", "none", "crowodot", false},
		{"A", "u", "teetee", "teetee", false},
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
	for i, want := range map[int]string{0: "zero or one", 5: "makes", 10: "", 14: ""} {
		if g.Edges[i].Label != want {
			t.Errorf("edge %d: expected the label %q, got %q", i, want, g.Edges[i].Label)
		}
	}
}

func TestERSubgraphsAndStyles(t *testing.T) {
	t.Parallel()
	g := classOf(t, `erDiagram
  direction LR
  PRODUCT ||--o{ domain : links
  subgraph domain["The domain"]
    direction TB
    CUSTOMER:::hot
    subgraph details
      ORDER
    end
  end
  CUSTOMER ||--o{ ORDER : places
  classDef hot fill:#f96
  classDef cool fill:#9cf
  class ORDER,PRODUCT cool
  style PRODUCT stroke:#333
  accDescr {
    Orders
    of products
  }`)
	if g.Direction != graph.LR {
		t.Errorf("expected LR, got %s", g.Direction)
	}
	if got := summary(g); got != "PRODUCT:rect:PRODUCT CUSTOMER:rect:CUSTOMER@domain ORDER:rect:ORDER@details "+
		"[domain:The domain] [details:details@domain] PRODUCT>domain:links CUSTOMER>ORDER:places" {
		t.Errorf("unexpected graph %s", got)
	}
	if g.Clusters[0].Direction != graph.TB {
		t.Errorf("expected the subgraph to run TB, got %q", g.Clusters[0].Direction)
	}
	if n := node(t, g, "CUSTOMER"); n.Style["fill"] != "#f96" {
		t.Errorf("CUSTOMER: unexpected style %v", n.Style)
	}
	if n := node(t, g, "PRODUCT"); n.Style["fill"] != "#9cf" || n.Style["stroke"] != "#333" {
		t.Errorf("PRODUCT: unexpected style %v", n.Style)
	}
	if g.Description != "Orders of products" {
		t.Errorf("unexpected description %q", g.Description)
	}
}

func TestERErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"erDiagram LR\n  A",
		"erDiagram\n  A {\n  string x",
		"erDiagram\n  A {\n  string\n  }",
		"erDiagram\n  A ||--o{ B places",
		"erDiagram\n  A <> B",
		"erDiagram\n  A ||~~o{ B : x",
		"erDiagram\n  subgraph S\n  A",
		"erDiagram\n  subgraph S\n  end\n  A ||--|| S : x",
		"erDiagram\n  \"A",
	} {
		d, err := Read(src)
		if err == nil {
			_, _, err = Graph(d, false)
		}
		if !errors.Is(err, ErrSyntax) {
			t.Errorf("%q: expected ErrSyntax, got %v", src, err)
		}
	}
	d, _ := Read("---\nconfig:\n  er:\n    layoutDirection: LR\n---\nerDiagram\n  A")
	if _, _, err := Graph(d, true); err == nil {
		t.Error("expected an error in the strict mode")
	}
}
