package mermaid

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// The geometry of the circles of a venn diagram. Mermaid 12.3 lays a venn
// diagram out with venn.js, and zuzu follows its rules: a circle has the
// area of the size of its set, two circles stand at the distance that
// gives the size of their union, a greedy placement puts each circle in
// turn, and Nelder and Mead's method of the simplex then moves the circles
// so that each overlap comes as near its size as it can.

// vennSmall is the tolerance of the geometry of a venn diagram.
const vennSmall = 1e-10

// vennCircle is the circle of a set: its id, its center, its radius and
// the size of its set.
type vennCircle struct {
	id      string
	x, y, r float64
	size    float64
}

// vennPoint is a point, and the indexes of the two circles that cross at
// it, if any.
type vennPoint struct {
	x, y   float64
	parent [2]int
	angle  float64
}

// vennArc is an arc of the border of an overlap: the index of its circle,
// its two ends and its width, and whether it is the large arc.
type vennArc struct {
	circle int
	p1, p2 vennPoint
	width  float64
	large  bool
}

// vennSize is an area that the layout fits: the ids of its sets and its
// size.
type vennSize struct {
	sets   []string
	size   float64
	weight float64
}

// vennDist returns the distance between two points.
func vennDist(ax, ay, bx, by float64) float64 {
	return math.Sqrt((ax-bx)*(ax-bx) + (ay-by)*(ay-by))
}

// vennSegment returns the area of the segment of a circle of radius r
// whose height is w.
func vennSegment(r, w float64) float64 {
	return r*r*math.Acos(1-w/r) - (r-w)*math.Sqrt(w*(2*r-w))
}

// vennOverlap returns the area of the overlap of two circles of radii r1
// and r2 at a distance d.
func vennOverlap(r1, r2, d float64) float64 {
	if d >= r1+r2 {
		return 0
	}
	if d <= math.Abs(r1-r2) {
		return math.Pi * math.Min(r1, r2) * math.Min(r1, r2)
	}
	w1 := r1 - (d*d-r2*r2+r1*r1)/(2*d)
	w2 := r2 - (d*d-r1*r1+r2*r2)/(2*d)
	return vennSegment(r1, w1) + vennSegment(r2, w2)
}

// vennCross returns the points where two circles cross, none when they
// do not touch or when one holds the other.
func vennCross(ax, ay, ar, bx, by, br float64) []vennPoint {
	d := vennDist(ax, ay, bx, by)
	if d >= ar+br || d <= math.Abs(ar-br) {
		return nil
	}
	a := (ar*ar - br*br + d*d) / (2 * d)
	h := math.Sqrt(ar*ar - a*a)
	x0 := ax + a*(bx-ax)/d
	y0 := ay + a*(by-ay)/d
	rx := -(by - ay) * (h / d)
	ry := -(bx - ax) * (h / d)
	return []vennPoint{{x: x0 + rx, y: y0 - ry}, {x: x0 - rx, y: y0 + ry}}
}

