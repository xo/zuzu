package geo

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"os"
	"testing"
)

// tier2Case is a case of testdata/projections-tier2.json, which
// test/oracle/projections-tier2.mjs writes with d3-geo-projection 4.0.0.
type tier2Case struct {
	Type      string         `json:"type"`
	Params    map[string]any `json:"params"`
	Points    []tier2Point   `json:"points"`
	Sphere    tier2Box       `json:"sphere"`
	Countries []tier2Box     `json:"countries"`
}

// tier2Point is [lon, lat, x, y], with x and y null where d3 gives no
// point. A point where the map is not smooth adds the [x, y] of 8 points
// around it, 1e-5° away.
type tier2Point struct {
	Lon, Lat float64
	X, Y     *float64
	Around   [][2]float64
}

func (p *tier2Point) UnmarshalJSON(b []byte) error {
	var l []any
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	if len(l) < 4 {
		return fmt.Errorf("the point %s: %w", b, ErrProjection)
	}
	p.Lon, p.Lat = Number(l[0]), Number(l[1])
	if l[2] != nil {
		x, y := Number(l[2]), Number(l[3])
		p.X, p.Y = &x, &y
	}
	if len(l) > 4 {
		for _, a := range list(l[4]) {
			if a := list(a); len(a) == 2 {
				p.Around = append(p.Around, [2]float64{Number(a[0]), Number(a[1])})
			}
		}
	}
	return nil
}

// tolerance returns how far, in pixels, the point of zuzu can be from the
// point of d3, and whether it can instead be near one of the points
// around it.
//
// A smooth point must be within 1e-6. At a corner of the square of a
// hemisphere, the map of Peirce and Guyou goes as a square root, so a
// rounding error ε of the input moves the point by about √ε: the
// tolerance grows with how far the points around move. On a cut, such as
// the meridian where two lobes or two squares part, zuzu and d3 can take
// different sides by a rounding error. There the point of zuzu must be
// within 0.2 of the point of d3 or of one of the points around it.
func (p tier2Point) tolerance() (tol float64, cut bool) {
	spread := 0.0
	for _, a := range p.Around {
		spread = max(spread, math.Hypot(a[0]-*p.X, a[1]-*p.Y))
	}
	if spread > 1 {
		return 0.2, true
	}
	return 1e-6 + 1e-3*spread, false
}

type tier2Box struct {
	Area   float64       `json:"area"`
	Bounds [2][2]float64 `json:"bounds"`
}

func (c tier2Case) name() string {
	return fmt.Sprintf("%s%v", c.Type, c.Params)
}

// tier2Projection makes the projection of a case.
func tier2Projection(t *testing.T, c tier2Case) *Projection {
	t.Helper()
	p := mustNew(t, c.Type)
	if r, ok := c.Params["rotate"]; ok {
		if err := p.Set("rotate", r); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok := c.Params["parallel"]; ok {
		if err := p.setArmadilloParallel(v); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok := c.Params["lobes"]; ok {
		if err := p.setBerghausLobes(v); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func readTier2(t *testing.T) []tier2Case {
	t.Helper()
	b, err := os.ReadFile("testdata/projections-tier2.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []tier2Case
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// TestTier2Points projects the points of each case and compares them with
// d3-geo-projection, within 1e-6 of a pixel where the map is smooth. The
// method tolerance says where the test is looser.
func TestTier2Points(t *testing.T) {
	t.Parallel()
	for _, c := range readTier2(t) {
		p := tier2Projection(t, c)
		for _, pt := range c.Points {
			x, y, _ := p.Project(pt.Lon, pt.Lat)
			if pt.X == nil {
				if !math.IsNaN(x) && !math.IsNaN(y) {
					t.Errorf("%s(%v, %v) = %v, %v, want no point", c.name(), pt.Lon, pt.Lat, x, y)
				}
				continue
			}
			tol, cut := pt.tolerance()
			if c.Type == "berghaus" && tier2NearTip(p, pt.Lon, pt.Lat) {
				// d3-geo-projection loses precision close to the tip of
				// a lobe: by about 1e-7 of the unit of the raw projection
				// at 0.001° from it.
				tol = max(tol, 1e-4)
			}
			ok := math.Abs(x-*pt.X) <= tol && math.Abs(y-*pt.Y) <= tol
			if c.Type == "peirceQuincuncial" && tier2Antipode(p, pt.Lon, pt.Lat) {
				// The antipode of the center goes to the four corners of
				// the square, and zuzu and d3 can take different corners
				// by a rounding error.
				ok = ok || math.Abs(math.Abs(x-p.x)-math.Abs(*pt.X-p.x)) <= tol && math.Abs(math.Abs(y-p.y)-math.Abs(*pt.Y-p.y)) <= tol
			}
			for _, a := range pt.Around {
				ok = ok || cut && math.Hypot(x-a[0], y-a[1]) <= tol
			}
			if !ok {
				t.Errorf("%s(%v, %v) = %v, %v, want %v, %v", c.name(), pt.Lon, pt.Lat, x, y, *pt.X, *pt.Y)
			}
		}
	}
}

// TestTier2Paths compares the area and the bounds of the path of the
// sphere and of each country of world-110m with d3-geo-projection. They
// test the clip and the outline of each projection. The paths go through
// the same clip and the same resampling as in d3. An area must be within
// 1e-8 of it, relative, and a bound within 1e-5 of a pixel: a bound of
// the sphere of guyou is at a corner of a square, where the square root
// of the map makes a rounding error a hundred times larger.
func TestTier2Paths(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../test/testdata/corpus/data/world-110m.json")
	if err != nil {
		t.Fatal(err)
	}
	var topo any
	if err := json.Unmarshal(b, &topo); err != nil {
		t.Fatal(err)
	}
	countries, err := Feature(topo, "countries")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range readTier2(t) {
		p := tier2Projection(t, c)
		if len(c.Countries) != len(countries) {
			t.Fatalf("%s: %d countries, want %d", c.name(), len(countries), len(c.Countries))
		}
		check := func(what string, o any, want tier2Box) {
			t.Helper()
			a := Area(p, o)
			bb := Bounds(p, o)
			near := math.Abs(a-want.Area) <= 1e-8*math.Max(1, math.Abs(want.Area))
			for i := range 2 {
				for j := range 2 {
					near = near && math.Abs(bb[i][j]-want.Bounds[i][j]) <= 1e-5
				}
			}
			if !near {
				t.Errorf("%s %s: area %v, bounds %v, want %v, %v", c.name(), what, a, bb, want.Area, want.Bounds)
			}
		}
		check("sphere", map[string]any{"type": "Sphere"}, c.Sphere)
		for i, f := range countries {
			check(fmt.Sprintf("country %d", i), f, c.Countries[i])
		}
	}
}

// tier2NearTip reports whether a point is within 0.01° of the antipode of
// the center of a star projection, where the tips of its lobes are.
func tier2NearTip(p *Projection, lon, lat float64) bool {
	l, f := p.rot.forward(lon*radians, lat*radians)
	return acos(math.Cos(l)*math.Cos(f)) > math.Pi-0.01*radians
}

// tier2Antipode reports whether a point is the antipode of the center of
// a projection, but for rounding.
func tier2Antipode(p *Projection, lon, lat float64) bool {
	l, f := p.rot.forward(lon*radians, lat*radians)
	return math.Cos(l)*math.Cos(f) < -1+1e-12
}
