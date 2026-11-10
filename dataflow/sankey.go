package dataflow

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
)

// extentOf reads the parameter extent, [[x0, y0], [x1, y1]], or else the
// parameter size, [width, height], from the origin.
func (p *params) extentOf() (x0, y0, x1, y1 float64, err error) {
	if v, ok, err := p.raw("extent"); err != nil {
		return 0, 0, 0, 0, err
	} else if ok && v != nil {
		l, _ := v.([]any)
		if len(l) == 2 {
			a, _ := l[0].([]any)
			b, _ := l[1].([]any)
			if len(a) == 2 && len(b) == 2 {
				return expr.Number(a[0]), expr.Number(a[1]), expr.Number(b[0]), expr.Number(b[1]), nil
			}
		}
		return 0, 0, 0, 0, fmt.Errorf("the extent %v: %w", v, ErrParam)
	}
	size, err := p.numbers("size")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if len(size) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("an extent or a size is required: %w", ErrParam)
	}
	return 0, 0, size[0], size[1], nil
}

// flowFields reads the fields source, target and value of a dataset of
// links.
func (p *params) flowFields() (src, dst, val *Accessor, err error) {
	read := func(name string) (*Accessor, error) {
		a, err := p.field(name)
		if err != nil || a != nil {
			return a, err
		}
		return FieldAccessor(name), nil
	}
	if src, err = read("source"); err != nil {
		return nil, nil, nil, err
	}
	if dst, err = read("target"); err != nil {
		return nil, nil, nil, err
	}
	val, err = read("value")
	return src, dst, val, err
}

// sankeyNode is a node of a sankey while the layout places it.
type sankeyNode struct {
	key          any
	order        int
	in, out      []*sankeyLink
	value        float64
	depth, layer int
	height       int
	x0, x1       float64
	y0, y1       float64
}

type sankeyLink struct {
	src, dst *sankeyNode
	value    float64
}

// sankeyGraph reads the nodes and the links of a dataset of links. The
// nodes come in the order in which each first appears.
func sankeyGraph(rows []Row, src, dst, val *Accessor) ([]*sankeyNode, []*sankeyLink) {
	byKey := map[string]*sankeyNode{}
	var nodes []*sankeyNode
	node := func(v any) *sankeyNode {
		k := key([]any{v})
		n, ok := byKey[k]
		if !ok {
			n = &sankeyNode{key: v, order: len(nodes)}
			byKey[k] = n
			nodes = append(nodes, n)
		}
		return n
	}
	links := make([]*sankeyLink, 0, len(rows))
	for _, r := range rows {
		s, t := node(src.Get(r)), node(dst.Get(r))
		v := expr.Number(val.Get(r))
		if math.IsNaN(v) || v < 0 {
			v = 0
		}
		l := &sankeyLink{src: s, dst: t, value: v}
		s.out = append(s.out, l)
		t.in = append(t.in, l)
		links = append(links, l)
	}
	for _, n := range nodes {
		var in, out float64
		for _, l := range n.in {
			in += l.value
		}
		for _, l := range n.out {
			out += l.value
		}
		n.value = math.Max(in, out)
	}
	return nodes, links
}

// sankey lays out the nodes of a dataset of links in columns: each node in
// the column of its longest path from a node with no links in, as tall as
// its value, and moved toward the middle of the nodes it links to. It
// makes a row for each node.
func sankey(p *params, rows []Row) ([]Row, error) {
	src, dst, val, err := p.flowFields()
	if err != nil {
		return nil, err
	}
	x0, y0, x1, y1, err := p.extentOf()
	if err != nil {
		return nil, err
	}
	nodeWidth, err := p.number("nodeWidth", 24)
	if err != nil {
		return nil, err
	}
	pad, err := p.number("nodePadding", 8)
	if err != nil {
		return nil, err
	}
	align, err := p.str("align", "justify")
	if err != nil {
		return nil, err
	}
	iterations, err := p.number("iterations", 6)
	if err != nil {
		return nil, err
	}
	bySize := true
	if v, ok, err := p.raw("nodeSort"); err != nil {
		return nil, err
	} else if ok && v == nil {
		bySize = false
	}
	as, err := p.as("key", "x0", "x1", "y0", "y1", "value", "depth", "layer")
	if err != nil {
		return nil, err
	}
	nodes, _ := sankeyGraph(rows, src, dst, val)
	if err := sankeyColumns(nodes, align); err != nil {
		return nil, err
	}
	columns := 0
	for _, n := range nodes {
		columns = max(columns, n.layer+1)
	}
	kx := 0.0
	if columns > 1 {
		kx = (x1 - x0 - nodeWidth) / float64(columns-1)
	}
	cols := make([][]*sankeyNode, columns)
	for _, n := range nodes {
		n.x0 = x0 + float64(n.layer)*kx
		n.x1 = n.x0 + nodeWidth
		cols[n.layer] = append(cols[n.layer], n)
	}
	if bySize {
		for _, c := range cols {
			slices.SortStableFunc(c, func(a, b *sankeyNode) int { return cmp.Compare(b.value, a.value) })
		}
	}
	sankeyBreadths(cols, y0, y1, pad, int(iterations), bySize)
	out := make([]Row, len(nodes))
	for i, n := range nodes {
		out[i] = Row{
			as[0]: n.key, as[1]: n.x0, as[2]: n.x1, as[3]: n.y0, as[4]: n.y1,
			as[5]: n.value, as[6]: float64(n.depth), as[7]: float64(n.layer),
		}
	}
	return out, nil
}

