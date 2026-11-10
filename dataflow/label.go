package dataflow

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// TextEnv is an Env that measures text as the writer draws it. The
// transform label needs it.
type TextEnv interface {
	// TextSize returns the width and the height of a text, which is a
	// string or a list of lines, in a font, a size and a weight. An empty
	// font or a size of 0 means that of the theme.
	TextSize(text any, font string, size float64, weight string) (float64, float64)
}

// MarkEnv is an Env that gives the items of the marks of a view, by name.
// The transform label needs it for the marks that it avoids.
type MarkEnv interface {
	// MarkShapes returns the type of a mark and a row for each of its
	// items, with x, y, x2, y2, width, height and bounds, and whether the
	// view has the mark.
	MarkShapes(name string) (string, []Row, bool)
}

// labelAnchor is a place of a label around its item: dx and dy are -1, 0
// or 1, from the left or the top to the right or the bottom.
type labelAnchor struct {
	dx, dy int
}

// labelAnchors are the anchors of a label, by name.
var labelAnchors = map[string]labelAnchor{
	"top-left": {-1, -1}, "top": {0, -1}, "top-right": {1, -1},
	"left": {-1, 0}, "middle": {0, 0}, "right": {1, 0},
	"bottom-left": {-1, 1}, "bottom": {0, 1}, "bottom-right": {1, 1},
}

// bitmap marks the pixels that a label cannot cover.
type bitmap struct {
	w, h int
	bits []bool
}

// newBitmap returns an empty bitmap of a size, in pixels.
func newBitmap(w, h int) *bitmap {
	return &bitmap{w: w, h: h, bits: make([]bool, w*h)}
}

// cells returns the range of the cells that a box covers, clipped to the
// bitmap, and whether any cell remains.
func (b *bitmap) cells(x1, y1, x2, y2 float64) (int, int, int, int, bool) {
	i1, j1 := max(0, int(math.Floor(x1))), max(0, int(math.Floor(y1)))
	i2, j2 := min(b.w-1, int(math.Ceil(x2))-1), min(b.h-1, int(math.Ceil(y2))-1)
	return i1, j1, i2, j2, i1 <= i2 && j1 <= j2
}

// fill marks the cells of a box.
func (b *bitmap) fill(x1, y1, x2, y2 float64) {
	i1, j1, i2, j2, ok := b.cells(x1, y1, x2, y2)
	if !ok {
		return
	}
	for j := j1; j <= j2; j++ {
		for i := i1; i <= i2; i++ {
			b.bits[j*b.w+i] = true
		}
	}
}

// set marks the cell of a point.
func (b *bitmap) set(x, y float64) {
	i, j := int(math.Floor(x)), int(math.Floor(y))
	if i >= 0 && j >= 0 && i < b.w && j < b.h {
		b.bits[j*b.w+i] = true
	}
}

// line marks the cells along a segment.
func (b *bitmap) line(x1, y1, x2, y2 float64) {
	n := int(math.Ceil(2*math.Max(math.Abs(x2-x1), math.Abs(y2-y1)))) + 1
	for k := range n + 1 {
		t := float64(k) / float64(n)
		b.set(x1+t*(x2-x1), y1+t*(y2-y1))
	}
}

// polygon marks the cells whose centers lie inside a closed polygon, by
// the even-odd rule.
func (b *bitmap) polygon(xs, ys []float64) {
	n := len(xs)
	for j := range b.h {
		cy := float64(j) + 0.5
		var cross []float64
		for k := range n {
			x1, y1, x2, y2 := xs[k], ys[k], xs[(k+1)%n], ys[(k+1)%n]
			if (y1 <= cy) != (y2 <= cy) {
				cross = append(cross, x1+(cy-y1)*(x2-x1)/(y2-y1))
			}
		}
		slices.Sort(cross)
		for k := 0; k+1 < len(cross); k += 2 {
			b.fill(math.Round(cross[k]), float64(j), math.Round(cross[k+1]), float64(j+1))
		}
	}
	for k := range n {
		b.line(xs[k], ys[k], xs[(k+1)%n], ys[(k+1)%n])
	}
}

// free reports whether no cell of a box is marked.
func (b *bitmap) free(x1, y1, x2, y2 float64) bool {
	i1, j1, i2, j2, ok := b.cells(x1, y1, x2, y2)
	if !ok {
		return true
	}
	for j := j1; j <= j2; j++ {
		for i := i1; i <= i2; i++ {
			if b.bits[j*b.w+i] {
				return false
			}
		}
	}
	return true
}

