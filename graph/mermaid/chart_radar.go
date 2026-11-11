package mermaid

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	// radarAxis matches an axis of a radar: an id and a label in brackets.
	radarAxis = regexp.MustCompile(`^([\w-]+)\s*(?:\[\s*"([^"]*)"\s*\])?$`)
	// radarCurve matches a curve of a radar: an id, a label in brackets and
	// the values in braces.
	radarCurve = regexp.MustCompile(`^([\w-]+)\s*(?:\[\s*"([^"]*)"\s*\])?\s*\{(.*)\}$`)
	// cScale matches a color of the scale of the theme of Mermaid.
	cScale = regexp.MustCompile(`^cScale(\d+)$`)
)

// radarEntry is an axis or a curve of a radar.
type radarEntry struct {
	id, label string
	values    []float64
}

// radarChart compiles a radar: axes around a center and a closed curve
// for each series, into a Vega spec of polygons on radial axes (D31). The
// compiler works out the places in Go, so the spec holds the points, the
// paths of the curves and the rings.
func radarChart(c *chart) (map[string]any, bool, error) {
	cfg, err := c.settings("radar", []string{
		"width", "height", "marginTop", "marginBottom", "marginLeft", "marginRight",
		"axisScaleFactor", "axisLabelFactor", "curveTension",
	}, "themeVariables")
	if err != nil {
		return nil, false, err
	}
	colors, err := c.radarColors()
	if err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	spec := map[string]any{}
	var axes, curves []radarEntry
	type curveLine struct {
		text string
		line int
	}
	var curveLines []curveLine
	minV, maxV := 0.0, math.NaN()
	ticks := 5
	polygon, legend := false, true
	for _, l := range c.d.Lines {
		c.line = l.N
		word, rest, _ := strings.Cut(l.Text, " ")
		rest = strings.TrimSpace(rest)
		switch word {
		case "title":
			spec["title"] = map[string]any{"text": rest, "anchor": "start"}
		case "axis":
			for _, part := range splitTop(rest) {
				m := radarAxis.FindStringSubmatch(part)
				if m == nil {
					return nil, false, c.errorf("the axis %q", part)
				}
				axes = append(axes, radarEntry{id: m[1], label: m[2]})
			}
		case "curve":
			// The curves read after the axes, since a curve can name them.
			curveLines = append(curveLines, curveLine{rest, l.N})
		case "showLegend":
			b, err := strconv.ParseBool(rest)
			if err != nil {
				return nil, false, c.errorf("showLegend %q is not true or false", rest)
			}
			legend = b
		case "min", "max":
			v, err := strconv.ParseFloat(rest, 64)
			if err != nil {
				return nil, false, c.errorf("%s %q is not a number", word, rest)
			}
			if word == "min" {
				minV = v
			} else {
				maxV = v
			}
		case "ticks":
			n, err := strconv.Atoi(rest)
			if err != nil || n < 1 {
				return nil, false, c.errorf("ticks %q is not a whole number above 0", rest)
			}
			ticks = n
		case "graticule":
			switch rest {
			case "circle":
				polygon = false
			case "polygon":
				polygon = true
			default:
				return nil, false, c.errorf("the graticule %q is not circle or polygon", rest)
			}
		default:
			if ok, err := c.accText(l.Text); ok || err != nil {
				if err != nil {
					return nil, false, err
				}
				continue
			}
			return nil, false, c.errorf("the statement %q", l.Text)
		}
	}
	if len(axes) < 3 {
		c.line = 0
		return nil, false, c.errorf("a radar with %d axes, and it needs 3 or more", len(axes))
	}
	byID := map[string]int{}
	for i, a := range axes {
		byID[a.id] = i
	}
	for _, cl := range curveLines {
		c.line = cl.line
		for _, part := range splitTop(cl.text) {
			m := radarCurve.FindStringSubmatch(part)
			if m == nil {
				return nil, false, c.errorf("the curve %q", part)
			}
			e := radarEntry{id: m[1], label: m[2], values: make([]float64, len(axes))}
			vals := splitTop(m[3])
			if len(vals) != len(axes) {
				return nil, false, c.errorf("the curve %s has %d values for %d axes", e.id, len(vals), len(axes))
			}
			keyed := strings.Contains(vals[0], ":")
			done := map[int]bool{}
			for i, v := range vals {
				at := i
				if k, x, ok := field(v); ok != keyed {
					return nil, false, c.errorf("the curve %s mixes values with keys and values without", e.id)
				} else if ok {
					j, found := byID[k]
					if !found {
						return nil, false, c.errorf("the curve %s names the axis %s, which no axis defines", e.id, k)
					}
					if done[j] {
						return nil, false, c.errorf("the curve %s gives the axis %s twice", e.id, k)
					}
					at, v = j, x
					done[j] = true
				}
				f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if err != nil {
					return nil, false, c.errorf("the value %q of the curve %s is not a number", v, e.id)
				}
				e.values[at] = f
			}
			curves = append(curves, e)
		}
	}
	c.line = 0
	if math.IsNaN(maxV) {
		maxV = minV
		for _, e := range curves {
			for _, v := range e.values {
				maxV = math.Max(maxV, v)
			}
		}
	}
	if maxV <= minV {
		return nil, false, c.errorf("the max %v is not above the min %v", maxV, minV)
	}
	w, h := settingNumber(cfg, "width", 400), settingNumber(cfg, "height", 400)
	ml, mr := settingNumber(cfg, "marginLeft", 80), settingNumber(cfg, "marginRight", 80)
	mt, mb := settingNumber(cfg, "marginTop", 30), settingNumber(cfg, "marginBottom", 30)
	cx, cy, radius := ml+w/2, mt+h/2, math.Min(w, h)/2
	axisFactor := settingNumber(cfg, "axisScaleFactor", 1)
	labelFactor := settingNumber(cfg, "axisLabelFactor", 1.05)
	tension := settingNumber(cfg, "curveTension", 0.17)
	angle := func(i int) float64 { return -math.Pi/2 + 2*math.Pi*float64(i)/float64(len(axes)) }
	at := func(i int, r float64) [2]float64 {
		return [2]float64{cx + math.Cos(angle(i))*r, cy + math.Sin(angle(i))*r}
	}
	// The rings of the graticule.
	var rings []any
	for k := 1; k <= ticks; k++ {
		r := radius * float64(k) / float64(ticks)
		var path string
		if polygon {
			pts := make([][2]float64, len(axes))
			for i := range axes {
				pts[i] = at(i, r)
			}
			path = closedPath(pts, 0)
		} else {
			path = "M" + pathPoint(cx+r, cy) + "A" + pathNum(r) + "," + pathNum(r) + " 0 1,1 " + pathPoint(cx-r, cy) +
				"A" + pathNum(r) + "," + pathNum(r) + " 0 1,1 " + pathPoint(cx+r, cy) + "Z"
		}
		rings = append(rings, map[string]any{"ring": k, "path": path})
	}
	var spokes []any
	for i, a := range axes {
		end, lab := at(i, radius*axisFactor), at(i, radius*labelFactor+4)
		cos, sin := math.Cos(angle(i)), math.Sin(angle(i))
		align, baseline := "center", "middle"
		if cos > 0.1 {
			align = "left"
		} else if cos < -0.1 {
			align = "right"
		}
		if sin < -0.1 {
			baseline = "bottom"
		} else if sin > 0.1 {
			baseline = "top"
		}
		label := a.label
		if label == "" {
			label = a.id
		}
		spokes = append(spokes, map[string]any{
			"axis": a.id, "label": label, "x": cx, "y": cy, "x2": end[0], "y2": end[1],
			"lx": lab[0], "ly": lab[1], "align": align, "baseline": baseline,
		})
	}
	var shapes, points []any
	var names []any
	for _, e := range curves {
		name := e.label
		if name == "" {
			name = e.id
		}
		names = append(names, name)
		pts := make([][2]float64, len(axes))
		for i, v := range e.values {
			pts[i] = at(i, radius*math.Max(0, (v-minV)/(maxV-minV)))
			points = append(points, map[string]any{
				"curve": name, "axis": axes[i].id, "value": v, "x": pts[i][0], "y": pts[i][1],
			})
		}
		t := tension
		if polygon {
			t = 0
		}
		shapes = append(shapes, map[string]any{"curve": name, "path": closedPath(pts, t)})
	}
	var rangeOf any = "category"
	if len(colors) > 0 {
		var list []any
		for i := range curves {
			if colors[i] == "" {
				if err := c.add("the colors cScale of Mermaid, which give a color to some curves and not to all"); err != nil {
					return nil, false, err
				}
				list = nil
				break
			}
			list = append(list, colors[i])
		}
		if list != nil {
			rangeOf = list
		}
	}
	spec["width"] = ml + w + mr
	spec["height"] = mt + h + mb
	spec["padding"] = 5
	spec["data"] = []any{
		map[string]any{"name": "rings", "values": rings},
		map[string]any{"name": "axes", "values": spokes},
		map[string]any{"name": "curves", "values": shapes},
		map[string]any{"name": "points", "values": points},
	}
	spec["scales"] = []any{map[string]any{"name": "color", "type": "ordinal", "domain": names, "range": rangeOf}}
	if legend && len(curves) > 0 {
		spec["legends"] = []any{map[string]any{"fill": "color", "orient": "right"}}
	}
	field := func(name string) map[string]any { return map[string]any{"field": name} }
	color := map[string]any{"scale": "color", "field": "curve"}
	spec["marks"] = []any{
		map[string]any{"type": "path", "name": "graticule", "from": map[string]any{"data": "rings"}, "style": []any{"zuzu.edge"}, "encode": map[string]any{"update": map[string]any{
			"path": field("path"), "strokeWidth": map[string]any{"value": 1},
		}}},
		map[string]any{"type": "rule", "name": "axes", "from": map[string]any{"data": "axes"}, "style": []any{"zuzu.edge"}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2"),
		}}},
		map[string]any{"type": "text", "name": "axisLabels", "from": map[string]any{"data": "axes"}, "style": []any{"zuzu.nodeLabel"}, "encode": map[string]any{"update": map[string]any{
			"x": field("lx"), "y": field("ly"), "align": field("align"), "baseline": field("baseline"),
			"fontWeight": map[string]any{"value": 600}, "text": field("label"),
		}}},
		map[string]any{"type": "path", "name": "curves", "from": map[string]any{"data": "curves"}, "encode": map[string]any{"update": map[string]any{
			"path": field("path"), "fill": color, "stroke": color,
			"fillOpacity": map[string]any{"value": 0.16}, "strokeWidth": map[string]any{"value": 2},
		}}},
		map[string]any{"type": "symbol", "name": "points", "from": map[string]any{"data": "points"}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "fill": color, "size": map[string]any{"value": 30},
		}}},
	}
	return spec, false, nil
}

