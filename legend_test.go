package zuzu

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
)

// legendItems renders a spec and returns the items of the marks of a role,
// in order.
func legendItems(t *testing.T, src string, dialect Dialect, role string) []scene.Item {
	t.Helper()
	c, _, err := Parse([]byte(src), WithDialect(dialect), WithStrict(true))
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
			if m.Role == role {
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

// TestLegendSymbolsFollowMarks checks that the symbols of a legend take the
// shape, the fill and the color of the marks that it names
// (docs/STYLE.md). Before, a legend of a stroke drew hollow squares beside
// filled circles, a legend of sizes drew filled circles in the theme color
// beside hollow steelblue rings, and a line drew as a square.
func TestLegendSymbolsFollowMarks(t *testing.T) {
	t.Parallel()
	const head = `{"width": 200, "height": 100,
		"data": [{"name": "t", "values": [{"k": "a", "v": 1}, {"k": "b", "v": 3}]}],
		"scales": [
			{"name": "c", "type": "ordinal", "domain": ["a", "b"], "range": ["#ff0000", "#0000ff"]},
			{"name": "s", "type": "linear", "domain": [0, 3], "range": [4, 400]}],`
	tests := []struct {
		name, body    string
		shape         string
		fill, stroke  string
		lineOfSecond  bool
		strokeOfFirst bool
	}{
		{"filled point", `"legends": [{"stroke": "c"}],
			"marks": [{"type": "symbol", "from": {"data": "t"}, "encode": {"update": {
				"fill": {"scale": "c", "field": "k"}, "shape": {"value": "triangle"}}}}]}`,
			"triangle", "#ff0000", "", false, false},
		{"hollow point", `"legends": [{"stroke": "c"}],
			"marks": [{"type": "symbol", "from": {"data": "t"}, "encode": {"update": {
				"stroke": {"scale": "c", "field": "k"}}}}]}`,
			"circle", "", "#ff0000", false, false},
		{"ring of a size", `"legends": [{"size": "s"}],
			"marks": [{"type": "symbol", "from": {"data": "t"}, "encode": {"update": {
				"size": {"scale": "s", "field": "v"}, "fill": {"value": "transparent"}, "stroke": {"value": "#4682b4"}}}}]}`,
			"circle", "", "#4682b4", false, false},
		{"box before outlier", `"legends": [{"fill": "c"}],
			"marks": [
				{"type": "symbol", "from": {"data": "t"}, "encode": {"update": {"fill": {"scale": "c", "field": "k"}}}},
				{"type": "rect", "from": {"data": "t"}, "encode": {"update": {"fill": {"scale": "c", "field": "k"}, "width": {"value": 5}, "height": {"value": 5}}}}]}`,
			"square", "#ff0000", "", false, false},
		{"line of one value", `"legends": [{"fill": "c"}],
			"marks": [
				{"type": "area", "from": {"data": "t"}, "encode": {"update": {"fill": {"signal": "scale('c', 'a')"}}}},
				{"type": "line", "from": {"data": "t"}, "encode": {"update": {"stroke": {"signal": "scale('c', 'b')"}}}}]}`,
			"square", "#ff0000", "", true, false},
	}
	for _, test := range tests {
		syms := legendItems(t, head+test.body, Vega, "legend-symbol")
		if len(syms) < 2 {
			t.Fatalf("%s: %d symbols", test.name, len(syms))
		}
		got := syms[0]
		if got.Shape != test.shape || got.Fill.Color != test.fill || got.Stroke.Color != test.stroke {
			t.Errorf("%s: symbol = %q fill %q stroke %q, want %q fill %q stroke %q",
				test.name, got.Shape, got.Fill.Color, got.Stroke.Color, test.shape, test.fill, test.stroke)
		}
		if test.lineOfSecond && syms[1].Shape != "stroke" {
			t.Errorf("%s: the symbol of the line is %q, want stroke", test.name, syms[1].Shape)
		}
	}
}

// TestLegendProperties checks that the properties of a legend that a spec
// sets win over the look, as D46 says, both in Vega and through the
// compiler of Vega-Lite: clipHeight, symbolLimit and labelLimit.
func TestLegendProperties(t *testing.T) {
	t.Parallel()
	vl := `{"data": {"values": [{"a": 0}, {"a": 1000000}, {"a": 3000000}]},
		"mark": "circle",
		"encoding": {"size": {"field": "a", "type": "quantitative",
			"legend": {"clipHeight": 30, "labelLimit": 50}, "scale": {"rangeMax": 5000}}}}`
	clips := legendItems(t, vl, VegaLite, "legend-clip")
	if len(clips) == 0 {
		t.Fatal("clipHeight cut no symbol")
	}
	for _, c := range clips {
		if !c.Clip || c.Height != 30 {
			t.Errorf("clip = %+v, want a height of 30", c)
		}
	}
	labels := legendItems(t, vl, VegaLite, "legend-label")
	if len(labels) < 4 || labels[0].Text[0] != "0" || labels[len(labels)-1].Text[0] != "3,000,000" || labels[0].Limit != 50 {
		t.Errorf("%d labels from %q to %q with a limit of %v, want 0 to 3,000,000 with a limit of 50",
			len(labels), labels[0].Text, labels[len(labels)-1].Text, labels[0].Limit)
	}
	vg := `{"width": 200, "height": 100,
		"scales": [{"name": "c", "type": "ordinal", "domain": ["a", "b", "c", "d"], "range": "category"}],
		"legends": [{"fill": "c", "symbolLimit": 2}]}`
	var texts []string
	for _, it := range legendItems(t, vg, Vega, "legend-label") {
		texts = append(texts, it.Text...)
	}
	if want := []string{"a", "…3 entries"}; !slices.Equal(texts, want) {
		t.Errorf("labels = %q, want %q", texts, want)
	}
}

// TestLegendPiecesOfRamp checks that a scale that cuts numbers into pieces
// takes one color of a ramp or a scheme for each piece, as Vega counts
// them, so that its legend has an entry for each piece. Before, a
// threshold scale with two thresholds took two colors, and its legend
// lost the last piece.
func TestLegendPiecesOfRamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		scale string
		want  int
	}{
		{`{"name": "c", "type": "threshold", "domain": [30, 70], "range": "ramp"}`, 3},
		{`{"name": "c", "type": "threshold", "domain": [30, 70], "range": {"scheme": "blues"}}`, 3},
		{`{"name": "c", "type": "quantize", "domain": [0, 100], "range": "ramp"}`, 5},
	}
	for _, test := range tests {
		src := `{"width": 300, "height": 100, "scales": [` + test.scale + `], "legends": [{"fill": "c", "type": "symbol"}]}`
		if got := len(legendItems(t, src, Vega, "legend-symbol")); got != test.want {
			t.Errorf("%s: %d entries, want %d", test.scale, got, test.want)
		}
	}
}

