package zuzu

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/internal/sketch"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/excalifont"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// lookCases are the sources of the tests of the look like Excalidraw, in
// testdata/look: a bar chart with axes and a legend, a line chart, a pie, a
// heatmap with the gradient of its legend, a flowchart with clusters and a
// sequence diagram.
var lookCases = []string{"bar.vl.json", "line.vl.json", "pie.vl.json", "heatmap.vl.json", "flowchart.mmd", "sequence.mmd"}

// lookFont is the option of the font of the look like Excalidraw.
var lookFont = WithLookFont(theme.LookExcalidraw, excalifont.Font())

// lookChart parses a source of testdata/look in the look like Excalidraw,
// with Excalifont.
func lookChart(t *testing.T, name string) *Chart {
	t.Helper()
	src, err := os.ReadFile("testdata/look/" + name)
	if err != nil {
		t.Fatal(err)
	}
	c, report, err := Parse(src, WithLook(theme.LookExcalidraw), lookFont, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("%s: the report of the parse is %v, want none", name, report)
	}
	return c
}

// lookSVG draws a chart as SVG in a mode, in the strict mode, with the
// font of the look.
func lookSVG(t *testing.T, c *Chart, mode theme.Mode) []byte {
	t.Helper()
	var b bytes.Buffer
	report, err := c.SVG(t.Context(), &b, WithLocation(time.UTC), WithMode(mode), lookFont)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("the report is %v, want none", report)
	}
	return b.Bytes()
}

// TestLookIsDeterministic draws each source twice, from two parses, and
// asks for the same bytes, in the light and the dark mode (hard rule 6).
func TestLookIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, name := range lookCases {
		for _, mode := range []theme.Mode{theme.Light, theme.Dark} {
			a := lookSVG(t, lookChart(t, name), mode)
			b := lookSVG(t, lookChart(t, name), mode)
			if !bytes.Equal(a, b) {
				t.Errorf("%s %s: two renders differ", name, mode)
			}
		}
	}
}

// TestLookDrawsEveryMark checks that the look turns every mark into paths,
// keeps each text in Excalifont, and leaves no rect, rule, symbol, arc,
// line or area in the scene (D40 part 4).
func TestLookDrawsEveryMark(t *testing.T) {
	t.Parallel()
	for _, name := range lookCases {
		s, _, err := lookChart(t, name).Render(t.Context(), WithLocation(time.UTC), lookFont)
		if err != nil {
			t.Fatal(err)
		}
		texts, paths := 0, 0
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			switch m.Type {
			case scene.Group:
			case scene.Text:
				for i := range m.Items {
					texts++
					if f := m.Items[i].Font; !strings.HasPrefix(f, "'Excalifont'") {
						t.Errorf("%s: a text of the mark %q has the family %q, want Excalifont first", name, m.Name, f)
					}
				}
			case scene.Path, scene.Image:
				paths += len(m.Items)
			default:
				t.Errorf("%s: the mark %q has the type %s", name, m.Name, m.Type)
			}
			for i := range m.Items {
				for j := range m.Items[i].Items {
					walk(&m.Items[i].Items[j])
				}
			}
		}
		walk(&s.Root)
		if texts == 0 || paths == 0 {
			t.Errorf("%s: %d texts and %d paths, want some of each", name, texts, paths)
		}
	}
}

// TestLookImageMatchesSVG draws the look with the direct renderer and with
// okage from the SVG, with Excalifont, and asks that at most 1 percent of
// the pixels differ, the gate of okage D31 (D11). Both read the same scene.
func TestLookImageMatchesSVG(t *testing.T) {
	t.Parallel()
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family),
		okagesvg.WithFontFile(excalifont.Font().Data()))
	for _, name := range lookCases {
		c := lookChart(t, name)
		img, report, err := c.Image(t.Context(), WithLocation(time.UTC), lookFont)
		if err != nil {
			t.Fatal(err)
		}
		if len(report) != 0 {
			t.Errorf("%s: the report is %v, want none", name, report)
		}
		want, err := r.Render(lookSVG(t, c, theme.Light))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds() != want.Bounds() {
			t.Fatalf("%s: the size is %v, want %v", name, img.Bounds(), want.Bounds())
		}
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			for k := range 4 {
				if d := int(img.Pix[i+k]) - int(want.Pix[i+k]); d > 1 || d < -1 {
					n++
					break
				}
			}
		}
		if share := float64(n) / float64(len(img.Pix)/4); share > 0.01 {
			t.Errorf("%s: %.2f%% of the pixels differ, more than 1%%", name, 100*share)
		}
	}
}

