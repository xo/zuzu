package dataflow

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
)

// kde2d estimates the density of points in two dimensions, for each group
// of rows: it counts the points in the cells of a grid over the size, and
// blurs the counts three times in each direction with a box as wide as the
// bandwidth, which comes close to a Gaussian kernel. Each group gives a row
// with the grid: its values, its width and height in cells, the size of a
// cell, and the cells of the size inside the margin of the blur. The grid
// holds numbers of 32 bits, as in Vega.
func kde2d(p *params, rows []Row) ([]Row, error) {
	size, err := p.numbers("size")
	if err != nil {
		return nil, err
	}
	if len(size) != 2 || !(size[0] >= 0 && size[1] >= 0) {
		return nil, fmt.Errorf("kde2d needs a size of two numbers: %w", ErrParam)
	}
	x, err := p.field("x")
	if err != nil {
		return nil, err
	}
	y, err := p.field("y")
	if err != nil {
		return nil, err
	}
	if x == nil || y == nil {
		return nil, fmt.Errorf("kde2d needs x and y: %w", ErrParam)
	}
	weight, err := p.field("weight")
	if err != nil {
		return nil, err
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	k := 2
	if c, ok, err := p.raw("cellSize"); err != nil {
		return nil, err
	} else if ok && c != nil {
		cell := expr.Number(c)
		if !(cell >= 1) {
			return nil, fmt.Errorf("kde2d with the cell size %v: %w", c, ErrParam)
		}
		k = int(math.Floor(math.Log(cell) / math.Ln2))
	}
	bw := [2]float64{-1, -1}
	if b, err := p.numbers("bandwidth"); err != nil {
		return nil, err
	} else if len(b) == 1 {
		bw = [2]float64{b[0], b[0]}
	} else if len(b) == 2 {
		bw = [2]float64{b[0], b[1]}
	}
	counts, err := p.boolean("counts", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as("grid")
	if err != nil {
		return nil, err
	}
	a := &aggregation{groupby: groupby}
	order, groups, keys := a.group(rows)
	out := make([]Row, 0, len(order))
	for _, key := range order {
		g := groups[key]
		grid := density2D(g, x, y, weight, size, k, bw, counts)
		r := Row{as[0]: grid}
		for i, f := range groupby {
			if f != nil {
				r[f.Name] = keys[key][i]
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// toInt32 converts a number to an integer of 32 bits, as the bit
// operators of JavaScript do.
func toInt32(f float64) int32 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	m := math.Mod(math.Trunc(f), 1<<32)
	if m < 0 {
		m += 1 << 32
	}
	if m >= 1<<31 {
		m -= 1 << 32
	}
	return int32(m)
}

// blurRadius returns the radius of the box of the blur for a bandwidth, or
// for the bandwidth of the rule of thumb of Scott, as in Vega, when it is
// negative.
func blurRadius(bw float64, rows []Row, f *Accessor) int32 {
	v := bw
	if bw < 0 {
		v = bandwidthNRD(rows, f)
	}
	return int32(math.Floor((math.Sqrt(4*v*v+1)-1)/2 + 0.5))
}

// bandwidthNRD is the bandwidth of a normal reference distribution: 1.06
// times the lesser of the deviation and the spread of the quartiles over
// 1.34, times the count to the power -1/5.
func bandwidthNRD(rows []Row, f *Accessor) float64 {
	var vals []float64
	for _, r := range rows {
		if v := f.Get(r); v != nil && !expr.IsUndefined(v) {
			if x := expr.Number(v); !math.IsNaN(x) {
				vals = append(vals, x)
			}
		}
	}
	d := math.NaN()
	if len(vals) > 1 {
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		mean /= float64(len(vals))
		s := 0.0
		for _, v := range vals {
			s += (v - mean) * (v - mean)
		}
		d = math.Sqrt(s / float64(len(vals)-1))
	}
	var q0, q2 float64
	if len(vals) > 0 {
		sorted := slices.Clone(vals)
		slices.Sort(sorted)
		q0, q2 = quantileSorted(sorted, 0.25), quantileSorted(sorted, 0.75)
	}
	h := (q2 - q0) / 1.34
	v := math.Min(d, h)
	switch {
	case v != 0 && !math.IsNaN(v):
	case d != 0 && !math.IsNaN(d):
		v = d
	case q0 != 0 && !math.IsNaN(q0):
		v = math.Abs(q0)
	default:
		v = 1
	}
	return 1.06 * v * math.Pow(float64(len(rows)), -0.2)
}

// density2D counts and blurs the points of rows on a grid of cells of 2^k
// pixels.
func density2D(rows []Row, x, y, weight *Accessor, size []float64, k int, bw [2]float64, counts bool) map[string]any {
	rx := blurRadius(bw[0], rows, x) >> k
	ry := blurRadius(bw[1], rows, y) >> k
	var ox, oy int32
	if rx != 0 {
		ox = rx + 2
	}
	if ry != 0 {
		oy = ry + 2
	}
	dx, dy := toInt32(size[0])>>k, toInt32(size[1])>>k
	n, m := int(2*ox+dx), int(2*oy+dy)
	v0 := make([]float32, n*m)
	v1 := make([]float32, n*m)
	for _, r := range rows {
		xi := int(ox + toInt32(expr.Number(x.Get(r)))>>k)
		yi := int(oy + toInt32(expr.Number(y.Get(r)))>>k)
		if xi >= 0 && xi < n && yi >= 0 && yi < m {
			w := 1.0
			if weight != nil {
				w = expr.Number(weight.Get(r))
			}
			v0[xi+yi*n] = float32(float64(v0[xi+yi*n]) + w)
		}
	}
	values := v0
	switch {
	case rx > 0 && ry > 0:
		for range 3 {
			blurX(n, m, v0, v1, int(rx))
			blurY(n, m, v1, v0, int(ry))
		}
	case rx > 0:
		blurX(n, m, v0, v1, int(rx))
		blurX(n, m, v1, v0, int(rx))
		blurX(n, m, v0, v1, int(rx))
		values = v1
	case ry > 0:
		blurY(n, m, v0, v1, int(ry))
		blurY(n, m, v1, v0, int(ry))
		blurY(n, m, v0, v1, int(ry))
		values = v1
	}
	var s float64
	if counts {
		s = math.Pow(2, float64(-2*k))
	} else {
		sum := 0.0
		for _, v := range values {
			sum += float64(v)
		}
		s = 1 / sum
	}
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = float64(float32(float64(v) * s))
	}
	return map[string]any{
		"values": out, "scale": float64(int(1) << k), "width": float64(n), "height": float64(m),
		"x1": float64(ox), "y1": float64(oy), "x2": float64(ox + dx), "y2": float64(oy + dy),
	}
}

// blurX blurs each row of a grid with a box of radius r, and divides each
// sum by the count of the cells inside the grid.
func blurX(n, m int, src, dst []float32, r int) {
	w := r<<1 + 1
	for j := range m {
		sr := 0.0
		for i := range n + r {
			if i < n {
				sr += float64(src[i+j*n])
			}
			if i >= r {
				if i >= w {
					sr -= float64(src[i-w+j*n])
				}
				dst[i-r+j*n] = float32(sr / float64(min(i+1, n-1+w-i, w)))
			}
		}
	}
}

// blurY blurs each column of a grid, as blurX blurs each row.
func blurY(n, m int, src, dst []float32, r int) {
	w := r<<1 + 1
	for i := range n {
		sr := 0.0
		for j := range m + r {
			if j < m {
				sr += float64(src[i+j*n])
			}
			if j >= r {
				if j >= w {
					sr -= float64(src[i+(j-w)*n])
				}
				dst[i+(j-r)*n] = float32(sr / float64(min(j+1, m-1+w-j, w)))
			}
		}
	}
}

// gridOf reads a grid: its width, its height and its values.
func gridOf(v any) (m map[string]any, values []float64, width, height int, ok bool) {
	m, ok = v.(map[string]any)
	if !ok {
		return nil, nil, 0, 0, false
	}
	l, _ := m["values"].([]any)
	values = make([]float64, len(l))
	for i, x := range l {
		values[i] = expr.Number(x)
	}
	width = int(math.Floor(expr.Number(m["width"])))
	height = int(math.Floor(expr.Number(m["height"])))
	return m, values, width, height, width >= 0 && height >= 0
}

// gridMax returns the greatest of the values of a grid that are numbers.
func gridMax(values []float64) float64 {
	hi := math.NaN()
	for _, v := range values {
		if !math.IsNaN(v) && (math.IsNaN(hi) || v > hi) {
			hi = v
		}
	}
	return hi
}

// levels returns count thresholds that split the span of values evenly,
// from zero or from the least value, or at nice steps.
func levels(values []float64, count float64, nice, zero bool) []float64 {
	lo, hi := math.NaN(), math.NaN()
	for _, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if math.IsNaN(lo) || v < lo {
			lo = v
		}
		if math.IsNaN(hi) || v > hi {
			hi = v
		}
	}
	if math.IsNaN(hi) {
		return nil
	}
	start := lo
	if zero {
		start = math.Min(lo, 0)
	}
	step := (hi - start) / (count + 1)
	if nice {
		step = scale.TickStep(start, hi, count)
	}
	var out []float64
	n := int(math.Max(0, math.Ceil((hi-(start+step))/step)))
	for i := range n {
		out = append(out, start+step+float64(i)*step)
	}
	return out
}

// isocontour traces the lines of a grid at thresholds, as GeoJSON
// MultiPolygons, by marching squares. Each row of a grid gives a row for
// each threshold, with the fields of the row and the contour.
func isocontour(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	get := func(r Row) any {
		if field == nil {
			return r
		}
		return field.Get(r)
	}
	thresholds, err := p.numbers("thresholds")
	if err != nil {
		return nil, err
	}
	nLevels, err := p.number("levels", 10)
	if err != nil {
		return nil, err
	}
	if nLevels == 0 {
		nLevels = 10
	}
	nice, err := p.boolean("nice", false)
	if err != nil {
		return nil, err
	}
	zero, err := p.boolean("zero", true)
	if err != nil {
		return nil, err
	}
	smooth, err := p.boolean("smooth", true)
	if err != nil {
		return nil, err
	}
	resolve, err := p.str("resolve", "independent")
	if err != nil {
		return nil, err
	}
	scaleOf, err := p.perRow("scale")
	if err != nil {
		return nil, err
	}
	translateOf, err := p.perRow("translate")
	if err != nil {
		return nil, err
	}
	as := "contour"
	asNull := false
	if v, ok, err := p.raw("as"); err != nil {
		return nil, err
	} else if ok {
		if v == nil {
			asNull = true
		} else {
			as = expr.String(v)
		}
	}
	shared := thresholds == nil && resolve == "shared"
	if shared {
		maxes := make([]float64, 0, len(rows))
		for _, r := range rows {
			_, values, _, _, _ := gridOf(get(r))
			maxes = append(maxes, gridMax(values))
		}
		thresholds = levels(maxes, nLevels, nice, zero)
	}
	var out []Row
	for _, r := range rows {
		grid, values, w, h, ok := gridOf(get(r))
		if !ok {
			continue
		}
		tz := thresholds
		if tz == nil {
			tz = levels(values, nLevels, nice, zero)
		}
		s, err := scaleOf(r)
		if err != nil {
			return nil, err
		}
		if s == nil {
			s = grid["scale"]
		}
		t, err := translateOf(r)
		if err != nil {
			return nil, err
		}
		if t == nil {
			t = grid["translate"]
		}
		for _, z := range tz {
			polys := contourPolygons(values, w, h, z, smooth)
			transformContour(polys, grid, s, t)
			geom := map[string]any{"type": "MultiPolygon", "value": z, "coordinates": polygonsJSON(polys)}
			c := clone(r)
			if asNull {
				maps.Copy(c, geom)
			} else {
				c[as] = geom
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// perRow reads a parameter that is a value or an expression of the row.
func (p *params) perRow(name string) (func(Row) (any, error), error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return func(Row) (any, error) { return nil, nil }, err
	}
	if m, ok := v.(map[string]any); ok {
		if src, ok := m["expr"].(string); ok {
			prog, err := p.env.Compile(src)
			if err != nil {
				return nil, err
			}
			env := p.env
			return func(r Row) (any, error) { return prog.Eval(env.Scope(r)) }, nil
		}
	}
	return func(Row) (any, error) { return v, nil }, nil
}

// pair reads a number or a pair of numbers.
func pair(v any) (float64, float64) {
	if l, ok := v.([]any); ok {
		var a, b float64
		if len(l) > 0 {
			a = expr.Number(l[0])
		}
		if len(l) > 1 {
			b = expr.Number(l[1])
		}
		return a, b
	}
	f := expr.Number(v)
	return f, f
}

// transformContour scales and moves the points of contours from the cells
// of a grid, from the corner of the size inside its margin. A scale that
// flips one axis reverses the rings, to keep their winding.
func transformContour(polys [][][][2]float64, grid map[string]any, s, t any) {
	sIsOne := s == nil
	if f, ok := s.(float64); ok && f == 1 {
		sIsOne = true
	}
	if sIsOne && t == nil {
		return
	}
	sx, sy := 1.0, 1.0
	if s != nil {
		sx, sy = pair(s)
	}
	if sx == 0 || math.IsNaN(sx) {
		sx = 1
	}
	if sy == 0 || math.IsNaN(sy) {
		sy = 1
	}
	var tx, ty float64
	if t != nil {
		tx, ty = pair(t)
		if math.IsNaN(tx) {
			tx = 0
		}
		if math.IsNaN(ty) {
			ty = 0
		}
	}
	x1, y1 := expr.Number(grid["x1"]), expr.Number(grid["y1"])
	if math.IsNaN(x1) {
		x1 = 0
	}
	if math.IsNaN(y1) {
		y1 = 0
	}
	flip := sx*sy < 0
	for _, poly := range polys {
		for _, ring := range poly {
			if flip {
				slices.Reverse(ring)
			}
			for i := range ring {
				ring[i][0] = (ring[i][0]-x1)*sx + tx
				ring[i][1] = (ring[i][1]-y1)*sy + ty
			}
		}
	}
}

func polygonsJSON(polys [][][][2]float64) []any {
	out := make([]any, len(polys))
	for i, poly := range polys {
		rings := make([]any, len(poly))
		for j, ring := range poly {
			pts := make([]any, len(ring))
			for k, p := range ring {
				pts[k] = []any{p[0], p[1]}
			}
			rings[j] = pts
		}
		out[i] = rings
	}
	return out
}

// marchCases are the segments of a cell of marching squares for each of
// the 16 cases of its corners, in units of half a cell.
var marchCases = [16][][2][2]float64{
	{},
	{{{1.0, 1.5}, {0.5, 1.0}}},
	{{{1.5, 1.0}, {1.0, 1.5}}},
	{{{1.5, 1.0}, {0.5, 1.0}}},
	{{{1.0, 0.5}, {1.5, 1.0}}},
	{{{1.0, 1.5}, {0.5, 1.0}}, {{1.0, 0.5}, {1.5, 1.0}}},
	{{{1.0, 0.5}, {1.0, 1.5}}},
	{{{1.0, 0.5}, {0.5, 1.0}}},
	{{{0.5, 1.0}, {1.0, 0.5}}},
	{{{1.0, 1.5}, {1.0, 0.5}}},
	{{{0.5, 1.0}, {1.0, 0.5}}, {{1.5, 1.0}, {1.0, 1.5}}},
	{{{1.5, 1.0}, {1.0, 0.5}}},
	{{{0.5, 1.0}, {1.5, 1.0}}},
	{{{1.0, 1.5}, {1.5, 1.0}}},
	{{{0.5, 1.0}, {1.0, 1.5}}},
	{},
}

// fragment is a line of segments that stitching has joined, open at both
// ends until it closes into a ring.
type fragment struct {
	start, end int
	ring       [][2]float64
}

// contourPolygons returns the polygons of a grid at a threshold: the rings
// around the cells at or above it, with the holes inside the ring that
// holds them.
func contourPolygons(values []float64, dx, dy int, value float64, smooth bool) [][][][2]float64 {
	var polygons [][][][2]float64
	var holes [][][2]float64
	isorings(values, dx, dy, value, func(ring [][2]float64) {
		if smooth {
			smoothLinear(ring, values, dx, dy, value)
		}
		if ringArea(ring) > 0 {
			polygons = append(polygons, [][][2]float64{ring})
		} else {
			holes = append(holes, ring)
		}
	})
	for _, hole := range holes {
		for i := range polygons {
			if contains(polygons[i][0], hole) != -1 {
				polygons[i] = append(polygons[i], hole)
				break
			}
		}
	}
	return polygons
}

// isorings finds the rings of a threshold by marching squares over the
// cells, with a margin of cells below the threshold all around, and joins
// the segments into rings.
func isorings(values []float64, dx, dy int, value float64, ring func([][2]float64)) {
	byStart := map[int]*fragment{}
	byEnd := map[int]*fragment{}
	above := func(i int) int {
		if i >= 0 && i < len(values) && values[i] >= value {
			return 1
		}
		return 0
	}
	index := func(p [2]float64) int {
		return int(p[0]*2 + p[1]*float64(dx+1)*4)
	}
	var x, y int
	stitch := func(line [2][2]float64) {
		start := [2]float64{line[0][0] + float64(x), line[0][1] + float64(y)}
		end := [2]float64{line[1][0] + float64(x), line[1][1] + float64(y)}
		si, ei := index(start), index(end)
		if f, ok := byEnd[si]; ok {
			if g, ok := byStart[ei]; ok {
				delete(byEnd, f.end)
				delete(byStart, g.start)
				if f == g {
					f.ring = append(f.ring, end)
					ring(f.ring)
				} else {
					h := &fragment{start: f.start, end: g.end, ring: append(slices.Clone(f.ring), g.ring...)}
					byStart[f.start], byEnd[g.end] = h, h
				}
			} else {
				delete(byEnd, f.end)
				f.ring = append(f.ring, end)
				f.end = ei
				byEnd[ei] = f
			}
		} else if f, ok := byStart[ei]; ok {
			if g, ok := byEnd[si]; ok {
				delete(byStart, f.start)
				delete(byEnd, g.end)
				if f == g {
					f.ring = append(f.ring, end)
					ring(f.ring)
				} else {
					h := &fragment{start: g.start, end: f.end, ring: append(slices.Clone(g.ring), f.ring...)}
					byStart[g.start], byEnd[f.end] = h, h
				}
			} else {
				delete(byStart, f.start)
				f.ring = append([][2]float64{start}, f.ring...)
				f.start = si
				byStart[si] = f
			}
		} else {
			f := &fragment{start: si, end: ei, ring: [][2]float64{start, end}}
			byStart[si], byEnd[ei] = f, f
		}
	}
	march := func(c int) {
		for _, l := range marchCases[c] {
			stitch(l)
		}
	}
	// The first row, with the margin above it.
	x, y = -1, -1
	t1 := above(0)
	march(t1 << 1)
	for x++; x < dx-1; x++ {
		t0 := t1
		t1 = above(x + 1)
		march(t0 | t1<<1)
	}
	march(t1)
	// The rows between.
	for y++; y < dy-1; y++ {
		x = -1
		t1 = above(y*dx + dx)
		t2 := above(y * dx)
		march(t1<<1 | t2<<2)
		for x++; x < dx-1; x++ {
			t0 := t1
			t1 = above(y*dx + dx + x + 1)
			t3 := t2
			t2 = above(y*dx + x + 1)
			march(t0 | t1<<1 | t2<<2 | t3<<3)
		}
		march(t1 | t2<<3)
	}
	// The last row, with the margin below it.
	x = -1
	t2 := above(y * dx)
	march(t2 << 2)
	for x++; x < dx-1; x++ {
		t3 := t2
		t2 = above(y*dx + x + 1)
		march(t2<<2 | t3<<3)
	}
	march(t2 << 3)
}

// smoothLinear moves each point of a ring along its edge of the cell, to
// where the values cross the threshold, between the values at the two ends
// of the edge.
func smoothLinear(ring [][2]float64, values []float64, dx, dy int, value float64) {
	at := func(i int) float64 {
		if i >= 0 && i < len(values) {
			return values[i]
		}
		return math.NaN()
	}
	for i := range ring {
		x, y := ring[i][0], ring[i][1]
		xt, yt := int(toInt32(x)), int(toInt32(y))
		v1 := at(yt*dx + xt)
		if x > 0 && x < float64(dx) && float64(xt) == x {
			v0 := at(yt*dx + xt - 1)
			ring[i][0] = x + (value-v0)/(v1-v0) - 0.5
		}
		if y > 0 && y < float64(dy) && float64(yt) == y {
			v0 := at((yt-1)*dx + xt)
			ring[i][1] = y + (value-v0)/(v1-v0) - 0.5
		}
	}
}

// ringArea returns twice the signed area of a ring, positive for a ring
// that winds clockwise on the screen.
func ringArea(ring [][2]float64) float64 {
	n := len(ring)
	if n == 0 {
		return 0
	}
	a := ring[n-1][1]*ring[0][0] - ring[n-1][0]*ring[0][1]
	for i := 1; i < n; i++ {
		a += ring[i-1][1]*ring[i][0] - ring[i-1][0]*ring[i][1]
	}
	return a
}

// contains returns 1 if a ring holds the first point of a hole that is not
// on its edge, -1 if it does not hold it, and 0 if every point lies on the
// edge.
func contains(ring, hole [][2]float64) int {
	for _, p := range hole {
		if c := ringContains(ring, p); c != 0 {
			return c
		}
	}
	return 0
}

func ringContains(ring [][2]float64, p [2]float64) int {
	x, y := p[0], p[1]
	c := -1
	n := len(ring)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		pi, pj := ring[i], ring[j]
		if segmentContains(pi, pj, p) {
			return 0
		}
		if (pi[1] > y) != (pj[1] > y) && x < (pj[0]-pi[0])*(y-pi[1])/(pj[1]-pi[1])+pi[0] {
			c = -c
		}
	}
	return c
}

func segmentContains(a, b, c [2]float64) bool {
	if (b[0]-a[0])*(c[1]-a[1]) != (c[0]-a[0])*(b[1]-a[1]) {
		return false
	}
	i := 0
	if a[0] == b[0] {
		i = 1
	}
	p, q, r := a[i], c[i], b[i]
	return p <= q && q <= r || r <= q && q <= p
}
