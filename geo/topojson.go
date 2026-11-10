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
	case lineString:
		return map[string]any{"type": typ, "coordinates": t.line(list(o["arcs"]))}
	case multiLineString:
		ls := list(o["arcs"])
		coords = make([]any, len(ls))
		for i, l := range ls {
			coords[i] = t.line(list(l))
		}
	case polygon:
		return map[string]any{"type": typ, "coordinates": t.polygon(list(o["arcs"]))}
	case multiPolygon:
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

// Mesh reads the object of a TopoJSON topology by name as a MultiLineString
// geometry of its arcs, each arc once, joined end to end into lines. A
// filter keeps the arcs between two geometries for "interior", the arcs of
// one geometry alone for "exterior", and every arc of the object for "".
// The geometries are those that hold arcs, inside any collection.
func Mesh(topo any, name, filter string) (map[string]any, error) {
	t, objects, err := readTopology(topo)
	if err != nil {
		return nil, err
	}
	o, ok := objects[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the object %q: %w", name, ErrTopoJSON)
	}
	var keep func(a, b int) bool
	switch filter {
	case "":
	case "interior":
		keep = func(a, b int) bool { return a != b }
	case "exterior":
		keep = func(a, b int) bool { return a == b }
	default:
		return nil, fmt.Errorf("the filter %q: %w", filter, ErrTopoJSON)
	}
	// uses holds, for each arc, its first reference, with the direction in
	// which the object first uses it, and the first and the last geometry
	// that use it.
	type use struct {
		ref         int
		first, last int
		seen        bool
	}
	uses := make([]use, len(t.arcs))
	geom := 0
	ref := func(v any) {
		i := int(Number(v))
		j := i
		if i < 0 {
			j = ^i
		}
		if j < 0 || j >= len(uses) {
			return
		}
		u := &uses[j]
		if !u.seen {
			*u = use{ref: i, first: geom, last: geom, seen: true}
			return
		}
		u.last = geom
	}
	var walk func(g map[string]any)
	walk = func(g map[string]any) {
		arcs := list(g["arcs"])
		switch g["type"] {
		case geometryCollection:
			for _, c := range list(g["geometries"]) {
				c, _ := c.(map[string]any)
				walk(c)
			}
			return
		case lineString:
			for _, a := range arcs {
				ref(a)
			}
		case multiLineString, polygon:
			for _, l := range arcs {
				for _, a := range list(l) {
					ref(a)
				}
			}
		case multiPolygon:
			for _, p := range arcs {
				for _, l := range list(p) {
					for _, a := range list(l) {
						ref(a)
					}
				}
			}
		default:
			return
		}
		geom++
	}
	walk(o)
	var refs []int
	for _, u := range uses {
		if u.seen && (keep == nil || keep(u.first, u.last)) {
			refs = append(refs, u.ref)
		}
	}
	lines := t.stitch(refs)
	coords := make([]any, len(lines))
	for i, l := range lines {
		arcs := make([]any, len(l))
		for k, r := range l {
			arcs[k] = float64(r)
		}
		coords[i] = t.line(arcs)
	}
	return map[string]any{"type": multiLineString, "coordinates": coords}, nil
}

// point is a point of a topology, as the key of a map.
type point [2]float64

// ends returns the first and the last point of an arc reference, in the
// direction of the reference.
func (t *topology) ends(i int) (point, point) {
	j := i
	if i < 0 {
		j = ^i
	}
	a := t.arcs[j]
	if len(a) == 0 {
		return point{}, point{}
	}
	first := list(a[0])
	start := point{Number(first[0]), Number(first[1])}
	end := start
	if t.quantized {
		// A quantized arc holds the first point and then the steps.
		for _, p := range a[1:] {
			q := list(p)
			end[0] += Number(q[0])
			end[1] += Number(q[1])
		}
	} else {
		last := list(a[len(a)-1])
		end = point{Number(last[0]), Number(last[1])}
	}
	if i < 0 {
		return end, start
	}
	return start, end
}

// fragments is a map of points to lines that keeps the order in which
// its keys first came, as an object of JavaScript does. A key set again
// keeps its place, and a key deleted and set again goes to the end.
type fragments struct {
	at   map[point]int
	keys []point
	vals []*[]int
	live []bool
}

func newFragments() *fragments {
	return &fragments{at: map[point]int{}}
}

func (m *fragments) get(k point) *[]int {
	if i, ok := m.at[k]; ok {
		return m.vals[i]
	}
	return nil
}

func (m *fragments) set(k point, f *[]int) {
	if i, ok := m.at[k]; ok {
		m.vals[i] = f
		return
	}
	m.at[k] = len(m.keys)
	m.keys, m.vals, m.live = append(m.keys, k), append(m.vals, f), append(m.live, true)
}

func (m *fragments) del(k point) {
	if i, ok := m.at[k]; ok {
		m.live[i] = false
		delete(m.at, k)
	}
}

// stitch joins arc references into lines, where the end of one is the
// start of the next, without reversing an arc, as topojson-client does.
// An empty arc, of two points at one place, comes first. A line that
// closes into a ring stays open to the arcs that come later. The lines
// come in the order of the keys of their ends, then of their starts, and
// an arc that no line holds at the end is a line of its own.
func (t *topology) stitch(refs []int) [][]int {
	refs = slices.Clone(refs)
	empty := -1
	for j, r := range refs {
		a := t.arcs[arcIndex(r)]
		if len(a) == 2 {
			if p := list(a[1]); len(p) >= 2 && Number(p[0]) == 0 && Number(p[1]) == 0 {
				empty++
				refs[empty], refs[j] = r, refs[empty]
			}
		}
	}
	type ends struct{ start, end point }
	at := map[*[]int]*ends{}
	byStart, byEnd := newFragments(), newFragments()
	for _, r := range refs {
		s, e := t.ends(r)
		if f := byEnd.get(s); f != nil {
			byEnd.del(at[f].end)
			*f = append(*f, r)
			at[f].end = e
			if g := byStart.get(e); g != nil {
				byStart.del(at[g].start)
				if g != f {
					*f = append(*f, *g...)
					at[f].end = at[g].end
				}
			}
			byStart.set(at[f].start, f)
			byEnd.set(at[f].end, f)
			continue
		}
		if f := byStart.get(e); f != nil {
			byStart.del(at[f].start)
			*f = append([]int{r}, *f...)
			at[f].start = s
			byStart.set(s, f)
			byEnd.set(at[f].end, f)
			continue
		}
		f := &[]int{r}
		at[f] = &ends{s, e}
		byStart.set(s, f)
		byEnd.set(e, f)
	}
	stitched := map[int]bool{}
	var out [][]int
	flush := func(from, other *fragments, key func(*ends) point) {
		for i, f := range from.vals {
			if !from.live[i] {
				continue
			}
			other.del(key(at[f]))
			for _, r := range *f {
				stitched[arcIndex(r)] = true
			}
			out = append(out, *f)
		}
	}
	flush(byEnd, byStart, func(e *ends) point { return e.start })
	flush(byStart, byEnd, func(e *ends) point { return e.end })
	for _, r := range refs {
		if !stitched[arcIndex(r)] {
			out = append(out, []int{r})
		}
	}
	return out
}

// arcIndex returns the index of the arc of a reference: ~i for a negative
// reference i.
func arcIndex(i int) int {
	if i < 0 {
		return ^i
	}
	return i
}
