package zuzu

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// update writes the golden files again.
var update = flag.Bool("update", false, "write the golden files")

// chart parses a spec of testdata.
func chart(t *testing.T, name string, opts ...Option) *Chart {
	t.Helper()
	src, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := Parse(src, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestRenderBar renders a bar chart and compares the SVG with its golden
// file, testdata/bar.svg.
func TestRenderBar(t *testing.T) {
	t.Parallel()
	c := chart(t, "bar.vg.json")
	ctx := t.Context()
	var b bytes.Buffer
	report, err := c.SVG(ctx, &b, WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("report = %v, want none", report)
	}
	golden := "testdata/bar.svg"
	if *update {
		if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), want) {
		t.Errorf("the SVG differs from %s: run go test -update and look at it", golden)
	}
	// The same spec draws the same bytes again.
	var again bytes.Buffer
	if _, err := c.SVG(ctx, &again, WithLocation(time.UTC)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Error("two renders of the same spec differ")
	}
}

func TestView(t *testing.T) {
	t.Parallel()
	c := chart(t, "bar.vg.json")
	v, _, err := c.NewView(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("table")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 || rows[4]["y0"] != 28.0 || rows[4]["y1"] != 109.0 {
		t.Errorf("table = %v", rows)
	}
	y, err := v.Scale("y")
	if err != nil {
		t.Fatal(err)
	}
	if d := y.Domain(); len(d) != 2 || d[0] != 0.0 || d[1] != 180.0 {
		t.Errorf("the domain of y = %v, want [0 180]", d)
	}
	var bars *scene.Mark
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		if m.Name == "bars" {
			bars = m
		}
		for i := range m.Items {
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&v.Scene().Root)
	if bars == nil || len(bars.Items) != 8 {
		t.Fatalf("bars = %v, want a mark of 8 items", bars)
	}
	if it := bars.Items[0]; it.Height <= 0 || it.Width <= 0 {
		t.Errorf("the first bar is %v by %v", it.Width, it.Height)
	}
	if w, err := v.Signal("width"); err != nil || w != 320.0 {
		t.Errorf("width = %v, %v", w, err)
	}
}

func TestFeatureMode(t *testing.T) {
	t.Parallel()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"data": [{"name": "d", "values": [{"a": 1}], "transform": [{"type": "nosuch"}]}],
		"marks": [{"type": "rect", "from": {"data": "d"}}]}`)
	loose, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	_, report, err := loose.Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupportedTransform) || !errors.Is(report[0], vega.ErrUnsupported) {
		t.Errorf("report = %v, want one unsupported transform", report)
	}
	strict, _, err := Parse(src, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := strict.Render(t.Context()); !errors.Is(err, vega.ErrUnsupportedTransform) {
		t.Errorf("strict render = %v, want %v", err, vega.ErrUnsupportedTransform)
	}
}

// TestLinksAndTooltips fills the link and the tooltip of each item from the
// encodings href and tooltip, writes them in the SVG, and reports a link
// whose scheme zuzu does not write.
func TestLinksAndTooltips(t *testing.T) {
	t.Parallel()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"encode": {"update": {"tooltip": {"value": "the chart"}}},
		"data": [{"name": "d", "values": [
			{"a": 1, "u": "https://example.com/1", "l": [1, [2]], "o": {"p": 1}},
			{"a": 2, "u": "javascript:alert(2)"},
			{"a": 3, "u": "javascript:alert(3)"}]}],
		"marks": [{"type": "rect", "from": {"data": "d"}, "encode": {"update": {
			"x": {"field": "a"}, "width": {"value": 1}, "height": {"value": 1},
			"href": {"field": "u"},
			"tooltip": {"signal": "datum.a === 1 ? {b: datum.l, a: datum.a, o: datum.o} : datum.a === 2 ? [datum.a, 'x'] : null"}}}}]}`)
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	s, report, err := c.Render(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupported) || !strings.Contains(report[0].Error(), "javascript:alert(2)") {
		t.Errorf("report = %v, want one issue for the link javascript:alert(2)", report)
	}
	if got := s.Root.Items[0].Tooltip; got != "the chart" {
		t.Errorf("the tooltip of the root = %q, want %q", got, "the chart")
	}
	items := s.Root.Items[0].Items[0].Items
	for i, want := range []struct {
		href    string
		tooltip scene.Tooltip
	}{
		{"https://example.com/1", "a: 1\nb: [1, […]]\no: {…}"},
		{"", "[2, x]"},
		{"", ""},
	} {
		if items[i].Href != want.href || items[i].Tooltip != want.tooltip {
			t.Errorf("item %d: got %q and %q, want %q and %q", i, items[i].Href, items[i].Tooltip, want.href, want.tooltip)
		}
	}
	var b bytes.Buffer
	if _, err := c.SVG(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<a href="https://example.com/1">`, "<title>a: 1\nb: [1, […]]\no: {…}</title>", "<title>the chart</title>"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the SVG does not hold %s:\n%s", want, b.String())
		}
	}
	strict, _, err := Parse(src, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := strict.Render(t.Context()); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("strict render = %v, want %v", err, vega.ErrUnsupported)
	}
}

func TestData(t *testing.T) {
	t.Parallel()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"data": [{"name": "d", "url": "d.csv", "format": {"type": "csv", "parse": {"a": "number"}}}]}`)
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Render(t.Context()); !errors.Is(err, ErrNoLoader) {
		t.Errorf("render with no loader = %v, want %v", err, ErrNoLoader)
	}
	loader := LoaderFunc(func(_ context.Context, req LoadRequest) (io.ReadCloser, error) {
		if req.URL != "d.csv" || req.Format != "csv" || req.Name != "d" {
			t.Errorf("request = %+v", req)
		}
		return io.NopCloser(strings.NewReader("a\n1\n2\n")), nil
	})
	v, _, err := c.NewView(t.Context(), WithLoader(loader))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("d")
	if err != nil || len(rows) != 2 || rows[1]["a"] != 2.0 {
		t.Errorf("d = %v, %v", rows, err)
	}
	v, _, err = c.NewView(t.Context(), WithValues("d", []map[string]any{{"a": 7}}))
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := v.Data("d"); len(rows) != 1 || rows[0]["a"] != 7.0 {
		t.Errorf("d with values = %v", rows)
	}
}

