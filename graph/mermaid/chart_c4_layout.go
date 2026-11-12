package mermaid

import (
	"maps"
	"math"
	"strings"

	"github.com/xo/zuzu/graph"
)

// c4Text is a text of a C4 diagram in lines: its font, its size, and its
// offset y from the top of the box that holds it.
type c4Text struct {
	lines   []string
	size    float64
	weight  int
	italic  bool
	w, h, y float64
}

// The sizes of the texts of a C4 diagram: the sizes of Mermaid 12.3 times
// 11/14, the size of a label of zuzu over the size of a label of Mermaid,
// so that a line holds the words that it holds in Mermaid (D32). The name
// of an element is 14 pixels in Mermaid, its stereotype 0.75 of it and its
// description 0.82. The label of a boundary is 16, its type 14 and its
// description 12, and the text of a relationship 12.
const (
	c4Scale      = 11.0 / 14
	c4NameSize   = 14 * c4Scale
	c4TypeSize   = 14 * 0.75 * c4Scale
	c4DescrSize  = 14 * 0.82 * c4Scale
	c4BoundSize  = 16 * c4Scale
	c4BTypeSize  = 14 * c4Scale
	c4BDescrSize = 12 * c4Scale
	c4RelSize    = 12 * c4Scale
	// c4Screen is the width of the screen of mmdc, the limit of a row of
	// elements in Mermaid.
	c4Screen = 800
	// c4Gap is the space between the texts of an element.
	c4Gap = 3
)

// text measures a text of a diagram, wrapped at a width in the pixels of
// zuzu, or not wrapped for 0. <br> breaks a line.
func (p *c4Diagram) text(s string, size float64, weight int, limit float64) *c4Text {
	lines, w := plainLines(labelText(s), size, min(weight, boldWeight), limit, p.m)
	if len(lines) == 0 {
		lines = []string{""}
	}
	return &c4Text{lines: lines, size: size, weight: weight, w: w, h: float64(len(lines)) * size * 1.3}
}

// measure sets the texts and the size of an element, as the shapes of
// Mermaid 12.3 size a C4 element: the width of c4.width at least, and the
// texts with the padding of c4.c4ShapePadding around them.
func (p *c4Diagram) measure(e *c4Elem) {
	pad, width := p.pad, p.width
	wrapW := width - 2*pad
	switch e.shape {
	case "cylinder":
		wrapW = width - pad - 2*pad
	case "h-cyl":
		wrapW = width - pad/2 - 2*pad
	}
	limit := 0.0
	if p.c4wrap {
		limit = scaleWrap(max(wrapW, 32))
	}
	e.texts = nil
	if e.label != "" {
		e.texts = append(e.texts, p.text(e.label, c4NameSize, boldWeight, limit))
	}
	e.texts = append(e.texts, p.text(c4Stereotype(e), c4TypeSize, 400, limit))
	if d := e.fields["descr"]; d != "" {
		e.texts = append(e.texts, p.text(d, c4DescrSize, 400, limit))
	}
	e.tw, e.th = 0, 0
	for i, t := range e.texts {
		t.y = e.th
		e.tw = max(e.tw, t.w)
		e.th += t.h
		if i < len(e.texts)-1 {
			e.th += c4Gap
		}
	}
	bw, bh := e.tw, e.th
	switch e.shape {
	case "person":
		e.w = max(bw+2*pad, width, 100)
		hr := c4Head(e.w)
		e.h = bh + 2*pad + 2*hr - 0.27*hr
	case "cylinder":
		e.w = max(width-pad, bw) + pad
		ry := e.w / 2 / (2.5 + e.w/50)
		e.h = bh + pad + 3*ry
	case "h-cyl":
		h := bh + pad/2
		rx := h / 2 / (2.5 + h/50)
		e.w = max(width-pad/2, bw) + rx + pad/2 + 2*rx
		e.h = h
	case "subroutine":
		e.w = max(bw+16+pad, width)
		e.h = bh + pad
	default:
		e.w = max(bw+2*pad, width)
		e.h = bh + 2*pad
	}
}

