package mermaid

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// sankeyChart compiles a sankey: lines of CSV with a source, a target and a
// value, into a Vega spec with zuzu.sankey and zuzu.sankeylinks (D31).
func sankeyChart(c *chart) (map[string]any, bool, error) {
	cfg, err := c.settings("sankey", []string{
		"width", "height", "linkColor", "nodeAlignment", "showValues", "prefix", "suffix",
		"nodeWidth", "nodePadding", "nodeColors", "labelStyle",
	})
	if err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	var links []any
	var names []string
	seen := map[string]bool{}
	for _, l := range c.d.Lines {
		c.line = l.N
		f, err := c.csv(l.Text)
		if err != nil {
			return nil, false, err
		}
		if len(f) != 3 {
			return nil, false, c.errorf("%d fields, and a link has 3: source, target and value", len(f))
		}
		v, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return nil, false, c.errorf("the value %q is not a number", f[2])
		}
		if v < 0 {
			return nil, false, c.errorf("the value %v is less than 0", v)
		}
		if f[0] == "" || f[1] == "" {
			return nil, false, c.errorf("a link with no source or no target")
		}
		for _, n := range f[:2] {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
		links = append(links, map[string]any{"source": f[0], "target": f[1], "value": v})
	}
	c.line = 0
	if len(links) == 0 {
		return nil, false, c.errorf("a sankey with no links")
	}
	width := settingNumber(cfg, "width", 600)
	height := settingNumber(cfg, "height", 400)
	align := "justify"
	if a, ok := cfg["nodeAlignment"].(string); ok {
		switch a {
		case "justify", "left", "right", "center":
			align = a
		default:
			if err := c.add("the setting sankey.nodeAlignment %s", a); err != nil {
				return nil, false, err
			}
		}
	}
	// A color of a node that the source gives goes before the color of
	// the theme.
	nodeColors, _ := cfg["nodeColors"].(map[string]any)
	colorOf := func(field string) []any {
		var rules []any
		for _, k := range slices.Sorted(maps.Keys(nodeColors)) {
			if s, ok := nodeColors[k].(string); ok {
				rules = append(rules, map[string]any{"test": "datum." + field + " === " + quoteJS(k), "value": s})
			}
		}
		return append(rules, map[string]any{"scale": "color", "field": field})
	}
	// colorExpr is colorOf as an expression of Vega.
	colorExpr := func(field string) string {
		e := "scale('color', datum." + field + ")"
		for _, k := range slices.Backward(slices.Sorted(maps.Keys(nodeColors))) {
			if s, ok := nodeColors[k].(string); ok {
				e = "(datum." + field + " === " + quoteJS(k) + " ? " + quoteJS(s) + " : " + e + ")"
			}
		}
		return e
	}
	linkFill := colorOf("source")
	if lc, ok := cfg["linkColor"].(string); ok {
		switch lc {
		case "source":
		case "target":
			linkFill = colorOf("target")
		case "gradient":
			// As in Mermaid, the color goes from the color of the source
			// at the left end of the link to the color of the target at
			// the right end. A gradient of Vega spans the bounds of the
			// band, which run from the source to the target.
			linkFill = []any{map[string]any{"signal": "{gradient: 'linear', x1: 0, y1: 0, x2: 1, y2: 0, stops: [" +
				"{offset: 0, color: " + colorExpr("source") + "}, {offset: 1, color: " + colorExpr("target") + "}]}"}}
		default:
			linkFill = []any{map[string]any{"value": lc}}
		}
	}
	// The label style outlined gives each label a halo, and puts the
	// label of a node in a layer before the layer of the largest node at
	// its left, as Mermaid does.
	labelStyle := []any{"zuzu.nodeLabel"}
	labels := map[string]any{"type": "zuzu.sankeylabels", "extent": []any{[]any{0, 0}, []any{width, height}}, "fontSize": 11, "fontWeight": "600"}
	switch s, _ := cfg["labelStyle"].(string); s {
	case "", "legacy":
	case "outlined":
		labelStyle = append(labelStyle, "zuzu.halo")
		labels["side"] = "layer"
	default:
		if err := c.add("the setting sankey.labelStyle %s", s); err != nil {
			return nil, false, err
		}
	}
	text := "datum.key"
	if show, ok := cfg["showValues"].(bool); !ok || show {
		prefix, _ := cfg["prefix"].(string)
		suffix, _ := cfg["suffix"].(string)
		text += " + ' ' + " + quoteJS(prefix) + " + format(datum.value, '.2~f') + " + quoteJS(suffix)
	}
	labels["text"] = map[string]any{"expr": text}
	spec := map[string]any{
		"width":   width,
		"height":  height,
		"padding": 5,
		"data": []any{
			map[string]any{"name": "links", "values": links},
			map[string]any{"name": "nodes", "source": "links", "transform": []any{map[string]any{
				"type": "zuzu.sankey", "extent": []any{[]any{0, 0}, []any{width, height}},
				"nodeWidth": settingNumber(cfg, "nodeWidth", 10), "nodePadding": settingNumber(cfg, "nodePadding", 12),
				"align": align,
			}, labels}},
			map[string]any{"name": "bands", "source": "links", "transform": []any{map[string]any{
				"type": "zuzu.sankeylinks", "nodes": "nodes",
			}}},
		},
		"scales": []any{map[string]any{"name": "color", "type": "ordinal", "domain": names, "range": "category"}},
		"marks": []any{
			map[string]any{"type": "path", "name": "links", "from": map[string]any{"data": "bands"}, "encode": map[string]any{"update": map[string]any{
				"path": map[string]any{"field": "path"},
				"fill": linkFill,
			}}},
			map[string]any{"type": "rect", "name": "nodes", "from": map[string]any{"data": "nodes"}, "encode": map[string]any{"update": map[string]any{
				"x": map[string]any{"field": "x0"}, "x2": map[string]any{"field": "x1"},
				"y": map[string]any{"field": "y0"}, "y2": map[string]any{"field": "y1"},
				"fill": colorOf("key"),
			}}},
			// zuzu.sankeylabels places each label clear of the other labels
			// and of the nodes.
			map[string]any{"type": "text", "name": "labels", "from": map[string]any{"data": "nodes"}, "style": labelStyle, "encode": map[string]any{"update": map[string]any{
				"x":          map[string]any{"field": "labelX"},
				"y":          map[string]any{"field": "labelY"},
				"align":      map[string]any{"field": "labelAlign"},
				"baseline":   map[string]any{"value": "middle"},
				"fontWeight": map[string]any{"value": 600},
				"text":       map[string]any{"field": "label"},
			}}},
		},
	}
	return spec, false, nil
}

