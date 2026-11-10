package dataflow

import (
	"math"
	"testing"
)

func TestForce(t *testing.T) {
	t.Parallel()
	// Two nodes on the same place, which a collision pushes apart and the
	// center keeps around the origin.
	rows := rowsOf(t, `[{"x":0,"y":0},{"x":0,"y":0}]`)
	ts := `[{"type":"force","forces":[{"force":"center"},{"force":"collide","radius":5,"strength":1}]}]`
	out, err := Run(newEnv(), rows, transformsOf(t, ts), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	ax, _ := out[0]["x"].(float64)
	ay, _ := out[0]["y"].(float64)
	bx, _ := out[1]["x"].(float64)
	by, _ := out[1]["y"].(float64)
	if d := math.Hypot(ax-bx, ay-by); d < 9.9 {
		t.Errorf("the nodes lie %v apart, want at least 10", d)
	}
	if mx, my := (ax+bx)/2, (ay+by)/2; math.Abs(mx) > 1e-9 || math.Abs(my) > 1e-9 {
		t.Errorf("the middle lies at %v, %v, want the origin", mx, my)
	}
	// The same rows give the same places.
	again, err := Run(newEnv(), rowsOf(t, `[{"x":0,"y":0},{"x":0,"y":0}]`), transformsOf(t, ts), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := again[0]["x"].(float64); x != ax {
		t.Errorf("a second run places the node at %v, want %v", x, ax)
	}
}
