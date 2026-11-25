package zuzu

import (
	"fmt"
	"maps"
	"math"
	"time"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/geo"
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
	arg := argAt
	// scaleOf returns the scale that the first argument names, or nil for
	// a copy of a scale. vega.js finds no scale for a copy, so a function
	// of a scale treats a copy as a name that it does not know.
	scaleOf := func(name string, args []any) (scale.Scale, error) {
		switch arg(args, 0).(type) {
		case scaleCopy:
			return nil, nil
		case projectionCopy:
			return nil, vega.Issue{Err: vega.ErrUnsupportedFunction, Message: "a projection in the function " + name}
		}
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
			if p, ok, err := s.projectionArg(arg(args, 0)); ok || err != nil {
				return project(p, arg(args, 1)), err
			}
			sc, err := scaleOf("scale", args)
			if err != nil || sc == nil {
				return expr.Undefined, err
			}
			return sc.Map(arg(args, 1)), nil
		},
		"invert": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf("invert", args)
			if err != nil || sc == nil {
				return expr.Undefined, err
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
			sc, err := scaleOf("bandwidth", args)
			if err != nil || sc == nil {
				return 0.0, err
			}
			return sc.Bandwidth(), nil
		},
		"domain": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf("domain", args)
			if err != nil || sc == nil {
				return []any{}, err
			}
			return sc.Domain(), nil
		},
		"range": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf("range", args)
			if err != nil || sc == nil {
				return []any{}, err
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
		"copy": func(_ *expr.Scope, args []any) (any, error) {
			return s.copyOf(arg(args, 0))
		},
		"gradient": func(_ *expr.Scope, args []any) (any, error) {
			sc, err := scaleOf("gradient", args)
			if err != nil {
				return nil, err
			}
			if sc == nil {
				return nil, fmt.Errorf("a copy of a scale in the function gradient: %w", expr.ErrArgument)
			}
			return gradient(sc, arg(args, 1), arg(args, 2), arg(args, 3))
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
	s.treeFuncs(fns)
	s.selectionFuncs(fns)
	maps.Copy(fns, s.rt.chart.opts.funcs)
	for _, name := range []string{"screen", "windowSize"} {
		if _, ok := fns[name]; !ok {
			fns[name] = unsupported(name)
		}
	}
	return fns
}

// scaleCopy is the copy of a scale that the function copy returns. zuzu
// never changes a scale after it builds it, so the copy shares the scale.
type scaleCopy struct {
	s scale.Scale
}

// projectionCopy is the copy of a projection that the function copy
// returns, which the functions of projections take in place of a name.
// zuzu never changes a projection after it fits it, so the copy shares the
// projection.
type projectionCopy struct {
	p *geo.Projection
}

// copyOf returns a copy of the scale or the projection of a name, or
// undefined for a name that is neither. A copy of a copy of a projection is
// a copy, and a copy of a copy of a scale is undefined, as in vega.js.
func (s *scope) copyOf(v any) (any, error) {
	switch x := v.(type) {
	case projectionCopy:
		return x, nil
	case string:
		for sc := s; sc != nil; sc = sc.parent {
			if _, ok := sc.scales[x]; ok {
				c, err := s.scale(x)
				if err != nil {
					return nil, err
				}
				return scaleCopy{s: c}, nil
			}
			if _, ok := sc.projections[x]; ok {
				p, err := s.projection(x)
				if err != nil {
					return nil, err
				}
				return projectionCopy{p: p}, nil
			}
		}
	}
	return expr.Undefined, nil
}

// projectionArg returns the projection that the first argument of the
// function scale names, when it names no scale: a copy of a projection, or
// the name of one.
func (s *scope) projectionArg(v any) (*geo.Projection, bool, error) {
	switch x := v.(type) {
	case projectionCopy:
		return x.p, true, nil
	case string:
		for sc := s; sc != nil; sc = sc.parent {
			if _, ok := sc.scales[x]; ok {
				return nil, false, nil
			}
			if _, ok := sc.projections[x]; ok {
				p, err := s.projection(x)
				return p, err == nil, err
			}
		}
	}
	return nil, false, nil
}

// project maps a point [longitude, latitude] with a projection to [x, y],
// or to null for a point that the projection does not show.
func project(p *geo.Projection, v any) any {
	l, ok := v.([]any)
	if p == nil || !ok || len(l) < 2 {
		return expr.Undefined
	}
	x, y, ok := p.Project(expr.Number(l[0]), expr.Number(l[1]))
	if !ok {
		return nil
	}
	return []any{x, y}
}

// gradient returns the linear gradient of a continuous scale of colors,
// from p0 to p1, each an [x, y] in fractions of the bounds of the item:
// [0, 0] and [1, 0] when they are not given. count asks for about that many
// stops, 15 by default.
func gradient(sc scale.Scale, p0, p1, count any) (any, error) {
	offsets, colors, ok := scale.GradientStops(sc, expr.Number(count))
	if !ok {
		return nil, vega.Issue{Err: vega.ErrUnsupportedFunction, Message: "the function gradient of a scale of the type " + sc.Type()}
	}
	stops := make([]any, len(offsets))
	for i, f := range offsets {
		stops[i] = map[string]any{"offset": f, "color": colors[i]}
	}
	at := func(p any, i int, def float64) any {
		if !expr.Truthy(p) {
			return def
		}
		if l, ok := p.([]any); ok && i < len(l) {
			return l[i]
		}
		return expr.Undefined
	}
	return map[string]any{
		"gradient": "linear",
		"x1":       at(p0, 0, 0), "y1": at(p0, 1, 0),
		"x2": at(p1, 0, 1), "y2": at(p1, 1, 0),
		"stops": stops,
	}, nil
}

// argAt returns the argument at an index, or undefined past the end.
func argAt(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return expr.Undefined
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
