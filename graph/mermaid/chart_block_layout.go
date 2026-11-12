package mermaid

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
)

// style returns the style of a block: the classDef default for a block
// with no class, or else its classes in order, and then its own style.
func (b *blockDiagram) style(n *blockNode) map[string]string {
	st := map[string]string{}
	names := n.classes
	if len(names) == 0 {
		names = []string{"default"}
	}
	for _, c := range names {
		maps.Copy(st, b.classDefs[c])
	}
	maps.Copy(st, n.own)
	return st
}

// measure sets the natural size of a block: the size of its shape around
// its label, or the size of the grid of a composite block. The blocks of a
// grid all take the size of the largest, with the width of a wide block
// shared by its columns, as in Mermaid.
func (b *blockDiagram) measure(n *blockNode) {
	n.row = map[string]any{}
	graph.StyleRow(n.row, b.style(n))
	n.size = b.font
	if v, ok := n.row["fontSize"].(float64); ok && v > 0 {
		n.size = v
	}
	weight := 400
	if w, ok := n.row["fontWeight"].(string); ok {
		if v, err := strconv.Atoi(w); err == nil {
			weight = v
		} else if w == "bold" {
			weight = boldWeight
		}
	}
	if n.kind != blockSpace && n.label != "" {
		n.lines, n.tw = plainLines(n.label, n.size, min(weight, boldWeight), 0, b.m)
		n.th = float64(len(n.lines)) * n.size * 1.3
	}
	switch n.kind {
	case blockSpace:
		return
	case blockLeaf:
		if n.shape == blockArrow {
			h := n.th + 2*b.pad
			n.w, n.h = n.tw+h+b.pad, h
			return
		}
		n.w, n.h = graph.NodeSize(n.shape, n.tw, n.th, b.pad)
		return
	}
	if n.label != "" {
		n.titleH = math.Round(n.th + 8)
	}
	col := 0
	for _, k := range n.kids {
		b.measure(k)
		if k.kind != blockSpace {
			n.cellW = max(n.cellW, k.w/float64(k.span))
			n.cellH = max(n.cellH, k.h)
		}
		// A block that is wider than the columns left in its row takes
		// the rest of the row, and draws over the edge of the grid, as
		// in Mermaid.
		fill := k.span
		if n.columns > 0 {
			k.px, k.py = col%n.columns, col/n.columns
			fill = min(fill, n.columns-k.px)
		} else {
			k.px, k.py = col, 0
		}
		n.cols, n.rows = max(n.cols, k.px+fill), max(n.rows, k.py+1)
		col += fill
	}
	n.w = float64(n.cols)*(n.cellW+b.pad) + b.pad
	n.h = n.titleH + float64(n.rows)*(n.cellH+b.pad) + b.pad
	if n.label != "" {
		n.w = max(n.w, n.tw+2*b.pad)
	}
}

// assign gives a block its box, at its x and y, of a width and a height
// at least its natural size. A composite block that is larger than its
// grid grows its cells to fill it.
func (b *blockDiagram) assign(n *blockNode, w, h float64) {
	n.bw, n.bh = w, h
	switch n.kind {
	case blockSpace:
		return
	case blockLeaf:
		n.dw, n.dh = w, h
		switch n.shape {
		case "circle", "doublecircle", "diamond":
			// These shapes keep their own size, in the middle of the box.
			n.dw, n.dh = n.w, n.h
		case "stadium":
			// A stadium keeps its own height, so that its ends stay round.
			n.dh = n.h
		case blockArrow:
			// A block arrow keeps its own size, unless it spans more than
			// one column.
			n.dh = n.h
			if n.span == 1 {
				n.dw = n.w
			}
		}
		return
	}
	if n.cols == 0 {
		return
	}
	cellW := max(n.cellW, (w-b.pad*float64(n.cols+1))/float64(n.cols))
	cellH := max(n.cellH, (h-n.titleH-b.pad*float64(n.rows+1))/float64(n.rows))
	for _, k := range n.kids {
		k.x = n.x + b.pad + float64(k.px)*(cellW+b.pad)
		k.y = n.y + n.titleH + b.pad + float64(k.py)*(cellH+b.pad)
		b.assign(k, float64(k.span)*cellW+float64(k.span-1)*b.pad, cellH)
	}
}

// center returns the center of the box of a block.
func (n *blockNode) center() (float64, float64) {
	return n.x + n.bw/2, n.y + n.bh/2
}