// vennIntersection returns the area that all the circles share, and the
// arcs of its border.
func vennIntersection(cs []vennCircle) (float64, []vennArc) {
	var points []vennPoint
	for i := range cs {
		for j := i + 1; j < len(cs); j++ {
			for _, p := range vennCross(cs[i].x, cs[i].y, cs[i].r, cs[j].x, cs[j].y, cs[j].r) {
				p.parent = [2]int{i, j}
				points = append(points, p)
			}
		}
	}
	var inner []vennPoint
	for _, p := range points {
		in := true
		for _, c := range cs {
			if vennDist(p.x, p.y, c.x, c.y) >= c.r+vennSmall {
				in = false
				break
			}
		}
		if in {
			inner = append(inner, p)
		}
	}
	arcArea, polygon := 0.0, 0.0
	var arcs []vennArc
	if len(inner) > 1 {
		cx, cy := vennMean(inner)
		for i := range inner {
			inner[i].angle = math.Atan2(inner[i].x-cx, inner[i].y-cy)
		}
		slices.SortStableFunc(inner, func(a, b vennPoint) int { return vennCmp(b.angle - a.angle) })
		p2 := inner[len(inner)-1]
		for _, p1 := range inner {
			polygon += (p2.x + p1.x) * (p1.y - p2.y)
			mx, my := (p1.x+p2.x)/2, (p1.y+p2.y)/2
			var arc *vennArc
			for _, k := range p1.parent {
				if k != p2.parent[0] && k != p2.parent[1] {
					continue
				}
				c := cs[k]
				a1 := math.Atan2(p1.x-c.x, p1.y-c.y)
				a2 := math.Atan2(p2.x-c.x, p2.y-c.y)
				diff := a2 - a1
				if diff < 0 {
					diff += 2 * math.Pi
				}
				a := a2 - diff/2
				w := vennDist(mx, my, c.x+c.r*math.Sin(a), c.y+c.r*math.Cos(a))
				w = math.Min(w, c.r*2)
				if arc == nil || arc.width > w {
					arc = &vennArc{circle: k, width: w, p1: p1, p2: p2, large: w > c.r}
				}
			}
			if arc != nil {
				arcs = append(arcs, *arc)
				arcArea += vennSegment(cs[arc.circle].r, arc.width)
				p2 = p1
			}
		}
	} else {
		// The circles are apart, or the smallest is inside all the
		// others.
		small := 0
		for i := range cs {
			if cs[i].r < cs[small].r {
				small = i
			}
		}
		s := cs[small]
		for _, c := range cs {
			if vennDist(c.x, c.y, s.x, s.y) > math.Abs(s.r-c.r) {
				return 0, nil
			}
		}
		arcArea = s.r * s.r * math.Pi
		arcs = append(arcs, vennArc{
			circle: small, width: s.r * 2, large: true,
			p1: vennPoint{x: s.x, y: s.y + s.r}, p2: vennPoint{x: s.x - vennSmall, y: s.y + s.r},
		})
	}
	return arcArea + polygon/2, arcs
}

// vennMean returns the mean of points.
func vennMean(ps []vennPoint) (float64, float64) {
	x, y := 0.0, 0.0
	for _, p := range ps {
		x += p.x
		y += p.y
	}
	return x / float64(len(ps)), y / float64(len(ps))
}

// vennCmp turns a difference into the result of a comparison.
func vennCmp(d float64) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}

// vennBisect returns a root of f between a and b, where f has opposite
// signs, by halving the interval up to 100 times.
func vennBisect(f func(float64) float64, a, b float64) float64 {
	fa, fb := f(a), f(b)
	if fa == 0 {
		return a
	}
	if fb == 0 {
		return b
	}
	delta := b - a
	for range 100 {
		delta /= 2
		mid := a + delta
		fm := f(mid)
		if fm*fa >= 0 {
			a = mid
		}
		if math.Abs(delta) < 1e-10 || fm == 0 {
			return mid
		}
	}
	return a + delta
}

// vennDistance returns the distance at which two circles of radii r1 and
// r2 overlap by an area.
func vennDistance(r1, r2, overlap float64) float64 {
	if math.Min(r1, r2)*math.Min(r1, r2)*math.Pi <= overlap+vennSmall {
		return math.Abs(r1 - r2)
	}
	return vennBisect(func(d float64) float64 { return vennOverlap(r1, r2, d) - overlap }, 0, r1+r2)
}

