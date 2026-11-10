package dataflow

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/expr"
)

// dotbin puts the values of a field into the stacks of a dot plot, by the
// method of Wilkinson: in order of value, a stack takes each value less
// than step above its first, and sits at the middle of its first and last
// values. With smooth, values move between stacks that lie close, to even
// out their heights. The rows keep their order.
func dotbin(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if field == nil {
		return nil, fmt.Errorf("dotbin needs a field: %w", ErrParam)
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	step, err := p.number("step", 0)
	if err != nil {
		return nil, err
	}
	smooth, err := p.boolean("smooth", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as("bin")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	num := func(r Row) float64 { return expr.Number(field.Get(r)) }
	if step == 0 || math.IsNaN(step) {
		lo, hi := math.NaN(), math.NaN()
		for _, r := range out {
			v := num(r)
			if math.IsNaN(v) {
				continue
			}
			if math.IsNaN(lo) || v < lo {
				lo = v
			}
			if math.IsNaN(hi) || v > hi {
				hi = v
			}
		}
		step = (hi - lo) / 30
	}
	a := &aggregation{groupby: groupby}
	order, groups, _ := a.group(out)
	for _, k := range order {
		g := slices.Clone(groups[k])
		slices.SortStableFunc(g, func(x, y Row) int {
			d := num(x) - num(y)
			switch {
			case d < 0:
				return -1
			case d > 0:
				return 1
			}
			return 0
		})
		vals := make([]float64, len(g))
		for i, r := range g {
			vals[i] = num(r)
		}
		for i, v := range dotStacks(vals, step, smooth) {
			g[i][as[0]] = v
		}
	}
	return out, nil
}

// dotStacks returns the stack of each value of a sorted list.
func dotStacks(x []float64, step float64, smooth bool) []float64 {
	n := len(x)
	v := make([]float64, n)
	if n == 0 {
		return v
	}
	i, j := 0, 1
	a := x[0]
	b := a
	w := a + step
	for ; j < n; j++ {
		if x[j] >= w {
			b = (a + b) / 2
			for ; i < j; i++ {
				v[i] = b
			}
			w = x[j] + step
			a = x[j]
		}
		b = x[j]
	}
	b = (a + b) / 2
	for ; i < j; i++ {
		v[i] = b
	}
	if smooth {
		smoothStacks(v, step+step/4)
	}
	return v
}

// smoothStacks moves values between adjacent stacks, those whose places
// lie closer than thresh, so that the two stacks have heights closer to
// each other.
func smoothStacks(v []float64, thresh float64) {
	n := len(v)
	at := func(i int) float64 {
		if i < n {
			return v[i]
		}
		return math.NaN()
	}
	a, b := 0, 1
	for b < n && at(a) == at(b) {
		b++
	}
	for b < n {
		c := b + 1
		for c < n && at(b) == at(c) {
			c++
		}
		if v[b]-v[b-1] < thresh {
			d := b + (a+c-b-b)>>1
			for d < b {
				v[d] = v[b]
				d++
			}
			for d > b {
				v[d] = v[a]
				d--
			}
		}
		a, b = b, c
	}
}

// countpattern counts the words of a field of text: the matches of a
// pattern, after a change of case, other than the stopwords. It gives a
// row for each word, with the word and its count, in the order of the
// keys of an object of JavaScript: the words that are indexes first, by
// number, then the others in the order they first appear.
func countpattern(p *params, rows []Row) ([]Row, error) {
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if field == nil {
		return nil, fmt.Errorf("countpattern needs a field: %w", ErrParam)
	}
	tcase, err := p.str("case", "mixed")
	if err != nil {
		return nil, err
	}
	pattern, err := p.str("pattern", `[\w"]+`)
	if err != nil {
		return nil, err
	}
	stopwords, err := p.str("stopwords", "")
	if err != nil {
		return nil, err
	}
	as, err := p.as("text", "count")
	if err != nil {
		return nil, err
	}
	match, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("countpattern with the pattern %q: %w: %w", pattern, ErrParam, err)
	}
	stop, err := regexp.Compile("(?i)^" + stopwords + "$")
	if err != nil {
		return nil, fmt.Errorf("countpattern with the stopwords %q: %w: %w", stopwords, ErrParam, err)
	}
	counts := map[string]int{}
	var words []string
	for _, r := range rows {
		text := expr.String(field.Get(r))
		switch tcase {
		case "upper":
			text = strings.ToUpper(text)
		case "lower":
			text = strings.ToLower(text)
		}
		for _, t := range match.FindAllString(text, -1) {
			if stop.MatchString(t) {
				continue
			}
			if _, ok := counts[t]; !ok {
				words = append(words, t)
			}
			counts[t]++
		}
	}
	slices.SortStableFunc(words, func(a, b string) int {
		ia, oka := arrayIndex(a)
		ib, okb := arrayIndex(b)
		switch {
		case oka && okb:
			return int(ia - ib)
		case oka:
			return -1
		case okb:
			return 1
		}
		return 0
	})
	out := make([]Row, 0, len(words))
	for _, w := range words {
		out = append(out, Row{as[0]: w, as[1]: float64(counts[w])})
	}
	return out, nil
}

// arrayIndex reports whether a key of an object of JavaScript is an index
// of an array, which the object lists first.
func arrayIndex(s string) (int64, bool) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == math.MaxUint32 || strconv.FormatUint(n, 10) != s {
		return 0, false
	}
	return int64(n), true
}
