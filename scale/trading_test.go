package scale

import (
	"math"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestTrading(t *testing.T) {
	t.Parallel()
	o := Defaults(Trading)
	// Friday, Monday and Tuesday: the weekend leaves no gap.
	o.Domain = []any{day("2009-06-09"), day("2009-06-05"), day("2009-06-08"), day("2009-06-08")}
	o.Range = []any{0.0, 300.0}
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s.Domain()); got != 3 {
		t.Fatalf("the domain holds %d days, want 3", got)
	}
	for i, d := range []string{"2009-06-05", "2009-06-08", "2009-06-09"} {
		if got := s.Map(day(d)); got != float64(100*i) {
			t.Errorf("Map(%s) = %v, want %v", d, got, 100*i)
		}
	}
	noon := day("2009-06-08").Add(12 * time.Hour)
	if got, _ := s.Map(noon).(float64); math.Abs(got-150) > 1e-9 {
		t.Errorf("Map(noon) = %v, want 150", got)
	}
	if got := s.Map(day("2009-06-06")); !expr.IsUndefined(got) {
		t.Errorf("a Saturday maps to %v, want nothing", got)
	}
	// With room for two ticks, they mark the weeks: the week of June 7
	// starts on its Monday, its first day of trade.
	ticks := s.Ticks(2)
	if second, _ := ticks[len(ticks)-1].(time.Time); len(ticks) != 2 || !second.Equal(day("2009-06-08")) {
		t.Errorf("ticks = %v, want June 5 and June 8", ticks)
	}
}
