package geo

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"os"
	"testing"
)

// tier1Case is a case of testdata/projections-tier1.json, which
// test/oracle/projections-tier1.mjs writes from d3-geo-projection 4.0.0.
type tier1Case struct {
	Type   string             `json:"type"`
	Rotate []any              `json:"rotate"`
	Param  map[string]float64 `json:"param"`
	XY     [][2]*float64      `json:"xy"`
	Sphere tier1Measure       `json:"sphere"`
	World  tier1Measure       `json:"world"`
}

// tier1Measure is the area and the bounds of a path.
type tier1Measure struct {
	Area   float64        `json:"area"`
	Bounds [2][2]*float64 `json:"bounds"`
}

func readTier1(t *testing.T) ([][2]float64, []tier1Case) {
	t.Helper()
	b, err := os.ReadFile("testdata/projections-tier1.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Points [][2]float64 `json:"points"`
		Cases  []tier1Case  `json:"cases"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v.Points, v.Cases
}

func tier1Projection(t *testing.T, c tier1Case) *Projection {
	t.Helper()
	p := mustNew(t, c.Type)
	if c.Rotate != nil {
		if err := p.Set("rotate", c.Rotate); err != nil {
			t.Fatal(err)
		}
	}
	for name, v := range c.Param {
		if err := p.Set(name, v); err != nil {
			t.Fatalf("%s: the parameter %s: %v", c.Type, name, err)
		}
	}
	return p
}

// TestExtendedParams checks that a type ignores a parameter of
// d3-geo-projection that it does not take, as vega.js does, and that a
// parameter that a type takes and zuzu does not read yet is
// ErrParamUnread.
func TestExtendedParams(t *testing.T) {
	t.Parallel()
	p := mustNew(t, "mercator")
	x0, y0, _ := p.Project(10, 20)
	if err := p.Set("coefficient", 3.0); err != nil {
		t.Fatal(err)
	}
	if x, y, _ := p.Project(10, 20); x != x0 || y != y0 {
		t.Errorf("mercator moves a point from %v,%v to %v,%v with a coefficient", x0, y0, x, y)
	}
	for typ, param := range map[string]string{"armadillo": "parallel", "berghaus": "lobes"} {
		if err := mustNew(t, typ).Set(param, 4.0); err != nil {
			t.Errorf("%s: the parameter %s: %v", typ, param, err)
		}
	}
	if !TakesParam("hammer", "coefficient") || TakesParam("mercator", "coefficient") {
		t.Error("TakesParam does not hold the coefficient of hammer alone")
	}
}

// TestTier1Project compares each point with d3-geo-projection to 1e-6.
// A point near the antipode of an azimuthal projection, or near a pole of
// Littrow, lies far out, where the oracle and zuzu round apart. There the
// test allows a part in a billion. The oracle gives no point at the
// antipode itself, and zuzu gives one more than 1e15 away.
func TestTier1Project(t *testing.T) {
	t.Parallel()
	points, cases := readTier1(t)
	for _, c := range cases {
		p := tier1Projection(t, c)
		for i, pt := range points {
			want := c.XY[i]
			x, y, _ := p.Project(pt[0], pt[1])
			if want[0] == nil || want[1] == nil {
				if !farOut(x) && !farOut(y) {
					t.Errorf("%s %v %v at %v: got %v, %v, want no point", c.Type, c.Rotate, c.Param, pt, x, y)
				}
				continue
			}
			if !nearPoint(x, *want[0]) || !nearPoint(y, *want[1]) {
				t.Errorf("%s %v %v at %v: got %v, %v, want %v, %v", c.Type, c.Rotate, c.Param, pt, x, y, *want[0], *want[1])
			}
		}
	}
}

func nearPoint(a, b float64) bool {
	return math.Abs(a-b) <= math.Max(1e-6, 1e-9*math.Abs(b))
}

func farOut(v float64) bool {
	return math.IsNaN(v) || math.Abs(v) > 1e15
}

func TestTier1Measures(t *testing.T) {
	t.Parallel()
	_, cases := readTier1(t)
	topo := readWorld(t)
	countries, err := Feature(topo, "countries")
	if err != nil {
		t.Fatal(err)
	}
	world := map[string]any{"type": "FeatureCollection", "features": countries}
	sphere := map[string]any{"type": "Sphere"}
	for _, c := range cases {
		p := tier1Projection(t, c)
		name := fmt.Sprintf("%s %v %v", c.Type, c.Rotate, c.Param)
		tier1Check(t, name+" sphere", p, sphere, c.Sphere)
		tier1Check(t, name+" world", p, world, c.World)
	}
}

// measure compares the area and the bounds of the path of an object with
// those of d3-geo, to a part in a million.
func tier1Check(t *testing.T, name string, p *Projection, object any, want tier1Measure) {
	t.Helper()
	within := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(b))
	}
	if a := Area(p, object); !within(a, want.Area) {
		t.Errorf("%s: the area is %v, want %v", name, a, want.Area)
	}
	b := Bounds(p, object)
	for i := range 2 {
		for j := range 2 {
			if w := want.Bounds[i][j]; w == nil || !within(b[i][j], *w) {
				t.Errorf("%s: the bounds are %v, want %v", name, b, want.Bounds)
				return
			}
		}
	}
}

// readWorld reads the countries of world-110m from the corpus of the test
// module.
func readWorld(t *testing.T) any {
	t.Helper()
	b, err := os.ReadFile("../test/testdata/corpus/data/world-110m.json")
	if err != nil {
		t.Fatal(err)
	}
	return obj(t, string(b))
}
