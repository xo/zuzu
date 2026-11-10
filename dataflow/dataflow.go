// Package dataflow reads the data of a Vega spec and runs its transforms
// (D6). It runs each dataset once, from its source to its last transform,
// since zuzu draws a static chart (D7). The view gives it the signals, the
// other datasets and the expressions through an Env.
package dataflow

import (
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/vega"
)

// Error is an error of the package dataflow.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package.
const (
	// ErrParam is the error of a parameter of a transform that is missing
	// or has a value that the transform cannot use.
	ErrParam Error = "invalid transform parameter"
	// ErrFormat is the error of data that does not read in its format.
	ErrFormat Error = "invalid data"
)

// Row is a row of a dataset. Its values are the values of expressions:
// float64, string, bool, nil, time.Time, []any and map[string]any.
type Row = map[string]any

// Env is what a transform reads from the view.
type Env interface {
	// Compile compiles an expression, which can read signals.
	Compile(src string) (*expr.Program, error)
	// Scope returns the scope that an expression of the view runs in, with
	// a datum.
	Scope(datum any) *expr.Scope
	// Data returns the rows of a dataset, after its transforms.
	Data(name string) ([]Row, error)
	// SetSignal sets a signal that a transform writes, such as the signal of
	// an extent.
	SetSignal(name string, v any)
	// Location is the time zone of local time (D27).
	Location() *time.Location
	// Random returns a random number from 0 to 1, from the source of the
	// render (D28).
	Random() float64
}

// transformFunc runs a transform over rows.
type transformFunc func(p *params, rows []Row) ([]Row, error)

// Supported reports whether zuzu runs a transform type.
func Supported(typ string) bool {
	_, ok := transforms[typ]
	return ok
}

// Run runs transforms over rows, in order, and returns the rows of the
// last. A transform that zuzu does not run is an error that wraps
// vega.ErrUnsupportedTransform. path is the JSON pointer of the list of
// transforms in the spec, for the errors.
func Run(env Env, rows []Row, ts []vega.Transform, path string) ([]Row, error) {
	for i, t := range ts {
		fn, ok := transforms[t.Type]
		if !ok {
			return nil, vega.Issue{Err: vega.ErrUnsupportedTransform, Path: fmt.Sprintf("%s/%d", path, i), Message: t.Type}
		}
		out, err := fn(&params{env: env, t: t}, rows)
		if err != nil {
			return nil, fmt.Errorf("%s/%d: transform %s: %w", path, i, t.Type, err)
		}
		rows = out
	}
	return rows, nil
}

// params reads the parameters of a transform. A parameter can be a
// literal, a signal, or a reference to a field or an expression.
type params struct {
	env Env
	t   vega.Transform
}

// raw returns a parameter as a value, with a signal evaluated, and whether
// the transform has it.
func (p *params) raw(name string) (any, bool, error) {
	v, ok := p.t.Param(name)
	if !ok {
		return nil, false, nil
	}
	if v.Signal != "" {
		out, err := p.eval(v.Signal)
		return out, true, err
	}
	var out any
	if err := v.Decode(&out); err != nil {
		return nil, true, fmt.Errorf("reading %s: %w: %w", name, ErrParam, err)
	}
	return resolve(p, out)
}

// resolve evaluates the signals in a literal, such as an element of an
// array that is {"signal": "expr"}.
func resolve(p *params, v any) (any, bool, error) {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			r, _, err := resolve(p, e)
			if err != nil {
				return nil, true, err
			}
			out[i] = r
		}
		return out, true, nil
	case map[string]any:
		if s, ok := x["signal"].(string); ok && len(x) == 1 {
			r, err := p.eval(s)
			return r, true, err
		}
	}
	return v, true, nil
}

// eval evaluates an expression with no datum.
func (p *params) eval(src string) (any, error) {
	prog, err := p.env.Compile(src)
	if err != nil {
		return nil, err
	}
	return prog.Eval(p.env.Scope(nil))
}

// number returns a number parameter, or def.
func (p *params) number(name string, def float64) (float64, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return def, err
	}
	return expr.Number(v), nil
}

// str returns a string parameter, or def.
func (p *params) str(name, def string) (string, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return def, err
	}
	return expr.String(v), nil
}

// boolean returns a boolean parameter, or def.
func (p *params) boolean(name string, def bool) (bool, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return def, err
	}
	return expr.Truthy(v), nil
}

// list returns a parameter as a list. A single value is a list of one.
func (p *params) list(name string) ([]any, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return nil, err
	}
	if l, ok := v.([]any); ok {
		return l, nil
	}
	return []any{v}, nil
}

