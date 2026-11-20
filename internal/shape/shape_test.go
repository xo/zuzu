package shape

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
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
		got, ok := Curve(c.name, pts, true, 0)
		if !ok {
			t.Fatalf("%s is not a curve", c.name)
		}
		if !same(t, got, c.want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// commands returns the commands of a path in order, without the commands
// that close a path when noClose is true.
func commands(d string, noClose bool) string {
	var b strings.Builder
	for _, r := range d {
		if strings.ContainsRune("MLCZ", r) && (!noClose || r != 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splinePoints are the points of TestSplinesMatchD3. dup repeats a point,
// which puts the control point of a Catmull-Rom segment on its end.
var splinePoints = map[string][][2]float64{
	"five":  {{0, 0}, {20, 30}, {40, 10}, {60, 40}, {80, 20}},
	"three": {{10, 10}, {50, 60}, {90, 20}},
	"two":   {{10, 10}, {50, 60}},
	"one":   {{10, 10}},
	"dup":   {{0, 0}, {0, 0}, {40, 10}, {60, 40}, {60, 40}, {80, 20}},
}

// TestSplinesMatchD3 checks the open and the closed curves against the
// lines that d3-shape 3.2.1 draws through the same points, with the curves
// that Vega gives each interpolation. A tension of zero is the default of
// Vega, as for the other curves.
func TestSplinesMatchD3(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, set string
		tension   float64
		want      string
	}{
		{"basis-open", "five", 0, "M20,21.667C26.667,23.333,33.333,16.667,40,18.333C46.667,20,53.333,30,60,31.667"},
		{"basis-open", "three", 0, "M50,45Z"},
		{"basis-open", "two", 0, ""},
		{"basis-open", "one", 0, ""},
		{"basis-open", "dup", 0, "M6.667,1.667C13.333,3.333,26.667,6.667,36.667,13.333C46.667,20,53.333,30,56.667,35C60,40,60,40,63.333,36.667"},
		{"basis-closed", "five", 0, "M20,21.667C26.667,23.333,33.333,16.667,40,18.333C46.667,20,53.333,30,60,31.667C66.667,33.333,73.333,26.667,63.333,20C53.333,13.333,26.667,6.667,16.667,8.333C6.667,10,13.333,20,20,21.667"},
		{"basis-closed", "three", 0, "M50,45C63.333,46.667,76.667,33.333,70,25C63.333,16.667,36.667,13.333,30,20C23.333,26.667,36.667,43.333,50,45"},
		{"basis-closed", "two", 0, "M36.667,43.333L23.333,26.667Z"},
		{"basis-closed", "one", 0, "M10,10Z"},
		{"basis-closed", "dup", 0, "M6.667,1.667C13.333,3.333,26.667,6.667,36.667,13.333C46.667,20,53.333,30,56.667,35C60,40,60,40,63.333,36.667C66.667,33.333,73.333,26.667,63.333,20C53.333,13.333,26.667,6.667,13.333,3.333C0,0,0,0,6.667,1.667"},
		{"cardinal-open", "five", 0, "M20,30C26.667,31.667,33.333,8.333,40,10C46.667,11.667,53.333,38.333,60,40"},
		{"cardinal-open", "three", 0, "M50,60Z"},
		{"cardinal-open", "two", 0, ""},
		{"cardinal-open", "one", 0, ""},
		{"cardinal-open", "dup", 0, "M0,0C6.667,1.667,30,3.333,40,10C50,16.667,56.667,35,60,40C63.333,45,56.667,43.333,60,40"},
		{"cardinal-open", "five", 0.3, "M20,30C24.667,31.167,35.333,8.833,40,10C44.667,11.167,55.333,38.833,60,40"},
		{"cardinal-closed", "five", 0, "M20,30C26.667,31.667,33.333,8.333,40,10C46.667,11.667,53.333,38.333,60,40C66.667,41.667,90,26.667,80,20C70,13.333,10,-1.667,0,0C-10,1.667,13.333,28.333,20,30"},
		{"cardinal-closed", "three", 0, "M50,60C63.333,61.667,96.667,28.333,90,20C83.333,11.667,16.667,3.333,10,10C3.333,16.667,36.667,58.333,50,60"},
		{"cardinal-closed", "two", 0, "M50,60L10,10Z"},
		{"cardinal-closed", "one", 0, "M10,10Z"},
		{"cardinal-closed", "dup", 0, "M0,0C6.667,1.667,30,3.333,40,10C50,16.667,56.667,35,60,40C63.333,45,56.667,43.333,60,40C63.333,36.667,90,26.667,80,20C70,13.333,13.333,3.333,0,0C-13.333,-3.333,-6.667,-1.667,0,0"},
		{"cardinal-closed", "five", 0.3, "M20,30C24.667,31.167,35.333,8.833,40,10C44.667,11.167,55.333,38.833,60,40C64.667,41.167,87,24.667,80,20C73,15.333,7,-1.167,0,0C-7,1.167,15.333,28.833,20,30"},
		{"catmull-rom-open", "five", 0, "M20,30C26.309,30.625,33.691,9.375,40,10C47.123,10.705,52.877,39.295,60,40"},
		{"catmull-rom-open", "three", 0, "M50,60Z"},
		{"catmull-rom-open", "two", 0, ""},
		{"catmull-rom-open", "one", 0, ""},
		{"catmull-rom-open", "dup", 0, "M0,0C0,0,29.873,2.863,40,10C49.47,16.674,60,40,60,40C60,40,60,40,60,40"},
		{"catmull-rom-open", "five", 0.3, "M20,30C26.441,31.025,33.559,8.975,40,10C46.928,11.102,53.072,38.898,60,40"},
		{"catmull-rom-closed", "five", 0, "M20,30C26.309,30.625,33.691,9.375,40,10C47.123,10.705,52.877,39.295,60,40C66.309,40.625,81.564,25.646,80,20C77.33,10.359,4.545,-6.45,0,0C-3.006,4.265,12.877,29.295,20,30"},
		{"catmull-rom-closed", "three", 0, "M50,60C62.945,60.717,92.926,28.528,90,20C86.507,9.819,14.655,1.682,10,10C5.851,17.413,36.227,59.237,50,60"},
		{"catmull-rom-closed", "two", 0, "M50,60L10,10Z"},
		{"catmull-rom-closed", "one", 0, "M10,10Z"},
		{"catmull-rom-closed", "dup", 0, "M0,0C0,0,29.873,2.863,40,10C49.47,16.674,60,40,60,40C60,40,60,40,60,40C60,40,81.564,25.646,80,20C77.33,10.359,0,0,0,0C0,0,0,0,0,0"},
		{"catmull-rom-closed", "five", 0.3, "M20,30C26.441,31.025,33.559,8.975,40,10C46.928,11.102,53.072,38.898,60,40C66.441,41.025,84.269,25.897,80,20C74.115,11.871,6.887,-4.278,0,0C-5.374,3.338,13.072,28.898,20,30"},
	} {
		got, ok := Curve(c.name, splinePoints[c.set], true, c.tension)
		if !ok {
			t.Fatalf("%s is not a curve", c.name)
		}
		if !same(t, got, c.want) || commands(got, false) != commands(c.want, false) {
			t.Errorf("%s through %s with %v:\n got %s\nwant %s", c.name, c.set, c.tension, got, c.want)
		}
	}
}

// TestSplineAreasMatchD3 checks an area with the open and the closed
// curves against the area that d3-shape 3.2.1 draws, with a base at y 100.
// A closed curve also closes the base, and an open one draws the base from
// the end of the top. The commands that close a path are left out of the
// comparison, because the area closes a path that d3-shape leaves open.
func TestSplineAreasMatchD3(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, set, want string
	}{
		{"basis-open", "five", "M20,21.667C26.667,23.333,33.333,16.667,40,18.333C46.667,20,53.333,30,60,31.667L60,100C53.333,100,46.667,100,40,100C33.333,100,26.667,100,20,100Z"},
		{"basis-open", "three", "M50,45L50,100Z"},
		{"basis-open", "two", ""},
		{"basis-closed", "five", "M20,21.667C26.667,23.333,33.333,16.667,40,18.333C46.667,20,53.333,30,60,31.667C66.667,33.333,73.333,26.667,63.333,20C53.333,13.333,26.667,6.667,16.667,8.333C6.667,10,13.333,20,20,21.667M60,100C53.333,100,46.667,100,40,100C33.333,100,26.667,100,20,100C13.333,100,6.667,100,16.667,100C26.667,100,53.333,100,63.333,100C73.333,100,66.667,100,60,100"},
		{"basis-closed", "two", "M36.667,43.333L23.333,26.667ZM23.333,100L36.667,100Z"},
		{"cardinal-open", "five", "M20,30C26.667,31.667,33.333,8.333,40,10C46.667,11.667,53.333,38.333,60,40L60,100C53.333,100,46.667,100,40,100C33.333,100,26.667,100,20,100Z"},
		{"cardinal-closed", "three", "M50,60C63.333,61.667,96.667,28.333,90,20C83.333,11.667,16.667,3.333,10,10C3.333,16.667,36.667,58.333,50,60M50,100C36.667,100,3.333,100,10,100C16.667,100,83.333,100,90,100C96.667,100,63.333,100,50,100"},
		{"catmull-rom-open", "five", "M20,30C26.309,30.625,33.691,9.375,40,10C47.123,10.705,52.877,39.295,60,40L60,100C53.333,100,46.667,100,40,100C33.333,100,26.667,100,20,100Z"},
		{"catmull-rom-closed", "five", "M20,30C26.309,30.625,33.691,9.375,40,10C47.123,10.705,52.877,39.295,60,40C66.309,40.625,81.564,25.646,80,20C77.33,10.359,4.545,-6.45,0,0C-3.006,4.265,12.877,29.295,20,30M60,100C53.333,100,46.667,100,40,100C33.333,100,26.667,100,20,100C13.333,100,0,100,0,100C0,100,80,100,80,100C80,100,66.667,100,60,100"},
	} {
		pts := splinePoints[c.set]
		items := make([]scene.Item, len(pts))
		for i, p := range pts {
			items[i] = scene.Item{X: p[0], Y: p[1], Y2: 100, Interpolate: c.name}
		}
		got, ok := Area(items)
		if !ok {
			t.Fatalf("%s is not a curve", c.name)
		}
		if !same(t, got, c.want) || commands(got, true) != commands(c.want, true) {
			t.Errorf("an area of %s through %s:\n got %s\nwant %s", c.name, c.set, got, c.want)
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
		if got := Arc(c.r0, c.r1, c.start, c.end, c.pad, c.corner); !same(t, got, c.want) {
			t.Errorf("arc %v:\n got %s\nwant %s", c, got, c.want)
		}
	}
}

// TestShapesScaleACustomPath checks that a custom shape scales its
// coordinates and leaves the flags of an arc alone.
func TestShapesScaleACustomPath(t *testing.T) {
	t.Parallel()
	got, ok := Symbol("M-1,-1L1,1A1,1 0 1,0 0,1Z", 16)
	if !ok {
		t.Fatal("the custom shape is not a shape")
	}
	if want := "M-2 -2L2 2A2 2 0 1 0 0 2Z"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
