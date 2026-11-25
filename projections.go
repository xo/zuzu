package zuzu

import (
	"errors"
	"fmt"
	"slices"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/geo"
	"github.com/xo/zuzu/vega"
)

// projState is the state of a projection of a scope.
type projState struct {
	def   *vega.Projection
	path  string
	proj  *geo.Projection
	state state
	err   error
}

// projectionParams are the parameters of a projection in the order that
// Vega sets them, which matters: a Mercator projection works out its clip
// when its scale, its translation or its center changes.
var projectionParams = []string{
	"clipAngle", "clipExtent", "scale", "translate", "center", "rotate",
	"parallels", "precision", "reflectX", "reflectY",
}

// knownProjectionParams are the other parameters that Vega reads: fit,
// extent, size and pointRadius, which the runtime reads apart, and the
// parameters of the projections of d3-geo-projection.
var knownProjectionParams = []string{
	"fit", "extent", "size", "pointRadius", "coefficient", "distance",
	"fraction", "lobes", "parallel", "radius", "ratio", "spacing", "tilt",
}

// extendedParams are the parameters of the projections of
// d3-geo-projection. A type that takes one reads it, or reports it when
// zuzu does not read it yet. A type that does not take one ignores it, as
// vega.js does (D52).
var extendedParams = []string{"coefficient", "distance", "fraction", "lobes", "parallel", "radius", "ratio", "spacing", "tilt"}

// projection returns a projection of this scope or of a scope above, by
// name.
func (s *scope) projection(name string) (*geo.Projection, error) {
	for sc := s; sc != nil; sc = sc.parent {
		st, ok := sc.projections[name]
		if !ok {
			continue
		}
		switch st.state {
		case done:
			return st.proj, st.err
		case busy:
			return nil, fmt.Errorf("the projection %q: %w", name, ErrCycle)
		}
		st.state = busy
		st.proj, st.err = sc.makeProjection(st)
		st.state = done
		return st.proj, st.err
	}
	return nil, fmt.Errorf("the projection %q: %w", name, ErrUnknownName)
}

// makeProjection makes a projection from its definition, and fits it to its
// data.
func (s *scope) makeProjection(st *projState) (*geo.Projection, error) {
	d := st.def
	t, err := s.value(d.Type)
	if err != nil {
		return nil, fmt.Errorf("%s/type: %w", st.path, err)
	}
	typ, _ := t.(string)
	p, err := geo.New(typ)
	if err != nil {
		// A type of d3-geo-projection that Vega does not have is
		// unsupported, as the transforms that read the projection.
		return nil, vega.Issue{Err: vega.ErrUnsupported, Path: st.path + "/type", Message: fmt.Sprintf("the projection type %q", typ)}
	}
	param := func(name string) (any, error) {
		raw, ok := d.Params[name]
		if !ok {
			return nil, nil
		}
		var v any
		if err := rawUnmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", st.path, name, err)
		}
		v, err := s.deepValue(v)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", st.path, name, err)
		}
		return v, nil
	}
	for name := range d.Params {
		if !slices.Contains(projectionParams, name) && !slices.Contains(knownProjectionParams, name) {
			if err := s.rt.issue(vega.ErrUnsupported, st.path+"/"+name, "the parameter %s of a projection", name); err != nil {
				return nil, err
			}
		}
	}
	for _, name := range projectionParams {
		v, err := param(name)
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		if err := p.Set(name, v); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", st.path, name, err)
		}
	}
	for _, name := range extendedParams {
		v, err := param(name)
		if err != nil {
			return nil, err
		}
		if v == nil || !geo.TakesParam(p.Type(), name) {
			continue
		}
		switch err := p.Set(name, v); {
		case errors.Is(err, geo.ErrParamUnread):
			if err := s.rt.issue(vega.ErrUnsupported, st.path+"/"+name, "the parameter %s of a projection of the type %s", name, p.Type()); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, fmt.Errorf("%s/%s: %w", st.path, name, err)
		}
	}
	r, err := param("pointRadius")
	if err != nil {
		return nil, err
	}
	if r != nil {
		p.Radius = geo.Number(r)
	}
	fit, err := param("fit")
	if err != nil || fit == nil {
		return p, err
	}
	extent, err := param("extent")
	if err != nil {
		return nil, err
	}
	size, err := param("size")
	if err != nil {
		return nil, err
	}
	object := collectGeoJSON(fit)
	switch {
	case extent != nil:
		e, ok := extent.([]any)
		if !ok || len(e) < 2 {
			return nil, fmt.Errorf("%s/extent %v: %w", st.path, extent, geo.ErrProjection)
		}
		a, _ := e[0].([]any)
		b, _ := e[1].([]any)
		if len(a) < 2 || len(b) < 2 {
			return nil, fmt.Errorf("%s/extent %v: %w", st.path, extent, geo.ErrProjection)
		}
		p.Fit(geo.Number(a[0]), geo.Number(a[1]), geo.Number(b[0]), geo.Number(b[1]), object)
	case size != nil:
		z, ok := size.([]any)
		if !ok || len(z) < 2 {
			return nil, fmt.Errorf("%s/size %v: %w", st.path, size, geo.ErrProjection)
		}
		p.Fit(0, 0, geo.Number(z[0]), geo.Number(z[1]), object)
	}
	return p, nil
}