// TestLookOfMermaid checks that look: handDrawn of a Mermaid diagram, in
// the config or in the section of its kind, selects the look like
// Excalidraw, that classic and neo draw in the standard look, that the
// look of the caller wins, and that handDrawnSeed seeds it (D35 part 12,
// D40).
func TestLookOfMermaid(t *testing.T) {
	t.Parallel()
	const body = "flowchart LR\n  a[Start] --> b[End]\n"
	draw := func(front string, ropts ...RenderOption) (string, Report) {
		t.Helper()
		src := body
		if front != "" {
			src = "---\nconfig:\n" + front + "---\n" + body
		}
		c, _, err := Parse([]byte(src), lookFont)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		report, err := c.SVG(t.Context(), &b, append([]RenderOption{WithLocation(time.UTC), lookFont}, ropts...)...)
		if err != nil {
			t.Fatal(err)
		}
		return b.String(), report
	}
	standard, _ := draw("")
	hand, report := draw("  look: handDrawn\n")
	if len(report) != 0 {
		t.Errorf("the report of look: handDrawn is %v, want none", report)
	}
	if hand == standard || !strings.Contains(hand, "Excalifont") || strings.Contains(hand, "<rect x=") {
		t.Error("look: handDrawn does not draw in the look like Excalidraw")
	}
	if got, _ := draw("  flowchart:\n    look: handDrawn\n"); got != hand {
		t.Error("flowchart.look: handDrawn draws other than look: handDrawn")
	}
	for _, look := range []string{"classic", "neo"} {
		if got, _ := draw("  look: " + look + "\n"); got != standard {
			t.Errorf("look: %s does not draw in the standard look", look)
		}
	}
	if got, _ := draw("  look: handDrawn\n", WithLook(theme.LookStandard)); got != standard {
		t.Error("the look of the render does not win over look: handDrawn")
	}
	if got, _ := draw("  look: handDrawn\n  handDrawnSeed: 0\n"); got != hand {
		t.Error("handDrawnSeed: 0 does not keep the seed of zuzu")
	}
	seven, _ := draw("  look: handDrawn\n  handDrawnSeed: 7\n")
	if again, _ := draw("  look: handDrawn\n  handDrawnSeed: 7\n"); seven == hand || again != seven {
		t.Error("handDrawnSeed: 7 does not give its own drawing, the same each time")
	}
	if got, _ := draw("  look: handDrawn\n  handDrawnSeed: 1\n"); got != hand {
		t.Error("handDrawnSeed: 1 draws other than the seed of zuzu, which is 1")
	}
	if got, _ := draw("  look: handDrawn\n  handDrawnSeed: 7\n", WithLookSeed(1)); got != hand {
		t.Error("the seed of the render does not win over handDrawnSeed")
	}
}

