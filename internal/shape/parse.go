package shape

import (
	"errors"
	"math"
	"strconv"
)

// ErrPath is the error of path data that does not read.
var ErrPath = errors.New("bad path data")

// Sink receives the segments of a path in absolute coordinates. An arc
// arrives as cubic curves.
type Sink interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
	QuadTo(x1, y1, x, y float64)
	CubicTo(x1, y1, x2, y2, x, y float64)
	Close()
}

// pathReader reads the numbers and the flags of path data.
type pathReader struct {
	d string
	i int
}

// skip passes spaces and commas.
func (r *pathReader) skip() {
	for r.i < len(r.d) {
		switch r.d[r.i] {
		case ' ', '\t', '\n', '\r', '\f', ',':
			r.i++
		default:
			return
		}
	}
}

// number reads a number.
func (r *pathReader) number() (float64, error) {
	r.skip()
	start := r.i
	if r.i < len(r.d) && (r.d[r.i] == '-' || r.d[r.i] == '+') {
		r.i++
	}
	dot, exp := false, false
	for r.i < len(r.d) {
		c := r.d[r.i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !dot && !exp:
			dot = true
		case (c == 'e' || c == 'E') && !exp:
			exp = true
			if r.i+1 < len(r.d) && (r.d[r.i+1] == '-' || r.d[r.i+1] == '+') {
				r.i++
			}
		default:
			return r.parse(start)
		}
		r.i++
	}
	return r.parse(start)
}

func (r *pathReader) parse(start int) (float64, error) {
	v, err := strconv.ParseFloat(r.d[start:r.i], 64)
	if err != nil {
		return 0, ErrPath
	}
	return v, nil
}

// flag reads a flag of an arc, a single 0 or 1 that needs no separator.
func (r *pathReader) flag() (bool, error) {
	r.skip()
	if r.i >= len(r.d) || (r.d[r.i] != '0' && r.d[r.i] != '1') {
		return false, ErrPath
	}
	r.i++
	return r.d[r.i-1] == '1', nil
}

// numbers reads n numbers.
func (r *pathReader) numbers(n int) ([]float64, error) {
	out := make([]float64, n)
	for i := range out {
		v, err := r.number()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// Parse reads the path data of SVG and sends its segments to a sink. A
// command repeats while numbers follow it, and the numbers after a move are
// lines. It stops at the first error, as SVG does, after the segments that
// read.
func Parse(d string, s Sink) error {
	r := &pathReader{d: d}
	st := &state{}
	var cmd byte
	for {
		r.skip()
		if r.i >= len(d) {
			return nil
		}
		if c := d[r.i]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			if cmd == 0 && c&^0x20 != 'M' {
				// A path starts with a move.
				return ErrPath
			}
			cmd = c
			r.i++
		} else if cmd == 0 || cmd&^0x20 == 'Z' {
			return ErrPath
		}
		if err := st.run(cmd, r, s); err != nil {
			return err
		}
		if cmd&^0x20 == 'M' {
			// The numbers after a move are lines.
			cmd--
		}
	}
}

// counts are the numbers that each command reads, but for an arc, which
// also reads flags.
var counts = map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4, 'Q': 4, 'T': 2}

// state is the state of a path between commands: the current point, the
// start of the subpath, and the control point of the last curve, for S and
// T.
type state struct {
	x, y, sx, sy, cx, cy float64
	last                 byte
}

