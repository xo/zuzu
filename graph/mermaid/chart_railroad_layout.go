package mermaid

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// The sizes of a rule of a railroad diagram in Mermaid 12.3, in the pixels
// of Mermaid, which scaleWrap turns into the pixels of zuzu at 11/16,
// because Mermaid draws the text of a railroad diagram at 16 pixels: the
// space between the name of a rule and its start marker, and between the
// marker and the definition, the length of the track after the
// definition, the least height of the track under the top of a rule, the
// least height of a rule, and the radius of the corners of a terminal.
const (
	rrNameGap       = 20.0
	rrEndGap        = 10.0
	rrTrackMin      = 20.0
	rrRuleMin       = 40.0
	rrTerminalRound = 10.0
)

// The fonts of a railroad diagram: the name of a rule is bold.
var (
	rrFont     = text.Font{Family: text.Family, Size: 11, Weight: 400}
	rrNameFont = text.Font{Family: text.Family, Size: 11, Weight: boldWeight}
)

// The keys of the colors of the boxes of a railroad diagram, in the order
// of the category scheme. Mermaid colors a box by its kind: a nonterminal
// in the primary color of the theme, a terminal in the secondary, and a
// special in the tertiary.
var rrKeys = []any{"nonterminal", "terminal", "special"}

// rrLayout lays out a railroad diagram and writes its rows.
type rrLayout struct {
	r *railroad
	m *text.Estimate
	// scale turns a size of Mermaid into a size of zuzu.
	scale float64
	// pad, vsep, hsep, arc and marker are the sizes of the config, in the
	// pixels of zuzu.
	pad, vsep, hsep, arc, marker float64
	// n numbers the boxes in the order of the source.
	n int

	rows                   map[string][]any
	minX, minY, maxX, maxY float64
}

