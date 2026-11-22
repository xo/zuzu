package geo

import (
	"math"
	"math/cmplx"
)

// This file holds the elliptic integrals that the projections of Peirce
// and of Guyou need. The formulas come from Abramowitz and Stegun,
// "Handbook of Mathematical Functions" (1964), chapter 17, and from
// Carlson, "Numerical computation of real or complex elliptic integrals",
// Numerical Algorithms 10 (1995), as the NIST Digital Library of
// Mathematical Functions gives them in sections 19.16 and 19.36.

// agm returns the arithmetic-geometric mean of two positive numbers
// (Abramowitz and Stegun 17.6).
func agm(a, b float64) float64 {
	for range 64 {
		if math.Abs(a-b) <= 1e-15*a {
			break
		}
		a, b = (a+b)/2, math.Sqrt(a*b)
	}
	return (a + b) / 2
}

// ellipticK returns the complete elliptic integral of the first kind of
// the parameter m, the square of the modulus, for m below 1. It is
// π / (2 agm(1, √(1-m))) (Abramowitz and Stegun 17.6.1).
func ellipticK(m float64) float64 {
	return math.Pi / (2 * agm(1, math.Sqrt(1-m)))
}

// carlsonRF returns the symmetric elliptic integral of the first kind
//
//	R_F(x, y, z) = ½ ∫₀^∞ dt / √((t+x)(t+y)(t+z))
//
// for complex numbers off the negative real axis, of which at most one is
// zero. It uses the duplication theorem of Carlson (DLMF 19.36.1): each
// step moves the three numbers closer to their mean, and a short series
// about the mean gives the value.
func carlsonRF(x, y, z complex128) complex128 {
	var a, dx, dy, dz complex128
	for range 64 {
		sx, sy, sz := cmplx.Sqrt(x), cmplx.Sqrt(y), cmplx.Sqrt(z)
		l := sx*sy + sy*sz + sz*sx
		x, y, z = (x+l)/4, (y+l)/4, (z+l)/4
		a = (x + y + z) / 3
		dx, dy, dz = 1-x/a, 1-y/a, 1-z/a
		// The error of the series is about the sixth power of the
		// distance from the mean.
		if max(cmplx.Abs(dx), cmplx.Abs(dy), cmplx.Abs(dz)) < 1e-3 {
			break
		}
	}
	e2 := dx*dy - dz*dz
	e3 := dx * dy * dz
	return (1 - e2/10 + e3/14 + e2*e2/24 - 3*e2*e3/44) / cmplx.Sqrt(a)
}
