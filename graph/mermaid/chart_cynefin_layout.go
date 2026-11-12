package mermaid

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// The sizes of a Cynefin diagram in pixels, as Mermaid 12.3 lays it out:
// an item is a badge 26 high, 30 under the badge before it, 10 wider than
// its text on each side. The items of a domain start 25 under its center,
// or 15 without the descriptions, and those of confusion 22 or 14. The
// ellipse of confusion is 0.15 of the width and of the height across each
// half. The center of confusion holds 3 items at most, and a badge "+N
// more" counts the rest.
const (
	cyItemH       = 26.0
	cyItemStep    = 30.0
	cyItemPad     = 10.0
	cyItemTop     = 25.0
	cyItemTopBare = 15.0
	cyMidTop      = 22.0
	cyMidTopBare  = 14.0
	cyMidRadius   = 0.15
	cyMidItems    = 3
)

// The fonts of a Cynefin diagram: the name of a domain, its description,
// and the label of an item or a transition.
var (
	cyNameFont  = text.Font{Family: text.Family, Size: 13, Weight: boldWeight}
	cyNoteFont  = text.Font{Family: text.Family, Size: 10, Weight: 400, Italic: true}
	cyLabelFont = text.Font{Family: text.Family, Size: 11, Weight: 400}
)

// cyNotes are the descriptions of the domains: the model and the practice
// of each domain, and the practice of confusion, as Mermaid writes them.
var cyNotes = map[string][2]string{
	cyComplex:     {"Probe → Sense → Respond", "Emergent Practices"},
	cyComplicated: {"Sense → Analyse → Respond", "Good Practices"},
	cyClear:       {"Sense → Categorise → Respond", "Best Practices"},
	cyChaotic:     {"Act → Sense → Respond", "Novel Practices"},
	cyConfusion:   {"", "Disorder"},
}

// cyScale is the domain of the scale of the colors of the domains. The
// order gives each domain the hue that Mermaid gives it in the scheme of
// zuzu: blue violet for complicated, teal for complex, saffron for clear,
// raspberry for chaotic and violet for confusion. The fifth color, sky,
// goes to no domain.
var cyScale = []any{cyComplicated, cyComplex, cyClear, cyChaotic, "", cyConfusion}

// cyDefaultID is the id of the SVG that mmdc draws, whose hash seeds the
// waves of the boundaries when cynefin.seed is 0, so that zuzu draws the
// waves of mmdc.
const cyDefaultID = "my-svg"

// cyLayout lays out a Cynefin diagram and writes its rows.
type cyLayout struct {
	y    *cynefin
	m    *text.Estimate
	rows map[string][]any
	// w and h are the size of the diagram, and cx and cy the center of
	// each domain.
	w, h   float64
	cx, cy map[string]float64
}

// newCynefinLayout lays out a Cynefin diagram: the four domains in the
// quarters of the diagram, complex at the top left, complicated at the
// top right, chaotic at the bottom left and clear at the bottom right,
// the wavy boundaries between them, the boundary between clear and
// chaotic, the ellipse of confusion in the middle, the names, the items
// and the transitions.
func newCynefinLayout(y *cynefin) *cyLayout {
	w, h := y.cfg.width, y.cfg.height
	l := &cyLayout{
		y: y, m: text.NewEstimate(), rows: map[string][]any{}, w: w, h: h,
		cx: map[string]float64{
			cyComplex: w / 4, cyComplicated: w * 3 / 4, cyChaotic: w / 4, cyClear: w * 3 / 4, cyConfusion: w / 2,
		},
		cy: map[string]float64{
			cyComplex: h / 4, cyComplicated: h / 4, cyChaotic: h * 3 / 4, cyClear: h * 3 / 4, cyConfusion: h / 2,
		},
	}
	for _, d := range cyDomains[:4] {
		x0, y0 := l.cx[d]-w/4, l.cy[d]-h/4
		l.add("domains", map[string]any{"domain": d, "x": x0, "y": y0, "x2": x0 + w/2, "y2": y0 + h/2})
	}
	seed := y.cfg.seed
	if seed == 0 || math.IsInf(seed, 0) || math.IsNaN(seed) {
		seed = float64(cyHash(cyDefaultID))
	}
	a := y.cfg.amplitude
	l.add("boundaries", map[string]any{"path": cyFold(w, h, seed, a)})
	l.add("boundaries", map[string]any{"path": cyCross(w, h, seed+100, a)})
	l.add("cliff", map[string]any{"path": cyCliff(w, h)})
	l.add("confusion", map[string]any{"domain": cyConfusion, "path": cyEllipse(w/2, h/2, w*cyMidRadius, h*cyMidRadius)})
	l.names()
	l.items()
	l.transitions()
	return l
}

