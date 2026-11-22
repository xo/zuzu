// SPDX-License-Identifier: EPL-2.0

// Package elk lays out the diagrams of zuzu with ELK, the Eclipse Layout
// Kernel. It runs a copy of elk-go, the port of ELK.js 0.12.0 to Go by the D2
// team, which is the version of ELK that Mermaid 12.3 uses (D39 part 13).
//
// The package is the module github.com/xo/zuzu/elk, under the Eclipse Public
// License 2.0, apart from the module github.com/xo/zuzu, which stays under
// the MIT license. A program that imports it ships code under the EPL-2.0.
// The README.md of the module says what that means.
//
// A caller makes the engine available to zuzu with
// zuzu.WithLayoutEngine(elk.Layout()). zuzu then draws with ELK each graph
// of Mermaid whose layout is ELK: a graph that sets no layout, in a kind
// whose default layout in Mermaid 12.3 is ELK, and a graph with layout: elk
// or a layout whose name starts with elk., such as elk.stress (D45).
// zuzu.WithLayout("elk") forces ELK on every graph of a chart.
package elk

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/xo/zuzu/elk/internal/adapter"
	"github.com/xo/zuzu/graph"
)

// mermaidScale is the ratio of the pixels of Mermaid to the pixels of zuzu.
// Mermaid 12.3 measures its text at 16 pixels, and zuzu at 11. The adapter
// gives ELK the spaces of Mermaid in its pixels, so the engine lays out a
// graph scaled up by mermaidScale and scales the places back down. The
// spaces then keep the ratio to the nodes that they have in Mermaid.
const mermaidScale = 16.0 / 11

// Layout returns the layout engine of ELK, named elk (graph.EngineELK). The
// engine holds no state, so a caller can share it between charts and
// between goroutines.
//
// The engine takes the algorithm from graph.LayoutInput.Algorithm: elk, or
// empty, for the layered algorithm of ELK, or the name of another
// algorithm of ELK that Mermaid 12.3 has: elk.stress, elk.force,
// elk.mrtree, elk.sporeOverlap, elk.box or elk.rectpacking. It gives the
// root and each cluster the layout options of Mermaid.
//
// The engine names in graph.Layout.Unsupported the ports, the lanes, the
// groups of the same rank, the edges that take no part in the ranks, the
// least length and the weight of an edge, and an algorithm that ELK does
// not have, which lays out with the layered algorithm. It lays out each
// such part as though the input did not ask for it. It gives no rank and
// no order. The labels at the ends of the edges and the xlabels take no
// place in the layout, and graph.Finish places them.
func Layout() graph.Engine {
	return engine{}
}

// engine is the layout engine of ELK.
type engine struct{}

// Name satisfies graph.Engine. It returns elk, so that the transform of the
// engine is zuzu.elk (D39 part 13).
func (engine) Name() string {
	return graph.EngineELK
}

// Lay satisfies graph.Engine. It converts the input to the graph of the
// adapter, lays it out, converts the result back, and finishes it with
// graph.Finish.
func (engine) Lay(in graph.LayoutInput) (*graph.Layout, error) {
	c, err := newConversion(&in)
	if err != nil {
		return nil, err
	}
	r, err := adapter.Layout(c.g)
	if err != nil {
		return nil, fmt.Errorf("laying out with ELK: %w: %w", graph.ErrGraph, err)
	}
	out := c.result(r)
	graph.Finish(&in, out)
	out.Unsupported = slices.Concat(c.unsupported, r.Issues)
	return out, nil
}

// conversion holds the graph of the adapter that stands for an input, and
// the place of each part of the input in it.
type conversion struct {
	in *graph.LayoutInput
	g  *adapter.Graph
	// nodeAt is the index in g.Nodes of each node of the input, and
	// clusterAt of each cluster.
	nodeAt, clusterAt []int
	// depth is the depth of each cluster, from 0 for an outer one.
	depth       []int
	pad         float64
	unsupported []string
}

// unsupportedf names a part of the input that ELK does not lay out.
func (c *conversion) unsupportedf(format string, args ...any) {
	c.unsupported = append(c.unsupported, fmt.Sprintf(format, args...)+", which zuzu.elk leaves out")
}

