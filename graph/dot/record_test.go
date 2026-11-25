package dot

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

func TestRecordFields(t *testing.T) {
	t.Parallel()
	// Each field between bars is a field, with text or not, as in
	// Graphviz 16. A group in braces holds its fields, and an empty group
	// is one empty field. The leaves are the fields in their order.
	for _, tc := range []struct {
		label string
		texts []string
		ports []string
	}{
		{`<f0> a| | |2`, []string{"a", "", "", "2"}, []string{"f0", "", "", ""}},
		{`{a|b}|c`, []string{"a", "b", "c"}, nil},
		{`{a||b}`, []string{"a", "", "b"}, nil},
		{`|a|`, []string{"", "a", ""}, nil},
		{`a||`, []string{"a", "", ""}, nil},
		{`{|}`, []string{"", ""}, nil},
		{`{}`, []string{""}, nil},
		{`a|{}|b`, []string{"a", "", "b"}, nil},
		{`<p>|b`, []string{"", "b"}, []string{"p", ""}},
		{``, []string{""}, nil},
		{`hello\nworld |{ b |{c|<here> d|e}| f}| g | h`, []string{"hello\nworld", "b", "c", "d", "e", "f", "g", "h"}, []string{"", "", "", "here", "", "", "", ""}},
		{`a \| b|\{c\}`, []string{"a | b", "{c}"}, nil},
	} {
		var texts, ports []string
		for _, f := range leaves(record(tc.label, "n")) {
			texts, ports = append(texts, f.Text), append(ports, f.Port)
		}
		if !slices.Equal(texts, tc.texts) {
			t.Errorf("%q: the fields %q, want %q", tc.label, texts, tc.texts)
		}
		if tc.ports != nil && !slices.Equal(ports, tc.ports) {
			t.Errorf("%q: the ports %q, want %q", tc.label, ports, tc.ports)
		}
	}
}

func TestRecordGroups(t *testing.T) {
	t.Parallel()
	// shape writes the fields of a record, a group in braces.
	var shape func(fs []graph.Field) string
	shape = func(fs []graph.Field) string {
		var parts []string
		for _, f := range fs {
			if f.Fields != nil {
				parts = append(parts, "{"+shape(f.Fields)+"}")
				continue
			}
			parts = append(parts, f.Text)
		}
		return strings.Join(parts, "|")
	}
	for label, want := range map[string]string{
		`a|{b|c}|d`:       "a|{b|c}|d",
		`{a|b|c}`:         "{a|b|c}",
		`a|{b|{c|d}|e}`:   "a|{b|{c|d}|e}",
		`x{a|b}y`:         "x|{a|b}|y",
		`{}`:              "{}",
		`\{a\}|{<p> b}`:   "{a}|{b}",
		`{{a|b}|{c|d}}|e`: "{{a|b}|{c|d}}|e",
	} {
		if got := shape(record(label, "n")); got != want {
			t.Errorf("%q: the groups %q, want %q", label, got, want)
		}
	}
}

func TestRecordAlign(t *testing.T) {
	t.Parallel()
	// \l ends a line at the left, \r at the right, and \n or the end of
	// the text in the middle, as in Graphviz.
	for label, want := range map[string][2]string{
		`a\lb\l`:  {"a\nb", "ll"},
		`a\lb`:    {"a\nb", "lc"},
		`a\rb\nc`: {"a\nb\nc", "rcc"},
		`a\nb`:    {"a\nb", ""},
		`a`:       {"a", ""},
	} {
		f := record(label, "n")[0]
		if f.Text != want[0] || f.Align != want[1] {
			t.Errorf("%q: the text %q and the alignment %q, want %q and %q", label, f.Text, f.Align, want[0], want[1])
		}
	}
	g, _, err := Parse(`digraph { a [shape=record, label="left\l|{r\r|c}"] }`, true)
	if err != nil {
		t.Fatal(err)
	}
	if fs := g.Nodes[0].Fields; fs[0].Align != "l" || fs[1].Fields[0].Align != "r" || fs[1].Fields[1].Align != "" {
		t.Errorf("unexpected alignments %+v", fs)
	}
}

func TestHTMLRecordEmptyFields(t *testing.T) {
	t.Parallel()
	// A field of a label of HTML with no text is a field too, and a bar
	// inside a tag is text.
	g, _, err := Parse(`digraph { a [shape=record, label=<a| |<b>x</b>|{c|d}|<font face="x|y">e</font>>] }`, false)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, f := range g.Nodes[0].Fields {
		texts = append(texts, f.Text)
	}
	if want := []string{"a", "", "x", "c", "d", "e"}; !slices.Equal(texts, want) {
		t.Errorf("the fields %q, want %q", texts, want)
	}
}

func TestEmptyLabel(t *testing.T) {
	t.Parallel()
	// label="" draws no text, as in Graphviz. A node with no label draws
	// its id, and a record with label="" has one empty field.
	src := `digraph {
	a [label=""]
	b
	c [label="\N!"]
	d [shape=box, label="\n"]
	e [shape=record, label=""]
	f [shape=record]
	node [label=""]
	g
}`
	g, _, err := Parse(src, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []struct {
		label   string
		noLabel bool
		fields  int
	}{{"", true, 0}, {"b", false, 0}, {"c!", false, 0}, {"", true, 0}, {"", true, 1}, {"f", false, 1}, {"", true, 0}} {
		n := g.Nodes[i]
		if n.Label != want.label || n.NoLabel != want.noLabel || len(n.Fields) != want.fields {
			t.Errorf("node %s: the label %q, NoLabel %v and %d fields, want %q, %v and %d", n.ID, n.Label, n.NoLabel, len(n.Fields), want.label, want.noLabel, want.fields)
		}
	}
}

func TestWeight(t *testing.T) {
	t.Parallel()
	// weight=0 is a weight, as in Graphviz. A weight that is not a number
	// of 0 or more is no weight, and counts as 1.
	g, _, err := Parse(`digraph { a -> b; a -> c [weight=0]; a -> d [weight=5]; a -> e [weight=-1]; a -> f [weight=x] }`, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"none", "0", "5", "none", "none"} {
		got := "none"
		if w := g.Edges[i].Weight; w != nil {
			got = strconv.FormatFloat(*w, 'g', -1, 64)
		}
		if got != want {
			t.Errorf("the edge to %s: the weight %s, want %s", g.Edges[i].Target, got, want)
		}
	}
}