// add appends a row to a set of rows.
func (l *cyLayout) add(set string, row map[string]any) {
	l.rows[set] = append(l.rows[set], row)
}

// names writes the name of each domain and, unless
// cynefin.showDomainDescriptions is false, its description under it.
func (l *cyLayout) names() {
	notes := l.y.cfg.descriptions
	for _, d := range []string{cyComplex, cyComplicated, cyChaotic, cyClear} {
		x, y := l.cx[d], l.cy[d]
		ny := y
		if notes {
			ny = y - 30
			l.add("notes", map[string]any{"text": cyNotes[d][0], "x": x, "y": y - 10})
			l.add("notes", map[string]any{"text": cyNotes[d][1], "x": x, "y": y + 5})
		}
		l.add("names", map[string]any{"domain": d, "text": strings.ToUpper(d[:1]) + d[1:], "x": x, "y": ny})
	}
	x, y := l.w/2, l.h/2
	ny := y
	if notes {
		ny = y - 10
		l.add("notes", map[string]any{"text": cyNotes[cyConfusion][1], "x": x, "y": y + 8})
	}
	l.add("names", map[string]any{"domain": cyConfusion, "text": "Confusion", "x": x, "y": ny})
}

// items writes the badges of the items of each domain, centered under its
// name, one under the other. Confusion shows 3 items and a badge for the
// rest.
func (l *cyLayout) items() {
	notes := l.y.cfg.descriptions
	for _, d := range cyDomains {
		labels := l.y.items[d]
		if len(labels) == 0 {
			continue
		}
		top := cyItemTop
		if !notes {
			top = cyItemTopBare
		}
		more := 0
		if d == cyConfusion {
			top = cyMidTop
			if !notes {
				top = cyMidTopBare
			}
			if len(labels) > cyMidItems {
				more = len(labels) - cyMidItems
				labels = labels[:cyMidItems]
			}
		}
		y := l.cy[d] + top
		for i, s := range labels {
			l.badge(d, d+strconv.Itoa(i), s, false, y)
			y += cyItemStep
		}
		if more > 0 {
			l.badge(d, d+"More", "+"+strconv.Itoa(more)+" more", true, y)
		}
	}
}

// badge writes a badge of an item of a domain, with its top at y.
func (l *cyLayout) badge(d, id, s string, more bool, y float64) {
	w := l.m.Width(cyLabelFont, s) + 2*cyItemPad
	x := l.cx[d] - w/2
	l.add("itemBacks", map[string]any{"x": x, "y": y, "x2": x + w, "y2": y + cyItemH})
	set := "items"
	if more {
		set = "itemsMore"
	}
	l.add(set, map[string]any{"id": id, "domain": d, "x": x, "y": y, "x2": x + w, "y2": y + cyItemH})
	l.add("itemLabels", map[string]any{"id": id, "domain": d, "text": s, "x": l.cx[d], "y": y + cyItemH/2})
}

