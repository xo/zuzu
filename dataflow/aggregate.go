package dataflow

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"

	"github.com/xo/zuzu/expr"
)

// measure computes one aggregate of the values of a field in a group. rows
// are the rows of the group, and vals the values of the field in them.
type measure func(rows []Row, vals []any) any

// randomMeasure is a measure that needs random numbers, such as the
// bootstrap of a confidence interval.
type randomMeasure func(rows []Row, vals []any, random func() float64) any

// randomMeasures are the measures that need random numbers.
var randomMeasures = map[string]randomMeasure{
	"ci0": func(_ []Row, vals []any, random func() float64) any { return bootstrap(vals, random, 0.025) },
	"ci1": func(_ []Row, vals []any, random func() float64) any { return bootstrap(vals, random, 0.975) },
}

// bootstrap returns a quantile of the means of 1000 samples of the valid
// values, drawn with replacement, as the bounds of a 95 percent confidence
// interval of the mean.
func bootstrap(vals []any, random func() float64, q float64) any {
	n := numbers(vals)
	if len(n) == 0 {
		return expr.Undefined
	}
	const samples = 1000
	means := make([]float64, samples)
	for i := range means {
		s := 0.0
		for range n {
			s += n[int(random()*float64(len(n)))]
		}
		means[i] = s / float64(len(n))
	}
	sort.Float64s(means)
	return quantile(means, q)
}

// lookupMeasure returns the measure of an op, with random numbers from env
// for those that need them.
func lookupMeasure(op string, env Env) (measure, bool) {
	if m, ok := measures[op]; ok {
		return m, true
	}
	if m, ok := randomMeasures[op]; ok {
		return func(rows []Row, vals []any) any { return m(rows, vals, env.Random) }, true
	}
	return nil, false
}

// measures are the aggregate operations, as Vega documents them.
var measures = map[string]measure{
	"count": func(rows []Row, _ []any) any { return float64(len(rows)) },
	"valid": func(_ []Row, vals []any) any {
		n := 0
		for _, v := range vals {
			if valid(v) {
				n++
			}
		}
		return float64(n)
	},
	"missing": func(_ []Row, vals []any) any {
		n := 0
		for _, v := range vals {
			if v == nil || expr.IsUndefined(v) {
				n++
			}
		}
		return float64(n)
	},
	"distinct": func(_ []Row, vals []any) any {
		seen := map[string]bool{}
		for _, v := range vals {
			seen[key([]any{v})] = true
		}
		return float64(len(seen))
	},
	"sum": func(_ []Row, vals []any) any {
		s := 0.0
		for _, v := range vals {
			if valid(v) {
				s += expr.Number(v)
			}
		}
		return s
	},
	"product": func(_ []Row, vals []any) any {
		s := 1.0
		for _, v := range vals {
			if valid(v) {
				s *= expr.Number(v)
			}
		}
		return s
	},
	"mean":      mean,
	"average":   mean,
	"variance":  func(_ []Row, vals []any) any { return variance(vals, 1) },
	"variancep": func(_ []Row, vals []any) any { return variance(vals, 0) },
	"stdev":     func(_ []Row, vals []any) any { return sqrtOf(variance(vals, 1)) },
	"stdevp":    func(_ []Row, vals []any) any { return sqrtOf(variance(vals, 0)) },
	"stderr": func(_ []Row, vals []any) any {
		v := variance(vals, 1)
		n := len(numbers(vals))
		if expr.IsUndefined(v) || n == 0 {
			return expr.Undefined
		}
		return math.Sqrt(expr.Number(v) / float64(n))
	},
	"median": func(_ []Row, vals []any) any { return quantileOf(vals, 0.5) },
	"q1":     func(_ []Row, vals []any) any { return quantileOf(vals, 0.25) },
	"q3":     func(_ []Row, vals []any) any { return quantileOf(vals, 0.75) },
	"min":    func(rows []Row, vals []any) any { return pick(rows, vals, -1, false) },
	"max":    func(rows []Row, vals []any) any { return pick(rows, vals, 1, false) },
	"argmin": func(rows []Row, vals []any) any { return pick(rows, vals, -1, true) },
	"argmax": func(rows []Row, vals []any) any { return pick(rows, vals, 1, true) },
	"values": func(rows []Row, _ []any) any {
		out := make([]any, len(rows))
		for i, r := range rows {
			out[i] = r
		}
		return out
	},
}

