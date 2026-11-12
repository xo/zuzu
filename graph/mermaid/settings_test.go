package mermaid

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

func TestFrontMatterComments(t *testing.T) {
	t.Parallel()
	src := "---\n# a comment\ntitle: Plan # the title\ndisplayMode: compact     #gantt specific\nconfig: # the config\n" +
		"  gantt:\n    topAxis: true  #false\n    axisFormat: \"%d # day\" # quoted\n    weekday: 'it''s # x' # single\n" +
		"    tickInterval: 1day#no space\n    empty: # nothing\n---\ngantt\n"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Plan" {
		t.Errorf("title %q", d.Title)
	}
	if d.Config["displayMode"] != "compact" {
		t.Errorf("displayMode %q", d.Config["displayMode"])
	}
	g, _ := d.Config["gantt"].(map[string]any)
	for k, exp := range map[string]any{
		"topAxis": true, "axisFormat": "%d # day", "weekday": "it''s # x", "tickInterval": "1day#no space",
	} {
		if g[k] != exp {
			t.Errorf("gantt.%s: expected %q, got %q", k, exp, g[k])
		}
	}
	if v, ok := g["empty"]; !ok || v == nil {
		t.Errorf("gantt.empty: %v", v)
	}
}

func TestDirectiveConfigKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, section string
	}{
		{"%%{init: {'config': {'wrap': true}}}%%\nsequenceDiagram\n  A->>B: hi\n", "sequence"},
		{"%%{init: {'config': {'wrap': true}}}%%\ngraph LR\n  A\n", "flowchart"},
		{"%%{init: {'config': {'wrap': true}}}%%\nflowchart LR\n  A\n", "flowchart"},
		{"%%{init: {'config': {'wrap': true}}}%%\nerDiagram\n  A\n", "er"},
		{"%%{init: {'config': {'wrap': true}}}%%\nclassDiagram\n  class A\n", "classDiagram"},
		{"%%{init: {'config': {'wrap': true}}}%%\nstateDiagram-v2\n  A\n", "stateDiagram"},
		{"%%{init: {'config': {'wrap': true}}}%%\nxychart-beta\n  bar [1]\n", "xychart"},
		{"%%{init: {'config': {'wrap': true}}}%%\ngantt\n", "gantt"},
	}
	for _, test := range tests {
		d, err := Read(test.src)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := d.Config["config"]; ok {
			t.Errorf("%q: the key config stays: %v", test.src, d.Config)
		}
		if m, _ := d.Config[test.section].(map[string]any); m["wrap"] != true {
			t.Errorf("%q: expected %s.wrap, got %v", test.src, test.section, d.Config)
		}
	}
	// The key config of a directive wins over the front matter, and merges
	// with it.
	d, err := Read("---\nconfig:\n  sequence:\n    wrap: false\n    mirrorActors: false\n---\n" +
		"%%{init: {'config': {'wrap': true}}}%%\nsequenceDiagram\n  A->>B: hi\n")
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := d.Config["sequence"].(map[string]any); m["wrap"] != true || m["mirrorActors"] != false {
		t.Errorf("expected sequence.wrap true and mirrorActors false, got %v", d.Config)
	}
}

// strictCheck reads a source of Mermaid in the strict mode, and returns
// the error.
func strictCheck(t *testing.T, src string) error {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	if IsChart(d.Kind) {
		_, _, _, err = Chart(d, true)
	} else {
		_, _, err = Graph(d, true)
	}
	return err
}

