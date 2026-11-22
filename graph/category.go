package graph

import (
	"slices"

	"github.com/xo/zuzu/theme"
)

// categoryScale is the name of the ordinal scale that maps the place of a
// color in the category scheme of the theme, from 0, to the color, for the
// nodes and the clusters that set Category (D37). After the last color of
// the scheme, the colors start again.
const categoryScale = "zuzu.category"

// categoryRow writes the place of the color of a node or a cluster into
// its row, from 0, and returns the number of places that the scale needs,
// at least n. A cluster draws as a node of its color draws: a solid tint
// and an outline. A fill or a stroke of its style wins over the color.
func categoryRow(r map[string]any, category, n int, cluster bool) int {
	if category <= 0 {
		return n
	}
	r["category"] = float64(category - 1)
	if cluster {
		if _, ok := r["strokeWidth"]; !ok {
			r["strokeWidth"] = theme.NodeOutline
		}
	}
	return max(n, category)
}

// categoryTintScale is the name of the ordinal scale that maps the place
// of a color to its solid tint, the fill of a node or a panel of that
// color (D55).
const categoryTintScale = "zuzu.categoryTint"

// categoryEncode sets the fill and the stroke of a mark to the color of
// the place in the field category of a row.
func categoryEncode(enc map[string]any) {
	enc["fill"] = map[string]any{"scale": categoryTintScale, "field": "category"}
	enc["stroke"] = map[string]any{"scale": categoryScale, "field": "category"}
}

// categoryScales returns the scales of a graph whose nodes or clusters
// take n places of the category scheme, or nil for none.
func categoryScales(n int) []any {
	if n == 0 {
		return nil
	}
	domain := make([]any, n)
	for i := range domain {
		domain[i] = float64(i)
	}
	return []any{
		map[string]any{"name": categoryScale, "type": "ordinal", "domain": domain, "range": "category"},
		map[string]any{"name": categoryTintScale, "type": "ordinal", "domain": domain, "range": theme.RangeTint},
	}
}

// TintScale returns an ordinal scale, named name, that maps each value of
// domain to the solid tint of its color of the category scheme, in the
// order of the scheme. A node of a chart that takes a color by kind fills
// with it, and its outline takes the color itself (D55).
func TintScale(name string, domain any) map[string]any {
	return map[string]any{"name": name, "type": "ordinal", "domain": domain, "range": theme.RangeTint}
}

// solidEncode sets the fill of a solid node, whose signature has the keys
// keys, to the color of its outline: the stroke of its style, or else the
// color of its category. A fill of its style wins. A solid node with
// neither takes the fill of the style of a solid node (D55).
func solidEncode(enc map[string]any, keys []string) {
	if !slices.Contains(keys, "solid") || slices.Contains(keys, "fill") {
		return
	}
	switch {
	case slices.Contains(keys, "stroke"):
		enc["fill"] = map[string]any{"field": "stroke"}
	case slices.Contains(keys, "category"):
		enc["fill"] = map[string]any{"scale": categoryScale, "field": "category"}
	}
}