// vennMinimize finds a minimum of f from x0 with the method of the simplex
// of Nelder and Mead, with the steps of venn.js: a reflection of 1, an
// expansion of 2, a contraction of a half and a shrink of a half. It stops
// after a number of steps, or when the values and the points of the
// simplex differ by less than a tolerance.
func vennMinimize(f func([]float64) float64, x0 []float64, steps int, minError, minTol float64) []float64 {
	type vertex struct {
		x  []float64
		fx float64
	}
	n := len(x0)
	simplex := make([]*vertex, n+1)
	simplex[0] = &vertex{x: slices.Clone(x0)}
	simplex[0].fx = f(simplex[0].x)
	for i := range n {
		p := slices.Clone(x0)
		if p[i] != 0 {
			p[i] *= 1.05
		} else {
			p[i] = 0.001
		}
		simplex[i+1] = &vertex{x: p, fx: f(p)}
	}
	sum := func(w1 float64, v1 []float64, w2 float64, v2 []float64) []float64 {
		out := make([]float64, n)
		for j := range n {
			out[j] = w1*v1[j] + w2*v2[j]
		}
		return out
	}
	update := func(x []float64, fx float64) {
		copy(simplex[n].x, x)
		simplex[n].fx = fx
	}
	order := func() {
		slices.SortStableFunc(simplex, func(a, b *vertex) int { return vennCmp(a.fx - b.fx) })
	}
	centroid := make([]float64, n)
	for range steps {
		order()
		diff := 0.0
		for i := range n {
			diff = math.Max(diff, math.Abs(simplex[0].x[i]-simplex[1].x[i]))
		}
		if math.Abs(simplex[0].fx-simplex[n].fx) < minError && diff < minTol {
			break
		}
		for i := range n {
			centroid[i] = 0
			for j := range n {
				centroid[i] += simplex[j].x[i]
			}
			centroid[i] /= float64(n)
		}
		worst := simplex[n]
		reflected := sum(2, centroid, -1, worst.x)
		fr := f(reflected)
		switch {
		case fr < simplex[0].fx:
			expanded := sum(3, centroid, -2, worst.x)
			if fe := f(expanded); fe < fr {
				update(expanded, fe)
			} else {
				update(reflected, fr)
			}
		case fr >= simplex[n-1].fx:
			var contracted []float64
			var fc float64
			var shrink bool
			if fr > worst.fx {
				contracted = sum(0.5, centroid, 0.5, worst.x)
				fc = f(contracted)
				shrink = fc >= worst.fx
			} else {
				contracted = sum(1.5, centroid, -0.5, worst.x)
				fc = f(contracted)
				shrink = fc >= fr
			}
			if !shrink {
				update(contracted, fc)
				break
			}
			for i := 1; i < len(simplex); i++ {
				simplex[i].x = sum(0.5, simplex[0].x, 0.5, simplex[i].x)
				simplex[i].fx = f(simplex[i].x)
			}
		default:
			update(reflected, fr)
		}
	}
	order()
	return simplex[0].x
}

// vennKeys returns ids in the order of the keys of an object of
// JavaScript, which venn.js keeps its circles in: the ids that are
// indexes of an array first, from the smallest, and then the others in
// the order that they came in. A repeated id keeps its first place.
func vennKeys(ids []string) []string {
	var index, other []string
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if n, err := strconv.ParseUint(id, 10, 32); err == nil && n < math.MaxUint32 && strconv.FormatUint(n, 10) == id {
			index = append(index, id)
		} else {
			other = append(other, id)
		}
	}
	slices.SortStableFunc(index, func(a, b string) int {
		x, _ := strconv.ParseUint(a, 10, 32)
		y, _ := strconv.ParseUint(b, 10, 32)
		return vennCmp(float64(x) - float64(y))
	})
	return append(index, other...)
}

// vennSolution is a layout of circles by the ids of their sets, with the
// order of its keys.
type vennSolution struct {
	keys    []string
	circles map[string]*vennCircle
}

// newVennSolution returns a solution of circles, in the order of the keys
// of an object of JavaScript.
func newVennSolution(cs []*vennCircle) *vennSolution {
	s := &vennSolution{circles: map[string]*vennCircle{}}
	ids := make([]string, 0, len(cs))
	for _, c := range cs {
		s.circles[c.id] = c
		ids = append(ids, c.id)
	}
	s.keys = vennKeys(ids)
	return s
}

