package svg

import (
	"math"
	"regexp"
	"strconv"
	"testing"
)

// numbers returns the numbers of a path in order, so that two paths compare
// whatever separators they use.
func numbers(t *testing.T, d string) []float64 {
	t.Helper()
	var out []float64
	for _, m := range regexp.MustCompile(`-?[0-9]*\.?[0-9]+(?:e-?[0-9]+)?`).FindAllString(d, -1) {
		v, err := strconv.ParseFloat(m, 64)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// same reports whether two paths have the same numbers, within a rounding of
// three decimals.
func same(t *testing.T, got, want string) bool {
	t.Helper()
	a, b := numbers(t, got), numbers(t, want)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 0.002 {
			return false
		}
	}
	return true
}

// TestCurvesMatchD3 checks each curve against the path that d3-shape 3.2.0,
// in d3 7.9.0, draws through the same points with the same parameters.
func TestCurvesMatchD3(t *testing.T) {
	t.Parallel()
	pts := [][2]float64{{0, 0}, {20, 30}, {40, 10}, {60, 40}, {80, 20}}
	for _, c := range []struct {
		name string
		want string
	}{
		{"cardinal", "M0,0C0,0,13.333,28.333,20,30C26.667,31.667,33.333,8.333,40,10C46.667,11.667,53.333,38.333,60,40C66.667,41.667,80,20,80,20"},
		{"catmull-rom", "M0,0C0,0,12.877,29.295,20,30C26.309,30.625,33.691,9.375,40,10C47.123,10.705,52.877,39.295,60,40C66.309,40.625,80,20,80,20"},
		{"natural", "M0,0C6.667,15.952,13.333,31.905,20,30C26.667,28.095,33.333,8.333,40,10C46.667,11.667,53.333,34.762,60,40C66.667,45.238,73.333,32.619,80,20"},
		{"bundle", "M0,0L3.333,4.375C6.667,8.75,13.333,17.5,20,19.167C26.667,20.833,33.333,15.417,40,17.083C46.667,18.75,53.333,27.5,60,29.167C66.667,30.833,73.333,25.417,76.667,22.708L80,20"},
	} {
		got, err := curve(c.name, pts, true, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !same(t, got, c.want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// TestArcsMatchD3 checks an arc with a pad angle and round corners, and a
// slice of a pie, against the paths that d3-shape draws for them.
func TestArcsMatchD3(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		r0, r1, start, end, pad, corner float64
		want                            string
	}{
		{30, 50, 0, 2.1, 0.02, 4, "M0.583,-45.771A4,4,0,0,1,4.982,-49.751A50,50,0,0,1,45.461,20.817A4,4,0,0,1,39.804,22.604L29.376,16.505A4,4,0,0,1,27.701,11.516A30,30,0,0,0,4.044,-29.726A4,4,0,0,1,0.583,-33.69Z"},
		{0, 50, 0, 1, 0, 4, "M0,-45.826A4,4,0,0,1,4.348,-49.811A50,50,0,0,1,39.565,-30.571A4,4,0,0,1,38.561,-24.76L0,0Z"},
	} {
		if got := arcPath(c.r0, c.r1, c.start, c.end, c.pad, c.corner); !same(t, got, c.want) {
			t.Errorf("arc %v:\n got %s\nwant %s", c, got, c.want)
		}
	}
}

// TestShapesScaleACustomPath checks that a custom shape scales its
// coordinates and leaves the flags of an arc alone.
func TestShapesScaleACustomPath(t *testing.T) {
	t.Parallel()
	got, err := shape("M-1,-1L1,1A1,1 0 1,0 0,1Z", 16)
	if err != nil {
		t.Fatal(err)
	}
	if want := "M-2 -2L2 2A2 2 0 1 0 0 2Z"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