// transitions writes each transition as a curve from the center of one
// domain to the center of another, bent to its left by 0.15 of its length,
// with an arrowhead at its end and its label at the control point of the
// curve, as Mermaid draws it.
func (l *cyLayout) transitions() {
	glyph, _ := graph.ArrowPaths("normal")
	for i, t := range l.y.transitions {
		x1, y1, x2, y2 := l.cx[t.from], l.cy[t.from], l.cx[t.to], l.cy[t.to]
		px, py := (x1+x2)/2-(y2-y1)*0.15, (y1+y2)/2+(x2-x1)*0.15
		// The curve ends inside the arrowhead, so that its end does not
		// show past the tip.
		tx, ty := x2-px, y2-py
		tl := math.Hypot(tx, ty)
		ex, ey := x2-tx/tl*theme.ArrowLength/2, y2-ty/tl*theme.ArrowLength/2
		id := "t" + strconv.Itoa(i)
		l.add("transitions", map[string]any{
			"id": id, "from": t.from, "to": t.to,
			"path": "M" + num(x1) + "," + num(y1) + "Q" + num(px) + "," + num(py) + " " + num(ex) + "," + num(ey),
		})
		l.add("arrows", map[string]any{"id": id, "x": x2, "y": y2, "angle": math.Atan2(ty, tx) * 180 / math.Pi, "glyph": glyph})
		if t.label == "" {
			continue
		}
		// Mermaid puts the baseline of the label 6 pixels over the control
		// point.
		w := l.m.Width(cyLabelFont, t.label)
		h := lh(cyLabelFont)
		cy := py - 6 - h/2
		l.add("transitionBoxes", map[string]any{"id": id, "x": px - w/2 - 3, "y": cy - h/2 - 1, "x2": px + w/2 + 3, "y2": cy + h/2 + 1})
		l.add("transitionLabels", map[string]any{"id": id, "text": t.label, "x": px, "y": cy})
	}
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is
// a data set, and each mark draws one set in a style of a diagram (D31).
// The domains take the colors of the category scheme, or the colors of
// themeVariables.cynefin that the source gives.
func (l *cyLayout) spec() map[string]any {
	cfg := l.y.cfg
	order := []string{
		"domains", "boundaries", "cliff", "confusion", "names", "notes", "itemBacks", "items", "itemsMore", "itemLabels",
		"transitions", "arrows", "transitionBoxes", "transitionLabels",
	}
	data := make([]any, 0, len(order))
	for _, set := range order {
		for _, r := range l.rows[set] {
			m, _ := r.(map[string]any)
			for _, k := range []string{"x", "y", "x2", "y2"} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round(v*100) / 100
				}
			}
		}
		data = append(data, map[string]any{"name": set, "values": orEmpty(l.rows[set])})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	// byDomain is the color of the domain of a row: the color that the
	// source gives to the domain, or the color of the scheme.
	byDomain := func() any {
		var rules []any
		for _, d := range cyDomains {
			if s := cfg.colors[d+"Bg"]; s != "" {
				rules = append(rules, map[string]any{"test": "datum.domain === " + quoteJS(d), "value": s})
			}
		}
		of := map[string]any{"scale": "color", "field": "domain"}
		if len(rules) == 0 {
			return of
		}
		return append(rules, of)
	}
	// domainColor is the color of a domain by name.
	domainColor := func(d string) any {
		if s := cfg.colors[d+"Bg"]; s != "" {
			return value(s)
		}
		return map[string]any{"scale": "color", "value": d}
	}
	// own sets a key of an encoding to a color of themeVariables.cynefin
	// when the source gives it.
	own := func(enc map[string]any, key, v string) map[string]any {
		if s := cfg.colors[v]; s != "" {
			enc[key] = value(s)
		}
		return enc
	}
	style := func(s string) []any { return []any{s} }
	var marks []any
	// mark adds a mark that draws a set of rows. name is the set, or the
	// set, a slash and the name of the mark, when two marks draw one set.
	mark := func(typ, name string, st []any, enc map[string]any) {
		set, _, _ := strings.Cut(name, "/")
		if len(l.rows[set]) == 0 {
			return
		}
		if _, n, ok := strings.Cut(name, "/"); ok {
			name = n
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": name, "from": map[string]any{"data": set},
			"style": st, "encode": map[string]any{"update": enc},
		})
	}
	box := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["x2"], enc["y2"] = field("x"), field("y"), field("x2"), field("y2")
		return enc
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = value(0), value(0), field("path")
		return enc
	}
	label := func(f text.Font, enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["text"] = field("x"), field("y"), field("text")
		enc["align"], enc["baseline"] = value("center"), value("middle")
		enc["fontSize"], enc["fontWeight"] = value(f.Size), value(f.Weight)
		if f.Italic {
			enc["fontStyle"] = value("italic")
		}
		return enc
	}
	mark("rect", "domains", style(graph.StyleNode), box(map[string]any{
		"fill": byDomain(), "fillOpacity": value(0.08), "strokeOpacity": value(0),
	}))
	mark("path", "boundaries", style(graph.StyleEdge), path(own(map[string]any{
		"strokeWidth": value(1.25), "strokeOpacity": value(0.7), "strokeDash": value([]any{6, 4}),
	}, "stroke", "boundaryColor")))
	mark("path", "cliff", style(graph.StyleEdge), path(own(map[string]any{
		"stroke": domainColor(cyChaotic), "strokeWidth": value(2 * theme.EdgeWidth),
	}, "stroke", "cliffColor")))
	// The ellipse of confusion hides the boundaries under it in part, as
	// in Mermaid.
	mark("path", "confusion/confusionBack", style(graph.StyleEdgeLabelBox), path(map[string]any{"fillOpacity": value(0.6)}))
	mark("path", "confusion", style(graph.StyleNode), path(map[string]any{
		"fill": byDomain(), "stroke": byDomain(), "fillOpacity": value(0.1), "strokeDash": value([]any{5, 3}),
	}))
	mark("text", "names", style(graph.StyleNodeLabel), label(cyNameFont, own(map[string]any{}, "fill", "labelColor")))
	mark("text", "notes", style(graph.StyleEdgeLabel), label(cyNoteFont, own(map[string]any{}, "fill", "textColor")))
	mark("rect", "itemBacks", style(graph.StyleEdgeLabelBox), box(map[string]any{
		"fillOpacity": value(1), "cornerRadius": value(4),
	}))
	mark("rect", "items", style(graph.StyleNode), box(map[string]any{
		"fill": byDomain(), "stroke": byDomain(), "cornerRadius": value(4),
	}))
	mark("rect", "itemsMore", style(graph.StyleNode), box(map[string]any{
		"fill": byDomain(), "stroke": byDomain(), "fillOpacity": value(0.06), "cornerRadius": value(4),
		"strokeDash": value([]any{3, 2}),
	}))
	mark("text", "itemLabels", style(graph.StyleNodeLabel), label(cyLabelFont, own(map[string]any{}, "fill", "textColor")))
	mark("path", "transitions", style(graph.StyleEdge), path(own(map[string]any{}, "stroke", "arrowColor")))
	mark("symbol", "arrows", style(graph.StyleArrow), own(own(map[string]any{
		"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("glyph"), "size": value(graph.ArrowSymbolSize),
	}, "fill", "arrowColor"), "stroke", "arrowColor"))
	mark("rect", "transitionBoxes", style(graph.StyleEdgeLabelBox), box(map[string]any{}))
	mark("text", "transitionLabels", style(graph.StyleEdgeLabel), label(cyLabelFont, own(map[string]any{}, "fill", "textColor")))
	pad := 8.0
	if cfg.hasPadding {
		pad = cfg.padding
	}
	return map[string]any{
		"width":    math.Ceil(l.w),
		"height":   math.Ceil(l.h),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": cyScale, "range": "category"}},
		"marks":    marks,
	}
}