// list returns the circles in the order of the keys.
func (s *vennSolution) list() []*vennCircle {
	out := make([]*vennCircle, len(s.keys))
	for i, k := range s.keys {
		out[i] = s.circles[k]
	}
	return out
}

// vennLoss returns how far the overlaps of circles are from the sizes of
// the areas: the sum of the squares of the differences.
func vennLoss(circles map[string]*vennCircle, areas []vennSize) float64 {
	out := 0.0
	for _, a := range areas {
		if len(a.sets) == 1 {
			continue
		}
		var overlap float64
		if len(a.sets) == 2 {
			l, r := circles[a.sets[0]], circles[a.sets[1]]
			overlap = vennOverlap(l.r, r.r, vennDist(l.x, l.y, r.x, r.y))
		} else {
			cs := make([]vennCircle, len(a.sets))
			for i, s := range a.sets {
				cs[i] = *circles[s]
			}
			overlap, _ = vennIntersection(cs)
		}
		out += a.weight * (overlap - a.size) * (overlap - a.size)
	}
	return out
}

// vennAddMissing returns the areas with an area of size 0 for each pair of
// sets that has none, so that the layout keeps the two sets apart.
func vennAddMissing(areas []vennSize) []vennSize {
	out := slices.Clone(areas)
	var ids []string
	pairs := map[[2]string]bool{}
	for _, a := range areas {
		switch len(a.sets) {
		case 1:
			ids = append(ids, a.sets[0])
		case 2:
			pairs[[2]string{a.sets[0], a.sets[1]}] = true
			pairs[[2]string{a.sets[1], a.sets[0]}] = true
		}
	}
	slices.Sort(ids)
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if !pairs[[2]string{ids[i], ids[j]}] {
				out = append(out, vennSize{sets: []string{ids[i], ids[j]}, weight: 1})
			}
		}
	}
	return out
}

// vennGreedy places the circles one at a time, from the set that overlaps
// the others most, each at the point of a few candidates that fits the
// overlaps of the pairs best: at the right distance from a placed circle
// along an axis, or where the right distances from two placed circles
// meet.
func vennGreedy(areas []vennSize) *vennSolution {
	var cs []*vennCircle
	for _, a := range areas {
		if len(a.sets) == 1 {
			cs = append(cs, &vennCircle{id: a.sets[0], x: 1e10, y: 1e10, size: a.size, r: math.Sqrt(a.size / math.Pi)})
		}
	}
	// A later area of the same set replaces the circle and keeps its
	// place, as a key of an object does.
	byID := map[string]*vennCircle{}
	for _, c := range cs {
		byID[c.id] = c
	}
	var uniq []*vennCircle
	for _, k := range vennKeys(func() []string {
		ids := make([]string, len(cs))
		for i, c := range cs {
			ids[i] = c.id
		}
		return ids
	}()) {
		uniq = append(uniq, byID[k])
	}
	s := newVennSolution(uniq)
	type overlap struct {
		set          string
		size, weight float64
	}
	overlaps := map[string][]overlap{}
	var pairs []vennSize
	for _, a := range areas {
		if len(a.sets) != 2 {
			continue
		}
		pairs = append(pairs, a)
		l, r := a.sets[0], a.sets[1]
		w := a.weight
		// A circle inside another does not place early.
		if a.size+vennSmall >= math.Min(s.circles[l].size, s.circles[r].size) {
			w = 0
		}
		overlaps[l] = append(overlaps[l], overlap{r, a.size, w})
		overlaps[r] = append(overlaps[r], overlap{l, a.size, w})
	}
	type most struct {
		set  string
		size float64
	}
	var order []most
	for _, k := range s.keys {
		total := 0.0
		for _, o := range overlaps[k] {
			total += o.size * o.weight
		}
		order = append(order, most{k, total})
	}
	slices.SortStableFunc(order, func(a, b most) int { return vennCmp(b.size - a.size) })
	placed := map[string]bool{}
	first := s.circles[order[0].set]
	first.x, first.y = 0, 0
	placed[order[0].set] = true
	for _, m := range order[1:] {
		var near []overlap
		for _, o := range overlaps[m.set] {
			if placed[o.set] {
				near = append(near, o)
			}
		}
		slices.SortStableFunc(near, func(a, b overlap) int { return vennCmp(b.size - a.size) })
		c := s.circles[m.set]
		var points []vennPoint
		for j, o := range near {
			p1 := s.circles[o.set]
			d1 := vennDistance(c.r, p1.r, o.size)
			points = append(points,
				vennPoint{x: p1.x + d1, y: p1.y}, vennPoint{x: p1.x - d1, y: p1.y},
				vennPoint{x: p1.x, y: p1.y + d1}, vennPoint{x: p1.x, y: p1.y - d1})
			for _, o2 := range near[j+1:] {
				p2 := s.circles[o2.set]
				d2 := vennDistance(c.r, p2.r, o2.size)
				points = append(points, vennCross(p1.x, p1.y, d1, p2.x, p2.y, d2)...)
			}
		}
		if len(points) == 0 {
			// addMissing gives each pair an area, so this does not
			// happen.
			points = append(points, vennPoint{})
		}
		best, bestLoss := points[0], 1e50
		for _, p := range points {
			c.x, c.y = p.x, p.y
			if l := vennLoss(s.circles, pairs); l < bestLoss {
				best, bestLoss = p, l
			}
		}
		c.x, c.y = best.x, best.y
		placed[m.set] = true
	}
	return s
}

