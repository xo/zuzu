package mermaid

import (
	"fmt"
	"strconv"
	"strings"
)

// xyAxis is an axis of an xychart: its title, and its categories or its
// range.
type xyAxis struct {
	title    string
	cats     []string
	min, max float64
	ranged   bool
}

// xyPlot is a bar or a line of an xychart.
type xyPlot struct {
	kind, name, key string
	values          []float64
	labels          []string
}

// xyChart compiles an xychart into bars and lines of Vega-Lite, with the
// axes that the source names (D31). A chart is vertical unless its header
// or its config says horizontal.
func xyChart(c *chart) (map[string]any, bool, error) {
	cfg := c.d.Config
	horizontal := str(cfg, "xyChart", "chartOrientation") == "horizontal"
	switch c.d.Header {
	case "":
	case "horizontal":
		horizontal = true
	case "vertical":
		horizontal = false
	default:
		c.line = c.d.N
		return nil, false, c.errorf("the header %q of an xychart", c.d.Header)
	}
	var title string
	var xa, ya *xyAxis
	var plots []*xyPlot
	keys := map[string]bool{}
	for i := 0; i < len(c.d.Lines); i++ {
		l := c.d.Lines[i]
		c.line = l.N
		next, ok, err := c.accessibility(c.d.Lines, i)
		if err != nil {
			return nil, false, err
		}
		if ok {
			i = next
			continue
		}
		word, rest, _ := strings.Cut(l.Text, " ")
		rest = strings.TrimSpace(rest)
		switch word {
		case "title":
			title = unquote(rest)
		case "x-axis", "y-axis":
			a, err := c.xyAxis(rest, word == "x-axis")
			if err != nil {
				return nil, false, err
			}
			if word == "x-axis" {
				xa = a
			} else {
				ya = a
			}
		case "bar", "line":
			p, err := c.xyPlot(word, rest)
			if err != nil {
				return nil, false, err
			}
			p.key = p.name
			if p.key == "" {
				p.key = fmt.Sprintf("%s %d", word, len(plots)+1)
			}
			for keys[p.key] {
				p.key += "'"
			}
			keys[p.key] = true
			plots = append(plots, p)
		default:
			return nil, false, c.errorf("the line %q of an xychart", l.Text)
		}
	}
	c.line = 0
	if len(plots) == 0 {
		return nil, false, c.errorf("an xychart with no bar and no line")
	}
	if xa == nil {
		xa = &xyAxis{}
	}
	if xa.cats == nil && !xa.ranged {
		// With no range and no categories, the x axis runs from 1 to the
		// number of values of the first plot, as in Mermaid.
		xa.ranged, xa.min, xa.max = true, 1, float64(max(len(plots[0].values), 1))
	}
	if ya == nil {
		ya = &xyAxis{}
	}
	read := []string{"xyChart.width", "xyChart.height", "xyChart.chartOrientation",
		"xyChart.showLegend", "xyChart.showTitle", "xyChart.showDataLabel",
		"xyChart.showDataLabelOutsideBar", "xyChart.titleFontSize", "xyChart.titlePadding",
		"xyChart.xAxis.labelFontSize", "xyChart.xAxis.titleFontSize",
		"xyChart.yAxis.labelFontSize", "xyChart.yAxis.titleFontSize",
		"themeVariables.xyChart.plotColorPalette"}
	if xa.cats != nil {
		// zuzu draws an axis line along an axis of categories alone.
		read = append(read, "xyChart.xAxis.axisLineWidth")
	}
	if err := c.unreadSettings(read...); err != nil {
		return nil, false, err
	}
	return c.xySpec(title, horizontal, xa, ya, plots)
}

