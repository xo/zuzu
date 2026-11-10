package geo

import (
	"errors"
	"fmt"
	"slices"
)

// ErrTopoJSON is the error of a TopoJSON topology that zuzu cannot read.
var ErrTopoJSON = errors.New("bad TopoJSON")

// topology is a TopoJSON topology: the arcs that the objects share, with
// the transform that decodes their quantized positions.
type topology struct {
	arcs      [][]any
	quantized bool
	kx, ky    float64
	dx, dy    float64
}

func readTopology(v any) (*topology, map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok || m["type"] != "Topology" {
		return nil, nil, fmt.Errorf("not a topology: %w", ErrTopoJSON)
	}
	objects, _ := m["objects"].(map[string]any)
	t := &topology{}
	for _, a := range list(m["arcs"]) {
		t.arcs = append(t.arcs, list(a))
	}
	if tr, ok := m["transform"].(map[string]any); ok {
		s, tl := list(tr["scale"]), list(tr["translate"])
		if len(s) < 2 || len(tl) < 2 {
			return nil, nil, fmt.Errorf("the transform: %w", ErrTopoJSON)
		}
		t.quantized = true
		t.kx, t.ky = Number(s[0]), Number(s[1])
		t.dx, t.dy = Number(tl[0]), Number(tl[1])
	}
	return t, objects, nil
}

// Feature reads the object of a TopoJSON topology by name, as GeoJSON
// features. A collection gives a feature for each of its geometries.
func Feature(topo any, name string) ([]any, error) {
	t, objects, err := readTopology(topo)
	if err != nil {
		return nil, err
	}
	o, ok := objects[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the object %q: %w", name, ErrTopoJSON)
	}
	if o["type"] == geometryCollection {
		gs := list(o["geometries"])
		out := make([]any, len(gs))
		for i, g := range gs {
			g, _ := g.(map[string]any)
			out[i] = t.feature(g)
		}
		return out, nil
	}
	return []any{t.feature(o)}, nil
}

// feature returns a geometry of a topology as a GeoJSON feature, with its
// id and its properties.
func (t *topology) feature(o map[string]any) map[string]any {
	f := map[string]any{"type": "Feature", "geometry": t.geometry(o)}
	props, ok := o["properties"]
	if !ok || props == nil {
		props = map[string]any{}
	}
	f["properties"] = props
	if id, ok := o["id"]; ok && id != nil {
		f["id"] = id
	}
	if b, ok := o["bbox"]; ok && b != nil {
		f["bbox"] = b
	}
	return f
}

// position decodes a position. A quantized position of an arc adds to the
// one before it, from 0 at the start of the arc.
func (t *topology) position(p []any, x0, y0 *float64) []any {
	out := slices.Clone(p)
	if !t.quantized || len(p) < 2 {
		return out
	}
	*x0 += Number(p[0])
	*y0 += Number(p[1])
	out[0] = *x0*t.kx + t.dx
	out[1] = *y0*t.ky + t.dy
	return out
}

// arc adds the points of an arc to a line. A negative index ~i is arc i
// reversed. The first point of the arc repeats the last of the line, so it
// replaces it.
func (t *topology) arc(i int, points []any) []any {
	if len(points) > 0 {
		points = points[:len(points)-1]
	}
	j := i
	if i < 0 {
		j = ^i
	}
	if j >= len(t.arcs) {
		return points
	}
	a := t.arcs[j]
	start := len(points)
	var x0, y0 float64
	for _, p := range a {
		points = append(points, t.position(list(p), &x0, &y0))
	}
	if i < 0 {
		slices.Reverse(points[start:])
	}
	return points
}

func (t *topology) line(arcs []any) []any {
	var points []any
	for _, a := range arcs {
		points = t.arc(int(Number(a)), points)
	}
	if len(points) == 1 {
		points = append(points, points[0])
	}
	if points == nil {
		points = []any{}
	}
	return points
}

func (t *topology) ring(arcs []any) []any {
	points := t.line(arcs)
	for len(points) > 0 && len(points) < 4 {
		points = append(points, points[0])
	}
	return points
}

func (t *topology) polygon(arcs []any) []any {
	out := make([]any, len(arcs))
	for i, r := range arcs {
		out[i] = t.ring(list(r))
	}
	return out
}

// geometry decodes a geometry of a topology as GeoJSON. A geometry with no
// type gives nil.
func (t *topology) geometry(o map[string]any) any {
	if o == nil {
		return nil
	}
	typ, _ := o["type"].(string)
	var coords []any
	switch typ {
	case geometryCollection:
		gs := list(o["geometries"])
		out := make([]any, len(gs))
		for i, g := range gs {
			g, _ := g.(map[string]any)
			out[i] = t.geometry(g)
		}
		return map[string]any{"type": typ, "geometries": out}
	case "Point":
		var x0, y0 float64
		return map[string]any{"type": typ, "coordinates": t.position(list(o["coordinates"]), &x0, &y0)}
	case "MultiPoint":
		ps := list(o["coordinates"])
		coords = make([]any, len(ps))
		for i, p := range ps {
			var x0, y0 float64
			coords[i] = t.position(list(p), &x0, &y0)
		}
	case "LineString":
		return map[string]any{"type": typ, "coordinates": t.line(list(o["arcs"]))}
	case multiLineString:
		ls := list(o["arcs"])
		coords = make([]any, len(ls))
		for i, l := range ls {
			coords[i] = t.line(list(l))
		}
	case "Polygon":
		return map[string]any{"type": typ, "coordinates": t.polygon(list(o["arcs"]))}
	case "MultiPolygon":
		ps := list(o["arcs"])
		coords = make([]any, len(ps))
		for i, p := range ps {
			coords[i] = t.polygon(list(p))
		}
	default:
		return nil
	}
	return map[string]any{"type": typ, "coordinates": coords}
}
