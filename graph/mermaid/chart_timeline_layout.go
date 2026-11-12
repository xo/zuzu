package mermaid

import (
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// The sizes of a timeline in the pixels of Mermaid 12.3, which scaleWrap
// turns into the pixels of zuzu. In a timeline LR, a period and an event
// are a box of 190 that wraps its text at 150, one each 200 pixels, and a
// section spans its periods. In a timeline TD, a period is a box of 210
// that wraps at 200, 20 pixels left of the axis, and an event is a box of
// 310 that wraps at 300, 50 pixels right of the axis.
const (
	tlStep       = 200.0
	tlBoxLR      = 190.0
	tlWrapLR     = 150.0
	tlPeriodTD   = 210.0
	tlPeriodWrap = 200.0
	tlEventTD    = 310.0
	tlEventWrap  = 300.0
	tlLeftGap    = 20.0
	tlRightGap   = 50.0
)

// The spaces of the look of zuzu in a timeline, in pixels: the space above
// and under the text of a box, the least height of a box, the space between
// two rows, between two events, and between the axis and a box, the length
// of the axis past the boxes, and the radius of the dot of a period.
const (
	tlPadY     = 8.0
	tlMinH     = 30.0
	tlGap      = 12.0
	tlEventGap = 8.0
	tlAxisGap  = 22.0
	tlOver     = 16.0
	tlDotR     = 5.0
)

// The fonts of a timeline: a section and a period are bold, and an event
// is not.
var (
	tlBoldFont = text.Font{Family: text.Family, Size: 11, Weight: boldWeight}
	tlFont     = text.Font{Family: text.Family, Size: 11, Weight: 400}
)

// tlLayout lays out a timeline and writes its rows.
type tlLayout struct {
	t *timeline
	m *text.Estimate
	// keys are the keys of the colors, in the order of the source.
	keys []string

	rows                   map[string][]any
	minX, minY, maxX, maxY float64
}

// tlBox is a box of a timeline with its text: a section, a period or an
// event.
type tlBox struct {
	lines []string
	h     float64
}

// newTimelineLayout lays out a timeline.
func newTimelineLayout(t *timeline) *tlLayout {
	l := &tlLayout{
		t: t, m: text.NewEstimate(), rows: map[string][]any{},
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	if t.dir == "TD" {
		l.vertical()
	} else {
		l.horizontal()
	}
	return l
}

// sectioned reports whether the timeline has a section.
func (l *tlLayout) sectioned() bool {
	return slices.ContainsFunc(l.t.groups, func(g *tlGroup) bool { return g.header })
}

// key returns the key of the color of a period of the group g, whose
// number in the timeline is n. Each group takes the next color. With no
// section, each period takes the next color, unless
// timeline.disableMulticolor is true.
func (l *tlLayout) key(g, n int) string {
	k := 0
	switch {
	case l.sectioned():
		k = g
	case l.t.multicolor:
		k = n
	}
	s := strconv.Itoa(k)
	if !slices.Contains(l.keys, s) {
		l.keys = append(l.keys, s)
	}
	return s
}

// box measures a text that wraps at a width in the pixels of Mermaid.
func (l *tlLayout) box(s string, f text.Font, wrap float64) tlBox {
	lines, _ := plainLines(s, f.Size, f.Weight, scaleWrap(wrap), l.m)
	return tlBox{lines: lines, h: math.Max(tlMinH, float64(len(lines))*lh(f)+2*tlPadY)}
}

// heights returns the height of the headers of the sections and of the
// periods. All the headers have one height, and so do all the periods, as
// in Mermaid.
func (l *tlLayout) heights(section func(g *tlGroup) float64, period float64) (float64, float64) {
	hs, hp := 0.0, 0.0
	for _, g := range l.t.groups {
		if g.header {
			hs = math.Max(hs, l.box(g.name, tlBoldFont, section(g)).h)
		}
		for _, p := range g.periods {
			hp = math.Max(hp, l.box(p.text, tlBoldFont, period).h)
		}
	}
	return hs, hp
}

// horizontal lays out a timeline LR: the headers of the sections at the
// top, the periods under them, the axis, and the events of each period
// under the axis.
func (l *tlLayout) horizontal() {
	span := func(g *tlGroup) float64 { return float64(max(len(g.periods), 1)) }
	hs, hp := l.heights(func(g *tlGroup) float64 { return tlStep*span(g) - 50 }, tlWrapLR)
	yPeriod := 0.0
	if l.sectioned() {
		yPeriod = hs + tlGap
	}
	axisY := yPeriod + hp + tlAxisGap
	step, w := scaleWrap(tlStep), scaleWrap(tlBoxLR)
	x, n := 0.0, 0
	for gi, g := range l.t.groups {
		if g.header {
			b := l.box(g.name, tlBoldFont, tlStep*span(g)-50)
			l.rect("section", "s"+strconv.Itoa(gi), l.key(gi, n), b, x, 0, scaleWrap(tlStep*span(g)-10), hs)
		}
		for pi, p := range g.periods {
			id := "p" + strconv.Itoa(n)
			key := l.key(gi, n)
			px := x + float64(pi)*step
			l.rect("period", id, key, l.box(p.text, tlBoldFont, tlWrapLR), px, yPeriod, w, hp)
			cx, y, end := px+w/2, axisY+tlAxisGap, axisY
			for ei, e := range p.events {
				b := l.box(e, tlFont, tlWrapLR)
				l.rect("event", id+"e"+strconv.Itoa(ei), key, b, px, y, w, b.h)
				end = y + b.h
				y += b.h + tlEventGap
			}
			l.connector(key, cx, yPeriod+hp, cx, end)
			l.dot(key, cx, axisY)
			n++
		}
		x += step * span(g)
	}
	x1 := x - step + w + tlOver
	l.axis(-tlOver, axisY, x1, axisY)
}

// vertical lays out a timeline TD: each section a header across the
// timeline, then a row for each period, the period left of the axis and
// its events right of it.
func (l *tlLayout) vertical() {
	pw, ew := scaleWrap(tlPeriodTD), scaleWrap(tlEventTD)
	axisX := pw + scaleWrap(tlLeftGap)
	ex := axisX + scaleWrap(tlRightGap)
	sw := ex + ew
	sectionWrap := tlPeriodTD + tlLeftGap + tlRightGap + tlEventTD - 10
	hs, hp := l.heights(func(*tlGroup) float64 { return sectionWrap }, tlPeriodWrap)
	y, n := 0.0, 0
	for gi, g := range l.t.groups {
		if g.header {
			l.rect("section", "s"+strconv.Itoa(gi), l.key(gi, n), l.box(g.name, tlBoldFont, sectionWrap), 0, y, sw, hs)
			y += hs + tlGap
		}
		for _, p := range g.periods {
			id := "p" + strconv.Itoa(n)
			key := l.key(gi, n)
			l.rect("period", id, key, l.box(p.text, tlBoldFont, tlPeriodWrap), 0, y, pw, hp)
			l.connector(key, pw, y+hp/2, axisX, y+hp/2)
			l.dot(key, axisX, y+hp/2)
			ey := y
			for ei, e := range p.events {
				b := l.box(e, tlFont, tlEventWrap)
				l.rect("event", id+"e"+strconv.Itoa(ei), key, b, ex, ey, ew, b.h)
				l.connector(key, axisX, ey+b.h/2, ex, ey+b.h/2)
				ey += b.h + tlEventGap
			}
			y = math.Max(y+hp, ey-tlEventGap) + tlGap
			n++
		}
	}
	l.axis(axisX, -tlOver, axisX, y-tlGap+tlOver)
}

// rect writes a box of a kind, section, period or event, with its opaque
// back and its label in the middle.
func (l *tlLayout) rect(kind, id, key string, b tlBox, x, y, w, h float64) {
	l.add("backs", map[string]any{"x": x, "y": y, "x2": x + w, "y2": y + h}, x, y, x+w, y+h)
	l.rows[kind+"s"] = append(l.rows[kind+"s"], map[string]any{
		"id": id, "key": key, "x": x, "y": y, "x2": x + w, "y2": y + h,
	})
	l.rows[kind+"Labels"] = append(l.rows[kind+"Labels"], map[string]any{
		"id": id, "key": key, "x": x + w/2, "y": y + h/2, "lines": anyStrings(b.lines),
	})
}

// connector writes the dashed line from a period to the axis and its
// events.
func (l *tlLayout) connector(key string, x0, y0, x1, y1 float64) {
	l.add("connectors", map[string]any{"key": key, "x": x0, "y": y0, "x2": x1, "y2": y1}, x0, y0, x1, y1)
}

// dot writes the dot of a period on the axis, with a ring of the
// background around it.
func (l *tlLayout) dot(key string, x, y float64) {
	e := tlDotR + 2
	l.add("dotBacks", map[string]any{"x": x, "y": y, "path": circlePath(0, 0, e)}, x-e, y-e, x+e, y+e)
	l.rows["dots"] = append(l.rows["dots"], map[string]any{"key": key, "x": x, "y": y, "path": circlePath(0, 0, tlDotR)})
}

// axis writes the axis from one point to another, with an arrowhead at its
// end.
func (l *tlLayout) axis(x0, y0, x1, y1 float64) {
	const long, half = 9.0, 5.0
	var path string
	if x0 == x1 {
		path = "M" + gitNum(x1-half) + "," + gitNum(y1-long) + "L" + gitNum(x1) + "," + gitNum(y1) + "L" + gitNum(x1+half) + "," + gitNum(y1-long) + "Z"
		y1 -= long / 2
	} else {
		path = "M" + gitNum(x1-long) + "," + gitNum(y1-half) + "L" + gitNum(x1) + "," + gitNum(y1) + "L" + gitNum(x1-long) + "," + gitNum(y1+half) + "Z"
		x1 -= long / 2
	}
	l.add("axis", map[string]any{"x": x0, "y": y0, "x2": x1, "y2": y1}, x0, y0, x1, y1)
	l.add("arrow", map[string]any{"x": 0.0, "y": 0.0, "path": path}, x1-half, y1-half, x1+half, y1+half)
}

// add appends a row to a set and grows the bounds by a box.
func (l *tlLayout) add(set string, row map[string]any, x0, y0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.minX, l.maxX = math.Min(l.minX, math.Min(x0, x1)), math.Max(l.maxX, math.Max(x0, x1))
	l.minY, l.maxY = math.Min(l.minY, math.Min(y0, y1)), math.Max(l.maxY, math.Max(y0, y1))
}

// anyStrings returns a list of strings as a list of values of JSON.
func anyStrings(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31).
// Each key takes the next color of the scheme. The rows move so that the
// timeline starts at 0.
func (l *tlLayout) spec() map[string]any {
	dx, dy := -l.minX, -l.minY
	var data []any
	for _, set := range slices.Sorted(maps.Keys(l.rows)) {
		rows := l.rows[set]
		for _, r := range rows {
			m, _ := r.(map[string]any)
			for k, d := range map[string]float64{"x": dx, "x2": dx, "y": dy, "y2": dy} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round((v+d)*100) / 100
				}
			}
		}
		data = append(data, map[string]any{"name": set, "values": rows})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	color := map[string]any{"scale": "color", "field": "key"}
	var marks []any
	mark := func(typ, set string, style []any, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": set, "from": map[string]any{"data": set},
			"style": style, "encode": map[string]any{"update": enc},
		})
	}
	style := func(s ...string) []any {
		out := make([]any, len(s))
		for i, v := range s {
			out[i] = v
		}
		return out
	}
	rule := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["x2"], enc["y2"] = field("x"), field("y"), field("x2"), field("y2")
		return enc
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = field("x"), field("y"), field("path")
		return enc
	}
	mark("rule", "axis", style(graph.StyleEdge), rule(map[string]any{"strokeWidth": value(2)}))
	mark("path", "arrow", style(graph.StyleArrow), path(map[string]any{}))
	mark("rule", "connectors", style(graph.StyleEdge), rule(map[string]any{
		"stroke": color, "strokeOpacity": value(0.6), "strokeDash": value([]any{4, 3}),
	}))
	mark("path", "dotBacks", style(graph.StyleEdgeLabelBox), path(map[string]any{"fillOpacity": value(1), "strokeOpacity": value(0)}))
	mark("path", "dots", style(graph.StyleNode), path(map[string]any{"fill": color, "stroke": color, "fillOpacity": value(1)}))
	mark("rect", "backs", style(graph.StyleEdgeLabelBox), rule(map[string]any{
		"fillOpacity": value(1), "strokeOpacity": value(0), "cornerRadius": value(4),
	}))
	box := func(opacity float64) map[string]any {
		return rule(map[string]any{"fill": color, "stroke": color, "fillOpacity": value(opacity), "cornerRadius": value(4)})
	}
	mark("rect", "sections", style(graph.StyleNode), box(0.24))
	mark("rect", "periods", style(graph.StyleNode), box(0.14))
	mark("rect", "events", style(graph.StyleNode), box(0.05))
	label := func(f text.Font, enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["text"] = field("x"), field("y"), field("lines")
		enc["align"], enc["baseline"] = value("center"), value("middle")
		enc["fontSize"], enc["fontWeight"], enc["lineHeight"] = value(f.Size), value(f.Weight), value(lh(f))
		return enc
	}
	mark("text", "sectionLabels", style(graph.StyleNodeLabel), label(tlBoldFont, map[string]any{"fill": color}))
	mark("text", "periodLabels", style(graph.StyleNodeLabel), label(tlBoldFont, map[string]any{}))
	mark("text", "eventLabels", style(graph.StyleNodeLabel), label(tlFont, map[string]any{}))
	var domain []any
	for _, k := range l.keys {
		domain = append(domain, k)
	}
	const pad = 8.0
	return map[string]any{
		"width":    math.Ceil(l.maxX - l.minX),
		"height":   math.Ceil(l.maxY - l.minY),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": domain, "range": "category"}},
		"marks":    marks,
	}
}
