package geo

import "math"

// The projections of Peirce (1879) and of Guyou (1887) map a hemisphere
// conformally onto a square, and the world onto a square or onto two
// squares side by side. Lee, "Conformal projections based on elliptic
// functions", Cartographica monograph 16 (1976), gives both as aspects of
// one map of a hemisphere onto a square.
//
// Take the stereographic projection w of the hemisphere about its center,
// which fills the unit disk. The map of Schwarz and Christoffel
//
//	z = ∫₀^w dt / √(1 + t⁴)
//
// takes the disk onto a square with its corners where w⁴ = -1. With
// t = tan(θ/2) the integral is F(2 arctan w | ½) / 2, and in the form of
// Carlson it is w R_F((1-w²)², 1+w⁴, (1+w²)²), which this file uses.

// squareScale scales the square, so that its size matches the one of
// d3-geo-projection, which the projections of vega.js use. The scale of a
// projection in a spec depends on it.
var squareScale = 1 + math.Sqrt2/2

// squareHalf is half the side of the square of a hemisphere, the distance
// from its center to the middle of a side: the image of the point w = 1.
var squareHalf = squareScale * ellipticK(0.5) / 2

// hemisphereSquare maps the hemisphere about the point (0°, 0°) onto a
// square about the origin. The poles go to the middles of the top and the
// bottom sides, the points (±90°, 0°) to the middles of the other two
// sides, and the points (±90°, ±45°) to the corners, where the map is not
// conformal.
func hemisphereSquare(lambda, phi float64) (float64, float64) {
	cp := math.Cos(phi)
	d := 1 + cp*math.Cos(lambda)
	w := complex(cp*math.Sin(lambda)/d, math.Sin(phi)/d)
	w2 := w * w
	z := w * carlsonRF((1-w2)*(1-w2), 1+w2*w2, (1+w2)*(1+w2))
	return squareScale * real(z), squareScale * imag(z)
}

// guyouRaw is the projection of Guyou: the eastern hemisphere, about
// (90°, 0°), in a square on the right, and the western hemisphere in a
// square on the left. The poles are at the middles of the top and the
// bottom sides of each square. The two squares meet along the meridian of
// 0° between the latitudes of -45° and 45°.
func guyouRaw(lambda, phi float64) (float64, float64) {
	// The meridian of 0° goes to the west, as in d3-geo-projection.
	east := lambda > 0
	if !east {
		lambda = -lambda
	}
	x, y := hemisphereSquare(lambda-halfPi, phi)
	x += squareHalf
	if !east {
		x = -x
	}
	return x, y
}

// peirceRaw is the quincuncial projection of Peirce, with the hemisphere
// about (0°, 0°) in a square that stands on a corner. The four quarters of
// the other hemisphere fill the four triangles in the corners of a larger
// square. Each quarter meets the inner square along the side that they
// share, so that the map stays continuous across each side. The projection
// puts the north pole at the center by its rotation.
func peirceRaw(lambda, phi float64) (float64, float64) {
	front := math.Abs(lambda) <= halfPi
	if !front {
		if lambda > 0 {
			lambda -= math.Pi
		} else {
			lambda += math.Pi
		}
	}
	hx, hy := hemisphereSquare(lambda, phi)
	x, y := (hx-hy)/math.Sqrt2, (hx+hy)/math.Sqrt2
	if front {
		return x, y
	}
	// d is the distance from the center to a corner of the inner square.
	// A quarter in the lower left goes to the corner at the upper right,
	// and the other way round. A quarter in the upper left or the lower
	// right turns half a circle about the middle of its own corner. The
	// sign of a zero follows d3-geo-projection, at the antipode of the
	// center.
	d := math.Sqrt2 * squareHalf
	sx, sy := 1.0, -1.0
	if x < 0 {
		sx = -1
	}
	if y > 0 {
		sy = 1
	}
	return sy * (math.Abs(x) - d), sx * (math.Abs(y) - d)
}
