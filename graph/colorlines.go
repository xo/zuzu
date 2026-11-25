package graph

import (
	"math"
	"strings"

	"github.com/xo/zuzu/internal/shape"
)

// ColorLines returns the paths of the lines of an edge with a list of n
// colors, as a list of colors of DOT, such as red:blue, draws it, from the
// path of its route. With no fractions, each color draws a line of its own
// beside the route, gap apart, the first at the right of the route as it
// runs from its tail. With fractions, the colors draw one after another
// along the route, from the tail, each over its share of the length of the
// route, from the weights of the list, -1 for a color with no weight, as
// shares says. Each path is a line through points along the route. Lines
// side by side stop tail and head pixels before the ends of the route, so
// that they end at the arrowheads, as in Graphviz. It returns nil for a
// route that does not read.
func ColorLines(path string, n int, weights []float64, gap, tail, head float64) []string {
	pts := flatten(path)
	if len(pts) < 2 || n < 1 {
		return nil
	}
	total := 0.0
	for j := 1; j < len(pts); j++ {
		total += math.Hypot(pts[j][0]-pts[j-1][0], pts[j][1]-pts[j-1][1])
	}
	out := make([]string, n)
	if weights == nil {
		if tail+head < total {
			pts = cutLine(pts, tail, total-head)
		}
		for k := range n {
			out[k] = polyline(offsetLine(pts, (float64(n-1)/2-float64(k))*gap))
		}
		return out
	}
	from := 0.0
	for k, f := range shares(weights, n) {
		to := math.Min(from+f, 1)
		if k == n-1 {
			to = 1
		}
		out[k] = polyline(cutLine(pts, from*total, to*total))
		from = to
	}
	return out
}

// shares returns the share of the length of an edge of each of n colors
// in series, from their weights in a list of colors of DOT, with -1 for a
// color with no weight. The colors with no weight share the rest of the
// length in equal parts. When each color has a weight and they add up to
// less than 1, the last color takes the rest.
func shares(weights []float64, n int) []float64 {
	out := make([]float64, n)
	sum, none := 0.0, 0
	for k := range n {
		w := -1.0
		if k < len(weights) {
			w = weights[k]
		}
		if w < 0 {
			none++
			continue
		}
		out[k] = w
		sum += w
	}
	rest := math.Max(1-sum, 0)
	switch {
	case none > 0:
		for k := range n {
			if k >= len(weights) || weights[k] < 0 {
				out[k] = rest / float64(none)
			}
		}
	case n > 0:
		out[n-1] += rest
	}
	return out
}

// flatSink reads a path into points, with each curve as a line through
// points along it.
type flatSink struct {
	pts [][2]float64
}

func (s *flatSink) MoveTo(x, y float64) { s.pts = append(s.pts, [2]float64{x, y}) }

func (s *flatSink) LineTo(x, y float64) { s.pts = append(s.pts, [2]float64{x, y}) }

func (s *flatSink) QuadTo(x1, y1, x, y float64) {
	p := s.last()
	for i := 1; i <= 8; i++ {
		t := float64(i) / 8
		u := 1 - t
		s.pts = append(s.pts, [2]float64{u*u*p[0] + 2*u*t*x1 + t*t*x, u*u*p[1] + 2*u*t*y1 + t*t*y})
	}
}

func (s *flatSink) CubicTo(x1, y1, x2, y2, x, y float64) {
	p := s.last()
	for i := 1; i <= 16; i++ {
		t := float64(i) / 16
		u := 1 - t
		a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		s.pts = append(s.pts, [2]float64{a*p[0] + b*x1 + c*x2 + d*x, a*p[1] + b*y1 + c*y2 + d*y})
	}
}

func (s *flatSink) Close() {}

// last returns the last point of the path, or 0, 0 for none.
func (s *flatSink) last() [2]float64 {
	if len(s.pts) == 0 {
		return [2]float64{}
	}
	return s.pts[len(s.pts)-1]
}

// flatten returns the points of a path, with each curve as a line through
// points along it, and without points that repeat the one before.
func flatten(path string) [][2]float64 {
	s := &flatSink{}
	if shape.Parse(path, s) != nil && len(s.pts) == 0 {
		return nil
	}
	var out [][2]float64
	for _, p := range s.pts {
		if n := len(out); n > 0 && out[n-1] == p {
			continue
		}
		out = append(out, p)
	}
	return out
}

// offsetLine returns a line moved d to the right of its run, each point
// along the mean of the normals of the two parts that meet at it.
func offsetLine(pts [][2]float64, d float64) [][2]float64 {
	if d == 0 {
		return pts
	}
	normal := func(a, b [2]float64) [2]float64 {
		dx, dy := b[0]-a[0], b[1]-a[1]
		l := math.Hypot(dx, dy)
		if l == 0 {
			return [2]float64{}
		}
		// The right of the run, with y down.
		return [2]float64{-dy / l, dx / l}
	}
	out := make([][2]float64, len(pts))
	for j, p := range pts {
		var nx, ny float64
		if j > 0 {
			v := normal(pts[j-1], p)
			nx, ny = nx+v[0], ny+v[1]
		}
		if j < len(pts)-1 {
			v := normal(p, pts[j+1])
			nx, ny = nx+v[0], ny+v[1]
		}
		if l := math.Hypot(nx, ny); l > 0 {
			nx, ny = nx/l, ny/l
		}
		out[j] = [2]float64{p[0] + nx*d, p[1] + ny*d}
	}
	return out
}

// cutLine returns the part of a line from the length from to the length
// to along it.
func cutLine(pts [][2]float64, from, to float64) [][2]float64 {
	var out [][2]float64
	at := 0.0
	for j := 1; j < len(pts); j++ {
		a, b := pts[j-1], pts[j]
		l := math.Hypot(b[0]-a[0], b[1]-a[1])
		point := func(s float64) [2]float64 {
			t := (s - at) / l
			return [2]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])}
		}
		if l > 0 && at+l >= from && at <= to {
			if len(out) == 0 {
				out = append(out, point(math.Max(from, at)))
			}
			out = append(out, point(math.Min(to, at+l)))
		}
		at += l
	}
	return out
}

// polyline returns the path of a line through points.
func polyline(pts [][2]float64) string {
	var b strings.Builder
	for j, p := range pts {
		if j == 0 {
			b.WriteString("M")
		} else {
			b.WriteString("L")
		}
		b.WriteString(num(p[0]) + "," + num(p[1]))
	}
	return b.String()
}