// cyHash returns the hash of a text that seeds the waves of the
// boundaries: the hash of the units of UTF-16 of the text, times 31 at
// each step, in 32 bits.
func cyHash(s string) int32 {
	var h int32
	for _, u := range utf16.Encode([]rune(s)) {
		h = h*31 + int32(u)
	}
	return h
}

// cyRandom returns a number in [0, 1) for a seed, the same for the same
// seed: the generator mulberry32, in 32 bits, as Mermaid seeds its
// boundaries.
func cyRandom(seed float64) float64 {
	t := toUint32(seed + 0x6d2b79f5)
	t = (t ^ t>>15) * (t | 1)
	t ^= t + (t^t>>7)*(t|61)
	return float64(t^t>>14) / 4294967296
}

// toUint32 returns a number as 32 bits, as JavaScript turns a number into
// a whole number of 32 bits: the part after the point goes, and the rest
// wraps.
func toUint32(f float64) uint32 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	m := math.Mod(math.Trunc(f), 4294967296)
	if m < 0 {
		m += 4294967296
	}
	return uint32(m)
}

// cyFold returns the path of the boundary from the top to the bottom of
// the diagram: 7 cubic curves through points that wave left and right of
// the center by the amplitude at most.
func cyFold(w, h, seed, a float64) string {
	return cyWave(seed, a, 17, 31, 7, func(i, jitter float64) (float64, float64) {
		return w/2 + jitter, i * h / 7
	}, true)
}

