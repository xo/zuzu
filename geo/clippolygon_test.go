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

// ringArea returns the area on the unit sphere that a ring holds on its
// right, as a fan of triangles from its first point, each with the solid
// angle of Van Oosterom and Strackee.
func ringArea(ring [][2]float64) float64 {
	if len(ring) < 3 {
		return 0
	}
	v0 := cartesian(ring[0][0], ring[0][1])
	sum := 0.0
	for i := 1; i+1 < len(ring); i++ {
		a := cartesian(ring[i][0], ring[i][1])
		b := cartesian(ring[i+1][0], ring[i+1][1])
		sum += 2 * math.Atan2(v0.dot(a.cross(b)), 1+v0.dot(a)+a.dot(b)+b.dot(v0))
	}
	area := -sum
	if area < 0 {
		area += 2 * tau
	}
	return area
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
	// The lobes cover the sphere but for the strips along the cuts, and
	// a point on a cut belongs to the lobe that the raw function picks.
	p := mustNew(t, "interruptedMollweide")
	if a := clipPolygon(p.preclip, nil); math.Abs(a-2*tau) > 1e-4 {
		t.Errorf("area %v, want about %v", a, 2*tau)
	}
	for _, test := range []struct {
		lon, lat float64
		want     bool
	}{
		{-40, 10, true}, {-40 + 1e-6*degrees, 10, false}, {-40, -10, true},
		{-20, -10, true}, {-20 + 1e-6*degrees, -10, false}, {-20 + 2e-6*degrees, -10, true},
		{180, 10, false}, {-180, 10, false}, {179.99, 10, true},
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
	if !b.preclip.visible(halfPi, -0.5) || b.preclip.visible(halfPi-1e-6, -0.5) || !b.preclip.visible(halfPi-1e-6, 0.5) {
		t.Error("butterfly: the cut at 90°")
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
	// A latitude of a lobe other than 0 or ±90 changes nothing, and the
	// report says so.
	err = p.Set("lobes", []any{
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, 80.0}, []any{180.0, 0.0}}},
		[]any{[]any{[]any{-180.0, 0.0}, []any{0.0, -90.0}, []any{180.0, 0.0}}},
	})
	if !errors.Is(err, ErrParamUnread) {
		t.Errorf("a latitude of 80 gives %v, want ErrParamUnread", err)
	}
	if err := mustNew(t, "mercator").Set("lobes", []any{1.0}); err != nil {
		t.Errorf("a projection without lobes: %v", err)
	}
}
