package mermaid_test

import (
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
)

// swimlaneRanks holds the rank of each node of each fixture of swimlane in
// mmdc, which bundles Mermaid 12.3.0: the order of its place along the
// ranks in the SVG of mmdc.
var swimlaneRanks = map[string]map[string]int{
	"01-flow-lr.mmd":         {"answer": 1, "fix": 1, "investigate": 0, "receive": 1, "request": 0, "triage": 0},
	"02-decisions-tb.mmd":    {"collect": 0, "complete": 3, "decide": 2, "review": 1, "schedule": 2, "validate": 1},
	"03-default-lane-bt.mmd": {"a": 0, "b": 1, "done": 1, "later": 2, "start": 0},
	"04-nested-rl.mmd":       {"check": 2, "intake": 0, "plan": 1, "review": 2, "sign": 3},
	"05-lane-edges.mmd":      {"a": 0, "b": 1, "c": 1, "d": 2, "e": 3},
	"06-styles.mmd":          {"intake": 0, "plan": 1, "review": 1},
}

// laneBox is the box of a row of a layout, from its top left to its bottom
// right.
type laneBox struct{ x1, y1, x2, y2 float64 }

func (b laneBox) holds(o laneBox) bool {
	return o.x1 >= b.x1-0.01 && o.y1 >= b.y1-0.01 && o.x2 <= b.x2+0.01 && o.y2 <= b.y2+0.01
}

func (b laneBox) overlaps(o laneBox) bool {
	return b.x1 < o.x2 && o.x1 < b.x2 && b.y1 < o.y2 && o.y1 < b.y2
}

// TestSwimlaneFixtures draws each fixture of swimlane and checks it: each
// node and each label of the source draws once, each node has the rank of
// mmdc, each node is in the laneBox of its lane, the lanes lie side by side in
// the order of the source and run along all the ranks, no two nodes
// overlap, and each edge routes at right angles around the other nodes.
func TestSwimlaneFixtures(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "swimlane") {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := mermaid.Read(string(src))
			if err != nil {
				t.Fatal(err)
			}
			g, report, err := mermaid.Graph(d, true)
			if err != nil || len(report) > 0 {
				t.Fatalf("expected no issue, got %v %v", err, report)
			}
			if !g.Lanes {
				t.Fatal("expected a layout in lanes")
			}
			items, _ := drawFixture(t, path)
			nodes, clusters := map[string]laneBox{}, map[string]laneBox{}
			rank := map[string]int{}
			var routes [][][2]float64
			for _, it := range items {
				switch it.datum["kind"] {
				case "node":
					if it.mark != scene.Path {
						continue
					}
					key := expr.String(it.datum["key"])
					if _, ok := nodes[key]; ok {
						t.Errorf("node %s draws twice", key)
					}
					x, y := expr.Number(it.datum["x"]), expr.Number(it.datum["y"])
					w, h := expr.Number(it.datum["width"]), expr.Number(it.datum["height"])
					nodes[key] = laneBox{x - w/2, y - h/2, x + w/2, y + h/2}
					rank[key] = int(expr.Number(it.datum["rank"]))
				case "cluster":
					if it.mark != scene.Rect {
						continue
					}
					x, y := expr.Number(it.datum["x"]), expr.Number(it.datum["y"])
					w, h := expr.Number(it.datum["width"]), expr.Number(it.datum["height"])
					clusters[expr.String(it.datum["key"])] = laneBox{x, y, x + w, y + h}
				case "edge":
					if it.mark != scene.Path {
						continue
					}
					var pts [][2]float64
					ps, _ := it.datum["points"].([]any)
					for _, p := range ps {
						xy, _ := p.([]any)
						pts = append(pts, [2]float64{expr.Number(xy[0]), expr.Number(xy[1])})
					}
					routes = append(routes, pts)
				}
			}
			for _, n := range g.Nodes {
				if _, ok := nodes[n.ID]; !ok {
					t.Errorf("node %s does not draw", n.ID)
				}
				label := strings.ReplaceAll(n.Label, "**", "")
				if label != "" && textDrawn(items, label) != 1 {
					t.Errorf("node %s: expected its label %q once, got %d", n.ID, label, textDrawn(items, label))
				}
			}
			for _, e := range g.Edges {
				if e.Label != "" && textDrawn(items, e.Label) != 1 {
					t.Errorf("edge %s -> %s: expected its label %q once", e.Source, e.Target, e.Label)
				}
			}
			if len(routes) != len(g.Edges) {
				t.Errorf("expected %d routes, got %d", len(g.Edges), len(routes))
			}
			if want := swimlaneRanks[name]; !maps.Equal(rank, want) {
				t.Errorf("expected the ranks of mmdc %v, got %v", want, rank)
			}
			checkLanes(t, g, nodes, clusters, routes)
		})
	}
}