// c4Head returns the radius of the head of a person of a width, as in
// Mermaid.
func c4Head(w float64) float64 {
	return math.Min(math.Max(w*0.23, 16), 56)
}

// c4Bounds is the box that a row of elements fills in Mermaid: the box of
// the elements so far, and the box of the row that the next element joins.
type c4Bounds struct {
	startx, stopx, starty, stopy     float64
	nstartx, nstopx, nstarty, nstopy float64
	// cnt is the number of elements in the row. A boundary keeps the
	// count of the boundary before it in its row, as in Mermaid.
	cnt   int
	limit float64
}

// set starts the box at a point.
func (b *c4Bounds) set(x, y float64) {
	b.startx, b.stopx, b.starty, b.stopy = x, x, y, y
	b.nstartx, b.nstopx, b.nstarty, b.nstopy = x, x, y, y
}

// insert places an element at the end of the row, or at the start of the
// next row when the row has c4ShapeInRow elements or the element passes
// the limit of the row, as Mermaid 12.3 does.
func (p *c4Diagram) insert(b *c4Bounds, e *c4Elem) {
	m := p.margin
	b.cnt++
	x0 := b.nstopx + 2*m
	if b.nstartx == b.nstopx {
		x0 = b.nstopx + m
	}
	x1 := x0 + e.w
	y0 := b.nstarty + 2*m
	y1 := y0 + e.h
	if x0 >= b.limit || x1 >= b.limit || b.cnt > p.shapeInRow {
		x0 = b.nstartx + m + p.lineX
		y0 = b.nstopy + 2*m
		x1 = x0 + e.w
		b.nstopx = x1
		b.nstarty = b.nstopy
		y1 = y0 + e.h
		b.nstopy = y1
		b.cnt = 1
	}
	e.x, e.y = x0, y0
	b.startx, b.starty = min(b.startx, x0), min(b.starty, y0)
	b.stopx, b.stopy = max(b.stopx, x1), max(b.stopy, y1)
	b.nstartx, b.nstarty = min(b.nstartx, x0), min(b.nstarty, y0)
	b.nstopx, b.nstopy = max(b.nstopx, x1), max(b.nstopy, y1)
}

// layout places the elements and the boundaries on the grid of Mermaid.
func (p *c4Diagram) layout() {
	for _, e := range p.elems {
		p.measure(e)
	}
	screen := &c4Bounds{limit: c4Screen}
	screen.set(p.marginX, p.marginY)
	p.inside(screen, p.bounds[:1])
}

// inside places a list of boundaries in the box of their parent: up to
// c4BoundaryInRow in a row, each with its elements in rows and then its
// own boundaries, as Mermaid 12.3 does.
func (p *c4Diagram) inside(parent *c4Bounds, list []*c4Boundary) {
	cur := &c4Bounds{limit: parent.limit / float64(min(p.boundaryInRow, len(list)))}
	for i, bd := range list {
		y := 0.0
		if bd.fields["sprite"] != "" {
			// Mermaid keeps the space of an image for a sprite, and draws
			// none.
			y = 48
		}
		limit := 0.0
		if p.wrap && p.c4wrap {
			limit = scaleWrap(cur.limit)
		}
		bd.texts = nil
		add := func(s string, size float64, weight int, gap float64) {
			t := p.text(s, size, weight, limit)
			t.y = y + gap
			y = t.y + t.h
			bd.texts = append(bd.texts, t)
		}
		add(bd.label, c4BoundSize, boldWeight, 8)
		if t := bd.fields["type"]; t != "" {
			add("["+t+"]", c4BTypeSize, 400, 5)
		}
		if d := bd.fields["descr"]; d != "" {
			add(d, c4BDescrSize, 400, 20)
		}
		if i%p.boundaryInRow == 0 {
			cur.set(parent.startx+p.marginX, parent.stopy+p.marginY+y)
		} else {
			x := cur.startx
			if cur.stopx != cur.startx {
				x = cur.stopx + p.marginX
			}
			cur.set(x, cur.starty)
		}
		n := 0
		for _, e := range p.elems {
			if e.parent == bd.alias {
				p.insert(cur, e)
				n++
			}
		}
		if n > 0 {
			cur.stopx += p.margin
			cur.stopy += p.margin
		}
		var kids []*c4Boundary
		for _, k := range p.bounds[1:] {
			if k.parent == bd.alias {
				kids = append(kids, k)
			}
		}
		if len(kids) > 0 {
			p.inside(cur, kids)
		}
		bd.x, bd.y, bd.w, bd.h = cur.startx, cur.starty, cur.stopx-cur.startx, cur.stopy-cur.starty
		parent.stopy = max(cur.stopy+p.margin, parent.stopy)
		parent.stopx = max(cur.stopx+p.margin, parent.stopx)
	}
}

