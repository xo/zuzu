package mermaid

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// quadPoint is a point of a quadrant chart.
type quadPoint struct {
	name, class string
	x, y        float64
	style       quadStyle
}

// quadStyle is the style of a point: a fill, a radius and a border. An
// empty field takes the value of the class, then of the theme.
type quadStyle struct {
	color, strokeColor  string
	radius, strokeWidth float64
}

// quadPointLine matches a point: its name, its class, x and y, and its
// styles.
var quadPointLine = regexp.MustCompile(`^("[^"]*"|[^:\[]+?)\s*(?::::([\w-]+))?\s*:\s*\[([^\]]*)\]\s*(.*)$`)

// quadrantChart compiles a quadrant chart into points of Vega-Lite on two
// axes from 0 to 1, with the four quadrants and their names (D31).
func quadrantChart(c *chart) (map[string]any, bool, error) {
	cfg := c.d.Config
	if err := c.unreadSettings("quadrantChart.chartWidth", "quadrantChart.chartHeight",
		"quadrantChart.pointRadius", "quadrantChart.xAxisPosition", "quadrantChart.yAxisPosition"); err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the header %q of a quadrant chart", c.d.Header)
	}
	var title string
	var xAxis, yAxis []string
	quadrants := make([]string, 4)
	classes := map[string]quadStyle{}
	var points []quadPoint
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
		word, rest, _ := strings.Cut(l.Text, " ")
		rest = strings.TrimSpace(rest)
		switch word {
		case "title":
			title = unquote(rest)
			continue
		case "x-axis", "y-axis":
			var ends []string
			for e := range strings.SplitSeq(rest, "-->") {
				ends = append(ends, unquote(e))
			}
			if len(ends) > 2 || ends[0] == "" {
				return nil, false, c.errorf("the axis %q", rest)
			}
			if word == "x-axis" {
				xAxis = ends
			} else {
				yAxis = ends
			}
			continue
		case "quadrant-1", "quadrant-2", "quadrant-3", "quadrant-4":
			quadrants[word[len(word)-1]-'1'] = unquote(rest)
			continue
		case "classDef":
			name, body, _ := strings.Cut(rest, " ")
			st, err := c.quadStyle(body)
			if err != nil {
				return nil, false, err
			}
			classes[name] = st
			continue
		}
		m := quadPointLine.FindStringSubmatch(l.Text)
		if m == nil {
			return nil, false, c.errorf("the line %q of a quadrant chart", l.Text)
		}
		p := quadPoint{name: unquote(m[1]), class: m[2]}
		xs, ys, ok := strings.Cut(m[3], ",")
		x, err1 := strconv.ParseFloat(strings.TrimSpace(xs), 64)
		y, err2 := strconv.ParseFloat(strings.TrimSpace(ys), 64)
		if !ok || err1 != nil || err2 != nil {
			return nil, false, c.errorf("the place [%s] of the point %q", m[3], p.name)
		}
		if x < 0 || x > 1 || y < 0 || y > 1 {
			return nil, false, c.errorf("the place [%s] of the point %q is not from 0 to 1", m[3], p.name)
		}
		p.x, p.y = x, y
		if p.style, err = c.quadStyle(m[4]); err != nil {
			return nil, false, err
		}
		points = append(points, p)
	}
	c.line = 0
	// A point takes each style from itself, then from its class, then
	// from the config.
	radius := 5.0
	if v, ok := configNumber(cfg, "quadrantChart", "pointRadius"); ok {
		radius = v
	}
	for i := range points {
		p := &points[i]
		if p.class != "" {
			cl, ok := classes[p.class]
			if !ok {
				c.line = 0
				if err := c.add("the class %s of the point %q, which no classDef defines", p.class, p.name); err != nil {
					return nil, false, err
				}
			}
			p.style = p.style.or(cl)
		}
		p.style = p.style.or(quadStyle{radius: radius})
	}
	return quadSpec(cfg, title, xAxis, yAxis, quadrants, points), true, nil
}

// or returns the style with each empty field taken from another.
func (s quadStyle) or(o quadStyle) quadStyle {
	if s.color == "" {
		s.color = o.color
	}
	if s.strokeColor == "" {
		s.strokeColor = o.strokeColor
	}
	if s.radius == 0 {
		s.radius = o.radius
	}
	if s.strokeWidth == 0 {
		s.strokeWidth = o.strokeWidth
	}
	return s
}

// quadStyle reads the styles of a point or of a class, as color: #f00,
// radius: 10, stroke-color: #00f, stroke-width: 2px.
func (c *chart) quadStyle(s string) (quadStyle, error) {
	var st quadStyle
	for part := range strings.SplitSeq(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		k, v, ok := field(part)
		if !ok || v == "" {
			return st, c.errorf("the style %q of a point", strings.TrimSpace(part))
		}
		switch k {
		case "color":
			st.color = v
		case "stroke-color":
			st.strokeColor = v
		case "radius", "stroke-width":
			n, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
			if err != nil || n < 0 {
				return st, c.errorf("the %s %q of a point", k, v)
			}
			if k == "radius" {
				st.radius = n
			} else {
				st.strokeWidth = n
			}
		default:
			return st, c.errorf("the style %q of a point", k)
		}
	}
	return st, nil
}