// radarColors reads the theme variables of a radar: the colors of the
// curves, cScale0 and up, by their number. Each other variable is an
// issue.
func (c *chart) radarColors() (map[int]string, error) {
	tv, _ := c.d.Config["themeVariables"].(map[string]any)
	colors := map[int]string{}
	for _, k := range slices.Sorted(maps.Keys(tv)) {
		if m := cScale.FindStringSubmatch(k); m != nil {
			if s, ok := tv[k].(string); ok {
				n, _ := strconv.Atoi(m[1])
				colors[n] = s
				continue
			}
		}
		if err := c.add("the theme variable %s of Mermaid, drawn in the look of zuzu", k); err != nil {
			return nil, err
		}
	}
	return colors, nil
}

// closedPath returns the path of a closed curve through points: straight
// lines with a tension of 0, and else a smooth curve. The control points
// of the part from one point to the next follow the line between the two
// points around them, by the tension.
func closedPath(pts [][2]float64, tension float64) string {
	n := len(pts)
	var b strings.Builder
	b.WriteString("M" + pathPoint(pts[0][0], pts[0][1]))
	for i := range n {
		p0, p1, p2, p3 := pts[(i+n-1)%n], pts[i], pts[(i+1)%n], pts[(i+2)%n]
		if tension == 0 {
			if i < n-1 {
				b.WriteString("L" + pathPoint(p2[0], p2[1]))
			}
			continue
		}
		c1 := [2]float64{p1[0] + (p2[0]-p0[0])*tension, p1[1] + (p2[1]-p0[1])*tension}
		c2 := [2]float64{p2[0] - (p3[0]-p1[0])*tension, p2[1] - (p3[1]-p1[1])*tension}
		b.WriteString("C" + pathPoint(c1[0], c1[1]) + " " + pathPoint(c2[0], c2[1]) + " " + pathPoint(p2[0], p2[1]))
	}
	b.WriteString("Z")
	return b.String()
}

// pathPoint writes a point of a path.
func pathPoint(x, y float64) string {
	return pathNum(x) + "," + pathNum(y)
}

// pathNum writes a number of a path with at most two decimals.
func pathNum(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

// splitTop splits a list at its commas, except the commas inside quotes,
// brackets and braces, and trims each part.
func splitTop(s string) []string {
	var parts []string
	depth, quoted, start := 0, false, 0
	for i := range len(s) {
		switch ch := s[i]; {
		case ch == '"':
			quoted = !quoted
		case quoted:
		case ch == '[' || ch == '{':
			depth++
		case ch == ']' || ch == '}':
			depth--
		case ch == ',' && depth == 0:
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" || len(parts) > 0 {
		parts = append(parts, last)
	}
	return parts
}
