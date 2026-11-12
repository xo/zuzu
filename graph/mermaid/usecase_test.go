package mermaid

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// usecaseFixture reads a fixture of testdata/usecase in the strict mode,
// and compiles it.
func usecaseFixture(t *testing.T, name string) *graph.Graph {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "usecase", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(string(src))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, report, err := Graph(d, true)
	if err != nil || len(report) > 0 {
		t.Fatalf("graph: %v %v", err, report)
	}
	if _, err := graph.Compile(g); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return g
}

// ucEdges writes the edges of a graph as source>target:head:tail:label:minlen.
func ucEdges(g *graph.Graph) []string {
	var out []string
	for _, e := range g.Edges {
		minlen := ""
		if e.MinLen > 1 {
			minlen = ":" + string(rune('0'+e.MinLen))
		}
		out = append(out, e.Source+">"+e.Target+":"+e.Head+":"+e.Tail+":"+e.Label+minlen)
	}
	return out
}

func TestUsecaseRelations(t *testing.T) {
	t.Parallel()
	g := usecaseFixture(t, "01-relations.mmd")
	if g.Direction != graph.LR || g.Title != "Library" {
		t.Errorf("expected LR and the title Library, got %s %q", g.Direction, g.Title)
	}
	var nodes []string
	for _, n := range g.Nodes {
		nodes = append(nodes, n.ID+":"+n.Shape+":"+n.Label+"@"+n.Cluster)
	}
	want := []string{
		"Member:rect:Library member@", "Librarian:rect:Librarian@",
		"Borrow:ellipse:Borrow a book@Desk", "Return:ellipse:Return a book@Desk", "Renew:rect:Renew a loan@Desk",
		"CheckCard:ellipse:CheckCard@", "Fine:ellipse:Fine@",
	}
	if !slices.Equal(nodes, want) {
		t.Errorf("expected the actors, then the use cases by their first mention,\n%q, got\n%q", want, nodes)
	}
	if len(g.Clusters) != 1 || g.Clusters[0].ID != "Desk" || g.Clusters[0].Label != "Front desk" {
		t.Errorf("expected the boundary Desk, got %v", g.Clusters)
	}
	edges := []string{
		"Member>Borrow:normal:none:", "Member>Return:normal:none::2", "Return>Librarian:none:normal:",
		"Member>Renew:normal:none:online", "Borrow>CheckCard:normal:none:«include»", "Fine>Return:normal:none:«extend»",
		"Librarian>Member:empty:none:", "Member>Fine:circle:none:", "Librarian>Fine:none:cross:", "Member>Renew:none:none:",
	}
	if got := ucEdges(g); !slices.Equal(got, edges) {
		t.Errorf("expected the edges\n%q, got\n%q", edges, got)
	}
	for _, i := range []int{4, 5} {
		if g.Edges[i].Style["strokeDash"] != dashed {
			t.Errorf("expected include and extend dashed, got %v", g.Edges[i].Style)
		}
	}
	if m := node(t, g, "Member").Media; m == nil || m.Path != ucFigure("", false) {
		t.Errorf("expected a stick figure, got %+v", m)
	}
}

