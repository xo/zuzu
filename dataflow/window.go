package dataflow

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/zuzu/expr"
)

// windowOp computes the value of an operation of a window for the row at
// i of a sorted group. peers gives the first and the last index of the
// peers of each row, the rows that sort equal to it, and frame the rows of
// the frame of the row.
type windowOp func(w *windowCtx, i int) any

// windowCtx is a sorted group and what the operations of a window read.
type windowCtx struct {
	rows       []Row
	first      []int // the index of the first peer of each row
	last       []int // the index of the last peer of each row
	field      *Accessor
	param      float64
	hasParam   bool
	frame      [2]*int
	ignorePeer bool
}

// window computes, for each row, values over the rows of its group in
// order: ranks, values of other rows, and aggregates over a frame of rows
// around it.
func window(p *params, rows []Row) ([]Row, error) {
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	cmp, err := p.sort()
	if err != nil {
		return nil, err
	}
	ops, err := p.strings("ops")
	if err != nil {
		return nil, err
	}
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	params, err := p.list("params")
	if err != nil {
		return nil, err
	}
	as, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	frame, err := readFrame(p)
	if err != nil {
		return nil, err
	}
	ignorePeers, err := p.boolean("ignorePeers", false)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = clone(r)
	}
	a := &aggregation{groupby: groupby}
	order, groups, _ := a.group(out)
	for _, k := range order {
		g := groups[k]
		if cmp != nil {
			slices.SortStableFunc(g, cmp)
		}
		w := &windowCtx{rows: g, frame: frame, ignorePeer: ignorePeers}
		w.peers(cmp)
		for j, op := range ops {
			w.field = nil
			if j < len(fields) {
				w.field = fields[j]
			}
			w.hasParam = j < len(params) && params[j] != nil
			if w.hasParam {
				w.param = expr.Number(params[j])
			}
			name := op
			if w.field != nil {
				name = op + "_" + w.field.Name
			}
			if j < len(as) && as[j] != "" {
				name = as[j]
			}
			fn, err := windowFunc(op)
			if err != nil {
				return nil, err
			}
			for i, r := range g {
				r[name] = fn(w, i)
			}
		}
	}
	return out, nil
}

// readFrame reads the frame of a window: the number of rows before and
// after each row, with null for no bound. The default is [null, 0].
func readFrame(p *params) ([2]*int, error) {
	zero := 0
	l, err := p.list("frame")
	if err != nil {
		return [2]*int{}, err
	}
	if l == nil {
		return [2]*int{nil, &zero}, nil
	}
	var f [2]*int
	for i := 0; i < 2 && i < len(l); i++ {
		if l[i] != nil {
			n := int(expr.Number(l[i]))
			f[i] = &n
		}
	}
	return f, nil
}

// peers finds the first and the last peer of each row.
func (w *windowCtx) peers(cmp func(a, b Row) int) {
	n := len(w.rows)
	w.first, w.last = make([]int, n), make([]int, n)
	for i := 0; i < n; {
		j := i
		for j+1 < n && cmp != nil && cmp(w.rows[i], w.rows[j+1]) == 0 {
			j++
		}
		if cmp == nil {
			j = i
		}
		for k := i; k <= j; k++ {
			w.first[k], w.last[k] = i, j
		}
		i = j + 1
	}
}

// bounds returns the rows of the frame of the row at i, from lo to hi
// inclusive. Unless the window ignores peers, the frame grows to hold the
// peers of its ends.
func (w *windowCtx) bounds(i int) (int, int) {
	lo, hi := 0, len(w.rows)-1
	if w.frame[0] != nil {
		lo = max(0, i+*w.frame[0])
	}
	if w.frame[1] != nil {
		hi = min(len(w.rows)-1, i+*w.frame[1])
	}
	if !w.ignorePeer && lo <= hi {
		lo, hi = w.first[lo], w.last[hi]
	}
	return lo, hi
}

