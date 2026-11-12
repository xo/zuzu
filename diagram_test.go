package zuzu

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

func TestSniffDiagram(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src string
		exp Dialect
	}{
		{"digraph { a -> b }", DOT},
		{"strict graph { a -- b }", DOT},
		{"graph {\n a -- b }", DOT},
		{"graph G { a -- b }", DOT},
		{"graph grid\n{\n a -- b }", DOT},
		{"graph \"\"\n\n{ a -- b }", DOT},
		{"graph\n{ a -- b }", DOT},
		{"graph\n A --> B", Mermaid},
		{"graph td;\n A --> B", Mermaid},
		{"// comment\ndigraph { a }", DOT},
		{"/* c */ digraph { a }", DOT},
		{"graph TD\n A --> B", Mermaid},
		{"flowchart LR\n A --> B", Mermaid},
		{"%% comment\nflowchart LR\n A", Mermaid},
		{"---\ntitle: x\n---\nflowchart LR\n A", Mermaid},
		{"pie\n \"a\": 1", Mermaid},
		{`{"$schema": "x"}`, 0},
		{"hello", 0},
	}
	for _, test := range tests {
		if d := sniffDiagram([]byte(test.src)); d != test.exp {
			t.Errorf("%q: expected %v, got %v", test.src, test.exp, d)
		}
	}
}

func TestSniff(t *testing.T) {
	t.Parallel()
	long := "---\ntitle: " + strings.Repeat("x", 600) + "\n---\nflowchart LR\n A"
	tests := []struct {
		src string
		exp Dialect
		ok  bool
	}{
		{"digraph { a -> b }", DOT, true},
		{"flowchart LR\n A --> B", Mermaid, true},
		{long[:512], Mermaid, true},
		{`{"$schema": "https://vega.github.io/schema/vega/v5.json", "marks": []}`, Vega, true},
		{`{"$schema":"https://vega.github.io/schema/vega-lite/v6.json"}`, VegaLite, true},
		{`{"$schema": "https://example.com/other.json"}`, 0, false},
		{`{"data": []}`, 0, false},
		{"/* an open comment", 0, false},
		{"hello world", 0, false},
		{"", 0, false},
	}
	for _, test := range tests {
		if d, ok := Sniff([]byte(test.src)); d != test.exp || ok != test.ok {
			t.Errorf("%.40q: expected %v %t, got %v %t", test.src, test.exp, test.ok, d, ok)
		}
	}
}

func TestBackground(t *testing.T) {
	t.Parallel()
	src := "digraph { a -> b }"
	bg := func(s string) bool { return strings.Contains(s, `<rect width=`) }
	s, _, err := svgOf(t, src, true)
	if err != nil || !bg(s) || !strings.Contains(s, `fill="#ffffff"`) {
		t.Fatalf("expected the white background of the light theme, got %v", err)
	}
	s, _, err = svgOf(t, src, true, WithBackground(color.NRGBA{}))
	if err != nil || bg(s) {
		t.Errorf("expected no background, got %v", err)
	}
	s, _, err = svgOf(t, src, true, WithBackground(color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}))
	if err != nil || !strings.Contains(s, `fill="#123456"`) {
		t.Errorf("expected the background #123456, got %v", err)
	}
	s, _, err = svgOf(t, src, true, WithBackground(nil))
	if err != nil || !strings.Contains(s, `fill="#ffffff"`) {
		t.Errorf("expected nil to keep the theme, got %v", err)
	}
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := c.Image(t.Context(), WithBackground(color.NRGBA{}))
	if err != nil {
		t.Fatal(err)
	}
	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Errorf("expected a transparent corner, got an alpha of %d", a)
	}
}

// svgOf parses a source and renders it as SVG.
func svgOf(t *testing.T, src string, strict bool, opts ...RenderOption) (string, Report, error) {
	t.Helper()
	c, _, err := Parse([]byte(src), WithStrict(strict))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	report, err := c.SVG(t.Context(), &b, append([]RenderOption{WithLocation(time.UTC)}, opts...)...)
	return b.String(), report, err
}

