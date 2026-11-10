package format

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Time returns a function that formats a time with a format spec of
// d3-time-format, such as "%b %d, %Y", in a locale. The function writes the
// time in its own location: pass t.UTC() for the UTC formats, and t.In(zone)
// for the local ones.
//
// A directive is a % and a letter. A -, _ or 0 between them pads the number
// with nothing, spaces or zeros. A letter that is not a directive is written
// as it is, without the %.
func Time(spec string, loc Locale) func(time.Time) string {
	return func(t time.Time) string {
		var b strings.Builder
		writeTime(&b, spec, loc, t)
		return b.String()
	}
}

// writeTime writes t with a spec.
func writeTime(b *strings.Builder, spec string, loc Locale, t time.Time) {
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		if c != '%' || i+1 >= len(spec) {
			b.WriteByte(c)
			continue
		}
		i++
		pad := byte(0)
		if p := spec[i]; (p == '-' || p == '_' || p == '0') && i+1 < len(spec) {
			pad = p
			i++
		}
		r, n := utf8.DecodeRuneInString(spec[i:])
		i += n - 1
		writeDirective(b, r, pad, loc, t)
	}
}

// writeDirective writes one directive of a spec.
func writeDirective(b *strings.Builder, r rune, pad byte, loc Locale, t time.Time) {
	num := func(n, width int, def byte) {
		if pad == 0 {
			pad = def
		}
		s := strconv.Itoa(n)
		if pad != '-' && len(s) < width {
			fill := "0"
			if pad == '_' {
				fill = " "
			}
			s = strings.Repeat(fill, width-len(s)) + s
		}
		b.WriteString(s)
	}
	wday, yday := int(t.Weekday()), t.YearDay()-1
	switch r {
	case 'a':
		b.WriteString(loc.ShortDays[wday])
	case 'A':
		b.WriteString(loc.Days[wday])
	case 'b':
		b.WriteString(loc.ShortMonths[t.Month()-1])
	case 'B':
		b.WriteString(loc.Months[t.Month()-1])
	case 'c':
		writeTime(b, loc.DateTime, loc, t)
	case 'd':
		num(t.Day(), 2, '0')
	case 'e':
		num(t.Day(), 2, '_')
	case 'f':
		num(t.Nanosecond()/int(time.Millisecond)*1000, 6, '0')
	case 'g':
		y, _ := t.ISOWeek()
		num(y%100, 2, '0')
	case 'G':
		y, _ := t.ISOWeek()
		num(y%10000, 4, '0')
	case 'H':
		num(t.Hour(), 2, '0')
	case 'I':
		num((t.Hour()+11)%12+1, 2, '0')
	case 'j':
		num(yday+1, 3, '0')
	case 'L':
		num(t.Nanosecond()/int(time.Millisecond), 3, '0')
	case 'm':
		num(int(t.Month()), 2, '0')
	case 'M':
		num(t.Minute(), 2, '0')
	case 'p':
		b.WriteString(loc.Periods[t.Hour()/12])
	case 'q':
		b.WriteString(strconv.Itoa((int(t.Month())-1)/3 + 1))
	case 'Q':
		b.WriteString(strconv.FormatInt(t.UnixMilli(), 10))
	case 's':
		b.WriteString(strconv.FormatInt(int64(math.Floor(float64(t.UnixMilli())/1000)), 10))
	case 'S':
		num(t.Second(), 2, '0')
	case 'u':
		b.WriteString(strconv.Itoa((wday+6)%7 + 1))
	case 'U':
		// Week 1 starts on the first Sunday of the year. The days before it
		// are in week 0.
		num((yday+7-wday)/7, 2, '0')
	case 'V':
		_, w := t.ISOWeek()
		num(w, 2, '0')
	case 'w':
		b.WriteString(strconv.Itoa(wday))
	case 'W':
		// Week 1 starts on the first Monday of the year.
		num((yday+7-(wday+6)%7)/7, 2, '0')
	case 'x':
		writeTime(b, loc.Date, loc, t)
	case 'X':
		writeTime(b, loc.Time, loc, t)
	case 'y':
		num(t.Year()%100, 2, '0')
	case 'Y':
		num(t.Year()%10000, 4, '0')
	case 'Z':
		_, off := t.Zone()
		sign := byte('+')
		if off < 0 {
			sign, off = '-', -off
		}
		b.WriteByte(sign)
		fmt2(b, off/3600)
		fmt2(b, off%3600/60)
	default:
		b.WriteRune(r)
	}
}

