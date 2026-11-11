package mermaid

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/text"
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
	placement := "bisector"
	switch v := str(c.d.Config, "pie", "zuzu.textPlacement"); v {
	case "":
	case "bisector", "centroid", "circle":
		placement = v
	default:
		if err := c.add("the text placement %s of a pie chart", v); err != nil {
			return nil, false, err
		}
	}
	read := []string{"pie.donutHole", "pie.legendPosition", "pie.zuzu.textPlacement"}
	if placement == "bisector" {
		read = append(read, "pie.textPosition")
	}
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
			c.line = c.d.N
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
	// the percent of each slice on its middle angle, at the distance from
	// the center that the text placement gives, with the center of the text
	// on that point.
	const size = 300
	ro, ri := size/2.0, hole*size/2
	rows := make([]any, 0, len(parts))
	domain := make([]any, 0, len(parts))
	var outside []*pieLabel
	measure := text.NewEstimate()
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
		mid, span := 0.0, 0.0
		if sum > 0 {
			mid, span = 2*math.Pi*(cum+s.value/2)/sum, 2*math.Pi*s.value/sum
		}
		r := textPosition * ro
		switch placement {
		case "centroid":
			r = sectorCentroid(span, ri, ro)
		case "circle":
			r = sectorCircle(span, ri, ro)
		}
		start := 0.0
		if sum > 0 {
			start = 2 * math.Pi * cum / sum
		}
		cum += s.value
		domain = append(domain, name)
		x, y := r*math.Sin(mid), r*math.Cos(mid)
		row := map[string]any{
			"kind": "slice", "i": i, "label": s.label, "value": s.value,
			"name": name, "percent": percent, "tx": size/2 + x, "ty": size/2 - y,
		}
		rows = append(rows, row)
		// A label that does not fit in its slice goes outside the pie.
		if percent != "" {
			w := measure.Width(text.Font{Family: text.Family, Size: pieLabelSize, Weight: 600}, percent)
			if !pieLabelFits(x, y, w, pieLabelSize, start, span, ri, ro) {
				outside = append(outside, &pieLabel{row: row, mid: mid, w: w})
			}
		}
	}
	placeOutside(outside, size/2, size/2, ro, size)
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
			pieLeader("px", "py", "ex", "ey", pieScale(domain, colors)),
			pieLeader("ex", "ey", "lx", "ly", pieScale(domain, colors)),
			map[string]any{
				"mark":      map[string]any{"type": "text", "fontWeight": 600, "align": "center", "baseline": "middle"},
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

// pieLabelSize is the font size of the percent of a slice.
const pieLabelSize = 11

// The leader line of a label outside the pie leaves the rim of the slice at
// its middle angle, and bends at pieElbow pixels out from the rim. The
// label starts pieColumn pixels out from the side of the pie, and the line
// stops pieLeaderGap pixels before the label. Two labels outside keep a
// gap of pieLabelGap pixels.
const (
	pieElbow     = 10
	pieColumn    = 18
	pieLeaderGap = 3
	pieLabelGap  = 2
	pieMargin    = 2
)

// pieLabel is a label of a slice that goes outside the pie.
type pieLabel struct {
	row map[string]any
	mid float64 // the middle angle of the slice, clockwise from the top
	w   float64 // the width of the label
	y   float64 // the middle of the label
}

// pieLabelFits reports whether a label of a width and a height, with its
// center at x, y from the center of the pie, with y up, lies inside a
// slice. The slice starts at the angle start, clockwise from the top, and
// spans the angle span, on a ring from the radius ri to the radius ro. The
// label keeps a margin of pieMargin from the arcs and the edges of the
// slice.
func pieLabelFits(x, y, w, h, start, span, ri, ro float64) bool {
	x1, x2, y1, y2 := x-w/2, x+w/2, y-h/2, y+h/2
	// The farthest point of a box from the center is a corner.
	for _, c := range [][2]float64{{x1, y1}, {x2, y1}, {x1, y2}, {x2, y2}} {
		if math.Hypot(c[0], c[1]) > ro-pieMargin {
			return false
		}
	}
	// The nearest point of a box to the center.
	if ri > 0 && math.Hypot(min(max(0, x1), x2), min(max(0, y1), y2)) < ri+pieMargin {
		return false
	}
	if span >= 2*math.Pi-1e-9 {
		return true
	}
	// Each point on the edge of the box, at steps of at most a pixel, lies
	// between the two edges of the slice, at least the margin from each.
	inSlice := func(px, py float64) bool {
		r := math.Hypot(px, py)
		if r == 0 {
			return false
		}
		t := math.Mod(math.Atan2(px, py)-start+4*math.Pi, 2*math.Pi)
		if t > span {
			return false
		}
		edge := func(d float64) float64 {
			if d >= math.Pi/2 {
				return r
			}
			return r * math.Sin(d)
		}
		return edge(t) >= pieMargin && edge(span-t) >= pieMargin
	}
	steps := func(a, b float64) int { return max(1, int(math.Ceil(b-a))) }
	nx, ny := steps(x1, x2), steps(y1, y2)
	for k := range nx + 1 {
		px := x1 + (x2-x1)*float64(k)/float64(nx)
		if !inSlice(px, y1) || !inSlice(px, y2) {
			return false
		}
	}
	for k := range ny + 1 {
		py := y1 + (y2-y1)*float64(k)/float64(ny)
		if !inSlice(x1, py) || !inSlice(x2, py) {
			return false
		}
	}
	return true
}

// placeOutside places the labels that do not fit in their slices outside
// a pie with its center at cx, cy and the radius ro, in a chart of a
// height. A label goes in a column at the right of the pie when its slice
// points right, and else at the left. In each column, the labels sort by
// the height of the bend of their leader lines, and each moves down to
// clear the label above. If the last label leaves the chart, the column
// moves up. Each label gets its center in tx and ty, and its leader line
// from px, py on the rim to the bend at ex, ey and on to lx, ly.
func placeOutside(labels []*pieLabel, cx, cy, ro, height float64) {
	var left, right []*pieLabel
	for _, l := range labels {
		sin, cos := math.Sin(l.mid), math.Cos(l.mid)
		l.row["outside"] = true
		l.row["px"], l.row["py"] = cx+ro*sin, cy-ro*cos
		l.row["ex"], l.row["ey"] = cx+(ro+pieElbow)*sin, cy-(ro+pieElbow)*cos
		l.y = cy - (ro+pieElbow)*cos
		if l.mid < math.Pi {
			right = append(right, l)
		} else {
			left = append(left, l)
		}
	}
	for _, col := range [][]*pieLabel{left, right} {
		slices.SortStableFunc(col, func(a, b *pieLabel) int { return cmp.Compare(a.y, b.y) })
		for i, l := range col {
			l.y = max(l.y, pieLabelSize/2)
			if i > 0 {
				l.y = max(l.y, col[i-1].y+pieLabelSize+pieLabelGap)
			}
		}
		if n := len(col); n > 0 {
			if over := col[n-1].y + pieLabelSize/2 - height; over > 0 {
				// The column moves up, but its first label stays in the
				// chart.
				over = min(over, col[0].y-pieLabelSize/2)
				for _, l := range col {
					l.y -= over
				}
			}
		}
	}
	for _, l := range labels {
		x := cx + ro + pieColumn
		l.row["tx"], l.row["lx"] = x+l.w/2, x-pieLeaderGap
		if l.mid >= math.Pi {
			x = cx - ro - pieColumn
			l.row["tx"], l.row["lx"] = x-l.w/2, x+pieLeaderGap
		}
		l.row["ty"], l.row["ly"] = l.y, l.y
	}
}

// pieLeader returns a layer that draws one part of the leader lines of the
// labels outside a pie, from the point of two fields to the point of two
// others, in the color of the slice. The layer takes the color scale of
// the slices, so that a line has the color of its own slice.
func pieLeader(x, y, x2, y2 string, colors map[string]any) map[string]any {
	pos := func(f string) map[string]any {
		return map[string]any{"field": f, "type": "quantitative", "scale": nil}
	}
	return map[string]any{
		"mark":      map[string]any{"type": "rule", "strokeWidth": 1},
		"transform": []any{map[string]any{"filter": "datum.outside === true"}},
		"encoding": map[string]any{
			"x": pos(x), "y": pos(y), "x2": pos(x2), "y2": pos(y2),
			"color": map[string]any{"field": "name", "type": "nominal", "scale": colors},
		},
	}
}

// sectorCentroid returns the distance from the center of a pie to the
// centroid of a slice that spans an angle, in radians, on a ring from the
// radius ri to the radius ro. The centroid of a wide slice of a ring falls
// toward the hole, so the distance is never less than the middle of the
// ring.
func sectorCentroid(span, ri, ro float64) float64 {
	if span <= 0 {
		return (ri + ro) / 2
	}
	d := 4 * math.Sin(span/2) * (ro*ro*ro - ri*ri*ri) / (3 * span * (ro*ro - ri*ri))
	if ri > 0 {
		d = math.Max(d, (ri+ro)/2)
	}
	return d
}

// sectorCircle returns the distance from the center of a pie to the center
// of the largest circle in a slice that spans an angle, in radians, on a
// ring from the radius ri to the radius ro. The circle touches the outer
// arc, and also the inner arc or the two sides of the slice, whichever it
// meets first. A slice of 180 degrees or more has sides that a circle on
// its middle angle does not meet, and a slice of the whole pie has none.
func sectorCircle(span, ri, ro float64) float64 {
	if span >= 2*math.Pi-1e-9 && ri == 0 {
		return 0
	}
	if span >= math.Pi {
		return (ri + ro) / 2
	}
	s := math.Sin(span / 2)
	radius := func(t float64) float64 { return min(ro-t, t-ri, t*s) }
	best := (ri + ro) / 2
	if t := ro / (1 + s); t >= ri && radius(t) > radius(best) {
		best = t
	}
	return best
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