func TestDiagrams(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"digraph { a [label=\"First\"]; a -> b [label=\"goes\"]; subgraph cluster_x { label=\"Box\"; b } }",
		"flowchart LR\n  a[First] -->|goes| b\n  subgraph x [Box]\n    b\n  end",
	} {
		s, report, err := svgOf(t, src, true)
		if err != nil {
			t.Fatalf("%q: %v (%v)", src, err, report)
		}
		for _, want := range []string{">First<", ">goes<", ">Box<", ">b<"} {
			if !strings.Contains(s, want) {
				t.Errorf("%q: the SVG has no %s", src, want)
			}
		}
	}
}

func TestClassAndERDiagrams(t *testing.T) {
	t.Parallel()
	class := `classDiagram
  class Shape{
    <<interface>>
    +area()$ double
    +draw()*
  }
  Shape <|-- Square
  Square "1" --> "*" Side : has`
	s, report, err := svgOf(t, class, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{">«interface»<", ">Shape<", ">+area() : double<", ">+draw()<", `font-style="italic"`, `font-weight="600"`, ">1<", ">*<", ">has<"} {
		if !strings.Contains(s, want) {
			t.Errorf("the class diagram has no %s", want)
		}
	}
	// The line under the static method is a thin box under it.
	if !strings.Contains(s, `height="1"`) {
		t.Error("the class diagram has no line under its static method")
	}
	er := `erDiagram
  CUSTOMER ||--o{ ORDER : places
  CUSTOMER {
    string name PK "the name"
    int age
  }`
	// The circle of the arrowhead crowodot draws at 1.4 times its size.
	s, report, err = svgOf(t, er, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{">CUSTOMER<", ">string<", ">name<", ">PK<", ">the name<", ">int<", ">age<", ">places<", "A3.36 3.36"} {
		if !strings.Contains(s, want) {
			t.Errorf("the ER diagram has no %s", want)
		}
	}
}

func TestRequirementDiagram(t *testing.T) {
	t.Parallel()
	src := `requirementDiagram
  requirement test_req {
    id: 1
    text: the test text.
    risk: high
    verifymethod: test
  }
  element test_entity {
    type: simulation
  }
  test_entity - satisfies -> test_req
  test_req - contains -> test_entity`
	s, report, err := svgOf(t, src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{">«Requirement»<", ">test_req<", ">ID: 1<", ">Text: the test text.<", ">Risk: High<",
		">Verification: Test<", ">«Element»<", ">Type: simulation<", ">«satisfies»<", ">«contains»<", `font-weight="600"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the requirement diagram has no %s", want)
		}
	}
}

func TestHTMLLabels(t *testing.T) {
	t.Parallel()
	src := `digraph {
	rankdir=LR
	node [shape=none, margin=0]
	books [label=<<table border="0" cellborder="1" cellspacing="0" cellpadding="4">
		<tr><td bgcolor="lightblue">books</td></tr>
		<tr><td align="left" port="id">id: integer</td></tr>
		<tr><td align="left" port="author">author_id: integer</td></tr>
	</table>>]
	authors [label=<<table border="0" cellborder="1" cellspacing="0"><tr><td port="id">id</td></tr></table>>]
	books:author:e -> authors:id:w
}`
	s, report, err := svgOf(t, src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{">author_id: integer<", ">books<", `fill="lightblue"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the SVG has no %s", want)
		}
	}
	if strings.Contains(s, ">books\nid") || strings.Contains(s, "<table") {
		t.Error("the SVG draws the label as plain text")
	}
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("layout")
	if err != nil {
		t.Fatal(err)
	}
	var books, edge map[string]any
	for _, r := range rows {
		switch {
		case r["kind"] == "node" && r["key"] == "books":
			books = r
		case r["kind"] == "edge":
			edge = r
		}
	}
	if books == nil || edge == nil {
		t.Fatal("the layout has no node books or no edge")
	}
	// The edge starts on the east side of the books node, below its
	// center, where the row of author_id is.
	pts, ok := edge["points"].([]any)
	if !ok || len(pts) == 0 {
		t.Fatalf("the edge has no points: %v", edge["points"])
	}
	start, ok := pts[0].([]any)
	if !ok || len(start) != 2 {
		t.Fatalf("the first point is %v", pts[0])
	}
	num := func(v any) float64 {
		f, _ := v.(float64)
		return f
	}
	x, y := num(start[0]), num(start[1])
	bx, by, bw := num(books["x"]), num(books["y"]), num(books["width"])
	if math.Abs(x-(bx+bw/2)) > 0.01 || y <= by {
		t.Errorf("the edge starts at %v, %v, not on the east side of books at %v, %v", x, y, bx+bw/2, by)
	}
}

func TestEndLabelsAndHTML(t *testing.T) {
	t.Parallel()
	src := `digraph {
	label=<<b>Shop</b> plan>
	subgraph cluster_a { label=<<i>Store</i>>; a; b }
	a [xlabel="outer"]
	a -> b [dir=both, arrowtail=teetee, arrowhead=crowodot, headlabel="0..*", taillabel="1", xlabel="sells", labelfontcolor=red]
	b -> c [label=<<table><tr><td>fk</td><td bgcolor="yellow">id</td></tr></table>>, headlabel=<<b>N</b>>]
}`
	s, report, err := svgOf(t, src, true)
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	for _, want := range []string{">0..*<", ">1<", ">sells<", ">outer<", ">Shop<", "plan<", ">Store<", ">fk<", ">id<", ">N<", `fill="yellow"`, `fill="red"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the SVG has no %s", want)
		}
	}
	for _, not := range []string{"&lt;b&gt;", "&lt;table", "&lt;i&gt;"} {
		if strings.Contains(s, not) {
			t.Errorf("the SVG draws %s as plain text", not)
		}
	}
	// The circle of the arrowhead crowodot, 0.8 of the size of the symbol
	// across.
	if !strings.Contains(s, "A2.4 2.4") {
		t.Error("the SVG has no circle of the arrowhead crowodot")
	}
}

func TestIcons(t *testing.T) {
	t.Parallel()
	src := "flowchart LR\n  a@{ icon: \"fa:user\", label: \"User\" } --> b@{ icon: \"database\", form: square } --> c[fa:fa-gear Gear]"
	s, report, err := svgOf(t, src, true, WithIcons("fa", fa.Pack()))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if !strings.Contains(s, "<metadata>Icons: Font Awesome Free") {
		t.Error("the SVG names no credit of Font Awesome")
	}
	// The icon of a pack draws as a path, and the label of the inline icon
	// stays.
	if !strings.Contains(s, ">Gear<") || strings.Contains(s, "fa-gear") {
		t.Error("the inline icon is not drawn apart from its label")
	}
	// Without the pack, the icon is missing: an error in the strict mode,
	// and a line of the report in the loose mode.
	if _, _, err := svgOf(t, src, true); !errors.Is(err, icon.ErrMissing) {
		t.Errorf("expected ErrMissing in the strict mode, got %v", err)
	}
	_, report, err = svgOf(t, src, false)
	if err != nil || len(report) != 2 {
		t.Errorf("expected 2 lines in the report, got %v, %v", report, err)
	}
}

func TestIconLoader(t *testing.T) {
	t.Parallel()
	calls := 0
	loader := IconLoaderFunc(func(_ context.Context, req IconRequest) (io.ReadCloser, error) {
		calls++
		if req.Prefix != "aws" || req.Name != "lambda" {
			return nil, icon.ErrMissing
		}
		svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" onload="x()"><script>x()</script>` +
			`<rect width="10" height="10" fill="#f80"/><image href="https://example.com/a.png"/></svg>`
		return io.NopCloser(strings.NewReader(svg)), nil
	})
	src := "flowchart LR\n  a@{ icon: \"aws:lambda\" } --> b@{ icon: \"aws:lambda\" }"
	s, report, err := svgOf(t, src, true, WithIconLoader(loader))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if calls != 1 {
		t.Errorf("expected 1 call of the loader, got %d", calls)
	}
	// The icon of several colors draws as an image, written once in the
	// definitions and used twice.
	if n := strings.Count(s, "<use "); n != 2 || strings.Count(s, "data:image/svg+xml") != 1 {
		t.Errorf("expected 2 uses of 1 image, got %d uses", n)
	}
	if strings.Contains(s, "example.com") {
		t.Error("the SVG holds a URL from outside")
	}
	// FSIconLoader reads an SVG, and else a pack.
	fsys := fstest.MapFS{
		"k/one.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path fill="currentColor" d="M0 0h4v4z"/></svg>`)},
		"p.json":    {Data: []byte(`{"prefix":"p","icons":{"two":{"body":"<path fill=\"currentColor\" d=\"M0 0h1v1z\"/>"}}}`)},
	}
	src = "flowchart LR\n  a@{ icon: \"k:one\" } --> b@{ icon: \"p:two\" }"
	if _, report, err := svgOf(t, src, true, WithIconLoader(FSIconLoader(fsys))); err != nil {
		t.Errorf("%v (%v)", err, report)
	}
}

func TestImageNode(t *testing.T) {
	t.Parallel()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		img.Set(x, 0, color.NRGBA{R: 255, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"a.png": {Data: b.Bytes()}}
	src := "flowchart LR\n  a@{ img: \"a.png\", label: \"Pic\", w: 40, constraint: on } --> b"
	s, report, err := svgOf(t, src, true, WithLoader(FSLoader(fsys)))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if !strings.Contains(s, `width="40" height="20" href="data:image/png;base64,`) {
		t.Error("the image does not draw at 40 by 20")
	}
	// With no loader, the node draws a box with a question mark.
	if _, report, err := svgOf(t, src, false); err != nil || len(report) != 1 {
		t.Errorf("expected 1 line in the report, got %v, %v", report, err)
	}
}

func TestScale(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 40, "height": 20, "padding": 0,
		"marks": [{"type": "rect", "encode": {"enter": {"x": {"value": 10}, "y": {"value": 5}, "width": {"value": 20}, "height": {"value": 10}, "fill": {"value": "#ff0000"}}}}]}`
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	one, _, err := c.Image(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	two, _, err := c.Image(t.Context(), WithScale(2))
	if err != nil {
		t.Fatal(err)
	}
	if b1, b2 := one.Bounds(), two.Bounds(); b2.Dx() != 2*b1.Dx() || b2.Dy() != 2*b1.Dy() {
		t.Fatalf("expected twice %v, got %v", b1.Size(), b2.Size())
	}
	// The rect covers 10 to 30 and 5 to 15, so 20 to 60 and 10 to 30 at
	// the scale 2.
	for _, p := range []struct {
		x, y int
		red  bool
	}{{21, 11, true}, {58, 28, true}, {18, 11, false}, {21, 31, false}} {
		if got := two.RGBAAt(p.x, p.y).R == 255 && two.RGBAAt(p.x, p.y).G == 0; got != p.red {
			t.Errorf("at %d,%d: expected red %t, got %v", p.x, p.y, p.red, two.RGBAAt(p.x, p.y))
		}
	}
	for _, s := range []float64{0, -1, math.Inf(1), math.NaN()} {
		if _, _, err := c.Image(t.Context(), WithScale(s)); err == nil {
			t.Errorf("expected an error of the scale %v", s)
		}
	}
}

func TestFillContrast(t *testing.T) {
	t.Parallel()
	src := "flowchart LR\n A[Light] --> B[Dark] --> C[Own]\n style A fill:#99ccff\n style B fill:#123456\n style C fill:#99ccff,color:#ff0000"
	label := regexp.MustCompile(`<text[^>]* fill="([^"]+)"[^>]*>(Light|Dark|Own)<`)
	for _, mode := range []theme.Mode{theme.Light, theme.Dark} {
		s, _, err := svgOf(t, src, true, WithMode(mode))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, m := range label.FindAllStringSubmatch(s, -1) {
			got[m[2]] = m[1]
		}
		exp := map[string]string{
			"Light": theme.New(theme.Light, theme.Zuzu).Text,
			"Dark":  theme.New(theme.Dark, theme.Zuzu).Text,
			"Own":   "#ff0000",
		}
		for k, v := range exp {
			if got[k] != v {
				t.Errorf("mode %v: expected the label %s in %s, got %q", mode, k, v, got[k])
			}
		}
	}
}

