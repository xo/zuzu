package zuzu

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/text/math"
	"github.com/xo/zuzu/vega"
)

// mathSVG parses a source with the font of math and renders it as SVG.
func mathSVG(t *testing.T, src string, strict bool) (string, Report) {
	t.Helper()
	c, report, err := Parse([]byte(src), WithStrict(strict), WithMath(math.Font()))
	if err != nil {
		t.Fatalf("%q: %v (%v)", src, err, report)
	}
	var b bytes.Buffer
	more, err := c.SVG(t.Context(), &b, WithLocation(time.UTC))
	if err != nil {
		t.Fatalf("%q: %v (%v)", src, err, more)
	}
	return b.String(), more
}

// mathRows parses a source with the font of math and returns the rows of
// the dataset layout of its first view.
func mathRows(t *testing.T, src string) []map[string]any {
	t.Helper()
	c, report, err := Parse([]byte(src), WithStrict(true), WithMath(math.Font()))
	if err != nil {
		t.Fatalf("%q: %v (%v)", src, err, report)
	}
	v, _, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("layout")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]map[string]any, len(rows))
	copy(out, rows)
	return out
}

// TestMathSVG checks that a flowchart with math draws each formula as a
// path of role math, with its TeX as its label, and no $$ (D36).
func TestMathSVG(t *testing.T) {
	t.Parallel()
	const src = "flowchart LR\n  a[\"Area $$x^2$$\"] -->|\"$$\\frac{a}{b}$$\"| b[plain]\n  subgraph s[\"Set $$S$$\"]\n    b\n  end\n"
	s, report := mathSVG(t, src, true)
	if len(report) > 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	if strings.Contains(s, "$$") {
		t.Error("the SVG holds $$")
	}
	for _, want := range []string{
		`role="img" aria-label="x^2" fill="#`, `aria-label="\frac{a}{b}"`, `aria-label="S"`,
		`>Area</text>`, `>Set</text>`, `>plain</text>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the SVG has no %s", want)
		}
	}
	// A formula is a filled outline, with no stroke.
	if i := strings.Index(s, `aria-label="x^2"`); i >= 0 && strings.Contains(s[i:i+strings.Index(s[i:], ">")], "stroke") {
		t.Error("the path of a formula has a stroke")
	}
}

// TestMathKinds checks that the labels of a class, a state, an entity and
// a use case draw their formulas, and keep the other text.
func TestMathKinds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		src  string
		want []string
	}{
		{"classDiagram\n  class Circle {\n    +float r\n    +float $$\\pi r^2$$ area\n  }\n  class Shape[\"Shape $$S$$\"]\n  Shape \"$$n$$\" <|-- \"1\" Circle : $$\\subset$$\n",
			[]string{`aria-label="\pi r^2"`, `aria-label="S"`, `aria-label="n"`, `aria-label="\subset"`, `>+float r</text>`, `>area</text>`}},
		{"stateDiagram-v2\n  A: Start $$x_0$$\n  A --> B: $$x \\to y$$\n", []string{`aria-label="x_0"`, `aria-label="x \to y"`, `>Start</text>`}},
		{"erDiagram\n  ORBIT ||--o{ BODY : \"$$F = ma$$\"\n  ORBIT {\n    float radius \"$$r^2$$\"\n  }\n", []string{`aria-label="F = ma"`, `aria-label="r^2"`, `>radius</text>`}},
		{"usecase-beta\n  actor U(\"User $$u$$\")\n  L(\"$$b$$\")\n  U --> L\n", []string{`aria-label="u"`, `aria-label="b"`, `>User</text>`}},
	} {
		s, report := mathSVG(t, test.src, true)
		if len(report) > 0 {
			t.Errorf("%q: expected no issue, got %v", test.src, report)
		}
		for _, want := range test.want {
			if !strings.Contains(s, want) {
				t.Errorf("%q: the SVG has no %s", test.src, want)
			}
		}
		if strings.Contains(s, "$$") {
			t.Errorf("%q: the SVG holds $$", test.src)
		}
	}
}

// TestMathSource checks the labels that draw their source: with no font
// of math, with a formula that does not lay out, and with htmlLabels false.
func TestMathSource(t *testing.T) {
	t.Parallel()
	const good = "flowchart LR\n  a[\"Area $$x^2$$\"] --> b\n"
	s, report, err := svgOf(t, good, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, ">Area $$x^2$$<") || strings.Contains(s, `role="img"`) || len(report) != 1 {
		t.Errorf("with no font, expected the source and one issue, got %v", report)
	}
	const bad = "flowchart LR\n  a[\"Area $$\\nosuch{x}$$\"] --> b[\"$$y$$\"]\n"
	s, report = mathSVG(t, bad, false)
	if !strings.Contains(s, `$$\nosuch{x}$$<`) || !strings.Contains(s, `aria-label="y"`) {
		t.Errorf("expected the source of the bad label and the math of the good one, got %s", s)
	}
	if len(report) != 1 || !strings.Contains(report[0].Message, `\nosuch`) {
		t.Errorf("expected one issue that names \\nosuch, got %v", report)
	}
	if _, _, err := Parse([]byte(bad), WithStrict(true), WithMath(math.Font())); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("in the strict mode, expected an error, got %v", err)
	}
	s, report = mathSVG(t, "---\nconfig:\n  htmlLabels: false\n---\n"+good, true)
	if !strings.Contains(s, ">Area $$x^2$$<") || strings.Contains(s, `role="img"`) || len(report) > 0 {
		t.Errorf("with htmlLabels false, expected the source and no issue, got %v", report)
	}
}

// TestMathSizes checks that the transform zuzu.layered measures a label
// with math by its formulas: a fraction makes its node and its edge label
// higher than a line of text, the space next to a formula takes no width,
// and the text and the formulas of a label sit in one row around its
// center.
func TestMathSizes(t *testing.T) {
	t.Parallel()
	rows := mathRows(t, "flowchart LR\n  a[\"$$\\frac{a}{b}$$ of a whole\"] -->|\"$$\\frac{p}{q}$$\"| b[\"of a whole\"]\n  b --> c[\"x $$y$$\"]\n  c --> d[\"x$$y$$\"]\n")
	node := map[string]map[string]any{}
	var edge map[string]any
	var runs, paths []map[string]any
	for _, r := range rows {
		switch r["kind"] {
		case "node":
			key, _ := r["key"].(string)
			node[key] = r
		case "edge":
			if r["source"] == "a" {
				edge = r
			}
		case "mathText":
			runs = append(runs, r)
		case "mathPath":
			paths = append(paths, r)
		}
	}
	num := func(r map[string]any, k string) float64 { f, _ := r[k].(float64); return f }
	a, b := node["a"], node["b"]
	if num(a, "height") <= num(b, "height") {
		t.Errorf("the node with a fraction is %v high, and the node of text %v", num(a, "height"), num(b, "height"))
	}
	if num(a, "width") <= num(b, "width") {
		t.Errorf("the node with a fraction is %v wide, and the node of its text alone %v", num(a, "width"), num(b, "width"))
	}
	if h := num(edge, "labelHeight"); h <= 11+4 {
		t.Errorf("the label of a fraction is %v high, want more than a line of text", h)
	}
	if num(node["c"], "width") != num(node["d"], "width") {
		t.Errorf("x $$y$$ is %v wide and x$$y$$ %v, want the same", num(node["c"], "width"), num(node["d"], "width"))
	}
	if lines, _ := a["lines"].([]any); len(lines) != 0 {
		t.Errorf("the label of a node with math draws its lines %v", a["lines"])
	}
	// The fraction of a starts the row, and its text follows it, both
	// around the middle of the node.
	var frac, text map[string]any
	for _, p := range paths {
		if p["key"] == "a" {
			frac = p
		}
	}
	for _, r := range runs {
		if r["key"] == "a" {
			text = r
		}
	}
	if frac == nil || text == nil {
		t.Fatalf("expected a path and a run of node a, got %v and %v", paths, runs)
	}
	if text["text"] != "of a whole" || text["part"] != "node" || frac["tex"] != `\frac{a}{b}` || frac["part"] != "node" {
		t.Errorf("unexpected rows %v and %v", frac, text)
	}
	if num(frac, "x") >= num(text, "x") || num(text, "y") != num(a, "y") {
		t.Errorf("the fraction at %v and the text at %v, %v, want the text after it at the middle %v", num(frac, "x"), num(text, "x"), num(text, "y"), num(a, "y"))
	}
	if y := num(frac, "y"); y < num(a, "y") || y > num(a, "y")+num(a, "height")/2 {
		t.Errorf("the baseline of the fraction at %v, want just below the middle %v", y, num(a, "y"))
	}
}