func TestInertSettings(t *testing.T) {
	t.Parallel()
	const inertConfig = "  logLevel: debug\n  loglevel: 1\n  securityLevel: loose\n  handDrawnSeed: 3\n  useMaxWidth: false\n"
	bodies := map[string]string{
		"flowchart":       "flowchart LR\n  A --> B\n",
		"stateDiagram-v2": "stateDiagram-v2\n  A --> B\n",
		"classDiagram":    "classDiagram\n  A --> B\n",
		"erDiagram":       "erDiagram\n  A ||--o{ B : has\n",
		"sequence":        "sequenceDiagram\n  A->>B: hi\n",
		"pie":             "pie\n  \"a\" : 1\n",
		"gantt":           "gantt\n  dateFormat YYYY-MM-DD\n  section S\n  T : 2024-01-01, 1d\n",
		"sankey":          "sankey\nA,B,1\n",
		"journey":         "journey\n  section S\n    T: 5: Me\n",
		"architecture":    "architecture-beta\n  service a(server)[A]\n",
		"gitGraph":        "gitGraph\n  commit\n",
		"block":           "block-beta\n  a b\n",
		"timeline":        "timeline\n  2020 : a\n",
		"c4":              "C4Context\n  Person(a, \"A\")\n",
		"ishikawa":        "ishikawa-beta\n  P\n  A\n",
		"cynefin":         "cynefin-beta\n  complex \"a\"\n",
		"info":            "info\n",
		"kanban":          "kanban\n  a[A]\n    b[B]\n",
		"packet":          "packet\n  0-3: \"a\"\n",
		"railroad":        "railroad-ebnf-beta\n  a = b ;\n",
		"treeView":        "treeView-beta\n  a\n",
	}
	sections := map[string]string{
		"flowchart": "flowchart", "stateDiagram-v2": "state", "classDiagram": "class", "erDiagram": "er",
		"sequence": "sequence", "pie": "pie", "gantt": "gantt", "sankey": "sankey", "journey": "journey",
		"architecture": "architecture", "gitGraph": "gitGraph", "block": "block",
		"timeline": "timeline",
		"c4":       "c4",
		"ishikawa": "ishikawa",
		"cynefin":  "cynefin", "info": "info",
		"kanban":   "kanban",
		"packet":   "packet",
		"railroad": "railroad",
		"treeView": "treeView",
	}
	for name, body := range bodies {
		src := "---\nconfig:\n" + inertConfig + "  " + sections[name] + ":\n    useMaxWidth: false\n---\n" + body
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, src := range []string{
		// htmlLabels changes nothing in a chart.
		"---\nconfig:\n  htmlLabels: false\n  flowchart:\n    htmlLabels: false\n---\nsequenceDiagram\n  A->>B: hi\n",
		"---\nconfig:\n  htmlLabels: false\n---\npie\n  \"a\" : 1\n",
		// Mermaid 12 does not read class.htmlLabels, and flowchart.htmlLabels
		// is the old form of htmlLabels in every graph.
		"---\nconfig:\n  class:\n    htmlLabels: false\n  flowchart:\n    htmlLabels: false\n---\nclassDiagram\n  A --> B\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nerDiagram\n  A ||--o{ B : has\n",
		"---\nconfig:\n  architecture:\n    seed: 7\n    randomize: true\n---\narchitecture-beta\n  service a(server)[A]\n",
		// A mindmap reads htmlLabels, and flowchart.htmlLabels in its
		// place.
		"---\nconfig:\n  htmlLabels: false\n---\nmindmap\n  root\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nmindmap\n  root\n",
		// The key config of a directive that holds the section of the
		// kind makes sequence.sequence, where Mermaid has no setting.
		"%%{init: {'config': {'sequence': {'hideUnusedParticipants': true}}}}%%\nsequenceDiagram\n  participant a\n",
		"%%{init: {'config': {'flowchart': {'curve': 'linear'}}}}%%\nflowchart LR\n  A --> B\n",
	} {
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	for _, src := range []string{
		// A section of another kind holds settings that change the image.
		"---\nconfig:\n  flowchart:\n    curve: linear\n---\nclassDiagram\n  A --> B\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: maybe\n---\nclassDiagram\n  A --> B\n",
	} {
		if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported setting, got %v", src, err)
		}
	}
}

func TestHints(t *testing.T) {
	t.Parallel()
	bodies := []string{
		"flowchart LR\n  A --> B\n",
		"stateDiagram-v2\n  A --> B\n",
		"classDiagram\n  A --> B\n",
		"erDiagram\n  A ||--o{ B : has\n",
		"sequenceDiagram\n  A->>B: hi\n",
		"pie\n  \"a\" : 1\n",
		"gantt\n  dateFormat YYYY-MM-DD\n  section S\n  T : 2024-01-01, 1d\n",
		"sankey\nA,B,1\n",
		"journey\n  section S\n    T: 5: Me\n",
		"architecture-beta\n  service a(server)[A]\n",
		"mindmap\n  root\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\n",
		"treemap-beta\n\"A\": 1\n",
		"quadrantChart\n  A: [0.5, 0.5]\n",
		"xychart-beta\n  bar [1, 2]\n",
		"C4Context\n  Person(a, \"A\")\n",
		"ishikawa-beta\n  P\n  A\n",
		"cynefin-beta\n  complex \"a\"\n", "info\n",
		"railroad-peg-beta\n  A <- B ;\n",
		"treeView-beta\n  a\n",
	}
	// The look handDrawn and the layout elk are hints, in the config or
	// at the top of the front matter, with a description (D32).
	for _, fm := range []string{
		"config:\n  look: handDrawn\n  layout: elk\n",
		"look: handDrawn\nlayout: elk\ndescription: A diagram\n",
	} {
		for _, body := range bodies {
			src := "---\n" + fm + "---\n" + body
			if err := strictCheck(t, src); err != nil {
				t.Errorf("%q: %v", src, err)
			}
		}
	}
	// Another layout stays an issue.
	src := "---\nconfig:\n  layout: tidy\n---\nflowchart LR\n  A --> B\n"
	if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("%q: expected an unsupported setting, got %v", src, err)
	}
}

