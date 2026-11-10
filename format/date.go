package format

import (
	"strconv"
	"strings"
	"time"
)

// ParseDate parses a date the way the Date of JavaScript parses a string,
// for the date formats that data and expressions give without a format
// spec.
//
// The ISO form, such as 2024-02-29 or 2024-02-29T12:30:05.007+05:30, is
// first. A date with no time is in UTC, and a time with no offset is in
// zone. Other forms, such as 2024/02/29, 02/29/2024, Feb 29 2024 and
// February 29, 2024 1:30 PM, are in zone unless they give an offset. A year
// of two digits is in 1950 to 2049.
func ParseDate(s string, zone *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if t, ok := parseISO(s, zone); ok {
		return t, true
	}
	return parseLoose(s, zone)
}

// parseISO parses the date time string format of ECMAScript.
func parseISO(s string, zone *time.Location) (time.Time, bool) {
	p := &scan{s: s}
	sign := 1
	var year int
	switch {
	case p.peek('+') || p.peek('-'):
		if p.s[0] == '-' {
			sign = -1
		}
		p.s = p.s[1:]
		y, ok := p.digits(6)
		if !ok || (sign < 0 && y == 0) {
			return time.Time{}, false
		}
		year = sign * y
	default:
		y, ok := p.digits(4)
		if !ok {
			return time.Time{}, false
		}
		year = y
	}
	month, day := 1, 1
	if p.eat('-') {
		m, ok := p.digits(2)
		if !ok || m < 1 || m > 12 {
			return time.Time{}, false
		}
		month = m
		if p.eat('-') {
			d, ok := p.digits(2)
			if !ok || d < 1 || d > 31 {
				return time.Time{}, false
			}
			day = d
		}
	}
	if p.s == "" {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
	}
	if !p.eat('T') && !p.eat('t') {
		return time.Time{}, false
	}
	h, ok := p.digits(2)
	if !ok || !p.eat(':') {
		return time.Time{}, false
	}
	mi, ok := p.digits(2)
	if !ok {
		return time.Time{}, false
	}
	sec, ms := 0, 0
	if p.eat(':') {
		if sec, ok = p.digits(2); !ok {
			return time.Time{}, false
		}
		if p.eat('.') {
			n := 0
			for n < len(p.s) && p.s[n] >= '0' && p.s[n] <= '9' {
				n++
			}
			if n == 0 {
				return time.Time{}, false
			}
			frac := (p.s[:n] + "00")[:3]
			ms, _ = strconv.Atoi(frac)
			p.s = p.s[n:]
		}
	}
	if h > 24 || mi > 59 || sec > 59 || (h == 24 && (mi != 0 || sec != 0 || ms != 0)) {
		return time.Time{}, false
	}
	loc := zone
	offset := 0
	switch {
	case p.eat('Z') || p.eat('z'):
		loc = time.UTC
	case p.peek('+') || p.peek('-'):
		sign := 1
		if p.s[0] == '-' {
			sign = -1
		}
		p.s = p.s[1:]
		oh, ok := p.digits(2)
		if !ok || !p.eat(':') {
			return time.Time{}, false
		}
		om, ok := p.digits(2)
		if !ok {
			return time.Time{}, false
		}
		loc, offset = time.UTC, sign*(oh*60+om)
	}
	if p.s != "" {
		return time.Time{}, false
	}
	t := WallTime(year, month, day, h, mi, sec, ms, loc)
	return t.Add(-time.Duration(offset) * time.Minute).In(zone), true
}

// scan reads a string from its start.
type scan struct {
	s string
}

// peek reports whether the next byte is c.
func (p *scan) peek(c byte) bool {
	return p.s != "" && p.s[0] == c
}

// eat reads c, if it is next.
func (p *scan) eat(c byte) bool {
	if p.peek(c) {
		p.s = p.s[1:]
		return true
	}
	return false
}

// digits reads exactly n digits.
func (p *scan) digits(n int) (int, bool) {
	if len(p.s) < n {
		return 0, false
	}
	for i := range n {
		if p.s[i] < '0' || p.s[i] > '9' {
			return 0, false
		}
	}
	v, _ := strconv.Atoi(p.s[:n])
	p.s = p.s[n:]
	return v, true
}

