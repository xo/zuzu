package zuzu

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text/math"
	"github.com/xo/zuzu/theme"
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
// center. minNodeWidth is 0, so that no node takes a least width.
func TestMathSizes(t *testing.T) {
	t.Parallel()
	rows := mathRows(t, "---\nconfig:\n  flowchart:\n    minNodeWidth: 0\n---\nflowchart LR\n  a[\"$$\\frac{a}{b}$$ of a whole\"] -->|\"$$\\frac{p}{q}$$\"| b[\"of a whole\"]\n  b --> c[\"x $$y$$\"]\n  c --> d[\"x$$y$$\"]\n")
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

// TestMathLaneTitle checks that the title of a lane that holds math turns
// by -90 degrees when the ranks run from the left to the right, as a plain
// title turns (D60). Each run of text and each formula takes the angle and
// turns around the place of the title, so that the title reads from the
// bottom to the top in the band at the left of the lane. The bounds of a
// formula turn with it, and the look drawn by hand keeps the turn.
func TestMathLaneTitle(t *testing.T) {
	t.Parallel()
	const src = "swimlane-beta LR\n  subgraph L[\"Lane $$x^2$$\"]\n    a[A]\n  end\n  subgraph M[Plain]\n    b[B]\n  end\n  a --> b\n"
	c, report, err := Parse([]byte(src), WithStrict(true), WithMath(math.Font()))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, look := range []theme.Look{theme.LookStandard, theme.LookExcalidraw} {
		s, more, err := c.Render(t.Context(), WithLocation(time.UTC), WithLook(look), lookFont)
		if err != nil || len(more) > 0 {
			t.Fatalf("%s: %v (%v)", look, err, more)
		}
		var lane map[string]any
		var plain *scene.Item
		var runs, paths []*scene.Item
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			for i := range m.Items {
				it := &m.Items[i]
				d, _ := it.Datum.(map[string]any)
				switch {
				case d["kind"] == "cluster" && d["key"] == "L":
					lane = d
				case d["kind"] == "cluster" && d["key"] == "M" && m.Type == scene.Text:
					plain = it
				case d["kind"] == "mathText" && d["part"] == "cluster":
					runs = append(runs, it)
				case d["kind"] == "mathPath" && d["part"] == "cluster":
					paths = append(paths, it)
				}
				for j := range it.Items {
					walk(&it.Items[j])
				}
			}
		}
		walk(&s.Root)
		if lane == nil || plain == nil || len(runs) != 1 || len(paths) == 0 {
			t.Fatalf("%s: expected the lanes, one run and a formula, got %v, %v, %d runs and %d paths", look, lane, plain, len(runs), len(paths))
		}
		if plain.Angle != -90 || lane["titleAngle"] != -90.0 {
			t.Errorf("%s: expected the titles turned by -90 degrees, got %v and %v", look, plain.Angle, lane["titleAngle"])
		}
		num := func(k string) float64 { f, _ := lane[k].(float64); return f }
		x, y, w, h := num("titleX"), num("titleY"), num("titleWidth"), num("titleHeight")
		// along is the place of a part along the title, from its start at
		// the bottom, and across is its place across the title, from its
		// left, as the part lay before it turned.
		along := func(it *scene.Item) float64 { return y - it.Y }
		across := func(it *scene.Item) float64 { return it.X - x }
		for _, it := range append(slices.Clone(runs), paths...) {
			if it.Angle != -90 {
				t.Errorf("%s: expected the part %q turned by -90 degrees, got %v", look, it.Text, it.Angle)
			}
			if a, b := along(it), across(it); a < 0 || a > w || b < 0 || b > h {
				t.Errorf("%s: the part %q is %v along and %v across the title, want in %v by %v", look, it.Text, a, b, w, h)
			}
		}
		if runs[0].Text[0] != "Lane" || along(runs[0]) >= along(paths[0]) {
			t.Errorf("%s: expected the run Lane before the formula, at %v and %v", look, along(runs[0]), along(paths[0]))
		}
		for _, it := range paths {
			b := layout.Style{}.ItemBounds(scene.Path, it)
			if b.X1 < x-1 || b.X2 > x+h+1 || b.Y1 < y-w-1 || b.Y2 > y+1 {
				t.Errorf("%s: the formula %q has the bounds %v, want in the band from %v,%v to %v,%v", look, it.Text, b, x, y-w, x+h, y)
			}
		}
	}
}