// vennSolve lays out the circles of the areas: the greedy placement, then
// the method of the simplex over the centers of the circles, for 500
// steps at most.
func vennSolve(areas []vennSize) *vennSolution {
	areas = vennAddMissing(areas)
	s := vennGreedy(areas)
	// The greedy placement suits two and three sets, and lines them up
	// with the axes. With more areas, Mermaid also tries a scaling of the
	// distances, and keeps it when it fits better.
	if len(areas) >= 8 {
		if m := vennMDS(areas, s); vennLoss(m.circles, areas)+1e-8 < vennLoss(s.circles, areas) {
			s = m
		}
	}
	x0 := make([]float64, 0, 2*len(s.keys))
	for _, c := range s.list() {
		x0 = append(x0, c.x, c.y)
	}
	cs := s.list()
	x := vennMinimize(func(v []float64) float64 {
		for i, c := range cs {
			c.x, c.y = v[2*i], v[2*i+1]
		}
		return vennLoss(s.circles, areas)
	}, x0, 500, 1e-6, 1e-5)
	for i, c := range cs {
		c.x, c.y = x[2*i], x[2*i+1]
	}
	return s
}

// vennMDS places the circles so that the distances between their centers
// come near the distances that give the sizes of the pairs, as a
// multidimensional scaling does. A pair that is apart only needs to be
// at its distance or farther, and a pair where one set holds the other at
// its distance or nearer. Mermaid starts the search from 10 random points
// and keeps the best. zuzu starts it from 10 points of a fixed sequence,
// so that the same source gives the same image.
func vennMDS(areas []vennSize, greedy *vennSolution) *vennSolution {
	ids := greedy.keys
	index := map[string]int{}
	for i, id := range ids {
		index[id] = i
	}
	n := len(ids)
	dist := make([][]float64, n)
	kind := make([][]int, n)
	for i := range n {
		dist[i], kind[i] = make([]float64, n), make([]int, n)
	}
	for _, a := range areas {
		if len(a.sets) != 2 {
			continue
		}
		i, j := index[a.sets[0]], index[a.sets[1]]
		ci, cj := greedy.circles[a.sets[0]], greedy.circles[a.sets[1]]
		d := vennDistance(ci.r, cj.r, a.size)
		dist[i][j], dist[j][i] = d, d
		k := 0
		switch {
		case a.size+1e-10 >= math.Min(ci.size, cj.size):
			k = 1
		case a.size <= 1e-10:
			k = -1
		}
		kind[i][j], kind[j][i] = k, k
	}
	// Keep the distances near 1, so that the search behaves.
	norm := 0.0
	for _, row := range dist {
		for _, d := range row {
			norm += d * d
		}
	}
	norm = math.Sqrt(norm) / float64(n)
	if norm == 0 {
		return greedy
	}
	loss := func(x []float64) float64 {
		out := 0.0
		for i := range n {
			for j := i + 1; j < n; j++ {
				dij := dist[i][j] / norm
				sq := (x[2*j]-x[2*i])*(x[2*j]-x[2*i]) + (x[2*j+1]-x[2*i+1])*(x[2*j+1]-x[2*i+1])
				d := math.Sqrt(sq)
				if kind[i][j] > 0 && d <= dij || kind[i][j] < 0 && d >= dij {
					continue
				}
				delta := sq - dij*dij
				out += 2 * delta * delta
			}
		}
		return out
	}
	// A fixed sequence of xorshift gives the starting points.
	state := uint64(0x9e3779b97f4a7c15)
	random := func() float64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return float64(state>>11) / (1 << 53)
	}
	var best []float64
	bestLoss := math.Inf(1)
	for range 10 {
		x0 := make([]float64, 2*n)
		for i := range x0 {
			x0[i] = random()
		}
		x := vennMinimize(loss, x0, 400*n, 1e-6, 1e-5)
		if l := loss(x); l < bestLoss {
			best, bestLoss = x, l
		}
	}
	cs := make([]*vennCircle, n)
	for i, id := range ids {
		c := *greedy.circles[id]
		c.x, c.y = best[2*i]*norm, best[2*i+1]*norm
		cs[i] = &c
	}
	return newVennSolution(cs)
}

