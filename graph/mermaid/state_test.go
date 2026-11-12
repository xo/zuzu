package mermaid

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// stateOf reads a state diagram in the loose mode.
func stateOf(t *testing.T, src string) (*graph.Graph, vega.Report) {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, report, err := Graph(d, false)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return g, report
}

// summary writes the nodes of a graph as id:shape:label@cluster, the
// clusters as [id:label@parent], and the edges as source>target:label, by
// spaces.
func summary(g *graph.Graph) string {
	var out []string
	for _, n := range g.Nodes {
		label := n.Label
		if len(n.Fields) > 0 {
			var fs []string
			for _, f := range n.Fields {
				fs = append(fs, f.Text)
			}
			label = strings.Join(fs, "|")
		}
		s := n.ID + ":" + n.Shape + ":" + strings.ReplaceAll(label, "\n", "/")
		if n.Cluster != "" {
			s += "@" + n.Cluster
		}
		out = append(out, s)
	}
	for _, c := range g.Clusters {
		s := "[" + c.ID + ":" + c.Label
		if c.Parent != "" {
			s += "@" + c.Parent
		}
		out = append(out, s+"]")
	}
	for _, e := range g.Edges {
		out = append(out, e.Source+">"+e.Target+":"+e.Label)
	}
	return strings.Join(out, " ")
}

func TestStateExample(t *testing.T) {
	t.Parallel()
	// The example of the state diagram that a user of zuzu draws: it must
	// draw with no issue.
	src := "stateDiagram-v2\n  [*] --> Still\n  Still --> [*]\n  Still --> Moving\n  Moving --> Still\n  Moving --> Crash\n  Crash --> [*]\n"
	g, report := stateOf(t, src)
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	exp := "root_start:point: Still:rounded:Still root_end:bullseye: Moving:rounded:Moving Crash:rounded:Crash " +
		"root_start>Still: Still>root_end: Still>Moving: Moving>Still: Moving>Crash: Crash>root_end:"
	if s := summary(g); s != exp {
		t.Errorf("expected\n%s\ngot\n%s", exp, s)
	}
	for _, n := range g.Nodes {
		if strings.HasPrefix(n.ID, "root_") && (n.Width != 14 || n.Height != 14 || n.Style["fillOpacity"] != "1") {
			t.Errorf("%s: expected a filled mark of 14 pixels, got %gx%g %v", n.ID, n.Width, n.Height, n.Style)
		}
	}
	spec, err := graph.Compile(g)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if spec == nil {
		t.Fatal("no spec")
	}
}

func TestStateParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, src, exp string
	}{
		{"states", "s1\ns2:::hot\ns1 --> s2", "s1:rounded:s1 s2:rounded:s2 s1>s2:"},
		{"label", "s1 --> s2 : go on", "s1:rounded:s1 s2:rounded:s2 s1>s2:go on"},
		{"label no space", "s1-->s2:go", "s1:rounded:s1 s2:rounded:s2 s1>s2:go"},
		{"class and label", "s1 --> s2:::hot : go", "s1:rounded:s1 s2:rounded:s2 s1>s2:go"},
		{"as", `state "A long name" as s1`, "s1:rounded:A long name"},
		{"description", "s1 : The first state", "s1:rounded:The first state"},
		{"descriptions", "s1 : one\ns1 : two\ns1 : three", "s1:rounded:one|two/three"},
		{"as and description", "state \"Title\" as s1\ns1 : more", "s1:rounded:Title|more"},
		{"as with description", "state \"Some long name\" as S1: The description", "S1:rounded:Some long name|The description"},
		{"as with two colons", "state \"Long\" as L: one: two", "L:rounded:Long|one"},
		{"state alone", "state a\nb", "b:rounded:b"},
		{"state with text", "state a: text\na --> b", "a:rounded:a b:rounded:b a>b:"},
		{"state with entities", "state f &lt;&lt;fork&gt;&gt;\nf --> a", "f:rounded:f a:rounded:a f>a:"},
		{"brace on the next line", "state A\n{\na\n}", "a:rounded:a@A [A:A]"},
		{"brace and text on the next line", "state \"Big\" as B\n{ b\n}", "b:rounded:b@B [B:Big]"},
		{"break", "s1 : one<br>two", "s1:rounded:one/two"},
		{"choice", "state c <<choice>>\nc --> a", "c:diamond: a:rounded:a c>a:"},
		{"fork and join", "state f <<fork>>\nstate j [[join]]\nf --> j", "f:bar: j:bar: f>j:"},
		{"start and end", "[*] --> a\na --> [*]", "root_start:point: a:rounded:a root_end:bullseye: root_start>a: a>root_end:"},
		{"composite", "[*] --> A\nstate A {\n[*] --> a\na --> [*]\n}\nA --> [*]",
			"root_start:point: A_start:point:@A a:rounded:a@A A_end:bullseye:@A root_end:bullseye: [A:A] root_start>A: A_start>a: a>A_end: A>root_end:"},
		{"composite as", "state \"Big state\" as B {\nb\n}", "b:rounded:b@B [B:Big state]"},
		{"nested", "state A {\nstate B {\nb\n}\n}", "b:rounded:b@B [A:A] [B:B@A]"},
		{"regions", "state A {\na1 --> a2\n--\nb1 --> b2\n}",
			"a1:rounded:a1@divider-id-1 a2:rounded:a2@divider-id-1 b1:rounded:b1@divider-id-2 b2:rounded:b2@divider-id-2 " +
				"[A:A] [divider-id-1:@A] [divider-id-2:@A] a1>a2: b1>b2:"},
		{"region scopes", "state A {\n[*] --> a\n--\n[*] --> b\n}",
			"divider-id-1_start:point:@divider-id-1 a:rounded:a@divider-id-1 divider-id-2_start:point:@divider-id-2 b:rounded:b@divider-id-2 " +
				"[A:A] [divider-id-1:@A] [divider-id-2:@A] divider-id-1_start>a: divider-id-2_start>b:"},
		{"note right", "a\nnote right of a : hello", "a:rounded:a a----note:note:hello a>a----note:"},
		{"note left", "a\nnote left of a\nline one\nline two\nend note", "a----note:note:line one/line two a:rounded:a a----note>a:"},
		{"last note wins", "a\nnote left of a : one\nnote right of a : two", "a:rounded:a a----note:note:two a>a----note:"},
		{"note of composite", "state A {\na\n}\nnote right of A : n", "a:rounded:a@A A----note:note:n [A:A] A>A----note:"},
		{"comment", "a --> b %% a comment\n%% a line\nb", "a:rounded:a b:rounded:b a>b:"},
		{"hide empty", "hide empty description\na", "a:rounded:a"},
		{"accessibility", "accTitle: T\naccDescr {\nline\n}\na", "a:rounded:a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g, report := stateOf(t, "stateDiagram-v2\n"+test.src)
			if len(report) != 0 {
				t.Errorf("expected no issue, got %v", report)
			}
			if s := summary(g); s != test.exp {
				t.Errorf("expected\n%s\ngot\n%s", test.exp, s)
			}
		})
	}
}

func TestStateDirection(t *testing.T) {
	t.Parallel()
	g, _ := stateOf(t, "stateDiagram\ndirection LR\nstate f <<fork>>\nstate A {\ndirection TB\nstate j <<join>>\n}")
	if g.Direction != graph.LR {
		t.Errorf("expected LR, got %s", g.Direction)
	}
	if c := g.Clusters[0]; c.Direction != graph.TB {
		t.Errorf("expected the composite state in TB, got %q", c.Direction)
	}
	size := map[string][2]float64{}
	for _, n := range g.Nodes {
		size[n.ID] = [2]float64{n.Width, n.Height}
	}
	if s := size["f"]; s != [2]float64{10, 70} {
		t.Errorf("expected a fork of 10x70 across ranks that run to the right, got %v", s)
	}
	if s := size["j"]; s != [2]float64{70, 10} {
		t.Errorf("expected a join of 70x10 across ranks that run down, got %v", s)
	}
}

