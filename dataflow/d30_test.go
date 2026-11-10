package dataflow

import (
	"math"
	"strings"
	"testing"
)

func TestSankey(t *testing.T) {
	t.Parallel()
	links := `[{"source":"a","target":"c","value":2},{"source":"b","target":"c","value":1},{"source":"c","target":"d","value":3}]`
	nodes, err := Run(newEnv(), rowsOf(t, links), transformsOf(t, `[{"type":"zuzu.sankey","size":[100,60],"nodeWidth":10,"nodePadding":10}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 4 {
		t.Fatalf("got %d nodes, want 4", len(nodes))
	}
	by := map[string]Row{}
	for _, n := range nodes {
		by[str(n["key"])] = n
	}
	// Three columns from 0 to 90, and the node c gathers both flows.
	if by["a"]["x0"] != 0.0 || by["c"]["x0"] != 45.0 || by["d"]["x1"] != 100.0 {
		t.Errorf("columns: a %v, c %v, d %v", by["a"]["x0"], by["c"]["x0"], by["d"]["x1"])
	}
	if by["c"]["value"] != 3.0 || by["c"]["depth"] != 1.0 {
		t.Errorf("c: %v", by["c"])
	}
	// The fullest column, a and b with the padding, fills the height.
	if h := f64(by["c"]["y1"]) - f64(by["c"]["y0"]); math.Abs(h-50) > 1e-9 {
		t.Errorf("c is %v tall, want 50", h)
	}
	env := newEnv()
	env.data["nodes"] = nodes
	out, err := Run(env, rowsOf(t, links), transformsOf(t, `[{"type":"zuzu.sankeylinks","nodes":"nodes"}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range out {
		p := str(l["path"])
		if !strings.HasPrefix(p, "M") || !strings.HasSuffix(p, "Z") {
			t.Errorf("path %q", p)
		}
	}
	if w := f64(out[0]["width"]); math.Abs(w-50.0*2/3) > 1e-9 {
		t.Errorf("the width of a to c is %v, want %v", w, 50.0*2/3)
	}
	cycle := `[{"source":"a","target":"b","value":1},{"source":"b","target":"a","value":1}]`
	if _, err := Run(newEnv(), rowsOf(t, cycle), transformsOf(t, `[{"type":"zuzu.sankey","size":[100,60]}]`), "/transform"); err == nil {
		t.Error("a cycle gives no error")
	}
}

func TestChord(t *testing.T) {
	t.Parallel()
	links := `[{"source":"a","target":"b","value":3},{"source":"b","target":"a","value":1},{"source":"a","target":"c","value":4}]`
	groups, err := Run(newEnv(), rowsOf(t, links), transformsOf(t, `[{"type":"zuzu.chord"}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	// The flows out: a 7, b 1, c 0, around the whole circle.
	if len(groups) != 3 || groups[0]["value"] != 7.0 || groups[2]["value"] != 0.0 {
		t.Fatalf("groups %v", groups)
	}
	if end := f64(groups[1]["endAngle"]); math.Abs(end-2*math.Pi) > 1e-9 {
		t.Errorf("the last group ends at %v, want 2π", end)
	}
	env := newEnv()
	env.data["groups"] = groups
	out, err := Run(env, rowsOf(t, links), transformsOf(t, `[{"type":"zuzu.chordribbons","groups":"groups","radius":100}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	// The link from b to a is the other end of the ribbon of a to b.
	if out[0]["path"] == nil || out[1]["path"] != nil || out[2]["path"] == nil {
		t.Errorf("paths %v, %v, %v", out[0]["path"], out[1]["path"], out[2]["path"])
	}
	if w := f64(out[0]["sourceEndAngle"]) - f64(out[0]["sourceStartAngle"]); math.Abs(w-3.0/8*2*math.Pi) > 1e-9 {
		t.Errorf("the end of a to b spans %v", w)
	}
}

func TestHexbin(t *testing.T) {
	t.Parallel()
	rows := `[{"x":1,"y":1,"v":2},{"x":2,"y":-1,"v":4},{"x":100,"y":100,"v":1}]`
	out, err := Run(newEnv(), rowsOf(t, rows), transformsOf(t, `[{"type":"zuzu.hexbin","x":"x","y":"y","radius":10,"ops":["count","mean"],"fields":[null,"v"],"as":["count","mean"]}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d hexagons, want 2", len(out))
	}
	if out[0]["x"] != 0.0 || out[0]["y"] != 0.0 || out[0]["count"] != 2.0 || out[0]["mean"] != 3.0 {
		t.Errorf("first hexagon %v", out[0])
	}
	if p := str(out[0]["path"]); p != "M0,-10L8.660254038,-5L8.660254038,5L0,10L-8.660254038,5L-8.660254038,-5Z" {
		t.Errorf("path %q", p)
	}
}

// f64 reads a number of a row in a test.
func f64(v any) float64 {
	f, _ := v.(float64)
	return f
}

// str reads a string of a row in a test.
func str(v any) string {
	s, _ := v.(string)
	return s
}
