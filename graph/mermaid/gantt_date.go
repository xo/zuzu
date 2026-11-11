package mermaid

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xo/zuzu/format"
)

// The dates of a gantt follow the format tokens of Day.js, which the
// statement dateFormat names, as the docs of Mermaid say. A date reads in a
// strict way: the date, written again in the format, must give the same
// text. A start date that does not read that way reads as the Date of
// JavaScript reads it.

// ganttToken is a part of a format of dates: a token, such as YYYY, or a
// literal text.
type ganttToken struct {
	tok string
	lit string
}

// ganttTokens are the tokens of a format, longest first.
var ganttTokens = strings.Fields("YYYY YY Q MMMM MMM MM M Do DDDD DDD DD D dddd ddd dd d " +
	"HH H hh h mm m ss s SSS SS S A a X x ZZ Z")

// ganttFormat splits a format of dates into its tokens. Text in brackets
// and every other character are literal.
func ganttFormat(layout string) []ganttToken {
	var out []ganttToken
	lit := func(s string) {
		if n := len(out); n > 0 && out[n-1].tok == "" {
			out[n-1].lit += s
			return
		}
		out = append(out, ganttToken{lit: s})
	}
	for i := 0; i < len(layout); {
		if layout[i] == '[' {
			if j := strings.IndexByte(layout[i:], ']'); j > 0 {
				lit(layout[i+1 : i+j])
				i += j + 1
				continue
			}
		}
		found := false
		for _, t := range ganttTokens {
			if strings.HasPrefix(layout[i:], t) {
				out = append(out, ganttToken{tok: t})
				i += len(t)
				found = true
				break
			}
		}
		if !found {
			lit(layout[i : i+1])
			i++
		}
	}
	return out
}