func TestDOTNodeSize(t *testing.T) {
	t.Parallel()
	src := `digraph {
		node [fontsize=24 shape=box]
		a [label="a fairly long label"]
		b [label="short" width=2.2 height=1]
		c [label="fixed" width=0.3 height=0.3 fixedsize=true]
		d [label="wide margin" fontsize=11 margin="0.5,0.25"]
		e [label="thick" style="filled,setlinewidth(5)"]
		a -> b -> c -> d -> e
	}`
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("layout")
	if err != nil {
		t.Fatal(err)
	}
	size := map[string][2]float64{}
	stroke := map[string]float64{}
	for _, r := range rows {
		if r["kind"] != "node" {
			continue
		}
		k, _ := r["key"].(string)
		w, _ := r["width"].(float64)
		h, _ := r["height"].(float64)
		size[k] = [2]float64{w, h}
		stroke[k], _ = r["strokeWidth"].(float64)
	}
	// The label of a is about 200 wide at 24 px, so the box is wider.
	if size["a"][0] < 190 {
		t.Errorf("expected the box of a to fit its label at 24 px, got %v", size["a"])
	}
	if size["b"][0] < 2.2*72 || size["b"][1] < 72 {
		t.Errorf("expected b to be at least 158 by 72, got %v", size["b"])
	}
	if math.Abs(size["c"][0]-21.6) > 1e-9 || math.Abs(size["c"][1]-21.6) > 1e-9 {
		t.Errorf("expected c to keep its fixed size, got %v", size["c"])
	}
	if size["d"][1] < 36+11 {
		t.Errorf("expected the margin of d to add 36 to its height, got %v", size["d"])
	}
	if stroke["e"] != 5 {
		t.Errorf("expected setlinewidth(5) to give a stroke of 5, got %v", stroke["e"])
	}
}

