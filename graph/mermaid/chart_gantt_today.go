package mermaid

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/xo/zuzu/graph"
)

// ganttToday returns the time of the render as the time of a wall clock in
// UTC, as the plan of a gantt takes its dates (D38). The time keeps the day
// and the hour that it has in its own time zone. A zero time is the start
// of 1970-01-01, as before the time of the render (D31).
func ganttToday(now time.Time) time.Time {
	if now.IsZero() {
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	y, mo, d := now.Date()
	h, mi, s := now.Clock()
	return time.Date(y, mo, d, h, mi, s, now.Nanosecond(), time.UTC)
}

// ganttMargin is the margin of Mermaid at each side of the time axis of a
// gantt, 75 pixels, in the pixels of zuzu.
const ganttMargin = 75.0 * wrapSize / 14

// todayLayer returns the layer of the marker of today, or nil when the gantt
// draws none: a rule across the chart at the time of the render, as Mermaid
// draws it (D38). The time axis runs from lo to hi. Outside the axis,
// Mermaid draws the marker in its margin of 75 pixels at each side of the
// axis, or past the edge of the image, where it does not show. zuzu draws
// the marker in the margin too, at its place in pixels, so that it does
// not stretch the axis, and draws no marker past the margin. The rule
// draws in the width of an edge of a diagram, edgeWidth of docs/STYLE.md,
// and in the color of a fall of the scheme, as Mermaid draws it red. The
// style of todayMarker goes over it.
func (g *gantt) todayLayer(lo, hi time.Time, x map[string]any) (map[string]any, error) {
	if g.c.d.Now.IsZero() || g.todayMarker == "off" {
		return nil, nil
	}
	if g.today.Before(lo) || g.today.After(hi) {
		span := ganttMillis(lo, hi)
		if span <= 0 {
			return nil, nil
		}
		px := g.width * ganttMillis(lo, g.today) / span
		if g.backward() {
			px = g.width - px
		}
		if px < -ganttMargin || px > g.width+ganttMargin {
			return nil, nil
		}
		x = map[string]any{"value": px}
	}
	mark := map[string]any{"type": "rule", "style": graph.StyleEdge}
	if err := g.todayStyle(mark); err != nil {
		return nil, err
	}
	// A rule of one item is a reference line in the look of zuzu, which
	// writes its value as a number. The marker draws twice, so that the
	// mark has more than one item.
	at := map[string]any{"at": g.today.Format(ganttLayout)}
	enc := map[string]any{"x": x}
	// An encoding wins over the mark, so a stroke of todayMarker takes the
	// place of the color of a fall.
	if _, ok := mark["stroke"]; !ok {
		enc["stroke"] = ganttFall()
	}
	return map[string]any{
		"data":     ganttData([]any{at, at}, "at"),
		"mark":     mark,
		"encoding": enc,
	}, nil
}

// todayStyle puts the style of todayMarker, such as
// stroke:#0f0,stroke-width:4px, on the mark of the marker. Mermaid puts the
// value on the line of SVG as its style, with each comma as a semicolon. CSS
// ignores a part with no colon and a value that does not read, and so does
// zuzu, with no issue (D32). A line has no fill and no text, so a fill, a
// color or a font changes nothing in Mermaid, and zuzu skips it with no
// issue too.
func (g *gantt) todayStyle(mark map[string]any) error {
	for _, part := range strings.FieldsFunc(g.todayMarker, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		switch k {
		case "stroke":
			mark["stroke"] = v
		case "stroke-width", "stroke-opacity", "opacity", "stroke-dashoffset":
			n, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
			if err != nil || (n < 0 && k != "stroke-dashoffset") {
				continue
			}
			if strings.HasSuffix(k, "opacity") {
				// CSS clamps an opacity to 1.
				n = min(n, 1)
			}
			name := map[string]string{
				"stroke-width": "strokeWidth", "stroke-opacity": "strokeOpacity",
				"opacity": "opacity", "stroke-dashoffset": "strokeDashOffset",
			}[k]
			mark[name] = n
		case "stroke-dasharray":
			var dash []any
			for f := range strings.FieldsFuncSeq(v, func(r rune) bool { return r == ' ' || r == ',' }) {
				n, err := strconv.ParseFloat(strings.TrimSuffix(f, "px"), 64)
				if err != nil || n < 0 {
					dash = nil
					break
				}
				dash = append(dash, n)
			}
			if dash != nil {
				mark["strokeDash"] = dash
			}
		case "stroke-linecap":
			if slices.Contains([]string{"butt", "round", "square"}, v) {
				mark["strokeCap"] = v
			}
		case "fill", "fill-opacity", "color", "font-family", "font-size", "font-weight", "font-style":
		default:
			g.c.line = g.todayLine
			if err := g.c.add("the style %s of todayMarker", k); err != nil {
				return err
			}
		}
	}
	return nil
}
