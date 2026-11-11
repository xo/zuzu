package mermaid

import (
	"maps"
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/zuzu/format"
)

// treemapNode matches a node of a treemap: a name in quotes, a class, a
// value after a colon, and a class after the value.
var treemapNode = regexp.MustCompile(`^("(?:[^"]*)"|'(?:[^']*)')\s*(?::::([\w-]+))?\s*(?::\s*([^:]+?))?\s*(?::::([\w-]+))?\s*;?$`)

// treemapRow is a node of a treemap while the compiler reads it.
type treemapRow struct {
	id, parent int
	name       string
	value      float64
	hasValue   bool
	class      string
	indent     int
	line       int
	children   int
}

// treemapChart compiles a treemap: the nodes of a tree, nested by indent,
// with a value on each leaf, into a Vega spec with stratify and treemap
// (D31).
func treemapChart(c *chart) (map[string]any, bool, error) {
	cfg, err := c.settings("treemap", []string{"padding", "diagramPadding", "showValues", "valueFontSize", "labelFontSize", "valueFormat"})
	if err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	spec := map[string]any{}
	rows := []*treemapRow{{}}
	classes := map[string]map[string]any{}
	// open holds the nodes that a deeper line can nest in, the outermost
	// first.
	var open []*treemapRow
	for _, l := range c.d.Lines {
		c.line = l.N
		if rest, ok := keyword(l.Text, "title"); ok {
			spec["title"] = map[string]any{"text": rest, "anchor": "start"}
			continue
		}
		if rest, ok := keyword(l.Text, "classDef"); ok {
			name, css, _ := strings.Cut(rest, " ")
			st, err := c.css(css)
			if err != nil {
				return nil, false, err
			}
			classes[name] = st
			continue
		}
		if ok, err := c.accText(l.Text); ok || err != nil {
			if err != nil {
				return nil, false, err
			}
			continue
		}
		m := treemapNode.FindStringSubmatch(l.Text)
		if m == nil {
			return nil, false, c.errorf("the line %q is not a node of a treemap", l.Text)
		}
		r := &treemapRow{id: len(rows), name: m[1][1 : len(m[1])-1], class: m[2], indent: l.Indent, line: l.N}
		if m[4] != "" {
			r.class = m[4]
		}
		if m[3] != "" {
			v, err := strconv.ParseFloat(strings.TrimSpace(m[3]), 64)
			if err != nil {
				return nil, false, c.errorf("the value %q is not a number", m[3])
			}
			if v < 0 {
				return nil, false, c.errorf("the value %v is less than 0", v)
			}
			r.value, r.hasValue = v, true
		}
		for len(open) > 0 && open[len(open)-1].indent >= r.indent {
			open = open[:len(open)-1]
		}
		if len(open) > 0 {
			r.parent = open[len(open)-1].id
		}
		rows[r.parent].children++
		open = append(open, r)
		rows = append(rows, r)
	}
	if len(rows) == 1 {
		return nil, false, c.errorf("a treemap with no nodes")
	}
	// The total of a node is its value, or the sum of the values of its
	// leaves. The rows come after their parents, so a walk from the end
	// adds each total to its parent.
	total := make([]float64, len(rows))
	depth := make([]int, len(rows))
	for i := len(rows) - 1; i >= 1; i-- {
		r := rows[i]
		if r.children == 0 {
			total[i] = r.value
		}
		total[r.parent] += total[i]
	}
	// The colors come from the first level of the tree that holds more
	// than one node, so that a tree with one root section still has its
	// colors. A node above that level has the color of its own.
	count := map[int]int{}
	for _, r := range rows[1:] {
		depth[r.id] = depth[r.parent] + 1
		count[depth[r.id]]++
	}
	level := 1
	for count[level] == 1 && count[level+1] > 0 {
		level++
	}
	var tops []any
	top := make([]int, len(rows))
	var values []any
	hasColor := false
	for _, r := range rows[1:] {
		c.line = r.line
		if depth[r.id] <= level {
			tops = append(tops, r.id)
			top[r.id] = r.id
		} else {
			top[r.id] = top[r.parent]
		}
		row := map[string]any{"id": r.id, "parent": r.parent, "name": r.name, "top": top[r.id], "total": total[r.id]}
		switch {
		case r.children > 0 && r.hasValue:
			if err := c.add("the value %v of the section %q, which draws the sum of its children", r.value, r.name); err != nil {
				return nil, false, err
			}
		case r.children == 0 && !r.hasValue:
			if err := c.add("the leaf %q with no value", r.name); err != nil {
				return nil, false, err
			}
		case r.hasValue:
			row["size"] = r.value
		}
		if r.class != "" {
			st, ok := classes[r.class]
			if !ok {
				if err := c.add("the class %s, which no classDef names", r.class); err != nil {
					return nil, false, err
				}
			}
			maps.Copy(row, st)
			if st["color"] != nil {
				hasColor = true
			}
		}
		values = append(values, row)
	}
	c.line = 0
	values = append([]any{map[string]any{"id": 0, "name": ""}}, values...)
	valueFormat := ","
	if f, ok := cfg["valueFormat"].(string); ok {
		// Mermaid reads $0,0 as a dollar sign with a thousands separator.
		if f == "$0,0" {
			f = "$,"
		}
		if _, err := format.Number(f, format.EnUS); err != nil {
			if err := c.add("the setting treemap.valueFormat %s", f); err != nil {
				return nil, false, err
			}
		} else {
			valueFormat = f
		}
	}
	showValues := true
	if b, ok := cfg["showValues"].(bool); ok {
		showValues = b
	}
	labelSize := settingNumber(cfg, "labelFontSize", 11)
	valueSize := settingNumber(cfg, "valueFontSize", 10)
	inner := settingNumber(cfg, "padding", 3)
	left := map[string]any{"signal": "datum.x0 + 6"}
	value := "format(datum.total, " + quoteJS(valueFormat) + ")"
	spec["width"] = 640
	spec["height"] = 400
	spec["padding"] = settingNumber(cfg, "diagramPadding", 5)
	data := []any{
		map[string]any{"name": "tree", "values": values, "transform": []any{
			map[string]any{"type": "stratify", "key": "id", "parentKey": "parent"},
			map[string]any{
				"type": "treemap", "field": "size", "sort": map[string]any{"field": "value", "order": "descending"},
				"round": true, "paddingTop": labelSize + 10, "paddingInner": inner, "paddingOuter": inner,
				"size": []any{map[string]any{"signal": "width"}, map[string]any{"signal": "height + " + strconv.FormatFloat(labelSize+10, 'f', -1, 64)}},
			},
			// The root draws no box, so its space for a name goes.
			map[string]any{"type": "formula", "as": "y0", "expr": "datum.y0 - " + strconv.FormatFloat(labelSize+10, 'f', -1, 64)},
			map[string]any{"type": "formula", "as": "y1", "expr": "datum.y1 - " + strconv.FormatFloat(labelSize+10, 'f', -1, 64)},
		}},
		filteredData("sections", "tree", "datum.depth >= 1 && datum.children > 0"),
		filteredData("leaves", "tree", "datum.depth >= 1 && !datum.children"),
	}
	paint := func(prop string) []any {
		return []any{map[string]any{"test": "datum." + prop, "field": prop}}
	}
	box := func(name, from string) map[string]any {
		return map[string]any{"type": "rect", "name": name, "from": map[string]any{"data": from}, "encode": map[string]any{"update": map[string]any{
			"x": map[string]any{"field": "x0"}, "x2": map[string]any{"field": "x1"},
			"y": map[string]any{"field": "y0"}, "y2": map[string]any{"field": "y1"},
			"fill":        append(paint("fill"), map[string]any{"scale": "color", "field": "top"}),
			"stroke":      paint("stroke"),
			"strokeWidth": paint("strokeWidth"),
		}}}
	}
	marks := []any{box("sections", "sections"), box("leaves", "leaves")}
	// text returns the marks of a label, one for the nodes in the color
	// of the theme and one for the nodes whose class gives a color.
	text := func(name, from string, style string, enc map[string]any) {
		parts := []struct{ suffix, test string }{{"", "!datum.color"}, {"Colored", "datum.color"}}
		if !hasColor {
			parts = parts[:1]
			parts[0].test = ""
		}
		for _, p := range parts {
			src := from
			if p.test != "" {
				src = from + name + p.suffix
				data = append(data, filteredData(src, from, p.test))
			}
			e := maps.Clone(enc)
			if p.suffix != "" {
				e["fill"] = map[string]any{"field": "color"}
			}
			marks = append(marks, map[string]any{
				"type": "text", "name": name + p.suffix, "from": map[string]any{"data": src},
				"style": []any{style}, "encode": map[string]any{"update": e},
			})
		}
	}
	// A section has its name at the top left and its total at the top
	// right. A leaf has its name and its value at the top left.
	text("sectionNames", "sections", "zuzu.nodeLabel", map[string]any{
		"x": left, "y": map[string]any{"signal": "datum.y0 + 5"},
		"baseline": map[string]any{"value": "top"}, "fontWeight": map[string]any{"value": 600},
		"fontSize": map[string]any{"value": labelSize},
		"limit":    map[string]any{"signal": "max(0, datum.x1 - datum.x0 - 60)"},
		"text":     map[string]any{"field": "name"},
	})
	text("leafNames", "leaves", "zuzu.edgeLabelBox", map[string]any{
		"x": left, "y": map[string]any{"signal": "datum.y0 + 6"},
		"baseline": map[string]any{"value": "top"}, "fontWeight": map[string]any{"value": 600},
		"fontSize": map[string]any{"value": labelSize},
		"limit":    map[string]any{"signal": "max(0, datum.x1 - datum.x0 - 12)"},
		"text":     map[string]any{"field": "name"},
	})
	if showValues {
		text("sectionValues", "sections", "zuzu.edgeLabel", map[string]any{
			"x": map[string]any{"signal": "datum.x1 - 6"}, "y": map[string]any{"signal": "datum.y0 + 5"},
			"align": map[string]any{"value": "right"}, "baseline": map[string]any{"value": "top"},
			"fontSize": map[string]any{"value": valueSize},
			"text":     map[string]any{"signal": value},
		})
		text("leafValues", "leaves", "zuzu.edgeLabelBox", map[string]any{
			"x": left, "y": map[string]any{"signal": "datum.y0 + 10 + " + strconv.FormatFloat(labelSize, 'f', -1, 64)},
			"baseline": map[string]any{"value": "top"},
			"fontSize": map[string]any{"value": valueSize},
			"limit":    map[string]any{"signal": "max(0, datum.x1 - datum.x0 - 12)"},
			"text":     map[string]any{"signal": value},
		})
	}
	spec["data"] = data
	spec["scales"] = []any{map[string]any{"name": "color", "type": "ordinal", "domain": tops, "range": "category"}}
	spec["marks"] = marks
	return spec, false, nil
}

// filteredData returns a dataset of the rows of another that pass a test.
func filteredData(name, source, test string) map[string]any {
	return map[string]any{"name": name, "source": source, "transform": []any{map[string]any{"type": "filter", "expr": test}}}
}

// css reads the style of a class of a chart, as fill:#f96,stroke:#333.
// Each property that the chart does not draw is an issue.
func (c *chart) css(s string) (map[string]any, error) {
	st := map[string]any{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			return nil, c.errorf("the style %q has no colon", strings.TrimSpace(part))
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		switch k {
		case "fill", "stroke", "color":
			st[k] = v
		case "stroke-width":
			w, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
			if err != nil {
				return nil, c.errorf("the stroke-width %q is not a number", v)
			}
			st["strokeWidth"] = w
		default:
			if err := c.add("the style %s", k); err != nil {
				return nil, err
			}
		}
	}
	return st, nil
}
