package zuzu

import (
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

// knownProjectionParams are the other parameters that Vega reads. The
// parameters of the projections of d3-geo-projection apply to none of the
// types that Vega and zuzu have, so they change nothing.
var knownProjectionParams = []string{
	"fit", "extent", "size", "pointRadius", "coefficient", "distance",
	"fraction", "lobes", "parallel", "radius", "ratio", "spacing", "tilt",
}

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

// geoFuncs adds the functions of expressions that measure a GeoJSON object
// in the plane of a projection: geoArea, geoBounds and geoCentroid. With
// no projection, Vega measures on the sphere, which zuzu does not support.
func (s *scope) geoFuncs(fns map[string]expr.Func) {
	measure := func(name string, f func(p *geo.Projection, o any) any) expr.Func {
		return func(_ *expr.Scope, args []any) (any, error) {
			if len(args) < 2 || !expr.Truthy(args[0]) {
				return nil, vega.Issue{Err: vega.ErrUnsupportedFunction, Message: "the function " + name + " with no projection"}
			}
			p, err := s.projection(expr.String(args[0]))
			if err != nil {
				return nil, err
			}
			return f(p, args[1]), nil
		}
	}
	fns["geoArea"] = measure("geoArea", func(p *geo.Projection, o any) any {
		return geo.Area(p, o)
	})
	fns["geoBounds"] = measure("geoBounds", func(p *geo.Projection, o any) any {
		b := geo.Bounds(p, o)
		return []any{[]any{b[0][0], b[0][1]}, []any{b[1][0], b[1][1]}}
	})
	fns["geoCentroid"] = measure("geoCentroid", func(p *geo.Projection, o any) any {
		x, y := geo.Centroid(p, o)
		return []any{x, y}
	})
}