// run runs one command on the numbers that r holds next.
func (st *state) run(cmd byte, r *pathReader, s Sink) error {
	up := cmd &^ 0x20
	var ox, oy float64
	if cmd >= 'a' {
		ox, oy = st.x, st.y
	}
	var p []float64
	if n, ok := counts[up]; ok {
		var err error
		if p, err = r.numbers(n); err != nil {
			return err
		}
	}
	switch up {
	case 'Z':
		s.Close()
		st.x, st.y = st.sx, st.sy
	case 'M':
		st.x, st.y = ox+p[0], oy+p[1]
		st.sx, st.sy = st.x, st.y
		s.MoveTo(st.x, st.y)
	case 'L':
		st.x, st.y = ox+p[0], oy+p[1]
		s.LineTo(st.x, st.y)
	case 'H':
		st.x = ox + p[0]
		s.LineTo(st.x, st.y)
	case 'V':
		st.y = oy + p[0]
		s.LineTo(st.x, st.y)
	case 'C':
		st.cx, st.cy = ox+p[2], oy+p[3]
		st.x, st.y = ox+p[4], oy+p[5]
		s.CubicTo(ox+p[0], oy+p[1], st.cx, st.cy, st.x, st.y)
	case 'S':
		x1, y1 := st.x, st.y
		if st.last == 'C' || st.last == 'S' {
			x1, y1 = 2*st.x-st.cx, 2*st.y-st.cy
		}
		st.cx, st.cy = ox+p[0], oy+p[1]
		st.x, st.y = ox+p[2], oy+p[3]
		s.CubicTo(x1, y1, st.cx, st.cy, st.x, st.y)
	case 'Q':
		st.cx, st.cy = ox+p[0], oy+p[1]
		st.x, st.y = ox+p[2], oy+p[3]
		s.QuadTo(st.cx, st.cy, st.x, st.y)
	case 'T':
		if st.last == 'Q' || st.last == 'T' {
			st.cx, st.cy = 2*st.x-st.cx, 2*st.y-st.cy
		} else {
			st.cx, st.cy = st.x, st.y
		}
		st.x, st.y = ox+p[0], oy+p[1]
		s.QuadTo(st.cx, st.cy, st.x, st.y)
	case 'A':
		if err := st.arc(r, s, ox, oy); err != nil {
			return err
		}
	default:
		return ErrPath
	}
	st.last = up
	return nil
}

// arc reads the numbers and the flags of an arc and sends the arc.
func (st *state) arc(r *pathReader, s Sink, ox, oy float64) error {
	p, err := r.numbers(3)
	if err != nil {
		return err
	}
	large, err := r.flag()
	if err != nil {
		return err
	}
	sweep, err := r.flag()
	if err != nil {
		return err
	}
	q, err := r.numbers(2)
	if err != nil {
		return err
	}
	x1, y1 := ox+q[0], oy+q[1]
	arcTo(s, st.x, st.y, p[0], p[1], p[2], large, sweep, x1, y1)
	st.x, st.y = x1, y1
	return nil
}

// arcTo sends an elliptical arc of SVG from (x0, y0) to (x1, y1) as cubic
// curves. It finds the center by the conversion in the appendix of SVG,
// and splits the arc into equal parts of at most a quarter turn for an
// ellipse of a radius up to a few hundred pixels, more for a larger one.
func arcTo(s Sink, x0, y0, rx, ry, rot float64, large, sweep bool, x1, y1 float64) {
	if x0 == x1 && y0 == y1 {
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		s.LineTo(x1, y1)
		return
	}
	phi := rot * math.Pi / 180
	sin, cos := math.Sincos(phi)
	dx, dy := (x0-x1)/2, (y0-y1)/2
	x1p := cos*dx + sin*dy
	y1p := -sin*dx + cos*dy
	// Grow the radii when they cannot reach.
	if l := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry); l > 1 {
		k := math.Sqrt(l)
		rx, ry = rx*k, ry*k
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	co := 0.0
	if den != 0 {
		co = math.Sqrt(math.Max(0, num/den))
	}
	if large == sweep {
		co = -co
	}
	cxp, cyp := co*rx*y1p/ry, -co*ry*x1p/rx
	cx := cos*cxp - sin*cyp + (x0+x1)/2
	cy := sin*cxp + cos*cyp + (y0+y1)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t0 := angle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dt := angle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	// The parts per turn, at least four, from the radius over a tolerance
	// of a tenth of a pixel.
	perTurn := math.Max(math.Pow(1.1163*math.Max(rx, ry)/0.1, 1.0/6), 3.999999)
	n := int(math.Ceil(perTurn * math.Abs(dt) / (2 * math.Pi)))
	n = max(n, 1)
	step := dt / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	point := func(t float64) (float64, float64, float64, float64) {
		st, ct := math.Sincos(t)
		ex, ey := rx*ct, ry*st
		// The derivative along t.
		dx, dy := -rx*st, ry*ct
		return cx + cos*ex - sin*ey, cy + sin*ex + cos*ey, cos*dx - sin*dy, sin*dx + cos*dy
	}
	t := t0
	px, py, pdx, pdy := point(t)
	for i := range n {
		t += step
		qx, qy, qdx, qdy := point(t)
		if i == n-1 {
			qx, qy = x1, y1
		}
		s.CubicTo(px+k*pdx, py+k*pdy, qx-k*qdx, qy-k*qdy, qx, qy)
		px, py, pdx, pdy = qx, qy, qdx, qdy
	}
}