func TestDialect(t *testing.T) {
	t.Parallel()
	if _, _, err := Parse([]byte(`{"marks": []}`)); !errors.Is(err, ErrUnknownDialect) {
		t.Errorf("Parse with no $schema = %v, want %v", err, ErrUnknownDialect)
	}
	if _, _, err := Parse([]byte(`{"marks": []}`), WithDialect(Vega)); err != nil {
		t.Errorf("Parse with WithDialect = %v", err)
	}
	if _, _, err := Parse([]byte(`{"$schema": "https://example.com/x.json"}`)); !errors.Is(err, ErrUnknownDialect) {
		t.Errorf("Parse with another $schema = %v, want %v", err, ErrUnknownDialect)
	}
}

func TestRenderVegaLite(t *testing.T) {
	t.Parallel()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
		"data": {"values": [{"a": "A", "b": 28}, {"a": "B", "b": 55}, {"a": "C", "b": 43}]},
		"mark": "bar",
		"encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}}`)
	c, report, err := Parse(src)
	if err != nil || len(report) != 0 {
		t.Fatalf("Parse = %v, %v", report, err)
	}
	v, _, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.Data("data_0")
	if err != nil || len(rows) != 3 || rows[1]["b_end"] != 55.0 {
		t.Errorf("data_0 = %v, %v", rows, err)
	}
	var b bytes.Buffer
	if _, err := c.SVG(t.Context(), &b); err != nil || !bytes.Contains(b.Bytes(), []byte("<svg")) {
		t.Errorf("SVG = %v", err)
	}
}

// findMark returns the first mark of a name in a scene.
func findMark(m *scene.Mark, name string) *scene.Mark {
	if m.Name == name {
		return m
	}
	for i := range m.Items {
		for j := range m.Items[i].Items {
			if f := findMark(&m.Items[i].Items[j], name); f != nil {
				return f
			}
		}
	}
	return nil
}