// csv splits a line of CSV into its fields, by RFC 4180: a field in double
// quotes holds commas, and two double quotes in it are one. Each field is
// trimmed, as Mermaid trims it.
func (c *chart) csv(s string) ([]string, error) {
	var fields []string
	var b strings.Builder
	quoted, inQuotes := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case inQuotes && ch == '"' && i+1 < len(s) && s[i+1] == '"':
			b.WriteByte('"')
			i++
		case inQuotes && ch == '"':
			inQuotes = false
		case inQuotes:
			b.WriteByte(ch)
		case ch == '"' && strings.TrimSpace(b.String()) == "" && !quoted:
			b.Reset()
			quoted, inQuotes = true, true
		case ch == ',':
			fields = append(fields, strings.TrimSpace(b.String()))
			b.Reset()
			quoted = false
		case quoted && ch != ' ' && ch != '\t':
			return nil, c.errorf("the text %q after a field in quotes", s[i:])
		default:
			b.WriteByte(ch)
		}
	}
	if inQuotes {
		return nil, c.errorf("a field in quotes with no end")
	}
	return append(fields, strings.TrimSpace(b.String())), nil
}

// settings reads the config of a chart: the settings of its section that
// read names, and the keys of the top level that own names, which the
// chart reads itself. Each other setting is an issue. It returns the
// section.
func (c *chart) settings(section string, read []string, own ...string) (map[string]any, error) {
	c.line = 0
	cfg := c.d.Config
	m, _ := cfg[section].(map[string]any)
	for _, k := range slices.Sorted(maps.Keys(cfg)) {
		var err error
		switch {
		case k == section:
			for _, s := range slices.Sorted(maps.Keys(m)) {
				if !slices.Contains(read, s) && !inert(c.d.Kind, section+"."+s) {
					if err = c.add("the setting %s.%s", section, s); err != nil {
						break
					}
				}
			}
		case slices.Contains(own, k), inert(c.d.Kind, k):
		case k == "theme" || k == "themeVariables" || k == "themeCSS":
			err = c.add("the %s of Mermaid, drawn in the look of zuzu", k)
		default:
			err = c.unreadIn(map[string]any{k: cfg[k]}, "", nil)
		}
		if err != nil {
			return nil, err
		}
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// settingNumber returns a number of a section of the config, or a default.
func settingNumber(cfg map[string]any, key string, def float64) float64 {
	switch v := cfg[key].(type) {
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64); err == nil {
			return f
		}
	}
	return def
}

// quoteJS returns a string as a literal of the expression language of
// Vega.
func quoteJS(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