// parseLoose parses the other forms of date that browsers read.
func parseLoose(s string, zone *time.Location) (time.Time, bool) {
	var (
		nums         []int
		widths       []int
		month        = -1
		hour, minute = -1, 0
		sec          = 0
		ms           = 0
		pm           = -1
		loc          = zone
		offset       = 0
	)
	tokens := tokenize(s)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok[0] >= '0' && tok[0] <= '9':
			// A time is digits, a colon and digits.
			if i+2 < len(tokens) && tokens[i+1] == ":" && hour < 0 {
				hour, _ = strconv.Atoi(tok)
				minute, _ = strconv.Atoi(tokens[i+2])
				i += 2
				if i+2 < len(tokens) && tokens[i+1] == ":" {
					sec, _ = strconv.Atoi(tokens[i+2])
					i += 2
					if i+2 < len(tokens) && tokens[i+1] == "." {
						frac := (tokens[i+2] + "00")[:3]
						ms, _ = strconv.Atoi(frac)
						i += 2
					}
				}
				continue
			}
			n, _ := strconv.Atoi(tok)
			nums = append(nums, n)
			widths = append(widths, len(tok))
		case tok == "+" || tok == "-":
			// An offset follows a time or GMT, as in GMT+0530 or +05:30.
			if hour < 0 || i+1 >= len(tokens) {
				continue
			}
			v := tokens[i+1]
			if v[0] < '0' || v[0] > '9' {
				return time.Time{}, false
			}
			n, _ := strconv.Atoi(v)
			i++
			if len(v) <= 2 {
				n *= 100
				if i+2 < len(tokens) && tokens[i+1] == ":" {
					m, _ := strconv.Atoi(tokens[i+2])
					n += m
					i += 2
				}
			}
			offset = n/100*60 + n%100
			if tok == "-" {
				offset = -offset
			}
			loc = time.UTC
		case tok == "/" || tok == "," || tok == "." || tok == ":":
		default:
			w := strings.ToLower(tok)
			switch {
			case w == "am" || w == "pm":
				pm = 0
				if w == "pm" {
					pm = 1
				}
			case w == "z" || w == "gmt" || w == "utc" || w == "ut":
				loc = time.UTC
			case len(w) >= 3 && monthOf(w) >= 0:
				month = monthOf(w)
			case len(w) >= 3 && isDayName(w):
			case w == "t":
			case strings.HasPrefix(w, "(") || strings.HasSuffix(w, ")"):
				// A zone name in parentheses, such as (EST), says nothing more.
			default:
				return time.Time{}, false
			}
		}
	}
	// yi is the index of the year in nums.
	var year, mon, day, yi int
	switch {
	case month >= 0 && len(nums) == 2:
		mon, day, year, yi = month+1, nums[0], nums[1], 1
		if widths[0] > 2 || nums[0] > 31 {
			year, day, yi = nums[0], nums[1], 0
		}
	case month >= 0 && len(nums) == 1:
		mon, day, year = month+1, 1, nums[0]
	case month < 0 && len(nums) == 3:
		mon, day, year, yi = nums[0], nums[1], nums[2], 2
		if widths[0] > 2 || nums[0] > 31 {
			year, mon, day, yi = nums[0], nums[1], nums[2], 0
		}
	default:
		return time.Time{}, false
	}
	if mon < 1 || mon > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	if widths[yi] <= 2 {
		if year < 50 {
			year += 2000
		} else {
			year += 1900
		}
	}
	if hour < 0 {
		hour = 0
	}
	if pm >= 0 {
		if hour > 12 {
			return time.Time{}, false
		}
		hour = hour%12 + pm*12
	}
	if hour > 24 || minute > 59 || sec > 59 {
		return time.Time{}, false
	}
	t := WallTime(year, mon, day, hour, minute, sec, ms, loc)
	return t.Add(-time.Duration(offset) * time.Minute).In(zone), true
}

// tokenize splits s into runs of digits, runs of letters and single
// punctuation marks. Spaces separate tokens.
func tokenize(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		j := i + 1
		switch {
		case c == ' ' || c == '\t':
			i = j
			continue
		case c >= '0' && c <= '9':
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		case c == '(':
			for j < len(s) && s[j] != ')' {
				j++
			}
			j = min(j+1, len(s))
		case isLetter(c):
			for j < len(s) && isLetter(s[j]) {
				j++
			}
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

// isLetter reports whether c is an ASCII letter.
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// monthOf returns the month, from 0, that a word names, by its first three
// letters, or -1.
func monthOf(w string) int {
	for i, m := range EnUS.Months {
		lm := strings.ToLower(m)
		if strings.HasPrefix(lm, w[:3]) && strings.HasPrefix(lm, w) {
			return i
		}
		if w == lm[:3] {
			return i
		}
	}
	return -1
}

// isDayName reports whether a word names a day of the week.
func isDayName(w string) bool {
	for _, d := range EnUS.Days {
		ld := strings.ToLower(d)
		if strings.HasPrefix(ld, w) {
			return true
		}
	}
	return false
}

// WallTime returns the time that a clock in loc shows as the given fields,
// as the Date of JavaScript builds it. A field out of its range carries
// into the next. A time that the clock skips, when it moves forward, reads
// with the offset from before the move, so 2:30 on a day that skips from
// 2:00 to 3:00 is 3:30. A time that the clock shows twice, when it moves
// back, is the first of the two.
func WallTime(year, month, day, hour, minute, sec, ms int, loc *time.Location) time.Time {
	wall := time.Date(year, time.Month(month), day, hour, minute, sec, ms*int(time.Millisecond), time.UTC)
	if loc == time.UTC {
		return wall
	}
	// The offsets a day before and a day after are the two that can apply.
	_, before := wall.Add(-24 * time.Hour).In(loc).Zone()
	_, after := wall.Add(24 * time.Hour).In(loc).Zone()
	var best time.Time
	for _, off := range []int{before, after} {
		t := wall.Add(-time.Duration(off) * time.Second).In(loc)
		_, o := t.Zone()
		if o == off && (best.IsZero() || t.Before(best)) {
			best = t
		}
	}
	if best.IsZero() {
		best = wall.Add(-time.Duration(before) * time.Second).In(loc)
	}
	return best
}
