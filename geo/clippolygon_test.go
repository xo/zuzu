package geo

import (
	"errors"
	"math"
	"testing"
)

// ringCollector keeps the lines that a clip sends, in radians.
type ringCollector struct {
	lines [][][2]float64
}

func (r *ringCollector) point(x, y float64) {
	n := len(r.lines) - 1
	r.lines[n] = append(r.lines[n], [2]float64{x, y})
}
func (r *ringCollector) lineStart()    { r.lines = append(r.lines, nil) }
func (r *ringCollector) lineEnd()      {}
func (r *ringCollector) polygonStart() {}
func (r *ringCollector) polygonEnd()   {}
func (r *ringCollector) sphere()       {}

// degreeRing returns a ring in degrees as radians.
func degreeRing(points ...[2]float64) [][2]float64 {
	out := make([][2]float64, len(points))
	for i, p := range points {
		out[i] = [2]float64{p[0] * radians, p[1] * radians}
	}
	return out
}

// ringArea returns the area on the unit sphere that a ring in radians
// holds on its right, as SphereArea measures it.
func ringArea(ring [][2]float64) float64 {
	if len(ring) < 3 {
		return 0
	}
	coords := make([]any, 0, len(ring)+1)
	for _, p := range ring {
		coords = append(coords, []any{p[0] * degrees, p[1] * degrees})
	}
	coords = append(coords, coords[0])
	return SphereArea(map[string]any{"type": "Polygon", "coordinates": []any{coords}})
}

// clipPolygon clips a polygon of one ring, or the sphere if the ring is
// nil, and returns the area of the result.
func clipPolygon(c *clipper, ring [][2]float64) float64 {
	r := &ringCollector{}
	s := c.stream(r)
	if ring == nil {
		s.sphere()
	} else {
		s.polygonStart()
		s.lineStart()
		for _, p := range ring {
			s.point(p[0], p[1])
		}
		s.lineEnd()
		s.polygonEnd()
	}
	area := 0.0
	for _, l := range r.lines {
		area += ringArea(l)
	}
	return area
}

// clipLine clips a line and returns its pieces, in degrees.
func clipLine(c *clipper, line [][2]float64) [][][2]float64 {
	r := &ringCollector{}
	s := c.stream(r)
	s.lineStart()
	for _, p := range line {
		s.point(p[0], p[1])
	}
	s.lineEnd()
	for _, l := range r.lines {
		for i := range l {
			l[i] = [2]float64{l[i][0] * degrees, l[i][1] * degrees}
		}
	}
	return r.lines
}

