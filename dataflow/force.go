package dataflow

import (
	"fmt"
	"math"
	"reflect"

	"github.com/xo/zuzu/expr"
)

// forceNode is a node of a force simulation: its place and its velocity.
type forceNode struct {
	row          Row
	x, y, vx, vy float64
	fx, fy       float64
	fixed        [2]bool
}

// forceFunc moves the nodes of a simulation for one tick, at a strength of
// alpha.
type forceFunc func(alpha float64)

// lcg is the generator of numbers from 0 to 1 that a simulation uses to
// split nodes that lie on the same place: a linear congruential generator
// with the constants of Numerical Recipes, from the seed 1. It is the same
// on every run, so the layout is too.
type lcg struct{ s uint64 }

func (l *lcg) next() float64 {
	l.s = (1664525*l.s + 1013904223) % (1 << 32)
	return float64(l.s) / (1 << 32)
}

// jiggle returns a tiny random distance.
func (l *lcg) jiggle() float64 { return (l.next() - 0.5) * 1e-6 }

// force runs a simulation of forces on the rows, as nodes, and writes the
// place and the velocity of each: x, y, vx and vy. The nodes start where
// their rows place them, or on a spiral of sunflower seeds around the
// origin. Each tick, the forces add to the velocities, the velocities
// decay, and the nodes move, while alpha, the strength of the forces,
// decays from 1 toward its target. zuzu draws a still image, so the
// simulation runs its iterations at once, as the static mode of Vega does.
// The forces are center, collide, nbody, link, x and y, with the
// parameters of Vega.
func force(p *params, rows []Row) ([]Row, error) {
	iterations, err := p.number("iterations", 300)
	if err != nil {
		return nil, err
	}
	alpha, err := p.number("alpha", 1)
	if err != nil {
		return nil, err
	}
	alphaMin, err := p.number("alphaMin", 0.001)
	if err != nil {
		return nil, err
	}
	alphaTarget, err := p.number("alphaTarget", 0)
	if err != nil {
		return nil, err
	}
	decay, err := p.number("velocityDecay", 0.4)
	if err != nil {
		return nil, err
	}
	as, err := p.as("x", "y", "vx", "vy")
	if err != nil {
		return nil, err
	}
	nodes := make([]*forceNode, len(rows))
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		out[i] = c
		n := &forceNode{row: c, x: numOrNaN(c[as[0]]), y: numOrNaN(c[as[1]]), vx: numOrNaN(c[as[2]]), vy: numOrNaN(c[as[3]])}
		if v, ok := c["fx"]; ok && v != nil && !expr.IsUndefined(v) {
			n.fx, n.fixed[0] = expr.Number(v), true
			n.x = n.fx
		}
		if v, ok := c["fy"]; ok && v != nil && !expr.IsUndefined(v) {
			n.fy, n.fixed[1] = expr.Number(v), true
			n.y = n.fy
		}
		if math.IsNaN(n.x) || math.IsNaN(n.y) {
			// The spiral of sunflower seeds spreads the nodes evenly.
			radius := 10 * math.Sqrt(0.5+float64(i))
			angle := float64(i) * math.Pi * (3 - math.Sqrt(5))
			n.x, n.y = radius*math.Cos(angle), radius*math.Sin(angle)
		}
		if math.IsNaN(n.vx) || math.IsNaN(n.vy) {
			n.vx, n.vy = 0, 0
		}
		nodes[i] = n
	}
	rnd := &lcg{s: 1}
	list, err := p.list("forces")
	if err != nil {
		return nil, err
	}
	var forces []forceFunc
	for i, f := range list {
		spec, ok := f.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("the force %d: %w", i, ErrParam)
		}
		ff, err := p.makeForce(spec, nodes, rnd)
		if err != nil {
			return nil, fmt.Errorf("the force %d: %w", i, err)
		}
		forces = append(forces, ff)
	}
	alphaDecay := 1 - math.Pow(alphaMin, 1.0/300)
	velocity := 1 - decay
	for range int(iterations) {
		alpha += (alphaTarget - alpha) * alphaDecay
		for _, f := range forces {
			f(alpha)
		}
		for _, n := range nodes {
			if n.fixed[0] {
				n.x, n.vx = n.fx, 0
			} else {
				n.vx *= velocity
				n.x += n.vx
			}
			if n.fixed[1] {
				n.y, n.vy = n.fy, 0
			} else {
				n.vy *= velocity
				n.y += n.vy
			}
		}
	}
	for _, n := range nodes {
		n.row[as[0]], n.row[as[1]], n.row[as[2]], n.row[as[3]] = n.x, n.y, n.vx, n.vy
	}
	if name, err := p.str("signal", ""); err != nil {
		return nil, err
	} else if name != "" {
		p.env.SetSignal(name, true)
	}
	return out, nil
}

// numOrNaN reads a number, with NaN for a value that is not one.
func numOrNaN(v any) float64 {
	if v == nil || expr.IsUndefined(v) {
		return math.NaN()
	}
	return expr.Number(v)
}