// newConversion builds the graph of the adapter for an input. Its nodes and
// clusters are in one list, as Mermaid gives them to ELK: the clusters
// first, then the nodes, each in the order of the input. With
// LayoutInput.ReverseClusters, the sibling clusters are in the reverse of
// the order of the input, as Mermaid gives the subgraphs of a flowchart
// (D45 part 5). Each id of the adapter is new, so that no node, cluster or
// edge of the input can take the id of another, or the id root.
func newConversion(in *graph.LayoutInput) (*conversion, error) {
	s := mermaidScale
	c := &conversion{in: in, g: &adapter.Graph{Direction: in.Direction}, pad: in.ClusterPadding}
	if c.pad <= 0 {
		c.pad = 12
	}
	switch a := in.Algorithm; {
	case a == "" || a == graph.EngineELK:
		c.g.Algorithm = adapter.Layered
	case slices.Contains(adapter.Algorithms, a):
		c.g.Algorithm = a
	default:
		c.g.Algorithm = adapter.Layered
		c.unsupported = append(c.unsupported, fmt.Sprintf("the algorithm %q, which zuzu.elk does not have, drawn with elk.layered", a))
	}
	switch in.Direction {
	case "", graph.TB, graph.BT, graph.LR, graph.RL:
	default:
		return nil, fmt.Errorf("the direction %q: %w", in.Direction, graph.ErrGraph)
	}
	if in.Lanes {
		c.unsupportedf("the lanes of the layout")
	}
	nodes := make(map[string]int, len(in.Nodes))
	for i, n := range in.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return nil, fmt.Errorf("the node %q twice: %w", n.ID, graph.ErrGraph)
		}
		nodes[n.ID] = i
	}
	clusters := make(map[string]int, len(in.Clusters))
	for i, cl := range in.Clusters {
		clusters[cl.ID] = i
	}
	parent := make([]int, len(in.Clusters))
	for i, cl := range in.Clusters {
		parent[i] = -1
		if p, ok := clusters[cl.Parent]; ok && cl.Parent != "" && p != i {
			parent[i] = p
		}
	}
	c.depth = make([]int, len(in.Clusters))
	for i := range in.Clusters {
		for p := parent[i]; p >= 0; p = parent[p] {
			if c.depth[i]++; c.depth[i] > len(in.Clusters) {
				return nil, fmt.Errorf("the cluster %q is inside itself: %w", in.Clusters[i].ID, graph.ErrGraph)
			}
		}
	}
	clusterID := func(i int) string { return "c" + strconv.Itoa(i) }
	order := make([]int, len(in.Clusters))
	for i := range order {
		order[i] = i
	}
	if in.ReverseClusters {
		// The adapter keeps the order of the list among siblings, so the
		// reverse of the whole list reverses each list of siblings.
		slices.Reverse(order)
	}
	c.clusterAt = make([]int, len(in.Clusters))
	for _, i := range order {
		cl := in.Clusters[i]
		n := adapter.Node{
			ID: clusterID(i), Cluster: true, Padding: c.pad * s, Direction: cl.Direction,
		}
		if parent[i] >= 0 {
			n.Parent = clusterID(parent[i])
		}
		if cl.TitleWidth > 0 || cl.TitleHeight > 0 {
			n.Label = &adapter.Label{Text: "title", Width: cl.TitleWidth * s, Height: cl.TitleHeight * s}
		} else {
			n.NoTitle = true
		}
		c.clusterAt[i] = len(c.g.Nodes)
		c.g.Nodes = append(c.g.Nodes, n)
	}
	c.nodeAt = make([]int, len(in.Nodes))
	ranks := map[string]bool{}
	for i, n := range in.Nodes {
		an := adapter.Node{ID: "n" + strconv.Itoa(i), Width: n.Width * s, Height: n.Height * s}
		if p, ok := clusters[n.Cluster]; ok && n.Cluster != "" {
			an.Parent = clusterID(p)
		}
		if n.Rank != "" && !ranks[n.Rank] {
			ranks[n.Rank] = true
			c.unsupportedf("the group of the same rank %q", n.Rank)
		}
		c.nodeAt[i] = len(c.g.Nodes)
		c.g.Nodes = append(c.g.Nodes, an)
	}
	end := func(id string) (string, error) {
		if i, ok := nodes[id]; ok {
			return "n" + strconv.Itoa(i), nil
		}
		if i, ok := clusters[id]; ok {
			return clusterID(i), nil
		}
		return "", fmt.Errorf("an edge from or to %q, which is neither a node nor a cluster: %w", id, graph.ErrGraph)
	}
	for i, e := range in.Edges {
		from, err := end(e.Source)
		if err != nil {
			return nil, err
		}
		to, err := end(e.Target)
		if err != nil {
			return nil, err
		}
		ae := adapter.Edge{ID: "e" + strconv.Itoa(i), From: from, To: to}
		if e.LabelWidth > 0 {
			ae.Label = &adapter.Label{Text: "label", Width: e.LabelWidth * s, Height: e.LabelHeight * s}
		}
		c.g.Edges = append(c.g.Edges, ae)
		if e.SourcePort != nil || e.TargetPort != nil {
			c.unsupportedf("the port of the edge %s -> %s", e.Source, e.Target)
		}
		if e.NoConstraint {
			c.unsupportedf("constraint false on the edge %s -> %s", e.Source, e.Target)
		}
		if e.MinLen > 1 {
			c.unsupportedf("the length %d of the edge %s -> %s", e.MinLen, e.Source, e.Target)
		}
		if e.Weight != nil && *e.Weight != 1 {
			c.unsupportedf("the weight %g of the edge %s -> %s", *e.Weight, e.Source, e.Target)
		}
	}
	return c, nil
}

