package mermaid

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/text"
)

// The sizes of a venn diagram in Mermaid 12.3, in its pixels. Mermaid lays
// the circles out in a box of venn.width by venn.height, 800 by 450 by
// default, less the title, with a padding of 15 around them. It places the
// texts of the areas in a layout of the same circles with venn.padding
// around them, 8 by default. Its sizes of type are a share of the width:
// at a width of 800, a label of a set or a union is 24 pixels, a text 20
// and the title 16.
const (
	vennDefaultWidth = 800.0
	vennPadding      = 15.0
	// vennReference is the width at which the sizes of type of Mermaid
	// are 48, 40 and 32 pixels.
	vennReference = 1600.0
)

// The sizes of the look of zuzu in a venn diagram: the opacity of the
// tint of a circle and the width of its outline.
const (
	vennTint    = 0.14
	vennOutline = 1.5
)

// vennLayout lays out a venn diagram and writes its rows.
type vennLayout struct {
	v *venn
	m *text.Estimate
	// k turns the pixels of Mermaid into the pixels of zuzu.
	k    float64
	keys []string

	rows                   map[string][]any
	minX, minY, maxX, maxY float64
}

// The fonts of a venn diagram: the label of a set or a union is bold, and
// a text is not. zuzu draws a text at 11 pixels, where Mermaid draws it at
// 20, so the label of a set, 24 pixels in Mermaid, is 13.2.
var (
	vennLabelFont = text.Font{Family: text.Family, Size: 13.2, Weight: boldWeight}
	vennTextFont  = text.Font{Family: text.Family, Size: 11, Weight: 400}
)

// vennRenderAreas returns the areas of a diagram that Mermaid lays out:
// the sets and the unions, and an area for each pair of the sets of a
// union of three sets or more that the source does not give. The size of
// such a pair is a quarter of the smaller of its two sets.
func vennRenderAreas(areas []*vennArea) []*vennArea {
	out := slices.Clone(areas)
	have := map[string]bool{}
	sizes := map[string]float64{}
	for _, a := range areas {
		have[vennKey(a.sets, "|")] = true
		if len(a.sets) == 1 {
			sizes[a.sets[0]] = a.size
		}
	}
	for _, a := range areas {
		if len(a.sets) < 3 {
			continue
		}
		for i := range a.sets {
			for j := i + 1; j < len(a.sets); j++ {
				pair := []string{a.sets[i], a.sets[j]}
				key := vennKey(pair, "|")
				if have[key] {
					continue
				}
				have[key] = true
				size := 2.5
				sa, okA := sizes[pair[0]]
				sb, okB := sizes[pair[1]]
				if okA && okB {
					size = math.Min(sa, sb) / 4
				}
				out = append(out, &vennArea{sets: pair, size: size})
			}
		}
	}
	return out
}

// vennPlaced is a venn diagram laid out in the pixels of Mermaid: the
// areas that it draws, the circles that it draws and their labels, and the
// circles that place the texts.
type vennPlaced struct {
	data          []*vennArea
	drawn, placed *vennSolution
	labels, texts map[string]vennCenter
}

// vennPlace lays out a venn diagram as Mermaid 12.3 does, in its pixels.
// It returns nil when no set has a size.
func vennPlace(v *venn, titled bool) *vennPlaced {
	render := vennRenderAreas(v.areas)
	// Mermaid draws nothing for a set of size 0, and for the unions
	// with it.
	zero := map[string]bool{}
	for _, a := range render {
		if len(a.sets) == 1 && a.size == 0 {
			zero[a.sets[0]] = true
		}
	}
	p := &vennPlaced{}
	var sizes []vennSize
	for _, a := range render {
		if !slices.ContainsFunc(a.sets, func(s string) bool { return zero[s] }) {
			p.data = append(p.data, a)
			sizes = append(sizes, vennSize{sets: a.sets, size: a.size, weight: 1})
		}
	}
	if len(p.data) == 0 {
		return nil
	}
	height := v.height
	if titled {
		height -= 48 * v.width / vennReference
	}
	sol := vennArrange(vennSolve(sizes))
	p.drawn = vennFit(sol, v.width, height, vennPadding)
	p.labels = vennTextCenters(p.drawn, sizes)
	p.placed = vennFit(sol, v.width, height, v.padding)
	p.texts = vennTextCenters(p.placed, sizes)
	return p
}

