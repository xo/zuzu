package dataflow

import (
	"fmt"
	"math"

	"github.com/xo/zuzu/expr"
)

// layoutTree builds the tree of rows for a layout that sizes its nodes:
// the values of a field summed up the tree, and the children sorted.
func layoutTree(p *params, rows []Row) (*node, error) {
	root, err := buildTree(p.hierarchyOf(), rows)
	if err != nil {
		return nil, err
	}
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	sumNodes(root, field, rows)
	cmp, err := p.sort()
	if err != nil {
		return nil, err
	}
	sortNodes(root, cmp, rows)
	return root, nil
}

// writeRects writes the rectangle, the depth and the count of children of
// each node of a layout to a copy of its row.
func writeRects(root *node, rows []Row, as []string) []Row {
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	eachBefore(root, func(n *node) {
		r := out[n.row]
		r[as[0]], r[as[1]], r[as[2]], r[as[3]] = n.x0, n.y0, n.x1, n.y1
		r[as[4]], r[as[5]] = float64(n.depth), float64(len(n.children))
	})
	return out
}

// dice lays out the children of a node side by side along x, each as wide
// as its share of the value of the node.
func dice(parent *node, x0, y0, x1, y1 float64) {
	k := 0.0
	if parent.value != 0 {
		k = (x1 - x0) / parent.value
	}
	for _, c := range parent.children {
		c.y0, c.y1 = y0, y1
		c.x0 = x0
		x0 += c.value * k
		c.x1 = x0
	}
}

// slice lays out the children of a node one above another along y, each
// as tall as its share of the value of the node.
func slice(parent *node, x0, y0, x1, y1 float64) {
	k := 0.0
	if parent.value != 0 {
		k = (y1 - y0) / parent.value
	}
	for _, c := range parent.children {
		c.x0, c.x1 = x0, x1
		c.y0 = y0
		y0 += c.value * k
		c.y1 = y0
	}
}

// roundRects rounds the edges of each rectangle of a tree to whole
// numbers.
func roundRects(root *node) {
	eachBefore(root, func(n *node) {
		n.x0, n.y0 = math.Round(n.x0), math.Round(n.y0)
		n.x1, n.y1 = math.Round(n.x1), math.Round(n.y1)
	})
}

