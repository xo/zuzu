package geo

import "math"

// Graticule makes the lines of a grid of meridians and parallels. The
// major lines span the major extent, and the minor lines, at a finer step,
// span the minor extent. Each line has a point at each step of precision
// degrees, and at each of its ends.
type Graticule struct {
	// MajorExtent and MinorExtent are [[x0, y0], [x1, y1]] in degrees.
	MajorExtent, MinorExtent [2][2]float64
	// MajorStep and MinorStep are the steps in longitude and latitude.
	MajorStep, MinorStep [2]float64
	Precision            float64
}

// NewGraticule returns the graticule of Vega: major lines every 90° of
// longitude, minor lines every 10°, and the minor parallels stop at 80°.
func NewGraticule() *Graticule {
	return &Graticule{
		MajorExtent: [2][2]float64{{-180, -90 + epsilon}, {180, 90 - epsilon}},
		MinorExtent: [2][2]float64{{-180, -80 - epsilon}, {180, 80 + epsilon}},
		MajorStep:   [2]float64{90, 360},
		MinorStep:   [2]float64{10, 10},
		Precision:   2.5,
	}
}

// rangeOf returns the numbers from start, by step, below stop.
func rangeOf(start, stop, step float64) []float64 {
	n := int(math.Max(0, math.Ceil((stop-start)/step)))
	out := make([]float64, n)
	for i := range out {
		out[i] = start + float64(i)*step
	}
	return out
}

// ordered returns an extent with each pair of bounds in order.
func ordered(e [2][2]float64) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = e[0][0], e[0][1], e[1][0], e[1][1]
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return
}

// meridian returns the points of a meridian from y0 to y1, at a step of dy.
func meridian(x, y0, y1, dy float64) []any {
	ys := append(rangeOf(y0, y1-epsilon, dy), y1)
	out := make([]any, len(ys))
	for i, y := range ys {
		out[i] = []any{x, y}
	}
	return out
}

// parallel returns the points of a parallel from x0 to x1, at a step of dx.
func parallel(y, x0, x1, dx float64) []any {
	xs := append(rangeOf(x0, x1-epsilon, dx), x1)
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = []any{x, y}
	}
	return out
}

// Lines returns the graticule as a GeoJSON MultiLineString: the major
// meridians and parallels first, then the minor ones that are not major.
func (g *Graticule) Lines() map[string]any {
	X0, Y0, X1, Y1 := ordered(g.MajorExtent)
	x0, y0, x1, y1 := ordered(g.MinorExtent)
	DX, DY := g.MajorStep[0], g.MajorStep[1]
	dx, dy := g.MinorStep[0], g.MinorStep[1]
	var lines []any
	for _, x := range rangeOf(math.Ceil(X0/DX)*DX, X1, DX) {
		lines = append(lines, meridian(x, Y0, Y1, 90))
	}
	for _, y := range rangeOf(math.Ceil(Y0/DY)*DY, Y1, DY) {
		lines = append(lines, parallel(y, X0, X1, g.Precision))
	}
	for _, x := range rangeOf(math.Ceil(x0/dx)*dx, x1, dx) {
		if math.Abs(math.Mod(x, DX)) > epsilon {
			lines = append(lines, meridian(x, y0, y1, 90))
		}
	}
	for _, y := range rangeOf(math.Ceil(y0/dy)*dy, y1, dy) {
		if math.Abs(math.Mod(y, DY)) > epsilon {
			lines = append(lines, parallel(y, x0, x1, g.Precision))
		}
	}
	if lines == nil {
		lines = []any{}
	}
	return map[string]any{"type": multiLineString, "coordinates": lines}
}