// TestThemes reads each theme of Mermaid 12.3.0 with no issue in every
// kind, in the front matter and in a directive, and checks the look that
// Look gives for it (D32). Another theme, themeVariables and themeCSS stay
// issues.
func TestThemes(t *testing.T) {
	t.Parallel()
	bodies := []string{
		"flowchart LR\n  A --> B\n",
		"stateDiagram-v2\n  A --> B\n",
		"classDiagram\n  A --> B\n",
		"erDiagram\n  A ||--o{ B : has\n",
		"requirementDiagram\n  requirement r {\n    id: 1\n  }\n",
		"sequenceDiagram\n  A->>B: hi\n",
		"pie\n  \"a\" : 1\n",
		"sankey\nA,B,1\n",
		"journey\n  section S\n    T: 5: Me\n",
		"architecture-beta\n  service a(server)[A]\n",
		"mindmap\n  root\n",
		"wardley-beta\ncomponent A [0.5, 0.5]\n",
		"radar-beta\naxis A, B, C\ncurve c{1, 2, 3}\n",
		"treemap-beta\n\"A\": 1\n",
		"quadrantChart\n  A: [0.5, 0.5]\n",
		"xychart-beta\n  bar [1, 2]\n",
		"gitGraph\n  commit\n",
		"block-beta\n  a b\n",
		"treeView-beta\n  a\n",
	}
	looks := map[string]string{
		"default": `zuzu.scheme:"mermaid-default"`, "base": `zuzu.scheme:"mermaid-base"`,
		"neutral": `zuzu.scheme:"mermaid-neutral"`, "forest": `zuzu.scheme:"mermaid-forest"`,
		"neo": `zuzu.scheme:"mermaid-neo"`, "redux": `zuzu.scheme:"mermaid-redux"`,
		"redux-color": `zuzu.scheme:"mermaid-redux-color"`,
		"dark":        `zuzu.mode:"dark"`, "neo-dark": `zuzu.mode:"dark"`,
		"redux-dark": `zuzu.mode:"dark"`, "redux-dark-color": `zuzu.mode:"dark"`,
	}
	for name, want := range looks {
		for _, head := range []string{
			"---\nconfig:\n  theme: " + name + "\n---\n",
			"%%{init: {'theme': '" + name + "'}}%%\n",
		} {
			for _, body := range bodies {
				if err := strictCheck(t, head+body); err != nil {
					t.Errorf("%q: %v", head+body, err)
				}
			}
			d, err := Read(head + bodies[0])
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for k, v := range Look(d) {
				got = append(got, k+":"+string(v))
			}
			if len(got) != 1 || got[0] != want {
				t.Errorf("%s: the look %v, want %s", name, got, want)
			}
		}
	}
	for _, body := range bodies {
		for _, head := range []string{
			"---\nconfig:\n  theme: sepia\n---\n",
			"---\nconfig:\n  theme: Forest\n---\n",
			"---\nconfig:\n  theme: forest\n  themeVariables:\n    primaryColor: '#ff0000'\n---\n",
			"---\nconfig:\n  theme: forest\n  themeCSS: '.node rect { fill: red; }'\n---\n",
		} {
			if err := strictCheck(t, head+body); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("%q: expected an issue, got %v", head+body, err)
			}
		}
	}
	d, err := Read("---\nconfig:\n  theme: sepia\n---\n" + bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if l := Look(d); l != nil {
		t.Errorf("the theme sepia: the look %v", l)
	}
}