func nearAngle(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// The north hemisphere, and a polygon of the north above 45° whose edges
// are great arcs between four points 90° apart.
var (
	northRing = degreeRing([2]float64{0, 0}, [2]float64{-90, 0}, [2]float64{180, 0}, [2]float64{90, 0})
	capRing   = degreeRing([2]float64{0, 45}, [2]float64{-90, 45}, [2]float64{180, 45}, [2]float64{90, 45})
)

func TestClipPolygonLune(t *testing.T) {
	t.Parallel()
	// The lune between the meridians 0° and 60°: north along 0°, then
	// south along 60°.
	lune := degreeRing([2]float64{0, 0}, [2]float64{0, 90}, [2]float64{60, 0}, [2]float64{0, -90})
	c := newPolygonClip(lune)
	for _, test := range [][2]float64{{30, 10}, {30, -80}, {59, 0}, {90, 10}, {-10, 0}, {240, 0}} {
		want := test[0] > 0 && test[0] < 60
		if got := c.visible(test[0]*radians, test[1]*radians); got != want {
			t.Errorf("visible(%v, %v) = %v", test[0], test[1], got)
		}
	}
	if a := clipPolygon(c, nil); !nearAngle(a, 2*math.Pi/3) {
		t.Errorf("sphere: area %v, want %v", a, 2*math.Pi/3)
	}
	if a := clipPolygon(c, northRing); !nearAngle(a, math.Pi/3) {
		t.Errorf("north: area %v, want %v", a, math.Pi/3)
	}
	// A polygon inside the lune keeps its area, and one outside is gone.
	in := degreeRing([2]float64{10, -10}, [2]float64{10, 10}, [2]float64{50, 10}, [2]float64{50, -10})
	if a, want := clipPolygon(c, in), ringArea(in); !nearAngle(a, want) {
		t.Errorf("inside: area %v, want %v", a, want)
	}
	out := degreeRing([2]float64{100, -10}, [2]float64{100, 10}, [2]float64{150, 10}, [2]float64{150, -10})
	if a := clipPolygon(c, out); a != 0 {
		t.Errorf("outside: area %v, want 0", a)
	}
	// A wider lune holds the whole clip.
	wide := degreeRing([2]float64{-30, 0}, [2]float64{-30, 90}, [2]float64{150, 0}, [2]float64{-30, -90})
	if a := clipPolygon(c, wide); !nearAngle(a, 2*math.Pi/3) {
		t.Errorf("wide: area %v, want %v", a, 2*math.Pi/3)
	}
	// The equator from -30° to 90° keeps the part from 0° to 60°.
	pieces := clipLine(c, degreeRing([2]float64{-30, 0}, [2]float64{90, 0}))
	if len(pieces) != 1 || len(pieces[0]) != 2 || !nearAngle(pieces[0][0][0], 0) || !nearAngle(pieces[0][1][0], 60) {
		t.Errorf("equator: %v", pieces)
	}
}

func TestClipPolygonTriangle(t *testing.T) {
	t.Parallel()
	// The octant from 0° to 90° in the north, a face of the octahedron.
	tri := degreeRing([2]float64{0, 0}, [2]float64{0, 90}, [2]float64{90, 0})
	c := newPolygonClip(tri)
	if a := clipPolygon(c, nil); !nearAngle(a, math.Pi/2) {
		t.Errorf("sphere: area %v, want %v", a, math.Pi/2)
	}
	// The cap turns into itself by 90°, so the octant holds a quarter.
	if a, want := clipPolygon(c, capRing), ringArea(capRing)/4; !nearAngle(a, want) {
		t.Errorf("cap: area %v, want %v", a, want)
	}
	// A line from inside to outside and back is cut twice.
	pieces := clipLine(c, degreeRing([2]float64{45, 30}, [2]float64{120, 30}, [2]float64{120, 10}, [2]float64{45, 10}))
	if len(pieces) != 2 || !nearAngle(pieces[0][len(pieces[0])-1][0], 90) || !nearAngle(pieces[1][0][0], 90) {
		t.Errorf("line: %v", pieces)
	}
}

func TestClipPolygonAntimeridian(t *testing.T) {
	t.Parallel()
	// The lune from 150° to -150°, across the antimeridian.
	lune := degreeRing([2]float64{150, 0}, [2]float64{150, 90}, [2]float64{-150, 0}, [2]float64{150, -90})
	c := newPolygonClip(lune)
	if !c.visible(math.Pi, 0) || !c.visible(-170*radians, 20*radians) || c.visible(0, 0) {
		t.Error("visible: wrong side of the antimeridian")
	}
	if a := clipPolygon(c, nil); !nearAngle(a, 2*math.Pi/3) {
		t.Errorf("sphere: area %v, want %v", a, 2*math.Pi/3)
	}
	if a := clipPolygon(c, northRing); !nearAngle(a, math.Pi/3) {
		t.Errorf("north: area %v, want %v", a, math.Pi/3)
	}
	// A polygon across the antimeridian keeps the part inside the lune.
	// Its top edge is a great arc, which crosses the meridians of the lune
	// at the latitude lat.
	box := degreeRing([2]float64{120, 0}, [2]float64{120, 60}, [2]float64{-120, 60}, [2]float64{-120, 0})
	n := cartesian(box[1][0], box[1][1]).cross(cartesian(box[2][0], box[2][1]))
	l := 150 * radians
	lat := math.Atan(-(n[0]*math.Cos(l) + n[1]*math.Sin(l)) / n[2])
	part := [][2]float64{{l, 0}, {l, lat}, {-l, lat}, {-l, 0}}
	if a, want := clipPolygon(c, box), ringArea(part); !nearAngle(a, want) {
		t.Errorf("box: area %v, want %v", a, want)
	}
	pieces := clipLine(c, degreeRing([2]float64{120, 0}, [2]float64{-120, 0}))
	if len(pieces) != 1 || !nearAngle(pieces[0][0][0], 150) || !nearAngle(pieces[0][1][0], -150) {
		t.Errorf("equator: %v", pieces)
	}
}

func TestClipPolygonPole(t *testing.T) {
	t.Parallel()
	// The cap holds the north pole.
	c := newPolygonClip(capRing)
	if !c.visible(0, halfPi) || !c.visible(2, 80*radians) || c.visible(0, 0) || c.visible(1, -halfPi) {
		t.Error("visible: the cap")
	}
	capArea := ringArea(capRing)
	if a := clipPolygon(c, nil); !nearAngle(a, capArea) {
		t.Errorf("sphere: area %v, want %v", a, capArea)
	}
	// The hemisphere holds the whole cap, with no crossing.
	if a := clipPolygon(c, northRing); !nearAngle(a, capArea) {
		t.Errorf("north: area %v, want %v", a, capArea)
	}
	// A polygon inside the cap keeps its area.
	in := degreeRing([2]float64{0, 70}, [2]float64{-120, 70}, [2]float64{120, 70})
	if a, want := clipPolygon(c, in), ringArea(in); !nearAngle(a, want) {
		t.Errorf("inside: area %v, want %v", a, want)
	}
	// Each lune of 90° holds a quarter of the cap, which turns into itself
	// by 90°.
	lune := degreeRing([2]float64{10, 0}, [2]float64{10, 90}, [2]float64{100, 0}, [2]float64{10, -90})
	if a, want := clipPolygon(c, lune), capArea/4; !nearAngle(a, want) {
		t.Errorf("lune: area %v, want %v", a, want)
	}
	// A meridian enters the cap, passes over the pole and leaves it.
	pieces := clipLine(c, degreeRing([2]float64{10, 0}, [2]float64{10, 80}, [2]float64{-170, 80}, [2]float64{-170, 0}))
	if len(pieces) != 1 || len(pieces[0]) != 4 {
		t.Errorf("meridian: %v", pieces)
	}
}

func TestLobeRing(t *testing.T) {
	t.Parallel()
	// The lobes cover the sphere but for the strips along the cuts.
	const e = 2e-6 * degrees
	p := mustNew(t, "interruptedMollweide")
	if a := clipPolygon(p.preclip, nil); math.Abs(a-2*tau) > 1e-4 {
		t.Errorf("area %v, want about %v", a, 2*tau)
	}
	for _, test := range []struct {
		lon, lat float64
		want     bool
	}{
		{-40, 10, false}, {-40 - e, 10, true}, {-40 + e, 10, true}, {-40, -10, true},
		{-20, -10, false}, {-20 - e, -10, true}, {-20 + e, -10, true},
		{180, 10, false}, {-180, 10, false}, {-180 + e, 10, true}, {180 - e, 10, true},
		{0, 90, false}, {0, 89.99, true},
	} {
		if got := p.preclip.visible(test.lon*radians, test.lat*radians); got != test.want {
			t.Errorf("visible(%v, %v) = %v", test.lon, test.lat, got)
		}
	}
	b := mustNew(t, "polyhedralButterfly")
	if a := clipPolygon(b.preclip, nil); math.Abs(a-2*tau) > 1e-4 {
		t.Errorf("butterfly: area %v, want about %v", a, 2*tau)
	}
	// A point on a cut, or in the strip along it, moves into the lobe that
	// the raw function gives it, as in d3: the west lobe of an interrupted
	// projection and the east face of the butterfly. A point at ±180°
	// stays on its side.
	for _, test := range []struct {
		p        *Projection
		lon, lat float64
		want     float64
	}{
		{p, -40, 10, -40 - e}, {p, -40 + e/4, 10, -40 + e}, {p, -20, -10, -20 - e},
		{p, -180, 10, -180 + e}, {p, 180, 10, 180 - e}, {p, 30, 10, 30},
		{b, 90, -30, 90 + e}, {b, 90 - e/4, -30, 90 - e}, {b, 90, 30, 90},
		{b, -180, -30, -180 + e}, {b, 180, 30, 180 - e},
	} {
		lambda, phi := test.p.preclip.move(test.lon*radians, test.lat*radians)
		if math.Abs(lambda*degrees-test.want) > 1e-12 || phi != test.lat*radians {
			t.Errorf("%s: move(%v, %v) = %v, %v, want %v", test.p.Type(), test.lon, test.lat, lambda*degrees, phi*degrees, test.want)
		}
		if !test.p.preclip.visible(lambda, phi) {
			t.Errorf("%s: move(%v, %v) is not visible", test.p.Type(), test.lon, test.lat)
		}
	}
}

func TestLobes(t *testing.T) {
	t.Parallel()
	p := mustNew(t, "interruptedSinusoidal")
	if err := p.Set("rotate", []any{0.0, 0.0}); err != nil {
		t.Fatal(err)
	}
	// One lobe in each half draws the sinusoidal projection.
	err := p.Set("lobes", []any{
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, 90.0}, []any{180.0, 0.0}}},
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, -90.0}, []any{180.0, 0.0}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	x, y, _ := p.Project(90, 60)
	if !near(x, 480+152.63*halfPi/2) || !near(y, 250-152.63*math.Pi/3) {
		t.Errorf("project = %v, %v", x, y)
	}
	if err := p.Set("lobes", []any{1.0}); err == nil {
		t.Error("bad lobes give no error")
	}
	// The latitudes of the lobes are read. TestLobeLatitudes compares
	// them with d3.
	err = p.Set("lobes", []any{
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, 80.0}, []any{180.0, 0.0}}},
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, -90.0}, []any{180.0, 0.0}}},
	})
	if err != nil {
		t.Errorf("a latitude of 80: %v", err)
	}
	if err := p.Set("lobes", []any{
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, math.NaN()}, []any{180.0, 0.0}}},
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, -90.0}, []any{180.0, 0.0}}},
	}); !errors.Is(err, ErrProjection) {
		t.Errorf("a latitude of NaN gives %v, want ErrProjection", err)
	}
	if err := mustNew(t, "mercator").Set("lobes", []any{1.0}); err != nil {
		t.Errorf("a projection without lobes: %v", err)
	}
}