// partition lays out a tree as adjacent rectangles: each depth is a band
// of the same height, and each node takes the share of the band of its
// parent that its value gives. A sunburst reads x as an angle and y as a
// radius.
func partition(p *params, rows []Row) ([]Row, error) {
	root, err := layoutTree(p, rows)
	if err != nil {
		return nil, err
	}
	dx, dy, _, err := sizeOf(p, "size")
	if err != nil {
		return nil, err
	}
	padding, err := p.number("padding", 0)
	if err != nil {
		return nil, err
	}
	round, err := p.boolean("round", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as("x0", "y0", "x1", "y1", "depth", "children")
	if err != nil {
		return nil, err
	}
	n := float64(root.height + 1)
	root.x0, root.y0 = padding, padding
	root.x1, root.y1 = dx, dy/n
	eachBefore(root, func(v *node) {
		if len(v.children) > 0 {
			d := float64(v.depth)
			dice(v, v.x0, dy*(d+1)/n, v.x1, dy*(d+2)/n)
		}
		x0, y0, x1, y1 := v.x0, v.y0, v.x1-padding, v.y1-padding
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		v.x0, v.y0, v.x1, v.y1 = x0, y0, x1, y1
	})
	if round {
		roundRects(root)
	}
	return writeRects(root, rows, as), nil
}

// goldenRatio is the default aspect ratio of the squarified treemap.
var goldenRatio = (1 + math.Sqrt(5)) / 2

// squarify lays out the children of a node in rows, as the squarified
// treemap of Bruls, Huizing and van Wijk does: a row takes the next
// children while that does not make its worst aspect ratio, against the
// ratio asked for, any worse. A row runs along the shorter side of what is
// left.
func squarify(ratio float64) func(parent *node, x0, y0, x1, y1 float64) {
	return func(parent *node, x0, y0, x1, y1 float64) {
		nodes := parent.children
		left := parent.value
		for i := 0; i < len(nodes); {
			dx, dy := x1-x0, y1-y0
			j := i
			sum := 0.0
			// The row starts with the next child that has a value.
			for j < len(nodes) {
				sum = nodes[j].value
				j++
				if sum != 0 {
					break
				}
			}
			lo, hi := sum, sum
			alpha := math.Max(dy/dx, dx/dy) / (left * ratio)
			beta := sum * sum * alpha
			worst := math.Max(hi/beta, beta/lo)
			for ; j < len(nodes); j++ {
				v := nodes[j].value
				nlo, nhi := math.Min(lo, v), math.Max(hi, v)
				b := (sum + v) * (sum + v) * alpha
				w := math.Max(nhi/b, b/nlo)
				if w > worst {
					break
				}
				sum, lo, hi, worst = sum+v, nlo, nhi, w
			}
			row := &node{value: sum, children: nodes[i:j]}
			if dx < dy {
				y := y1
				if left != 0 {
					y = y0 + dy*sum/left
				}
				dice(row, x0, y0, x1, y)
				y0 = y
			} else {
				x := x1
				if left != 0 {
					x = x0 + dx*sum/left
				}
				slice(row, x0, y0, x, y1)
				x0 = x
			}
			left -= sum
			i = j
		}
	}
}

// binary splits the children of a node in two halves of about the same
// value, again and again, each half along the longer side.
func binary(parent *node, x0, y0, x1, y1 float64) {
	nodes := parent.children
	if len(nodes) == 0 {
		return
	}
	sums := make([]float64, len(nodes)+1)
	for i, c := range nodes {
		sums[i+1] = sums[i] + c.value
	}
	var part func(i, j int, value, x0, y0, x1, y1 float64)
	part = func(i, j int, value, x0, y0, x1, y1 float64) {
		if i >= j-1 {
			n := nodes[i]
			n.x0, n.y0, n.x1, n.y1 = x0, y0, x1, y1
			return
		}
		offset, target := sums[i], value/2+sums[i]
		k, hi := i+1, j-1
		for k < hi {
			mid := (k + hi) / 2
			if sums[mid] < target {
				k = mid + 1
			} else {
				hi = mid
			}
		}
		if target-sums[k-1] < sums[k]-target && i+1 < k {
			k--
		}
		lv := sums[k] - offset
		rv := value - lv
		if x1-x0 > y1-y0 {
			xk := x1
			if value != 0 {
				xk = (x0*rv + x1*lv) / value
			}
			part(i, k, lv, x0, y0, xk, y1)
			part(k, j, rv, xk, y0, x1, y1)
			return
		}
		yk := y1
		if value != 0 {
			yk = (y0*rv + y1*lv) / value
		}
		part(i, k, lv, x0, y0, x1, yk)
		part(k, j, rv, x0, yk, x1, y1)
	}
	part(0, len(nodes), parent.value, x0, y0, x1, y1)
}

// treemap lays out a tree as nested rectangles, each as large as its value,
// by a tiling method.
func treemap(p *params, rows []Row) ([]Row, error) {
	root, err := layoutTree(p, rows)
	if err != nil {
		return nil, err
	}
	dx, dy, _, err := sizeOf(p, "size")
	if err != nil {
		return nil, err
	}
	method, err := p.str("method", "squarify")
	if err != nil {
		return nil, err
	}
	ratio, err := p.number("ratio", goldenRatio)
	if err != nil {
		return nil, err
	}
	round, err := p.boolean("round", false)
	if err != nil {
		return nil, err
	}
	pad := func(name string, def float64) (float64, error) { return p.number(name, def) }
	inner, err := pad("paddingInner", 0)
	if err != nil {
		return nil, err
	}
	outer, err := pad("padding", 0)
	if err != nil {
		return nil, err
	}
	if v, ok, err := p.raw("paddingOuter"); err != nil {
		return nil, err
	} else if ok && v != nil {
		outer = expr.Number(v)
	}
	sides := [4]float64{outer, outer, outer, outer}
	for i, name := range []string{"paddingTop", "paddingRight", "paddingBottom", "paddingLeft"} {
		if sides[i], err = pad(name, outer); err != nil {
			return nil, err
		}
	}
	as, err := p.as("x0", "y0", "x1", "y1", "depth", "children")
	if err != nil {
		return nil, err
	}
	var tile func(parent *node, x0, y0, x1, y1 float64)
	switch method {
	case "squarify", "resquarify":
		tile = squarify(ratio)
	case "binary":
		tile = binary
	case "dice":
		tile = dice
	case "slice":
		tile = slice
	case "slicedice":
		tile = func(parent *node, x0, y0, x1, y1 float64) {
			if parent.depth%2 == 1 {
				slice(parent, x0, y0, x1, y1)
			} else {
				dice(parent, x0, y0, x1, y1)
			}
		}
	default:
		return nil, fmt.Errorf("treemap method %q: %w", method, ErrParam)
	}
	root.x0, root.y0, root.x1, root.y1 = 0, 0, dx, dy
	padStack := map[int]float64{0: 0}
	eachBefore(root, func(n *node) {
		pd := padStack[n.depth]
		x0, y0, x1, y1 := n.x0+pd, n.y0+pd, n.x1-pd, n.y1-pd
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		n.x0, n.y0, n.x1, n.y1 = x0, y0, x1, y1
		if len(n.children) == 0 {
			return
		}
		pd = inner / 2
		padStack[n.depth+1] = pd
		x0 += sides[3] - pd
		y0 += sides[0] - pd
		x1 -= sides[1] - pd
		y1 -= sides[2] - pd
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		tile(n, x0, y0, x1, y1)
	})
	if round {
		roundRects(root)
	}
	return writeRects(root, rows, as), nil
}
