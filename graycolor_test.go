package zuzu

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
)

// renderVegaMode renders a Vega spec in a mode.
func renderVegaMode(t *testing.T, src string, m theme.Mode) *scene.Scene {
	t.Helper()
	c, _, err := Parse([]byte(`{"$schema": "https://vega.github.io/schema/vega/v5.json",` + src + `}`))
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(t.Context(), WithLocation(time.UTC), WithMode(m))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestTheDarkModeFlipsGrays checks that the dark mode flips the lightness
// of a gray near black or near white that the spec names, as a value, a
// value of a range, a gradient and the config, and keeps a color with a
// hue and a color of an expression (D48 part 1). The light mode keeps
// every color.
func TestTheDarkModeFlipsGrays(t *testing.T) {
	t.Parallel()
	src := `"width": 100, "height": 100, "config": {"rule": {"stroke": "#eeeeee"}},
		"data": [{"name": "d", "values": [{"k": "p"}, {"k": "q"}]}],
		"scales": [{"name": "c", "type": "ordinal", "domain": ["p", "q"], "range": ["black", "red"]}],
		"marks": [
			{"type": "text", "name": "t", "encode": {"enter": {"text": {"value": "a"}, "fill": {"value": "#000"}}}},
			{"type": "rule", "name": "u", "encode": {"enter": {"x": {"value": 0}, "x2": {"value": 5}}}},
			{"type": "rect", "name": "r", "encode": {"enter": {"width": {"value": 5}, "height": {"value": 5},
				"fill": {"value": "steelblue"}, "stroke": {"signal": "'white'"}}}},
			{"type": "symbol", "name": "s", "from": {"data": "d"}, "encode": {"enter": {"fill": {"scale": "c", "field": "k"}}}},
			{"type": "rect", "name": "g", "encode": {"enter": {"width": {"value": 5}, "height": {"value": 5},
				"fill": {"value": {"gradient": "linear", "stops": [{"offset": 0, "color": "white"}, {"offset": 1, "color": "red"}]}}}}}
		]`
	for _, c := range []struct {
		mode theme.Mode
		want []string
	}{
		{theme.Light, []string{"#000", "#eeeeee", "steelblue", "white", "black", "red", "white"}},
		{theme.Dark, []string{"#ffffff", "#111111", "steelblue", "white", "#ffffff", "red", "#000000"}},
	} {
		s := renderVegaMode(t, src, c.mode)
		byName := map[string]*scene.Mark{}
		for _, m := range marksOf(s, "mark") {
			byName[m.Name] = m
		}
		got := []string{
			byName["t"].Items[0].Fill.Color, byName["u"].Items[0].Stroke.Color,
			byName["r"].Items[0].Fill.Color, byName["r"].Items[0].Stroke.Color,
			byName["s"].Items[0].Fill.Color, byName["s"].Items[1].Fill.Color,
			byName["g"].Items[0].Fill.Gradient.Stops[0].Color,
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: the colors are %v, want %v", c.mode, got, c.want)
		}
	}
}

// TestALightBackgroundKeepsGrays checks that a chart whose spec draws on
// a light background keeps its grays in the dark mode (D48 part 1).
func TestALightBackgroundKeepsGrays(t *testing.T) {
	t.Parallel()
	s := renderVegaMode(t, `"width": 100, "height": 100, "background": "white",
		"marks": [{"type": "text", "encode": {"enter": {"text": {"value": "a"}, "fill": {"value": "black"}}}}]`, theme.Dark)
	if got := marksOf(s, "mark")[0].Items[0].Fill.Color; got != "black" {
		t.Errorf("the text is %s, want black", got)
	}
}

// TestALightBackgroundDrawsInTheLightMode checks that a chart whose spec
// paints a light background, also through a signal, draws its guides with
// the neutrals of the light mode in the dark mode, and keeps its grays,
// and that a dark background of a signal flips them (D48). Before, the
// labels of the axes of point_background were light gray on orange, and a
// background of a signal counted as the background of the mode.
func TestALightBackgroundDrawsInTheLightMode(t *testing.T) {
	t.Parallel()
	light, dark := theme.New(theme.Light, theme.Zuzu), theme.New(theme.Dark, theme.Zuzu)
	for _, c := range []struct {
		name, background, label, text string
	}{
		{"a light color", `"background": "orange"`, light.Muted, "black"},
		{"a light signal", `"signals": [{"name": "bg", "value": "white"}], "background": {"signal": "bg"}`, light.Muted, "black"},
		{"a dark signal", `"background": {"signal": "'#202020'"}`, dark.Muted, "#ffffff"},
		{"a light config", `"config": {"background": "#fafafa"}`, light.Muted, "black"},
		{"no background", `"padding": 0`, dark.Muted, "#ffffff"},
	} {
		s := renderVegaMode(t, `"width": 100, "height": 100, `+c.background+`,
			"scales": [{"name": "x", "type": "linear", "domain": [0, 10], "range": "width"}],
			"axes": [{"scale": "x", "orient": "bottom"}],
			"marks": [{"type": "text", "encode": {"enter": {"text": {"value": "a"}, "fill": {"value": "black"}}}}]`, theme.Dark)
		label := marksOf(s, "axis-label")
		if len(label) == 0 || label[0].Items[0].Fill.Color != c.label {
			t.Errorf("%s: the labels of the axis are %+v, want %s", c.name, label, c.label)
		}
		if got := marksOf(s, "mark")[0].Items[0].Fill.Color; got != c.text {
			t.Errorf("%s: the text is %s, want %s", c.name, got, c.text)
		}
	}
}

// TestEndLabelsFit checks that the look names the ends of lines only where
// the names fit (D48 part 2): not for a faint line of context, not for
// more than eight series, and not when the spec draws text of its own.
// The legend names the series then.
func TestEndLabelsFit(t *testing.T) {
	t.Parallel()
	rows := func(n int) string {
		parts := make([]string, 0, n)
		for i := range n {
			parts = append(parts, fmt.Sprintf(`{"a": 1, "b": %d, "c": "s%d"}, {"a": 2, "b": %d, "c": "s%d"}`, i*10, i, i*10, i))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	line := `{"mark": "line", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "color": {"field": "c"}}}`
	for _, c := range []struct {
		name, spec string
		labels     int
	}{
		{"three series", `"data": {"values": ` + rows(3) + `}, "layer": [` + line + `]`, 3},
		{"ten series", `"data": {"values": ` + rows(10) + `}, "layer": [` + line + `]`, 0},
		{"three series and a text", `"data": {"values": ` + rows(3) + `}, "layer": [` + line + `,
			{"mark": "text", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "text": {"field": "c"}}}]`, 0},
		{"three faint series", `"data": {"values": ` + rows(3) + `}, "layer": [{"mark": {"type": "line", "opacity": 0.2},
			"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "color": {"field": "c"}}}]`, 0},
	} {
		ch, _, err := Parse([]byte(`{"$schema": "https://vega.github.io/schema/vega-lite/v5.json",` + c.spec + `}`))
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := ch.Render(t.Context(), WithLocation(time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		if got := len(marksOf(s, "zuzu-end-label")); got != c.labels {
			t.Errorf("%s: %d names at the ends of the lines, want %d", c.name, got, c.labels)
		}
		if legend := len(marksOf(s, "legend")) != 0; legend == (c.labels > 0) {
			t.Errorf("%s: whether the legend draws is %v", c.name, legend)
		}
	}
}

// TestRangedArea checks that an area with an end of its own draws as a
// flat tint with no line on top, no gradient and no high, and that an
// opacity that the spec sets wins over the tint (D48 parts 3 and 5).
func TestRangedArea(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, mark string
		tint       bool
	}{
		{"a band", `"area"`, true},
		{"a band with an opacity", `{"type": "area", "opacity": 0.3}`, false},
	} {
		s := renderVL(t, `"transform": [{"filter": "datum.c == 'p'"}, {"calculate": "datum.b - 1", "as": "lo"}], "mark": `+c.mark+`,
			"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "y2": {"field": "lo"}}`)
		area := marksOf(s, "mark")[0].Items[0]
		if area.Fill.Gradient != nil {
			t.Errorf("%s has a gradient", c.name)
		}
		if tint := area.FillOpacity != nil && *area.FillOpacity == theme.RangedTint; tint != c.tint {
			t.Errorf("%s: whether it is a tint is %v, want %v", c.name, tint, c.tint)
		}
		if n := len(marksOf(s, "zuzu-area-line")) + len(marksOf(s, "zuzu-high-label")); n != 0 {
			t.Errorf("%s has a line on top or a high", c.name)
		}
	}
}

// TestOneHigh checks that the label of the high draws in one area only,
// and not in two layers of areas (D48 part 2).
func TestOneHigh(t *testing.T) {
	t.Parallel()
	area := `{"mark": "area", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}}`
	s := renderVL(t, `"transform": [{"filter": "datum.c == 'p'"}], "layer": [`+area+`, `+area+`]`)
	if n := len(marksOf(s, "zuzu-high-label")); n != 0 {
		t.Errorf("two areas have %d labels of the high, want 0", n)
	}
}

// TestAreaOpacityOfTheSpec checks that an area whose spec sets its
// opacity draws as a flat fill with no gradient and no line on top, and
// that an area of a spec that draws text has no label of its high
// (D48 parts 2 and 5).
func TestAreaOpacityOfTheSpec(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"transform": [{"filter": "datum.c == 'p'"}], "mark": {"type": "area", "color": "#a4cedb", "opacity": 0.7},
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}`)
	area := marksOf(s, "mark")[0].Items[0]
	if area.Fill.Gradient != nil || area.Fill.Color != "#a4cedb" || area.Opacity == nil || *area.Opacity != 0.7 {
		t.Errorf("the area has the fill %+v and the opacity %v, want #a4cedb at 0.7", area.Fill, area.Opacity)
	}
	if len(marksOf(s, "zuzu-area-line")) != 0 {
		t.Error("the area has a line on top")
	}
	area2 := `{"mark": "area", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}}}`
	text := `{"mark": "text", "encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "text": {"field": "c"}}}`
	s = renderVL(t, `"transform": [{"filter": "datum.c == 'p'"}], "layer": [`+area2+`, `+text+`]`)
	if n := len(marksOf(s, "zuzu-high-label")); n != 0 {
		t.Errorf("an area beside a text has %d labels of the high, want 0", n)
	}
}
