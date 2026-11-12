package scale

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
)

func TestBand(t *testing.T) {
	t.Parallel()
	o := Defaults("band")
	o.Domain = []any{"a", "b", "c"}
	o.Range = []any{0.0, 120.0}
	o.PaddingInner, o.HasPaddingInner = 0.2, true
	o.PaddingOuter, o.HasPaddingOuter = 0.1, true
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	// The step is 120 / (3 - 0.2 + 0.2) = 40, and a band 32.
	if got := s.Bandwidth(); math.Abs(got-32) > 1e-9 {
		t.Errorf("Bandwidth = %v, want 32", got)
	}
	if got := s.Map("b"); math.Abs(expr.Number(got)-44) > 1e-9 {
		t.Errorf("Map(b) = %v, want 44", got)
	}
	if got := s.Map("z"); !expr.IsUndefined(got) {
		t.Errorf("Map(z) = %v, want undefined", got)
	}
}

func TestLinear(t *testing.T) {
	t.Parallel()
	o := Defaults("linear")
	o.Domain = []any{3.0, 97.0}
	o.Range = []any{0.0, 100.0}
	o.Nice = true
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	// Zero is in the domain by default, and nice rounds the end to 100.
	if got := s.Domain(); !slices.Equal(got, []any{0.0, 100.0}) {
		t.Errorf("Domain = %v, want [0 100]", got)
	}
	if got := s.Map(25.0); got != 25.0 {
		t.Errorf("Map(25) = %v, want 25", got)
	}
	if got := s.Invert(50); got != 50.0 {
		t.Errorf("Invert(50) = %v, want 50", got)
	}
	if got := s.Ticks(5); !slices.Equal(got, []any{0.0, 20.0, 40.0, 60.0, 80.0, 100.0}) {
		t.Errorf("Ticks(5) = %v", got)
	}
	o.Range = []any{"#000000", "#ffffff"}
	o.Scheme = true
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if c.Type() != "sequential-linear" {
		t.Errorf("Type = %s, want sequential-linear", c.Type())
	}
	if got := c.Map(50.0); got != "#808080" {
		t.Errorf("Map(50) = %v, want #808080", got)
	}
}

// TestDivergingScheme makes sure that a scheme of many colors spreads over
// a domain of three values, so that the middle value takes the middle
// color and the last value the last color, not the third.
func TestDivergingScheme(t *testing.T) {
	t.Parallel()
	o := Defaults("linear")
	o.Domain = []any{-6.0, 0.0, 6.0}
	o.Range = []any{"#000000", "#202020", "#404040", "#606060", "#808080", "#a0a0a0", "#c0c0c0", "#e0e0e0", "#ffffff"}
	o.Scheme = true
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if s.Type() != "diverging-linear" {
		t.Errorf("Type = %s, want diverging-linear", s.Type())
	}
	for _, c := range []struct {
		v    float64
		want string
	}{{-6, "#000000"}, {-3, "#404040"}, {0, "#808080"}, {6, "#ffffff"}} {
		if got := s.Map(c.v); got != c.want {
			t.Errorf("Map(%v) = %v, want %s", c.v, got, c.want)
		}
	}
}

func TestTicks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		lo, hi, count float64
		want          []float64
	}{
		{0, 1, 5, []float64{0, 0.2, 0.4, 0.6, 0.8, 1}},
		{-1, 1, 4, []float64{-1, -0.5, 0, 0.5, 1}},
		{0, 1000, 3, []float64{0, 500, 1000}},
	}
	for _, test := range tests {
		if got := Ticks(test.lo, test.hi, test.count); !slices.Equal(got, test.want) {
			t.Errorf("Ticks(%v, %v, %v) = %v, want %v", test.lo, test.hi, test.count, got, test.want)
		}
	}
}

func TestTimeTicks(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	ticks := TimeTicks(t0, t1, 4)
	var got []string
	for _, x := range ticks {
		got = append(got, x.Format("2006-01-02"))
	}
	want := []string{"2024-01-01", "2024-04-01", "2024-07-01", "2024-10-01"}
	if !slices.Equal(got, want) {
		t.Errorf("TimeTicks = %v, want %v", got, want)
	}
}

func TestDiscretizing(t *testing.T) {
	t.Parallel()
	o := Defaults("quantize")
	o.Domain = []any{0.0, 100.0}
	o.Range = []any{"a", "b", "c", "d"}
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	for v, want := range map[float64]string{0: "a", 24: "a", 25: "b", 99: "d"} {
		if got := s.Map(v); got != want {
			t.Errorf("Map(%v) = %v, want %v", v, got, want)
		}
	}
	if got := Thresholds(s); !slices.Equal(got, []float64{25, 50, 75}) {
		t.Errorf("Thresholds = %v", got)
	}
}
