package mermaid

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// wrapsOf reads a diagram and writes the width at which each label wraps,
// in the pixels of Mermaid: the nodes as id=wrap, and the edges as
// source>target=wrap, in order, and a b when the words break.
func wrapsOf(t *testing.T, src string) string {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	g, _, err := Graph(d, true)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return wraps(g)
}

func wraps(g *graph.Graph) string {
	var parts []string
	// zuzu scales each width of Mermaid by 11/14.
	px := func(w float64) float64 { return math.Round(w*14/11*1e6) / 1e6 }
	for _, n := range g.Nodes {
		parts = append(parts, fmt.Sprintf("%s=%g", n.ID, px(n.Wrap)))
	}
	for _, e := range g.Edges {
		parts = append(parts, fmt.Sprintf("%s>%s=%g", e.Source, e.Target, px(e.Wrap)))
	}
	if g.BreakWords {
		parts = append(parts, "b")
	}
	return strings.Join(parts, " ")
}

func TestWrap(t *testing.T) {
	t.Parallel()
	md := "flowchart LR\n  A[\"`a **b**`\"] -->|\"`c`\"| B[plain] -- d --> C"
	tests := []struct {
		name, src, exp string
	}{
		{"default", "flowchart LR\n  A[a] -->|x| B", "A=120 B=120 A>B=200"},
		{"front matter", "---\nconfig:\n  flowchart:\n    wrappingWidth: 300\n---\nflowchart LR\n  A --> B", "A=300 B=300 A>B=200"},
		{"directive", "%%{init: {'flowchart': {'wrappingWidth': 60}}}%%\nflowchart LR\n  A --> B", "A=60 B=60 A>B=200"},
		{"markdown", md, "A=120 B=120 C=120 A>B=200 B>C=200"},
		{"no markdown wrap", "---\nconfig:\n  markdownAutoWrap: false\n---\n" + md, "A=0 B=120 C=120 A>B=0 B>C=200"},
		{"svg labels", "---\nconfig:\n  htmlLabels: false\n  markdownAutoWrap: false\n---\n" + md, "A=120 B=120 C=120 A>B=200 B>C=200 b"},
		{"property label", "flowchart LR\n  A@{ shape: rect, label: \"`a b`\" }", "A=120"},
		{"state", "stateDiagram-v2\n  [*] --> A : go\n  A : a long state\n  note right of A : a note", "root_start=120 A=120 A----note=120 root_start>A=200 A>A----note=200"},
		{"state width", "---\nconfig:\n  state:\n    wrappingWidth: 80\n---\nstateDiagram-v2\n  A --> B", "A=80 B=80 A>B=200"},
		{"class", "classDiagram\n  A <|-- B : a label\n  note for A \"a note\"", "A=0 B=0 note0=0 A>B=200 note0>A=200"},
		{"er", "erDiagram\n  A ||--o{ B : has", "A=0 B=0 A>B=200"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := wrapsOf(t, test.src); got != test.exp {
				t.Errorf("expected %q, got %q", test.exp, got)
			}
		})
	}
}

