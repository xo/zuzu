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

	"github.com/xo/zuzu/scene"
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
// a mark that sets none.
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
	if s := points.Items[0].Stroke; !s.IsZero() {
		t.Errorf("the symbol has the stroke %v, want none", s)
	}
}