// vennBounds is the box of circles.
type vennBounds struct {
	x0, x1, y0, y1 float64
}

// vennBox returns the box of circles.
func vennBox(cs []*vennCircle) vennBounds {
	b := vennBounds{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)}
	for _, c := range cs {
		b.x0, b.x1 = math.Min(b.x0, c.x-c.r), math.Max(b.x1, c.x+c.r)
		b.y0, b.y1 = math.Min(b.y0, c.y-c.r), math.Max(b.y1, c.y+c.r)
	}
	return b
}

// vennOrient turns a group of circles that overlap so that the largest is
// at the origin, the second largest is straight under it, and the third is
// on its right. A circle inside the only other one moves to its side.
func vennOrient(cs []*vennCircle) {
	slices.SortStableFunc(cs, func(a, b *vennCircle) int { return vennCmp(b.r - a.r) })
	x0, y0 := cs[0].x, cs[0].y
	for _, c := range cs {
		c.x -= x0
		c.y -= y0
	}
	if len(cs) == 2 && vennDist(cs[0].x, cs[0].y, cs[1].x, cs[1].y) < math.Abs(cs[1].r-cs[0].r) {
		cs[1].x = cs[0].x + cs[0].r - cs[1].r - 1e-10
		cs[1].y = cs[0].y
	}
	const orientation = math.Pi / 2
	if len(cs) > 1 {
		rot := math.Atan2(cs[1].x, cs[1].y) - orientation
		cos, sin := math.Cos(rot), math.Sin(rot)
		for _, c := range cs {
			x, y := c.x, c.y
			c.x, c.y = cos*x-sin*y, sin*x+cos*y
		}
	}
	if len(cs) > 2 {
		angle := math.Atan2(cs[2].x, cs[2].y) - orientation
		for angle < 0 {
			angle += 2 * math.Pi
		}
		for angle > 2*math.Pi {
			angle -= 2 * math.Pi
		}
		if angle > math.Pi {
			slope := cs[1].y / (1e-10 + cs[1].x)
			for _, c := range cs {
				d := (c.x + slope*c.y) / (1 + slope*slope)
				c.x, c.y = 2*d-c.x, 2*d*slope-c.y
			}
		}
	}
}

