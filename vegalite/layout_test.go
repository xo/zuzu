package vegalite

import "testing"

// TestFacetLayout checks the layout of a facet against the layout that
// vega-lite.js compiles for the same spec: the offset of a title of the
// facet, the anchor end of a title at the bottom, the align none of cells
// along one channel with an independent scale, the band of the headers of
// a facet of facets, and a spacing of the rows alone.
func TestFacetLayout(t *testing.T) {
	t.Parallel()
	const data = `"data": {"values": [{"a": "A", "b": "x", "c": "p", "v": 1}, {"a": "B", "b": "y", "c": "q", "v": 2}]}`
	for _, c := range []struct {
		name, src, want string
	}{
		{
			"column", `"mark": "bar", "encoding": {"column": {"field": "a"}, "x": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}`,
			`{"align":"all","bounds":"full","columns":{"signal":"length(data('column_domain'))"},"offset":{"columnTitle":10},"padding":20}`,
		},
		{
			"row and column", `"mark": "point", "encoding": {"row": {"field": "a"}, "column": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}`,
			`{"align":"all","bounds":"full","columns":{"signal":"length(data('column_domain'))"},"offset":{"columnTitle":10,"rowTitle":10},"padding":20}`,
		},
		{
			"title at the bottom", `"mark": "bar", "encoding": {"column": {"field": "a", "header": {"orient": "bottom"}}, "x": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}`,
			`{"align":"all","bounds":"full","columns":{"signal":"length(data('column_domain'))"},"offset":{"columnTitle":10},"padding":20,"titleAnchor":{"column":"end"}}`,
		},
		{
			"independent", `"mark": "bar", "encoding": {"column": {"field": "a"}, "x": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}, "resolve": {"scale": {"x": "independent"}}`,
			`{"align":"none","bounds":"full","columns":{"signal":"length(data('column_domain'))"},"offset":{"columnTitle":10},"padding":20}`,
		},
		{
			"independent across", `"mark": "bar", "encoding": {"row": {"field": "a"}, "x": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}, "resolve": {"scale": {"x": "independent"}}`,
			`{"align":"all","bounds":"full","columns":1,"offset":{"rowTitle":10},"padding":20}`,
		},
		{
			"spacing of the rows", `"mark": "bar", "spacing": {"row": 1}, "encoding": {"row": {"field": "a"}, "x": {"field": "b"}, "y": {"field": "v", "type": "quantitative"}}`,
			`{"align":"all","bounds":"full","columns":1,"offset":{"rowTitle":10},"padding":{"column":20,"row":1}}`,
		},
		{
			"facet of facets", `"facet": {"column": {"field": "a"}}, "spec": {"facet": {"column": {"field": "b"}}, "spec": {"mark": "point", "encoding": {"y": {"field": "v", "type": "quantitative"}}}}`,
			`{"align":"all","bounds":"full","columns":{"signal":"length(data('column_domain'))"},"headerBand":{"column":0.5},"offset":{"columnTitle":10},"padding":20}`,
		},
	} {
		m := compileMap(t, `{`+data+`, `+c.src+`}`)
		if got := asJSON(m["layout"]); got != c.want {
			t.Errorf("%s: the layout is\n%s, want\n%s", c.name, got, c.want)
		}
	}
}