// fmt2 writes a number from 0 to 99 with two digits.
func fmt2(b *strings.Builder, n int) {
	b.WriteByte(byte('0' + n/10%10))
	b.WriteByte(byte('0' + n%10))
}

// fields are the parts of a time that a parse reads. A field that the text
// does not give is unset.
type fields struct {
	year, month, day, hour, minute, second, milli int
	pm                                            int // -1 when unset
	zone                                          *int
	yday, wday                                    int // -1 when unset
	weekU, weekW, weekV                           int // -1 when unset
	unixMilli                                     *int64
}

// Parse returns a function that parses a time with a format spec of
// d3-time-format. The function reads the time in zone, unless the text
// gives an offset with %Z. It reports false when the text does not match
// the spec, or has text after it. A field that the text does not give is
// the start of its range: January 1, 1900, at midnight. A field out of its
// range carries into the next, so the month 13 of a year is January of the
// next year.
func Parse(spec string, loc Locale, zone *time.Location) func(string) (time.Time, bool) {
	return func(s string) (time.Time, bool) {
		f := fields{year: 1900, month: 1, day: 1, pm: -1, yday: -1, wday: -1, weekU: -1, weekW: -1, weekV: -1}
		n, ok := parseSpec(&f, spec, s, loc)
		if !ok || n != len(s) {
			return time.Time{}, false
		}
		return f.time(zone), true
	}
}

