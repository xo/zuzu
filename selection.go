package zuzu

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
)

// The functions of this file are the helpers of the selections of
// Vega-Lite: vlSelectionTest, vlSelectionIdTest and vlSelectionResolve.
// Each reads a store, a dataset with an entry for each selected value or
// range. zuzu draws a chart once, with no events, so a store holds only
// the entries that the spec gives it, such as the value of a param that
// vega-lite.js compiles into the values of the store.
//
// An entry of a store is {unit, fields, values}. unit names the view that
// made the entry. Each field is {field, channel, type}, and the value at
// the same index is what the field must match. The type says how:
//
//	E       the value, or one of the values of an array
//	R       a range [lo, hi], with both ends in it
//	R-RE    a range with its low end in it, as a bin
//	R-E     a range with neither end in it
//	R-LE    a range with its high end in it
//	E-LT    less than the value, and E-LTE, E-GT and E-GTE as their names say
//	E-VALID neither null nor NaN
//	E-ONE   one of the values of an array
//
// An entry of a point selection with no fields is {unit, _vgsid_}, the id
// of the row that it selects.

// The ops that join the units of a selection, and the field of the id of
// a row that a point selection selects.
const (
	opUnion     = "union"
	opIntersect = "intersect"
	selectionID = "_vgsid_"
)

// selectionFuncs adds the helpers of the selections of Vega-Lite.
func (s *scope) selectionFuncs(fns map[string]expr.Func) {
	store := func(args []any) ([]dataflow.Row, error) {
		name := ""
		if len(args) > 0 {
			name = expr.String(args[0])
		}
		return s.dataset(name)
	}
	fns["vlSelectionTest"] = func(_ *expr.Scope, args []any) (any, error) {
		entries, err := store(args)
		if err != nil {
			return nil, err
		}
		return selectionTest(entries, rowOf(argAt(args, 1)), expr.String(argAt(args, 2)) == opIntersect), nil
	}
	fns["vlSelectionIdTest"] = func(_ *expr.Scope, args []any) (any, error) {
		entries, err := store(args)
		if err != nil {
			return nil, err
		}
		return selectionIDTest(entries, rowOf(argAt(args, 1)), expr.String(argAt(args, 2)) == opIntersect), nil
	}
	fns["vlSelectionResolve"] = func(_ *expr.Scope, args []any) (any, error) {
		entries, err := store(args)
		if err != nil {
			return nil, err
		}
		op := opUnion
		if v := argAt(args, 1); expr.Truthy(v) {
			op = expr.String(v)
		}
		return selectionResolve(entries, op, expr.Truthy(argAt(args, 2)), expr.Truthy(argAt(args, 3))), nil
	}
}

// rowOf returns a datum as a row, or an empty row.
func rowOf(v any) dataflow.Row {
	if r, ok := v.(map[string]any); ok {
		return r
	}
	return dataflow.Row{}
}

// unitOf returns the unit of an entry of a store.
func unitOf(e dataflow.Row) string {
	return expr.String(e["unit"])
}

