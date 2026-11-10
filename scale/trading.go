package scale

import (
	"math"
	"slices"
	"time"

	"github.com/xo/zuzu/expr"
)

// Trading is the type of the scale of D30 that places the days of its
// domain at even steps, so that a day with no trade leaves no gap.
const Trading = "zuzu.trading"

// trading is a band scale over the days of its domain. A time maps inside
// the band of its day, by its share of the day.
type trading struct {
	b    *band
	days []time.Time
	loc  *time.Location
}

// dayOf returns the start of the day of a time.
func dayOf(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func newTrading(o Options) *trading {
	loc := o.Location
	if loc == nil {
		loc = time.UTC
	}
	seen := map[int64]bool{}
	var days []time.Time
	for _, v := range o.Domain {
		x := expr.Number(v)
		if math.IsNaN(x) {
			continue
		}
		d := dayOf(time.UnixMilli(int64(math.Round(x))), loc)
		if !seen[d.UnixMilli()] {
			seen[d.UnixMilli()] = true
			days = append(days, d)
		}
	}
	slices.SortFunc(days, func(a, b time.Time) int { return a.Compare(b) })
	bo := o
	bo.Type = "band"
	bo.Domain = make([]any, len(days))
	for i, d := range days {
		bo.Domain[i] = d
	}
	return &trading{b: newBand(bo), days: days, loc: loc}
}

func (s *trading) Type() string { return Trading }

func (s *trading) Map(v any) any {
	x := expr.Number(v)
	if math.IsNaN(x) {
		return expr.Undefined
	}
	t := time.UnixMilli(int64(math.Round(x))).In(s.loc)
	d := dayOf(t, s.loc)
	p, ok := s.b.Map(d).(float64)
	if !ok {
		return expr.Undefined
	}
	next := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, s.loc)
	share := float64(t.Sub(d)) / float64(next.Sub(d))
	return p + share*s.b.Bandwidth()
}

func (s *trading) Invert(v float64) any { return s.b.Invert(v) }

func (s *trading) Domain() []any { return s.b.Domain() }

func (s *trading) Range() []any { return s.b.Range() }

func (s *trading) Bandwidth() float64 { return s.b.Bandwidth() }

// tradingPeriods are the periods that the ticks mark, finest first. Each
// gives a number that changes when a new period starts.
var tradingPeriods = []func(t time.Time) int{
	func(t time.Time) int { return t.Year()*400 + t.YearDay() },
	func(t time.Time) int {
		// A week starts on Sunday, as the weeks of d3 do.
		return int(t.AddDate(0, 0, -int(t.Weekday())).Unix() / 86400)
	},
	func(t time.Time) int { return t.Year()*12 + int(t.Month()) },
	func(t time.Time) int { return t.Year()*4 + (int(t.Month())-1)/3 },
	func(t time.Time) int { return t.Year() },
}

// Ticks returns the first day in the domain of each day, week, month,
// quarter or year, at the finest of those periods that gives no more than
// count ticks.
func (s *trading) Ticks(count float64) []any {
	var out []any
	for _, period := range tradingPeriods {
		out = out[:0]
		prev := 0
		for i, d := range s.days {
			p := period(d)
			if i == 0 || p != prev {
				out = append(out, d)
			}
			prev = p
		}
		if float64(len(out)) <= math.Max(1, count) {
			break
		}
	}
	return out
}
