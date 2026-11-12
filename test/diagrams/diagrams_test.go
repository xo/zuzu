// Package diagrams checks the fixtures of DOT, and of the flowcharts, the
// state diagrams, the class diagrams, the entity relationship diagrams and
// the requirement diagrams of Mermaid, against the references of dot and
// of mmdc (D31). zuzu lays out a graph its own way, so the check does not
// ask for the places of the reference. It asks that every node, edge,
// cluster and label of the reference exists, that the hard rules of the
// layout hold, and that the crossings and the span of the edges are no
// worse than those of the reference, plus 10 percent and 1.
//
// Run oracle/diagrams.sh to write the references again.
package diagrams

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/dot"
	"github.com/xo/zuzu/graph/mermaid"
)

// ref is a reference in one form for both languages.
type ref struct {
	nodes    map[string]string // the label of each node
	xlabels  map[string]string // the xlabel of each node
	edges    []refEdge
	clusters map[string]string // the title of each cluster
	// rank holds the rank of each node, from its place across the ranks.
	rank map[string]int
	// routes are the routes of the edges, to count their crossings.
	routes [][][2]float64
}

type refEdge struct {
	source, target, label string
	// head, tail and x are the labels at the ends and the xlabel.
	head, tail, x string
}

func TestDOT(t *testing.T) {
	t.Parallel()
	files, _ := filepath.Glob("../testdata/diagrams/dot/*.dot")
	if len(files) < 30 {
		t.Fatalf("expected 30 fixtures or more, got %d", len(files))
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := dot.Parse(string(src), false)
			if err != nil {
				t.Fatal(err)
			}
			check(t, g, readDOT(t, strings.TrimSuffix(f, ".dot")+".json", g.Direction))
		})
	}
}

func TestMermaid(t *testing.T) {
	t.Parallel()
	files, _ := filepath.Glob("../testdata/diagrams/mermaid/*.mmd")
	if len(files) < 30 {
		t.Fatalf("expected 30 fixtures or more, got %d", len(files))
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			d, err := mermaid.Read(string(src))
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := mermaid.Graph(d, false)
			if err != nil {
				t.Fatal(err)
			}
			check(t, g, readMermaid(t, strings.TrimSuffix(f, ".mmd")+".json", g.Direction))
		})
	}
}

// readDOT reads the JSON of dot -Tjson0.
func readDOT(t *testing.T, path, dir string) *ref {
	t.Helper()
	var doc struct {
		Objects []struct {
			Name   string `json:"name"`
			Label  string `json:"label"`
			XLabel string `json:"xlabel"`
			Pos    string `json:"pos"`
			Nodes  []int  `json:"nodes"`
		} `json:"objects"`
		Edges []struct {
			Tail      int    `json:"tail"`
			Head      int    `json:"head"`
			Label     string `json:"label"`
			HeadLabel string `json:"headlabel"`
			TailLabel string `json:"taillabel"`
			XLabel    string `json:"xlabel"`
			Pos       string `json:"pos"`
		} `json:"edges"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	r := &ref{nodes: map[string]string{}, xlabels: map[string]string{}, clusters: map[string]string{}}
	across := map[string]float64{}
	for _, o := range doc.Objects {
		switch {
		case o.Nodes != nil && strings.HasPrefix(o.Name, "cluster"):
			r.clusters[o.Name] = o.Label
		case o.Nodes == nil && o.Pos != "":
			label := o.Label
			if label == `\N` {
				label = o.Name
			}
			r.nodes[o.Name] = label
			if o.XLabel != "" {
				r.xlabels[o.Name] = o.XLabel
			}
			x, y := point(o.Pos)
			across[o.Name] = pick(dir, x, y)
		}
	}
	for _, e := range doc.Edges {
		re := refEdge{doc.Objects[e.Tail].Name, doc.Objects[e.Head].Name, e.Label, e.HeadLabel, e.TailLabel, e.XLabel}
		r.edges = append(r.edges, re)
		var pts [][2]float64
		for p := range strings.FieldsSeq(e.Pos) {
			if strings.HasPrefix(p, "e,") || strings.HasPrefix(p, "s,") {
				continue
			}
			x, y := point(p)
			pts = append(pts, [2]float64{x, y})
		}
		r.routes = append(r.routes, pts)
	}
	r.rank = ranks(across)
	return r
}

// readMermaid reads the reference that cmd/mmdcref wrote.
func readMermaid(t *testing.T, path, dir string) *ref {
	t.Helper()
	var doc struct {
		Nodes []struct {
			ID    string  `json:"id"`
			Label string  `json:"label"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
		} `json:"nodes"`
		Edges []struct {
			Source string       `json:"source"`
			Target string       `json:"target"`
			Label  string       `json:"label"`
			Head   string       `json:"head"`
			Tail   string       `json:"tail"`
			Points [][2]float64 `json:"points"`
		} `json:"edges"`
		Clusters []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"clusters"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	r := &ref{nodes: map[string]string{}, clusters: map[string]string{}}
	across := map[string]float64{}
	for _, n := range doc.Nodes {
		r.nodes[n.ID] = n.Label
		across[n.ID] = pick(dir, n.X, n.Y)
	}
	for _, c := range doc.Clusters {
		r.clusters[c.ID] = c.Label
	}
	for _, e := range doc.Edges {
		r.edges = append(r.edges, refEdge{source: e.Source, target: e.Target, label: e.Label, head: e.Head, tail: e.Tail})
		r.routes = append(r.routes, e.Points)
	}
	r.rank = ranks(across)
	return r
}

