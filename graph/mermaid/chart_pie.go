package mermaid

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

// pieChart compiles a pie chart into an arc of Vega-Lite. Each slice has
// its percent inside it, and the legend names the slices, with the values
// when the header holds showData (D31).
func pieChart(c *chart) (map[string]any, bool, error) {
	textPosition, hole, legend := 0.75, 0.0, "right"
	if v, ok := configNumber(c.d.Config, "pie", "textPosition"); ok {
		textPosition = v
	}
	if v, ok := configNumber(c.d.Config, "pie", "donutHole"); ok {
		hole = v
	}
	if v := str(c.d.Config, "pie", "legendPosition"); v != "" {
		switch v {
		case "top", "bottom", "left", "right":
			legend = v
		default:
			if err := c.add("the legend position %s of a pie chart", v); err != nil {
				return nil, false, err
			}
		}
	}
	read := []string{"pie.textPosition", "pie.donutHole", "pie.legendPosition"}
	// The theme variables pie1 to pie12 are the colors of the slices. A
	// gap ends the list, and the variables after it are issues.
	var colors []any
	for i := 1; i <= 12; i++ {
		k := "pie" + strconv.Itoa(i)
		v := str(c.d.Config, "themeVariables", k)
		if v == "" {
			break
		}
		colors = append(colors, v)
		read = append(read, "themeVariables."+k)
	}
	if err := c.unreadSettings(read...); err != nil {
		return nil, false, err
	}
	showData, title := false, ""
	header := c.d.Header
	if rest, ok := keyword(header, "showData"); ok {
		showData, header = true, rest
	}
	if header != "" {
		rest, ok := keyword(header, "title")
		if !ok {
			c.line = 1
			return nil, false, c.errorf("the header %q of a pie chart", c.d.Header)
		}
		title = rest
	}
	type slice struct {
		label string
		value float64
	}
	var parts []slice
	seen := map[string]bool{}
	for i := 0; i < len(c.d.Lines); i++ {
		l := c.d.Lines[i]
		c.line = l.N
		next, ok, err := c.skipAccessibility(c.d.Lines, i)
		if err != nil {
			return nil, false, err
		}
		if ok {
			i = next
			continue
		}
		if rest, ok := keyword(l.Text, "title"); ok {
			title = rest
			continue
		}
		if l.Text == "showData" {
			showData = true
			continue
		}
		if !strings.HasPrefix(l.Text, `"`) {
			return nil, false, c.errorf("the line %q of a pie chart", l.Text)
		}
		end := strings.Index(l.Text[1:], `"`)
		if end < 0 {
			return nil, false, c.errorf("a label with no end quote")
		}
		label := l.Text[1 : end+1]
		rest := strings.TrimSpace(l.Text[end+2:])
		rest, ok = strings.CutPrefix(rest, ":")
		if !ok {
			return nil, false, c.errorf("the slice %q has no colon", label)
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return nil, false, c.errorf("the value %q of the slice %q", strings.TrimSpace(rest), label)
		}
		if v < 0 {
			return nil, false, c.errorf("the value %v of the slice %q is negative", v, label)
		}
		// Mermaid keeps the first slice of a label and drops the rest.
		if seen[label] {
			if err := c.add("the second slice %q, which Mermaid also drops", label); err != nil {
				return nil, false, err
			}
			continue
		}
		seen[label] = true
		parts = append(parts, slice{label, v})
	}
	c.line = 0
	if len(parts) == 0 {
		return nil, false, c.errorf("a pie chart with no slice")
	}
	var sum float64
	for _, s := range parts {
		sum += s.value
	}
	// The slices start at the top and go clockwise. The compiler places
	// the percent of each slice at its middle angle, at textPosition of
	// the radius.
	const size = 300
	r := textPosition * size / 2
	rows := make([]any, 0, len(parts))
	domain := make([]any, 0, len(parts))
	var cum float64
	for i, s := range parts {
		name := s.label
		if showData {
			name += " [" + strconv.FormatFloat(s.value, 'f', -1, 64) + "]"
		}
		percent := ""
		if sum > 0 && s.value > 0 {
			percent = strconv.FormatFloat(s.value/sum*100, 'f', 0, 64) + "%"
		}
		mid := 0.0
		if sum > 0 {
			mid = 2 * math.Pi * (cum + s.value/2) / sum
		}
		cum += s.value
		domain = append(domain, name)
		rows = append(rows, map[string]any{
			"kind": "slice", "i": i, "label": s.label, "value": s.value,
			"name": name, "percent": percent,
			"tx": size/2 + r*math.Sin(mid), "ty": size/2 - r*math.Cos(mid),
		})
	}
	arc := map[string]any{"type": "arc", "outerRadius": size / 2}
	if hole > 0 {
		arc["innerRadius"] = hole * size / 2
	}
	spec := map[string]any{
		"width":  size,
		"height": size,
		"data":   map[string]any{"values": rows},
		"layer": []any{
			map[string]any{
				"mark": arc,
				"encoding": map[string]any{
					"theta": map[string]any{"field": "value", "type": "quantitative", "stack": true},
					"order": map[string]any{"field": "i", "type": "quantitative"},
					"color": map[string]any{
						"field": "name", "type": "nominal",
						"scale":  pieScale(domain, colors),
						"legend": map[string]any{"title": nil, "orient": legend},
					},
				},
			},
			map[string]any{
				"mark":      map[string]any{"type": "text", "fontWeight": 600},
				"transform": []any{map[string]any{"filter": "datum.percent != ''"}},
				"encoding": map[string]any{
					"x":    map[string]any{"field": "tx", "type": "quantitative", "scale": nil},
					"y":    map[string]any{"field": "ty", "type": "quantitative", "scale": nil},
					"text": map[string]any{"field": "percent"},
				},
			},
		},
	}
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	return spec, true, nil
}