// TestLabelMarks runs the transform label on the text of the items of a
// mark, and checks the places that it writes back, and the default paint of
// a mark that sets none, from the config of the theme.
func TestLabelMarks(t *testing.T) {
	t.Parallel()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 200, "height": 100,
		"data": [{"name": "d", "values": [{"x": 50, "y": 50, "t": "first"}, {"x": 190, "y": 50, "t": "second"}, {"x": 100, "y": 4, "t": "a text too long to fit anywhere at all"}]}],
		"marks": [
			{"name": "points", "type": "symbol", "from": {"data": "d"},
				"encode": {"enter": {"x": {"field": "x"}, "y": {"field": "y"}, "size": {"value": 16}}}},
			{"name": "base", "type": "rule",
				"encode": {"enter": {"x": {"value": 0}, "x2": {"value": 200}, "y": {"value": 90}}}},
			{"name": "labels", "type": "text", "from": {"data": "points"},
				"encode": {"enter": {"text": {"field": "datum.t"}}},
				"transform": [{"type": "label", "size": {"signal": "[width, height]"}, "anchor": ["right", "left"], "avoidMarks": ["base"]}]}
		]}`)
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := c.NewView(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	root := &v.Scene().Root
	labels := findMark(root, "labels")
	if labels == nil || len(labels.Items) != 3 {
		t.Fatalf("labels = %v, want a mark of 3 items", labels)
	}
	// The first label goes to the right of its point, which has a radius
	// of 2, and the second, at the right edge, goes to the left.
	for i, want := range []struct {
		x       float64
		align   string
		opacity float64
	}{{53, "left", 1}, {187, "right", 1}, {0, "", 0}} {
		it := labels.Items[i]
		if it.Opacity == nil || *it.Opacity != want.opacity {
			t.Errorf("label %d: the opacity is %v, want %v", i, it.Opacity, want.opacity)
		}
		if want.opacity == 0 {
			continue
		}
		if it.X != want.x || it.Align != want.align || it.Baseline != "middle" {
			t.Errorf("label %d: x %v, align %q, baseline %q, want %v, %q, middle", i, it.X, it.Align, it.Baseline, want.x, want.align)
		}
	}
	points, base := findMark(root, "points"), findMark(root, "base")
	if points == nil || base == nil {
		t.Fatal("no points or no base")
	}
	if p := points.Items[0].Fill.Color; p == "" {
		t.Error("a symbol with no fill and no stroke has no default fill")
	}
	if s := base.Items[0].Stroke.Color; s == "" {
		t.Error("a rule with no stroke has no default stroke")
	}
	// A point has a ring in the background color (docs/STYLE.md), but for
	// one as small as these, of 16 square pixels.
	if s := points.Items[0].Stroke; !s.IsZero() {
		t.Errorf("the small symbol has the stroke %v, want none", s)
	}
}

// TestImage draws the bar chart as an image and compares it with the image
// that okage draws from its SVG, at most 1 percent of the pixels apart, the
// gate of okage D31 (D11).
func TestImage(t *testing.T) {
	t.Parallel()
	c := chart(t, "bar.vg.json")
	img, report, err := c.Image(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 0 {
		t.Errorf("report = %v, want none", report)
	}
	var b bytes.Buffer
	if _, err := c.SVG(t.Context(), &b, WithLocation(time.UTC)); err != nil {
		t.Fatal(err)
	}
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
	want, err := r.Render(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != want.Bounds() {
		t.Fatalf("the size is %v, want %v", img.Bounds(), want.Bounds())
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
		t.Errorf("%.2f%% of the pixels differ, more than 1%%", 100*share)
	}
}

// TestBarEnds checks the look of zuzu on the bars of a Vega-Lite spec: the
// color of the theme, and a round end only on the last bar of each stack,
// which the config of the spec turns off.
func TestBarEnds(t *testing.T) {
	t.Parallel()
	spec := func(config string) []byte {
		return []byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
			"data": {"values": [{"a": "x", "b": 1, "c": "p"}, {"a": "x", "b": 2, "c": "q"}, {"a": "y", "b": 3, "c": "p"}]},
			"mark": "bar",
			"encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative", "aggregate": "sum"}, "color": {"field": "c"}}` + config + `}`)
	}
	bars := func(src []byte) []scene.Item {
		t.Helper()
		c, _, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		var out []scene.Item
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			for i := range m.Items {
				if m.Type == scene.Rect && m.Role == "mark" {
					out = append(out, m.Items[i])
				}
				for j := range m.Items[i].Items {
					walk(&m.Items[i].Items[j])
				}
			}
		}
		walk(&s.Root)
		return out
	}
	items := bars(spec(""))
	if len(items) != 3 {
		t.Fatalf("%d bars, want 3", len(items))
	}
	rounded := 0
	for _, it := range items {
		if it.CornerRadiusTopLeft == 3 && it.CornerRadiusTopRight == 3 {
			rounded++
		}
		if it.CornerRadiusBottomLeft != 0 {
			t.Errorf("a bar has a round bottom: %+v", it)
		}
	}
	// The stack at x has two bars, and only its top one is round.
	if rounded != 2 {
		t.Errorf("%d bars have a round end, want 2", rounded)
	}
	for _, it := range bars(spec(`, "config": {"style": {"zuzu.bar.vertical": {"zuzu.cornerRadiusEnd": 0}}}`)) {
		if it.CornerRadiusTopLeft != 0 {
			t.Errorf("the config does not turn the round end off: %+v", it)
		}
	}
	// A bar with no color takes the first color of the theme.
	one := bars([]byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
		"data": {"values": [{"a": "x", "b": 1}]}, "mark": "bar",
		"encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}}`))
	if len(one) != 1 || one[0].Fill.Color != "#5b6ee1" {
		t.Errorf("the bar is %+v, want the fill #5b6ee1", one)
	}
}

