package mermaid_test

import (
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/mermaid"
)

// TestFontFamily checks that fontFamily of the config and font-family of a
// style give a weight and a style, and that zuzu accepts the rest of the
// name with no issue (D32).
func TestFontFamily(t *testing.T) {
	t.Parallel()
	src := "---\nconfig:\n  fontFamily: Helvetica-Oblique\n---\nflowchart LR\n" +
		"  A --> B\n  style B font-family:Arial Black\n  style A font-family:Times-Bold,font-weight:400\n"
	d, err := mermaid.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	g, report, err := mermaid.Graph(d, true)
	if err != nil {
		t.Fatalf("the strict mode: %v", err)
	}
	if len(report) != 0 {
		t.Errorf("the report %v, want none", report)
	}
	st := map[string]map[string]string{}
	for _, n := range g.Nodes {
		st[n.ID] = n.Style
	}
	// font-weight wins over the weight of the family of the same style.
	if st["A"]["fontWeight"] != "400" || st["A"]["fontStyle"] != "italic" {
		t.Errorf("A has the style %v, want the weight 400 and italic", st["A"])
	}
	if st["B"]["fontWeight"] != graph.BoldWeight || st["B"]["fontStyle"] != "italic" {
		t.Errorf("B has the style %v, want the weight %s and italic", st["B"], graph.BoldWeight)
	}
	if g.Edges[0].Style["fontStyle"] != "italic" {
		t.Errorf("the edge has the style %v, want italic", g.Edges[0].Style)
	}
}

// TestSequenceFontFamily checks that the families of the config of a
// sequence diagram give the weight and the style of its text.
func TestSequenceFontFamily(t *testing.T) {
	t.Parallel()
	src := "---\nconfig:\n  fontFamily: courier\n  sequence:\n    actorFontFamily: Times-BoldItalic\n" +
		"    messageFontFamily: Arial\n---\nsequenceDiagram\nA->>B: hi\n"
	d, err := mermaid.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _, report, err := mermaid.Chart(d, true)
	if err != nil {
		t.Fatalf("the strict mode: %v", err)
	}
	if len(report) != 0 {
		t.Errorf("the report %v, want none", report)
	}
	if !strings.Contains(string(b), `"fontStyle":{"value":"italic"}`) {
		t.Errorf("the spec has no italic text:\n%s", b)
	}
	// A family with a weight on a chart that does not read it is an issue.
	d, err = mermaid.Read("---\nconfig:\n  fontFamily: Helvetica-Bold\n---\npie\n\"a\": 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, report, err = mermaid.Chart(d, false); err != nil || len(report) != 1 || !strings.Contains(report[0].Message, "fontFamily") {
		t.Errorf("a pie with a bold family: the report %v and the error %v, want one issue of fontFamily", report, err)
	}
}
