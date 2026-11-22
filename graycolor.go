package zuzu

import (
	"encoding/json/v2"
	"maps"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/theme"
)

// flipsGrays reports whether the render flips the lightness of a gray near
// black or near white that the spec names (D48 part 1): in the dark mode,
// for a chart of Vega or Vega-Lite, on a dark background. A diagram of DOT
// or Mermaid keeps its colors, because its theme sets its mode (D32, D37).
// The background is the one of the caller or of zuzu.background, or else
// the color that the spec or its config gives, or else the one of the
// mode. A background that is not a color, such as a signal, counts as the
// one of the mode.
func (rt *runtime) flipsGrays() bool {
	if rt.chart.diagram || rt.theme.Mode() != theme.Dark {
		return false
	}
	bg := rt.theme.Background
	if !rt.opts.hasBackground {
		var s string
		if b := rt.chart.spec.Background; b.Signal == "" && len(b.JSON) > 0 && json.Unmarshal(b.JSON, &s) == nil {
			bg = s
		} else if raw, ok := rt.chart.spec.Config["background"]; ok && json.Unmarshal(raw, &s) == nil {
			bg = s
		}
	}
	c, ok := color.Parse(bg)
	if !ok || c.A <= 0 {
		// A transparent chart draws over the page of the mode.
		return true
	}
	return theme.IsDark(color.RGBA{R: c.R, G: c.G, B: c.B, A: 1}.Hex())
}

// specColor returns a color that the spec names as the render draws it. In
// the dark mode, a gray near black or near white flips its lightness, so
// that it reads on the dark background, and any other color stays
// (D48 part 1). The color can be a string of CSS, or a gradient as an
// object, whose stops each flip. A value that is not a color stays as it
// is, and so does every value when the render does not flip.
func (rt *runtime) specColor(v any) any {
	if !rt.flipGrays {
		return v
	}
	switch x := v.(type) {
	case string:
		return flipGray(x)
	case map[string]any:
		stops, ok := x["stops"].([]any)
		if !ok || !expr.Truthy(x["gradient"]) {
			return v
		}
		out := maps.Clone(x)
		flipped := make([]any, len(stops))
		for i, st := range stops {
			flipped[i] = st
			if m, ok := st.(map[string]any); ok {
				if c, ok := m["color"].(string); ok {
					m = maps.Clone(m)
					m["color"] = flipGray(c)
					flipped[i] = m
				}
			}
		}
		out["stops"] = flipped
		return out
	}
	return v
}

// flipGray returns a color of CSS with its lightness flipped when it is a
// gray near black or near white, and keeps its opacity. It returns any
// other string as it is, and so a transparent color too.
func flipGray(s string) string {
	c, ok := color.Parse(s)
	if !ok || c.A <= 0 {
		return s
	}
	r, g, b, ok := theme.FlipGray(c.R, c.G, c.B)
	if !ok {
		return s
	}
	return color.RGBA{R: r, G: g, B: b, A: c.A}.Hex()
}

// specConfig returns a value of the config of the spec, or a property of
// a guide, under a key, with each color in it as the render draws it,
// through specColor, at any depth. A color is the value of a key of a
// color, such as fill, stroke, color or labelColor, and each value of a
// range of the config. The background stays, because the background of
// the chart decides whether the render flips at all.
func (rt *runtime) specConfig(key string, v any) any {
	if !rt.flipGrays {
		return v
	}
	return rt.configColors(key, v, false)
}

// configColors flips the colors of a value of the config under a key.
// colors says that every string in the value is a color, as in a range.
func (rt *runtime) configColors(key string, v any, colors bool) any {
	colors = colors || colorKey(key)
	switch x := v.(type) {
	case map[string]any:
		if expr.Truthy(x["gradient"]) {
			if colors {
				return rt.specColor(x)
			}
			return v
		}
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = rt.configColors(k, e, colors || key == "range")
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = rt.configColors(key, e, colors)
		}
		return out
	case string:
		if colors {
			return flipGray(x)
		}
	}
	return v
}

// colorKey reports whether a key of the config or of a guide names a
// color: fill, stroke, color, or a key that ends in Color, such as
// labelColor. The background is not one of them.
func colorKey(k string) bool {
	return k == "fill" || k == "stroke" || k == "color" || strings.HasSuffix(k, "Color")
}