// xySpec writes the spec of an xychart.
func (c *chart) xySpec(title string, horizontal bool, xa, ya *xyAxis, plots []*xyPlot) (map[string]any, bool, error) {
	cfg := c.d.Config
	cat, val, orient := "x", "y", "vertical"
	if horizontal {
		cat, val, orient = "y", "x", "horizontal"
	}
	// On a range, a bar spans half of the space between the values of the
	// plot with the most bars, and the range grows to hold the bars at its
	// ends.
	var half float64
	if xa.cats == nil {
		half = (xa.max - xa.min) * 0.25
		for _, p := range plots {
			if p.kind == "bar" && len(p.values) > 1 {
				half = min(half, (xa.max-xa.min)*0.25/float64(len(p.values)-1))
			}
		}
		if half == 0 {
			half = 0.25
		}
	}
	var rows, domain, named, barKeys []any
	for _, p := range plots {
		domain = append(domain, p.key)
		if p.name != "" {
			named = append(named, p.key)
		}
		if p.kind == "bar" {
			barKeys = append(barKeys, p.key)
		}
		if xa.cats != nil && len(p.values) > len(xa.cats) {
			c.line = 0
			if err := c.add("%d values of the %s %q, which have no category on the x axis", len(p.values)-len(xa.cats), p.kind, p.key); err != nil {
				return nil, false, err
			}
		}
		for i, v := range p.values {
			row := map[string]any{"kind": p.kind, "series": p.key, "name": p.name, "i": i, "y": v, "label": p.labels[i]}
			switch {
			case xa.cats != nil && i >= len(xa.cats):
				continue
			case xa.cats != nil:
				row["x"] = xa.cats[i]
			case len(p.values) == 1:
				row["x"] = xa.min
			default:
				// Mermaid spreads the values of each plot over the range.
				row["x"] = xa.min + float64(i)*(xa.max-xa.min)/float64(len(p.values)-1)
			}
			if ya.ranged {
				row["base"] = ya.min
			}
			if x, ok := row["x"].(float64); ok && p.kind == "bar" {
				row["x0"], row["x1"] = x-half, x+half
			}
			rows = append(rows, row)
		}
	}
	catAxis := axisTitle(xa.title)
	catEnc := map[string]any{"field": "x", "axis": catAxis}
	if xa.cats != nil {
		catDomain := make([]any, 0, len(xa.cats))
		for _, s := range xa.cats {
			catDomain = append(catDomain, s)
		}
		catEnc["type"] = "nominal"
		catEnc["scale"] = map[string]any{"domain": catDomain}
		if !horizontal {
			catAxis["labelAngle"] = 0
		}
	} else {
		lo, hi := xa.min, xa.max
		switch {
		case len(barKeys) > 0:
			lo, hi = lo-half, hi+half
		case lo == hi:
			lo, hi = lo-0.5, hi+0.5
		}
		catEnc["type"] = "quantitative"
		catEnc["scale"] = map[string]any{"domain": []any{lo, hi}, "nice": false, "zero": false}
	}
	valEnc := map[string]any{"field": "y", "type": "quantitative", "axis": axisTitle(ya.title), "stack": nil}
	if ya.ranged {
		valEnc["scale"] = map[string]any{"domain": []any{ya.min, ya.max}, "nice": false, "zero": false}
	}
	color := map[string]any{"field": "series", "type": "nominal", "legend": nil}
	scale := map[string]any{"domain": domain}
	if p := str(cfg, "themeVariables", "xyChart", "plotColorPalette"); p != "" {
		var colors []any
		for s := range strings.SplitSeq(p, ",") {
			colors = append(colors, strings.TrimSpace(s))
		}
		scale["range"] = colors
	}
	color["scale"] = scale
	// The legend names the named plots alone. When every plot is a named
	// line, the names at the ends of the lines take the place of the
	// legend.
	if show, ok := configBool(cfg, "xyChart", "showLegend"); len(named) > 0 && (!ok || show) {
		legend := map[string]any{"title": nil}
		if len(named) < len(plots) {
			legend["values"] = named
		}
		color["legend"] = legend
	}
	encoding := map[string]any{cat: catEnc, val: valEnc, "color": color}
	var layers []any
	if len(barKeys) > 0 {
		bar := map[string]any{
			"mark":      map[string]any{"type": "bar", "clip": ya.ranged, "orient": orient},
			"transform": []any{map[string]any{"filter": "datum.kind == 'bar'"}},
		}
		enc := map[string]any{}
		if ya.ranged {
			enc[val+"2"] = map[string]any{"field": "base"}
		}
		if xa.cats == nil {
			// A bar on a range spans from x0 to x1.
			enc[cat] = map[string]any{"field": "x0", "type": "quantitative"}
			enc[cat+"2"] = map[string]any{"field": "x1"}
		}
		if xa.cats != nil && len(barKeys) > 1 {
			enc[cat+"Offset"] = map[string]any{"field": "series", "type": "nominal", "scale": map[string]any{"domain": barKeys}}
		}
		bar["encoding"] = enc
		layers = append(layers, bar)
		if b, _ := configBool(cfg, "xyChart", "showDataLabel"); b {
			outside, _ := configBool(cfg, "xyChart", "showDataLabelOutsideBar")
			layers = append(layers, xyDataLabels(horizontal, outside, enc))
		}
	}
	if len(barKeys) < len(plots) {
		// A named line has its name at its end, in the look of zuzu, and
		// the style xyUnnamed turns that off for a line with no name.
		for _, named := range []bool{true, false} {
			mark := map[string]any{"type": "line", "clip": ya.ranged}
			test := "datum.kind == 'line' && datum.name != ''"
			if !named {
				mark["style"] = "xyUnnamed"
				test = "datum.kind == 'line' && datum.name == ''"
			}
			layers = append(layers, map[string]any{
				"mark":      mark,
				"transform": []any{map[string]any{"filter": test}},
				"encoding":  map[string]any{"order": map[string]any{"field": "i", "type": "quantitative"}},
			})
		}
		label := map[string]any{"type": "text", "fontWeight": 600}
		if horizontal {
			label["align"], label["dx"] = "left", 8
		} else {
			label["baseline"], label["dy"] = "bottom", -12
		}
		layers = append(layers, map[string]any{
			"mark":      label,
			"transform": []any{map[string]any{"filter": "datum.kind == 'line' && datum.label != ''"}},
			"encoding":  map[string]any{"text": map[string]any{"field": "label"}},
		})
	}
	width, height := 560.0, 320.0
	if v, ok := configNumber(cfg, "xyChart", "width"); ok {
		width = v
	}
	if v, ok := configNumber(cfg, "xyChart", "height"); ok {
		height = v
	}
	var w, h any = width, height
	if horizontal && xa.cats != nil {
		// A step places the first category at the top.
		h = map[string]any{"step": height / float64(max(len(xa.cats), 1))}
	}
	config := map[string]any{
		"style": map[string]any{"xyUnnamed": map[string]any{"zuzu.endLabel": false}},
	}
	// The x axis of Mermaid is the axis of the categories, which is the y
	// of Vega-Lite in a horizontal chart.
	for _, a := range []struct{ name, channel string }{{"xAxis", cat}, {"yAxis", val}} {
		ac := map[string]any{}
		for from, to := range map[string]string{"labelFontSize": "labelFontSize", "titleFontSize": "titleFontSize", "axisLineWidth": "domainWidth"} {
			if v, ok := configNumber(cfg, "xyChart", a.name, from); ok && (from != "axisLineWidth" || (a.name == "xAxis" && xa.cats != nil)) {
				ac[to] = v
			}
		}
		if len(ac) > 0 {
			config["axis"+strings.ToUpper(a.channel)] = ac
		}
	}
	spec := map[string]any{
		"width":    w,
		"height":   h,
		"data":     map[string]any{"values": rows},
		"encoding": encoding,
		"layer":    layers,
		"config":   config,
	}
	if show, ok := configBool(cfg, "xyChart", "showTitle"); ok && !show {
		spec["title"] = map[string]any{"text": ""}
	} else if title != "" {
		t := map[string]any{"text": title, "anchor": "start"}
		if v, ok := configNumber(cfg, "xyChart", "titleFontSize"); ok {
			t["fontSize"] = v
		}
		// Mermaid puts titlePadding above and below the title. Vega-Lite
		// puts the offset between the title and the chart.
		if v, ok := configNumber(cfg, "xyChart", "titlePadding"); ok {
			t["offset"] = v
		}
		spec["title"] = t
	}
	return spec, true, nil
}