// nodeValue reads a parameter of a force for each node: a constant, a
// field or an expression.
func (p *params) nodeValue(spec map[string]any, name string, def float64, nodes []*forceNode) ([]float64, error) {
	out := make([]float64, len(nodes))
	v, ok := spec[name]
	if ok {
		r, _, err := resolve(p, v)
		if err != nil {
			return nil, err
		}
		v = r
	}
	switch x := v.(type) {
	case nil:
		for i := range out {
			out[i] = def
		}
	case string, map[string]any:
		a, err := p.accessor(x)
		if err != nil {
			return nil, err
		}
		for i, n := range nodes {
			out[i] = expr.Number(a.Get(n.row))
		}
	default:
		f := expr.Number(x)
		for i := range out {
			out[i] = f
		}
	}
	return out, nil
}

// constant reads a parameter of a force that is the same for every node.
func (p *params) constant(spec map[string]any, name string, def float64) (float64, error) {
	v, ok := spec[name]
	if !ok || v == nil {
		return def, nil
	}
	r, _, err := resolve(p, v)
	if err != nil {
		return 0, err
	}
	if r == nil {
		return def, nil
	}
	return expr.Number(r), nil
}

// makeForce returns a force from its parameters.
func (p *params) makeForce(spec map[string]any, nodes []*forceNode, rnd *lcg) (forceFunc, error) {
	kind, _ := spec["force"].(string)
	switch kind {
	case "center":
		cx, err := p.constant(spec, "x", 0)
		if err != nil {
			return nil, err
		}
		cy, err := p.constant(spec, "y", 0)
		if err != nil {
			return nil, err
		}
		strength, err := p.constant(spec, "strength", 1)
		if err != nil {
			return nil, err
		}
		return centerForce(nodes, cx, cy, strength), nil
	case "collide":
		radii, err := p.nodeValue(spec, "radius", 1, nodes)
		if err != nil {
			return nil, err
		}
		strength, err := p.constant(spec, "strength", 0.7)
		if err != nil {
			return nil, err
		}
		iters, err := p.constant(spec, "iterations", 1)
		if err != nil {
			return nil, err
		}
		return collideForce(nodes, radii, strength, int(iters), rnd), nil
	case "nbody":
		strengths, err := p.nodeValue(spec, "strength", -30, nodes)
		if err != nil {
			return nil, err
		}
		dmin, err := p.constant(spec, "distanceMin", 1)
		if err != nil {
			return nil, err
		}
		dmax, err := p.constant(spec, "distanceMax", math.Inf(1))
		if err != nil {
			return nil, err
		}
		return nbodyForce(nodes, strengths, dmin*dmin, dmax*dmax, rnd), nil
	case "link":
		return p.linkForce(spec, nodes, rnd)
	case "x", "y":
		targets, err := p.nodeValue(spec, kind, 0, nodes)
		if err != nil {
			return nil, err
		}
		strengths, err := p.nodeValue(spec, "strength", 0.1, nodes)
		if err != nil {
			return nil, err
		}
		if kind == "x" {
			return func(alpha float64) {
				for i, n := range nodes {
					n.vx += (targets[i] - n.x) * strengths[i] * alpha
				}
			}, nil
		}
		return func(alpha float64) {
			for i, n := range nodes {
				n.vy += (targets[i] - n.y) * strengths[i] * alpha
			}
		}, nil
	}
	return nil, fmt.Errorf("the kind %q: %w", kind, ErrParam)
}

// centerForce moves all the nodes so that their mean lies at the center.
func centerForce(nodes []*forceNode, cx, cy, strength float64) forceFunc {
	return func(float64) {
		if len(nodes) == 0 {
			return
		}
		sx, sy := 0.0, 0.0
		for _, n := range nodes {
			sx += n.x
			sy += n.y
		}
		k := float64(len(nodes))
		sx = (sx/k - cx) * strength
		sy = (sy/k - cy) * strength
		for _, n := range nodes {
			n.x -= sx
			n.y -= sy
		}
	}
}

// collideForce pushes apart the nodes whose circles overlap, at their next
// places, each by a share of the overlap that grows with the area of the
// other.
func collideForce(nodes []*forceNode, radii []float64, strength float64, iterations int, rnd *lcg) forceFunc {
	return func(float64) {
		for range iterations {
			for i, a := range nodes {
				ri := radii[i]
				ri2 := ri * ri
				xi, yi := a.x+a.vx, a.y+a.vy
				for j := i + 1; j < len(nodes); j++ {
					b := nodes[j]
					rj := radii[j]
					r := ri + rj
					x, y := xi-b.x-b.vx, yi-b.y-b.vy
					l := x*x + y*y
					if l >= r*r {
						continue
					}
					if x == 0 {
						x = rnd.jiggle()
						l += x * x
					}
					if y == 0 {
						y = rnd.jiggle()
						l += y * y
					}
					l = math.Sqrt(l)
					l = (r - l) / l * strength
					x, y = x*l, y*l
					rj2 := rj * rj
					share := rj2 / (ri2 + rj2)
					a.vx += x * share
					a.vy += y * share
					b.vx -= x * (1 - share)
					b.vy -= y * (1 - share)
				}
			}
		}
	}
}