// sankeyColumns works out the depth of each node, its longest path from a
// node with no links in, its height, the longest path to a node with no
// links out, and its column by the alignment. A cycle is an error.
func sankeyColumns(nodes []*sankeyNode, align string) error {
	walk := func(start []*sankeyNode, next func(*sankeyNode) []*sankeyNode, set func(*sankeyNode, int)) error {
		current := start
		for x := 0; len(current) > 0; x++ {
			if x > len(nodes) {
				return fmt.Errorf("zuzu.sankey: the links form a cycle: %w", ErrParam)
			}
			var after []*sankeyNode
			seen := map[*sankeyNode]bool{}
			for _, n := range current {
				set(n, x)
				for _, m := range next(n) {
					if !seen[m] {
						seen[m] = true
						after = append(after, m)
					}
				}
			}
			current = after
		}
		return nil
	}
	targets := func(n *sankeyNode) []*sankeyNode {
		out := make([]*sankeyNode, len(n.out))
		for i, l := range n.out {
			out[i] = l.dst
		}
		return out
	}
	sources := func(n *sankeyNode) []*sankeyNode {
		out := make([]*sankeyNode, len(n.in))
		for i, l := range n.in {
			out[i] = l.src
		}
		return out
	}
	if err := walk(nodes, targets, func(n *sankeyNode, x int) { n.depth = x }); err != nil {
		return err
	}
	if err := walk(nodes, sources, func(n *sankeyNode, x int) { n.height = x }); err != nil {
		return err
	}
	last := 0
	for _, n := range nodes {
		last = max(last, n.depth)
	}
	for _, n := range nodes {
		switch align {
		case "left":
			n.layer = n.depth
		case "right":
			n.layer = last - n.height
		case "center":
			switch {
			case len(n.in) > 0:
				n.layer = n.depth
			case len(n.out) > 0:
				d := math.MaxInt
				for _, l := range n.out {
					d = min(d, l.dst.depth)
				}
				n.layer = d - 1
			}
		case "justify":
			n.layer = n.depth
			if len(n.out) == 0 {
				n.layer = last
			}
		default:
			return fmt.Errorf("zuzu.sankey with the align %q: %w", align, ErrParam)
		}
	}
	return nil
}

// sankeyBreadths places the nodes of each column from the top, at a scale
// that fits the fullest column, then moves each node toward the weighted
// middle of the nodes it links to, from right to left and back, with less
// movement on each pass, and keeps the nodes from overlapping.
func sankeyBreadths(cols [][]*sankeyNode, y0, y1, pad float64, iterations int, sorted bool) {
	ky := math.Inf(1)
	for _, c := range cols {
		sum := 0.0
		for _, n := range c {
			sum += n.value
		}
		if sum > 0 {
			ky = math.Min(ky, (y1-y0-float64(len(c)-1)*pad)/sum)
		}
	}
	if math.IsInf(ky, 1) {
		ky = 0
	}
	for _, c := range cols {
		y := y0
		for _, n := range c {
			n.y0, n.y1 = y, y+n.value*ky
			y = n.y1 + pad
		}
		// The column sits in the middle of the extent.
		shift := (y1 - (y - pad)) / 2
		for _, n := range c {
			n.y0 += shift
			n.y1 += shift
		}
	}
	middle := func(n *sankeyNode) float64 { return (n.y0 + n.y1) / 2 }
	relax := func(c []*sankeyNode, links func(*sankeyNode) []*sankeyLink, other func(*sankeyLink) *sankeyNode, alpha float64) {
		for _, n := range c {
			var y, w float64
			for _, l := range links(n) {
				y += middle(other(l)) * l.value
				w += l.value
			}
			if w > 0 {
				dy := (y/w - middle(n)) * alpha
				n.y0 += dy
				n.y1 += dy
			}
		}
		if sorted {
			slices.SortStableFunc(c, func(a, b *sankeyNode) int { return cmp.Compare(a.y0, b.y0) })
		}
		collide(c, y0, y1, pad)
	}
	in := func(n *sankeyNode) []*sankeyLink { return n.in }
	out := func(n *sankeyNode) []*sankeyLink { return n.out }
	src := func(l *sankeyLink) *sankeyNode { return l.src }
	dst := func(l *sankeyLink) *sankeyNode { return l.dst }
	for i := range iterations {
		alpha := math.Pow(0.99, float64(i))
		for j := len(cols) - 2; j >= 0; j-- {
			relax(cols[j], out, dst, alpha)
		}
		for j := 1; j < len(cols); j++ {
			relax(cols[j], in, src, alpha)
		}
	}
}