// clip returns the point where a line from the center of a block toward a
// point leaves the shape of the block.
func (n *blockNode) clip(tx, ty float64) (float64, float64) {
	cx, cy := n.center()
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return cx, cy
	}
	w, h := n.dw/2, n.dh/2
	if n.kind == blockComposite {
		w, h = n.bw/2, n.bh/2
	}
	var t float64
	switch n.shape {
	case "circle", "doublecircle":
		t = w / math.Hypot(dx, dy)
	case "diamond":
		t = 1 / (math.Abs(dx)/w + math.Abs(dy)/h)
	default:
		t = math.Inf(1)
		if dx != 0 {
			t = w / math.Abs(dx)
		}
		if dy != 0 {
			t = min(t, h/math.Abs(dy))
		}
	}
	t = min(t, 1)
	return cx + dx*t, cy + dy*t
}

// arrowPath returns the path of a block arrow of a size around 0, 0, by its
// directions. An arrow to one side, or to both sides of one axis, has a
// shaft as high as its label and a head as wide as the arrow. An arrow on
// both axes is a box with a point on each side of its directions.
func arrowPath(dirs map[string]bool, w, h, pad float64) string {
	x, y := w/2, h/2
	l, r, u, d := dirs["left"], dirs["right"], dirs["up"], dirs["down"]
	p := pad / 2
	switch {
	case (l || r) && !u && !d:
		m := min(y, x/2)
		x0, x1 := -x, x
		if l {
			x0 = -x + m
		}
		if r {
			x1 = x - m
		}
		pts := [][2]float64{{x0, -y + p}, {x1, -y + p}}
		if r {
			pts = append(pts, [2]float64{x1, -y}, [2]float64{x, 0}, [2]float64{x1, y})
		}
		pts = append(pts, [2]float64{x1, y - p}, [2]float64{x0, y - p})
		if l {
			pts = append(pts, [2]float64{x0, y}, [2]float64{-x, 0}, [2]float64{x0, -y})
		}
		return polygon(pts)
	case (u || d) && !l && !r:
		m := min(y/2, x)
		y0, y1 := -y, y
		if u {
			y0 = -y + m
		}
		if d {
			y1 = y - m
		}
		s := min(x/2, y)
		pts := [][2]float64{{-x + s, y0}}
		if u {
			pts = append(pts, [2]float64{-x, y0}, [2]float64{0, -y}, [2]float64{x, y0})
		}
		pts = append(pts, [2]float64{x - s, y0}, [2]float64{x - s, y1})
		if d {
			pts = append(pts, [2]float64{x, y1}, [2]float64{0, y}, [2]float64{-x, y1})
		}
		pts = append(pts, [2]float64{-x + s, y1})
		return polygon(pts)
	}
	// A point of 2 times the padding on each side of a direction.
	q := 2 * p
	bx, by := x, y
	if l || r {
		bx = x - q
	}
	if u || d {
		by = y - q
	}
	pts := [][2]float64{{-bx, -by}}
	if u {
		pts = append(pts, [2]float64{-bx / 3, -by}, [2]float64{0, -y}, [2]float64{bx / 3, -by})
	}
	pts = append(pts, [2]float64{bx, -by})
	if r {
		pts = append(pts, [2]float64{bx, -by / 3}, [2]float64{x, 0}, [2]float64{bx, by / 3})
	}
	pts = append(pts, [2]float64{bx, by})
	if d {
		pts = append(pts, [2]float64{bx / 3, by}, [2]float64{0, y}, [2]float64{-bx / 3, by})
	}
	pts = append(pts, [2]float64{-bx, by})
	if l {
		pts = append(pts, [2]float64{-bx, by / 3}, [2]float64{-x, 0}, [2]float64{-bx, -by / 3})
	}
	return polygon(pts)
}

// all returns the blocks of a grid and of the grids in it, each composite
// block before the blocks in it, and sets the parent of each.
func (b *blockDiagram) all(n *blockNode) []*blockNode {
	var out []*blockNode
	for _, k := range n.kids {
		if n != b.root {
			k.parent = n
		}
		out = append(out, k)
		out = append(out, b.all(k)...)
	}
	return out
}

