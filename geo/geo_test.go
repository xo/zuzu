package geo

import (
	"encoding/json/v2"
	"math"
	"strings"
	"testing"
)

// The expected numbers come from d3-geo 3.1.2, the version that the oracle
// of vega.js runs.

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-8*math.Max(1, math.Abs(b))
}

func mustNew(t *testing.T, typ string) *Projection {
	t.Helper()
	p, err := New(typ)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func obj(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestProject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		typ      string
		set      map[string]any
		lon, lat float64
		x, y     float64
		ok       bool
	}{
		{"mercator", nil, -122.4, 37.8, 153.25999999999993, 140.86183723542047, true},
		{"albersUsa", nil, -122.4, 37.8, 107.8293996861774, 213.76568215360805, true},
		{"albersUsa", nil, -150, 61, 170.9370687971184, 448.39147153215595, true},
		{"albersUsa", nil, -157.8, 21.3, 299.4995950411043, 451.1187364729819, true},
		{"albersUsa", nil, 0, 0, 0, 0, false},
		{"orthographic", map[string]any{"rotate": []any{10.0, -20.0}}, 30, 40, 602.854767188273, 149.3723799109734, true},
		{"equalEarth", nil, 100, -45, 708.3083370578737, 402.39681864890827, true},
		{"transverseMercator", nil, 10, 50, 497.83903751594363, 109.91296922979038, true},
	}
	for _, test := range tests {
		p := mustNew(t, test.typ)
		for k, v := range test.set {
			if err := p.Set(k, v); err != nil {
				t.Fatal(err)
			}
		}
		x, y, ok := p.Project(test.lon, test.lat)
		if ok != test.ok || ok && (!near(x, test.x) || !near(y, test.y)) {
			t.Errorf("%s(%v, %v) = %v, %v, %v, want %v, %v, %v", test.typ, test.lon, test.lat, x, y, ok, test.x, test.y, test.ok)
		}
	}
}

func TestPath(t *testing.T) {
	t.Parallel()
	// A polygon across the antimeridian winds the other way round the
	// globe, so it covers all of it but itself.
	p := mustNew(t, "equirectangular")
	got, _ := Path(p, obj(t, `{"type":"Polygon","coordinates":[[[170,10],[-170,10],[-170,20],[170,20],[170,10]]]}`), 4.5)
	want := "M0.499,222.959L27.138,223.361L27.138,196.722L0.499,195.967L0.499,149.537L0.499,103.108L0.499,56.679L0.499,10.249L480,10.249L959.501,10.249L959.501,56.679L959.501,103.108L959.501,149.537L959.501,195.967L932.862,196.722L932.862,223.361L932.862,223.361L959.501,222.959L959.501,289.657L959.501,356.355L959.501,423.053L959.501,489.751L480,489.751L0.499,489.751L0.499,423.053L0.499,356.355L0.499,289.657Z"
	if got != want {
		t.Errorf("antimeridian:\ngot  %s\nwant %s", got, want)
	}
	// A line on the far side of an orthographic globe draws nothing.
	if got, ok := Path(mustNew(t, "orthographic"), obj(t, `{"type":"LineString","coordinates":[[-100,0],[100,0]]}`), 4.5); ok {
		t.Errorf("far side: got %q, want nothing", got)
	}
	// The sphere of an orthographic projection is its circle, in steps of
	// 2 degrees.
	o := mustNew(t, "orthographic")
	o.setScale(10)
	o.setTranslate(0, 0)
	got, _ = Path(o, obj(t, `{"type":"Sphere"}`), 4.5)
	if !strings.HasPrefix(got, "M0,10L-0.349,9.994L-0.698,9.976") || !strings.HasSuffix(got, "L0.349,9.994L0,10Z") || strings.Count(got, "L") != 180 {
		t.Errorf("sphere: got %s", got)
	}
	got, _ = Path(mustNew(t, "mercator"), obj(t, `{"type":"Point","coordinates":[0,0]}`), 4.5)
	if want := "M480,250m0,4.5a4.5,4.5 0 1,1 0,-9a4.5,4.5 0 1,1 0,9z"; got != want {
		t.Errorf("point: got %s, want %s", got, want)
	}
}

func TestMeasures(t *testing.T) {
	t.Parallel()
	square := obj(t, `{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]]]}`)
	p := mustNew(t, "mercator")
	if a := Area(p, square); !near(a, 922804.7610326195) {
		t.Errorf("area = %v", a)
	}
	if x, y := Centroid(p, square); !near(x, 479.98964049486364) || !near(y, 250.01041250400905) {
		t.Errorf("centroid = %v, %v", x, y)
	}
}

func TestFit(t *testing.T) {
	t.Parallel()
	p := mustNew(t, "albersUsa")
	p.Fit(0, 0, 500, 300, obj(t, `{"type":"MultiPoint","coordinates":[[-122,37],[-70,45],[-80,25]]}`))
	l := p.usa.lower48
	if !near(l.k, 745.2243344777293) || !near(l.x, 258.12752651642217) || !near(l.y, 126.14265599791868) {
		t.Errorf("fit: scale %v, translate %v, %v", l.k, l.x, l.y)
	}
}

func TestGraticule(t *testing.T) {
	t.Parallel()
	lines, _ := NewGraticule().Lines()["coordinates"].([]any)
	counts := map[int]int{}
	for _, l := range lines {
		points, _ := l.([]any)
		counts[len(points)]++
	}
	// 36 meridians of 3 points, and 17 parallels of 145 points: the
	// equator and 16 minor ones.
	if len(lines) != 53 || counts[3] != 36 || counts[145] != 17 {
		t.Errorf("got %d lines, with these counts of points: %v", len(lines), counts)
	}
}

func TestFeature(t *testing.T) {
	t.Parallel()
	// The line runs along arc 0 and back along it, as ~0 is -1.
	topo := obj(t, `{"type":"Topology","transform":{"scale":[2,3],"translate":[10,20]},
		"arcs":[[[0,0],[1,0],[0,1]],[[1,1],[-1,-1]]],
		"objects":{"o":{"type":"GeometryCollection","geometries":[
			{"type":"LineString","arcs":[0,-1],"id":7},
			{"type":"Point","coordinates":[1,1],"properties":{"a":1}}]}}}`)
	fs, err := Feature(topo, "o")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(fs, json.Deterministic(true))
	want := `[{"geometry":{"coordinates":[[10,20],[12,20],[12,23],[12,20],[10,20]],"type":"LineString"},"id":7,"properties":{},"type":"Feature"},{"geometry":{"coordinates":[12,23],"type":"Point"},"properties":{"a":1},"type":"Feature"}]`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
	if _, err := Feature(topo, "nope"); err == nil {
		t.Error("a missing object gives no error")
	}
}