func TestOldHTMLLabels(t *testing.T) {
	t.Parallel()
	for src, exp := range map[string]bool{
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nclassDiagram\n  A --> B\n":                     true,
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nflowchart LR\n  A --> B\n":                     true,
		"---\nconfig:\n  htmlLabels: true\n  flowchart:\n    htmlLabels: false\n---\nflowchart LR\n  A --> B\n": false,
		"flowchart LR\n  A --> B\n": false,
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := Graph(d, true)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if g.BreakWords != exp {
			t.Errorf("%q: expected BreakWords %v, got %v", src, exp, g.BreakWords)
		}
	}
}

func TestFlowchartSpacing(t *testing.T) {
	t.Parallel()
	src := "---\ntitle: T\nconfig:\n  flowchart:\n    nodeSpacing: 100\n    rankSpacing: '25'\n    diagramPadding: 0\n" +
		"    titleTopMargin: 50\n    subGraphTitleMargin:\n      top: 6\n      bottom: 9\n---\nflowchart LR\n  A --> B\n"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.NodeSep != 48 || g.RankSep != 20 {
		t.Errorf("expected the spaces 48 and 20, got %g and %g", g.NodeSep, g.RankSep)
	}
	if g.Padding == nil || *g.Padding != 0 {
		t.Errorf("expected the padding 0, got %v", g.Padding)
	}
	if g.TitleOffset == nil || *g.TitleOffset != 20 {
		t.Errorf("expected the title offset 20, got %v", g.TitleOffset)
	}
	if g.ClusterTitleTop != 6 || g.ClusterTitleBottom != 9 {
		t.Errorf("expected the title margins 6 and 9, got %g and %g", g.ClusterTitleTop, g.ClusterTitleBottom)
	}
	if _, err := graph.Compile(g); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{
		"---\nconfig:\n  flowchart:\n    nodeSpacing: -1\n---\nflowchart LR\n  A\n",
		"---\nconfig:\n  flowchart:\n    diagramPadding: wide\n---\nflowchart LR\n  A\n",
		"---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      left: 3\n---\nflowchart LR\n  A\n",
		"---\nconfig:\n  flowchart:\n    inheritDir: maybe\n---\nflowchart LR\n  A\n",
		"---\nconfig:\n  flowchart:\n    minNodeWidth: 120\n---\nflowchart LR\n  A\n",
	} {
		if err := strictCheck(t, src); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported setting, got %v", src, err)
		}
	}
}

func TestInheritDir(t *testing.T) {
	t.Parallel()
	body := "flowchart LR\n  subgraph O\n    direction TB\n    subgraph I\n      a --> b\n    end\n  end\n"
	for cfg, exp := range map[string]string{
		"": "",
		"%%{init: {'flowchart': {'inheritDir': false}}}%%\n": "",
		"%%{init: {'flowchart': {'inheritDir': true}}}%%\n":  graph.LR,
		"%%{init: {'config': {'inheritDir': true}}}%%\n":     graph.LR,
	} {
		d, err := Read(cfg + body)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := Graph(d, true)
		if err != nil {
			t.Fatal(err)
		}
		dirs := map[string]string{}
		for _, c := range g.Clusters {
			dirs[c.ID] = c.Direction
		}
		if dirs["O"] != graph.TB || dirs["I"] != exp {
			t.Errorf("%q: expected O TB and I %q, got %v", cfg, exp, dirs)
		}
	}
}

