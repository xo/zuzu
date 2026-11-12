package mermaid

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// The tests of this file check the parts of Mermaid where zuzu follows
// what mmdc 12.3.0 draws.

// reportOf reads a source in the loose mode, and returns each issue of its
// report as its path and its message.
func reportOf(t *testing.T, src string) []string {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("%q: read: %v", src, err)
	}
	var report vega.Report
	if isGraph(d.Kind) {
		_, report, err = Graph(d, false)
	} else {
		_, _, report, err = Chart(d, false)
	}
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var out []string
	for _, i := range report {
		out = append(out, i.Path+": "+i.Message)
	}
	return out
}

func TestFrontMatterUnread(t *testing.T) {
	t.Parallel()
	const body = "pie\n  \"a\" : 1\n"
	tests := []struct{ fm, exp string }{
		{"tags:\n  - a\n  - b\n", "line 2: the list of YAML under tags"},
		{"tags:\n- a\n- b\ntitle: T\n", "line 2: the list of YAML under tags"},
		{"- a\ntitle: T\n", "line 2: a list of YAML in the front matter"},
		{"title: T\nplain words\n", `line 3: the text "plain words" of the front matter, which is not a key of YAML`},
		{"config:\n  pie:\n    textPosition:\n      - 1\n", "line 4: the list of YAML under config.pie.textPosition"},
		{"config:\n  pie: [1, 2]\n", "line 3: the list of YAML under config.pie"},
		{"config:\n  pie: {a: [1}\n", "line 3: the map of YAML in braces under config.pie"},
		{"config: dark\n", "line 2: the config of the front matter, which is not a map of YAML"},
		{"title:\n  a: b\n", "line 2: the title of the front matter, which is not a text"},
		{"gantt:\n  useWidth: 400\n", "line 2: the key gantt of the front matter, which Mermaid ignores"},
	}
	for _, test := range tests {
		src := "---\n" + test.fm + "---\n" + body
		got := reportOf(t, src)
		if !slices.Contains(got, test.exp+"") {
			t.Errorf("%q: expected %q, got %q", src, test.exp, got)
		}
		if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported error in the strict mode, got %v", src, err)
		}
	}
}

func TestFrontMatterRead(t *testing.T) {
	t.Parallel()
	// A map in braces reads as a map, a text in quotes runs to its closing
	// quote at any indent, a number is a title, and a hint at the top
	// level is no issue (D32).
	src := "---\ntitle: 2024\nlook: handDrawn\nconfig: {pie: {textPosition: 0.5}}\ndescription: \"one\n  two\n\"\n---\npie\n  \"a\" : 1\n"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "2024" {
		t.Errorf("expected the title 2024, got %q", d.Title)
	}
	if v, _ := configNumber(d.Config, "pie", "textPosition"); v != 0.5 {
		t.Errorf("expected pie.textPosition 0.5, got %v", d.Config)
	}
	if strings.TrimSpace(d.Description) != "one two" {
		t.Errorf("expected the description one two, got %q", d.Description)
	}
	if len(d.unread) != 0 {
		t.Errorf("expected no part that zuzu does not read, got %v", d.unread)
	}
	if err := strictCheck(t, src); err != nil {
		t.Errorf("%q: %v", src, err)
	}
}