// collide moves the nodes of a column apart, down from the top and then
// up from the bottom, so that they keep the padding inside the extent.
func collide(c []*sankeyNode, y0, y1, pad float64) {
	y := y0
	for _, n := range c {
		if dy := y - n.y0; dy > 0 {
			n.y0 += dy
			n.y1 += dy
		}
		y = n.y1 + pad
	}
	y = y1
	for _, n := range slices.Backward(c) {
		if dy := n.y1 - y; dy > 0 {
			n.y0 -= dy
			n.y1 -= dy
		}
		y = n.y0 - pad
	}
}

// sankeylinks places the links of a sankey from its nodes: each link is a
// band as wide as its value, which leaves the right edge of its source and
// reaches the left edge of its target. The links at a node stack in the
// order of the other end, from the top.
func sankeylinks(p *params, rows []Row) ([]Row, error) {
	src, dst, val, err := p.flowFields()
	if err != nil {
		return nil, err
	}
	name, err := p.str("nodes", "")
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("zuzu.sankeylinks needs nodes: %w", ErrParam)
	}
	nodeRows, err := p.env.Data(name)
	if err != nil {
		return nil, err
	}
	nodeKey := FieldAccessor("key")
	if k, err := p.field("nodeKey"); err != nil {
		return nil, err
	} else if k != nil {
		nodeKey = k
	}
	byOrder := false
	if v, ok, err := p.raw("linkSort"); err != nil {
		return nil, err
	} else if ok && v == nil {
		byOrder = true
	}
	as, err := p.as("x0", "x1", "y0", "y1", "width", "path")
	if err != nil {
		return nil, err
	}
	type box struct{ x0, x1, y0, y1, value, ky float64 }
	boxes := map[string]*box{}
	for _, r := range nodeRows {
		get := func(f string) float64 { return expr.Number(r[f]) }
		b := &box{x0: get("x0"), x1: get("x1"), y0: get("y0"), y1: get("y1"), value: get("value")}
		if b.value > 0 {
			b.ky = (b.y1 - b.y0) / b.value
		}
		boxes[key([]any{nodeKey.Get(r)})] = b
	}
	type end struct {
		row   int
		other *box
		width float64
	}
	outs, ins := map[*box][]*end{}, map[*box][]*end{}
	sb := make([]*box, len(rows))
	tb := make([]*box, len(rows))
	widths := make([]float64, len(rows))
	for i, r := range rows {
		s, t := boxes[key([]any{src.Get(r)})], boxes[key([]any{dst.Get(r)})]
		if s == nil || t == nil {
			return nil, fmt.Errorf("zuzu.sankeylinks: a link names a node that %s does not hold: %w", name, ErrParam)
		}
		v := expr.Number(val.Get(r))
		if math.IsNaN(v) || v < 0 {
			v = 0
		}
		sb[i], tb[i], widths[i] = s, t, v*s.ky
		outs[s] = append(outs[s], &end{row: i, other: t, width: widths[i]})
		ins[t] = append(ins[t], &end{row: i, other: s, width: widths[i]})
	}
	ys, yt := make([]float64, len(rows)), make([]float64, len(rows))
	stack := func(m map[*box][]*end, into []float64) {
		for b, ends := range m {
			if !byOrder {
				slices.SortStableFunc(ends, func(a, c *end) int {
					if d := cmp.Compare(a.other.y0, c.other.y0); d != 0 {
						return d
					}
					return cmp.Compare(a.row, c.row)
				})
			}
			y := b.y0
			for _, e := range ends {
				into[e.row] = y + e.width/2
				y += e.width
			}
		}
	}
	stack(outs, ys)
	stack(ins, yt)
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		x0, x1 := sb[i].x1, tb[i].x0
		w := widths[i]
		c[as[0]], c[as[1]], c[as[2]], c[as[3]], c[as[4]] = x0, x1, ys[i], yt[i], w
		c[as[5]] = bandPath(x0, ys[i], x1, yt[i], w)
		out[i] = c
	}
	return out, nil
}

// bandPath returns a band of a width from (x0, y0) to (x1, y1), which
// leaves and arrives level and curves between, as a closed shape.
func bandPath(x0, y0, x1, y1, w float64) string {
	xm := (x0 + x1) / 2
	h := w / 2
	pt := func(x, y float64) string { return num(x) + "," + num(y) }
	var b strings.Builder
	b.WriteString("M" + pt(x0, y0-h))
	b.WriteString("C" + pt(xm, y0-h) + " " + pt(xm, y1-h) + " " + pt(x1, y1-h))
	b.WriteString("L" + pt(x1, y1+h))
	b.WriteString("C" + pt(xm, y1+h) + " " + pt(xm, y0+h) + " " + pt(x0, y0+h))
	b.WriteString("Z")
	return b.String()
}