// strings returns a parameter as a list of strings. A null element stays
// an empty string.
func (p *params) strings(name string) ([]string, error) {
	l, err := p.list(name)
	if err != nil || l == nil {
		return nil, err
	}
	out := make([]string, len(l))
	for i, v := range l {
		if v != nil {
			out[i] = expr.String(v)
		}
	}
	return out, nil
}

// numbers returns a parameter as a list of numbers.
func (p *params) numbers(name string) ([]float64, error) {
	l, err := p.list(name)
	if err != nil || l == nil {
		return nil, err
	}
	out := make([]float64, len(l))
	for i, v := range l {
		out[i] = expr.Number(v)
	}
	return out, nil
}

// field returns a parameter that names a field, or nil if the transform
// does not have it.
func (p *params) field(name string) (*Accessor, error) {
	v, ok, err := p.raw(name)
	if err != nil || !ok || v == nil {
		return nil, err
	}
	return p.accessor(v)
}

// fields returns a parameter that names a list of fields. An element that
// is null is a nil accessor, as for the count of an aggregate.
func (p *params) fields(name string) ([]*Accessor, error) {
	l, err := p.list(name)
	if err != nil || l == nil {
		return nil, err
	}
	out := make([]*Accessor, len(l))
	for i, v := range l {
		if v == nil {
			continue
		}
		a, err := p.accessor(v)
		if err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}

// accessor reads a field name, or an object {"field": name} or
// {"expr": src}.
func (p *params) accessor(v any) (*Accessor, error) {
	switch x := v.(type) {
	case string:
		return FieldAccessor(x), nil
	case map[string]any:
		if f, ok := x["field"].(string); ok {
			a := FieldAccessor(f)
			if as, ok := x["as"].(string); ok {
				a.Name = as
			}
			return a, nil
		}
		if src, ok := x["expr"].(string); ok {
			prog, err := p.env.Compile(src)
			if err != nil {
				return nil, err
			}
			env := p.env
			a := &Accessor{Name: src, get: func(r Row) any {
				out, err := prog.Eval(env.Scope(r))
				if err != nil {
					return expr.Undefined
				}
				return out
			}}
			if as, ok := x["as"].(string); ok {
				a.Name = as
			}
			return a, nil
		}
	}
	return nil, fmt.Errorf("reading the field %v: %w", v, ErrParam)
}

// expr returns a parameter that holds an expression, compiled to run on a
// row.
func (p *params) expr(name string) (func(Row) (any, error), error) {
	src, err := p.str(name, "")
	if err != nil {
		return nil, err
	}
	if src == "" {
		return nil, fmt.Errorf("reading the expression %s: %w", name, ErrParam)
	}
	prog, err := p.env.Compile(src)
	if err != nil {
		return nil, err
	}
	env := p.env
	return func(r Row) (any, error) {
		return prog.Eval(env.Scope(r))
	}, nil
}

// as returns the output names of a transform, from the parameter as, with
// defaults for the names it does not give.
func (p *params) as(defaults ...string) ([]string, error) {
	names, err := p.strings("as")
	if err != nil {
		return nil, err
	}
	out := make([]string, max(len(defaults), len(names)))
	for i := range out {
		switch {
		case i < len(names) && names[i] != "":
			out[i] = names[i]
		case i < len(defaults):
			out[i] = defaults[i]
		}
	}
	return out, nil
}

// clone returns a copy of a row, with room for more fields.
func clone(r Row) Row {
	out := make(Row, len(r)+2)
	maps.Copy(out, r)
	return out
}

// valid reports whether a value counts in a sum or a mean: it is not null,
// undefined or NaN.
func valid(v any) bool {
	if v == nil || expr.IsUndefined(v) {
		return false
	}
	if f, ok := v.(float64); ok && math.IsNaN(f) {
		return false
	}
	return true
}

// key returns a string that identifies a list of values, to group rows.
func key(vals []any) string {
	var b strings.Builder
	for i, v := range vals {
		if i > 0 {
			b.WriteByte('|')
		}
		switch x := v.(type) {
		case nil:
			b.WriteString("\x00null")
		case string:
			b.WriteString(strings.ReplaceAll(x, "|", "\\|"))
		case time.Time:
			b.WriteString(expr.String(float64(x.UnixMilli())))
		default:
			if expr.IsUndefined(v) {
				b.WriteString("\x00undefined")
			} else {
				b.WriteString(expr.String(v))
			}
		}
	}
	return b.String()
}