// units returns the units of the entries of a store, in their order.
func units(entries []dataflow.Row) []string {
	var out []string
	for _, e := range entries {
		if u := unitOf(e); !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
}

// selectionTest reports whether a datum is in a selection. With union, it
// is in when it matches an entry. With intersect, it must match an entry
// of each unit. An empty store gives 0, as in vega.js, and the empty of a
// Vega-Lite predicate is in the expression that calls it, such as
// !length(data('brush_store')) || vlSelectionTest('brush_store', datum).
func selectionTest(entries []dataflow.Row, datum dataflow.Row, intersect bool) any {
	if len(entries) == 0 {
		return 0.0
	}
	if !intersect {
		return slices.ContainsFunc(entries, func(e dataflow.Row) bool { return matchEntry(datum, e) })
	}
	for _, u := range units(entries) {
		if !slices.ContainsFunc(entries, func(e dataflow.Row) bool { return unitOf(e) == u && matchEntry(datum, e) }) {
			return false
		}
	}
	return true
}

// selectionIDTest reports whether the id of a datum is in a selection of
// ids. With intersect, each unit must hold the id.
func selectionIDTest(entries []dataflow.Row, datum dataflow.Row, intersect bool) bool {
	id := dataflow.FieldAccessor(selectionID).Get(datum)
	n := 0
	for _, e := range entries {
		if expr.StrictEqual(e[selectionID], id) {
			n++
		}
	}
	if n == 0 {
		return false
	}
	if u := len(units(entries)); intersect && u > 1 {
		return n >= u
	}
	return true
}

// plain returns a date as its number of milliseconds, and any other value
// as it is.
func plain(v any) any {
	if t, ok := v.(time.Time); ok {
		return expr.Number(t)
	}
	return v
}

// plainList returns a list whose first value is a date as numbers.
func plainList(v any) any {
	l, ok := v.([]any)
	if !ok || len(l) == 0 {
		return v
	}
	if _, ok := l[0].(time.Time); !ok {
		return v
	}
	out := make([]any, len(l))
	for i, x := range l {
		out[i] = expr.Number(x)
	}
	return out
}

// matchEntry reports whether a datum matches every field of an entry.
func matchEntry(datum, e dataflow.Row) bool {
	fields, _ := e["fields"].([]any)
	values, _ := e["values"].([]any)
	for i, f := range fields {
		def, _ := f.(map[string]any)
		got := plain(dataflow.FieldAccessor(expr.String(def["field"])).Get(datum))
		want := plainList(plain(argAt(values, i)))
		if !matchField(got, want, expr.String(def["type"])) {
			return false
		}
	}
	return true
}

// matchField reports whether a value matches the value of a field of an
// entry, by the type of the field.
func matchField(got, want any, typ string) bool {
	// rel reports whether a relation of JavaScript, such as <=, holds.
	rel := func(a any, op string, b any) bool {
		c, ok := expr.Compare(a, b)
		switch op {
		case "<":
			return ok && c < 0
		case "<=":
			return ok && c <= 0
		case ">":
			return ok && c > 0
		}
		return ok && c >= 0
	}
	switch typ {
	case "E":
		if l, ok := want.([]any); ok {
			return slices.ContainsFunc(l, func(x any) bool { return sameValueZero(x, got) })
		}
		return expr.StrictEqual(got, want)
	case "R":
		return inRange(got, want, true, true)
	case "R-RE":
		return inRange(got, want, true, false)
	case "R-E":
		return inRange(got, want, false, false)
	case "R-LE":
		return inRange(got, want, false, true)
	case "E-LT":
		return !rel(got, ">=", want)
	case "E-LTE":
		return !rel(got, ">", want)
	case "E-GT":
		return !rel(got, "<=", want)
	case "E-GTE":
		return !rel(got, "<", want)
	case "E-VALID":
		return got != nil && !math.IsNaN(expr.Number(got))
	case "E-ONE":
		switch l := want.(type) {
		case []any:
			return slices.ContainsFunc(l, func(x any) bool { return expr.StrictEqual(x, got) })
		case string:
			return strings.Contains(l, expr.String(got))
		}
		return false
	}
	return true
}

// sameValueZero is the equality of includes in JavaScript: ===, with NaN
// equal to itself.
func sameValueZero(a, b any) bool {
	x, okx := a.(float64)
	y, oky := b.(float64)
	if okx && oky && math.IsNaN(x) && math.IsNaN(y) {
		return true
	}
	return expr.StrictEqual(a, b)
}

// inRange reports whether a value lies in a range [lo, hi], in either
// order, with each end in it or not.
func inRange(v, r any, left, right bool) bool {
	l, ok := r.([]any)
	if !ok || len(l) == 0 {
		return false
	}
	lo, hi := l[0], l[len(l)-1]
	if c, ok := expr.Compare(lo, hi); ok && c > 0 {
		lo, hi = hi, lo
	}
	cmp := func(a, b any, orEqual bool) bool {
		c, ok := expr.Compare(a, b)
		return ok && (c < 0 || orEqual && c == 0)
	}
	return cmp(lo, v, left) && cmp(v, hi, right)
}

// resolvedField is the values of one field of a store, by unit.
type resolvedField struct {
	name  string
	typ   byte
	units []string
	vals  map[string][]any
}

// add joins values to the values of a unit, with the union of the type of
// the field.
func (f *resolvedField) add(unit string, vals []any, typ byte) {
	f.typ = typ
	base, ok := f.vals[unit]
	if !ok {
		f.units = append(f.units, unit)
	}
	f.vals[unit] = combine(base, vals, typ, false)
}

// selectionResolve returns the values that a selection holds, as an
// object of a list for each field: the values of a field that enumerates,
// and the range [lo, hi] of a field that ranges. The entries of each unit
// join by union, and the units join by op, union or intersect. With multi,
// the object also holds each entry as an object of its fields, at vlPoint
// with vl5 and at vlMulti without: {"or": [...]} for union, and
// {"and": [{"or": [...]}, ...]}, one for each unit, for intersect. A store
// of ids gives the ids at _vgsid_, as a list where vega.js gives a Set.
func selectionResolve(entries []dataflow.Row, op string, multi, vl5 bool) map[string]any {
	var (
		fields    []*resolvedField
		multiUnit []string
		multiRes  = map[string][]any{}
	)
	field := func(name string) *resolvedField {
		for _, f := range fields {
			if f.name == name {
				return f
			}
		}
		f := &resolvedField{name: name, vals: map[string][]any{}}
		fields = append(fields, f)
		return f
	}
	addMulti := func(unit string, obj map[string]any) {
		if !slices.Contains(multiUnit, unit) {
			multiUnit = append(multiUnit, unit)
		}
		multiRes[unit] = append(multiRes[unit], obj)
	}
	ids := false
	for _, e := range entries {
		unit := unitOf(e)
		defs, okf := e["fields"].([]any)
		values, okv := e["values"].([]any)
		if !okf || !okv {
			ids = true
			id := e[selectionID]
			f := field(selectionID)
			if _, ok := f.vals[unit]; !ok {
				f.units = append(f.units, unit)
			}
			f.vals[unit] = append(f.vals[unit], id)
			if multi {
				addMulti(unit, map[string]any{selectionID: id})
			}
			continue
		}
		for j, d := range defs {
			def, _ := d.(map[string]any)
			typ := byte('E')
			if strings.HasPrefix(expr.String(def["type"]), "R") {
				typ = 'R'
			}
			field(expr.String(def["field"])).add(unit, listOf(argAt(values, j)), typ)
		}
		if multi {
			obj := map[string]any{}
			for j, v := range values {
				if j < len(defs) {
					def, _ := defs[j].(map[string]any)
					obj[expr.String(def["field"])] = v
				}
			}
			addMulti(unit, obj)
		}
	}
	intersect := op == opIntersect
	out := map[string]any{}
	for _, f := range fields {
		switch {
		case ids && f.name == selectionID:
			out[f.name] = idSet(f, intersect)
		case ids:
			// vega.js leaves the other fields of a store of ids by unit.
			byUnit := map[string]any{}
			for _, u := range f.units {
				byUnit[u] = f.vals[u]
			}
			out[f.name] = byUnit
		default:
			acc := f.vals[f.units[0]]
			for _, u := range f.units[1:] {
				acc = combine(acc, f.vals[u], f.typ, intersect)
			}
			out[f.name] = acc
		}
	}
	if multi && len(multiUnit) > 0 {
		key := "vlMulti"
		if vl5 {
			key = "vlPoint"
		}
		if op == opUnion {
			var all []any
			for _, u := range multiUnit {
				all = append(all, multiRes[u]...)
			}
			out[key] = map[string]any{"or": all}
		} else {
			and := make([]any, len(multiUnit))
			for i, u := range multiUnit {
				and[i] = map[string]any{"or": multiRes[u]}
			}
			out[key] = map[string]any{"and": and}
		}
	}
	return out
}

// listOf returns a value as a list: null and undefined as an empty list,
// and any other value that is not a list as a list of one.
func listOf(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	if v == nil || expr.IsUndefined(v) {
		return []any{}
	}
	return []any{v}
}

// idSet joins the ids of the units of a store, with no id twice.
func idSet(f *resolvedField, intersect bool) []any {
	out := []any{}
	add := func(v any) {
		if !slices.ContainsFunc(out, func(x any) bool { return sameValueZero(x, v) }) {
			out = append(out, v)
		}
	}
	if !intersect {
		for _, u := range f.units {
			for _, v := range f.vals[u] {
				add(v)
			}
		}
		return out
	}
	for _, v := range f.vals[f.units[0]] {
		inAll := true
		for _, u := range f.units[1:] {
			if !slices.ContainsFunc(f.vals[u], func(x any) bool { return sameValueZero(x, v) }) {
				inAll = false
				break
			}
		}
		if inAll {
			add(v)
		}
	}
	return out
}

// combine joins the values of a field: for E, the values of both, or the
// values of base that value holds, and for R, the range that holds both
// ranges, or the range that both hold. An empty base gives value, as in
// vega.js, so a range of an intersect that is empty gives the next range.
func combine(base, value []any, typ byte, intersect bool) []any {
	if typ == 'E' {
		if len(base) == 0 {
			return slices.Clone(value)
		}
		if intersect {
			var out []any
			for _, v := range base {
				if slices.ContainsFunc(value, func(x any) bool { return sameValueZero(x, v) }) {
					out = append(out, v)
				}
			}
			if out == nil {
				out = []any{}
			}
			return out
		}
		out := slices.Clone(base)
		for _, v := range value {
			if !slices.ContainsFunc(out, func(x any) bool { return sameValueZero(x, v) }) {
				out = append(out, v)
			}
		}
		return out
	}
	at := func(i int) any {
		if i < len(value) {
			return value[i]
		}
		return expr.Undefined
	}
	var lo, hi any = expr.Number(at(0)), expr.Number(at(1))
	if c, ok := expr.Compare(lo, hi); ok && c > 0 {
		lo, hi = at(1), at(0)
	}
	if len(base) == 0 {
		return []any{lo, hi}
	}
	out := slices.Clone(base)
	greater := func(a, b any) bool { c, ok := expr.Compare(a, b); return ok && c > 0 }
	if intersect {
		if greater(out[0], hi) || greater(lo, out[1]) {
			return []any{}
		}
		if greater(lo, out[0]) {
			out[0] = lo
		}
		if greater(out[1], hi) {
			out[1] = hi
		}
		return out
	}
	if greater(out[0], lo) {
		out[0] = lo
	}
	if greater(hi, out[1]) {
		out[1] = hi
	}
	return out
}
