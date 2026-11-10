package vegalite

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/zuzu/vega"
)

// projComp is the one projection of a spec, which the views that show
// geographic data share. Vega-Lite names it projection.
type projComp struct {
	params map[string]any
	// fitted says that the projection fits the data to the size of the
	// view, which it does unless the spec gives a scale or a translation.
	fitted bool
	// fit lists what the projection fits, in order: the signals of the
	// geojson transforms, and the datasets of the views that draw GeoJSON.
	fit  []fitRef
	size [2]string
}

// fitRef is a signal or a dataset that a projection fits.
type fitRef struct {
	signal, data string
}

// pushProjection makes the projection of a spec the projection of the views
// inside it, until the returned function runs.
func (c *compiler) pushProjection(raw jsontext.Value) func() {
	if len(raw) == 0 {
		return func() {}
	}
	c.projStack = append(c.projStack, raw)
	n := len(c.projStack)
	return func() { c.projStack = c.projStack[:n-1] }
}

// projection returns the projection of the view being compiled, made when
// the first view needs it.
func (c *compiler) projection(u *unit) (*projComp, error) {
	params := map[string]any{}
	if n := len(c.projStack); n > 0 {
		if err := json.Unmarshal(c.projStack[n-1], &params); err != nil {
			return nil, fmt.Errorf("reading the projection: %w", err)
		}
	}
	if c.proj != nil {
		if !sameJSON(c.proj.params, params) {
			if err := c.issue(u.path, "views with different projections"); err != nil {
				return nil, err
			}
		}
		return c.proj, nil
	}
	_, scaled := params["scale"]
	_, moved := params["translate"]
	size := [2]string{"width", "height"}
	if u.v.sizeNames[0] != "" {
		size = u.v.sizeNames
	}
	c.proj = &projComp{params: params, fitted: !scaled && !moved, size: size}
	return c.proj, nil
}

func sameJSON(a, b any) bool {
	x, err1 := json.Marshal(a, json.Deterministic(true))
	y, err2 := json.Marshal(b, json.Deterministic(true))
	return err1 == nil && err2 == nil && string(x) == string(y)
}

// finishProjection adds the projection to the spec.
func (c *compiler) finishProjection(out *vega.Spec) {
	p := c.proj
	if p == nil {
		return
	}
	vp := vega.Projection{Name: "projection", Params: vega.Extra{}}
	if t, ok := p.params["type"]; ok {
		vp.Type = vega.Lit(t)
	}
	for k, v := range p.params {
		if k != "type" {
			vp.Params[k] = mustJSON(v)
		}
	}
	if p.fitted {
		vp.Params["size"] = mustJSON(map[string]any{"signal": "[" + p.size[0] + ", " + p.size[1] + "]"})
		var parts []string
		for _, f := range p.fit {
			if f.signal != "" {
				parts = append(parts, f.signal)
			} else {
				parts = append(parts, "data('"+f.data+"')")
			}
		}
		sig := strings.Join(parts, ", ")
		if len(parts) != 1 {
			sig = "[" + sig + "]"
		}
		vp.Params["fit"] = mustJSON(map[string]any{"signal": sig})
	}
	out.Projections = append(out.Projections, vp)
}

// geoPairs are the pairs of channels of longitude and latitude, and the
// channels of the place that each pair gives.
var geoPairs = [][4]string{
	{"longitude", "latitude", "x", "y"},
	{"longitude2", "latitude2", "x2", "y2"},
}

// readGeo moves the geographic channels of a view out of its channels:
// longitude and latitude, which a projection places, and a shape of type
// geojson, which a geoshape mark draws.
func (u *unit) readGeo() {
	u.geo = map[string]*channelDef{}
	for _, p := range geoPairs {
		for _, name := range p[:2] {
			if d, ok := u.ch[name]; ok {
				u.geo[name] = d
				delete(u.ch, name)
			}
		}
	}
	if d, ok := u.ch["shape"]; ok && d.typ == "geojson" {
		u.geoShape = d
		delete(u.ch, "shape")
	}
}

// usesGeo reports whether the view needs a projection.
func (u *unit) usesGeo() bool {
	return u.mark == "geoshape" || len(u.geo) > 0 || u.geoShape != nil
}

// geoField returns the field of a geographic channel for a transform: its
// name, or an expression for a constant.
func geoField(d *channelDef) any {
	if d == nil {
		return nil
	}
	if d.hasDatum {
		b, err := json.Marshal(d.datum)
		if err != nil {
			return nil
		}
		return map[string]any{"expr": string(b)}
	}
	return d.field
}

// addGeo adds the transforms that place the geographic data of a view: the
// geojson transforms that collect what the projection fits, then a geopoint
// transform for each pair of longitude and latitude.
func (u *unit) addGeo(n *dataNode) error {
	p := u.proj
	if p == nil {
		return nil
	}
	c := u.c
	k := 0
	signal := func() string {
		s := u.v.prefix + "geojson_" + strconv.Itoa(k)
		k++
		p.fit = append(p.fit, fitRef{signal: s})
		return s
	}
	type pair struct {
		lon, lat any
		as       [2]string
	}
	var pairs []pair
	for _, gp := range geoPairs {
		lon, lat := u.geo[gp[0]], u.geo[gp[1]]
		if lon == nil && lat == nil {
			continue
		}
		pairs = append(pairs, pair{geoField(lon), geoField(lat), [2]string{u.v.prefix + gp[2], u.v.prefix + gp[3]}})
		if p.fitted {
			c.add(n, transform("geojson", map[string]any{"fields": []any{geoField(lon), geoField(lat)}, "signal": signal()}))
		}
	}
	if d := u.geoShape; d != nil {
		c.add(n, transform("filter", map[string]any{"expr": "isValid(" + fieldRef(d.out) + ")"}))
		if p.fitted {
			c.add(n, transform("geojson", map[string]any{"geojson": d.field, "signal": signal()}))
		}
	}
	for _, pr := range pairs {
		c.add(n, transform("geopoint", map[string]any{"projection": "projection", "fields": []any{pr.lon, pr.lat}, "as": []any{pr.as[0], pr.as[1]}}))
		u.geoAs = append(u.geoAs, pr.as[0], pr.as[1])
	}
	return nil
}

// fitData adds the data of a view that draws GeoJSON to what the
// projection fits, after the transforms of the view.
func (u *unit) fitData() {
	if u.proj == nil || !u.proj.fitted || u.mark != "geoshape" || u.geoShape != nil || len(u.geo) > 0 {
		return
	}
	u.proj.fit = append(u.proj.fit, fitRef{data: u.c.name(u.data)})
}

// geoEncode sets the place of a mark from the fields that geopoint makes,
// and the transform of a geoshape mark.
func (u *unit) geoEncode(m *vega.Mark) {
	if u.proj == nil {
		return
	}
	upd := m.Encode["update"]
	for i := 0; i+1 < len(u.geoAs); i += 2 {
		x, y := "x", "y"
		if i > 0 {
			x, y = "x2", "y2"
		}
		upd[x] = prod(map[string]any{"field": u.geoAs[i]})
		upd[y] = prod(map[string]any{"field": u.geoAs[i+1]})
	}
	if !u.proj.fitted {
		m.Clip = lit(true)
	}
	if u.mark != "geoshape" {
		return
	}
	params := vega.Extra{"projection": mustJSON("projection")}
	if d := u.geoShape; d != nil {
		params["field"] = mustJSON(fieldRef(d.out))
	}
	m.Transform = append(m.Transform, vega.Transform{Type: "geoshape", Params: params})
}