// newVennLayout lays out a venn diagram as Mermaid 12.3 does, and returns
// nil when no set has a size.
func newVennLayout(v *venn, titled bool) (*vennLayout, error) {
	p := vennPlace(v, titled)
	if p == nil {
		return nil, nil
	}
	l := &vennLayout{
		v: v, m: text.NewEstimate(), rows: map[string][]any{},
		k:    scaleWrap("venn-beta", "", vennDefaultWidth) / v.width,
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	if err := l.circles(p.data, p.drawn); err != nil {
		return nil, err
	}
	if err := l.labels(p.data, p.labels); err != nil {
		return nil, err
	}
	if err := l.texts(p.data, p.placed, p.texts, v.width/vennReference); err != nil {
		return nil, err
	}
	return l, nil
}

// color returns a color of a style, and reports a value that is not a
// color of CSS.
func (l *vennLayout) color(target, prop string) (string, bool, error) {
	s, ok := l.v.styles[target][prop]
	if !ok {
		return "", false, nil
	}
	if _, ok := color.Parse(s); !ok {
		l.v.c.line = l.v.styleLines[target]
		return "", false, l.v.c.add("the %s %q in the style of %s, which is not a color", prop, s, strings.ReplaceAll(target, "|", ","))
	}
	return s, true, nil
}

// number returns a number of a style, such as 2 or 2px, and false for a
// value that is not a number of 0 or more. Mermaid writes 2px as "2 px",
// which is not a value of CSS.
func (l *vennLayout) number(target, prop string) (float64, bool, bool) {
	s, ok := l.v.styles[target][prop]
	if !ok {
		return 0, false, false
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "px"), 64)
	if err != nil || n < 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, true, false
	}
	return n, true, true
}

// circles writes the circle of each set, in the order of the source: its
// tint and its outline. Each set takes the next color of the scheme, and
// a style can give it a fill, an outline, an opacity of the fill and a
// width of the outline. A union with a fill in a style draws its overlap
// in that color, opaque, as in Mermaid.
func (l *vennLayout) circles(data []*vennArea, s *vennSolution) error {
	n := 0
	for _, a := range data {
		key := vennKey(a.sets, "|")
		if len(a.sets) > 1 {
			fill, ok, err := l.color(key, "fill")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			cs := make([]vennCircle, len(a.sets))
			for i, id := range a.sets {
				cs[i] = l.zuzu(*s.circles[id])
			}
			_, arcs := vennIntersection(cs)
			if len(arcs) == 0 {
				continue
			}
			path, x, y := vennArcPath(cs, arcs)
			l.add("overlaps", map[string]any{"x": x, "y": y, "path": path, "color": fill}, x, y, x, y)
			continue
		}
		c := l.zuzu(*s.circles[a.sets[0]])
		k := strconv.Itoa(n)
		n++
		l.keys = append(l.keys, k)
		path := circlePath(0, 0, c.r)
		tint := map[string]any{"key": k, "x": c.x, "y": c.y, "path": path, "fillOpacity": vennTint}
		line := map[string]any{"key": k, "x": c.x, "y": c.y, "path": path, "strokeWidth": vennOutline}
		fill, ok, err := l.color(key, "fill")
		if err != nil {
			return err
		}
		if ok {
			tint["color"], line["stroke"] = fill, fill
		}
		if stroke, ok, err := l.color(key, "stroke"); err != nil {
			return err
		} else if ok {
			line["stroke"] = stroke
		}
		if o, set, ok := l.number(key, "fill-opacity"); set {
			// A value that is not a number leaves the fill opaque in
			// Mermaid.
			tint["fillOpacity"] = 1.0
			if ok {
				tint["fillOpacity"] = math.Min(o, 1)
			}
		}
		if w, set, ok := l.number(key, "stroke-width"); set {
			// A value that is not a number leaves the outline 1 pixel
			// wide in Mermaid.
			line["strokeWidth"] = l.k
			if ok {
				line["strokeWidth"] = w * l.k
			}
		}
		l.add("circles", tint, c.x-c.r, c.y-c.r, c.x+c.r, c.y+c.r)
		l.add("outlines", line, c.x-c.r, c.y-c.r, c.x+c.r, c.y+c.r)
	}
	return nil
}

// zuzu returns a circle in the pixels of zuzu.
func (l *vennLayout) zuzu(c vennCircle) vennCircle {
	c.x, c.y, c.r = c.x*l.k, c.y*l.k, c.r*l.k
	return c
}