// layoutRows draws a diagram and returns the rows of its layout by their
// kind and their key.
func layoutRows(t *testing.T, src string) map[string]map[string]any {
	t.Helper()
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := c.NewView(t.Context(), WithIcons("fa", fa.Pack()))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("layout")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, r := range rows {
		kind, _ := r["kind"].(string)
		key, _ := r["key"].(string)
		out[kind+" "+key] = r
	}
	return out
}

func TestWrap(t *testing.T) {
	t.Parallel()
	const long = "This is a rather long label of a node that goes on and on"
	src := "flowchart LR\n" +
		"  A[" + long + "] -->|an edge label that is also rather long and goes on and on| B[Supercalifragilisticexpialidocious_with_no_spaces]\n" +
		"  B --> C[fa:fa-gear Shared Business Logic] --> D[Shared Business Logic]\n" +
		"  subgraph S [A subgraph title that is rather long and goes on and on and on]\n    C\n  end"
	m := text.NewEstimate()
	font := text.Font{Family: text.Family, Size: 11, Weight: 400}
	linesOf := func(r map[string]any) []string {
		var out []string
		ls, _ := r["lines"].([]any)
		for _, l := range ls {
			s, _ := l.(string)
			out = append(out, s)
		}
		return out
	}
	check := func(name string, lines []string, limit float64, n int) {
		t.Helper()
		if len(lines) != n {
			t.Errorf("%s: expected %d lines, got %q", name, n, lines)
		}
		for _, l := range lines {
			if w := m.Width(font, l); w > limit {
				t.Errorf("%s: the line %q is %g wide, wider than %g", name, l, w, limit)
			}
		}
	}
	// zuzu wraps at the widths of Mermaid, 120 for a node and 200 for an
	// edge, scaled by 11/14, the size of a label of zuzu over that of
	// Mermaid.
	node, edge := 120.0*11/14, 200.0*11/14
	rows := layoutRows(t, src)
	check("A", linesOf(rows["node A"]), node, 4)
	if got := strings.Join(linesOf(rows["node A"]), " "); got != long {
		t.Errorf("A: expected the words of the label, got %q", got)
	}
	check("the edge", linesOf(rows["edge 0"]), edge, 2)
	// A word wider than the width keeps a line of its own.
	check("B", linesOf(rows["node B"]), 1000, 1)
	// The icon in the line takes 18 of the width.
	check("C", linesOf(rows["node C"]), node-18, 2)
	check("D", linesOf(rows["node D"]), node, 2)
	if l, _ := rows["cluster S"]["label"].(string); strings.Contains(l, "\n") {
		t.Errorf("expected the title of the subgraph on one line, got %q", l)
	}
	// SVG labels break the word, and DOT does not wrap.
	rows = layoutRows(t, "---\nconfig:\n  htmlLabels: false\n---\n"+src)
	check("B in SVG", linesOf(rows["node B"]), node, 3)
	rows = layoutRows(t, `digraph { a [label="`+long+`"] }`)
	check("DOT", linesOf(rows["node a"]), 1000, 1)
}