func TestWrapDirective(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"%%{wrap}%%\nsequenceDiagram\n  A->>B: hi\n",
		"%%{init: {'wrap': false}}%%\n%%{ wrap }%%\nsequenceDiagram\n  A->>B: hi\n",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if d.Config["wrap"] != true {
			t.Errorf("%q: expected wrap, got %v", src, d.Config)
		}
	}
	// Only the sequence diagram reads wrap. It changes nothing in the
	// other kinds.
	for _, src := range []string{
		"%%{wrap}%%\nflowchart LR\n  A --> B\n",
		"%%{wrap}%%\nstateDiagram-v2\n  A --> B\n",
		"%%{wrap}%%\npie\n  \"a\" : 1\n",
		"%%{wrap}%%\ngantt\n  dateFormat YYYY-MM-DD\n  T : 2024-01-01, 1d\n",
	} {
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestTopSpacing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src        string
		node, rank float64
	}{
		// The top level wins over the section flowchart.
		{"---\nconfig:\n  nodeSpacing: 100\n  rankSpacing: 25\n  flowchart:\n    nodeSpacing: 10\n---\nflowchart LR\n  A --> B\n", 48, 20},
		// 0 keeps the space of the section, as in Mermaid.
		{"---\nconfig:\n  nodeSpacing: 0\n  flowchart:\n    nodeSpacing: 25\n---\nflowchart LR\n  A --> B\n", 12, 0},
		{"---\nconfig:\n  nodeSpacing: 100\n---\nstateDiagram-v2\n  A --> B\n", 48, 0},
		{"---\nconfig:\n  rankSpacing: '100'\n---\nclassDiagram\n  A --> B\n", 0, 80},
		// The defaults of an entity relationship diagram are 140 and 80.
		{"---\nconfig:\n  nodeSpacing: 280\n  rankSpacing: 40\n---\nerDiagram\n  A ||--o{ B : has\n", 48, 20},
	}
	for _, test := range tests {
		d, err := Read(test.src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, true)
		if err != nil {
			t.Fatalf("%q: %v", test.src, err)
		}
		if len(report) != 0 {
			t.Errorf("%q: expected no issue, got %v", test.src, report)
		}
		if g.NodeSep != test.node || g.RankSep != test.rank {
			t.Errorf("%q: expected the spaces %v and %v, got %v and %v", test.src, test.node, test.rank, g.NodeSep, g.RankSep)
		}
	}
	src := "---\nconfig:\n  nodeSpacing: wide\n---\nflowchart LR\n  A --> B\n"
	if got := reportOf(t, src); !slices.Contains(got, "config: the setting nodeSpacing: wide") {
		t.Errorf("expected an issue of nodeSpacing, got %q", got)
	}
}

func TestLayoutHints(t *testing.T) {
	t.Parallel()
	bodies := []string{"flowchart LR\n  A --> B\n", "stateDiagram-v2\n  A --> B\n", "pie\n  \"a\" : 1\n", "mindmap\n  root\n"}
	for _, cfg := range []string{
		"  layout: elk.box\n", "  layout: elk.rectpacking\n", "  layout: elk.mrtree\n", "  layout: dagre\n",
		"  elk:\n    mergeEdges: true\n    keepEntryNodeOnTop: true\n",
	} {
		for _, body := range bodies {
			src := "---\nconfig:\n" + cfg + "---\n" + body
			if err := strictCheck(t, src); err != nil {
				t.Errorf("%q: %v", src, err)
			}
		}
	}
	// Mermaid 12.3 has no layout elk.layered.
	src := "---\nconfig:\n  layout: elk.layered\n---\nflowchart LR\n  A --> B\n"
	if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("%q: expected an unsupported setting, got %v", src, err)
	}
}

func TestGanttSettings(t *testing.T) {
	t.Parallel()
	const body = "gantt\n  dateFormat YYYY-MM-DD\n  T : 2024-01-01, 1d\n"
	// A setting of the top level of the config is an issue, unless it
	// changes nothing.
	src := "---\nconfig:\n  fontFamily: Arial\n  pie:\n    textPosition: 0.5\n---\n" + body
	if got := reportOf(t, src); !slices.Equal(got, []string{"config: the setting pie.textPosition"}) {
		t.Errorf("expected one issue of pie.textPosition, got %q", got)
	}
	// Mermaid has no setting gantt.compact. useWidth is the width of the
	// gantt with leftPadding and rightPadding.
	g, c, err := ganttOf(t, "---\nconfig:\n  gantt:\n    compact: true\n    useWidth: 400\n---\n"+body, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.width != 250 || len(c.issues.Report) != 0 {
		t.Errorf("expected a time axis of 250 and no issue, got %v and %v", g.width, c.issues.Report)
	}
	g, _, err = ganttOf(t, "---\nconfig:\n  gantt:\n    rightPadding: 0\n    useWidth: 400\n---\n"+body, true)
	if err != nil || g.width != 325 {
		t.Errorf("expected a time axis of 325, got %v, %v", g, err)
	}
	if _, _, err := ganttOf(t, "---\nconfig:\n  gantt:\n    useWidth: 100\n---\n"+body, true); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("expected an issue of useWidth, got %v", err)
	}
	// Mermaid does not read a text after the word gantt.
	if _, _, err := ganttOf(t, "gantt foo\n  dateFormat YYYY-MM-DD\n  T : 2024-01-01, 1d\n", false); !errors.Is(err, ErrSyntax) {
		t.Errorf("expected a syntax error, got %v", err)
	}
}