func TestClassNamespaces(t *testing.T) {
	t.Parallel()
	body := "classDiagram\n  namespace A.B.C {\n    class X\n  }\n  namespace A {\n    class Y\n  }\n  namespace A.D {\n    class Z\n  }\n"
	type cl struct{ label, parent string }
	for cfg, exp := range map[string]map[string]cl{
		"": {
			"A": {"A", ""}, "A.B": {"B", "A"}, "A.B.C": {"C", "A.B"}, "A.D": {"D", "A"},
		},
		"---\nconfig:\n  class:\n    hierarchicalNamespaces: false\n---\n": {
			"A": {"A", ""}, "A.B.C": {"A.B.C", ""}, "A.D": {"A.D", ""},
		},
	} {
		d, err := Read(cfg + body)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := Graph(d, true)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]cl{}
		for _, c := range g.Clusters {
			got[c.ID] = cl{c.Label, c.Parent}
		}
		if len(got) != len(exp) {
			t.Errorf("%q: expected %v, got %v", cfg, exp, got)
		}
		for id, c := range exp {
			if got[id] != c {
				t.Errorf("%q: cluster %s: expected %v, got %v", cfg, id, c, got[id])
			}
		}
		nodes := map[string]string{}
		for _, n := range g.Nodes {
			nodes[n.ID] = n.Cluster
		}
		if nodes["X"] != "A.B.C" || nodes["Y"] != "A" || nodes["Z"] != "A.D" {
			t.Errorf("%q: the clusters of the classes %v", cfg, nodes)
		}
	}
	if err := strictCheck(t, "---\nconfig:\n  class:\n    hierarchicalNamespaces: 1\n---\nclassDiagram\n  class A\n"); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("expected an unsupported value, got %v", err)
	}
}

// TestReportOrder checks that the issues of the settings, and the error of
// the strict mode, are the same on each run, though the config is a map.
func TestReportOrder(t *testing.T) {
	t.Parallel()
	const cfg = "---\nconfig:\n  fontFamily: a\n  fontSize: 2\n  zeta: 1\n  alpha: 2\n" +
		"  flowchart:\n    minNodeWidth: 1\n    padding: 2\n    arrowMarkerAbsolute: true\n" +
		"  class:\n    padding: 1\n    dividerMargin: 2\n  state:\n    padding: 1\n    textHeight: 2\n" +
		"  er:\n    fontSize: 1\n    minEntityWidth: 2\n  sequence:\n    actorFontFamily: a\n    noteFontFamily: b\n" +
		"  sankey:\n    a: 1\n    b: 2\n---\n"
	for _, body := range []string{
		"flowchart LR\n  A --> B\n",
		"classDiagram\n  A --> B\n",
		"stateDiagram-v2\n  A --> B\n",
		"erDiagram\n  A ||--o{ B : has\n",
		"sequenceDiagram\n  A->>B: hi\n",
		"sankey\nA,B,1\n",
	} {
		var first []string
		var firstErr string
		for i := range 50 {
			d, err := Read(cfg + body)
			if err != nil {
				t.Fatal(err)
			}
			var report vega.Report
			var strictErr error
			if isGraph(d.Kind) {
				_, report, _ = Graph(d, false)
				_, _, strictErr = Graph(d, true)
			} else {
				_, _, report, _ = Chart(d, false)
				_, _, _, strictErr = Chart(d, true)
			}
			var got []string
			for _, r := range report {
				got = append(got, r.Error())
			}
			e := fmt.Sprint(strictErr)
			if i == 0 {
				first, firstErr = got, e
				if len(got) < 10 {
					t.Errorf("%s: expected 10 issues or more, got %v", d.Kind, got)
				}
				continue
			}
			if !slices.Equal(got, first) || e != firstErr {
				t.Fatalf("%s: the report changed between runs:\n%v, %s\n%v, %s", d.Kind, first, firstErr, got, e)
			}
		}
	}
}

// TestLinkStyleOrder checks that a flowchart with two linkStyles past its
// links names the same link on each run.
func TestLinkStyleOrder(t *testing.T) {
	t.Parallel()
	src := "flowchart LR\n  A --> B\n  linkStyle 9 stroke:red\n  linkStyle 3 stroke:red\n  linkStyle 5 stroke:red\n"
	for range 50 {
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Graph(d, false)
		if err == nil || !strings.Contains(err.Error(), "link 3,") {
			t.Fatalf("expected the error of the link 3, got %v", err)
		}
	}
}
