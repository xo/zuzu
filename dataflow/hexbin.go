package dataflow

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/xo/zuzu/expr"
)

// hexbin counts points in a grid of hexagons with a corner at the top, and
// makes a row for each hexagon that holds a point: its center, the values
// of the group by fields, the aggregates of its points, and the path of a
// hexagon around 0, 0. The hexagons tile the plane from the origin, and an
// extent keeps those whose centers lie inside it.
func hexbin(p *params, rows []Row) ([]Row, error) {
	x, err := p.field("x")
	if err != nil {
		return nil, err
	}
	y, err := p.field("y")
	if err != nil {
		return nil, err
	}
	if x == nil || y == nil {
		return nil, fmt.Errorf("zuzu.hexbin needs x and y: %w", ErrParam)
	}
	r, err := p.number("radius", 20)
	if err != nil {
		return nil, err
	}
	if !(r > 0) {
		return nil, fmt.Errorf("zuzu.hexbin with the radius %v: %w", r, ErrParam)
	}
	clip := false
	var ex0, ey0, ex1, ey1 float64
	if v, ok, err := p.raw("extent"); err != nil {
		return nil, err
	} else if ok && v != nil {
		if ex0, ey0, ex1, ey1, err = p.extentOf(); err != nil {
			return nil, err
		}
		clip = true
	}
	a, err := readAggregation(p)
	if err != nil {
		return nil, err
	}
	w := math.Sqrt(3) * r
	type cell struct {
		q, r int
		rows []Row
	}
	var order []string
	cells := map[string]*cell{}
	keys := map[string][]any{}
	for _, row := range rows {
		px, py := expr.Number(x.Get(row)), expr.Number(y.Get(row))
		if math.IsNaN(px) || math.IsNaN(py) {
			continue
		}
		q, rr := hexRound((math.Sqrt(3)/3*px-py/3)/r, 2.0/3*py/r)
		vals := make([]any, len(a.groupby))
		for i, g := range a.groupby {
			vals[i] = g.Get(row)
		}
		k := key(append(slices.Clone(vals), float64(q), float64(rr)))
		c, ok := cells[k]
		if !ok {
			c = &cell{q: q, r: rr}
			cells[k] = c
			keys[k] = vals
			order = append(order, k)
		}
		c.rows = append(c.rows, row)
	}
	path := hexagonPath(r)
	var out []Row
	for _, k := range order {
		c := cells[k]
		cx := w * (float64(c.q) + float64(c.r)/2)
		cy := 1.5 * r * float64(c.r)
		if clip && (cx < ex0 || cx > ex1 || cy < ey0 || cy > ey1) {
			continue
		}
		row := Row{"x": cx, "y": cy, "path": path}
		for i, g := range a.groupby {
			if g != nil {
				row[g.Name] = keys[k][i]
			}
		}
		a.measure(row, c.rows)
		out = append(out, row)
	}
	return out, nil
}

// hexRound returns the hexagon that holds a point in axial coordinates, by
// rounding its cube coordinates and fixing the one that rounded the most.
func hexRound(q, r float64) (int, int) {
	s := -q - r
	rq, rr, rs := math.Round(q), math.Round(r), math.Round(s)
	dq, dr, ds := math.Abs(rq-q), math.Abs(rr-r), math.Abs(rs-s)
	switch {
	case dq > dr && dq > ds:
		rq = -rr - rs
	case dr > ds:
		rr = -rq - rs
	}
	return int(rq), int(rr)
}

// hexagonPath returns a hexagon of radius r around 0, 0, with a corner at
// the top.
func hexagonPath(r float64) string {
	var b strings.Builder
	for k := range 6 {
		t := float64(k) * math.Pi / 3
		if k == 0 {
			b.WriteString("M")
		} else {
			b.WriteString("L")
		}
		b.WriteString(num(round9(r*math.Sin(t))) + "," + num(round9(-r*math.Cos(t))))
	}
	b.WriteString("Z")
	return b.String()
}

// round9 rounds a coordinate to 9 decimals, so that a corner at 0 does not
// print as a tiny number.
func round9(x float64) float64 {
	return math.Round(x*1e9) / 1e9
}