func TestUsecaseActors(t *testing.T) {
	t.Parallel()
	g := usecaseFixture(t, "02-actors.mmd")
	buyer := node(t, g, "Buyer")
	if buyer.Label != "«Person»\nBuyer" || buyer.Style["fontStyle"] != "italic" {
		t.Errorf("expected the stereotype over the name, in italic, got %q %v", buyer.Label, buyer.Style)
	}
	clerk := node(t, g, "Clerk")
	if clerk.Media.Path != ucFigure("hollow", true) || clerk.Style["fillOpacity"] != "0" {
		t.Errorf("expected a hollow business figure with no fill, got %q %v", clerk.Media.Path, clerk.Style)
	}
	if robot := node(t, g, "Robot"); robot.Media.Path != ucFigure("awesome", false) {
		t.Errorf("expected an awesome figure, got %q", robot.Media.Path)
	}
	if bell := node(t, g, "Bell"); bell.Media.Icon != "fa:bell" || bell.Media.Form != "rounded" {
		t.Errorf("expected the icon in a rounded box, got %+v", bell.Media)
	}
	order := node(t, g, "Order")
	if order.Shape != "businessEllipse" || order.Label != "«Core»\nPlace order" {
		t.Errorf("expected a business use case with its stereotype, got %s %q", order.Shape, order.Label)
	}
	if pay := node(t, g, "Pay"); pay.Label != `Pay "now"` {
		t.Errorf("expected the entities read, got %q", pay.Label)
	}
	note := node(t, g, "note-0")
	if note.Shape != "note" || note.Label != "Needs a cart" {
		t.Errorf("expected a note without its marks, got %s %q", note.Shape, note.Label)
	}
	cart := node(t, g, "Cart")
	var rows []string
	for _, f := range cart.Fields {
		if f.Cells == nil {
			rows = append(rows, f.Text)
			continue
		}
		rows = append(rows, strings.Join(f.Cells, "="))
	}
	want := []string{"Cart", "items[0].name=Book", "tags=a", "=b", "total=29.5", "empty={}"}
	if !cart.Stack || !slices.Equal(rows, want) {
		t.Errorf("expected the rows of the JSON\n%q, got\n%q", want, rows)
	}
	last := g.Edges[len(g.Edges)-1]
	if last.Source != "note-0" || last.Target != "Order" || last.Head != "none" || last.Tail != "none" || last.Style["strokeDash"] != dashed {
		t.Errorf("expected a dashed link from the note, got %+v", last)
	}
	if g.Description != "Shop The actors of a shop." {
		t.Errorf("expected the description, got %q", g.Description)
	}
}

func TestUsecaseStyles(t *testing.T) {
	t.Parallel()
	g := usecaseFixture(t, "03-styles.mmd")
	if g.Direction != graph.TB || g.NodeSep != 60*zuzuNodeSep/mermaidNodeSpacing || g.RankSep != 70*zuzuRankSep/mermaidRankSpacing {
		t.Errorf("expected TB and the spaces of the config, got %s %v %v", g.Direction, g.NodeSep, g.RankSep)
	}
	user := node(t, g, "User")
	if user.Style["stroke"] != "#333333" || user.Style["strokeWidth"] != "3" || user.Style["fontSize"] != "13" {
		t.Errorf("expected the class default, the class outside and the size of the font, got %v", user.Style)
	}
	login := node(t, g, "Login")
	if login.Style["fill"] != "#ffe6e6" || login.Style["strokeWidth"] != "3" || login.Style["fontWeight"] != "700" {
		t.Errorf("expected the style over the classes, got %v", login.Style)
	}
	if s := node(t, g, "Session").Style; s["fill"] != "#fffbea" {
		t.Errorf("expected the class of the block, got %v", s)
	}
	if c := g.Clusters[0].Style; c["fill"] != "#fffbea" || c["stroke"] != "#333333" {
		t.Errorf("expected the classes of the boundary, got %v", c)
	}
	if e := g.Edges[0].Style; e["stroke"] != "#06c" || e["strokeDash"] != animatedDash {
		t.Errorf("expected the style of the edge go, dashed for its animation, got %v", e)
	}
	if e := g.Edges[1].Style; e["stroke"] != "#333333" || e["strokeDash"] != "" || e["fill"] != "" {
		t.Errorf("expected the stroke of the class default and no fill, got %v", e)
	}
}

