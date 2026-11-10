// Package geo holds the cartographic projections of zuzu and the geographic
// paths that they draw. A projection maps longitude and latitude, in
// degrees, to the plane of the chart. The names and the defaults of the
// projections follow Vega, which takes them from d3-geo, and zuzu matches
// the numbers of vega.js (D6, D17).
package geo

import (
	"math"
	"strconv"
)

const (
	epsilon  = 1e-6
	epsilon2 = 1e-12
	halfPi   = math.Pi / 2
	quarter  = math.Pi / 4
	tau      = 2 * math.Pi
	radians  = math.Pi / 180
	degrees  = 180 / math.Pi
)

// vec is a point in three dimensions, on the unit sphere where a point of
// the globe lies.
type vec [3]float64

// cartesian returns the place on the unit sphere of a longitude and a
// latitude in radians.
func cartesian(lambda, phi float64) vec {
	c := math.Cos(phi)
	return vec{c * math.Cos(lambda), c * math.Sin(lambda), math.Sin(phi)}
}

// spherical returns the longitude and the latitude of a place on the unit
// sphere.
func spherical(v vec) (float64, float64) {
	return math.Atan2(v[1], v[0]), asin(v[2])
}

func (a vec) dot(b vec) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func (a vec) cross(b vec) vec {
	return vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func (a vec) add(b vec) vec { return vec{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func (a vec) scale(k float64) vec { return vec{a[0] * k, a[1] * k, a[2] * k} }

// normalize returns a with a length of 1.
func (a vec) normalize() vec {
	l := math.Sqrt(a.dot(a))
	return vec{a[0] / l, a[1] / l, a[2] / l}
}

// asin is the arcsine, with the argument held to the range from -1 to 1.
func asin(x float64) float64 {
	switch {
	case x > 1:
		return halfPi
	case x < -1:
		return -halfPi
	}
	return math.Asin(x)
}

// acos is the arccosine, with the argument held to the range from -1 to 1.
func acos(x float64) float64 {
	switch {
	case x > 1:
		return 0
	case x < -1:
		return math.Pi
	}
	return math.Acos(x)
}

// sign returns 1 for a positive number, -1 for a negative one, and 0
// otherwise.
func sign(x float64) float64 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}

// adder sums numbers with the error of each sum kept apart, so that a long
// sum keeps its precision.
type adder struct {
	partials []float64
}

func (a *adder) add(x float64) {
	i := 0
	for _, y := range a.partials {
		if math.Abs(x) < math.Abs(y) {
			x, y = y, x
		}
		hi := x + y
		lo := y - (hi - x)
		if lo != 0 {
			a.partials[i] = lo
			i++
		}
		x = hi
	}
	a.partials = append(a.partials[:i], x)
}

func (a *adder) value() float64 {
	n := len(a.partials)
	if n == 0 {
		return 0
	}
	n--
	hi := a.partials[n]
	for n > 0 {
		x := hi
		n--
		y := a.partials[n]
		hi = x + y
		lo := y - (hi - x)
		if lo != 0 {
			if n > 0 && ((lo < 0 && a.partials[n-1] < 0) || (lo > 0 && a.partials[n-1] > 0)) {
				y = lo * 2
				x = hi + y
				if y == x-hi {
					hi = x
				}
			}
			break
		}
	}
	return hi
}

// Number reads a coordinate as a number, as JavaScript does in arithmetic.
// A value that is not a number gives NaN.
func Number(v any) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case nil:
		return 0
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			if v == "" {
				return 0
			}
			return math.NaN()
		}
		return f
	}
	return math.NaN()
}