// vennArcPath returns the path of an overlap from the arcs of its border,
// from the first point of its border, and that point. A path starts at the
// point of its item, so that its item adds no point outside it to the
// bounds.
func vennArcPath(cs []vennCircle, arcs []vennArc) (string, float64, float64) {
	if len(arcs) == 1 {
		c := cs[arcs[0].circle]
		return circlePath(0, 0, c.r), c.x, c.y
	}
	ox, oy := arcs[0].p2.x, arcs[0].p2.y
	var b strings.Builder
	b.WriteString("M0,0")
	for _, a := range arcs {
		r := gitNum(cs[a.circle].r)
		large := "0"
		if a.large {
			large = "1"
		}
		b.WriteString("A" + r + "," + r + " 0 " + large + ",1 " + gitNum(a.p1.x-ox) + "," + gitNum(a.p1.y-oy))
	}
	b.WriteString("Z")
	return b.String(), ox, oy
}

// labels writes the label of each set and of each union that has one, at
// the point of its area that is farthest from the borders, as Mermaid
// places it. A set with no label shows its id. Mermaid draws these
// labels on one line.
func (l *vennLayout) labels(data []*vennArea, centers map[string]vennCenter) error {
	names := map[string]string{}
	for _, a := range data {
		if a.label != "" {
			names[vennKey(a.sets, ",")] = a.label
		}
	}
	n := 0
	for _, a := range data {
		single := len(a.sets) == 1
		key := strconv.Itoa(n)
		if single {
			n++
		}
		name, ok := names[vennKey(a.sets, ",")]
		if !ok && single {
			name = a.sets[0]
		}
		c, has := centers[vennKey(a.sets, ",")]
		if name == "" || !has || c.disjoint {
			continue
		}
		x, y := math.Floor(c.x)*l.k, math.Floor(c.y)*l.k
		row := map[string]any{"x": x, "y": y, "text": name}
		target := vennKey(a.sets, "|")
		col, colored, err := l.color(target, "color")
		if err != nil {
			return err
		}
		set := "overlapLabels"
		if single {
			set = "setLabels"
			row["key"] = key
			if fill, ok, err := l.color(target, "fill"); err != nil {
				return err
			} else if ok && !colored {
				col, colored = fill, true
			}
		}
		if colored {
			row["color"] = col
		}
		w := l.m.Width(vennLabelFont, name)
		h := lh(vennLabelFont)
		l.add(set, row, x-w/2, y-h/2, x+w/2, y+h/2)
	}
	return nil
}

