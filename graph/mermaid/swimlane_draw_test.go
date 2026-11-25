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
	"07-lane-order.mmd":      {"confirm": 0, "invoice": 0, "order": 0, "paid": 1, "pick": 0, "receive": 1, "ship": 1},
	"08-line-hops.mmd":       {"draft": 0, "lead": 0, "legal": 1, "plan": 0, "price": 2, "sign": 3, "staff": 1, "start": 2},
	"09-line-hops-gap.mmd":   {"draft": 0, "lead": 0, "legal": 1, "plan": 0, "price": 2, "sign": 3, "staff": 1, "start": 2},
	"10-rank-crossings.mmd":  {"approve": 1, "ask": 0, "create": 2, "grant": 3, "use": 4},
}

// swimlaneLanes holds the order of the lanes of each fixture whose lanes
// mmdc orders with automaticLaneOrdering. The lanes of the other fixtures
// keep the order of the source.
var swimlaneLanes = map[string][]string{
	"07-lane-order.mmd": {"Finance", "Customer", "Sales", "Warehouse"},
}

// swimlaneHops holds the kind of the hops that a fixture draws where two
// routes cross: A for an arc, and M for a gap.
var swimlaneHops = map[string]string{
	"08-line-hops.mmd":     "A",
	"09-line-hops-gap.mmd": "M",
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
// mmdc, each node is in the laneBox of its lane, the lanes lie side by side
// in the order of mmdc and run along all the ranks, no two nodes overlap,
// each edge routes at right angles around the other nodes and shares no
// line with another edge, a hop draws where two edges cross, and the title
// of a lane turns when the ranks run from the left to the right.
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
			var paths []string
			titles := map[string]drawnItem{}
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
					if it.mark == scene.Text {
						titles[expr.String(it.datum["key"])] = it
					}
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
					paths = append(paths, expr.String(it.datum["path"]))
				}
			}
			for _, n := range g.Nodes {
				if _, ok := nodes[n.ID]; !ok {
					t.Errorf("node %s does not draw", n.ID)
				}
				if len(n.Runs) > 0 {
					// A label in more than one font draws each run as a
					// text of its own.
					for _, run := range n.Runs {
						want := strings.TrimSpace(run.Text)
						if want == "" {
							continue
						}
						got := countDrawn(items, scene.Text, func(it drawnItem) bool { return strings.TrimSpace(it.text) == want })
						if got == 0 {
							t.Errorf("node %s: the run %q does not draw", n.ID, want)
						}
					}
					continue
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
			checkLanes(t, g, swimlaneLanes[name], nodes, clusters, routes)
			checkHops(t, swimlaneHops[name], paths)
			checkTitles(t, g, nodes, clusters, titles)
		})
	}
}

// checkHops checks that the paths of the edges draw the hops of a kind, A
// for an arc and M for a gap, and no hop of another kind.
func checkHops(t *testing.T, kind string, paths []string) {
	t.Helper()
	arcs, gaps := 0, 0
	for _, p := range paths {
		arcs += strings.Count(p, "A")
		gaps += strings.Count(p[min(1, len(p)):], "M")
	}
	switch {
	case kind == "A" && (arcs == 0 || gaps > 0), kind == "M" && (gaps == 0 || arcs > 0), kind == "" && arcs+gaps > 0:
		t.Errorf("expected the hops %q, got %d arcs and %d gaps", kind, arcs, gaps)
	}
}

// checkTitles checks the title of each lane: it turns by -90 degrees and
// lies at the left of the nodes of its lane, inside the lane, when the
// ranks run from the left to the right, and it does not turn otherwise.
func checkTitles(t *testing.T, g *graph.Graph, nodes, clusters map[string]laneBox, titles map[string]drawnItem) {
	t.Helper()
	for _, c := range g.Clusters {
		it, ok := titles[c.ID]
		if c.Parent != "" || !ok {
			continue
		}
		angle := it.item.Angle
		if g.Direction != graph.LR {
			if angle != 0 {
				t.Errorf("lane %s: expected a title that does not turn, got the angle %v", c.ID, angle)
			}
			continue
		}
		if angle != -90 {
			t.Errorf("lane %s: expected a title turned by -90 degrees, got %v", c.ID, angle)
		}
		x, y := it.item.X, it.item.Y
		lane := clusters[c.ID]
		if x < lane.x1 || x > lane.x2 || y < lane.y1 || y > lane.y2 {
			t.Errorf("lane %s: expected its title at %v %v inside the lane %v", c.ID, x, y, lane)
		}
		for _, n := range g.Nodes {
			if b, ok := nodes[n.ID]; ok && n.Cluster == c.ID && b.x1 < x {
				t.Errorf("lane %s: expected its title at the left of the node %s, got %v and %v", c.ID, n.ID, x, b)
			}
		}
	}
}

// checkLanes checks the boxes of the lanes and of the nodes, and the
// routes, of a layout in lanes. The lanes lie in the order of order, or of
// the source for none.
func checkLanes(t *testing.T, g *graph.Graph, order []string, nodes, clusters map[string]laneBox, routes [][][2]float64) {
	t.Helper()
	parent := map[string]string{}
	var lanes []string
	for _, c := range g.Clusters {
		parent[c.ID] = c.Parent
		if c.Parent == "" {
			lanes = append(lanes, c.ID)
		}
	}
	if order != nil {
		lanes = order
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
		for j := range i {
			if sharesLine(pts, routes[j]) {
				t.Errorf("edge %s -> %s shares a line with edge %s -> %s", e.Source, e.Target, g.Edges[j].Source, g.Edges[j].Target)
			}
		}
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

// sharesLine reports whether two routes at right angles run along the same
// line for more than a pixel.
func sharesLine(a, b [][2]float64) bool {
	for k := 1; k < len(a); k++ {
		for m := 1; m < len(b); m++ {
			p, q, r, s := a[k-1], a[k], b[m-1], b[m]
			for axis := range 2 {
				o := 1 - axis
				if math.Abs(p[o]-q[o]) > 0.01 || math.Abs(r[o]-s[o]) > 0.01 || math.Abs(p[o]-r[o]) > 1 {
					continue
				}
				lo := math.Max(math.Min(p[axis], q[axis]), math.Min(r[axis], s[axis]))
				hi := math.Min(math.Max(p[axis], q[axis]), math.Max(r[axis], s[axis]))
				if hi-lo > 1 {
					return true
				}
			}
		}
	}
	return false
}

// cuts reports whether a part of a route at a right angle passes through a
// laneBox.
func cuts(b laneBox, p, q [2]float64) bool {
	if p[1] == q[1] {
		return p[1] > b.y1 && p[1] < b.y2 && math.Max(p[0], q[0]) > b.x1 && math.Min(p[0], q[0]) < b.x2
	}
	return p[0] > b.x1 && p[0] < b.x2 && math.Max(p[1], q[1]) > b.y1 && math.Min(p[1], q[1]) < b.y2
}