// vennArrange orients each group of circles that overlap, and puts the
// groups apart: the largest first, then the others in turn at its right,
// under it, and at its bottom right.
func vennArrange(s *vennSolution) *vennSolution {
	cs := make([]*vennCircle, 0, len(s.keys))
	for _, c := range s.list() {
		cp := *c
		cs = append(cs, &cp)
	}
	// Join the circles that overlap with a union find.
	parent := make([]int, len(cs))
	for i := range parent {
		parent[i] = i
	}
	var find func(i int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range cs {
		for j := i + 1; j < len(cs); j++ {
			if vennDist(cs[i].x, cs[i].y, cs[j].x, cs[j].y)+1e-10 < cs[i].r+cs[j].r {
				parent[find(j)] = find(i)
			}
		}
	}
	type cluster struct {
		cs   []*vennCircle
		b    vennBounds
		size float64
	}
	var clusters []*cluster
	byRoot := map[int]*cluster{}
	for i, c := range cs {
		r := find(i)
		if byRoot[r] == nil {
			byRoot[r] = &cluster{}
			clusters = append(clusters, byRoot[r])
		}
		byRoot[r].cs = append(byRoot[r].cs, c)
	}
	for _, cl := range clusters {
		vennOrient(cl.cs)
		cl.b = vennBox(cl.cs)
		cl.size = (cl.b.x1 - cl.b.x0) * (cl.b.y1 - cl.b.y0)
	}
	slices.SortStableFunc(clusters, func(a, b *cluster) int { return vennCmp(b.size - a.size) })
	out := slices.Clone(clusters[0].cs)
	bounds := clusters[0].b
	spacing := (bounds.x1 - bounds.x0) / 50
	add := func(i int, right, bottom bool) {
		if i >= len(clusters) {
			return
		}
		cl := clusters[i]
		var dx, dy float64
		if right {
			dx = bounds.x1 - cl.b.x0 + spacing
		} else {
			dx = bounds.x1 - cl.b.x1
			if c := (cl.b.x1-cl.b.x0)/2 - (bounds.x1-bounds.x0)/2; c < 0 {
				dx += c
			}
		}
		if bottom {
			dy = bounds.y1 - cl.b.y0 + spacing
		} else {
			dy = bounds.y1 - cl.b.y1
			if c := (cl.b.y1-cl.b.y0)/2 - (bounds.y1-bounds.y0)/2; c < 0 {
				dy += c
			}
		}
		for _, c := range cl.cs {
			c.x += dx
			c.y += dy
			out = append(out, c)
		}
	}
	for i := 1; i < len(clusters); i += 3 {
		add(i, true, false)
		add(i+1, false, true)
		add(i+2, true, true)
		bounds = vennBox(out)
	}
	return newVennSolution(out)
}

// vennFit scales and moves the circles so that they fit in a box of a
// width and a height, less a padding on each side, in the middle.
func vennFit(s *vennSolution, width, height, padding float64) *vennSolution {
	width -= 2 * padding
	height -= 2 * padding
	cs := s.list()
	b := vennBox(cs)
	if b.x1 == b.x0 || b.y1 == b.y0 {
		return s
	}
	scale := math.Min(height/(b.y1-b.y0), width/(b.x1-b.x0))
	dx := (width - (b.x1-b.x0)*scale) / 2
	dy := (height - (b.y1-b.y0)*scale) / 2
	out := make([]*vennCircle, len(cs))
	for i, c := range cs {
		out[i] = &vennCircle{
			id: c.id, size: c.size, r: scale * c.r,
			x: padding + dx + (c.x-b.x0)*scale,
			y: padding + dy + (c.y-b.y0)*scale,
		}
	}
	return newVennSolution(out)
}

// vennCenter is the point where the label of an area goes. disjoint is
// true when the area is empty.
type vennCenter struct {
	x, y     float64
	disjoint bool
}

// vennMargin returns how far a point is inside the circles of an area and
// outside the other circles: the least of these distances.
func vennMargin(x, y float64, interior, exterior []vennCircle) float64 {
	margin := interior[0].r - vennDist(interior[0].x, interior[0].y, x, y)
	for _, c := range interior[1:] {
		if m := c.r - vennDist(c.x, c.y, x, y); m <= margin {
			margin = m
		}
	}
	for _, c := range exterior {
		if m := vennDist(c.x, c.y, x, y) - c.r; m <= margin {
			margin = m
		}
	}
	return margin
}

// vennTextCenter returns the point of an area that is farthest inside its
// circles and outside the others. It starts from the best of a few points
// around the circles of the area, and finds the best point with the
// method of the simplex.
func vennTextCenter(interior, exterior []vennCircle) vennCenter {
	var points [][2]float64
	for _, c := range interior {
		points = append(points, [2]float64{c.x, c.y}, [2]float64{c.x + c.r/2, c.y}, [2]float64{c.x - c.r/2, c.y},
			[2]float64{c.x, c.y + c.r/2}, [2]float64{c.x, c.y - c.r/2})
	}
	initial := points[0]
	margin := vennMargin(initial[0], initial[1], interior, exterior)
	for _, p := range points[1:] {
		if m := vennMargin(p[0], p[1], interior, exterior); m >= margin {
			initial, margin = p, m
		}
	}
	x := vennMinimize(func(p []float64) float64 {
		return -vennMargin(p[0], p[1], interior, exterior)
	}, initial[:], 500, 1e-10, 1e-10)
	ret := vennCenter{x: x[0], y: x[1]}
	valid := true
	for _, c := range interior {
		if vennDist(ret.x, ret.y, c.x, c.y) > c.r {
			valid = false
		}
	}
	for _, c := range exterior {
		if vennDist(ret.x, ret.y, c.x, c.y) < c.r {
			valid = false
		}
	}
	if valid {
		return ret
	}
	if len(interior) == 1 {
		return vennCenter{x: interior[0].x, y: interior[0].y}
	}
	_, arcs := vennIntersection(interior)
	switch {
	case len(arcs) == 0:
		return vennCenter{x: 0, y: -1000, disjoint: true}
	case len(arcs) == 1:
		c := interior[arcs[0].circle]
		return vennCenter{x: c.x, y: c.y}
	case len(exterior) > 0:
		return vennTextCenter(interior, nil)
	}
	ps := make([]vennPoint, len(arcs))
	for i, a := range arcs {
		ps[i] = a.p1
	}
	cx, cy := vennMean(ps)
	return vennCenter{x: cx, y: cy}
}

// vennTextCenters returns the point of the label of each area, by the
// ids of its sets joined with commas. A circle that holds a circle of the
// area does not count against it.
func vennTextCenters(s *vennSolution, areas []vennSize) map[string]vennCenter {
	holders := map[string][]string{}
	for i, a := range s.keys {
		for _, b := range s.keys[i+1:] {
			ca, cb := s.circles[a], s.circles[b]
			d := vennDist(ca.x, ca.y, cb.x, cb.y)
			switch {
			case d+cb.r <= ca.r+1e-10:
				holders[b] = append(holders[b], a)
			case d+ca.r <= cb.r+1e-10:
				holders[a] = append(holders[a], b)
			}
		}
	}
	out := map[string]vennCenter{}
	for _, a := range areas {
		in, skip := map[string]bool{}, map[string]bool{}
		for _, id := range a.sets {
			in[id] = true
			for _, h := range holders[id] {
				skip[h] = true
			}
		}
		var interior, exterior []vennCircle
		for _, k := range s.keys {
			switch {
			case in[k]:
				interior = append(interior, *s.circles[k])
			case !skip[k]:
				exterior = append(exterior, *s.circles[k])
			}
		}
		if len(interior) == 0 {
			continue
		}
		out[vennKey(a.sets, ",")] = vennTextCenter(interior, exterior)
	}
	return out
}

// vennKey joins the ids of the sets of an area.
func vennKey(sets []string, sep string) string {
	return strings.Join(sets, sep)
}
