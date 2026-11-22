package shape

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
)

// recorder writes the segments of a path as text, with two decimals.
type recorder struct {
	b strings.Builder
	// pts holds each point that a segment reaches, for the arcs.
	pts [][2]float64
}

func (r *recorder) MoveTo(x, y float64)                  { r.add("M", x, y) }
func (r *recorder) LineTo(x, y float64)                  { r.add("L", x, y) }
func (r *recorder) QuadTo(x1, y1, x, y float64)          { r.add("Q", x1, y1, x, y) }
func (r *recorder) CubicTo(x1, y1, x2, y2, x, y float64) { r.add("C", x1, y1, x2, y2, x, y) }
func (r *recorder) Close()                               { r.add("Z") }

func (r *recorder) add(c string, v ...float64) {
	r.b.WriteString(c)
	for i, x := range v {
		if i > 0 {
			r.b.WriteByte(' ')
		}
		fmt.Fprintf(&r.b, "%.2f", x+0)
	}
	if len(v) >= 2 {
		r.pts = append(r.pts, [2]float64{v[len(v)-2], v[len(v)-1]})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ d, want string }{
		{"M1,2L3,4H5V6Z", "M1.00 2.00L3.00 4.00L5.00 4.00L5.00 6.00Z"},
		// The numbers after a move are lines, and a command repeats.
		{"M0 0 1 1 2 0", "M0.00 0.00L1.00 1.00L2.00 0.00"},
		{"m1 1l1 0 0 1h-1z", "M1.00 1.00L2.00 1.00L2.00 2.00L1.00 2.00Z"},
		{"M0,0C1,1 2,1 3,0S5,-1 6,0", "M0.00 0.00C1.00 1.00 2.00 1.00 3.00 0.00C4.00 -1.00 5.00 -1.00 6.00 0.00"},
		{"M0,0Q1,1 2,0T4,0", "M0.00 0.00Q1.00 1.00 2.00 0.00Q3.00 -1.00 4.00 0.00"},
		{"M-1.5e1,.5-2,3", "M-15.00 0.50L-2.00 3.00"},
		// An arc with no radius is a line.
		{"M0,0A0,5 0 0,1 4,0", "M0.00 0.00L4.00 0.00"},
	} {
		var r recorder
		if err := Parse(c.d, &r); err != nil {
			t.Errorf("%s: %v", c.d, err)
			continue
		}
		if got := r.b.String(); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.d, got, c.want)
		}
	}
	for _, d := range []string{"L1,1", "M1", "M1,1A1,1 0 2,0 3,3", "M1,1X"} {
		if err := Parse(d, &recorder{}); err == nil {
			t.Errorf("%s gives no error", d)
		}
	}
}

// TestScalePath checks that ScalePath scales each x and each y apart, with
// relative commands and an arc, and keeps an error of the path.
func TestScalePath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		d, want string
		sx, sy  float64
	}{
		{"M0,0 C 0,20 0,40 0,60 20,40 40,20 60,0 Z", "M0,0C0,10 0,20 0,30C10,20 20,10 30,0Z", 0.5, 0.5},
		{"m 60,0 c 0,20 0,40 0,60 H 0", "M120,0C120,60 120,120 120,180L0,180", 2, 3},
		{"M0,0 Q 10,10 20,0", "M0,0Q10,-10 20,0", 1, -1},
	} {
		got, err := ScalePath(c.d, c.sx, c.sy)
		if err != nil || got != c.want {
			t.Errorf("ScalePath(%q, %v, %v) = %q, %v, want %q", c.d, c.sx, c.sy, got, err, c.want)
		}
	}
	got, err := ScalePath("M0,0A5,5 0 0,1 10,0", 2, 1)
	if err != nil || strings.Count(got, "C") != 2 || !strings.HasSuffix(got, "20,0") {
		t.Errorf("an arc scales to %q, %v, want two cubics that end at 20,0", got, err)
	}
	if _, err := ScalePath("L1,1", 2, 2); err == nil {
		t.Error("a path that does not start with a move gives no error")
	}
}

// TestLineBaselines checks that the baselines line-top and line-bottom of
// Vega are the top and the bottom of a box one line height high around
// the text, and that the lines of a text at line-bottom go up from it.
func TestLineBaselines(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		baseline string
		lines    int
		want     float64
	}{
		{"top", 1, 0.79 * 10},
		{"line-top", 1, 0.79*10 + 5},
		{"bottom", 1, -0.21 * 10},
		{"line-bottom", 1, -0.21*10 - 5},
		{"line-bottom", 2, -0.21*10 - 5 - 20},
		{"line-top", 2, 0.79*10 + 5},
	} {
		it := &scene.Item{FontSize: 10, LineHeight: 20, Baseline: c.baseline, Text: make(scene.Lines, c.lines)}
		if got := TextOf(it, nil).Y; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s with %d lines: y = %v, want %v", c.baseline, c.lines, got, c.want)
		}
	}
}

func TestParseArcs(t *testing.T) {
	t.Parallel()
	// A full circle in two arcs, and a quarter of a circle drawn with the
	// flags written together.
	for _, c := range []struct {
		d     string
		r     float64
		parts int
	}{
		{"M10,0A10,10 0 1,1 -10,0A10,10 0 1,1 10,0Z", 10, 4},
		{"M5,0a5 5 0 0010-0", 5, 2},
	} {
		var r recorder
		if err := Parse(c.d, &r); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(r.b.String(), "C"); n != c.parts {
			t.Errorf("%s: %d cubics, want %d", c.d, n, c.parts)
		}
		cx := 0.0
		if c.r == 5 {
			cx = 10
		}
		for _, p := range r.pts {
			if d := math.Hypot(p[0]-cx, p[1]); math.Abs(d-c.r) > 1e-9 {
				t.Errorf("%s: the point %v lies %v from the center, want %v", c.d, p, d, c.r)
			}
		}
	}
}
