package dataflow

import (
	"math"
	"testing"
)

// A small tree: a root with two children, the second with two leaves.
const treeRows = `[{"id":1,"p":null,"v":0},{"id":2,"p":1,"v":1},{"id":3,"p":1,"v":0},{"id":4,"p":3,"v":1},{"id":5,"p":3,"v":2}]`

func TestHierarchyTransforms(t *testing.T) {
	t.Parallel()
	stratify := `{"type":"stratify","key":"id","parentKey":"p"}`
	tests := []struct {
		name, ts, want string
	}{
		{"tree", `[` + stratify + `,{"type":"tree","size":[4,2]},{"type":"project","fields":["id","x","y","depth","children"]}]`,
			// Node 2 lies at 0 and leaf 5 at 1.5 before the scaling. They
			// are cousins, so each side keeps a margin of 1, and x scales
			// by 4 over 3.5.
			`[{"children":2,"depth":0,"id":1,"x":1.7142857142857142,"y":0},{"children":0,"depth":1,"id":2,"x":1.1428571428571428,"y":1},{"children":2,"depth":1,"id":3,"x":2.2857142857142856,"y":1},{"children":0,"depth":2,"id":4,"x":1.7142857142857142,"y":2},{"children":0,"depth":2,"id":5,"x":2.8571428571428568,"y":2}]`},
		{"treelinks", `[` + stratify + `,{"type":"treelinks"},{"type":"formula","expr":"datum.source.id + '-' + datum.target.id","as":"l"},{"type":"project","fields":["l"]}]`,
			`[{"l":"1-2"},{"l":"1-3"},{"l":"3-4"},{"l":"3-5"}]`},
		{"partition", `[` + stratify + `,{"type":"partition","field":"v","size":[4,3]},{"type":"project","fields":["id","x0","x1","y0","y1"]}]`,
			`[{"id":1,"x0":0,"x1":4,"y0":0,"y1":1},{"id":2,"x0":0,"x1":1,"y0":1,"y1":2},{"id":3,"x0":1,"x1":4,"y0":1,"y1":2},{"id":4,"x0":1,"x1":2,"y0":2,"y1":3},{"id":5,"x0":2,"x1":4,"y0":2,"y1":3}]`},
		{"treemap dice", `[` + stratify + `,{"type":"treemap","field":"v","method":"dice","size":[4,1]},{"type":"project","fields":["id","x0","x1"]}]`,
			`[{"id":1,"x0":0,"x1":4},{"id":2,"x0":0,"x1":1},{"id":3,"x0":1,"x1":4},{"id":4,"x0":1,"x1":2},{"id":5,"x0":2,"x1":4}]`},
		{"linkpath", `[{"type":"linkpath","orient":"horizontal","shape":"diagonal"}]`,
			`[{"path":"M0,0C1,0 1,2 2,2","source":{"x":0,"y":0},"target":{"x":2,"y":2}}]`},
	}
	for _, test := range tests {
		rows := treeRows
		if test.name == "linkpath" {
			rows = `[{"source":{"x":0,"y":0},"target":{"x":2,"y":2}}]`
		}
		if got := run(t, newEnv(), rows, test.ts); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", test.name, got, test.want)
		}
	}
}

func TestPackSiblings(t *testing.T) {
	t.Parallel()
	cs := []circle{{r: 1}, {r: 2}, {r: 3}, {r: 1.5}, {r: 0.5}}
	r := packSiblings(cs)
	for i, a := range cs {
		if math.Hypot(a.x, a.y)+a.r > r+1e-9 {
			t.Errorf("circle %d lies outside the enclosing circle", i)
		}
		for j := range i {
			b := cs[j]
			if math.Hypot(a.x-b.x, a.y-b.y) < a.r+b.r-1e-6 {
				t.Errorf("circles %d and %d overlap", i, j)
			}
		}
	}
}

func TestStratifyNeedsOneRoot(t *testing.T) {
	t.Parallel()
	rows := rowsOf(t, `[{"id":1,"p":null},{"id":2,"p":null}]`)
	if _, err := Run(newEnv(), rows, transformsOf(t, `[{"type":"stratify","key":"id","parentKey":"p"}]`), "/transform"); err == nil {
		t.Error("two roots give no error")
	}
}
