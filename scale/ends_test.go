package scale

import (
	"slices"
	"testing"
)

// TestEndTicks checks that the ticks of an axis land on both ends of a
// nice domain, with at most six of them (D46).
func TestEndTicks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		lo, hi, count float64
		want          []float64
		step          float64
	}{
		// The ticks of Ticks land on both ends.
		{0, 100, 6, []float64{0, 20, 40, 60, 80, 100}, 20},
		// A step of 10,000 misses 45,000.
		{0, 45000, 6, []float64{0, 15000, 30000, 45000}, 1500},
		// A step of 100 gives nine ticks.
		{0, 800, 6, []float64{0, 200, 400, 600, 800}, 200},
		{-1, 1, 6, []float64{-1, -0.5, 0, 0.5, 1}, 0.5},
		// A reversed domain gives the ticks in reverse.
		{800, 0, 6, []float64{800, 600, 400, 200, 0}, 200},
		// A nice domain that no step lands on from zero takes a step from
		// its start.
		{50, 500, 6, []float64{50, 200, 350, 500}, 15},
		{1970, 1982, 6, []float64{1970, 1974, 1978, 1982}, 4},
		{1500, 5500, 6, []float64{1500, 2500, 3500, 4500, 5500}, 1000},
		// The ticks of Ticks, though more than six, win over the two ends
		// alone.
		{0, 70, 6, []float64{0, 10, 20, 30, 40, 50, 60, 70}, 10},
		// A short axis takes fewer ticks.
		{0, 45000, 2.5, []float64{0, 45000}, 4500},
		// No step lands on both ends of 0 to 1.1 with six ticks or fewer,
		// and the ends alone say less than the ticks of Ticks.
		{0, 1.1, 6, nil, 0},
		// A domain that is not nice keeps the ticks of Ticks.
		{0.3, 10.3, 6, nil, 0},
	} {
		got, step := EndTicks(tc.lo, tc.hi, tc.count, 6)
		if !slices.Equal(got, tc.want) || step != tc.step {
			t.Errorf("EndTicks(%v, %v, %v) = %v, %v, want %v, %v", tc.lo, tc.hi, tc.count, got, step, tc.want, tc.step)
		}
	}
}
