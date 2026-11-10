package dataflow

import (
	"fmt"
	"math"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/geo"
)

// ProjectionEnv is an Env that holds the projections of a view, by name.
// The geographic transforms need it.
type ProjectionEnv interface {
	Projection(name string) (*geo.Projection, error)
}

// projection returns the projection that a transform names, or nil if it
// names none.
func (p *params) projection() (*geo.Projection, error) {
	name, err := p.str("projection", "")
	if err != nil || name == "" {
		return nil, err
	}
	env, ok := p.env.(ProjectionEnv)
	if !ok {
		return nil, fmt.Errorf("the projection %q: %w", name, ErrParam)
	}
	return env.Projection(name)
}

// geojson collects GeoJSON features into a signal, for a projection to fit:
// the values of the field geojson, and a MultiPoint feature of the points
// of the fields of longitude and latitude. The rows pass unchanged.
func geojson(p *params, rows []Row) ([]Row, error) {
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	gj, err := p.field("geojson")
	if err != nil {
		return nil, err
	}
	name, err := p.str("signal", "")
	if err != nil {
		return nil, err
	}
	var lon, lat *Accessor
	if len(fields) >= 2 {
		lon, lat = fields[0], fields[1]
	}
	features := []any{}
	switch {
	case gj != nil:
		for _, r := range rows {
			features = append(features, gj.Get(r))
		}
	case lon == nil:
		for _, r := range rows {
			features = append(features, r)
		}
	}
	if lon != nil && lat != nil {
		points := []any{}
		for _, r := range rows {
			x, y := lon.Get(r), lat.Get(r)
			if x == nil || y == nil || expr.IsUndefined(x) || expr.IsUndefined(y) {
				continue
			}
			fx, fy := expr.Number(x), expr.Number(y)
			if math.IsNaN(fx) || math.IsNaN(fy) {
				continue
			}
			points = append(points, []any{fx, fy})
		}
		features = append(features, map[string]any{
			"type":     "Feature",
			"geometry": map[string]any{"type": "MultiPoint", "coordinates": points},
		})
	}
	if name != "" {
		p.env.SetSignal(name, map[string]any{"type": "FeatureCollection", "features": features})
	}
	return rows, nil
}

// geopoint projects the longitude and the latitude of each row to x and y.
// A point that the projection does not show gets no x and y.
func geopoint(p *params, rows []Row) ([]Row, error) {
	proj, err := p.projection()
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, fmt.Errorf("geopoint needs a projection: %w", ErrParam)
	}
	fields, err := p.fields("fields")
	if err != nil {
		return nil, err
	}
	if len(fields) < 2 || fields[0] == nil || fields[1] == nil {
		return nil, fmt.Errorf("geopoint needs two fields: %w", ErrParam)
	}
	as, err := p.as("x", "y")
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		x, y, ok := proj.Project(geo.Number(fields[0].Get(r)), geo.Number(fields[1].Get(r)))
		if ok {
			c[as[0]], c[as[1]] = x, y
		} else {
			delete(c, as[0])
			delete(c, as[1])
		}
		out[i] = c
	}
	return out, nil
}

// geopath writes the SVG path of the GeoJSON of each row: the row itself,
// or the value of field.
func geopath(p *params, rows []Row) ([]Row, error) {
	return geoPaths(p, rows, nil, "path")
}

// geoshape writes the SVG path of the GeoJSON of each item of a shape mark,
// its datum by default, for the mark to draw.
func geoshape(p *params, rows []Row) ([]Row, error) {
	return geoPaths(p, rows, FieldAccessor("datum"), "shape")
}

func geoPaths(p *params, rows []Row, def *Accessor, as string) ([]Row, error) {
	proj, err := p.projection()
	if err != nil {
		return nil, err
	}
	field, err := p.field("field")
	if err != nil {
		return nil, err
	}
	if field == nil {
		field = def
	}
	radius := 4.5
	if proj != nil {
		radius = proj.Radius
	}
	if radius, err = p.number("pointRadius", radius); err != nil {
		return nil, err
	}
	names, err := p.as(as)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := clone(r)
		var o any = r
		if field != nil {
			o = field.Get(r)
		}
		if path, ok := geo.Path(proj, o, radius); ok {
			c[names[0]] = path
		} else {
			c[names[0]] = nil
		}
		out[i] = c
	}
	return out, nil
}

// graticule makes one row: the lines of a grid of meridians and parallels,
// as a GeoJSON MultiLineString.
func graticule(p *params, _ []Row) ([]Row, error) {
	g := geo.NewGraticule()
	extent := func(name string) (*[2][2]float64, error) {
		v, ok, err := p.raw(name)
		if err != nil || !ok || v == nil {
			return nil, err
		}
		l, _ := v.([]any)
		if len(l) < 2 {
			return nil, fmt.Errorf("the %s %v: %w", name, v, ErrParam)
		}
		a, _ := l[0].([]any)
		b, _ := l[1].([]any)
		if len(a) < 2 || len(b) < 2 {
			return nil, fmt.Errorf("the %s %v: %w", name, v, ErrParam)
		}
		return &[2][2]float64{{expr.Number(a[0]), expr.Number(a[1])}, {expr.Number(b[0]), expr.Number(b[1])}}, nil
	}
	step := func(name string) (*[2]float64, error) {
		l, err := p.numbers(name)
		if err != nil || l == nil {
			return nil, err
		}
		if len(l) < 2 {
			return nil, fmt.Errorf("the %s %v: %w", name, l, ErrParam)
		}
		return &[2]float64{l[0], l[1]}, nil
	}
	for _, name := range []string{"extent", "extentMajor", "extentMinor"} {
		e, err := extent(name)
		if err != nil {
			return nil, err
		}
		if e == nil {
			continue
		}
		if name != "extentMinor" {
			g.MajorExtent = *e
		}
		if name != "extentMajor" {
			g.MinorExtent = *e
		}
	}
	for _, name := range []string{"step", "stepMajor", "stepMinor"} {
		s, err := step(name)
		if err != nil {
			return nil, err
		}
		if s == nil {
			continue
		}
		if name != "stepMinor" {
			g.MajorStep = *s
		}
		if name != "stepMajor" {
			g.MinorStep = *s
		}
	}
	precision, err := p.number("precision", g.Precision)
	if err != nil {
		return nil, err
	}
	g.Precision = precision
	return []Row{g.Lines()}, nil
}