// TestUsecaseErrors checks the sources that Mermaid 12.3.0 reads as an
// error, and some that it reads, as mmdc read them.
func TestUsecaseErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, src string
		ok        bool
	}{
		{"basic", "actor A\nB\nA --> B", true},
		{"no spaces", "A-->B", true},
		{"actor list", "actor A, B(\"Bee\")\nC\nA --> C", true},
		{"quoted use case", "\"Reset password\"\nactor A\nA --> Reset_password", true},
		{"single quotes", "actor A\n'Reset it'\nA --> Reset_it", true},
		{"label without quotes", "A[Generate a report: now!]\nactor X\nX --> A", true},
		{"labels", "actor A\nB\nC\nA -- go there --> B\nB <-- \"back\" -- C\nC o-- x -- A", true},
		{"dash in a label", "actor A\nB\nA -- a-b c --> B", true},
		{"metadata on lines", "actor A@{\n  type: hollow,\n  business: true\n}\nB\nA --> B", true},
		{"metadata on two lines", "actor A@{ type: hollow\n business: true }", true},
		{"trailing comma", "actor A@{ type: hollow, }\nB\nA --> B", true},
		{"empty metadata", "actor A@{}", true},
		{"two classes", "actor A\nB\nA --> B\nclassDef x,y fill:#f00\nclass A,B x,y", true},
		{"include in capitals", "A\nB\nA ..> : Extend B", true},
		{"edge metadata", "actor A\nB\nA e1@--> B\ne1@{ animation: slow }", true},
		{"empty JSON", "json J@{}\nA\nA --> J", true},
		{"note without quotes", "actor A\nnote for A Price #35; each", true},
		{"same label again", "A(\"x\")\nA(\"x\")", true},
		{"comment in a boundary", "systemBoundary S\n  %% c\n  A\nend", true},
		{"classes on an actor of a relation", "actor A:::c --> B", true},
		{"stereotype of an actor of a relation", "actor A <<x>> --> B", true},
		{"id of digits", "actor 123\nB\n123 --> B", true},
		{"brace in accDescr", "accDescr: a {b}\nactor A", true},
		{"unquoted icon", "actor A@{ icon: user }", true},
		{"two directions", "direction TB\ndirection RL\nactor A", true},
		{"text after the keyword", "", false},
		{"quote with no end", "A(\"x)", false},
		{"brace", "} foo", false},
		{"plantuml", "package Foo", false},
		{"keyword as a name", "actor end", false},
		{"number as a name", "actor 1.5\nB", false},
		{"dash in a name", "actor Admin-Panel", false},
		{"letter beyond ASCII", "actor Ünï", false},
		{"x before an arrow", "x-->y", false},
		{"o after an arrow", "A --open--> B", false},
		{"actor in brackets", "actor A[Sys]", false},
		{"list and relation", "actor A, B --> C", false},
		{"parenthesis in a label", "A(foo (bar))", false},
		{"long arrow with a label", "actor A\nB\nB <--- \"x\" -- A", false},
		{"three dashes with a label", "actor A\nB\nA --- \"l\" --> B", false},
		{"relation in a boundary", "systemBoundary S\n  A --> B\nend", false},
		{"boundary with no end", "systemBoundary S\n  A", false},
		{"text after end", "systemBoundary S\nA\nend foo", false},
		{"boundary in a boundary", "systemBoundary S\n systemBoundary T\n A\n end\nend", false},
		{"style of two names", "actor A\nB\nstyle A,B fill:#f00", false},
		{"variable of CSS", "actor A\nstyle A --my-var:3px", false},
		{"style of an edge with no id", "actor A\nB\nA --> B\nstyle edge-0 stroke:#f00", false},
		{"empty stereotype", "A(\"x\") <<>>", false},
		{"metadata after a stereotype", "A(\"x\") <<s>> @{ business: true }", false},
		{"bad direction", "direction XY", false},
		{"JSON with no end", "json J@{ \"a\": 1", false},
		{"JSON that does not read", "json J@{ \"a\": }", false},
		{"metadata of nothing", "Foo@{ business: true }", false},
		{"unknown metadata", "actor A@{ shape: x }", false},
		{"metadata of a number", "actor A@{ type: 1 }", false},
		{"type of a use case", "A(\"x\")@{ type: hollow }", false},
		{"metadata of JSON", "json J@{ \"a\": 1 }\nJ@{ business: true }", false},
		{"bad type of a boundary", "systemBoundary S@{ type: box }\nA\nend", false},
		{"bad animation", "actor A\nB\nA e@--> B\ne@{ animation: medium }", false},
		{"actor and use case", "actor A\nA(\"x\")", false},
		{"two labels", "A(\"x\")\nA(\"y\")", false},
		{"two shapes", "A(\"x\")\nA[x]", false},
		{"two parents", "systemBoundary S\nA\nend\nsystemBoundary T\nA\nend", false},
		{"two titles", "systemBoundary S[\"a\"]\nend\nsystemBoundary S[\"b\"]\nend", false},
		{"two values of business", "A(\"x\")@{ business: true }\nA(\"x\")@{ business: false }", false},
		{"include of an actor", "actor A\nB\nA ..> : include B", false},
		{"generalization of an actor and a use case", "actor A\nB\nA --|> B", false},
		{"circle at JSON", "json J@{ \"a\": 1 }\nA\nA --o J", false},
		{"relation to a boundary", "systemBoundary S\nA\nend\nactor X\nX --> S", false},
		{"class of nothing", "actor A\nclass Z x", false},
		{"note of nothing", "note for Z \"x\"", false},
		{"note of a boundary", "systemBoundary S\nA\nend\nnote for S \"x\"", false},
		{"classes on a reference", "actor A\nB:::x --> A", false},
		{"icon and type", "actor A@{ icon: \"fa:user\", type: hollow }", false},
		{"awesome business actor", "actor A@{ type: awesome, business: true }", false},
		{"business use case in brackets", "A[x]@{ business: true }", false},
		{"quoted name twice", "\"Reset password\"\n\"Reset password\"", false},
		{"JSON twice", "json J@{ \"a\": 1 }\njson J@{ \"b\": 1 }", false},
		{"edge id twice", "actor A\nB\nA e@--> B\nA e@--> B", false},
	} {
		src := "usecase-beta\n" + c.src + "\n"
		if c.src == "" {
			src = "usecase-beta foo\nactor A\n"
		}
		d, err := Read(src)
		if err == nil {
			_, _, err = Graph(d, true)
		}
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case !c.ok && !errors.Is(err, ErrSyntax):
			t.Errorf("%s: expected an error of the syntax, got %v", c.name, err)
		}
	}
}