// time builds the time from the fields that a parse read.
func (f *fields) time(zone *time.Location) time.Time {
	if f.unixMilli != nil {
		return time.UnixMilli(*f.unixMilli).In(zone)
	}
	if f.pm >= 0 {
		f.hour = f.hour%12 + f.pm*12
	}
	loc := zone
	if f.zone != nil {
		loc = time.UTC
	}
	month, day := f.month, f.day
	switch {
	case f.yday >= 0:
		month, day = 1, f.yday
	case f.weekU >= 0 || f.weekW >= 0 || f.weekV >= 0:
		month, day = 1, f.weekDay(loc)
	}
	year := f.year
	if 0 <= year && year < 100 {
		// d3 builds a date of a year from 0 to 99 in the year -1, which is
		// not a leap year, and then sets the year. The day of the month
		// overflows there, so 29 February becomes 1 March.
		probe := time.Date(-1, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		month, day = int(probe.Month()), probe.Day()
		year += probe.Year() + 1
	}
	t := WallTime(year, month, day, f.hour, f.minute, f.second, f.milli, loc)
	if f.zone != nil {
		t = t.Add(-time.Duration(*f.zone) * time.Minute).In(zone)
	}
	return t
}

// weekDay returns the day of January, which can be past its end, of a day
// that a week number and a weekday give.
func (f *fields) weekDay(loc *time.Location) int {
	jan1 := int(time.Date(f.year, 1, 1, 0, 0, 0, 0, loc).Weekday())
	w := f.wday
	switch {
	case f.weekV >= 0:
		// ISO week 1 holds January 4. Its Monday is the start of the count.
		if w < 0 {
			w = 0
		}
		jan4 := (jan1 + 3) % 7
		monday := 4 - (jan4+6)%7
		return monday + (f.weekV-1)*7 + (w+6)%7
	case f.weekW >= 0:
		// Week 1 starts on the first Monday.
		if w < 0 {
			w = 1
		}
		monday := 1 + (8-jan1)%7
		return monday + (f.weekW-1)*7 + (w+6)%7
	}
	// Week 1 starts on the first Sunday.
	if w < 0 {
		w = 0
	}
	sunday := 1 + (7-jan1)%7
	return sunday + (f.weekU-1)*7 + w
}

// parseSpec reads s with a spec and returns how much of s it read.
func parseSpec(f *fields, spec, s string, loc Locale) (int, bool) {
	i := 0
	for j := 0; j < len(spec); j++ {
		c := spec[j]
		if c != '%' || j+1 >= len(spec) {
			if i >= len(s) || s[i] != c {
				return 0, false
			}
			i++
			continue
		}
		j++
		if p := spec[j]; (p == '-' || p == '_' || p == '0') && j+1 < len(spec) {
			j++
		}
		n, ok := parseDirective(f, spec[j], s[i:], loc)
		if !ok {
			return 0, false
		}
		i += n
	}
	return i, true
}

// parseDirective reads one directive at the start of s and returns how much
// of s it read.
func parseDirective(f *fields, d byte, s string, loc Locale) (int, bool) {
	set := func(p *int, width int) (int, bool) {
		v, n, ok := parseInt(s, width)
		if ok {
			*p = v
		}
		return n, ok
	}
	switch d {
	case 'a':
		return parseName(s, loc.ShortDays[:], &f.wday)
	case 'A':
		return parseName(s, loc.Days[:], &f.wday)
	case 'b':
		return parseName(s, loc.ShortMonths[:], &f.month, 1)
	case 'B':
		return parseName(s, loc.Months[:], &f.month, 1)
	case 'c':
		return parseSpec(f, loc.DateTime, s, loc)
	case 'd', 'e':
		return set(&f.day, 2)
	case 'f':
		var micro int
		n, ok := set(&micro, 6)
		f.milli = micro / 1000
		return n, ok
	case 'g', 'y':
		n, ok := set(&f.year, 2)
		if ok {
			// A year of two digits is in 1969 to 2068.
			if f.year > 68 {
				f.year += 1900
			} else {
				f.year += 2000
			}
		}
		return n, ok
	case 'G', 'Y':
		return set(&f.year, 4)
	case 'H', 'I':
		return set(&f.hour, 2)
	case 'j':
		return set(&f.yday, 3)
	case 'L':
		return set(&f.milli, 3)
	case 'm':
		return set(&f.month, 2)
	case 'M':
		return set(&f.minute, 2)
	case 'p':
		return parseName(s, loc.Periods[:], &f.pm)
	case 'q':
		var q int
		n, ok := set(&q, 1)
		f.month = q*3 - 2
		return n, ok
	case 'Q', 's':
		v, n, ok := parseInt(s, len(s))
		if ok {
			ms := int64(v)
			if d == 's' {
				ms *= 1000
			}
			f.unixMilli = &ms
		}
		return n, ok
	case 'S':
		return set(&f.second, 2)
	case 'u':
		n, ok := set(&f.wday, 1)
		f.wday %= 7
		return n, ok
	case 'U':
		return set(&f.weekU, 2)
	case 'V':
		return set(&f.weekV, 2)
	case 'w':
		return set(&f.wday, 1)
	case 'W':
		return set(&f.weekW, 2)
	case 'x':
		return parseSpec(f, loc.Date, s, loc)
	case 'X':
		return parseSpec(f, loc.Time, s, loc)
	case 'Z':
		return parseZone(f, s)
	case '%':
		if strings.HasPrefix(s, "%") {
			return 1, true
		}
		return 0, false
	}
	return 0, false
}

// parseInt reads an integer at the start of s, after any spaces, within
// the first width bytes of s.
func parseInt(s string, width int) (int, int, bool) {
	s = s[:min(width, len(s))]
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i {
		return 0, 0, false
	}
	v, err := strconv.Atoi(s[i:j])
	return v, j, err == nil
}

// parseName reads the longest name of names at the start of s, in any case,
// and sets *p to its index plus add.
func parseName(s string, names []string, p *int, add ...int) (int, bool) {
	best, n := -1, 0
	for i, name := range names {
		if len(name) > n && len(s) >= len(name) && strings.EqualFold(s[:len(name)], name) {
			best, n = i, len(name)
		}
	}
	if best < 0 {
		return 0, false
	}
	*p = best
	for _, a := range add {
		*p += a
	}
	return n, true
}

// parseZone reads Z or an offset such as +0530 or -05:00.
func parseZone(f *fields, s string) (int, bool) {
	if strings.HasPrefix(s, "Z") {
		z := 0
		f.zone = &z
		return 1, true
	}
	if len(s) < 3 || (s[0] != '+' && s[0] != '-') {
		return 0, false
	}
	h, err := strconv.Atoi(s[1:3])
	if err != nil {
		return 0, false
	}
	n, m := 3, 0
	rest := strings.TrimPrefix(s[3:], ":")
	if len(rest) >= 2 {
		if v, err := strconv.Atoi(rest[:2]); err == nil {
			m = v
			n = len(s) - len(rest) + 2
		}
	}
	z := h*60 + m
	if s[0] == '-' {
		z = -z
	}
	f.zone = &z
	return n, true
}