// c4Box is the box of an end of a relationship.
type c4Box struct {
	x, y, w, h float64
}

// box returns the box of an element or of a boundary by its alias.
func (p *c4Diagram) box(alias string) c4Box {
	if e, ok := p.byAlias[alias]; ok {
		return c4Box{e.x, e.y, e.w, e.h}
	}
	b := p.boundBy[alias]
	return c4Box{b.x, b.y, b.w, b.h}
}

// clip returns the point where a line from the center of a box toward a
// point leaves the box, as the rect of Mermaid computes it.
func (b c4Box) clip(px, py float64) (float64, float64) {
	cx, cy := b.x+b.w/2, b.y+b.h/2
	dx, dy := px-cx, py-cy
	w, h := b.w/2, b.h/2
	if math.Abs(dy)*w > math.Abs(dx)*h {
		if dy < 0 {
			h = -h
		}
		sx := 0.0
		if dy != 0 {
			sx = h * dx / dy
		}
		return cx + sx, cy + h
	}
	if dx < 0 {
		w = -w
	}
	sy := 0.0
	if dx != 0 {
		sy = w * dy / dx
	}
	return cx + w, cy + sy
}

// c4PersonPath returns the outline of a person of a size around 0, 0: a
// head over a body with round corners, as in Mermaid.
func c4PersonPath(w, h float64) string {
	hr := c4Head(w)
	overlap := hr * 0.27
	body := h - 2*hr + overlap
	top := -h / 2
	bodyTop := top + 2*hr - overlap
	r := math.Min(w*0.177, body*0.45)
	x, bottom := w/2, h/2
	hc := top + hr
	d := bodyTop - hc
	ix := math.Sqrt(math.Max(hr*hr-d*d, 0))
	ix = math.Min(ix, x-r)
	var b strings.Builder
	b.WriteString("M" + num(-ix) + "," + num(bodyTop))
	b.WriteString("A" + num(hr) + "," + num(hr) + " 0 1,1 " + num(ix) + "," + num(bodyTop))
	b.WriteString("H" + num(x-r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(x) + "," + num(bodyTop+r))
	b.WriteString("V" + num(bottom-r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(x-r) + "," + num(bottom))
	b.WriteString("H" + num(-x+r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(-x) + "," + num(bottom-r))
	b.WriteString("V" + num(bodyTop+r) + "A" + num(r) + "," + num(r) + " 0 0,1 " + num(-x+r) + "," + num(bodyTop) + "Z")
	return b.String()
}

// c4Color returns the name of the color of an element: its kind, or
// external for an element outside the system.
func c4Color(kind string) string {
	if strings.HasPrefix(kind, "external_") {
		return "external"
	}
	return strings.TrimSuffix(strings.TrimSuffix(kind, "_db"), "_queue")
}

// c4Colors are the names of the colors of the elements, in the order of
// the category scheme.
var c4Colors = []any{"person", "system", "container", "component", "external"}

// paint returns the style of an element or a boundary from the colors that
// UpdateElementStyle and the named arguments give it.
func paint(fields map[string]string) map[string]any {
	st := map[string]string{}
	if v := fields["bgColor"]; v != "" {
		st["fill"], st["fillOpacity"] = v, "1"
	}
	if v := fields["borderColor"]; v != "" {
		st["stroke"] = v
	}
	if v := fields["fontColor"]; v != "" {
		st["color"] = v
	}
	row := map[string]any{}
	graph.StyleRow(row, st)
	return row
}

// spec writes the Vega spec of the diagram that layout laid out.
func (p *c4Diagram) spec() map[string]any {
	type relLine struct {
		r              *c4Rel
		x0, y0, x1, y1 float64
		cx, cy         float64
		label, techn   *c4Text
		lx, ly         float64
	}
	var lines []relLine
	for i, r := range p.rels {
		a, b := p.box(r.from), p.box(r.to)
		x0, y0 := a.clip(b.x+b.w/2, b.y+b.h/2)
		x1, y1 := b.clip(a.x+a.w/2, a.y+a.h/2)
		l := relLine{r: r, x0: x0, y0: y0, x1: x1, y1: y1}
		// Mermaid draws the first relationship straight, and each other as
		// a curve.
		l.cx, l.cy = (x0+x1)/2, (y0+y1)/2
		if i > 0 {
			l.cx, l.cy = x0+(x1-x0)/4, y0+(y1-y0)/2
		}
		text := r.label
		if p.c.d.Kind == c4Dynamic {
			text = itoa(i+1) + ": " + text
		}
		l.label = p.text(text, c4RelSize, 400, 0)
		if t := r.fields["techn"]; t != "" {
			l.techn = p.text("["+t+"]", c4RelSize, 400, 0)
			l.techn.italic = true
		}
		offX, _ := jsParseInt(r.fields["offsetX"])
		offY, _ := jsParseInt(r.fields["offsetY"])
		l.lx = math.Min(x0, x1) + math.Abs(x1-x0)/2 + float64(offX)
		l.ly = math.Min(y0, y1) + math.Abs(y1-y0)/2 + float64(offY)
		lines = append(lines, l)
	}
	// The diagram starts at 0, 0.
	minX, minY := math.Inf(1), math.Inf(1)
	for _, b := range p.bounds[1:] {
		minX, minY = min(minX, b.x), min(minY, b.y)
	}
	for _, e := range p.elems {
		minX, minY = min(minX, e.x), min(minY, e.y)
	}
	for _, l := range lines {
		minX, minY = min(minX, l.x0, l.x1, l.cx, l.lx), min(minY, l.y0, l.y1, l.cy, l.ly-l.label.h/2)
	}
	if math.IsInf(minX, 1) {
		minX, minY = 0, 0
	}
	links := false
	link := func(r map[string]any, fields map[string]string) {
		if l := fields["link"]; l != "" {
			r["href"], links = l, true
		}
	}
	var bounds, bTexts, elems, eTexts, rels, arrows, boxes, rTexts []any
	for _, b := range p.bounds[1:] {
		x, y := b.x-minX, b.y-minY
		r := map[string]any{"id": b.alias, "label": b.label, "parent": c4Parent(b.parent), "x": x, "y": y, "width": b.w, "height": b.h}
		maps.Copy(r, paint(b.fields))
		if !b.node {
			r["strokeDash"] = []any{5.0, 4.0}
		}
		link(r, b.fields)
		bounds = append(bounds, r)
		for _, t := range b.texts {
			for i, s := range t.lines {
				if s == "" {
					continue
				}
				row := map[string]any{
					"id": b.alias, "text": s, "x": x + 10, "y": y + t.y + (float64(i)+0.5)*t.size*1.3,
					"fontSize": t.size, "fontWeight": t.weight,
				}
				copyKeys(row, r, "textColor", "href")
				bTexts = append(bTexts, row)
			}
		}
	}
	for _, e := range p.elems {
		x, y := e.x-minX, e.y-minY
		cx, cy := x+e.w/2, y+e.h/2
		r := map[string]any{"id": e.alias, "kind": e.kind, "color": c4Color(e.kind), "shape": e.shape, "label": e.label,
			"parent": c4Parent(e.parent), "x": cx, "y": cy, "width": e.w, "height": e.h}
		maps.Copy(r, paint(e.fields))
		link(r, e.fields)
		// The texts sit in the middle of the shape, or of the body of a
		// person, or under the top of a cylinder.
		ty := cy
		switch e.shape {
		case "person":
			r["path"] = c4PersonPath(e.w, e.h)
			ty = y + e.h - (e.th+2*p.pad)/2
		case "cylinder":
			r["path"] = graph.ShapePath(e.shape, e.w, e.h)
			ty = cy + math.Min(e.h*0.12, 10)/2
		default:
			r["path"] = graph.ShapePath(e.shape, e.w, e.h)
		}
		elems = append(elems, r)
		top := ty - e.th/2
		for _, t := range e.texts {
			for i, s := range t.lines {
				row := map[string]any{
					"id": e.alias, "text": s, "x": cx, "y": top + t.y + (float64(i)+0.5)*t.size*1.3,
					"fontSize": t.size, "fontWeight": t.weight,
				}
				copyKeys(row, r, "textColor", "href")
				eTexts = append(eTexts, row)
			}
		}
	}
	for i, l := range lines {
		r := l.r
		x0, y0, x1, y1 := l.x0-minX, l.y0-minY, l.x1-minX, l.y1-minY
		cx, cy := l.cx-minX, l.cy-minY
		path := "M" + num(x0) + "," + num(y0) + "L" + num(x1) + "," + num(y1)
		if i > 0 {
			path = "M" + num(x0) + "," + num(y0) + "Q" + num(cx) + "," + num(cy) + " " + num(x1) + "," + num(y1)
		}
		row := map[string]any{"key": i, "id": r.from + "-" + r.to, "source": r.from, "target": r.to, "label": r.label, "path": path}
		color := r.fields["lineColor"]
		if color != "" {
			row["stroke"] = color
		}
		link(row, r.fields)
		rels = append(rels, row)
		glyph, _ := graph.ArrowPaths("normal")
		head := func(at string, x, y, fx, fy float64) {
			a := map[string]any{"edge": i, "end": at, "x": x, "y": y, "angle": math.Atan2(y-fy, x-fx) * 180 / math.Pi, "glyph": glyph}
			if color != "" {
				a["fill"], a["stroke"] = color, color
			}
			arrows = append(arrows, a)
		}
		if r.kind != c4RelBack {
			head(r.to, x1, y1, cx, cy)
		}
		if r.kind == c4BiRel || r.kind == c4RelBack {
			head(r.from, x0, y0, cx, cy)
		}
		// The label starts at the middle of the line, moved by offsetX and
		// offsetY, as in Mermaid, and the technology goes under it.
		lx, ly := l.lx-minX, l.ly-minY
		w := l.label.w
		h := l.label.h
		if l.techn != nil {
			w = max(w, l.techn.w)
			h += l.techn.h + 2
		}
		top := ly - l.label.h/2
		if w > 0 {
			boxes = append(boxes, map[string]any{"edge": i, "x": lx - 3, "y": top - 2, "width": w + 6, "height": h + 4})
		}
		texts := []*c4Text{l.label}
		if l.techn != nil {
			l.techn.y = l.label.h + 2
			texts = append(texts, l.techn)
		}
		for _, t := range texts {
			for k, s := range t.lines {
				if s == "" {
					continue
				}
				row := map[string]any{"edge": i, "text": s, "x": lx + w/2, "y": top + t.y + (float64(k)+0.5)*t.size*1.3, "fontSize": t.size}
				if t.italic {
					row["fontStyle"] = "italic"
				}
				if c := r.fields["textColor"]; c != "" {
					row["textColor"] = c
				}
				copyKeys(row, map[string]any{"href": r.fields["link"]}, "href")
				if row["href"] == "" {
					delete(row, "href")
				}
				rTexts = append(rTexts, row)
			}
		}
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	data := []any{
		map[string]any{"name": "boundaries", "values": orEmpty(bounds)},
		map[string]any{"name": "boundaryTexts", "values": orEmpty(bTexts)},
		map[string]any{"name": "elements", "values": orEmpty(elems)},
		map[string]any{"name": "elementTexts", "values": orEmpty(eTexts)},
		map[string]any{"name": "rels", "values": orEmpty(rels)},
		map[string]any{"name": "arrows", "values": orEmpty(arrows)},
		map[string]any{"name": "relBoxes", "values": orEmpty(boxes)},
		map[string]any{"name": "relTexts", "values": orEmpty(rTexts)},
	}
	textRename := map[string]string{"textColor": "fill"}
	textKeys := []string{"textColor", "fontStyle"}
	var marks []any
	marks = append(marks, sigMarks(&data, "boundaries", bounds, graph.PaintKeys(), nil, func() map[string]any {
		return map[string]any{"type": "rect", "style": []any{graph.StyleCluster}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "boundaryTexts", bTexts, textKeys, textRename, func() map[string]any {
		return map[string]any{"type": "text", "style": []any{graph.StyleClusterTitle}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"), "fontWeight": field("fontWeight"),
			"align": value("left"), "baseline": value("middle"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "elements", elems, graph.PaintKeys(), nil, func() map[string]any {
		color := map[string]any{"scale": "color", "field": "color"}
		return map[string]any{"type": "path", "style": []any{graph.StyleNode}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"), "fill": color, "stroke": color,
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "elementTexts", eTexts, textKeys, textRename, func() map[string]any {
		return map[string]any{"type": "text", "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"), "fontWeight": field("fontWeight"),
			"align": value("center"), "baseline": value("middle"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "rels", rels, []string{"stroke"}, nil, func() map[string]any {
		return map[string]any{"type": "path", "style": []any{graph.StyleEdge}, "encode": map[string]any{"update": map[string]any{
			"x": value(0), "y": value(0), "path": field("path"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "arrows", arrows, []string{"fill", "stroke"}, nil, func() map[string]any {
		return map[string]any{"type": "symbol", "style": []any{graph.StyleArrow}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("glyph"), "size": value(graph.ArrowSymbolSize),
		}}}
	})...)
	marks = append(marks,
		map[string]any{"type": "rect", "name": "relBoxes", "from": map[string]any{"data": "relBoxes"}, "style": []any{graph.StyleEdgeLabelBox}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}}},
	)
	marks = append(marks, sigMarks(&data, "relTexts", rTexts, textKeys, textRename, func() map[string]any {
		return map[string]any{"type": "text", "style": []any{graph.StyleEdgeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
			"align": value("center"), "baseline": value("middle"),
		}}}
	})...)
	if links {
		// Each mark of an element, a boundary or a relationship carries the
		// link of its row (D32).
		for _, m := range marks {
			mm, _ := m.(map[string]any)
			switch mm["type"] {
			case "rect", "path", "text":
				enc, _ := mm["encode"].(map[string]any)["update"].(map[string]any)
				enc["href"] = field("href")
			}
		}
	}
	return map[string]any{
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": c4Colors, "range": "category"}},
		"marks":    marks,
	}
}

// c4Parent returns the alias of the boundary that holds a part, or nil
// for the whole diagram.
func c4Parent(alias string) any {
	if alias == c4Global {
		return nil
	}
	return alias
}

// itoa writes a whole number.
func itoa(n int) string {
	return num(float64(n))
}