// result converts the layout of the adapter back to the input: the center
// of each node, the box and the title of each cluster, and the points and
// the center of the label of each edge, in the pixels of zuzu.
func (c *conversion) result(r *adapter.Result) *graph.Layout {
	in := c.in
	k := 1 / mermaidScale
	out := &graph.Layout{
		Nodes:    make([]graph.NodePlace, len(in.Nodes)),
		Edges:    make([]graph.EdgePlace, len(in.Edges)),
		Clusters: make([]graph.ClusterPlace, len(in.Clusters)),
	}
	for i, n := range in.Nodes {
		p := r.Nodes[c.nodeAt[i]]
		out.Nodes[i] = graph.NodePlace{X: (p.X + p.Width/2) * k, Y: (p.Y + p.Height/2) * k, Width: n.Width, Height: n.Height}
	}
	for i, cl := range in.Clusters {
		p := r.Nodes[c.clusterAt[i]]
		x, y := p.X*k, p.Y*k
		out.Clusters[i] = graph.ClusterPlace{
			X: x, Y: y, Width: p.Width * k, Height: p.Height * k, Depth: c.depth[i],
			TitleX: x + c.pad, TitleY: y + c.pad/2 + cl.TitleTop,
		}
	}
	for i, e := range in.Edges {
		p := r.Edges[i]
		ep := &out.Edges[i]
		for _, pt := range p.Points {
			ep.Points = append(ep.Points, [2]float64{pt.X * k, pt.Y * k})
		}
		if len(ep.Points) < 2 {
			// An algorithm that routes no edge, such as elk.box, leaves
			// the edge a line between the centers of its ends, which
			// graph.Finish cuts at their outlines.
			a, aok := c.center(e.Source, out)
			b, bok := c.center(e.Target, out)
			ep.Points = nil
			if aok && bok && a != b {
				ep.Points = [][2]float64{a, b}
			}
		}
		switch {
		case p.Label != nil:
			ep.LabelX, ep.LabelY = (p.Label.X+p.Label.Width/2)*k, (p.Label.Y+p.Label.Height/2)*k
		case len(ep.Points) >= 2:
			m := len(ep.Points) / 2
			ep.LabelX, ep.LabelY = (ep.Points[m-1][0]+ep.Points[m][0])/2, (ep.Points[m-1][1]+ep.Points[m][1])/2
		}
	}
	return out
}

// center returns the center of a node or a cluster of the input in a
// layout, by its id.
func (c *conversion) center(id string, out *graph.Layout) ([2]float64, bool) {
	for i, n := range c.in.Nodes {
		if n.ID == id {
			return [2]float64{out.Nodes[i].X, out.Nodes[i].Y}, true
		}
	}
	for i, cl := range c.in.Clusters {
		if cl.ID == id {
			p := out.Clusters[i]
			return [2]float64{p.X + p.Width/2, p.Y + p.Height/2}, true
		}
	}
	return [2]float64{}, false
}