// The names of the months and the days, in US English.
var (
	ganttMonths = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	ganttDays   = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// ganttFields are the fields that a date gives, with -1 for a field that it
// does not give.
type ganttFields struct {
	year, quarter, month, day, yday     int
	hour, minute, second, milli         int
	pm                                  int
	unix                                int64
	unixKind                            byte
	offset                              int
	hasOffset, hasYear, hasMonth, hasPM bool
}

// digitsRe match the numbers of the tokens, by their width.
var (
	ganttRe1to2    = regexp.MustCompile(`^\d\d?`)
	ganttRe1to3    = regexp.MustCompile(`^\d{1,3}`)
	ganttReSigned  = regexp.MustCompile(`^[+-]?\d+`)
	ganttReOffset  = regexp.MustCompile(`^(?:[+-]\d\d:?(?:\d\d)?|Z)`)
	ganttReOrdinal = regexp.MustCompile(`^(\d\d?)(st|nd|rd|th)`)
	ganttReWord    = regexp.MustCompile(`^[A-Za-z]+`)
)

// ganttDigits reads exactly n digits from the start of s.
func ganttDigits(s string, n int) (int, bool) {
	if len(s) < n {
		return 0, false
	}
	for i := range n {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	v, _ := strconv.Atoi(s[:n])
	return v, true
}

// ganttParse reads a date in a format of Day.js, in the time zone loc.
// The fields that the format does not give come from today: the year, the
// month and the day, when no larger field is given.
func ganttParse(s string, toks []ganttToken, today time.Time, loc *time.Location) (time.Time, bool) {
	f := ganttFields{year: -1, quarter: -1, month: -1, day: -1, yday: -1, pm: -1}
	in := s
	for _, t := range toks {
		if t.tok == "" {
			if !strings.HasPrefix(in, t.lit) {
				return time.Time{}, false
			}
			in = in[len(t.lit):]
			continue
		}
		n, ok := ganttReadToken(in, t.tok, &f)
		if !ok {
			return time.Time{}, false
		}
		in = in[n:]
	}
	if in != "" {
		return time.Time{}, false
	}
	tm := f.time(today, loc)
	// A strict read writes the date again and asks for the same text.
	if f.unixKind == 0 {
		at := tm
		if f.hasOffset {
			at = tm.In(time.FixedZone("", f.offset))
		}
		if ganttWrite(at, toks, f.offset) != s {
			return time.Time{}, false
		}
	}
	return tm, true
}

// ganttReadToken reads one token from the start of s into f, and returns the
// length it read.
func ganttReadToken(s, tok string, f *ganttFields) (int, bool) {
	num := func(n int, set *int) (int, bool) {
		v, ok := ganttDigits(s, n)
		if !ok {
			return 0, false
		}
		*set = v
		return n, true
	}
	re := func(r *regexp.Regexp, set *int) (int, bool) {
		m := r.FindString(s)
		if m == "" {
			return 0, false
		}
		*set, _ = strconv.Atoi(m)
		return len(m), true
	}
	switch tok {
	case "YYYY":
		f.hasYear = true
		return num(4, &f.year)
	case "YY":
		f.hasYear = true
		n, ok := num(2, &f.year)
		if f.year > 68 {
			f.year += 1900
		} else {
			f.year += 2000
		}
		return n, ok
	case "Q":
		return num(1, &f.quarter)
	case "MMMM", "MMM":
		w := ganttReWord.FindString(s)
		for i, name := range ganttMonths {
			if len(tok) == 3 {
				name = name[:3]
			}
			if strings.EqualFold(w, name) {
				f.month, f.hasMonth = i+1, true
				return len(w), true
			}
		}
		return 0, false
	case "MM":
		f.hasMonth = true
		return num(2, &f.month)
	case "M":
		f.hasMonth = true
		return re(ganttRe1to2, &f.month)
	case "Do":
		m := ganttReOrdinal.FindStringSubmatch(s)
		if m == nil {
			return 0, false
		}
		f.day, _ = strconv.Atoi(m[1])
		return len(m[0]), true
	case "DDDD":
		return num(3, &f.yday)
	case "DDD":
		return re(ganttRe1to3, &f.yday)
	case "DD":
		return num(2, &f.day)
	case "D":
		return re(ganttRe1to2, &f.day)
	case "dddd", "ddd", "dd":
		w := ganttReWord.FindString(s)
		if w == "" {
			return 0, false
		}
		return len(w), true
	case "d":
		var wd int
		return num(1, &wd)
	case "HH", "hh":
		return num(2, &f.hour)
	case "H", "h":
		return re(ganttRe1to2, &f.hour)
	case "mm":
		return num(2, &f.minute)
	case "m":
		return re(ganttRe1to2, &f.minute)
	case "ss":
		return num(2, &f.second)
	case "s":
		return re(ganttRe1to2, &f.second)
	case "SSS":
		return num(3, &f.milli)
	case "SS":
		n, ok := num(2, &f.milli)
		f.milli *= 10
		return n, ok
	case "S":
		n, ok := num(1, &f.milli)
		f.milli *= 100
		return n, ok
	case "A", "a":
		switch {
		case len(s) >= 2 && strings.EqualFold(s[:2], "am"):
			f.pm, f.hasPM = 0, true
		case len(s) >= 2 && strings.EqualFold(s[:2], "pm"):
			f.pm, f.hasPM = 1, true
		default:
			return 0, false
		}
		return 2, true
	case "X", "x":
		m := ganttReSigned.FindString(s)
		if m == "" {
			return 0, false
		}
		f.unix, _ = strconv.ParseInt(m, 10, 64)
		f.unixKind = tok[0]
		return len(m), true
	case "ZZ", "Z":
		m := ganttReOffset.FindString(s)
		if m == "" {
			return 0, false
		}
		f.hasOffset = true
		if m == "Z" {
			return 1, true
		}
		h, _ := strconv.Atoi(m[1:3])
		mi := 0
		if rest := strings.TrimPrefix(m[3:], ":"); rest != "" {
			mi, _ = strconv.Atoi(rest)
		}
		f.offset = (h*60 + mi) * 60
		if m[0] == '-' {
			f.offset = -f.offset
		}
		return len(m), true
	}
	return 0, false
}

// time makes the time of the fields.
func (f *ganttFields) time(today time.Time, loc *time.Location) time.Time {
	switch f.unixKind {
	case 'X':
		return time.Unix(f.unix, 0).In(loc)
	case 'x':
		return time.UnixMilli(f.unix).In(loc)
	}
	year, month, day := f.year, f.month, f.day
	if year < 0 {
		year = today.Year()
	}
	if month < 0 && f.quarter > 0 {
		month = (f.quarter-1)*3 + 1
		f.hasMonth = true
	}
	if month < 0 {
		month = int(today.Month())
		if f.hasYear {
			month = 1
		}
	}
	if day < 0 {
		day = today.Day()
		if f.hasYear || f.hasMonth {
			day = 1
		}
	}
	if f.yday > 0 {
		month, day = 1, f.yday
	}
	hour := f.hour
	if f.hasPM {
		hour %= 12
		if f.pm == 1 {
			hour += 12
		}
	}
	zone := loc
	if f.hasOffset {
		zone = time.FixedZone("", f.offset)
	}
	t := time.Date(year, time.Month(month), day, hour, f.minute, f.second, f.milli*int(time.Millisecond), zone)
	return t.In(loc)
}

// ganttPad writes a number with zeros up to a width.
func ganttPad(v, n int) string {
	s := strconv.Itoa(v)
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// ganttWrite writes a time in a format of Day.js. offset is the offset of
// the zone in seconds, for the tokens Z and ZZ.
func ganttWrite(t time.Time, toks []ganttToken, offset int) string {
	var b strings.Builder
	for _, k := range toks {
		if k.tok == "" {
			b.WriteString(k.lit)
			continue
		}
		b.WriteString(ganttWriteToken(t, k.tok, offset))
	}
	return b.String()
}

// ganttWriteToken writes one token of a time.
func ganttWriteToken(t time.Time, tok string, offset int) string {
	h12 := t.Hour() % 12
	if h12 == 0 {
		h12 = 12
	}
	switch tok {
	case "YYYY":
		return ganttPad(t.Year(), 4)
	case "YY":
		return ganttPad(t.Year()%100, 2)
	case "Q":
		return strconv.Itoa((int(t.Month())-1)/3 + 1)
	case "MMMM":
		return ganttMonths[t.Month()-1]
	case "MMM":
		return ganttMonths[t.Month()-1][:3]
	case "MM":
		return ganttPad(int(t.Month()), 2)
	case "M":
		return strconv.Itoa(int(t.Month()))
	case "Do":
		return strconv.Itoa(t.Day()) + ganttOrdinal(t.Day())
	case "DDDD":
		return ganttPad(t.YearDay(), 3)
	case "DDD":
		return strconv.Itoa(t.YearDay())
	case "DD":
		return ganttPad(t.Day(), 2)
	case "D":
		return strconv.Itoa(t.Day())
	case "dddd":
		return ganttDays[t.Weekday()]
	case "ddd":
		return ganttDays[t.Weekday()][:3]
	case "dd":
		return ganttDays[t.Weekday()][:2]
	case "d":
		return strconv.Itoa(int(t.Weekday()))
	case "HH":
		return ganttPad(t.Hour(), 2)
	case "H":
		return strconv.Itoa(t.Hour())
	case "hh":
		return ganttPad(h12, 2)
	case "h":
		return strconv.Itoa(h12)
	case "mm":
		return ganttPad(t.Minute(), 2)
	case "m":
		return strconv.Itoa(t.Minute())
	case "ss":
		return ganttPad(t.Second(), 2)
	case "s":
		return strconv.Itoa(t.Second())
	case "SSS":
		return ganttPad(t.Nanosecond()/1e6, 3)
	case "SS":
		return ganttPad(t.Nanosecond()/1e7, 2)
	case "S":
		return strconv.Itoa(t.Nanosecond() / 1e8)
	case "A":
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case "a":
		if t.Hour() < 12 {
			return "am"
		}
		return "pm"
	case "X":
		return strconv.FormatInt(t.Unix(), 10)
	case "x":
		return strconv.FormatInt(t.UnixMilli(), 10)
	case "ZZ", "Z":
		sign := "+"
		if offset < 0 {
			sign, offset = "-", -offset
		}
		sep := ":"
		if tok == "ZZ" {
			sep = ""
		}
		return sign + ganttPad(offset/3600, 2) + sep + ganttPad(offset/60%60, 2)
	}
	return ""
}

// ganttOrdinal returns the English suffix of an ordinal number.
func ganttOrdinal(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}

// ganttLoose reads a date as the Date of JavaScript reads a string.
func ganttLoose(s string, loc *time.Location) (time.Time, bool) {
	t, ok := format.ParseDate(s, loc)
	if !ok {
		return time.Time{}, false
	}
	return t.In(loc), true
}

// ganttDuration is a length of a task, such as 3d: a number and a unit.
type ganttDuration struct {
	n    float64
	unit string
}

// ganttDurationRe matches a duration: a number, with a fraction or not, and a
// unit of ms, s, m, h, d, w, M or y.
var ganttDurationRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ms|s|m|h|d|w|M|y)$`)

// ganttParseDuration reads a duration.
func ganttParseDuration(s string) (ganttDuration, bool) {
	m := ganttDurationRe.FindStringSubmatch(s)
	if m == nil {
		return ganttDuration{}, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return ganttDuration{}, false
	}
	return ganttDuration{n: n, unit: m[2]}, true
}

// add adds a duration to a time, as Mermaid does. A length in hours or
// less is exact, to the millisecond. Days and weeks round to whole days on
// the calendar. Months and years drop their fraction, and keep the day in
// the month, or the last day of a shorter month.
func (d ganttDuration) add(t time.Time) time.Time {
	switch d.unit {
	case "ms", "s", "m", "h":
		unit := map[string]float64{"ms": 1, "s": 1e3, "m": 60e3, "h": 3600e3}[d.unit]
		return t.Add(time.Duration(math.Trunc(d.n*unit)) * time.Millisecond)
	case "d":
		return t.AddDate(0, 0, int(math.Floor(d.n+0.5)))
	case "w":
		return t.AddDate(0, 0, int(math.Floor(d.n*7+0.5)))
	case "M":
		return ganttAddMonths(t, int(d.n))
	case "y":
		return ganttAddMonths(t, 12*int(d.n))
	}
	return t
}

// ganttAddMonths adds months to a time, and keeps the day in the month, or the
// last day of a shorter month.
func ganttAddMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d, last)-1)
}