// point reads x,y.
func point(s string) (float64, float64) {
	a, b, _ := strings.Cut(s, ",")
	x, _ := strconv.ParseFloat(a, 64)
	y, _ := strconv.ParseFloat(strings.TrimSuffix(b, "!"), 64)
	return x, y
}

// pick returns the coordinate along the ranks.
func pick(dir string, x, y float64) float64 {
	switch dir {
	case graph.LR, graph.RL:
		return x
	}
	return y
}

// ranks numbers the distinct places along the ranks, from 0, with places
// closer than 2 pixels as one.
func ranks(across map[string]float64) map[string]int {
	var vals []float64
	for _, v := range across {
		vals = append(vals, v)
	}
	slices.Sort(vals)
	var levels []float64
	for _, v := range vals {
		if len(levels) == 0 || v-levels[len(levels)-1] > 2 {
			levels = append(levels, v)
		}
	}
	out := map[string]int{}
	for id, v := range across {
		i, _ := slices.BinarySearchFunc(levels, v-2, func(a, b float64) int {
			switch {
			case a < b:
				return -1
			case a > b:
				return 1
			}
			return 0
		})
		out[id] = i
	}
	return out
}

// norm makes a label comparable: its words, joined by one space.
func norm(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, `\n`, " ")), " ")
}

// source returns the text of a label of the model as the reference writes
// it: the source of a label of HTML, or else its text.
func source(text, html string) string {
	if html != "" {
		return html
	}
	return text
}