func TestScaleWrap(t *testing.T) {
	t.Parallel()
	g, _, err := Graph(&Diagram{Kind: "flowchart", Header: "LR", Config: map[string]any{}, Lines: []Line{{Text: "A -->|x| B", N: 2}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	// A label of zuzu is 11 pixels and one of Mermaid is 14.
	if n, e := g.Nodes[0].Wrap, g.Edges[0].Wrap; n != 120.0*11/14 || e != 200.0*11/14 {
		t.Errorf("expected the widths 120 and 200 scaled by 11/14, got %g and %g", n, e)
	}
	// Mermaid draws the labels of a timeline and of a mindmap at 16
	// pixels, so their widths scale by 11/16. mmdc measures the labels of a
	// sequence diagram as wide as 14.5 pixels of Plex, so they scale by
	// 11/14.5. Each other kind scales by 11/14 (D32).
	for kind, want := range map[string]float64{
		"timeline": 110, "mindmap": 110, "flowchart": 160.0 * 11 / 14, "sequenceDiagram": 160.0 * 11 / 14.5,
		"C4Context": 160.0 * 11 / 14, "journey": 160.0 * 11 / 14, "usecase-beta": 160.0 * 11 / 14,
		"eventmodeling": 110,
	} {
		if got := scaleWrap(kind, "", 160); got != want {
			t.Errorf("%s: scaleWrap(160) is %g, want %g", kind, got, want)
		}
	}
	// In a kind whose labels have more than one size, each label scales
	// by its own size: a message of a sequence diagram is 16 pixels in
	// Mermaid, and a use case 12. A label that the kind does not name
	// takes the size of the kind (D32).
	for _, test := range []struct {
		kind, label string
		want        float64
	}{
		{"sequenceDiagram", labelMessage, 160.0 * 11 / 14.5},
		{"usecase-beta", labelUsecase, 160.0 * 11 / 12},
		{"timeline", labelMessage, 110},
		{"flowchart", labelUsecase, 160.0 * 11 / 14},
	} {
		if got := scaleWrap(test.kind, test.label, 160); got != test.want {
			t.Errorf("%s %s: scaleWrap(160) is %g, want %g", test.kind, test.label, got, test.want)
		}
	}
}

// TestSequenceWraps checks that each label of a sequence diagram wraps at
// 11/14.5 of its width in Mermaid, as mmdc measures it at 16 pixels in
// Times New Roman, as wide as 14.5 pixels of the font of zuzu (D32).
func TestSequenceWraps(t *testing.T) {
	t.Parallel()
	d, err := Read("sequenceDiagram\n  A ->> B : hi")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := (&chart{d: d, issues: &graph.Issues{Strict: true}}).sequenceSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.messageWrap != 150.0*11/14.5 || cfg.noteWrap != 150.0*11/14.5 || cfg.actorWrap != 130.0*11/14.5 {
		t.Errorf("expected the widths %g, %g and %g, got %g, %g and %g",
			150.0*11/14.5, 150.0*11/14.5, 130.0*11/14.5, cfg.messageWrap, cfg.noteWrap, cfg.actorWrap)
	}
}

// TestSequenceMermaidWraps checks the widths at which Mermaid 12.3 wraps
// the labels of a sequence diagram, in the pixels of Mermaid. Two
// participants of the width 150 have their centers 200 apart, with the
// actorMargin 50 between them. A message between them wraps at that gap,
// less 2 for the lifelines and 3 for the arrowhead, plus wrapPadding on
// each side: 215. A message to itself wraps at sequence.width, 150. A note
// on one side wraps at 150 less wrapPadding on each side, and a note over
// both at the gap plus actorMargin, less the padding: 230. A frame takes
// labelBoxWidth, 50, from the width of its message, so its label wraps at
// 215 less 50 and the padding: 145.
func TestSequenceMermaidWraps(t *testing.T) {
	t.Parallel()
	src := "---\nconfig:\n  sequence:\n    wrap: true\n---\nsequenceDiagram\n  participant A\n  participant B\n" +
		"  A ->> B : across\n  A ->> A : self\n  Note right of A : right\n  Note over A,B : over\n" +
		"  loop the loop\n    A ->> B : inside\n  end\n  A -> B : open"
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	c := &chart{d: d, issues: &graph.Issues{Strict: true}}
	cfg, err := c.sequenceSettings()
	if err != nil {
		t.Fatal(err)
	}
	s := &sequence{c: c, byID: map[string]*seqParticipant{}, active: map[string]int{}}
	if err := s.parse(); err != nil {
		t.Fatal(err)
	}
	l := newSeqLayout(s, cfg)
	msg, note := 11.0/14.5, 11.0/14.5
	want := map[string]float64{
		"across": 215 * msg, "self": 150 * msg, "right": 130 * note, "over": 230 * note,
		"the loop": 145 * msg, "inside": 215 * msg, "open": 218 * msg,
	}
	for k, e := range s.events {
		if len(e.text.lines) == 0 {
			continue
		}
		name := e.text.lines[0]
		if w, ok := want[name]; ok {
			if got := l.wrapOf(k, 0); math.Abs(got-w) > 1e-9 {
				t.Errorf("%s: expected a wrap of %g, got %g", name, w, got)
			}
			delete(want, name)
		}
	}
	if len(want) > 0 {
		t.Errorf("no event for %v", want)
	}
	// A frame wraps its label even when the diagram does not wrap, and a
	// text with a break in it does not wrap, as in Mermaid.
	no := false
	long := seqText{lines: []string{"a b c d e f g h i j k l m n o p q r s t u v w x y z"}, wrap: &no}
	if got := l.frameLines(long, cfg.messageFont, 40); len(got) < 2 {
		t.Errorf("expected the label of a frame to wrap, got %q", got)
	}
	broken := seqText{lines: []string{"a b c d e f g h i j k l m", "n o p"}}
	if got := l.lines(broken, cfg.messageFont, 20); len(got) != 2 {
		t.Errorf("expected a text with a break to keep its lines, got %q", got)
	}
}

func TestWrapIssues(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"---\nconfig:\n  flowchart:\n    wrappingWidth: wide\n---\nflowchart LR\n  A",
		"---\nconfig:\n  flowchart:\n    wrappingWidth: 0\n---\nflowchart LR\n  A",
		"---\nconfig:\n  htmlLabels: maybe\n---\nflowchart LR\n  A",
		"---\nconfig:\n  state:\n    wrappingWidth: -5\n---\nstateDiagram-v2\n  A",
	} {
		d, err := Read(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if _, _, err := Graph(d, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported setting in the strict mode, got %v", src, err)
		}
		g, report, err := Graph(d, false)
		if err != nil || len(report) != 1 {
			t.Errorf("%q: expected 1 line in the report, got %v, %v", src, report, err)
			continue
		}
		if g.Nodes[0].Wrap != scaleWrap(d.Kind, "", nodeWrap) {
			t.Errorf("%q: expected the default width, got %g", src, g.Nodes[0].Wrap)
		}
	}
}
