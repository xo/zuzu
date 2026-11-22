package geo

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestEllipticK(t *testing.T) {
	t.Parallel()
	tests := []struct {
		m, want float64
	}{
		{0, math.Pi / 2},
		// K(½) = Γ(¼)² / (4√π), the lemniscate case.
		{0.5, math.Gamma(0.25) * math.Gamma(0.25) / (4 * math.Sqrt(math.Pi))},
		// Abramowitz and Stegun, table 17.1.
		{0.1, 1.6124413487202194},
		{0.9, 2.5780921133481733},
	}
	for _, test := range tests {
		if got := ellipticK(test.m); math.Abs(got-test.want) > 1e-14*test.want {
			t.Errorf("K(%v) = %v, want %v", test.m, got, test.want)
		}
	}
}

// The values come from Carlson (1995), section 3, which tests the
// algorithm with them.
func TestCarlsonRF(t *testing.T) {
	t.Parallel()
	tests := []struct {
		x, y, z, want complex128
	}{
		{1, 2, 0, 1.3110287771461},
		{1i, -1i, 0, 1.8540746773014},
		{-1 + 1i, 1i, 0, 0.79612586584234 - 1.2138566698365i},
		{2, 3, 4, 0.58408284167715},
		{1i, -1i, 2, 1.0441445654064},
		{-1 + 1i, 1i, 1 - 1i, 0.93912050218619 - 0.53296252018635i},
	}
	for _, test := range tests {
		if got := carlsonRF(test.x, test.y, test.z); cmplx.Abs(got-test.want) > 1e-13 {
			t.Errorf("R_F(%v, %v, %v) = %v, want %v", test.x, test.y, test.z, got, test.want)
		}
	}
}

// tier2Jacobi returns sn, cn and dn of a real u for the parameter m below 1,
// by the descending Landen transformation, which the arithmetic-geometric
// mean gives (Abramowitz and Stegun 16.4). It takes dn as √(1 - m sn²)
// (16.9.1), which keeps its precision where cn is near 0.
func tier2Jacobi(u, m float64) (sn, cn, dn float64) {
	a, b, c := []float64{1}, math.Sqrt(1-m), []float64{math.Sqrt(m)}
	for math.Abs(c[len(c)-1]) > 1e-16 && len(a) < 32 {
		an := a[len(a)-1]
		a = append(a, (an+b)/2)
		c = append(c, (an-b)/2)
		b = math.Sqrt(an * b)
	}
	n := len(a) - 1
	phi := math.Ldexp(a[n]*u, n)
	for i := n; i > 0; i-- {
		phi = (phi + math.Asin(c[i]/a[i]*math.Sin(phi))) / 2
	}
	sn = math.Sin(phi)
	return sn, math.Cos(phi), math.Sqrt(1 - m*sn*sn)
}

// tier2JacobiComplex returns sn and cn of a complex u by the addition theorem
// and the imaginary transformation of Jacobi (Abramowitz and Stegun
// 16.21).
func tier2JacobiComplex(u complex128, m float64) (sn, cn complex128) {
	s, c, d := tier2Jacobi(real(u), m)
	s1, c1, d1 := tier2Jacobi(imag(u), 1-m)
	delta := c1*c1 + m*s*s*s1*s1
	sn = complex(s*d1, c*d*s1*c1) / complex(delta, 0)
	cn = complex(c*c1, -s*d*s1*d1) / complex(delta, 0)
	return sn, cn
}

func TestJacobi(t *testing.T) {
	t.Parallel()
	for _, m := range []float64{0, 0.3, 0.5, 0.9} {
		k := ellipticK(m)
		sn, cn, dn := tier2Jacobi(k, m)
		if math.Abs(sn-1) > 1e-14 || math.Abs(cn) > 1e-7 || math.Abs(dn-math.Sqrt(1-m)) > 1e-14 {
			t.Errorf("m %v: sn, cn, dn(K) = %v, %v, %v", m, sn, cn, dn)
		}
		// sn(K/2) = 1/√(1+√(1-m)) (Abramowitz and Stegun 16.5).
		if sn, _, _ := tier2Jacobi(k/2, m); math.Abs(sn-1/math.Sqrt(1+math.Sqrt(1-m))) > 1e-14 {
			t.Errorf("m %v: sn(K/2) = %v", m, sn)
		}
	}
	if sn, cn, _ := tier2Jacobi(0.7, 0); math.Abs(sn-math.Sin(0.7)) > 1e-15 || math.Abs(cn-math.Cos(0.7)) > 1e-15 {
		t.Errorf("sn, cn(0.7 | 0) = %v, %v", sn, cn)
	}
}

// TestHemisphereSquare inverts the map onto the square with the Jacobi
// functions of a complex argument: z = F(φ | ½) / 2 with tan(φ/2) = w, so
// w = sn(2z) / (1 + cn(2z)). The integral and the functions come from
// different formulas, so they test each other.
func TestHemisphereSquare(t *testing.T) {
	t.Parallel()
	for lon := -90.0; lon <= 90; lon += 7.5 {
		for lat := -89.0; lat <= 89; lat += 8 {
			l, f := lon*radians, lat*radians
			x, y := hemisphereSquare(l, f)
			sn, cn := tier2JacobiComplex(2*complex(x, y)/complex(squareScale, 0), 0.5)
			got := sn / (1 + cn)
			cp := math.Cos(f)
			d := 1 + cp*math.Cos(l)
			want := complex(cp*math.Sin(l)/d, math.Sin(f)/d)
			if cmplx.Abs(got-want) > 1e-12 {
				t.Errorf("(%v, %v): w = %v, want %v", lon, lat, got, want)
			}
		}
	}
	// The middles of the sides and the corners of the square.
	s := squareHalf
	for _, test := range []struct{ lon, lat, x, y float64 }{
		{0, 0, 0, 0},
		{90, 0, s, 0},
		{-90, 0, -s, 0},
		{0, 90, 0, s},
		{0, -90, 0, -s},
		{90, 45, s, s},
		{-90, -45, -s, -s},
	} {
		x, y := hemisphereSquare(test.lon*radians, test.lat*radians)
		// A corner is where the map is a square root, so a rounding
		// error of the input grows to about its square root.
		if math.Abs(x-test.x) > 1e-7 || math.Abs(y-test.y) > 1e-7 {
			t.Errorf("(%v, %v) = %v, %v, want %v, %v", test.lon, test.lat, x, y, test.x, test.y)
		}
	}
}