// nbodyForce makes the nodes attract, with a positive strength, or repel
// each other, with a negative one, by the inverse of their distance. The
// sum runs over every pair, the exact form of the approximation of Barnes
// and Hut.
func nbodyForce(nodes []*forceNode, strengths []float64, dmin2, dmax2 float64, rnd *lcg) forceFunc {
	return func(alpha float64) {
		for _, a := range nodes {
			for j, b := range nodes {
				if a == b {
					continue
				}
				x, y := b.x-a.x, b.y-a.y
				l := x*x + y*y
				if l >= dmax2 {
					continue
				}
				if x == 0 {
					x = rnd.jiggle()
					l += x * x
				}
				if y == 0 {
					y = rnd.jiggle()
					l += y * y
				}
				if l < dmin2 {
					l = math.Sqrt(dmin2 * l)
				}
				w := strengths[j] * alpha / l
				a.vx += x * w
				a.vy += y * w
			}
		}
	}
}

// linkForce pulls the two nodes of each link toward a distance. A link
// names its nodes by their index, or by the field id of the nodes, and the
// force writes the nodes into the rows of the links, so that a mark of the
// links can read their places.
func (p *params) linkForce(spec map[string]any, nodes []*forceNode, rnd *lcg) (forceFunc, error) {
	name, _ := spec["links"].(string)
	links, err := p.env.Data(name)
	if err != nil {
		return nil, err
	}
	var id *Accessor
	if v, ok := spec["id"]; ok && v != nil {
		if id, err = p.accessor(v); err != nil {
			return nil, err
		}
	}
	byID := map[string]int{}
	for i, n := range nodes {
		k := key([]any{float64(i)})
		if id != nil {
			k = key([]any{id.Get(n.row)})
		}
		byID[k] = i
	}
	find := func(v any) (int, bool) {
		if m, ok := v.(map[string]any); ok {
			for i, n := range nodes {
				if sameRow(n.row, m) {
					return i, true
				}
			}
			if id != nil {
				v = id.Get(m)
			}
		}
		if id == nil {
			v = expr.Number(v)
		}
		i, ok := byID[key([]any{v})]
		return i, ok
	}
	type link struct {
		s, t      int
		dist, str float64
		bias      float64
	}
	var ls []*link
	count := make([]float64, len(nodes))
	for _, r := range links {
		s, ok1 := find(r["source"])
		t, ok2 := find(r["target"])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("a link of %s names a node that is missing: %w", name, ErrParam)
		}
		ls = append(ls, &link{s: s, t: t})
		count[s]++
		count[t]++
		// The rows of the links hold the nodes, as in Vega.
		r["source"], r["target"] = nodes[s].row, nodes[t].row
	}
	linkRows := make([]Row, len(ls))
	for i := range ls {
		linkRows[i] = links[i]
	}
	dists, err := p.linkValue(spec, "distance", 30, linkRows)
	if err != nil {
		return nil, err
	}
	strs, err := p.linkValue(spec, "strength", math.NaN(), linkRows)
	if err != nil {
		return nil, err
	}
	for i, l := range ls {
		l.dist = dists[i]
		l.str = strs[i]
		if math.IsNaN(l.str) {
			l.str = 1 / math.Min(count[l.s], count[l.t])
		}
		l.bias = count[l.s] / (count[l.s] + count[l.t])
	}
	iters, err := p.constant(spec, "iterations", 1)
	if err != nil {
		return nil, err
	}
	return func(alpha float64) {
		for range int(iters) {
			for _, l := range ls {
				s, t := nodes[l.s], nodes[l.t]
				x := t.x + t.vx - s.x - s.vx
				if x == 0 {
					x = rnd.jiggle()
				}
				y := t.y + t.vy - s.y - s.vy
				if y == 0 {
					y = rnd.jiggle()
				}
				d := math.Sqrt(x*x + y*y)
				d = (d - l.dist) / d * alpha * l.str
				x, y = x*d, y*d
				t.vx -= x * l.bias
				t.vy -= y * l.bias
				s.vx += x * (1 - l.bias)
				s.vy += y * (1 - l.bias)
			}
		}
	}, nil
}

// linkValue reads a parameter of a link force for each link.
func (p *params) linkValue(spec map[string]any, name string, def float64, links []Row) ([]float64, error) {
	nodes := make([]*forceNode, len(links))
	for i, r := range links {
		nodes[i] = &forceNode{row: r}
	}
	return p.nodeValue(spec, name, def, nodes)
}

// sameRow reports whether two rows are the same map.
func sameRow(a, b map[string]any) bool {
	return reflect.ValueOf(a).UnsafePointer() == reflect.ValueOf(b).UnsafePointer()
}