func TestStateStyles(t *testing.T) {
	t.Parallel()
	src := `stateDiagram-v2
    classDef hot fill:#f96,stroke-width:2px
    classDef cold,calm fill:#9cf
    a:::hot --> b
    class b cold
    class c, d calm
    style a stroke:#c00
    state A {
        x
    }
    class A hot
    accTitle: Doors
    accDescr: The states of a door
`
	g, report := stateOf(t, src)
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	style := map[string]map[string]string{}
	for _, n := range g.Nodes {
		style[n.ID] = n.Style
	}
	for _, c := range g.Clusters {
		style[c.ID] = c.Style
	}
	tests := []struct{ id, key, exp string }{
		{"a", "fill", "#f96"}, {"a", "strokeWidth", "2"}, {"a", "stroke", "#c00"},
		{"b", "fill", "#9cf"}, {"c", "fill", "#9cf"}, {"d", "fill", "#9cf"}, {"A", "fill", "#f96"},
	}
	for _, test := range tests {
		if v := style[test.id][test.key]; v != test.exp {
			t.Errorf("%s: expected %s %s, got %q", test.id, test.key, test.exp, v)
		}
	}
	if g.Description != "Doors The states of a door" {
		t.Errorf("expected the description, got %q", g.Description)
	}
}

func TestStateIssues(t *testing.T) {
	t.Parallel()
	tests := []struct{ src, exp string }{
		{"scale 350 width\na", "the scale of a state diagram"},
		{"click a call cb()\na", "the callback cb(), since no script runs"},
		{`note "floating" as N1`, "a note that floats, with no state"},
		{"state h <<history>>", "the kind of state history, drawn as a state"},
		{"a\nclassDef x text-decoration:underline", "the style text-decoration"},
		{"---\nconfig:\n  theme: sepia\n---\nstateDiagram-v2\na", "the theme of Mermaid, drawn in the look of zuzu"},
		{"%%{init: {'state': {'nodeSpacing': 10}}}%%\nstateDiagram-v2\na", "the setting state.nodeSpacing"},
	}
	for _, test := range tests {
		t.Run(test.exp, func(t *testing.T) {
			t.Parallel()
			src := test.src
			if !strings.Contains(src, "stateDiagram") {
				src = "stateDiagram-v2\n" + src
			}
			d, err := Read(src)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			_, report, err := Graph(d, false)
			if err != nil {
				t.Fatalf("loose: %v", err)
			}
			if !slices.ContainsFunc(report, func(i vega.Issue) bool { return i.Message == test.exp }) {
				t.Errorf("expected the issue %q, got %v", test.exp, report)
			}
			if _, _, err := Graph(d, true); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("expected an unsupported error in the strict mode, got %v", err)
			}
		})
	}
}

func TestStateErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"state A {\na",
		"a\n}",
		"a\n--",
		"note right of a\ntext",
		"note above a : x",
		"a b c",
		"state A\n\n{\na\n}",
		"state a b {\n}",
		`state "x"`,
		"[*]",
		"state f <<fork>>\nstate f {\nx\n}",
		"note right of x : hello\nstate x <<choice>>\nstate x {\n}",
		"direction up",
	}
	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			d, err := Read("stateDiagram-v2\n" + src)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if _, _, err := Graph(d, false); !errors.Is(err, ErrSyntax) {
				t.Errorf("expected a syntax error, got %v", err)
			}
		})
	}
	d, err := Read("stateDiagram-v2 LR\na")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Graph(d, false); !errors.Is(err, ErrSyntax) {
		t.Errorf("expected a syntax error for a word after the kind, got %v", err)
	}
}

func TestStateCompile(t *testing.T) {
	t.Parallel()
	g, _ := stateOf(t, "stateDiagram-v2\naccTitle: Doors\n[*] --> a\nstate c <<choice>>\na --> c\nc --> [*]")
	spec, err := graph.Compile(g)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"description":"Doors"`) {
		t.Error("expected the description in the spec")
	}
	// A node of a fixed size and no label draws no text, so neither the
	// start nor the choice shows its id.
	for _, id := range []string{"root_start", "c"} {
		if strings.Contains(s, fmt.Sprintf(`"label":%q`, id)) {
			t.Errorf("expected no label %s", id)
		}
	}
}