// numbers returns the valid values as numbers.
func numbers(vals []any) []float64 {
	var out []float64
	for _, v := range vals {
		if valid(v) {
			if f := expr.Number(v); !math.IsNaN(f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// mean returns the mean of the valid values, or undefined if none is.
func mean(_ []Row, vals []any) any {
	n := numbers(vals)
	if len(n) == 0 {
		return expr.Undefined
	}
	s := 0.0
	for _, f := range n {
		s += f
	}
	return s / float64(len(n))
}

// variance returns the variance of the valid values, of the sample when
// ddof is 1 and of the population when it is 0.
func variance(vals []any, ddof int) any {
	n := numbers(vals)
	if len(n) <= ddof {
		return expr.Undefined
	}
	m := 0.0
	for _, f := range n {
		m += f
	}
	m /= float64(len(n))
	ss := 0.0
	for _, f := range n {
		ss += (f - m) * (f - m)
	}
	return ss / float64(len(n)-ddof)
}

// sqrtOf returns the square root of a number, and undefined of undefined.
func sqrtOf(v any) any {
	if expr.IsUndefined(v) {
		return v
	}
	return math.Sqrt(expr.Number(v))
}

// quantileOf returns a quantile of the valid values, with linear
// interpolation between the two values around it.
func quantileOf(vals []any, q float64) any {
	n := numbers(vals)
	if len(n) == 0 {
		return expr.Undefined
	}
	sort.Float64s(n)
	return quantile(n, q)
}

// quantile returns a quantile of sorted numbers: the value at the position
// q times (n - 1), interpolated.
func quantile(sorted []float64, q float64) float64 {
	h := float64(len(sorted)-1) * q
	i := int(math.Floor(h))
	if i+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[i] + (h-float64(i))*(sorted[i+1]-sorted[i])
}

// pick returns the least valid value, when sign is -1, or the greatest,
// when it is 1, or the row that holds it, when arg is true.
func pick(rows []Row, vals []any, sign int, arg bool) any {
	best := -1
	for i, v := range vals {
		if !valid(v) {
			continue
		}
		if best < 0 || compareValues(v, vals[best])*sign > 0 {
			best = i
		}
	}
	if best < 0 {
		return expr.Undefined
	}
	if arg {
		return rows[best]
	}
	return vals[best]
}

// aggregation is the parsed parameters of an aggregate or a join
// aggregate.
type aggregation struct {
	groupby []*Accessor
	fields  []*Accessor
	ops     []measure
	names   []string
}

// readAggregation reads the parameters of an aggregate.
func readAggregation(p *params) (*aggregation, error) {
	a := &aggregation{}
	var err error
	if a.groupby, err = p.fields("groupby"); err != nil {
		return nil, err
	}
	if a.fields, err = p.fields("fields"); err != nil {
		return nil, err
	}
	ops, err := p.strings("ops")
	if err != nil {
		return nil, err
	}
	as, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		ops = []string{"count"}
	}
	for i, op := range ops {
		m, ok := lookupMeasure(op, p.env)
		if !ok {
			return nil, fmt.Errorf("the aggregate op %q: %w", op, ErrParam)
		}
		var f *Accessor
		if i < len(a.fields) {
			f = a.fields[i]
		}
		name := op
		if f != nil {
			name = op + "_" + f.Name
		}
		if i < len(as) && as[i] != "" {
			name = as[i]
		}
		a.ops = append(a.ops, m)
		a.names = append(a.names, name)
		if i >= len(a.fields) {
			a.fields = append(a.fields, nil)
		}
	}
	return a, nil
}

// group splits rows into groups by the values of the group by fields, in
// the order each group first appears.
func (a *aggregation) group(rows []Row) ([]string, map[string][]Row, map[string][]any) {
	var order []string
	groups := map[string][]Row{}
	keys := map[string][]any{}
	for _, r := range rows {
		vals := make([]any, len(a.groupby))
		for i, g := range a.groupby {
			vals[i] = g.Get(r)
		}
		k := key(vals)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
			keys[k] = vals
		}
		groups[k] = append(groups[k], r)
	}
	return order, groups, keys
}

// measure computes the measures of a group into a row.
func (a *aggregation) measure(out Row, rows []Row) {
	for i, m := range a.ops {
		f := a.fields[i]
		vals := make([]any, len(rows))
		if f != nil {
			for j, r := range rows {
				vals[j] = f.Get(r)
			}
		}
		out[a.names[i]] = m(rows, vals)
	}
}

// aggregate groups rows and computes a row of measures for each group.
func aggregate(p *params, rows []Row) ([]Row, error) {
	a, err := readAggregation(p)
	if err != nil {
		return nil, err
	}
	cross, err := p.boolean("cross", false)
	if err != nil {
		return nil, err
	}
	order, groups, keys := a.group(rows)
	if cross && len(a.groupby) > 1 {
		order, keys = crossKeys(a, order, keys)
	}
	if len(a.groupby) == 0 && len(order) == 0 {
		// With no group by, the aggregate of no rows is one row.
		order = []string{""}
		keys[""] = nil
	}
	out := make([]Row, 0, len(order))
	for _, k := range order {
		r := Row{}
		for i, g := range a.groupby {
			r[g.Name] = keys[k][i]
		}
		a.measure(r, groups[k])
		out = append(out, r)
	}
	return out, nil
}

// crossKeys returns every combination of the values of the group by
// fields, in the order of the values in the data.
func crossKeys(a *aggregation, order []string, keys map[string][]any) ([]string, map[string][]any) {
	distinct := make([][]any, len(a.groupby))
	for i := range a.groupby {
		seen := map[string]bool{}
		for _, k := range order {
			v := keys[k][i]
			if s := key([]any{v}); !seen[s] {
				seen[s] = true
				distinct[i] = append(distinct[i], v)
			}
		}
	}
	combos := [][]any{{}}
	for _, d := range distinct {
		var next [][]any
		for _, c := range combos {
			for _, v := range d {
				next = append(next, append(slices.Clone(c), v))
			}
		}
		combos = next
	}
	out := make([]string, 0, len(combos))
	for _, c := range combos {
		k := key(c)
		keys[k] = c
		out = append(out, k)
	}
	return out, keys
}

// joinAggregate adds the measures of the group of each row to the row.
func joinAggregate(p *params, rows []Row) ([]Row, error) {
	a, err := readAggregation(p)
	if err != nil {
		return nil, err
	}
	order, groups, _ := a.group(rows)
	m := make(map[string]Row, len(order))
	for _, k := range order {
		r := Row{}
		a.measure(r, groups[k])
		m[k] = r
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		vals := make([]any, len(a.groupby))
		for j, g := range a.groupby {
			vals[j] = g.Get(r)
		}
		c := clone(r)
		maps.Copy(c, m[key(vals)])
		out[i] = c
	}
	return out, nil
}