// check runs the checks of D31 on a graph against its reference.
func check(t *testing.T, g *graph.Graph, r *ref) {
	t.Helper()
	nodes := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	clusters := map[string]*graph.Cluster{}
	for _, c := range g.Clusters {
		clusters[c.ID] = c
	}
	// 1. Every node, edge, cluster and label of the reference exists.
	for id, label := range r.nodes {
		n, ok := nodes[id]
		if !ok {
			t.Errorf("no node %s", id)
			continue
		}
		if len(n.Fields) == 0 && n.HTML == "" && norm(label) != "" && norm(n.Label) != norm(label) {
			t.Errorf("node %s: expected the label %q, got %q", id, norm(label), norm(n.Label))
		}
		if x := source(n.XLabel, n.XHTML); norm(x) != norm(r.xlabels[id]) {
			t.Errorf("node %s: expected the xlabel %q, got %q", id, norm(r.xlabels[id]), norm(x))
		}
	}
	for id, label := range r.clusters {
		c, ok := clusters[id]
		if !ok {
			t.Errorf("no cluster %s", id)
			continue
		}
		if norm(source(c.Label, c.HTML)) != norm(label) {
			t.Errorf("cluster %s: expected the title %q, got %q", id, norm(label), norm(source(c.Label, c.HTML)))
		}
	}
	member := func(id, cluster string) bool {
		n, ok := nodes[id]
		if !ok {
			return false
		}
		for c := n.Cluster; c != ""; c = clusters[c].Parent {
			if c == cluster {
				return true
			}
		}
		return false
	}
	matches := func(end, want string) bool {
		if end == want {
			return true
		}
		_, isCluster := clusters[want]
		return isCluster && member(end, want)
	}
	used := make([]bool, len(g.Edges))
	for _, re := range r.edges {
		found := false
		for i, e := range g.Edges {
			fwd := matches(e.Source, re.source) && matches(e.Target, re.target)
			back := !g.Directed && matches(e.Source, re.target) && matches(e.Target, re.source)
			if used[i] || !fwd && !back || norm(source(e.Label, e.HTML)) != norm(re.label) {
				continue
			}
			if norm(source(e.HeadLabel, e.HeadHTML)) != norm(re.head) || norm(source(e.TailLabel, e.TailHTML)) != norm(re.tail) ||
				norm(source(e.XLabel, e.XHTML)) != norm(re.x) {
				continue
			}
			used[i], found = true, true
			break
		}
		if !found {
			t.Errorf("no edge %s -> %s %q, with the labels %q, %q and %q", re.source, re.target, re.label, re.head, re.tail, re.x)
		}
	}
	// 2. The hard rules of the layout hold.
	in, ids := input(g)
	l, err := graph.Lay(in)
	if err != nil {
		t.Fatal(err)
	}
	rules(t, in, l, ids)
	// 3. The crossings and the span are no worse than the reference.
	got := graph.Crossings(l)
	refLayout := &graph.Layout{}
	for _, pts := range r.routes {
		refLayout.Edges = append(refLayout.Edges, graph.EdgePlace{Points: pts})
	}
	want := graph.Crossings(refLayout)
	if float64(got) > float64(want)*1.1+1 {
		t.Errorf("%d crossings, and the reference has %d", got, want)
	}
	across := map[string]float64{}
	for i, n := range g.Nodes {
		across[n.ID] = float64(l.Nodes[i].Rank)
	}
	ours := ranks(across)
	span := func(rank map[string]int) int {
		s := 0
		for _, e := range g.Edges {
			a, aok := rank[e.Source]
			b, bok := rank[e.Target]
			if aok && bok {
				s += int(math.Abs(float64(b - a)))
			}
		}
		return s
	}
	if s, w := span(ours), span(r.rank); float64(s) > float64(w)*1.1+1 {
		t.Errorf("the edges span %d ranks, and those of the reference %d", s, w)
	}
}

// input builds the input of the layout of a graph, with the sizes of the
// nodes from the length of their labels.
func input(g *graph.Graph) (graph.LayoutInput, map[string]int) {
	in := graph.LayoutInput{Direction: g.Direction, RankSep: g.RankSep, NodeSep: g.NodeSep, ClusterPadding: 12}
	if in.Direction == "" {
		in.Direction = graph.TB
	}
	ids := map[string]int{}
	size := func(s string) (float64, float64) {
		lines := strings.Split(s, "\n")
		w := 0
		for _, l := range lines {
			w = max(w, len([]rune(l)))
		}
		return 6.5 * float64(w), 14 * float64(len(lines))
	}
	for i, n := range g.Nodes {
		ids[n.ID] = i
		kind, ok := graph.ShapeOf(n.Shape)
		if !ok {
			kind = "rounded"
		}
		tw, th := size(n.Label)
		w, h := graph.NodeSize(kind, tw, th, 8)
		in.Nodes = append(in.Nodes, graph.LayoutNode{ID: n.ID, Cluster: n.Cluster, Rank: n.Rank, Shape: kind, Width: w, Height: h})
	}
	for _, e := range g.Edges {
		le := graph.LayoutEdge{Source: e.Source, Target: e.Target, MinLen: e.MinLen, Weight: e.Weight, NoConstraint: e.NoConstraint}
		if e.Label != "" {
			le.LabelWidth, le.LabelHeight = size(e.Label)
		}
		in.Edges = append(in.Edges, le)
	}
	for _, c := range g.Clusters {
		in.Clusters = append(in.Clusters, graph.LayoutCluster{ID: c.ID, Parent: c.Parent, Direction: c.Direction, TitleWidth: 6.5 * float64(len(c.Label)), TitleHeight: 16})
	}
	return in, ids
}