// pieScale returns the color scale of the slices: their names, and the
// colors that the theme variables give.
func pieScale(domain, colors []any) map[string]any {
	s := map[string]any{"domain": domain}
	if len(colors) > 0 {
		s["range"] = colors
	}
	return s
}

// unreadSettings reports each setting of the config that a chart does not
// read. A setting that the chart reads is a path of keys with dots, such
// as pie.textPosition. The look handDrawn is an issue, and the other looks
// draw in the look of zuzu (D12).
func (c *chart) unreadSettings(read ...string) error {
	line := c.line
	c.line = 0
	defer func() { c.line = line }()
	return c.unreadIn(c.d.Config, "", read)
}

// unreadIn reports the settings of a map of the config under a prefix.
func (c *chart) unreadIn(m map[string]any, prefix string, read []string) error {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		path := prefix + k
		if slices.Contains(read, path) {
			continue
		}
		// The theme variables are one issue, unless the chart reads some
		// of them. Every other map is an issue for each of its settings.
		themed := path == "theme" || path == "themeCSS" || strings.HasPrefix(path, "themeVariables")
		if sub, ok := m[k].(map[string]any); ok && (!themed || slices.ContainsFunc(read, func(r string) bool {
			return strings.HasPrefix(r, path+".")
		})) {
			if err := c.unreadIn(sub, path+".", read); err != nil {
				return err
			}
			continue
		}
		var err error
		switch {
		case path == "look":
			if m[k] == "handDrawn" {
				err = c.add("the look handDrawn")
			}
		case themed:
			err = c.add("the %s of Mermaid, drawn in the look of zuzu", path)
		default:
			err = c.add("the setting %s", path)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// skipAccessibility reads the line i when it is accTitle or accDescr, in
// one line or in a block of braces. zuzu does not write them, so each is
// an issue. It returns the last line that it read, and whether it read
// one.
func (c *chart) skipAccessibility(lines []Line, i int) (int, bool, error) {
	t := lines[i].Text
	var key string
	switch {
	case strings.HasPrefix(t, "accTitle"):
		key = "accTitle"
	case strings.HasPrefix(t, "accDescr"):
		key = "accDescr"
	default:
		return i, false, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(t, key))
	switch {
	case strings.HasPrefix(rest, ":"):
	case key == "accDescr" && strings.HasPrefix(rest, "{"):
		for !strings.Contains(lines[i].Text, "}") {
			i++
			if i >= len(lines) {
				return i, false, c.errorf("an accDescr block with no end brace")
			}
		}
	default:
		return i, false, nil
	}
	if err := c.add("the %s of the chart", key); err != nil {
		return i, false, err
	}
	return i, true, nil
}

// configNumber returns a number of a config by a path of keys. A string
// that holds a number also counts.
func configNumber(cfg map[string]any, keys ...string) (float64, bool) {
	var v any = cfg
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return 0, false
		}
		v = m[k]
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(n), "px"), 64)
		return f, err == nil
	}
	return 0, false
}
