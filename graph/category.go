package graph

import "github.com/xo/zuzu/theme"

// categoryScale is the name of the ordinal scale that maps the place of a
// color in the category scheme of the theme, from 0, to the color, for the
// nodes and the clusters that set Category (D37). After the last color of
// the scheme, the colors start again.
const categoryScale = "zuzu.category"

// categoryRow writes the place of the color of a node or a cluster into
// its row, from 0, and returns the number of places that the scale needs,
// at least n. A cluster draws as a node of its color draws: a tint and an
// outline. A fill or a stroke of its style wins over the color.
func categoryRow(r map[string]any, category, n int, cluster bool) int {
	if category <= 0 {
		return n
	}
	r["category"] = float64(category - 1)
	if cluster {
		if _, ok := r["fillOpacity"]; !ok {
			r["fillOpacity"] = theme.NodeTint
		}
		if _, ok := r["strokeWidth"]; !ok {
			r["strokeWidth"] = theme.NodeOutline
		}
	}
	return max(n, category)
}

// categoryEncode sets the fill and the stroke of a mark to the color of
// the place in the field category of a row.
func categoryEncode(enc map[string]any) {
	enc["fill"] = map[string]any{"scale": categoryScale, "field": "category"}
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
	return []any{map[string]any{"name": categoryScale, "type": "ordinal", "domain": domain, "range": "category"}}
}