// rules checks the hard rules of D31: an edge that keeps its direction
// goes from a lower rank to a higher one by its minlen at least, the nodes
// of a group of the same rank share it, no two nodes overlap, and each
// cluster holds its members and its nested clusters.
func rules(t *testing.T, in graph.LayoutInput, l *graph.Layout, ids map[string]int) {
	t.Helper()
	// Ranks double when an edge has a label, for the dummy of the label.
	factor := 1
	for _, e := range in.Edges {
		if e.LabelWidth > 0 && e.Source != e.Target {
			factor = 2
		}
	}
	reach := func(from, to int) bool {
		seen := map[int]bool{from: true}
		stack := []int{from}
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if v == to {
				return true
			}
			for _, e := range in.Edges {
				if ids[e.Source] == v && !seen[ids[e.Target]] {
					seen[ids[e.Target]] = true
					stack = append(stack, ids[e.Target])
				}
			}
		}
		return false
	}
	// A cluster with a direction can lay out on its own, with ranks of its
	// own, so an edge inside it spans its minlen without the factor.
	parent, dir := map[string]string{}, map[string]string{}
	for _, c := range in.Clusters {
		parent[c.ID], dir[c.ID] = c.Parent, c.Direction
	}
	own := func(i int) string {
		for c := in.Nodes[i].Cluster; c != ""; c = parent[c] {
			if dir[c] != "" {
				return c
			}
		}
		return ""
	}
	for _, e := range in.Edges {
		a, aok := ids[e.Source]
		b, bok := ids[e.Target]
		// An edge to a cluster has no rank to check.
		if e.NoConstraint || a == b || !aok || !bok {
			continue
		}
		// An edge between two nodes of one group of the same rank spans no
		// rank.
		if r := in.Nodes[a].Rank; r != "" && r == in.Nodes[b].Rank {
			continue
		}
		minlen := max(e.MinLen, 1) * factor
		if c := own(a); c != "" && c == own(b) {
			minlen = max(e.MinLen, 1)
		}
		if l.Nodes[b].Rank-l.Nodes[a].Rank < minlen && !reach(b, a) {
			t.Errorf("edge %s -> %s spans %d ranks, less than %d, and is on no cycle", e.Source, e.Target, l.Nodes[b].Rank-l.Nodes[a].Rank, minlen)
		}
	}
	groups := map[string]int{}
	for i, n := range in.Nodes {
		if n.Rank == "" {
			continue
		}
		if r, ok := groups[n.Rank]; ok && r != l.Nodes[i].Rank {
			t.Errorf("node %s is not in the rank of its group %s", n.ID, n.Rank)
		}
		groups[n.Rank] = l.Nodes[i].Rank
	}
	for i := range l.Nodes {
		for j := range i {
			a, b := l.Nodes[i], l.Nodes[j]
			if math.Abs(a.X-b.X) < (a.Width+b.Width)/2-0.5 && math.Abs(a.Y-b.Y) < (a.Height+b.Height)/2-0.5 {
				t.Errorf("nodes %s and %s overlap", in.Nodes[i].ID, in.Nodes[j].ID)
			}
		}
	}
	box := map[string]graph.ClusterPlace{}
	for i, c := range in.Clusters {
		box[c.ID] = l.Clusters[i]
	}
	inside := func(c graph.ClusterPlace, x0, y0, x1, y1 float64) bool {
		return x0 >= c.X-0.5 && y0 >= c.Y-0.5 && x1 <= c.X+c.Width+0.5 && y1 <= c.Y+c.Height+0.5
	}
	for i, n := range in.Nodes {
		if n.Cluster == "" {
			continue
		}
		p := l.Nodes[i]
		if c := box[n.Cluster]; c.Width > 0 && !inside(c, p.X-p.Width/2, p.Y-p.Height/2, p.X+p.Width/2, p.Y+p.Height/2) {
			t.Errorf("node %s is outside its cluster %s", n.ID, n.Cluster)
		}
	}
	for _, c := range in.Clusters {
		inner, outer := box[c.ID], box[c.Parent]
		if c.Parent != "" && inner.Width > 0 && outer.Width > 0 && !inside(outer, inner.X, inner.Y, inner.X+inner.Width, inner.Y+inner.Height) {
			t.Errorf("cluster %s is outside its cluster %s", c.ID, c.Parent)
		}
	}
}