// texts writes the texts of each area in a grid in its area, as Mermaid
// places them: the grid is a square around the point of the label, as
// wide as the circle that fits in the area, under the label. A text wraps
// at the width of its cell.
func (l *vennLayout) texts(data []*vennArea, s *vennSolution, centers map[string]vennCenter, scale float64) error {
	byArea := map[string]*vennArea{}
	for _, a := range data {
		byArea[vennKey(a.sets, "|")] = a
	}
	var order []string
	groups := map[string][]*vennText{}
	for _, t := range l.v.texts {
		key := vennKey(t.sets, "|")
		if groups[key] == nil {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	for _, key := range order {
		a, ok := byArea[key]
		if !ok {
			// Mermaid draws no text of an area that it does not draw.
			continue
		}
		c, ok := centers[vennKey(a.sets, ",")]
		if !ok || c.disjoint {
			// Mermaid draws the texts of an empty area far above the
			// diagram, where they do not show.
			continue
		}
		nodes := groups[key]
		x0, y0, cw, ch, cols := vennGrid(a, len(nodes), c, s, scale)
		label := func(t *vennText) string {
			if t.hasLabel {
				return t.label
			}
			return t.id
		}
		// A word wider than its cell does not wrap, and Mermaid then draws it
		// over the text beside it. zuzu stacks the texts of such an area in
		// one column, as wide as the grid: the first text stays where the
		// first row of the grid is, under the label, and each next text goes
		// one line under it, so that no two texts overlap.
		if cols > 1 {
			for _, t := range nodes {
				if _, lw := plainLines(label(t), vennTextFont.Size, vennTextFont.Weight, cw*0.9*l.k, l.m); lw > cw*0.9*l.k {
					step := lh(vennTextFont) * 1.3 / l.k
					y0 += ch/2 - step/2
					cw, ch, cols = cw*float64(cols), step, 1
					break
				}
			}
		}
		for i, t := range nodes {
			x := (x0 + cw*(float64(i%cols)+0.5)) * l.k
			y := (y0 + ch*(float64(i/cols)+0.5)) * l.k
			lines, lw := plainLines(label(t), vennTextFont.Size, vennTextFont.Weight, cw*0.9*l.k, l.m)
			row := map[string]any{"x": x, "y": y, "lines": anyStrings(lines)}
			if col, ok, err := l.color(t.id, "color"); err != nil {
				return err
			} else if ok {
				row["color"] = col
			}
			h := float64(len(lines)) * lh(vennTextFont)
			l.add("texts", row, x-lw/2, y-h/2, x+lw/2, y+h/2)
		}
	}
	return nil
}

// vennGrid returns the grid of the texts of an area in the pixels of
// Mermaid: its top left corner, the size of a cell, and the count of its
// columns. The grid is a square around the point of the label, as wide as
// the circle that fits in the area, and it starts under the label.
func vennGrid(a *vennArea, n int, c vennCenter, s *vennSolution, scale float64) (x0, y0, cw, ch float64, cols int) {
	inner, least := math.Inf(1), math.Inf(1)
	for _, id := range a.sets {
		ci := s.circles[id]
		least = math.Min(least, ci.r)
		inner = math.Min(inner, ci.r-vennDist(c.x, c.y, ci.x, ci.y))
	}
	inner = math.Max(0, inner)
	if inner == 0 {
		inner = least * 0.6
	}
	w := math.Max(80*scale, inner*2*0.95)
	h := math.Max(60*scale, inner*2*0.95)
	offset := 0.0
	if a.label != "" {
		offset = math.Min(32*scale, inner*0.25)
	}
	if n <= 2 {
		offset += 30 * scale
	}
	cols = max(1, int(math.Ceil(math.Sqrt(float64(n)))))
	rows := max(1, (n+cols-1)/cols)
	return c.x - w/2, c.y - h/2 + offset, w / float64(cols), h / float64(rows), cols
}

// add appends a row to a set and grows the bounds by a box.
func (l *vennLayout) add(set string, row map[string]any, x0, y0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.minX, l.maxX = math.Min(l.minX, x0), math.Max(l.maxX, x1)
	l.minY, l.maxY = math.Min(l.minY, y0), math.Max(l.maxY, y1)
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31).
// Each set takes the next color of the scheme. The rows move so that the
// diagram starts at 0.
func (l *vennLayout) spec() map[string]any {
	dx, dy := -l.minX, -l.minY
	var data []any
	for _, set := range slices.Sorted(maps.Keys(l.rows)) {
		rows := l.rows[set]
		for _, r := range rows {
			m, _ := r.(map[string]any)
			x, _ := m["x"].(float64)
			y, _ := m["y"].(float64)
			m["x"], m["y"] = math.Round((x+dx)*100)/100, math.Round((y+dy)*100)/100
		}
		data = append(data, map[string]any{"name": set, "values": rows})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	custom := func(f string, fallback bool) []any {
		rules := []any{map[string]any{"test": "datum." + f, "field": f}}
		if fallback {
			rules = append(rules, map[string]any{"scale": "color", "field": "key"})
		}
		return rules
	}
	var marks []any
	mark := func(typ, set string, style []any, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		enc["x"], enc["y"] = field("x"), field("y")
		marks = append(marks, map[string]any{
			"type": typ, "name": set, "from": map[string]any{"data": set},
			"style": style, "encode": map[string]any{"update": enc},
		})
	}
	node := []any{graph.StyleNode}
	label := []any{graph.StyleNodeLabel, "zuzu.halo"}
	mark("path", "circles", node, map[string]any{
		"path": field("path"), "fill": custom("color", true), "fillOpacity": field("fillOpacity"), "strokeOpacity": value(0),
	})
	mark("path", "overlaps", node, map[string]any{
		"path": field("path"), "fill": field("color"), "fillOpacity": value(1), "strokeOpacity": value(0),
	})
	mark("path", "outlines", node, map[string]any{
		"path": field("path"), "stroke": custom("stroke", true), "strokeWidth": field("strokeWidth"), "fillOpacity": value(0),
	})
	text := func(f text.Font, fill []any, txt string) map[string]any {
		return map[string]any{
			"text": field(txt), "fill": fill, "align": value("center"), "baseline": value("middle"),
			"fontSize": value(f.Size), "fontWeight": value(f.Weight), "lineHeight": value(lh(f)),
		}
	}
	mark("text", "setLabels", label, text(vennLabelFont, custom("color", true), "text"))
	mark("text", "overlapLabels", label, text(vennLabelFont, custom("color", false), "text"))
	mark("text", "texts", label, text(vennTextFont, custom("color", false), "lines"))
	domain := make([]any, len(l.keys))
	for i, k := range l.keys {
		domain[i] = k
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
