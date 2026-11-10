package dataflow

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/xo/zuzu/expr"
)

// transforms are the transforms that zuzu runs, by type.
var transforms = map[string]transformFunc{
	"aggregate":         aggregate,
	"bin":               bin,
	"collect":           collect,
	"density":           density,
	"kde":               kde,
	"loess":             loess,
	"quantile":          quantiles,
	"regression":        regression,
	"extent":            extent,
	"filter":            filter,
	"flatten":           flatten,
	"fold":              fold,
	"formula":           formula,
	"identifier":        identifier,
	"impute":            impute,
	"joinaggregate":     joinAggregate,
	"countpattern":      countpattern,
	"dotbin":            dotbin,
	"force":             force,
	"geojson":           geojson,
	"geopath":           geopath,
	"geopoint":          geopoint,
	"geoshape":          geoshape,
	"graticule":         graticule,
	"heatmap":           heatmap,
	"isocontour":        isocontour,
	"kde2d":             kde2d,
	"linkpath":          linkpath,
	"lookup":            lookup,
	"pack":              pack,
	"partition":         partition,
	"pie":               pie,
	"pivot":             pivot,
	"project":           project,
	"sample":            sample,
	"sequence":          sequence,
	"stack":             stack,
	"stratify":          stratify,
	"tree":              tree,
	"wordcloud":         wordcloud,
	"treemap":           treemap,
	"treelinks":         treelinks,
	"timeunit":          timeUnit,
	"window":            window,
	"zuzu.chord":        chord,
	"zuzu.chordribbons": chordribbons,
	"zuzu.hexbin":       hexbin,
	"zuzu.sankey":       sankey,
	"zuzu.sankeylinks":  sankeylinks,
}

// filter keeps the rows for which an expression is true.
func filter(p *params, rows []Row) ([]Row, error) {
	test, err := p.expr("expr")
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		v, err := test(r)
		if err != nil {
			return nil, err
		}
		if expr.Truthy(v) {
			out = append(out, r)
		}
	}
	return out, nil
}

// formula sets a field of each row to the value of an expression.
func formula(p *params, rows []Row) ([]Row, error) {
	fn, err := p.expr("expr")
	if err != nil {
		return nil, err
	}
	as, err := p.str("as", "")
	if err != nil {
		return nil, err
	}
	if as == "" {
		return nil, fmt.Errorf("formula without as: %w", ErrParam)
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		v, err := fn(r)
		if err != nil {
			return nil, err
		}
		c := clone(r)
		c[as] = v
		out[i] = c
	}
	return out, nil
}

// collect sorts the rows.
func collect(p *params, rows []Row) ([]Row, error) {
	cmp, err := p.sort()
	if err != nil || cmp == nil {
		return rows, err
	}
	out := slices.Clone(rows)
	slices.SortStableFunc(out, cmp)
	return out, nil
}

// extent sets a signal to the least and the greatest valid value of a
// field.
func extent(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("extent without a field: %w", ErrParam)
	}
	lo, hi := expr.Undefined, expr.Undefined
	for _, r := range rows {
		v := f.Get(r)
		if !valid(v) {
			continue
		}
		n := expr.Number(v)
		if math.IsNaN(n) {
			continue
		}
		if expr.IsUndefined(lo) || n < expr.Number(lo) {
			lo = n
		}
		if expr.IsUndefined(hi) || n > expr.Number(hi) {
			hi = n
		}
	}
	name, err := p.str("signal", "")
	if err != nil {
		return nil, err
	}
	if name != "" {
		p.env.SetSignal(name, []any{lo, hi})
	}
	return rows, nil
}

// identifier gives each row a number, from 1, in a field.
func identifier(p *params, rows []Row) ([]Row, error) {
	as, err := p.str("as", "")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		c[as] = float64(i + 1)
		out[i] = c
	}
	return out, nil
}

// project keeps some fields of each row, under new names.
func project(p *params, rows []Row) ([]Row, error) {
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	as, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	for i := range names {
		if i < len(as) && as[i] != "" {
			names[i] = as[i]
		}
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		o := make(Row, len(fields))
		for j, f := range fields {
			o[names[j]] = f.Get(r)
		}
		out[i] = o
	}
	return out, nil
}

// sequence makes rows of numbers from start to stop, by step.
func sequence(p *params, _ []Row) ([]Row, error) {
	start, err := p.number("start", math.NaN())
	if err != nil {
		return nil, err
	}
	stop, err := p.number("stop", math.NaN())
	if err != nil {
		return nil, err
	}
	step, err := p.number("step", 1)
	if err != nil {
		return nil, err
	}
	as, err := p.str("as", "data")
	if err != nil {
		return nil, err
	}
	n := math.Ceil((stop - start) / step)
	if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return []Row{}, nil
	}
	out := make([]Row, int(n))
	for i := range out {
		out[i] = Row{as: start + float64(i)*step}
	}
	return out, nil
}

// fold turns the fields of each row into rows of a key and a value.
func fold(p *params, rows []Row) ([]Row, error) {
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	as, err := p.as("key", "value")
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows)*len(fields))
	for _, r := range rows {
		for _, f := range fields {
			c := clone(r)
			c[as[0]] = f.Name
			c[as[1]] = f.Get(r)
			out = append(out, c)
		}
	}
	return out, nil
}