func TestFlowchartClasses(t *testing.T) {
	t.Parallel()
	src := `flowchart LR
  direction TB
  classDef calm fill:#9cf,stroke-dasharray: 9\,5
  class C calm
  A --> B
  B --> C
  class B calm
  class Z calm
  subgraph S
    D
  end
  class S calm
`
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	// A direction outside a subgraph changes nothing in Mermaid 12.3.
	if g.Direction != graph.LR {
		t.Errorf("expected the direction LR, got %s", g.Direction)
	}
	var ids []string
	style := map[string]map[string]string{}
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
		style[n.ID] = n.Style
	}
	if !slices.Equal(ids, []string{"A", "B", "C", "D"}) {
		t.Errorf("expected the nodes A, B, C and D, got %v", ids)
	}
	if style["B"]["strokeDash"] != "9,5" || style["B"]["fill"] != "#9cf" {
		t.Errorf("expected the dash 9,5 and the fill of the class on B, got %v", style["B"])
	}
	// A class before the node is lost, as in Mermaid.
	if style["C"]["fill"] != "" {
		t.Errorf("expected no class on C, got %v", style["C"])
	}
	// A part of a style with no colon is an issue.
	src = "flowchart LR\n  classDef odd fill#969,stroke:#f00\n  A:::odd\n"
	if got := reportOf(t, src); !slices.Contains(got, `line 2: the style "fill#969", which has no colon`) {
		t.Errorf("expected an issue of fill#969, got %q", got)
	}
}

func TestStateMermaidParts(t *testing.T) {
	t.Parallel()
	src := `stateDiagram-v2
  classDef hot fill:#f96
  class c hot
  state h <<history>>
  state "x" as s1:::hot
  state "y" as s2:
  a --> c
  a --> s1
  a --> s2
  class end hot
`
	g, report := stateOf(t, src)
	if len(report) != 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	nodes := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	// <<history>> and a class make no state.
	for _, id := range []string{"h", "end"} {
		if nodes[id] != nil {
			t.Errorf("expected no state %s", id)
		}
	}
	// A class before the state applies to it.
	if nodes["c"] == nil || nodes["c"].Style["fill"] != "#f96" {
		t.Errorf("expected the class hot on c, got %v", nodes["c"])
	}
	// A colon after the id adds an empty description, and :::hot is no
	// class there.
	for _, id := range []string{"s1", "s2"} {
		n := nodes[id]
		if n == nil || len(n.Fields) != 2 || n.Fields[1].Text != "" || n.Style["fill"] != "" {
			t.Errorf("expected %s with an empty description and no class, got %+v", id, n)
		}
	}
}

func TestRadarAxes(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"radar-beta\n  axis a, b\n  curve c{1, 2}\n",
		"radar-beta\n  axis a\n  curve c{1}\n  graticule polygon\n",
		"radar-beta\n  title Nothing\n",
	} {
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestMindmapHTMLLabels(t *testing.T) {
	t.Parallel()
	m := text.NewEstimate()
	const word = "Supercalifragilisticexpialidocious"
	runs := breakRuns([]textRun{{text: "a " + word + " b"}}, 12, 400, 60, m)
	var parts []string
	for _, r := range runs {
		if r.text == "\n" {
			continue
		}
		parts = append(parts, r.text)
		if w := m.Width(textFont(12, 400, r), strings.TrimSpace(r.text[strings.LastIndex(r.text, " ")+1:])); w > 60 {
			t.Errorf("expected parts no wider than 60, got %q of %v", r.text, w)
		}
	}
	if got := strings.ReplaceAll(strings.Join(parts, ""), " ", ""); got != "a"+word+"b" {
		t.Errorf("expected the whole text in parts, got %q", parts)
	}
	src := "---\nconfig:\n  htmlLabels: maybe\n---\nmindmap\n  root\n"
	if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("%q: expected an unsupported setting, got %v", src, err)
	}
}