// TestLegendUnreadProperties checks that a property of a legend that zuzu
// does not draw, such as titlePadding, goes in the report, and that a
// property that it draws, such as clipHeight, does not (hard rule 4).
func TestLegendUnreadProperties(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 100, "height": 100,
		"scales": [{"name": "c", "type": "ordinal", "domain": ["a", "b"], "range": "category"}],
		"legends": [{"fill": "c", "titlePadding": 4, "clipHeight": 10, "labelOverlap": true}]}`
	_, report, err := svgOf(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range report {
		got = append(got, is.Error())
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"titlePadding", "labelOverlap"} {
		if !strings.Contains(all, want) {
			t.Errorf("the report has no %s: %q", want, got)
		}
	}
	if strings.Contains(all, "clipHeight") {
		t.Errorf("the report names clipHeight, which zuzu draws: %q", got)
	}
}

// TestAxisUnreadProperties checks that a property of an axis that zuzu does
// not draw, such as tickBand, goes in the report (hard rule 4).
func TestAxisUnreadProperties(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 100, "height": 100,
		"scales": [{"name": "x", "type": "band", "domain": ["a", "b"], "range": "width"}],
		"axes": [{"scale": "x", "orient": "bottom", "tickBand": "extent"}]}`
	_, report, err := svgOf(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 1 || !strings.Contains(report[0].Error(), "tickBand") {
		t.Errorf("the report is %v, want tickBand", report)
	}
}

// TestClipToSphere checks that a clip of a mark to the sphere of a
// projection, or to a path, clips the mark to that path in the scene and
// in the SVG, as vega.js clips the graticule and the land of the maps of
// vega/projections, with nothing in the report.
func TestClipToSphere(t *testing.T) {
	t.Parallel()
	for _, clip := range []string{`{"sphere": "p"}`, `{"path": "M0,0L50,0L50,50Z"}`} {
		src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 100, "height": 100,
			"projections": [{"name": "p", "type": "orthographic", "scale": 40, "translate": [50, 50]}],
			"data": [{"name": "d", "values": [{"type": "Sphere"}]}],
			"marks": [{"type": "shape", "from": {"data": "d"}, "clip": ` + clip + `,
				"encode": {"update": {"fill": {"value": "#ff0000"}}},
				"transform": [{"type": "geoshape", "projection": "p"}]}]}`
		out, report, err := svgOf(t, src, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(report) != 0 {
			t.Errorf("%s: the report is %v, want none", clip, report)
		}
		if !regexp.MustCompile(`<clipPath id="[^"]*"><path d="M`).MatchString(out) {
			t.Errorf("%s: the SVG has no clip to a path:\n%s", clip, out)
		}
	}
}

// TestClipToPathInRaster checks that the direct renderer clips a mark to
// its path too: a red square clipped to a triangle paints the corner
// inside the triangle and leaves the corner outside it.
func TestClipToPathInRaster(t *testing.T) {
	t.Parallel()
	src := `{"$schema": "https://vega.github.io/schema/vega/v6.json", "width": 100, "height": 100, "padding": 0,
		"marks": [{"type": "rect", "clip": {"path": "M0,0L100,0L0,100Z"},
			"encode": {"update": {"x": {"value": 0}, "y": {"value": 0}, "width": {"value": 100}, "height": {"value": 100}, "fill": {"value": "#ff0000"}}}}]}`
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := c.Image(t.Context(), WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	in := img.RGBAAt(b.Min.X+b.Dx()/5, b.Min.Y+b.Dy()/5)
	out := img.RGBAAt(b.Max.X-b.Dx()/5, b.Max.Y-b.Dy()/5)
	if in.R < 200 || in.G > 80 {
		t.Errorf("the corner inside the path is %v, want red", in)
	}
	if out.R > 200 && out.G < 80 {
		t.Errorf("the corner outside the path is %v, want no red", out)
	}
}