// labelBox is the box of an item, as x1, y1, x2, y2.
type labelBox [4]float64

// boundsOf returns the box of an item row: its bounds, or else the point
// of x and y, and whether it has one.
func boundsOf(r Row) (labelBox, bool) {
	if b, ok := r["bounds"].(map[string]any); ok {
		return labelBox{expr.Number(b["x1"]), expr.Number(b["y1"]), expr.Number(b["x2"]), expr.Number(b["y2"])}, true
	}
	x, y := expr.Number(r["x"]), expr.Number(r["y"])
	if math.IsNaN(x) || math.IsNaN(y) {
		return labelBox{}, false
	}
	return labelBox{x, y, x, y}, true
}

// label places the text of each row around the item that its datum names,
// where the text covers no other mark and no label placed before it, and
// inside the size. It tries each offset, and for each the anchors in
// order. A label that has no place gets the opacity 0.
func label(p *params, rows []Row) ([]Row, error) {
	w, h, ok, err := sizeOf(p, "size")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("the size: %w", ErrParam)
	}
	for _, name := range []string{"lineAnchor", "markIndex"} {
		if _, ok := p.t.Param(name); ok {
			return nil, vega.Issue{Err: vega.ErrUnsupported, Message: "the label parameter " + name}
		}
	}
	method, err := p.str("method", "naive")
	if err != nil {
		return nil, err
	}
	if method != "naive" {
		return nil, vega.Issue{Err: vega.ErrUnsupported, Message: "the label method " + method}
	}
	te, ok := p.env.(TextEnv)
	if !ok {
		return nil, fmt.Errorf("the measurement of text: %w", ErrParam)
	}
	cmp, err := p.sort()
	if err != nil {
		return nil, err
	}
	names, err := p.strings("anchor")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		names = []string{"top-left", "left", "bottom-left", "top", "bottom", "top-right", "right", "bottom-right"}
	}
	anchors := make([]labelAnchor, len(names))
	for i, n := range names {
		a, ok := labelAnchors[n]
		if !ok {
			return nil, fmt.Errorf("the anchor %q: %w", n, ErrParam)
		}
		anchors[i] = a
	}
	offsets, err := p.numbers("offset")
	if err != nil {
		return nil, err
	}
	if len(offsets) == 0 {
		offsets = []float64{1}
	}
	padding, err := p.number("padding", 0)
	if err != nil {
		return nil, err
	}
	avoidBase, err := p.boolean("avoidBaseMark", true)
	if err != nil {
		return nil, err
	}
	avoid, err := p.strings("avoidMarks")
	if err != nil {
		return nil, err
	}
	as, err := p.as("x", "y", "opacity", "align", "baseline")
	if err != nil {
		return nil, err
	}
	// The bitmap covers the size and the padding, with the origin moved by
	// the padding.
	pad := math.Max(0, padding)
	bw, bh := int(math.Ceil(w+2*pad)), int(math.Ceil(h+2*pad))
	marks, labels := newBitmap(bw, bh), newBitmap(bw, bh)
	shift := func(b labelBox) labelBox {
		return labelBox{b[0] + pad, b[1] + pad, b[2] + pad, b[3] + pad}
	}
	bases := make([]labelBox, len(rows))
	hasBase := make([]bool, len(rows))
	for i, r := range rows {
		d, _ := r["datum"].(map[string]any)
		bases[i], hasBase[i] = boundsOf(d)
		if hasBase[i] && avoidBase {
			b := shift(bases[i])
			marks.fill(b[0], b[1], b[2], b[3])
		}
	}
	if len(avoid) > 0 {
		me, ok := p.env.(MarkEnv)
		if !ok {
			return nil, fmt.Errorf("the marks to avoid: %w", ErrParam)
		}
		for _, name := range avoid {
			typ, items, ok := me.MarkShapes(name)
			if !ok {
				return nil, fmt.Errorf("the mark %q to avoid: %w", name, ErrParam)
			}
			markShapes(marks, typ, items, pad)
		}
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	if cmp != nil {
		slices.SortStableFunc(order, func(a, b int) int { return cmp(rows[a], rows[b]) })
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	for _, i := range order {
		r := out[i]
		r[as[2]] = 0.0
		if !hasBase[i] {
			continue
		}
		size := 0.0
		if v, ok := r["fontSize"]; ok {
			size = expr.Number(v)
		}
		tw, th := te.TextSize(r["text"], expr.String(r["font"]), size, expr.String(r["fontWeight"]))
		if tw <= 0 || th <= 0 {
			continue
		}
		base := shift(bases[i])
		x, y, a, ok := placeLabel(marks, labels, base, tw, th, anchors, offsets)
		if !ok {
			continue
		}
		r[as[0]], r[as[1]], r[as[2]] = x-pad, y-pad, 1.0
		r[as[3]] = [...]string{"right", "center", "left"}[a.dx+1]
		r[as[4]] = [...]string{"bottom", "middle", "top"}[a.dy+1]
	}
	return out, nil
}

// placeLabel returns the first place of a text of a size, around a base
// box, that lies inside the bitmaps and covers no marked cell: the point
// of the anchor, the anchor, and whether a place exists. It marks the box
// of the text in labels.
func placeLabel(marks, labels *bitmap, base labelBox, tw, th float64, anchors []labelAnchor, offsets []float64) (float64, float64, labelAnchor, bool) {
	cx, cy := (base[0]+base[2])/2, (base[1]+base[3])/2
	for _, off := range offsets {
		for _, a := range anchors {
			// An offset at a corner runs along the diagonal.
			o := off
			if a.dx != 0 && a.dy != 0 {
				o *= math.Sqrt2 / 2
			}
			x := [...]float64{base[0], cx, base[2]}[a.dx+1] + float64(a.dx)*o
			y := [...]float64{base[1], cy, base[3]}[a.dy+1] + float64(a.dy)*o
			// The box of the text, from its align and its baseline.
			x1 := x - tw*[...]float64{1, 0.5, 0}[a.dx+1]
			y1 := y - th*[...]float64{1, 0.5, 0}[a.dy+1]
			x2, y2 := x1+tw, y1+th
			if x1 < 0 || y1 < 0 || x2 > float64(marks.w) || y2 > float64(marks.h) {
				continue
			}
			if !labels.free(x1, y1, x2, y2) {
				continue
			}
			if a.dx == 0 && a.dy == 0 {
				// A label in the middle lies inside its item.
				if tw > base[2]-base[0] || th > base[3]-base[1] {
					continue
				}
			} else if !marks.free(x1, y1, x2, y2) {
				continue
			}
			labels.fill(x1, y1, x2, y2)
			return x, y, a, true
		}
	}
	return 0, 0, labelAnchor{}, false
}

// markShapes marks the items of a mark in a bitmap, moved by pad: the
// path of a line or a trail, the shape of an area, and the bounds of the
// item of any other mark.
func markShapes(b *bitmap, typ string, items []Row, pad float64) {
	pt := func(r Row, xk, yk string) (float64, float64, bool) {
		x, y := expr.Number(r[xk]), expr.Number(r[yk])
		return x + pad, y + pad, !math.IsNaN(x) && !math.IsNaN(y)
	}
	switch typ {
	case "line", "trail":
		var px, py float64
		have := false
		for _, r := range items {
			x, y, ok := pt(r, "x", "y")
			if !ok {
				have = false
				continue
			}
			if have {
				b.line(px, py, x, y)
			} else {
				b.set(x, y)
			}
			px, py, have = x, y, true
		}
	case "area":
		var xs, ys, bx, by []float64
		for _, r := range items {
			x, y, ok := pt(r, "x", "y")
			if !ok {
				continue
			}
			x2, y2 := x, y
			if v := expr.Number(r["x2"]); !math.IsNaN(v) && r["x2"] != nil {
				x2 = v + pad
			}
			if v := expr.Number(r["y2"]); !math.IsNaN(v) && r["y2"] != nil {
				y2 = v + pad
			}
			xs, ys = append(xs, x), append(ys, y)
			bx, by = append(bx, x2), append(by, y2)
		}
		slices.Reverse(bx)
		slices.Reverse(by)
		if len(xs) > 0 {
			b.polygon(append(xs, bx...), append(ys, by...))
		}
	default:
		for _, r := range items {
			if box, ok := boundsOf(r); ok {
				b.fill(box[0]+pad, box[1]+pad, box[2]+pad, box[3]+pad)
			}
		}
	}
}