// checkLanes checks the boxes of the lanes and of the nodes, and the
// routes, of a layout in lanes.
func checkLanes(t *testing.T, g *graph.Graph, nodes, clusters map[string]laneBox, routes [][][2]float64) {
	t.Helper()
	parent := map[string]string{}
	var lanes []string
	for _, c := range g.Clusters {
		parent[c.ID] = c.Parent
		if c.Parent == "" {
			lanes = append(lanes, c.ID)
		}
	}
	top := func(c string) string {
		for parent[c] != "" {
			c = parent[c]
		}
		return c
	}
	for _, n := range g.Nodes {
		for c := n.Cluster; c != ""; c = parent[c] {
			if !clusters[c].holds(nodes[n.ID]) {
				t.Errorf("node %s is not in the laneBox of %s", n.ID, c)
			}
		}
		if n.Cluster == "" || !slices.Contains(lanes, top(n.Cluster)) {
			t.Errorf("node %s is in no lane", n.ID)
		}
	}
	// The lanes lie side by side across the ranks, in order, and each runs
	// along all the ranks.
	sideways := g.Direction == graph.LR || g.Direction == graph.RL
	for k := 1; k < len(lanes); k++ {
		a, b := clusters[lanes[k-1]], clusters[lanes[k]]
		before := a.x2 <= b.x1
		same := a.y1 == b.y1 && a.y2 == b.y2
		if sideways {
			before, same = a.y2 <= b.y1, a.x1 == b.x1 && a.x2 == b.x2
		}
		if !before || !same {
			t.Errorf("lane %s does not lie after lane %s along the same ranks: %v %v", lanes[k], lanes[k-1], a, b)
		}
	}
	ids := slices.Sorted(maps.Keys(nodes))
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			if nodes[a].overlaps(nodes[b]) {
				t.Errorf("nodes %s and %s overlap", a, b)
			}
		}
	}
	for i, pts := range routes {
		e := g.Edges[i]
		for k := 1; k < len(pts); k++ {
			p, q := pts[k-1], pts[k]
			if math.Abs(p[0]-q[0]) > 0.01 && math.Abs(p[1]-q[1]) > 0.01 {
				t.Errorf("edge %s -> %s: the part %v %v is not at a right angle", e.Source, e.Target, p, q)
			}
			for _, id := range ids {
				if id == e.Source || id == e.Target {
					continue
				}
				b := nodes[id]
				if cuts(b, p, q) {
					t.Errorf("edge %s -> %s crosses node %s", e.Source, e.Target, id)
				}
			}
		}
	}
}

// cuts reports whether a part of a route at a right angle passes through a
// laneBox.
func cuts(b laneBox, p, q [2]float64) bool {
	if p[1] == q[1] {
		return p[1] > b.y1 && p[1] < b.y2 && math.Max(p[0], q[0]) > b.x1 && math.Min(p[0], q[0]) < b.x2
	}
	return p[0] > b.x1 && p[0] < b.x2 && math.Max(p[1], q[1]) > b.y1 && math.Min(p[1], q[1]) < b.y2
}
