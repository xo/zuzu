package geo

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"os"
	"testing"
)

// tier3 is the file that test/oracle/projections-tier3.mjs writes, from
// d3-geo-projection 4.0.0 and d3-geo 3.1.2.
type tier3 struct {
	Points [][2]float64 `json:"points"`
	Cases  []struct {
		Type      string        `json:"type"`
		Rotate    []any         `json:"rotate"`
		XY        []*[2]float64 `json:"xy"`
		Sphere    measure       `json:"sphere"`
		Graticule measure       `json:"graticule"`
		Countries []measure     `json:"countries"`
	} `json:"cases"`
}

// measure is the area and the bounds of a path in d3. Gap marks a path
// that crosses a gap between two lobes, which d3 does not cut.
type measure struct {
	Area   float64       `json:"area"`
	Bounds [2][2]float64 `json:"bounds"`
	Gap    bool          `json:"gap"`
}

func readTier3(t *testing.T) *tier3 {
	t.Helper()
	b, err := os.ReadFile("testdata/projections-tier3.json")
	if err != nil {
		t.Fatal(err)
	}
	var d tier3
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return &d
}

func tier3Projection(t *testing.T, typ string, rotate []any) *Projection {
	t.Helper()
	p := mustNew(t, typ)
	if rotate != nil {
		if err := p.Set("rotate", rotate); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// onCut reports whether a point, once turned, lies within 1e-9 radians of
// a meridian where two lobes or two faces meet. There the lobe depends on
// the last bits of the turn, which differ from those of d3.
func onCut(p *Projection, q [2]float64) bool {
	lambda, phi := p.rot.forward(q[0]*radians, q[1]*radians)
	var cuts []float64
	if p.lobes != nil {
		lobes := p.lobes.north
		if phi < 0 {
			lobes = p.lobes.south
		}
		for _, l := range lobes {
			cuts = append(cuts, l.east)
		}
	} else {
		cuts = []float64{-halfPi, 0, halfPi}
	}
	for _, c := range cuts {
		if math.Abs(lambda-c) < 1e-9 {
			return true
		}
	}
	return false
}

func TestTier3Points(t *testing.T) {
	t.Parallel()
	d := readTier3(t)
	for _, c := range d.Cases {
		p := tier3Projection(t, c.Type, c.Rotate)
		bad := 0
		for i, q := range d.Points {
			x, y, _ := p.Project(q[0], q[1])
			want := c.XY[i]
			if want == nil {
				t.Errorf("%s %v: no point for %v", c.Type, c.Rotate, q)
				continue
			}
			if (math.Abs(x-want[0]) > 1e-6 || math.Abs(y-want[1]) > 1e-6) && !onCut(p, q) {
				if bad++; bad <= 5 {
					t.Errorf("%s %v: %v = %v, %v, want %v, %v", c.Type, c.Rotate, q, x, y, want[0], want[1])
				}
			}
		}
		if bad > 5 {
			t.Errorf("%s %v: %d points differ", c.Type, c.Rotate, bad)
		}
	}
}

// TestTier3Paths compares the area and the bounds of the paths with those
// of d3: the outline of the sphere, the graticule and the countries of
// world-110m. The clip of zuzu leaves out a strip of 2e-6 radians along
// each cut, where d3 does not clip, and it sends a point every 2° along the
// edge of a lobe, where d3 leaves the edge to the resampling, so an area
// can differ by 0.4%. Where the path of d3 crosses a gap, the path of zuzu must lie
// inside its bounds and have no larger area.
func TestTier3Paths(t *testing.T) {
	t.Parallel()
	d := readTier3(t)
	b, err := os.ReadFile("../test/testdata/corpus/data/world-110m.json")
	if err != nil {
		t.Skip("the corpus of the test module is not here")
	}
	var topo any
	if err := json.Unmarshal(b, &topo); err != nil {
		t.Fatal(err)
	}
	countries, err := Feature(topo, "countries")
	if err != nil {
		t.Fatal(err)
	}
	sphere := map[string]any{"type": "Sphere"}
	graticule := NewGraticule().Lines()
	for _, c := range d.Cases {
		p := tier3Projection(t, c.Type, c.Rotate)
		check := func(what string, o any, want measure) {
			t.Helper()
			const tol = 0.05
			area, got := Area(p, o), Bounds(p, o)
			if math.IsInf(got[0][0], 1) {
				// An empty path has no bounds, as in d3.
				got = [2][2]float64{{math.NaN(), math.NaN()}, {math.NaN(), math.NaN()}}
			}
			w := want.Bounds
			if want.Gap {
				if area > want.Area*(1+4e-3)+tol || got[0][0] < w[0][0]-tol || got[0][1] < w[0][1]-tol || got[1][0] > w[1][0]+tol || got[1][1] > w[1][1]+tol {
					t.Errorf("%s %v %s: area %v, bounds %v, want no more than %v, %v", c.Type, c.Rotate, what, area, got, want.Area, w)
				}
				return
			}
			if math.Abs(area-want.Area) > tol+4e-3*want.Area {
				t.Errorf("%s %v %s: area %v, want %v", c.Type, c.Rotate, what, area, want.Area)
			}
			for i := range 2 {
				for j := range 2 {
					if math.Abs(got[i][j]-w[i][j]) > tol {
						t.Errorf("%s %v %s: bounds %v, want %v", c.Type, c.Rotate, what, got, w)
						return
					}
				}
			}
		}
		check("sphere", sphere, c.Sphere)
		check("graticule", graticule, c.Graticule)
		for i, f := range countries {
			check(fmt.Sprint("country ", i), f, c.Countries[i])
		}
	}
}
