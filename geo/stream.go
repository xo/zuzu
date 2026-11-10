package geo

// The types of GeoJSON that more than one file names.
const (
	geometryCollection = "GeometryCollection"
	multiLineString    = "MultiLineString"
	lineString         = "LineString"
	polygon            = "Polygon"
	multiPolygon       = "MultiPolygon"
)

// stream receives a geometry as a sequence of events: the points, the
// start and the end of each line, the start and the end of each polygon,
// and the whole sphere. Each stage of a projection is a stream that passes
// the events on to the next.
type stream interface {
	point(x, y float64)
	lineStart()
	lineEnd()
	polygonStart()
	polygonEnd()
	sphere()
}

// flagged is a stream that keeps a flag with a point, which a clip uses to
// mark the points where a line crosses its edge.
type flagged interface {
	pointFlag(x, y float64, m int)
}

// pointFlag sends a point with a flag to a stream, and drops the flag if
// the stream does not keep it.
func pointFlag(s stream, x, y float64, m int) {
	if f, ok := s.(flagged); ok {
		f.pointFlag(x, y, m)
		return
	}
	s.point(x, y)
}

// streamObject sends a GeoJSON object to a stream, with its coordinates in
// degrees. A ring of a polygon leaves out its last point, which repeats the
// first.
func streamObject(o any, s stream) {
	m, ok := o.(map[string]any)
	if !ok {
		return
	}
	switch m["type"] {
	case "Feature":
		streamObject(m["geometry"], s)
	case "FeatureCollection":
		fs, _ := m["features"].([]any)
		for _, f := range fs {
			if f, ok := f.(map[string]any); ok {
				streamObject(f["geometry"], s)
			}
		}
	case geometryCollection:
		gs, _ := m["geometries"].([]any)
		for _, g := range gs {
			streamObject(g, s)
		}
	case "Sphere":
		s.sphere()
	case "Point":
		if x, y, ok := position(m["coordinates"]); ok {
			s.point(x, y)
		}
	case "MultiPoint":
		for _, p := range list(m["coordinates"]) {
			if x, y, ok := position(p); ok {
				s.point(x, y)
			}
		}
	case lineString:
		streamLine(list(m["coordinates"]), s, false)
	case multiLineString:
		for _, l := range list(m["coordinates"]) {
			streamLine(list(l), s, false)
		}
	case polygon:
		streamPolygon(list(m["coordinates"]), s)
	case multiPolygon:
		for _, p := range list(m["coordinates"]) {
			streamPolygon(list(p), s)
		}
	}
}

// list reads a value as a list.
func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// position reads a position of GeoJSON.
func position(v any) (float64, float64, bool) {
	l, ok := v.([]any)
	if !ok || len(l) < 2 {
		return 0, 0, false
	}
	return Number(l[0]), Number(l[1]), true
}

// streamLine sends a line, or a ring without its last point.
func streamLine(coords []any, s stream, closed bool) {
	n := len(coords)
	if closed {
		n--
	}
	s.lineStart()
	for i := range n {
		if x, y, ok := position(coords[i]); ok {
			s.point(x, y)
		}
	}
	s.lineEnd()
}

// streamPolygon sends a polygon, ring by ring.
func streamPolygon(rings []any, s stream) {
	s.polygonStart()
	for _, r := range rings {
		streamLine(list(r), s, true)
	}
	s.polygonEnd()
}

// pass is a stream that passes each event to the next stream. A stage
// embeds it and changes only the events it needs.
type pass struct {
	next stream
}

func (p *pass) point(x, y float64) { p.next.point(x, y) }
func (p *pass) lineStart()         { p.next.lineStart() }
func (p *pass) lineEnd()           { p.next.lineEnd() }
func (p *pass) polygonStart()      { p.next.polygonStart() }
func (p *pass) polygonEnd()        { p.next.polygonEnd() }
func (p *pass) sphere()            { p.next.sphere() }

// pointMap is a stage that maps each point with a function.
type pointMap struct {
	pass

	f func(x, y float64) (float64, float64)
}

func (p *pointMap) point(x, y float64) { p.next.point(p.f(x, y)) }

// multiplex sends each event to several streams.
type multiplex []stream

func (m multiplex) point(x, y float64) {
	for _, s := range m {
		s.point(x, y)
	}
}

func (m multiplex) lineStart() {
	for _, s := range m {
		s.lineStart()
	}
}

func (m multiplex) lineEnd() {
	for _, s := range m {
		s.lineEnd()
	}
}

func (m multiplex) polygonStart() {
	for _, s := range m {
		s.polygonStart()
	}
}

func (m multiplex) polygonEnd() {
	for _, s := range m {
		s.polygonEnd()
	}
}

func (m multiplex) sphere() {
	for _, s := range m {
		s.sphere()
	}
}