// spec writes the Vega spec of the diagram that measure and assign laid
// out.
func (b *blockDiagram) spec() map[string]any {
	blocks := b.all(b.root)
	type edgeLine struct {
		e              *blockEdge
		x0, y0, x1, y1 float64
	}
	var lines []edgeLine
	for _, e := range b.edges {
		na, nb := b.byID[e.a], b.byID[e.b]
		ax, ay := na.center()
		bx, by := nb.center()
		x0, y0 := na.clip(bx, by)
		x1, y1 := nb.clip(ax, ay)
		lines = append(lines, edgeLine{e, x0, y0, x1, y1})
	}
	// The diagram starts at 0, 0.
	minX, minY := math.Inf(1), math.Inf(1)
	for _, n := range blocks {
		if n.kind == blockSpace {
			continue
		}
		cx, cy := n.center()
		if n.kind == blockComposite {
			minX, minY = min(minX, n.x), min(minY, n.y)
		} else {
			minX, minY = min(minX, cx-n.dw/2), min(minY, cy-n.dh/2)
		}
	}
	for _, l := range lines {
		minX, minY = min(minX, l.x0, l.x1), min(minY, l.y0, l.y1)
	}
	if math.IsInf(minX, 1) {
		minX, minY = 0, 0
	}
	var composites, shapes, labels, titles, edges, arrows, edgeLabels []any
	lh := func(n *blockNode) float64 { return n.size * 1.3 }
	for _, n := range blocks {
		if n.kind == blockSpace {
			continue
		}
		n.x, n.y = n.x-minX, n.y-minY
		cx, cy := n.center()
		r := map[string]any{"id": n.id, "label": n.label, "parent": nil}
		if n.parent != nil {
			r["parent"] = n.parent.id
		}
		maps.Copy(r, n.row)
		if n.kind == blockComposite {
			r["x"], r["y"], r["width"], r["height"] = n.x, n.y, n.bw, n.bh
			composites = append(composites, r)
			if len(n.lines) > 0 {
				t := map[string]any{"id": n.id, "text": strings.Join(n.lines, " "), "x": n.x + b.pad, "y": n.y + n.titleH/2 + 2, "fontSize": n.size}
				copyKeys(t, n.row, "textColor", "fontWeight", "fontStyle")
				titles = append(titles, t)
			}
			continue
		}
		path := graph.ShapePath(n.shape, n.dw, n.dh)
		if n.shape == blockArrow {
			path = arrowPath(n.dirs, n.dw, n.dh, b.pad)
		}
		r["shape"], r["x"], r["y"], r["width"], r["height"], r["path"] = n.shape, cx, cy, n.dw, n.dh, path
		shapes = append(shapes, r)
		y := cy - n.th/2 + lh(n)/2
		for i, l := range n.lines {
			t := map[string]any{"id": n.id, "line": i, "text": l, "x": cx, "y": y, "fontSize": n.size}
			copyKeys(t, n.row, "textColor", "fontWeight", "fontStyle")
			labels = append(labels, t)
			y += lh(n)
		}
	}
	counts := map[[2]string]int{}
	for i, l := range lines {
		e := l.e
		x0, y0, x1, y1 := l.x0-minX, l.y0-minY, l.x1-minX, l.y1-minY
		key := [2]string{e.a, e.b}
		counts[key]++
		r := map[string]any{
			"key": i, "id": fmt.Sprintf("%d-%s-%s", counts[key], e.a, e.b), "source": e.a, "target": e.b, "label": e.label,
			"path": "M" + num(x0) + "," + num(y0) + "L" + num(x1) + "," + num(y1),
		}
		if e.thick {
			r["strokeWidth"] = 2.5
		}
		if e.dotted {
			r["strokeDash"] = []any{3.0, 3.0}
		}
		edges = append(edges, r)
		angle := math.Atan2(y1-y0, x1-x0) * 180 / math.Pi
		for _, end := range []struct {
			kind, at string
			x, y, a  float64
		}{{e.head, e.b, x1, y1, angle}, {e.tail, e.a, x0, y0, angle + 180}} {
			if end.kind == "none" {
				continue
			}
			fill, open := graph.ArrowPaths(end.kind)
			arrows = append(arrows, map[string]any{
				"edge": i, "end": end.at, "kind": end.kind, "x": end.x, "y": end.y, "angle": end.a, "fill": fill, "open": open,
			})
		}
		if e.label != "" {
			ls, w := plainLines(e.label, b.font, 400, 0, b.m)
			tl := []any{}
			for _, s := range ls {
				tl = append(tl, s)
			}
			edgeLabels = append(edgeLabels, map[string]any{
				"edge": i, "text": tl, "x": (x0 + x1) / 2, "y": (y0 + y1) / 2,
				"width": w + 8, "height": float64(len(ls))*b.font*1.3 + 4,
			})
		}
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	var data, marks []any
	data = append(data,
		map[string]any{"name": "composites", "values": orEmpty(composites)},
		map[string]any{"name": "titles", "values": orEmpty(titles)},
		map[string]any{"name": "blocks", "values": orEmpty(shapes)},
		map[string]any{"name": "labels", "values": orEmpty(labels)},
		map[string]any{"name": "edges", "values": orEmpty(edges)},
		map[string]any{"name": "arrows", "values": orEmpty(arrows)},
		filteredData("arrowsFill", "arrows", "datum.fill !== ''"),
		filteredData("arrowsOpen", "arrows", "datum.open !== ''"),
		map[string]any{"name": "edgeLabels", "values": orEmpty(edgeLabels)},
	)
	textRename := map[string]string{"textColor": "fill"}
	marks = append(marks, sigMarks(&data, "composites", composites, graph.PaintKeys(), nil, func() map[string]any {
		return map[string]any{"type": "rect", "style": []any{graph.StyleCluster}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "titles", titles, []string{"textColor", "fontWeight", "fontStyle"}, textRename, func() map[string]any {
		return map[string]any{"type": "text", "style": []any{graph.StyleClusterTitle}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
			"align": value("left"), "baseline": value("middle"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "blocks", shapes, graph.PaintKeys(), nil, func() map[string]any {
		return map[string]any{"type": "path", "style": []any{graph.StyleNode}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "labels", labels, []string{"textColor", "fontWeight", "fontStyle"}, textRename, func() map[string]any {
		return map[string]any{"type": "text", "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
			"align": value("center"), "baseline": value("middle"),
		}}}
	})...)
	marks = append(marks, sigMarks(&data, "edges", edges, []string{"strokeWidth", "strokeDash"}, nil, func() map[string]any {
		return map[string]any{"type": "path", "style": []any{graph.StyleEdge}, "encode": map[string]any{"update": map[string]any{
			"x": value(0), "y": value(0), "path": field("path"),
		}}}
	})...)
	marks = append(marks,
		map[string]any{"type": "symbol", "name": "arrows", "from": map[string]any{"data": "arrowsFill"}, "style": []any{graph.StyleArrow}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("fill"), "size": value(graph.ArrowSymbolSize),
		}}},
		map[string]any{"type": "symbol", "name": "openArrows", "from": map[string]any{"data": "arrowsOpen"}, "style": []any{graph.StyleArrowOpen}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("open"), "size": value(graph.ArrowSymbolSize),
		}}},
		map[string]any{"type": "rect", "name": "edgeLabelBoxes", "from": map[string]any{"data": "edgeLabels"}, "style": []any{graph.StyleEdgeLabelBox}, "encode": map[string]any{"update": map[string]any{
			"xc": field("x"), "yc": field("y"), "width": field("width"), "height": field("height"),
		}}},
		map[string]any{"type": "text", "name": "edgeLabels", "from": map[string]any{"data": "edgeLabels"}, "style": []any{graph.StyleEdgeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": value(b.font), "lineHeight": value(b.font * 1.3),
			"align": value("center"), "baseline": value("middle"),
		}}},
	)
	return map[string]any{
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"marks":    marks,
	}
}

// copyKeys copies the keys of a map that it holds into another.
func copyKeys(dst, src map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

// sigMarks returns a mark of the rows of a dataset for each set of the
// keys of a style that its rows hold, each from a dataset of the rows that
// hold that set, and encodes each key of the set from its field. The mark
// of the rows that hold no key takes the name of the dataset.
func sigMarks(data *[]any, source string, rows []any, keys []string, rename map[string]string, mark func() map[string]any) []any {
	var sigs []string
	for _, row := range rows {
		r, _ := row.(map[string]any)
		var set []string
		for _, k := range keys {
			if _, ok := r[k]; ok {
				set = append(set, k)
			}
		}
		s := strings.Join(set, ",")
		r["_sig"] = s
		if !slices.Contains(sigs, s) {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) == 0 {
		sigs = []string{""}
	}
	slices.Sort(sigs)
	var out []any
	for i, s := range sigs {
		m := mark()
		name, src := source, source
		if s != "" {
			name = fmt.Sprintf("%s%d", source, i)
			src = name + "Data"
			*data = append(*data, filteredData(src, source, fmt.Sprintf("datum._sig === %q", s)))
		} else if len(sigs) > 1 {
			src = source + "Plain"
			*data = append(*data, filteredData(src, source, "datum._sig === ''"))
		}
		m["name"], m["from"] = name, map[string]any{"data": src}
		if s != "" {
			enc, _ := m["encode"].(map[string]any)["update"].(map[string]any)
			for k := range strings.SplitSeq(s, ",") {
				n := k
				if r, ok := rename[k]; ok {
					n = r
				}
				enc[n] = map[string]any{"field": k}
			}
		}
		out = append(out, m)
	}
	return out
}