// TestLookOfTheSpec checks the keys zuzu.look and zuzu.lookSeed of the
// config of a Vega-Lite spec, and that WithLook of the parse holds for
// each render that gives none.
func TestLookOfTheSpec(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/look/bar.vl.json")
	if err != nil {
		t.Fatal(err)
	}
	withConfig := func(cfg string) []byte {
		return bytes.Replace(src, []byte(`"title":`), []byte(`"config": `+cfg+`, "title":`), 1)
	}
	draw := func(b []byte, popts []Option, ropts ...RenderOption) string {
		t.Helper()
		c, _, err := Parse(b, append([]Option{WithStrict(true)}, popts...)...)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if _, err := c.SVG(t.Context(), &out, append([]RenderOption{WithLocation(time.UTC), lookFont}, ropts...)...); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	byOption := draw(src, []Option{WithLook(theme.LookExcalidraw)})
	if got := draw(withConfig(`{"zuzu.look": "excalidraw"}`), nil); got != byOption {
		t.Error("zuzu.look excalidraw draws other than WithLook")
	}
	if got := draw(src, nil, WithLook(theme.LookExcalidraw)); got != byOption {
		t.Error("WithLook of the render draws other than WithLook of the parse")
	}
	if got := draw(src, []Option{WithLook(theme.LookExcalidraw)}, WithLook(theme.LookStandard)); got != draw(src, nil) {
		t.Error("the look of the render does not win over the look of the parse")
	}
	seeded := draw(withConfig(`{"zuzu.look": "excalidraw", "zuzu.lookSeed": 42}`), nil)
	if seeded == byOption {
		t.Error("zuzu.lookSeed 42 draws as the seed of zuzu")
	}
	if got := draw(withConfig(`{"zuzu.look": "excalidraw", "zuzu.lookSeed": 1}`), nil); got != byOption {
		t.Error("zuzu.lookSeed 1 draws other than the seed of zuzu, which is 1")
	}
	look := WithLook(theme.LookExcalidraw)
	if got := draw(src, []Option{look, WithLookSeed(42)}); got != seeded {
		t.Error("WithLookSeed 42 of the parse draws other than zuzu.lookSeed 42")
	}
	if got := draw(src, []Option{look}, WithLookSeed(42)); got != seeded {
		t.Error("WithLookSeed 42 of the render draws other than zuzu.lookSeed 42")
	}
	if got := draw(withConfig(`{"zuzu.lookSeed": 42}`), []Option{look, WithLookSeed(1)}); got != byOption {
		t.Error("the seed of the parse does not win over zuzu.lookSeed")
	}
	if got := draw(src, []Option{look, WithLookSeed(42)}, WithLookSeed(1)); got != byOption {
		t.Error("the seed of the render does not win over the seed of the parse")
	}
	if got := draw(src, []Option{look}, WithLookSeed(0)); got != byOption {
		t.Error("WithLookSeed 0 draws other than the seed of zuzu, which is 1")
	}
}

// TestLookReport checks the report of the look: a look that zuzu does not
// have and a seed that is not a whole number are issues in the loose mode
// and errors in the strict mode. A look with no font is a warning in both
// modes, because its text still draws, in IBM Plex Sans. A gradient is no
// issue, because the look keeps it (D40 part 8).
func TestLookReport(t *testing.T) {
	t.Parallel()
	const gradient = `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 40, "height": 30,
		"marks": [{"type": "rect", "name": "box", "encode": {"enter": {"x": {"value": 0}, "y": {"value": 0},
		"width": {"value": 40}, "height": {"value": 30},
		"fill": {"value": {"gradient": "linear", "stops": [{"offset": 0, "color": "red"}, {"offset": 1, "color": "blue"}]}}}}}]}`
	bar, err := os.ReadFile("testdata/look/bar.vl.json")
	if err != nil {
		t.Fatal(err)
	}
	withConfig := func(cfg string) string {
		return string(bytes.Replace(bar, []byte(`"title":`), []byte(`"config": `+cfg+`, "title":`), 1))
	}
	cases := []struct {
		name    string
		src     string
		warning bool
		ropts   []RenderOption
		err     error
		text    string
	}{
		{"an unknown look of the spec", withConfig(`{"zuzu.look": "xkcd"}`), false, nil, vega.ErrUnsupported, `the look "xkcd"`},
		{"an unknown look of the caller", string(bar), false, []RenderOption{WithLook("chalk")}, vega.ErrUnsupported, `the look "chalk"`},
		{"an unknown look of Mermaid", "---\nconfig:\n  look: sketchy\n---\nflowchart LR\n  a --> b\n", false, nil, vega.ErrUnsupported, `the look "sketchy"`},
		{"a seed that is not whole", withConfig(`{"zuzu.look": "excalidraw", "zuzu.lookSeed": 1.5}`), false, []RenderOption{lookFont}, vega.ErrUnsupported, "the seed 1.5"},
		{"no font", string(bar), true, []RenderOption{WithLook(theme.LookExcalidraw)}, ErrNoLookFont, "draws its text in IBM Plex Sans"},
	}
	for _, c := range cases {
		for _, strict := range []bool{false, true} {
			ch, _, err := Parse([]byte(c.src), WithStrict(strict))
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			var b bytes.Buffer
			report, err := ch.SVG(t.Context(), &b, append([]RenderOption{WithLocation(time.UTC)}, c.ropts...)...)
			if strict && !c.warning {
				if !errors.Is(err, c.err) || !strings.Contains(err.Error(), c.text) {
					t.Errorf("%s: the strict mode gives the error %v, want %v with %q", c.name, err, c.err, c.text)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			found := false
			for _, is := range report {
				found = found || errors.Is(is, c.err) && strings.Contains(is.Error(), c.text)
			}
			if !found {
				t.Errorf("%s: the report %v has no issue %v with %q", c.name, report, c.err, c.text)
			}
		}
	}
	ch, _, err := Parse([]byte(gradient), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	report, err := ch.SVG(t.Context(), &b, WithLocation(time.UTC), WithLook(theme.LookExcalidraw), lookFont)
	if err != nil || len(report) != 0 {
		t.Errorf("a gradient gives the report %v and the error %v, want neither", report, err)
	}
	if !strings.Contains(b.String(), `stroke="url(#`) {
		t.Error("the hachure of a gradient does not draw in the gradient")
	}
}

// TestLookMeasuresWhenItParses checks that a kind of Mermaid that lays
// out when it compiles measures its text in Excalifont when the parse has
// the look and its font, so the layout differs (D40 part 1).
func TestLookMeasuresWhenItParses(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("testdata/look/sequence.mmd")
	if err != nil {
		t.Fatal(err)
	}
	width := func(opts ...Option) float64 {
		c, _, err := Parse(src, opts...)
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := c.Render(t.Context(), WithLook(theme.LookExcalidraw), lookFont)
		if err != nil {
			t.Fatal(err)
		}
		return s.Width
	}
	plain, measured := width(), width(WithLook(theme.LookExcalidraw), lookFont)
	if plain == measured {
		t.Errorf("the sequence diagram is %g wide with and without the look of the parse", plain)
	}
	// A source that asks for the look measures in its font too.
	if got := width(lookFont, WithDialect(Mermaid)); got != plain {
		t.Errorf("a source with no look measures %g wide, want %g", got, plain)
	}
	hand := append([]byte("---\nconfig:\n  look: handDrawn\n---\n"), src...)
	c, _, err := Parse(hand, lookFont)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), lookFont)
	if err != nil {
		t.Fatal(err)
	}
	if s.Width != measured {
		t.Errorf("look: handDrawn measures %g wide, want %g", s.Width, measured)
	}
}

// TestLookSeedsFollowTheMarks checks that the seed of an item comes from
// the name of its mark and its index, not from the order of a walk: the
// paths of a mark stay the same when another mark comes before it (D39
// part 6).
func TestLookSeedsFollowTheMarks(t *testing.T) {
	t.Parallel()
	mark := func(name string, x int) string {
		return `{"type": "rect", "name": "` + name + `", "encode": {"enter": {"x": {"value": ` + strconv.Itoa(x) +
			`}, "y": {"value": 5}, "width": {"value": 30}, "height": {"value": 20}, "fill": {"value": "#5b6ee1"}, "stroke": {"value": "#1b2230"}}}}`
	}
	spec := func(marks ...string) string {
		return `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 100, "height": 40, "marks": [` + strings.Join(marks, ",") + `]}`
	}
	paths := func(src string) map[string]string {
		c, _, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := c.Render(t.Context(), WithLook(theme.LookExcalidraw), lookFont)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, m := range s.Root.Items[0].Items {
			var b strings.Builder
			for _, it := range m.Items {
				b.WriteString(it.Path)
			}
			out[m.Name] = b.String()
		}
		return out
	}
	one := paths(spec(mark("a", 10)))
	two := paths(spec(mark("b", 50), mark("a", 10)))
	if one["a"] == "" || one["a"] != two["a"] {
		t.Error("the paths of the mark a change when the mark b comes before it")
	}
	if two["a"] == two["b"] {
		t.Error("the marks a and b draw the same paths")
	}
}

// TestLookKeepsTheStandardLook checks that a chart with no look draws no
// mark of the pass.
func TestLookKeepsTheStandardLook(t *testing.T) {
	t.Parallel()
	c, _, err := Parse([]byte("flowchart LR\n  subgraph g\n    a --> b\n  end\n"), lookFont)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), lookFont)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(m *scene.Mark) bool
	walk = func(m *scene.Mark) bool {
		if m.Role == sketch.RoleBackground {
			return true
		}
		for i := range m.Items {
			if strings.Contains(m.Items[i].Font, "Excalifont") {
				return true
			}
			for j := range m.Items[i].Items {
				if walk(&m.Items[i].Items[j]) {
					return true
				}
			}
		}
		return false
	}
	if walk(&s.Root) {
		t.Error("the standard look draws a part of the look like Excalidraw")
	}
}