// quadSpec writes the spec of a quadrant chart.
func quadSpec(cfg map[string]any, title string, xAxis, yAxis, quadrants []string, points []quadPoint) map[string]any {
	// The quadrants are numbered as in mathematics: 1 at the top right,
	// then counterclockwise. The first is the darkest, as in Mermaid.
	boxes := [4][4]float64{{0.5, 1, 0.5, 1}, {0, 0.5, 0.5, 1}, {0, 0.5, 0, 0.5}, {0.5, 1, 0, 0.5}}
	shades := [4]float64{0.16, 0.11, 0.07, 0.04}
	rows := make([]any, 0, 8+len(points))
	for i, b := range boxes {
		rows = append(rows, map[string]any{
			"kind": "quadrant", "quadrant": i + 1, "label": quadrants[i],
			"x0": b[0], "x1": b[1], "y0": b[2], "y1": b[3], "shade": shades[i],
			"cx": (b[0] + b[1]) / 2, "cy": (b[2] + b[3]) / 2, "top": b[3] - 0.02,
		})
	}
	// Each set of points with the same style is a layer of its own.
	var groups []quadStyle
	for _, p := range points {
		k := slices.Index(groups, p.style)
		if k < 0 {
			k = len(groups)
			groups = append(groups, p.style)
		}
		rows = append(rows, map[string]any{"kind": "point", "label": p.name, "x": p.x, "y": p.y, "group": k})
	}
	x := map[string]any{"field": "x", "type": "quantitative", "scale": map[string]any{"domain": []any{0, 1}, "nice": false}}
	y := map[string]any{"field": "y", "type": "quantitative", "scale": map[string]any{"domain": []any{0, 1}, "nice": false}}
	x["axis"], y["axis"] = nil, nil
	// The names of the ends of the axes are text, at the middle of each
	// half, just outside the plot. With points, the x axis is always at the
	// bottom, as in Mermaid.
	xBottom := len(points) > 0 || str(cfg, "quadrantChart", "xAxisPosition") == "bottom"
	yRight := str(cfg, "quadrantChart", "yAxisPosition") == "right"
	for i, e := range xAxis {
		row := map[string]any{"kind": "xAxis", "label": e, "x": 0.25 + 0.5*float64(i), "y": 1.02}
		if xBottom {
			row["y"] = -0.02
		}
		rows = append(rows, row)
	}
	for i, e := range yAxis {
		row := map[string]any{"kind": "yAxis", "label": e, "x": -0.02, "y": 0.25 + 0.5*float64(i)}
		if yRight {
			row["x"] = 1.02
		}
		rows = append(rows, row)
	}
	xLabel := map[string]any{"type": "text", "fontSize": 12, "align": "center", "baseline": "bottom"}
	if xBottom {
		xLabel["baseline"] = "top"
	}
	// A turned label reads from the bottom up, so its top is on the left.
	yLabel := map[string]any{"type": "text", "fontSize": 12, "align": "center", "angle": -90, "baseline": "bottom"}
	if yRight {
		yLabel["baseline"] = "top"
	}
	nameY, baseline := "cy", "middle"
	if len(points) > 0 {
		// With points, the name of a quadrant is at its top.
		nameY, baseline = "top", "top"
	}
	layers := []any{
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'quadrant'"}},
			"mark":      map[string]any{"type": "rect"},
			"encoding": map[string]any{
				"x": map[string]any{"field": "x0", "type": "quantitative"}, "x2": map[string]any{"field": "x1"},
				"y": map[string]any{"field": "y0", "type": "quantitative"}, "y2": map[string]any{"field": "y1"},
				"opacity": map[string]any{"field": "shade", "type": "quantitative", "scale": nil},
			},
		},
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'quadrant' && datum.label != ''"}},
			"mark":      map[string]any{"type": "text", "fontSize": 13, "fontWeight": 600, "align": "center", "baseline": baseline},
			"encoding": map[string]any{
				"x":    map[string]any{"field": "cx", "type": "quantitative"},
				"y":    map[string]any{"field": nameY, "type": "quantitative"},
				"text": map[string]any{"field": "label"},
			},
		},
	}
	layers = append(layers,
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'xAxis'"}},
			"mark":      xLabel,
			"encoding":  map[string]any{"text": map[string]any{"field": "label"}},
		},
		map[string]any{
			"transform": []any{map[string]any{"filter": "datum.kind == 'yAxis'"}},
			"mark":      yLabel,
			"encoding":  map[string]any{"text": map[string]any{"field": "label"}},
		})
	for k, g := range groups {
		filter := "datum.kind == 'point' && datum.group == " + strconv.Itoa(k)
		mark := map[string]any{"type": "circle", "opacity": 1, "size": 4 * g.radius * g.radius}
		if g.color != "" {
			mark["color"] = g.color
		}
		if g.strokeWidth > 0 {
			mark["strokeWidth"] = g.strokeWidth
			if g.strokeColor != "" {
				mark["stroke"] = g.strokeColor
			}
		}
		layers = append(layers,
			map[string]any{"transform": []any{map[string]any{"filter": filter}}, "mark": mark},
			map[string]any{
				"transform": []any{map[string]any{"filter": filter}},
				"mark": map[string]any{
					"type": "text", "align": "center", "baseline": "top", "fontSize": 11,
					"dy": g.radius + g.strokeWidth/2 + 3,
				},
				"encoding": map[string]any{"text": map[string]any{"field": "label"}},
			})
	}
	width, height := 400.0, 400.0
	if v, ok := configNumber(cfg, "quadrantChart", "chartWidth"); ok {
		width = v
	}
	if v, ok := configNumber(cfg, "quadrantChart", "chartHeight"); ok {
		height = v
	}
	spec := map[string]any{
		"width":    width,
		"height":   height,
		"data":     map[string]any{"values": rows},
		"encoding": map[string]any{"x": x, "y": y},
		"layer":    layers,
	}
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	return spec
}