// xyDataLabels writes the layer of the values on the bars, inside the end
// of each bar or past it. As in Mermaid, a value is 10 pixels from the end
// of its bar, so that it stays clear of a line that ends there. In a
// horizontal chart, the value is on the middle line of its bar.
func xyDataLabels(horizontal, outside bool, barEnc map[string]any) map[string]any {
	mark := map[string]any{"type": "text", "fontWeight": 600}
	switch {
	case horizontal && outside:
		mark["align"], mark["baseline"], mark["dx"] = "left", "middle", 10
	case horizontal:
		mark["align"], mark["baseline"], mark["dx"] = "right", "middle", -10
	case outside:
		mark["baseline"], mark["dy"] = "alphabetic", -10
	default:
		mark["baseline"], mark["dy"] = "top", 10
	}
	enc := map[string]any{"text": map[string]any{"field": "y", "type": "quantitative"}}
	for k, v := range barEnc {
		if strings.HasSuffix(k, "Offset") {
			enc[k] = v
		}
	}
	if !outside {
		// A label inside a bar takes the color of the text, not the color
		// of the bar.
		enc["color"] = nil
	}
	return map[string]any{
		"mark":      mark,
		"transform": []any{map[string]any{"filter": "datum.kind == 'bar'"}},
		"encoding":  enc,
	}
}

