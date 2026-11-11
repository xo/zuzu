package dataflow

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
)

// textBox is the box of a label or a node, in pixels.
type textBox struct {
	x1, y1, x2, y2 float64
}

// hits reports whether two boxes overlap, with a gap of pad between them.
// Boxes that overlap by less than a thousandth of a pixel do not hit.
func (a textBox) hits(b textBox, pad float64) bool {
	const eps = 1e-3
	return a.x1+eps < b.x2+pad && b.x1-pad+eps < a.x2 && a.y1+eps < b.y2+pad && b.y1-pad+eps < a.y2
}

// sankeyLabel is the label of a node while zuzu.sankeylabels places it.
type sankeyLabel struct {
	text  string
	side  float64 // 1 at the right of the node, and -1 at the left
	y     float64 // the middle of the label
	mid   float64 // the middle of the node
	reach float64 // how far the middle of the label can move from mid
	box   textBox
}

// sankeylabels places the label of each node of a sankey, on the rows that
// zuzu.sankey made. Each row gets the place, the alignment and the text of
// its label. The rule has four steps:
//
//  1. The labels of the larger nodes go first. A label goes on the inner
//     side of its node: at the right of a node in the left half of the
//     extent, and at the left of the others. If it hits a label or a node
//     there, it goes on the other side. If it hits on both sides, it stays
//     on the first side that is inside the extent and clear of the nodes,
//     or else on the inner side.
//  2. A label that hits a node moves up or down, away from it. A label
//     stays beside its node: the box of a label stays within the height of
//     its node, or the middle of a label moves at most half the height of
//     a label from the middle of its node. If no move within this reach
//     clears the node, the label loses the end of its text to an
//     ellipsis, at the side and the height that keep the most of it.
//  3. Labels that hit each other move apart, up and down, within their
//     reach.
//  4. A label that still hits loses the end of its text in the same way.
//     The labels of the smaller nodes lose their ends first. A label that
//     cannot keep one letter keeps all of its text where it is.
func sankeylabels(p *params, rows []Row) ([]Row, error) {
	textOf, err := p.field("text")
	if err != nil {
		return nil, err
	}
	if textOf == nil {
		textOf = FieldAccessor("key")
	}
	ex0, _, ex1, _, err := p.extentOf()
	if err != nil {
		return nil, err
	}
	offset, err := p.number("offset", 6)
	if err != nil {
		return nil, err
	}
	font, err := p.str("font", "")
	if err != nil {
		return nil, err
	}
	size, err := p.number("fontSize", 11)
	if err != nil {
		return nil, err
	}
	weight, err := p.str("fontWeight", "normal")
	if err != nil {
		return nil, err
	}
	as, err := p.as("labelX", "labelY", "labelAlign", "label")
	if err != nil {
		return nil, err
	}
	te, _ := p.env.(TextEnv)
	width := func(s string) float64 {
		if s == "" {
			return 0
		}
		if te == nil {
			return 0.6 * size * float64(len([]rune(s)))
		}
		w, _ := te.TextSize(s, font, size, weight)
		return w
	}
	// A label is as tall as its font size. A label keeps a gap of 2 from a
	// node, and a gap of 1 from another label.
	const nodeGap, labelGap = 2.0, 1.0
	h := size
	get := func(r Row, f string) float64 { return expr.Number(r[f]) }
	nodes := make([]textBox, len(rows))
	labels := make([]sankeyLabel, len(rows))
	for i, r := range rows {
		n := textBox{get(r, "x0"), get(r, "y0"), get(r, "x1"), get(r, "y1")}
		nodes[i] = n
		l := &labels[i]
		if v := textOf.Get(r); v != nil && !expr.IsUndefined(v) {
			l.text = expr.String(v)
		}
		l.mid = (n.y1 + n.y2) / 2
		l.y = l.mid
		l.reach = max((n.y2-n.y1-h)/2, h/2)
		l.side = 1
		if n.x1 >= (ex0+ex1)/2 {
			l.side = -1
		}
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Compare(get(rows[b], "value"), get(rows[a], "value"))
	})
	boxAt := func(i int, side, y float64, text string) textBox {
		n, w := nodes[i], width(text)
		if side > 0 {
			return textBox{n.x2 + offset, y - h/2, n.x2 + offset + w, y + h/2}
		}
		return textBox{n.x1 - offset - w, y - h/2, n.x1 - offset, y + h/2}
	}
	inside := func(b textBox) bool { return b.x1 >= ex0 && b.x2 <= ex1 }
	hitsNode := func(b textBox) bool {
		for _, n := range nodes {
			if b.hits(n, nodeGap) {
				return true
			}
		}
		return false
	}
	set := make([]bool, len(rows))
	// clashes reports whether the box of the label i, at b, leaves the
	// extent or hits a node or a label that has a place.
	clashes := func(i int, b textBox) bool {
		if !inside(b) || hitsNode(b) {
			return true
		}
		for j := range labels {
			if j != i && set[j] && b.hits(labels[j].box, labelGap) {
				return true
			}
		}
		return false
	}
	// Step 1: the side.
	for _, i := range order {
		l := &labels[i]
		inner := l.side
		chosen := false
		for _, side := range []float64{inner, -inner} {
			if b := boxAt(i, side, l.y, l.text); !clashes(i, b) {
				l.side, l.box, chosen = side, b, true
				break
			}
		}
		if !chosen {
			l.side = inner
			for _, side := range []float64{inner, -inner} {
				if b := boxAt(i, side, l.y, l.text); inside(b) && !hitsNode(b) {
					l.side = side
					break
				}
			}
			l.box = boxAt(i, l.side, l.y, l.text)
		}
		set[i] = true
	}
	// move moves a label by up to dy, within its reach, and returns how far
	// it moved.
	move := func(i int, dy float64) float64 {
		l := &labels[i]
		y := math.Min(math.Max(l.y+dy, l.mid-l.reach), l.mid+l.reach)
		moved := y - l.y
		l.y = y
		l.box.y1 += moved
		l.box.y2 += moved
		return moved
	}
	// clearNodes moves a label off each node that it hits: away from the
	// middle of the node, or else the other way, but only when the move
	// clears the node. It reports whether the label moved.
	clearNodes := func(i int) bool {
		l := &labels[i]
		moved := false
		for _, n := range nodes {
			b := l.box
			if !b.hits(n, nodeGap) {
				continue
			}
			dys := []float64{n.y2 + nodeGap - b.y1, n.y1 - nodeGap - b.y2}
			if l.y < (n.y1+n.y2)/2 {
				dys[0], dys[1] = dys[1], dys[0]
			}
			for _, dy := range dys {
				if math.Abs(l.y+dy-l.mid) <= l.reach+1e-9 {
					move(i, dy)
					moved = true
					break
				}
			}
		}
		return moved
	}
	// shorten cuts the end of the text of a label off with an ellipsis, so
	// that it does not clash, at the side and the height that keep the most
	// of it. It tries the place of the label first, then the middle of the
	// node, then steps of a pixel up and down within the reach.
	shorten := func(i int, clash func(int, textBox) bool) {
		l := &labels[i]
		runes := []rune(l.text)
		cut := func(k int) string {
			if k >= len(runes) {
				return l.text
			}
			return strings.TrimRight(string(runes[:k]), " ") + "…"
		}
		keep, best := 0, *l
		ys := []float64{l.y, l.mid}
		for d := 1.0; d <= l.reach; d++ {
			ys = append(ys, l.mid+d, l.mid-d)
		}
		for _, y := range ys {
			for _, side := range []float64{l.side, -l.side} {
				// The longest start of the text that fits, by a binary
				// search, since a longer start is never narrower.
				lo, hi := 0, len(runes)
				for lo < hi {
					k := (lo + hi + 1) / 2
					if clash(i, boxAt(i, side, y, cut(k))) {
						hi = k - 1
					} else {
						lo = k
					}
				}
				if lo > keep {
					keep = lo
					best.side, best.y, best.text = side, y, cut(lo)
					best.box = boxAt(i, side, y, best.text)
				}
			}
		}
		if keep > 0 {
			*l = best
		}
	}
	// Step 2: a label that hits a node moves off it, or else loses the end
	// of its text.
	for _, i := range order {
		for range nodes {
			if !clearNodes(i) {
				break
			}
		}
		if hitsNode(labels[i].box) {
			shorten(i, func(_ int, b textBox) bool { return !inside(b) || hitsNode(b) })
		}
	}
	// Step 3: labels that hit move apart.
	for range 200 {
		moved := false
		for i := range labels {
			if clearNodes(i) {
				moved = true
			}
		}
		for i := range labels {
			for j := i + 1; j < len(labels); j++ {
				if !labels[i].box.hits(labels[j].box, labelGap) {
					continue
				}
				up, down := i, j
				if labels[j].y < labels[i].y {
					up, down = j, i
				}
				need := h + labelGap - (labels[down].y - labels[up].y)
				a := move(up, -need/2)
				b := move(down, need+a)
				c := move(up, -(need + a - b))
				if math.Abs(a)+math.Abs(b)+math.Abs(c) > 1e-9 {
					moved = true
				}
			}
		}
		if !moved {
			break
		}
	}
	// Step 4: a label that still hits loses the end of its text.
	for _, i := range slices.Backward(order) {
		if clashes(i, labels[i].box) {
			shorten(i, clashes)
		}
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		l := labels[i]
		x, align := l.box.x2, "right"
		if l.side > 0 {
			x, align = l.box.x1, "left"
		}
		c[as[0]], c[as[1]], c[as[2]], c[as[3]] = x, l.y, align, l.text
		out[i] = c
	}
	return out, nil
}
