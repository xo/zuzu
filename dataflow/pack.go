package dataflow

import (
	"math"

	"github.com/xo/zuzu/expr"
)

// circle is a circle of a packing.
type circle struct {
	x, y, r float64
}

// tangentTo returns the place of a circle of radius r that touches a and
// b, on the left of the direction from a to b, with y up.
func tangentTo(a, b circle, r float64) (float64, float64) {
	dx, dy := b.x-a.x, b.y-a.y
	d2 := dx*dx + dy*dy
	if d2 == 0 {
		return a.x + a.r + r, a.y
	}
	ra, rb := (a.r+r)*(a.r+r), (b.r+r)*(b.r+r)
	if ra > rb {
		t := (d2 + rb - ra) / (2 * d2)
		h := math.Sqrt(math.Max(0, rb/d2-t*t))
		return b.x - t*dx - h*dy, b.y - t*dy + h*dx
	}
	t := (d2 + ra - rb) / (2 * d2)
	h := math.Sqrt(math.Max(0, ra/d2-t*t))
	return a.x + t*dx - h*dy, a.y + t*dy + h*dx
}

// overlaps reports whether two circles overlap by more than a hair.
func overlaps(a, b circle) bool {
	dr := a.r + b.r - 1e-6
	dx, dy := b.x-a.x, b.y-a.y
	return dr > 0 && dr*dr > dx*dx+dy*dy
}

// packSiblings places circles side by side, without overlap, by the front
// chain of Wang, Wang, Dai and Wang: each circle in turn touches the pair
// of neighbors on the chain of the outer circles whose point of contact is
// the nearest to the origin, and the chain closes over any circle that the
// new one overlaps. It returns the circles, centered on the least circle
// that encloses them, and its radius.
func packSiblings(cs []circle) float64 {
	n := len(cs)
	if n == 0 {
		return 0
	}
	cs[0].x, cs[0].y = 0, 0
	if n == 1 {
		return cs[0].r
	}
	cs[0].x = -cs[1].r
	cs[1].x, cs[1].y = cs[0].r, 0
	if n == 2 {
		return cs[0].r + cs[1].r
	}
	cs[2].x, cs[2].y = tangentTo(cs[0], cs[1], cs[2].r)
	// The front chain, as a ring of indexes.
	next := map[int]int{0: 1, 1: 2, 2: 0}
	prev := map[int]int{1: 0, 2: 1, 0: 2}
	a, b := 0, 1
	score := func(i int) float64 {
		p, q := cs[i], cs[next[i]]
		s := p.r + q.r
		x := (p.x*q.r + q.x*p.r) / s
		y := (p.y*q.r + q.y*p.r) / s
		return x*x + y*y
	}
	for i := 3; i < n; {
		c := &cs[i]
		c.x, c.y = tangentTo(cs[b], cs[a], c.r)
		// Look for an overlap along the chain from both ends of the pair,
		// the nearer end first.
		j, k := next[b], prev[a]
		sj, sk := cs[b].r, cs[a].r
		moved := false
		for {
			if sj <= sk {
				if overlaps(cs[j], *c) {
					b = j
					next[a], prev[b] = b, a
					moved = true
					break
				}
				sj += cs[j].r
				j = next[j]
			} else {
				if overlaps(cs[k], *c) {
					a = k
					next[a], prev[b] = b, a
					moved = true
					break
				}
				sk += cs[k].r
				k = prev[k]
			}
			if j == next[k] {
				break
			}
		}
		if moved {
			continue
		}
		next[a], prev[i] = i, a
		next[i], prev[b] = b, i
		best, bs := a, score(a)
		for p := next[i]; p != i; p = next[p] {
			if s := score(p); s < bs {
				best, bs = p, s
			}
		}
		a, b = best, next[best]
		i++
	}
	e := enclose(cs)
	for i := range cs {
		cs[i].x -= e.x
		cs[i].y -= e.y
	}
	return e.r
}

// encloses reports whether a circle holds another, with room for error.
func encloses(a, b circle) bool {
	dr := a.r - b.r + 1e-9*math.Max(1, math.Max(a.r, b.r))
	dx, dy := b.x-a.x, b.y-a.y
	return dr > 0 && dr*dr > dx*dx+dy*dy
}

// enclose returns the least circle that holds circles: it grows by the
// circles that fall outside it, each on its edge, with one, two or three
// circles on the edge at a time.
func enclose(cs []circle) circle {
	if len(cs) == 0 {
		return circle{}
	}
	e := cs[0]
	for i := 1; i < len(cs); i++ {
		if encloses(e, cs[i]) {
			continue
		}
		e = cs[i]
		for j := range i {
			if encloses(e, cs[j]) {
				continue
			}
			e = encloseTwo(cs[i], cs[j])
			for k := range j {
				if !encloses(e, cs[k]) {
					e = encloseThree(cs[i], cs[j], cs[k])
				}
			}
		}
	}
	return e
}