// windowFunc returns the function of an operation of a window.
func windowFunc(op string) (windowOp, error) {
	n := func(w *windowCtx) float64 { return float64(len(w.rows)) }
	switch op {
	case "row_number":
		return func(_ *windowCtx, i int) any { return float64(i + 1) }, nil
	case "rank":
		return func(w *windowCtx, i int) any { return float64(w.first[i] + 1) }, nil
	case "dense_rank":
		return func(w *windowCtx, i int) any {
			r := 1
			for k := 1; k <= w.first[i]; k++ {
				if w.first[k] == k {
					r++
				}
			}
			return float64(r)
		}, nil
	case "percent_rank":
		return func(w *windowCtx, i int) any {
			if len(w.rows) <= 1 {
				return 0.0
			}
			return float64(w.first[i]) / (n(w) - 1)
		}, nil
	case "cume_dist":
		return func(w *windowCtx, i int) any { return float64(w.last[i]+1) / n(w) }, nil
	case "ntile":
		return func(w *windowCtx, i int) any {
			k := w.param
			if !w.hasParam || k <= 0 {
				return math.NaN()
			}
			return math.Ceil(k * float64(i+1) / n(w))
		}, nil
	case "lag", "lead":
		sign := -1
		if op == "lead" {
			sign = 1
		}
		return func(w *windowCtx, i int) any {
			off := 1
			if w.hasParam {
				off = int(w.param)
			}
			j := i + sign*off
			if j < 0 || j >= len(w.rows) || w.field == nil {
				return nil
			}
			return w.field.Get(w.rows[j])
		}, nil
	case "first_value", "last_value", "nth_value":
		return func(w *windowCtx, i int) any {
			lo, hi := w.bounds(i)
			if w.field == nil || lo > hi {
				return nil
			}
			switch op {
			case "first_value":
				return w.field.Get(w.rows[lo])
			case "last_value":
				return w.field.Get(w.rows[hi])
			}
			k := int(w.param)
			if !w.hasParam || k < 1 || lo+k-1 > hi {
				return nil
			}
			return w.field.Get(w.rows[lo+k-1])
		}, nil
	case "prev_value", "next_value":
		return func(w *windowCtx, i int) any {
			if w.field == nil {
				return nil
			}
			step := -1
			if op == "next_value" {
				step = 1
			}
			for j := i; j >= 0 && j < len(w.rows); j += step {
				if v := w.field.Get(w.rows[j]); valid(v) {
					return v
				}
			}
			return nil
		}, nil
	}
	m, ok := measures[op]
	if !ok {
		return nil, fmt.Errorf("the window op %q: %w", op, ErrParam)
	}
	return func(w *windowCtx, i int) any {
		lo, hi := w.bounds(i)
		if lo > hi {
			return m(nil, nil)
		}
		frame := w.rows[lo : hi+1]
		vals := make([]any, len(frame))
		if w.field != nil {
			for j, r := range frame {
				vals[j] = w.field.Get(r)
			}
		}
		return m(frame, vals)
	}, nil
}

// impute adds the rows that are missing from each group, one for each
// value of the key that the group lacks, with a value for the field.
func impute(p *params, rows []Row) ([]Row, error) {
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	k, err := p.field("key")
	if err != nil {
		return nil, err
	}
	if f == nil || k == nil {
		return nil, fmt.Errorf("impute without a field or a key: %w", ErrParam)
	}
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	keyvals, err := p.list("keyvals")
	if err != nil {
		return nil, err
	}
	method, err := p.str("method", "value")
	if err != nil {
		return nil, err
	}
	fill, _, err := p.raw("value")
	if err != nil {
		return nil, err
	}
	// The values of the key, in the order they first appear, then those of
	// keyvals.
	var keys []any
	seen := map[string]bool{}
	addKey := func(v any) {
		if s := key([]any{v}); !seen[s] {
			seen[s] = true
			keys = append(keys, v)
		}
	}
	for _, r := range rows {
		addKey(k.Get(r))
	}
	for _, v := range keyvals {
		addKey(v)
	}
	a := &aggregation{groupby: groupby}
	order, groups, gkeys := a.group(rows)
	out := slices.Clone(rows)
	for _, g := range order {
		have := map[string]bool{}
		var vals []any
		for _, r := range groups[g] {
			have[key([]any{k.Get(r)})] = true
			vals = append(vals, f.Get(r))
		}
		v := fill
		if method != "value" {
			m, ok := measures[method]
			if !ok {
				return nil, fmt.Errorf("the impute method %q: %w", method, ErrParam)
			}
			v = m(groups[g], vals)
		}
		for _, kv := range keys {
			if have[key([]any{kv})] {
				continue
			}
			// An imputed row says so in the field _impute, as in Vega.
			r := Row{f.Name: v, k.Name: kv, "_impute": true}
			for i, gb := range groupby {
				r[gb.Name] = gkeys[g][i]
			}
			out = append(out, r)
		}
	}
	return out, nil
}

// pivot turns the values of a field into fields, one for each distinct
// value, which hold an aggregate of another field, with one row for each
// group.
func pivot(p *params, rows []Row) ([]Row, error) {
	groupby, err := p.fields("groupby")
	if err != nil {
		return nil, err
	}
	f, err := p.field("field")
	if err != nil {
		return nil, err
	}
	v, err := p.field("value")
	if err != nil {
		return nil, err
	}
	if f == nil || v == nil {
		return nil, fmt.Errorf("pivot without a field or a value: %w", ErrParam)
	}
	op, err := p.str("op", "sum")
	if err != nil {
		return nil, err
	}
	m, ok := measures[op]
	if !ok {
		return nil, fmt.Errorf("the pivot op %q: %w", op, ErrParam)
	}
	limit, err := p.number("limit", 0)
	if err != nil {
		return nil, err
	}
	// The pivot fields, sorted.
	var names []string
	seen := map[string]bool{}
	for _, r := range rows {
		n := expr.String(f.Get(r))
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	slices.Sort(names)
	if limit > 0 && int(limit) < len(names) {
		names = names[:int(limit)]
	}
	a := &aggregation{groupby: groupby}
	order, groups, keys := a.group(rows)
	out := make([]Row, 0, len(order))
	for _, g := range order {
		r := Row{}
		for i, gb := range groupby {
			r[gb.Name] = keys[g][i]
		}
		byName := map[string][]Row{}
		for _, row := range groups[g] {
			n := expr.String(f.Get(row))
			byName[n] = append(byName[n], row)
		}
		for _, n := range names {
			grp := byName[n]
			if len(grp) == 0 {
				// A value that the group does not have is no field.
				continue
			}
			vals := make([]any, len(grp))
			for i, row := range grp {
				vals[i] = v.Get(row)
			}
			r[n] = m(grp, vals)
		}
		out = append(out, r)
	}
	return out, nil
}