// deepValue evaluates the signals inside a value of a spec: an object
// {"signal": ...} anywhere in a list.
func (s *scope) deepValue(v any) (any, error) {
	switch v := v.(type) {
	case map[string]any:
		if src, ok := v["signal"].(string); ok && len(v) == 1 {
			return s.eval(src, nil)
		}
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			y, err := s.deepValue(x)
			if err != nil {
				return nil, err
			}
			out[i] = y
		}
		return out, nil
	}
	return v, nil
}

// collectGeoJSON joins the data of a fit as one GeoJSON object: a single
// object as it is, and a list as a collection of features.
func collectGeoJSON(v any) any {
	l, ok := v.([]any)
	if !ok {
		return v
	}
	if len(l) == 1 {
		return l[0]
	}
	var features []any
	for _, f := range l {
		features = append(features, featurize(f)...)
	}
	return map[string]any{"type": "FeatureCollection", "features": features}
}

// featurize returns the features of a GeoJSON object: the features of a
// collection, or each geometry as a feature.
func featurize(v any) []any {
	if m, ok := v.(map[string]any); ok && m["type"] == "FeatureCollection" {
		fs, _ := m["features"].([]any)
		return fs
	}
	l, ok := v.([]any)
	if !ok {
		l = []any{v}
	}
	var out []any
	for _, d := range l {
		if d == nil {
			continue
		}
		if m, ok := d.(map[string]any); ok && m["type"] == "Feature" {
			out = append(out, d)
			continue
		}
		out = append(out, map[string]any{"type": "Feature", "geometry": d})
	}
	return out
}

// geoFuncs adds the functions of expressions that measure or draw a
// GeoJSON object: geoArea, geoBounds and geoCentroid, which measure in the
// plane of a projection, or on the sphere when the projection is null, and
// geoShape, which draws a path with a projection. A projection is a name or
// a copy that the function copy returns.
func (s *scope) geoFuncs(fns map[string]expr.Func) {
	projectionOf := func(v any) (*geo.Projection, error) {
		if c, ok := v.(projectionCopy); ok {
			return c.p, nil
		}
		return s.projection(expr.String(v))
	}
	measure := func(plane func(p *geo.Projection, o any) any, sphere func(o any) any) expr.Func {
		return func(_ *expr.Scope, args []any) (any, error) {
			o := argAt(args, 1)
			if !expr.Truthy(argAt(args, 0)) {
				return sphere(o), nil
			}
			p, err := projectionOf(args[0])
			if err != nil {
				return nil, err
			}
			return plane(p, o), nil
		}
	}
	pair := func(x, y float64) any { return []any{x, y} }
	box := func(b [2][2]float64) any { return []any{pair(b[0][0], b[0][1]), pair(b[1][0], b[1][1])} }
	fns["geoArea"] = measure(
		func(p *geo.Projection, o any) any { return geo.Area(p, o) },
		func(o any) any { return geo.SphereArea(o) },
	)
	fns["geoBounds"] = measure(
		func(p *geo.Projection, o any) any { return box(geo.Bounds(p, o)) },
		func(o any) any { return box(geo.SphereBounds(o)) },
	)
	fns["geoCentroid"] = measure(
		func(p *geo.Projection, o any) any { return pair(geo.Centroid(p, o)) },
		func(o any) any { return pair(geo.SphereCentroid(o)) },
	)
	fns["geoShape"] = func(_ *expr.Scope, args []any) (any, error) {
		if !expr.Truthy(argAt(args, 0)) {
			return geoShape{}, nil
		}
		p, err := projectionOf(args[0])
		if err != nil {
			return nil, err
		}
		path, _ := geo.Path(p, argAt(args, 1), p.Radius)
		return geoShape{path: path}, nil
	}
}

// geoShape is what the function geoShape returns: the path of a GeoJSON
// object in a projection, which a shape mark draws as its shape. vega.js
// returns a function that draws the path, and zuzu keeps the path. With no
// projection, the path is empty.
type geoShape struct {
	path string
}