// axisTitle returns the axis of an encoding with a title, or with none.
func axisTitle(t string) map[string]any {
	if t == "" {
		return map[string]any{"title": nil}
	}
	return map[string]any{"title": t}
}

// xyAxis reads the rest of an x-axis or a y-axis line: a title, then the
// categories in brackets or a range min --> max. Only the x axis has
// categories.
func (c *chart) xyAxis(s string, x bool) (*xyAxis, error) {
	toks, err := c.xyTokens(s)
	if err != nil {
		return nil, err
	}
	a := &xyAxis{}
	if len(toks) > 0 && !strings.HasPrefix(toks[0], "[") && toks[0] != "-->" &&
		(len(toks) < 2 || toks[1] != "-->") {
		a.title, toks = unquote(toks[0]), toks[1:]
	}
	switch {
	case len(toks) == 0:
	case len(toks) == 1 && strings.HasPrefix(toks[0], "[") && x:
		a.cats = []string{}
		for _, item := range splitList(toks[0]) {
			a.cats = append(a.cats, unquote(item))
		}
	case len(toks) == 3 && toks[1] == "-->":
		lo, err1 := strconv.ParseFloat(toks[0], 64)
		hi, err2 := strconv.ParseFloat(toks[2], 64)
		if err1 != nil || err2 != nil {
			return nil, c.errorf("the range %s --> %s of an axis", toks[0], toks[2])
		}
		a.ranged, a.min, a.max = true, lo, hi
	default:
		return nil, c.errorf("the axis %q", s)
	}
	return a, nil
}

// xyPlot reads the rest of a bar or a line: a name, then the values in
// brackets. A value of a line can have a label in quotes after it.
func (c *chart) xyPlot(kind, s string) (*xyPlot, error) {
	toks, err := c.xyTokens(s)
	if err != nil {
		return nil, err
	}
	p := &xyPlot{kind: kind}
	if len(toks) == 2 && !strings.HasPrefix(toks[0], "[") {
		p.name, toks = unquote(toks[0]), toks[1:]
	}
	if len(toks) != 1 || !strings.HasPrefix(toks[0], "[") {
		return nil, c.errorf("the %s %q", kind, s)
	}
	for _, item := range splitList(toks[0]) {
		num, label := item, ""
		if q := strings.IndexByte(item, '"'); q > 0 {
			num, label = strings.TrimSpace(item[:q]), unquote(item[q:])
		}
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return nil, c.errorf("the value %q of a %s", item, kind)
		}
		if label != "" && kind == "bar" {
			if err := c.add("the label %q of a bar, which Mermaid also does not draw", label); err != nil {
				return nil, err
			}
			label = ""
		}
		p.values = append(p.values, v)
		p.labels = append(p.labels, label)
	}
	return p, nil
}

// xyTokens splits the rest of a line of an xychart into strings in quotes,
// lists in brackets, arrows and words.
func (c *chart) xyTokens(s string) ([]string, error) {
	var toks []string
	for i := 0; i < len(s); {
		switch {
		case s[i] == ' ' || s[i] == '\t':
			i++
		case strings.HasPrefix(s[i:], "-->"):
			toks = append(toks, "-->")
			i += 3
		case s[i] == '"':
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				return nil, c.errorf("a string with no end quote")
			}
			toks = append(toks, s[i:i+end+2])
			i += end + 2
		case s[i] == '[':
			end := matching(s, i, '[', ']')
			if end < 0 {
				return nil, c.errorf("a list with no end bracket")
			}
			toks = append(toks, s[i:end+1])
			i = end + 1
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '"' && s[j] != '[' && !strings.HasPrefix(s[j:], "-->") {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		}
	}
	return toks, nil
}

// splitList splits a list in brackets at each comma outside quotes, and
// trims each item.
func splitList(s string) []string {
	s = strings.TrimSpace(s[1 : len(s)-1])
	if s == "" {
		return nil
	}
	var items []string
	start := 0
	for i := range len(s) {
		if s[i] == ',' && !inQuotes(s, i) {
			items = append(items, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	return append(items, strings.TrimSpace(s[start:]))
}

// configBool returns a boolean of a config by a path of keys, and whether
// the config holds one.
func configBool(cfg map[string]any, keys ...string) (bool, bool) {
	var v any = cfg
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return false, false
		}
		v = m[k]
	}
	b, ok := v.(bool)
	return b, ok
}
