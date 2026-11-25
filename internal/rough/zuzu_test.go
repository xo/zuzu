package rough

// This file is a file of zuzu, not of rough-go. It holds the tests of the
// changes of zuzu that the tests of upstream do not cover.

import (
	"math"
	"slices"
	"strconv"
	"testing"
)

// TestNonzeroFillsWhereTwoRingsOverlap checks change 3. Two squares that
// overlap and run the same way, joined by a bridge as zuzu joins the
// subpaths of a path, fill their overlap with the rule nonzero, and leave it
// empty with the rule evenodd. A square inside a square that runs the other
// way stays a hole with both rules.
func TestNonzeroFillsWhereTwoRingsOverlap(t *testing.T) {
	t.Parallel()
	// bridged joins rings as the sketch of zuzu joins them: each ring closes
	// to its start and comes back to the start of the first ring.
	bridged := func(rings ...[]Point) []Point {
		var out []Point
		for _, r := range rings {
			if len(out) > 0 {
				out = append(out, out[0])
			}
			out = append(out, r...)
			if len(out) > len(r) {
				out = append(out, r[0])
			}
		}
		return out
	}
	// spans returns the spans of the scan line at y, with the lines of the
	// hachure across, from left to right.
	spans := func(points []Point, rule string, y float64) [][2]float64 {
		o := defaultResolvedOptions()
		o.Roughness, o.HachureAngle, o.HachureGap, o.FillRule = 0, -90, 4, rule
		var out [][2]float64
		for _, l := range polygonHachureLines(points, &o) {
			if l[0][1] == y {
				out = append(out, [2]float64{math.Min(l[0][0], l[1][0]), math.Max(l[0][0], l[1][0])})
			}
		}
		return out
	}
	a := []Point{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	b := []Point{{20, 20}, {60, 20}, {60, 60}, {20, 60}}
	hole := []Point{{10, 10}, {10, 30}, {30, 30}, {30, 10}}
	for _, c := range []struct {
		name   string
		points []Point
		rule   string
		want   [][2]float64
	}{
		{"overlap, nonzero", bridged(a, b), "nonzero", [][2]float64{{0, 60}}},
		{"overlap, evenodd", bridged(a, b), "", [][2]float64{{0, 20}, {40, 60}}},
		{"hole, nonzero", bridged(a, hole), "nonzero", [][2]float64{{0, 10}, {30, 40}}},
		{"hole, evenodd", bridged(a, hole), "evenodd", [][2]float64{{0, 10}, {30, 40}}},
	} {
		const y = 28
		if got := spans(c.points, c.rule, y); !slices.Equal(got, c.want) {
			t.Errorf("%s: the spans at y %d are %v, want %v", c.name, y, got, c.want)
		}
	}
}

// drawAll draws a line, a curve, an ellipse and a hachured rectangle with the
// seed, each with a new generator, and returns their path data.
func drawAll(seed *float64) []string {
	var out []string
	draws := []func(g *Generator, o *Options) Drawable{
		func(g *Generator, o *Options) Drawable { return g.Line(0, 0, 120, 40, o) },
		func(g *Generator, o *Options) Drawable {
			return g.Curve([]Point{{0, 0}, {30, 40}, {60, 10}, {90, 50}}, o)
		},
		func(g *Generator, o *Options) Drawable { return g.Ellipse(40, 30, 60, 40, o) },
		func(g *Generator, o *Options) Drawable { return g.Rectangle(0, 0, 50, 30, o) },
	}
	for _, draw := range draws {
		g := NewGenerator(nil)
		o := &Options{Seed: seed, Fill: String("red")}
		for _, p := range g.ToPaths(draw(g, o)) {
			out = append(out, p.D)
		}
	}
	return out
}

// TestNoSeedIsRandom checks change 2: a seed that Rough.js replaces with
// Math.random gives the same paths each time. A seed of 0, NaN, a fraction
// under 1, a multiple of 2^32 or an infinity takes the seed 1, and so does
// the second stroke of a seed of -1, whose seed plus 1 is 0.
func TestNoSeedIsRandom(t *testing.T) {
	t.Parallel()
	seeds := []*float64{
		nil, Float64(0), Float64(math.NaN()), Float64(0.5), Float64(-0.5),
		Float64(1 << 32), Float64(math.Inf(1)), Float64(math.Inf(-1)), Float64(-1),
		Float64(-5), Float64(3.7), Float64(1<<31 + 5), Float64(1), Float64(2147483647),
	}
	for _, seed := range seeds {
		name := "nil"
		if seed != nil {
			name = strconv.FormatFloat(*seed, 'g', -1, 64)
		}
		first, second := drawAll(seed), drawAll(seed)
		if len(first) != len(second) {
			t.Fatalf("seed %s: the first draw has %d paths, and the second %d", name, len(first), len(second))
		}
		for i := range first {
			if first[i] != second[i] {
				t.Errorf("seed %s: path %d differs between two draws:\n%.200s\n%.200s", name, i, first[i], second[i])
			}
		}
	}
	// A line draws with one stream of draws, so a seed of 0 draws it as the
	// seed 1 does.
	g0, g1 := NewGenerator(nil), NewGenerator(nil)
	zero := g0.ToPaths(g0.Line(0, 0, 120, 40, &Options{Seed: Float64(0)}))
	one := g1.ToPaths(g1.Line(0, 0, 120, 40, &Options{Seed: Float64(1)}))
	if zero[0].D != one[0].D {
		t.Errorf("the seed 0 draws a line as\n%s\nand the seed 1 as\n%s", zero[0].D, one[0].D)
	}
}

// TestTheStateIsNeverZero checks the reason that change 2 holds for every
// later draw: from a state that is not 0, the next state is not 0 either.
func TestTheStateIsNeverZero(t *testing.T) {
	t.Parallel()
	for _, seed := range []float64{1, 2, -1, 65536, 1 << 31, 2147483647, -2147483648, 4294967295} {
		r := newRandom(seed)
		for i := range 100000 {
			v := r.next()
			if r.seed == 0 || v < 0 || v >= 1 {
				t.Fatalf("seed %v: draw %d gives %v with the state %v", seed, i, v, r.seed)
			}
		}
	}
}
