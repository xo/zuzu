package zuzu

import (
	"maps"
	"math"
	"time"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/vega"
)

// viewFuncs returns the functions of expressions that read the view: its
// data and its scales. A function that the caller gives takes the place of
// one of the same name.
func (s *scope) viewFuncs() map[string]expr.Func {
	str := func(args []any, i int) string {
		if i < len(args) {
			return expr.String(args[i])
		}
		return ""
	}
	arg := func(args []any, i int) any {
		if i < len(args) {
			return args[i]
		}
		return expr.Undefined
	}
	scaleOf := func(args []any) (scale.Scale, error) {
		return s.scale(str(args, 0))
	}
	fns := map[string]expr.Func{
		"data": func(_ *expr.Scope, args []any) (any, error) {
			rows, err := s.dataset(str(args, 0))
			if err != nil {
				return nil, err
			}
			out := make([]any, len(rows))
			for i, r := range rows {
				out[i] = r
			}
			return out, nil
		},
		"indata": func(_ *expr.Scope, args []any) (any, error) {
			rows, err := s.dataset(str(args, 0))
			if err != nil {
				return nil, err
			}
			f := dataflow.FieldAccessor(str(args, 1))
			want := arg(args, 2)
			for _, r := range rows {
				if same(f.Get(r), want) {
					return true, nil
				}
			}
			return false, nil
		},
		"scale": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf(args)
			if err != nil {
				return nil, err
			}
			return sc.Map(arg(args, 1)), nil
		},
		"invert": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf(args)
			if err != nil {
				return nil, err
			}
			if l, ok := arg(args, 1).([]any); ok {
				out := make([]any, len(l))
				for i, v := range l {
					out[i] = sc.Invert(expr.Number(v))
				}
				return out, nil
			}
			return sc.Invert(expr.Number(arg(args, 1))), nil
		},
		"bandwidth": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf(args)
			if err != nil {
				return nil, err
			}
			return sc.Bandwidth(), nil
		},
		"domain": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf(args)
			if err != nil {
				return nil, err
			}
			return sc.Domain(), nil
		},
		"range": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf(args)
			if err != nil {
				return nil, err
			}
			return sc.Range(), nil
		},
		"bandspace": func(_ *expr.Scope, args []any) (any, error) {
			n := number(arg(args, 0), 0)
			inner := number(arg(args, 1), 0)
			outer := number(arg(args, 2), 0)
			return math.Max(1, n-inner+2*outer), nil
		},
		"pluck": func(_ *expr.Scope, args []any) (any, error) {
			l, _ := arg(args, 0).([]any)
			f := dataflow.FieldAccessor(str(args, 1))
			out := make([]any, len(l))
			for i, v := range l {
				if m, ok := v.(map[string]any); ok {
					out[i] = f.Get(m)
				} else {
					out[i] = expr.Undefined
				}
			}
			return out, nil
		},
		"containerSize": func(*expr.Scope, []any) (any, error) {
			return []any{expr.Undefined, expr.Undefined}, nil
		},
		"inScope": func(*expr.Scope, []any) (any, error) { return false, nil },
		"info":    func(_ *expr.Scope, args []any) (any, error) { return arg(args, 0), nil },
		"warn":    func(_ *expr.Scope, args []any) (any, error) { return arg(args, 0), nil },
		"debug":   func(_ *expr.Scope, args []any) (any, error) { return arg(args, 0), nil },
	}
	s.geoFuncs(fns)
	maps.Copy(fns, s.rt.chart.opts.funcs)
	for _, name := range []string{
		"copy", "gradient", "geoArea", "geoBounds", "geoCentroid", "geoShape",
		"vlSelectionTest", "vlSelectionResolve", "treePath", "treeAncestors", "screen", "windowSize",
	} {
		if _, ok := fns[name]; !ok {
			fns[name] = unsupported(name)
		}
	}
	return fns
}

// unsupported returns a function that fails with ErrUnsupportedFunction.
func unsupported(name string) expr.Func {
	return func(*expr.Scope, []any) (any, error) {
		return nil, vega.Issue{Err: vega.ErrUnsupportedFunction, Message: "the function " + name}
	}
}

// same reports whether two values are the same, as === does, with dates
// equal at the same time. An array or an object is never the same.
func same(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case nil:
		return b == nil
	case time.Time:
		y, ok := b.(time.Time)
		return ok && x.Equal(y)
	}
	return false
}
