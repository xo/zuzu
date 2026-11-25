package graph

import (
	"slices"
	"strings"
	"testing"
)

// words returns a text with each run of spaces as one space.
func words(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestHTMLLinks(t *testing.T) {
	t.Parallel()
	src := `<table href="a&amp;b" title="t"><tr><td URL="u" tooltip="x" bgcolor="red">1</td><td>2</td><td href="h">3</td></tr></table>`
	out, tables, cells := HTMLLinks(src, false)
	if want := []HTMLLink{{"a&b", "t"}}; !slices.Equal(tables, want) {
		t.Errorf("expected the tables %v, got %v", want, tables)
	}
	if want := []HTMLLink{{"u", "x"}, {}, {"h", ""}}; !slices.Equal(cells, want) {
		t.Errorf("expected the cells %v, got %v", want, cells)
	}
	// Each attribute turns into spaces, so the source keeps its length.
	if len(out) != len(src) || words(out) != `<table ><tr><td bgcolor="red">1</td><td>2</td><td >3</td></tr></table>` {
		t.Errorf("unexpected source %q", out)
	}
	// With fill, a table or a cell with a link and no bgcolor takes a
	// transparent one.
	out, _, _ = HTMLLinks(src, true)
	if want := `<table bgcolor="transparent" ><tr><td bgcolor="red">1</td><td>2</td><td bgcolor="transparent">3</td></tr></table>`; words(out) != want {
		t.Errorf("expected\n%s\ngot\n%s", want, out)
	}
	if c, ok := Color("transparent"); !ok || c == "" {
		t.Errorf("expected transparent to be a color, got %q", c)
	}
	if !htmlLinks(&Graph{Nodes: []*Node{{HTML: src}}}) || htmlLinks(&Graph{Nodes: []*Node{{HTML: "<b>x</b>"}}}) {
		t.Error("expected a graph with a link in a label of HTML to have links, and no other")
	}
}
