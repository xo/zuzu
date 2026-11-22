package vegalite

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
)

// lookSpec is a Vega-Lite spec of rows a, b and c, with the rest of its
// keys.
func lookSpec(rest string) string {
	return `{"data": {"values": [{"a": 1, "b": 3, "c": 4}, {"a": 2, "b": 5, "c": 6}]},` + rest + `}`
}

// TestRangedAreaStyle checks that an area with an end of its own, or
// whose scale leaves out zero, takes the style of a band, and that an
// area from zero keeps the style of the gradient (D48 part 3).
func TestRangedAreaStyle(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, enc, want string
	}{
		{"an area from zero", `"y": {"field": "b", "type": "quantitative"}`, theme.StyleArea},
		{"an area with y2", `"y": {"field": "b", "type": "quantitative"}, "y2": {"field": "c"}`, theme.StyleAreaRanged},
		{"an area with zero off", `"y": {"field": "b", "type": "quantitative", "scale": {"zero": false}}`, theme.StyleAreaRanged},
	} {
		s := compile(t, lookSpec(`"mark": "area", "encoding": {"x": {"field": "a", "type": "quantitative"}, `+c.enc+`}`))
		if got := s.Marks[0].Style.List; !slices.Contains(got, c.want) {
			t.Errorf("%s has the styles %v, want %s", c.name, got, c.want)
		}
	}
}

// TestAreaBaseline checks that an area that does not stack and whose
// scale leaves out zero runs down to the bottom of its domain, as in
// vega-lite.js. A mean does not stack.
func TestAreaBaseline(t *testing.T) {
	t.Parallel()
	m := compileMap(t, lookSpec(`"mark": "area", "encoding": {"x": {"field": "a", "type": "quantitative"},
		"y": {"field": "b", "aggregate": "mean", "type": "quantitative", "scale": {"zero": false}}}`))
	b, _ := json.Marshal(find(m, "marks", "marks")["encode"])
	if !strings.Contains(string(b), `inrange(0, domain(\"y\")) ? 0 : domain(\"y\")[0]`) {
		t.Errorf("the encode of the area is %s, want y2 at the bottom of the domain", b)
	}
}

// TestDomainMidDiverges checks that a color scale with a middle of its
// domain takes the diverging range, as in vega-lite.js (D48 part 4).
func TestDomainMidDiverges(t *testing.T) {
	t.Parallel()
	s := compile(t, lookSpec(`"mark": "point", "encoding": {"x": {"field": "a", "type": "quantitative"},
		"color": {"field": "b", "type": "quantitative", "scale": {"domainMid": 4}}}`))
	for _, sc := range s.Scales {
		if sc.Name == "color" && sc.Range.Name != "diverging" {
			t.Errorf("the color scale has the range %q, want diverging", sc.Range.Name)
		}
	}
}

// TestPointOpacityOfTheSpec checks that the opacity of a point that the
// mark or the config sets wins over the 0.7 of vega-lite.js (D48 part 5).
func TestPointOpacityOfTheSpec(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, mark, config, want string
	}{
		{"no opacity", `"point"`, `{}`, `"opacity":{"value":0.7}`},
		{"the opacity of the mark", `{"type": "point", "opacity": 0.3}`, `{}`, `"opacity":{"value":0.3}`},
		{"the opacity of the config", `"circle"`, `{"circle": {"opacity": 1}}`, `"opacity":{"value":1}`},
	} {
		m := compileMap(t, lookSpec(`"mark": `+c.mark+`, "config": `+c.config+`,
			"encoding": {"x": {"field": "a", "type": "quantitative"}}`))
		b, _ := json.Marshal(find(m, "marks", "marks")["encode"], json.Deterministic(true))
		if !strings.Contains(string(b), c.want) {
			t.Errorf("%s: the encode is %s, want %s", c.name, b, c.want)
		}
	}
}

// TestViewPaint checks that the paint of the view of the spec paints the
// group of the top level, and that the paint of the view of the config
// goes in the style cell, as in vega-lite.js (D48 part 5).
func TestViewPaint(t *testing.T) {
	t.Parallel()
	m := compileMap(t, lookSpec(`"mark": "point", "view": {"fill": "yellow"}, "config": {"view": {"stroke": "red", "step": 10}},
		"encoding": {"x": {"field": "a", "type": "quantitative"}}`))
	b, _ := json.Marshal(m["encode"], json.Deterministic(true))
	if string(b) != `{"update":{"fill":{"value":"yellow"}}}` {
		t.Errorf("the encode of the top level is %s", b)
	}
	b, _ = json.Marshal(m["config"], json.Deterministic(true))
	if !strings.Contains(string(b), `"style":{"cell":{"stroke":"red"}}`) {
		t.Errorf("the config is %s, want the stroke of the view in the style cell", b)
	}
}

// TestErrorbandBorders checks that the properties of the borders and of
// the band of an error band that the spec gives win (D48 part 5).
func TestErrorbandBorders(t *testing.T) {
	t.Parallel()
	m := compileMap(t, lookSpec(`"mark": {"type": "errorband", "borders": {"strokeDash": [6, 4]}, "band": {"opacity": 0.5}},
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}`))
	b, _ := json.Marshal(m["marks"], json.Deterministic(true))
	if !strings.Contains(string(b), `"strokeDash":{"value":[6,4]}`) || !strings.Contains(string(b), `"opacity":{"value":0.5}`) {
		t.Errorf("the marks are %s, want the dash of the borders and the opacity of the band", b)
	}
}