// cyCross returns the path of the boundary from the left to the right of
// the diagram, as cyFold does from the top to the bottom.
func cyCross(w, h, seed, a float64) string {
	return cyWave(seed, a, 23, 37, 11, func(i, jitter float64) (float64, float64) {
		return i * w / 7, h/2 + jitter
	}, false)
}

// cyWave returns the path of a wavy boundary of 7 cubic curves. Each point
// moves across the boundary by a random number of the seed, the point i
// by the number of seed + i*step, and the control points of the curve i
// bend it by 1.5 times the amplitude, times the number of seed + i*bend +
// off, to one side and then the other.
func cyWave(seed, a, step, bend, off float64, at func(i, jitter float64) (float64, float64), vertical bool) string {
	const n = 7
	var xs, ys [n + 1]float64
	for i := range n + 1 {
		xs[i], ys[i] = at(float64(i), cyRandom(seed+float64(i)*step)*a*2-a)
	}
	var b strings.Builder
	b.WriteString("M" + num(xs[0]) + "," + num(ys[0]))
	for i := range n {
		dir := 1.0
		if i%2 == 1 {
			dir = -1
		}
		o := a * 1.5 * dir * cyRandom(seed+float64(i)*bend+off)
		x0, y0, x1, y1 := xs[i], ys[i], xs[i+1], ys[i+1]
		var c1x, c1y, c2x, c2y float64
		if vertical {
			c1x, c1y, c2x, c2y = x0+o, (y0+y1)/2, x1-o, (y0+y1)/2
		} else {
			c1x, c1y, c2x, c2y = (x0+x1)/2, y0+o, (x0+x1)/2, y1-o
		}
		b.WriteString("C" + num(c1x) + "," + num(c1y) + " " + num(c2x) + "," + num(c2y) + " " + num(x1) + "," + num(y1))
	}
	return b.String()
}

// cyCliff returns the path of the boundary between clear and chaotic, from
// the center of the diagram down to its bottom: a steep S of two cubic
// curves that swing by 0.03 of the width.
func cyCliff(w, h float64) string {
	cx, top, span, a := w/2, h/2, h/2, w*0.03
	pt := func(x, f float64) string { return num(x) + "," + num(top+span*f) }
	return "M" + pt(cx, 0) +
		"C" + pt(cx+a, 0.2) + " " + pt(cx-a*1.5, 0.55) + " " + pt(cx+a*0.5, 0.75) +
		"C" + pt(cx-a, 0.85) + " " + pt(cx+a*0.3, 0.95) + " " + pt(cx, 1)
}

// cyEllipse returns the path of an ellipse around a center.
func cyEllipse(cx, cy, rx, ry float64) string {
	return "M" + num(cx-rx) + "," + num(cy) +
		"A" + num(rx) + "," + num(ry) + " 0 1,1 " + num(cx+rx) + "," + num(cy) +
		"A" + num(rx) + "," + num(ry) + " 0 1,1 " + num(cx-rx) + "," + num(cy) + "Z"
}