// newRailroadLayout lays out the rules of a railroad diagram, one under
// another, as Mermaid 12.3 does.
func newRailroadLayout(r *railroad) *rrLayout {
	s := scaleWrap(r.c.d.Kind, "", 1)
	l := &rrLayout{
		r: r, m: text.NewEstimate(), scale: s,
		pad: r.sizes.padding * s, vsep: r.sizes.vsep * s, hsep: r.sizes.hsep * s, arc: r.sizes.arc * s, marker: r.sizes.marker * s,
		rows: map[string][]any{},
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	y := 0.0
	for i, rule := range r.rules {
		y += l.rule(i, rule, y) + l.vsep
	}
	return l
}

// rrText returns the text that a box draws: its text with each run of
// spaces as one space, as SVG draws it, and a special between question
// marks, as Mermaid draws it.
func rrText(n *rrNode) string {
	s := n.text
	if n.kind == rrSpecial {
		s = "? " + s + " ?"
	}
	return strings.Join(strings.Fields(decodeEntities(s)), " ")
}

// measure sets the size of a node and of the nodes in it, as Mermaid 12.3
// sizes them. A box is its text and the padding on each side. A sequence
// puts its nodes in a row on one track. A choice stacks its alternatives,
// with room for the curves at each side. An optional has a bypass over its
// node, and a repeat a loop under it, and a bypass over it when its least
// count is 0.
func (l *rrLayout) measure(n *rrNode) {
	r2, r4 := 2*l.arc, 4*l.arc
	switch n.kind {
	case rrTerminal, rrNonterminal, rrSpecial:
		s := rrText(n)
		w, h := 0.0, 0.0
		if s != "" {
			w, h = l.m.Width(rrFont, s), lh(rrFont)
		}
		n.w, n.h = w+2*l.pad, h+2*l.pad
		n.up = n.h / 2
	case rrSequence:
		up, down := 0.0, 0.0
		n.w = l.hsep * float64(len(n.items)-1)
		for _, it := range n.items {
			l.measure(it)
			n.w += it.w
			up, down = math.Max(up, it.up), math.Max(down, it.h-it.up)
		}
		n.h, n.up = up+down, up
	case rrChoice:
		widest := 0.0
		n.h = l.vsep * float64(len(n.items)-1)
		for _, it := range n.items {
			l.measure(it)
			widest = math.Max(widest, it.w)
			n.h += it.h
		}
		n.w, n.up = widest+r4, n.h/2
	case rrOptional, rrRepeat:
		it := n.items[0]
		l.measure(it)
		n.w = it.w + r4
		top := r2
		if n.kind == rrRepeat && n.min > 0 {
			top = 0
		}
		n.h, n.up = it.h+top, top+it.up
		if n.kind == rrRepeat {
			n.h += r2
		}
	}
}

// rule draws a rule with its top at y: its name, the start marker, the
// definition and the end marker, on one track. It returns the height of
// the rule.
func (l *rrLayout) rule(i int, rule rrRule, y float64) float64 {
	def := rule.def
	l.measure(def)
	name := rule.name + " ="
	start := l.m.Width(rrNameFont, name) + rrNameGap*l.scale
	x := start + rrNameGap*l.scale
	track := math.Max(rrTrackMin*l.scale, def.up)
	top := track - def.up
	l.draw(i, def, x, y+top)
	ty := y + track
	l.rows["names"] = append(l.rows["names"], map[string]any{"id": "r" + strconv.Itoa(i), "x": 0.0, "y": ty, "text": name})
	l.grow(0, ty-lh(rrNameFont)/2, start-rrNameGap*l.scale, ty+lh(rrNameFont)/2)
	end := x + def.w + rrEndGap*l.scale
	for _, cx := range []float64{start, end} {
		row := map[string]any{"id": "r" + strconv.Itoa(i), "x": cx, "y": ty, "path": circlePath(0, 0, l.marker)}
		l.add("markers", row, cx-l.marker, ty-l.marker, cx+l.marker, ty+l.marker)
	}
	l.line(start+l.marker, ty, x, ty)
	l.line(x+def.w, ty, end-l.marker, ty)
	return math.Max(rrRuleMin*l.scale, top+def.h+2*l.pad)
}

// draw draws a node with its top left corner at x and y.
func (l *rrLayout) draw(rule int, n *rrNode, x, y float64) {
	r := l.arc
	switch n.kind {
	case rrTerminal, rrNonterminal, rrSpecial:
		l.box(rule, n, x, y)
	case rrSequence:
		ty := y + n.up
		for k, it := range n.items {
			l.draw(rule, it, x, ty-it.up)
			x += it.w
			if k < len(n.items)-1 {
				l.line(x, ty, x+l.hsep, ty)
			}
			x += l.hsep
		}
	case rrChoice:
		widest := n.w - 4*r
		cy, right := y+n.up, x+n.w
		top := y
		for _, it := range n.items {
			ex := x + 2*r + (widest-it.w)/2
			ey := top + it.up
			l.draw(rule, it, ex, top)
			var in, out rrPath
			if math.Abs(ey-cy) < 1e-9 {
				in.move(x, cy).line(ex, ey)
				out.move(ex+it.w, ey).line(right, cy)
			} else {
				d := r
				if ey < cy {
					d = -r
				}
				below := ey > cy
				in.move(x, cy).arc(r, below, x+r, cy+d).line(x+r, ey-d).arc(r, !below, x+2*r, ey).line(ex, ey)
				out.move(ex+it.w, ey).line(right-2*r, ey).arc(r, !below, right-r, ey-d).line(right-r, cy+d).arc(r, below, right, cy)
			}
			l.track(in, x, math.Min(cy, ey), ex, math.Max(cy, ey))
			l.track(out, ex+it.w, math.Min(cy, ey), right, math.Max(cy, ey))
			top += it.h + l.vsep
		}
	case rrOptional, rrRepeat:
		it := n.items[0]
		ex, ey := x+2*r, y+n.up-it.up
		cy, right := y+n.up, x+n.w
		l.draw(rule, it, ex, ey)
		l.line(x, cy, ex, cy)
		l.line(ex+it.w, cy, right, cy)
		if n.kind == rrRepeat {
			// The loop runs back under the node.
			end := ex + it.w
			low := ey + it.h + r
			var p rrPath
			p.move(end, cy).arc(r, true, end+r, cy+r).line(end+r, low).arc(r, true, end, low+r).
				line(ex, low+r).arc(r, true, x+r, low).line(x+r, cy+r).arc(r, true, ex, cy)
			l.track(p, x+r, cy, end+r, low+r)
		}
		if n.kind == rrOptional || n.min == 0 {
			// The bypass runs over the node.
			var p rrPath
			p.move(x, cy).arc(r, false, x+r, cy-r).line(x+r, y+r).arc(r, true, x+2*r, y).
				line(right-2*r, y).arc(r, true, right-r, y+r).line(right-r, cy-r).arc(r, false, right, cy)
			l.track(p, x, y, right, cy)
		}
	}
}

// box draws a terminal, a nonterminal or a special with its text in the
// middle.
func (l *rrLayout) box(rule int, n *rrNode, x, y float64) {
	kind := map[rrKind]string{rrTerminal: "terminal", rrNonterminal: "nonterminal", rrSpecial: "special"}[n.kind]
	id := "r" + strconv.Itoa(rule) + "b" + strconv.Itoa(l.n)
	l.add(kind+"s", map[string]any{"id": id, "key": kind, "n": l.n, "x": x, "y": y, "x2": x + n.w, "y2": y + n.h}, x, y, x+n.w, y+n.h)
	l.rows["labels"] = append(l.rows["labels"], map[string]any{
		"id": id, "key": kind, "n": l.n, "x": x + n.w/2, "y": y + n.h/2, "text": rrText(n),
	})
	l.n++
}

// line draws a straight piece of track.
func (l *rrLayout) line(x0, y0, x1, y1 float64) {
	var p rrPath
	p.move(x0, y0).line(x1, y1)
	l.track(p, x0, y0, x1, y1)
}

// track draws a piece of track, which runs inside a box.
func (l *rrLayout) track(p rrPath, x0, y0, x1, y1 float64) {
	l.add("tracks", map[string]any{"x": 0.0, "y": 0.0, "path": p.b.String()}, x0, y0, x1, y1)
}

// add appends a row to a set and grows the bounds by a box.
func (l *rrLayout) add(set string, row map[string]any, x0, y0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.grow(x0, y0, x1, y1)
}

// grow grows the bounds by a box.
func (l *rrLayout) grow(x0, y0, x1, y1 float64) {
	l.minX, l.maxX = math.Min(l.minX, math.Min(x0, x1)), math.Max(l.maxX, math.Max(x0, x1))
	l.minY, l.maxY = math.Min(l.minY, math.Min(y0, y1)), math.Max(l.maxY, math.Max(y0, y1))
}

// rrPath writes a path of moves, lines and quarter circles.
type rrPath struct {
	b strings.Builder
}

func (p *rrPath) move(x, y float64) *rrPath {
	p.b.WriteString("M" + gitNum(x) + "," + gitNum(y))
	return p
}

func (p *rrPath) line(x, y float64) *rrPath {
	p.b.WriteString("L" + gitNum(x) + "," + gitNum(y))
	return p
}

// arc draws an arc of a radius to a point, clockwise when sweep is true.
func (p *rrPath) arc(r float64, sweep bool, x, y float64) *rrPath {
	s := "0"
	if sweep {
		s = "1"
	}
	p.b.WriteString("A" + gitNum(r) + "," + gitNum(r) + " 0 0," + s + " " + gitNum(x) + "," + gitNum(y))
	return p
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31). The
// rows move so that the diagram starts at 0.
func (l *rrLayout) spec() map[string]any {
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
	mark := func(typ, set, style string, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": set, "from": map[string]any{"data": set},
			"style": []any{style}, "encode": map[string]any{"update": enc},
		})
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = field("x"), field("y"), field("path")
		return enc
	}
	rect := func(radius float64, enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["x2"], enc["y2"] = field("x"), field("y"), field("x2"), field("y2")
		enc["fill"], enc["stroke"], enc["cornerRadius"] = color, color, value(radius)
		return enc
	}
	label := func(f text.Font, enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["text"] = field("x"), field("y"), field("text")
		enc["baseline"], enc["fontSize"], enc["fontWeight"] = value("middle"), value(f.Size), value(f.Weight)
		return enc
	}
	mark("path", "tracks", graph.StyleEdge, path(map[string]any{}))
	mark("path", "markers", graph.StyleArrow, path(map[string]any{}))
	mark("rect", "nonterminals", graph.StyleNode, rect(2, map[string]any{}))
	mark("rect", "terminals", graph.StyleNode, rect(rrTerminalRound*l.scale, map[string]any{}))
	mark("rect", "specials", graph.StyleNode, rect(2, map[string]any{"strokeDash": value([]any{4, 3})}))
	mark("text", "labels", graph.StyleNodeLabel, label(rrFont, map[string]any{"align": value("center")}))
	mark("text", "names", graph.StyleNodeLabel, label(rrNameFont, map[string]any{"align": value("left")}))
	const pad = 8.0
	return map[string]any{
		"width":    math.Ceil(l.maxX - l.minX),
		"height":   math.Ceil(l.maxY - l.minY),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": rrKeys, "range": "category"}},
		"marks":    marks,
	}
}