// guideScene renders a Vega spec of 200 by 100 with one bar per value of a
// field and the guides that it names.
func guideScene(t *testing.T, guides string) *scene.Scene {
	t.Helper()
	src := []byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"width": 200, "height": 100,
		"data": [{"name": "d", "values": [{"k": "a", "v": 1}, {"k": "b", "v": 2}, {"k": "c", "v": 3}]}],
		"scales": [
			{"name": "x", "type": "band", "domain": {"data": "d", "field": "k"}, "range": "width"},
			{"name": "y", "type": "linear", "domain": {"data": "d", "field": "v"}, "range": "height"},
			{"name": "c", "type": "ordinal", "domain": {"data": "d", "field": "k"}, "range": "category"}],
		"marks": [{"type": "rect", "from": {"data": "d"}, "encode": {"enter": {
			"x": {"scale": "x", "field": "k"}, "width": {"scale": "x", "band": 1},
			"y": {"scale": "y", "field": "v"}, "y2": {"scale": "y", "value": 0}, "fill": {"scale": "c", "field": "k"}}}}]` + guides + `}`)
	c, _, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// guide returns the first group of a role in a scene, with its place in
// the frame of the plot area.
func guide(s *scene.Scene, role string) *scene.Item {
	var out *scene.Item
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			if m.Role == role && out == nil {
				out = &m.Items[i]
			}
			for j := range m.Items[i].Items {
				walk(&m.Items[i].Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

func TestLegendPlaces(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		legend string
		check  func(x, y float64) bool
	}{
		{`{"fill": "c"}`, func(_, y float64) bool { return y < 0 }},
		{`{"fill": "c", "orient": "left"}`, func(x, _ float64) bool { return x < -20 }},
		{`{"fill": "c", "orient": "right"}`, func(x, _ float64) bool { return x > 200 }},
		{`{"fill": "c", "orient": "bottom"}`, func(_, y float64) bool { return y > 100 }},
		{`{"fill": "c", "orient": "bottom-right"}`, func(x, y float64) bool { return x > 100 && x < 200 && y > 0 && y < 100 }},
		{`{"fill": "c", "orient": "none", "legendX": 7, "legendY": 9}`, func(x, y float64) bool { return x == 7 && y == 9 }},
	} {
		s := guideScene(t, `, "legends": [`+c.legend+`]`)
		g := guide(s, "legend")
		if g == nil {
			t.Fatalf("%s: no legend", c.legend)
		}
		if !c.check(g.X, g.Y) {
			t.Errorf("%s: the legend is at %v, %v", c.legend, g.X, g.Y)
		}
	}
}

func TestLegendLayout(t *testing.T) {
	t.Parallel()
	symbols := func(legend string) []scene.Item {
		g := guide(guideScene(t, `, "legends": [`+legend+`]`), "legend-entry")
		if g == nil {
			t.Fatalf("%s: no entries", legend)
		}
		for _, m := range g.Items {
			if m.Role == "legend-symbol" {
				return m.Items
			}
		}
		return nil
	}
	row := symbols(`{"fill": "c"}`)
	col := symbols(`{"fill": "c", "direction": "vertical"}`)
	grid := symbols(`{"fill": "c", "columns": 2}`)
	if len(row) != 3 || len(col) != 3 || len(grid) != 3 {
		t.Fatalf("the legends have %d, %d and %d entries, want 3", len(row), len(col), len(grid))
	}
	if row[0].Y != row[2].Y || row[0].X >= row[2].X {
		t.Errorf("a row: %v %v and %v %v", row[0].X, row[0].Y, row[2].X, row[2].Y)
	}
	if col[0].X != col[2].X || col[0].Y >= col[2].Y {
		t.Errorf("a column: %v %v and %v %v", col[0].X, col[0].Y, col[2].X, col[2].Y)
	}
	if grid[0].X != grid[2].X || grid[0].Y >= grid[2].Y || grid[0].Y != grid[1].Y {
		t.Errorf("a grid of 2: %v", grid)
	}
	// The encode of a legend sets the parts that it names.
	s := guideScene(t, `, "legends": [{"fill": "c", "encode": {"labels": {"update": {"fontSize": {"value": 20}}}}}]`)
	if l := guide(s, "legend-label"); l == nil || l.FontSize != 20 {
		t.Errorf("the encode of the labels gives %v", l)
	}
}

func TestAxisLabels(t *testing.T) {
	t.Parallel()
	labels := func(axis string) []scene.Item {
		s := guideScene(t, `, "axes": [`+axis+`]`)
		var out []scene.Item
		var walk func(m *scene.Mark)
		walk = func(m *scene.Mark) {
			for i := range m.Items {
				if m.Role == "axis-label" {
					out = append(out, m.Items[i])
				}
				for j := range m.Items[i].Items {
					walk(&m.Items[i].Items[j])
				}
			}
		}
		walk(&s.Root)
		return out
	}
	flush := labels(`{"scale": "y", "orient": "left", "labelFlush": true}`)
	if len(flush) < 2 || flush[0].Baseline != "bottom" || flush[len(flush)-1].Baseline != "top" {
		t.Errorf("the flush labels: %+v", flush)
	}
	if l := labels(`{"scale": "x", "orient": "bottom"}`); len(l) == 0 || l[0].Limit != 180 {
		t.Errorf("the limit of a label is not 180 by default: %+v", l)
	}
	if l := labels(`{"scale": "x", "orient": "bottom", "labelLimit": 30}`); len(l) == 0 || l[0].Limit != 30 {
		t.Errorf("the limit of a label is not 30: %+v", l)
	}
}

// TestAutosizeFit checks that the autosize fit keeps the size of the spec
// for the whole chart, the plot area smaller.
func TestAutosizeFit(t *testing.T) {
	t.Parallel()
	pad := guideScene(t, `, "padding": 0, "axes": [{"scale": "y", "orient": "left"}]`)
	fit := guideScene(t, `, "padding": 0, "autosize": "fit", "axes": [{"scale": "y", "orient": "left"}]`)
	if pad.Width <= 200 {
		t.Errorf("the pad is %v wide, want more than 200", pad.Width)
	}
	if fit.Width < 199 || fit.Width > 201 {
		t.Errorf("the fit is %v wide, want 200", fit.Width)
	}
}

// TestSizeOfAChart checks the size of the image of a chart of a nominal x,
// whose width Vega-Lite works out from a step: the autosize fit that holds
// the padding, and the size that the caller gives.
func TestSizeOfAChart(t *testing.T) {
	t.Parallel()
	spec := func(size string) []byte {
		return []byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",
			"data": {"values": [{"k": "a", "v": 1, "s": "p"}, {"k": "a", "v": 2, "s": "q"}, {"k": "b", "v": 3, "s": "p"}]},
			"mark": "bar", "padding": 5, "title": {"text": "T", "subtitle": "S"},
			"encoding": {"x": {"field": "k", "type": "nominal", "sort": null}, "y": {"field": "v", "type": "quantitative"},
				"color": {"field": "s"}, "xOffset": {"field": "s"}}` + size + `}`)
	}
	for _, c := range []struct {
		name string
		src  []byte
		opts []RenderOption
		w, h int
	}{
		{"fit with the padding", spec(`, "width": 320, "height": 240, "autosize": {"type": "fit", "contains": "padding"}`), nil, 320, 240},
		{"fit", spec(`, "width": 320, "height": 240, "autosize": {"type": "fit"}`), nil, 330, 250},
		{"the size of the caller", spec(`, "autosize": {"type": "fit", "contains": "padding"}`), []RenderOption{WithSize(320, 240)}, 320, 240},
	} {
		chart, _, err := Parse(c.src)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := chart.Image(t.Context(), append(c.opts, WithLocation(time.UTC))...)
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() < c.w-1 || b.Dx() > c.w+1 || b.Dy() < c.h-1 || b.Dy() > c.h+1 {
			t.Errorf("%s: the image is %dx%d, want %dx%d", c.name, b.Dx(), b.Dy(), c.w, c.h)
		}
	}
}