// flatten makes a row for each element of the arrays in fields. Arrays of
// one row with different lengths give as many rows as the longest, with
// null where an array ends.
func flatten(p *params, rows []Row) ([]Row, error) {
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	as, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	for i := range names {
		if i < len(as) && as[i] != "" {
			names[i] = as[i]
		}
	}
	index, err := p.str("index", "")
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, r := range rows {
		arrays := make([][]any, len(fields))
		n := 0
		for i, f := range fields {
			a, _ := f.Get(r).([]any)
			arrays[i] = a
			n = max(n, len(a))
		}
		for j := range n {
			c := clone(r)
			for i := range fields {
				var v any
				if j < len(arrays[i]) {
					v = arrays[i][j]
				}
				c[names[i]] = v
			}
			if index != "" {
				c[index] = float64(j)
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// lookup finds, for each row, the row of another dataset whose key equals
// a field, and copies that row or some of its fields.
func lookup(p *params, rows []Row) ([]Row, error) {
	from, err := p.str("from", "")
	if err != nil {
		return nil, err
	}
	other, err := p.env.Data(from)
	if err != nil {
		return nil, err
	}
	k, err := p.field("key")
	if err != nil {
		return nil, err
	}
	if k == nil {
		return nil, fmt.Errorf("lookup without a key: %w", ErrParam)
	}
	// The last row with a key wins, as in the index of Vega.
	index := map[string]Row{}
	for _, r := range other {
		index[key([]any{k.Get(r)})] = r
	}
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	values, err := p.fields("values")
	if err != nil {
		return nil, err
	}
	as, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	def, _, err := p.raw("default")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		for j, f := range fields {
			match, ok := index[key([]any{f.Get(r)})]
			if values == nil {
				name := f.Name
				if j < len(as) && as[j] != "" {
					name = as[j]
				}
				if ok {
					c[name] = match
				} else {
					c[name] = def
				}
				continue
			}
			for vi, v := range values {
				// The names of as run over the fields, and over the values
				// of each field, as in Vega.
				name := v.Name
				if n := j*len(values) + vi; n < len(as) && as[n] != "" {
					name = as[n]
				}
				if ok {
					c[name] = v.Get(match)
				} else {
					c[name] = def
				}
			}
		}
		out[i] = c
	}
	return out, nil
}

// pie sets the start and the end angles of a pie chart from a field.
func pie(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	start, err := p.number("startAngle", 0)
	if err != nil {
		return nil, err
	}
	end, err := p.number("endAngle", 2*math.Pi)
	if err != nil {
		return nil, err
	}
	sorted, err := p.boolean("sort", false)
	if err != nil {
		return nil, err
	}
	as, err := p.as("startAngle", "endAngle")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	vals := make([]float64, len(rows))
	sum := 0.0
	for i, r := range rows {
		v := 1.0
		if f != nil {
			v = expr.Number(f.Get(r))
		}
		if math.IsNaN(v) {
			v = 0
		}
		vals[i] = v
		sum += v
		out[i] = clone(r)
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	if sorted {
		slices.SortStableFunc(order, func(a, b int) int { return cmpFloat(vals[a], vals[b]) })
	}
	k := 0.0
	if sum != 0 {
		k = (end - start) / sum
	}
	a := start
	for _, i := range order {
		out[i][as[0]] = a
		a += vals[i] * k
		out[i][as[1]] = a
	}
	return out, nil
}

// sample keeps a random sample of the rows, of at most size rows.
func sample(p *params, rows []Row) ([]Row, error) {
	size, err := p.number("size", 1000)
	if err != nil {
		return nil, err
	}
	n := int(size)
	if len(rows) <= n {
		return rows, nil
	}
	out := slices.Clone(rows[:n])
	for i := n; i < len(rows); i++ {
		j := int(p.env.Random() * float64(i+1))
		if j < n {
			out[j] = rows[i]
		}
	}
	return out, nil
}

// cmpFloat compares two numbers.
func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareValues compares two values in the ascending order of Vega: null
// and undefined first, then dates and numbers by their values, then
// strings, with NaN after the numbers.
func compareValues(a, b any) int {
	an, bn := a == nil || expr.IsUndefined(a), b == nil || expr.IsUndefined(b)
	switch {
	case an && bn:
		return 0
	case an:
		return -1
	case bn:
		return 1
	}
	if t, ok := a.(time.Time); ok {
		a = float64(t.UnixMilli())
	}
	if t, ok := b.(time.Time); ok {
		b = float64(t.UnixMilli())
	}
	fa, aNum := a.(float64)
	fb, bNum := b.(float64)
	if aNum && bNum {
		switch {
		case math.IsNaN(fa) && math.IsNaN(fb):
			return 0
		case math.IsNaN(fa):
			return 1
		case math.IsNaN(fb):
			return -1
		}
		return cmpFloat(fa, fb)
	}
	return expr.Ascending(a, b)
}

// sort reads the sort parameter, {"field": ..., "order": ...}, as a function
// that compares rows. It returns nil if the transform has none.
func (p *params) sort() (func(a, b Row) int, error) {
	v, ok, err := p.raw("sort")
	if err != nil || !ok || v == nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reading the sort %v: %w", v, ErrParam)
	}
	var fields []*Accessor
	var list []any
	switch f := m["field"].(type) {
	case []any:
		list = f
	case nil:
	default:
		list = []any{f}
	}
	for _, f := range list {
		a, err := p.accessor(f)
		if err != nil {
			return nil, err
		}
		fields = append(fields, a)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	var orders []string
	switch o := m["order"].(type) {
	case []any:
		for _, x := range o {
			orders = append(orders, expr.String(x))
		}
	case string:
		orders = []string{o}
	}
	return Comparator(fields, orders), nil
}

// Comparator returns a function that compares rows by fields, each in the
// order ascending or descending. A missing order is ascending.
func Comparator(fields []*Accessor, orders []string) func(a, b Row) int {
	return func(a, b Row) int {
		for i, f := range fields {
			c := compareValues(f.Get(a), f.Get(b))
			if i < len(orders) && orders[i] == "descending" {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return 0
	}
}