// encloseTwo returns the least circle that holds two circles.
func encloseTwo(a, b circle) circle {
	dx, dy := b.x-a.x, b.y-a.y
	d := math.Hypot(dx, dy)
	if encloses(a, b) {
		return a
	}
	if encloses(b, a) {
		return b
	}
	r := (d + a.r + b.r) / 2
	t := (r - a.r) / d
	return circle{x: a.x + dx*t, y: a.y + dy*t, r: r}
}

// encloseThree returns the circle that holds three circles and touches
// each from inside, the solution of the problem of Apollonius.
func encloseThree(a, b, c circle) circle {
	// Each pair of the equations (x-xi)² + (y-yi)² = (r-ri)² gives a line
	// in x, y and r.
	a2, b2 := 2*(a.x-b.x), 2*(a.y-b.y)
	c2 := 2 * (b.r - a.r)
	d2 := (b.x*b.x + b.y*b.y - b.r*b.r) - (a.x*a.x + a.y*a.y - a.r*a.r)
	a3, b3 := 2*(a.x-c.x), 2*(a.y-c.y)
	c3 := 2 * (c.r - a.r)
	d3 := (c.x*c.x + c.y*c.y - c.r*c.r) - (a.x*a.x + a.y*a.y - a.r*a.r)
	det := a2*b3 - a3*b2
	if det == 0 {
		return encloseTwo(encloseTwo(a, b), c)
	}
	xa, xb := (b2*d3-b3*d2)/det, (b2*c3-b3*c2)/det
	ya, yb := (a3*d2-a2*d3)/det, (a3*c2-a2*c3)/det
	px, py := xa-a.x, ya-a.y
	qa := xb*xb + yb*yb - 1
	qb := 2 * (px*xb + py*yb + a.r)
	qc := px*px + py*py - a.r*a.r
	var r float64
	if math.Abs(qa) < 1e-12 {
		r = -qc / qb
	} else {
		disc := math.Sqrt(math.Max(0, qb*qb-4*qa*qc))
		r = (-qb - disc) / (2 * qa)
		if r2 := (-qb + disc) / (2 * qa); r < math.Max(a.r, math.Max(b.r, c.r)) || (r2 > r && r2 >= math.Max(a.r, math.Max(b.r, c.r)) && r < 0) {
			r = math.Max(r, r2)
		}
	}
	return circle{x: xa + xb*r, y: ya + yb*r, r: r}
}

// pack lays out a tree as nested circles: the circles of the leaves have
// areas as their values, the children of a node pack inside its circle, and
// the root fills the size.
func pack(p *params, rows []Row) ([]Row, error) {
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
	radius, err := p.field("radius")
	if err != nil {
		return nil, err
	}
	as, err := p.as("x", "y", "r", "depth", "children")
	if err != nil {
		return nil, err
	}
	eachBefore(root, func(n *node) {
		if len(n.children) > 0 {
			return
		}
		if radius != nil {
			n.r = math.Max(0, expr.Number(radius.Get(rows[n.row])))
		} else {
			n.r = math.Sqrt(n.value)
		}
	})
	packChildren := func(pad float64) {
		eachAfter(root, func(n *node) {
			if len(n.children) == 0 {
				return
			}
			cs := make([]circle, len(n.children))
			for i, c := range n.children {
				cs[i] = circle{r: c.r + pad}
			}
			r := packSiblings(cs)
			for i, c := range n.children {
				c.x, c.y = cs[i].x, cs[i].y
			}
			n.r = r
		})
	}
	packChildren(0)
	if padding > 0 {
		// The padding scales with the size, so it is in pixels at the end.
		packChildren(padding * root.r / math.Min(dx, dy))
	}
	k := math.Min(dx, dy) / (2 * root.r)
	root.x, root.y = dx/2, dy/2
	eachBefore(root, func(n *node) {
		n.r *= k
		if n.parent != nil {
			n.x = n.parent.x + k*n.x
			n.y = n.parent.y + k*n.y
		}
	})
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	eachBefore(root, func(n *node) {
		r := out[n.row]
		r[as[0]], r[as[1]], r[as[2]] = n.x, n.y, n.r
		r[as[3]], r[as[4]] = float64(n.depth), float64(len(n.children))
	})
	return out, nil
}