func TestUsecaseIssues(t *testing.T) {
	t.Parallel()
	g, report := stateOf(t, `---
config:
  usecase:
    colorScheme: rotate
    useMaxWidth: false
    actorFontFamily: Arial-Bold
    curve: basis
---
usecase-beta
systemBoundary S@{ type: package }
  A
end`)
	var msgs []string
	for _, r := range report {
		msgs = append(msgs, r.Error())
	}
	text := strings.Join(msgs, "\n")
	for _, want := range []string{"usecase.colorScheme: rotate", "usecase.curve", "the type package of the system boundary S"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected an issue about %s, got\n%s", want, text)
		}
	}
	if len(report) != 3 {
		t.Errorf("expected 3 issues, got\n%s", text)
	}
	if len(g.Clusters) != 1 || g.Nodes[0].Cluster != "S" {
		t.Errorf("expected the boundary S to hold A, got %v %v", g.Clusters, g.Nodes)
	}
	d, _ := Read("usecase-beta\nsystemBoundary S@{ type: package }\nA\nend\n")
	if _, _, err := Graph(d, true); err == nil {
		t.Error("expected an error in the strict mode")
	}
}

func TestUsecaseEmpty(t *testing.T) {
	t.Parallel()
	g, report := stateOf(t, "usecase-beta\n")
	if len(report) > 0 || len(g.Nodes) > 0 || len(g.Edges) > 0 {
		t.Errorf("expected an empty graph, got %v %v", g, report)
	}
}

func TestUsecaseJSONRows(t *testing.T) {
	t.Parallel()
	rows, err := jsonRows(`{"2": "two", "1": "one", "a": {"b": [1, {"c": null}]}, "2": true, "e": [], "f": 1e21}`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r[0]+"="+r[1])
	}
	want := []string{"2=true", "1=one", "a.b[0]=1", "a.b[1].c=null", "e=[]", "f=1e+21"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
	for _, src := range []string{`[1]`, `{"a": 1} x`, `{"a": }`} {
		if _, err := jsonRows(src); err == nil {
			t.Errorf("%s: expected an error", src)
		}
	}
}
