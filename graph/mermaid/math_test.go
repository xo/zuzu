package mermaid

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	mathfont "github.com/xo/zuzu/text/math"
	"github.com/xo/zuzu/vega"
)

// TestMathWithFont checks the labels with math of a graph with the font
// of math (D36): a label whose formulas lay out draws as math with no
// issue, and a label with a formula that does not lay out draws its
// source and is an issue that names the command, an error in the strict
// mode. With no font, the label is the issue of today, and with
// htmlLabels false, it draws its source with no issue.
func TestMathWithFont(t *testing.T) {
	t.Parallel()
	read := func(src string, font bool) *Diagram {
		t.Helper()
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if font {
			d.Math = mathfont.Font()
		}
		return d
	}
	const good = "flowchart LR\n  A[\"Area $$x^2$$\"] -->|\"$$\\frac{a}{b}$$\"| B\n  subgraph S[\"$$c$$\"]\n    B\n  end\n"
	g, report, err := Graph(read(good, true), true)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Math || len(g.MathSource) > 0 || len(report) > 0 {
		t.Errorf("expected math with no source and no issue, got %v, %q and %v", g.Math, g.MathSource, report)
	}
	// With no font, each label is the issue of today, and no label draws
	// math.
	g, report, err = Graph(read(good, false), false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Math || len(report) != 3 || report[0].Message != fmt.Sprintf(mathIssue, "Area $$x^2$$") {
		t.Errorf("expected three issues of the source and no math, got %v and %v", g.Math, report)
	}
	// A formula with a command that does not lay out draws its source,
	// once for each label, and names the command.
	const bad = "classDiagram\n  class A[\"$$\\nosuch{x}$$\"]\n  class B {\n    +int $$\\nosuch{x}$$ n\n    +int $$y$$ m\n  }\n  A --> B\n"
	g, report, err = Graph(read(bad, true), false)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(mathCommandIssue, `$$\nosuch{x}$$`, `\nosuch`)
	if len(report) != 2 || report[0].Message != want || !strings.Contains(report[1].Message, `\nosuch`) {
		t.Errorf("expected the issues %q, got %v", want, report)
	}
	if !g.Math || len(g.MathSource) != 2 {
		t.Errorf("expected the member $$y$$ to draw math and two labels to draw their source, got %v and %q", g.Math, g.MathSource)
	}
	if _, _, err := Graph(read(bad, true), true); !errors.Is(err, vega.ErrUnsupported) || !strings.Contains(err.Error(), `\nosuch`) {
		t.Errorf("in the strict mode, expected an error that names \\nosuch, got %v", err)
	}
	// With htmlLabels false, Mermaid draws the source, so zuzu does too,
	// with no issue and no math.
	g, report, err = Graph(read("---\nconfig:\n  htmlLabels: false\n---\n"+good, true), true)
	if err != nil {
		t.Fatal(err)
	}
	if g.Math || len(g.MathSource) > 0 || len(report) > 0 {
		t.Errorf("with htmlLabels false, expected no math and no issue, got %v, %q and %v", g.Math, g.MathSource, report)
	}
}

// TestMath checks that each label with math of KaTeX is one issue in the
// loose mode and an error in the strict mode, in the kinds where Mermaid
// 12.3 draws the math, and that zuzu draws no math.
func TestMath(t *testing.T) {
	t.Parallel()
	issue := func(path, label string) string {
		return path + ": " + fmt.Sprintf(mathIssue, label)
	}
	tests := []struct {
		name, src string
		want      []string
	}{
		{"flowchart", "flowchart LR\n  A[\"$$a$$\"] -->|\"$$b$$\"| B\n  subgraph S[\"$$c$$\"]\n    B\n  end\n", []string{
			issue("node A", "$$a$$"), issue("edge A to B", "$$b$$"), issue("cluster S", "$$c$$"),
		}},
		{"state", "stateDiagram-v2\n  A: x $$a$$ y\n  A --> B: $$b$$\n", []string{
			issue("node A", "x $$a$$ y"), issue("edge A to B", "$$b$$"),
		}},
		{"er", "erDiagram\n  C ||--o{ D : \"$$a$$\"\n  C {\n    string name \"$$b$$\"\n  }\n", []string{
			issue("node C", "$$b$$"), issue("edge C to D", "$$a$$"),
		}},
		{"usecase", "usecase-beta\n  actor U(\"$$a$$\")\n  L(\"$$b$$\")\n  U --> L\n", []string{
			issue("node U", "$$a$$"), issue("node L", "$$b$$"),
		}},
		{"sequence", "sequenceDiagram\n  box Aqua $$z$$\n    participant A as $$a$$\n  end\n  A->>B: $$b$$\n  Note over A: $$c$$\n  loop $$d$$\n    A->>B: hi\n  end\n", []string{
			issue("line 3", "$$a$$"), issue("line 5", "$$b$$"), issue("line 6", "$$c$$"), issue("line 7", "$$d$$"),
		}},
		{"mindmap", "mindmap\n  root\n    a[\"$$a$$\"]\n", []string{issue("line 3", "$$a$$")}},
		{"block", "block-beta\n  a[\"$$a$$\"] b\n  a -- \"$$b$$\" --> b\n", []string{issue("line 2", "$$a$$"), issue("line 3", "$$b$$")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := reportOf(t, test.src)
			for _, w := range test.want {
				if !slices.Contains(got, w) {
					t.Errorf("the report %q does not hold %q", got, w)
				}
			}
			if len(got) != len(test.want) {
				t.Errorf("the report %q, want %d issues", got, len(test.want))
			}
			if err := strictCheck(t, test.src); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("in the strict mode, the error %v", err)
			}
		})
	}
	// Mermaid draws the source of the math as text in a label of SVG, and
	// in the kinds that draw no math, as zuzu does. That is no issue.
	for _, src := range []string{
		"---\nconfig:\n  htmlLabels: false\n---\nflowchart LR\n  A[\"$$a$$\"] --> B\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nclassDiagram\n  class A[\"$$a$$\"]\n",
		"---\nconfig:\n  htmlLabels: false\n---\nmindmap\n  root[\"$$a$$\"]\n",
		"---\nconfig:\n  htmlLabels: false\n---\nblock-beta\n  a[\"$$a$$\"]\n",
		"timeline\n  title $$a$$\n  2020 : $$b$$\n",
		"pie title $$a$$\n  \"$$b$$\" : 1\n",
		"gantt\n  dateFormat YYYY-MM-DD\n  $$a$$ : 2024-01-01, 1d\n",
		"C4Context\n  Person(a, \"$$a$$\")\n",
		"flowchart LR\n  A[\"$a$ and $5\"] --> B\n",
		"ishikawa-beta\n  $$a$$\n  $$b$$\n",
	} {
		if err := strictCheck(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}
