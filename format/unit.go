package format

import (
	"slices"
	"strings"
)

// UnitSpecifiers are the default parts of the format spec that
// TimeUnitSpecifier joins, as Vega documents them for its function
// timeUnitSpecifier.
var UnitSpecifiers = map[string]string{
	"year":            "%Y ",
	"quarter":         "Q%q ",
	"month":           "%b ",
	"date":            "%d ",
	"week":            "W%U ",
	"day":             "%a ",
	"dayofyear":       "%j ",
	"hours":           "%H:00",
	"minutes":         "00:%M",
	"seconds":         ":%S",
	"milliseconds":    ".%L",
	"year-month":      "%Y-%m ",
	"year-month-date": "%Y-%m-%d ",
	"hours-minutes":   "%H:%M",
}

// TimeUnitSpecifier returns a format spec of d3-time-format for a set of
// time units, such as year and month. A part in specs takes the place of
// the default part of the same name in UnitSpecifiers. The parts are joined
// from the longest unit to the shortest. Year, month and date join as
// year-month-date, year and month as year-month, and hours and minutes as
// hours-minutes.
func TimeUnitSpecifier(units []string, specs map[string]string) string {
	spec := func(name string) string {
		if s, ok := specs[name]; ok {
			return s
		}
		return UnitSpecifiers[name]
	}
	has := func(u Interval) bool {
		return slices.Contains(units, string(u))
	}
	var b strings.Builder
	done := map[Interval]bool{}
	switch {
	case has(Year) && has(Month) && has(Date):
		b.WriteString(spec("year-month-date"))
		done[Year], done[Month], done[Date] = true, true, true
	case has(Year) && has(Month):
		b.WriteString(spec("year-month"))
		done[Year], done[Month] = true, true
	}
	for _, u := range Intervals {
		if done[u] || !has(u) {
			continue
		}
		if u == Hours && has(Minutes) {
			b.WriteString(spec("hours-minutes"))
			done[Minutes] = true
			continue
		}
		b.WriteString(spec(string(u)))
	}
	return strings.TrimSpace(b.String())
}
