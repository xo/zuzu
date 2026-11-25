// SPDX-License-Identifier: EPL-2.0

// Package adapter lays out a graph that is described in plain Go with the
// copy of elk-go in internal/elkgo. It builds the graph of ELK with the
// layout options that Mermaid 12.3 gives in @mermaid-js/layout-elk, runs
// the layout, and returns the places of the nodes, the clusters, the ports,
// the edges and the labels in the coordinates of the whole graph.
//
// The package knows nothing of the package graph of zuzu. The package elk
// above it binds it to the interface of a layout engine (D39 part 10).
package adapter

import (
	"fmt"
	"slices"
	"strconv"

	elk "github.com/xo/zuzu/elk/internal/elkgo"
)

// Error is an error of the package adapter.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

const (
	// ErrGraph is the error of a graph that does not lay out, such as an
	// edge to a node that does not exist or two nodes with one ID.
	ErrGraph Error = "invalid graph"

	// ErrAlgorithm is the error of an algorithm of ELK that the adapter does
	// not know.
	ErrAlgorithm Error = "unknown ELK algorithm"
)

// The algorithms of ELK that Mermaid names. Layered is the algorithm of
// `layout: elk`, and each other one is the algorithm of the layout of the
// same name. A cluster can also take Radial.
const (
	Layered      = "elk.layered"
	Stress       = "elk.stress"
	Force        = "elk.force"
	MrTree       = "elk.mrtree"
	SporeOverlap = "elk.sporeOverlap"
	Box          = "elk.box"
	RectPacking  = "elk.rectpacking"
	Radial       = "elk.radial"
)

// Algorithms lists the algorithms that a graph can take, in the order of
// Mermaid: layered first, then the algorithms of the layouts of
// @mermaid-js/layout-elk.
var Algorithms = []string{Layered, Stress, Force, MrTree, SporeOverlap, Box, RectPacking}

// The sides of a node that a port can take, with the names of ELK.
const (
	North = "NORTH"
	South = "SOUTH"
	East  = "EAST"
	West  = "WEST"
)

// Graph is a graph to lay out. Its nodes and its clusters are in one list,
// in the order of the source, as Mermaid holds them. The order matters,
// because ELK keeps the order of the model where it can.
type Graph struct {
	// Algorithm is one of Algorithms. Empty is Layered.
	Algorithm string
	// Direction is TB, TD, BT, LR or RL. Empty is TB.
	Direction string
	// Config is the configuration `elk` of Mermaid.
	Config Config
	// Options holds more layout options of ELK for the root, which win over
	// the options above, as the rootLayoutOptions of Mermaid do.
	Options map[string]string

	Nodes []Node
	Edges []Edge
}

// Config holds the settings of the configuration `elk` of Mermaid 12.3 that
// change the layout. A zero field takes the default of Mermaid.
type Config struct {
	// Preset is default, legacy, modelOrder or depthFirst. It gives each
	// strategy that the fields below leave empty. Another name is default.
	Preset string
	// NodePlacementStrategy is the strategy of the placement of the nodes,
	// such as BRANDES_KOEPF, NETWORK_SIMPLEX, LINEAR_SEGMENTS or SIMPLE.
	NodePlacementStrategy string
	// NodePlacementAlignment is the alignment of BRANDES_KOEPF, such as
	// BALANCED or NONE.
	NodePlacementAlignment string
	// CycleBreakingStrategy is the strategy that breaks the cycles, such as
	// GREEDY, DEPTH_FIRST or GREEDY_MODEL_ORDER.
	CycleBreakingStrategy string
	// LayeringStrategy is the strategy of the layers, such as
	// NETWORK_SIMPLEX, LONGEST_PATH or COFFMAN_GRAHAM.
	LayeringStrategy string
	// LayeringLayerBound is the bound of COFFMAN_GRAHAM. Zero is 4.
	LayeringLayerBound int
	// MergeEdges merges the edges that enter or leave a node at one point.
	MergeEdges bool
	// ForceNodeModelOrder keeps the nodes of a layer in the order of the
	// model.
	ForceNodeModelOrder bool
	// ConsiderModelOrder is the strategy of the order of the model, such as
	// NONE, NODES_AND_EDGES or PREFER_EDGES. Empty is NODES_AND_EDGES.
	ConsiderModelOrder string
	// KeepEntryNodeOnTop pins the entry of each cycle to the first layer,
	// as keepEntryNodeOnTop of Mermaid does. The entry is the first node
	// of a group with no source, in the order of the graph, that no edge
	// enters once the edges that close a cycle are left out.
	KeepEntryNodeOnTop bool
	// KeepFeedbackEdges gives ELK each edge in its own direction, as
	// orientFeedbackEdges false of Mermaid does. By default, an edge that
	// closes a cycle only through a cluster goes to ELK reversed, so that
	// ELK routes it downstream and not around the cluster. Its route still
	// runs from its start to its end.
	KeepFeedbackEdges bool
}

// Node is a node or a cluster.
type Node struct {
	// ID names the node. It is unique among the nodes, the ports and the
	// edges, and it is not root.
	ID string
	// Parent is the ID of the cluster that holds the node, or empty.
	Parent string
	// Width and Height are the size of a node. A cluster takes its size
	// from its children.
	Width, Height float64
	// Ports are the ports of a node, in their order.
	Ports []Port
	// SpreadPorts spreads the points where the edges meet the node along
	// each side, as the spreadPorts of Mermaid does.
	SpreadPorts bool

	// Cluster marks a cluster. The nodes that name it as Parent are inside
	// it.
	Cluster bool
	// Label is the title of a cluster.
	Label *Label
	// Padding is the padding of a cluster, the padding of Mermaid.
	Padding float64
	// NoTitle marks a cluster whose frame draws no title, so that the title
	// does not widen it.
	NoTitle bool
	// Direction is the direction of a cluster that lays out on its own,
	// or empty.
	Direction string
	// Algorithm is the algorithm of a cluster that lays out on its own, one
	// of Algorithms or Radial, or empty.
	Algorithm string
}

// Port is a point of a node where an edge ends.
type Port struct {
	// ID names the port, unique among the nodes, the ports and the edges.
	ID string
	// Side is North, South, East or West, or empty for any side.
	Side string
	// Width and Height are the size of the port.
	Width, Height float64
	// At is the place of the top left of the port from the top left of
	// its node, or nil to let ELK place it. A port with a place fixes the
	// place of each port of its node, and a port of the node with no place
	// takes the top left of the node.
	At *Point
}

// Label is the box of a label. ELK lays out only a label with a text, so
// Text is empty only for a label that takes no place.
type Label struct {
	Text          string
	Width, Height float64
}

// Edge is an edge from a node, a cluster or a port to another.
type Edge struct {
	// ID names the edge, unique among the nodes, the ports and the edges.
	// Empty is e and the index of the edge.
	ID string
	// From and To are the IDs of the ends, each a node or a cluster.
	From, To string
	// FromPort and ToPort are the IDs of the ports at the ends, or empty.
	FromPort, ToPort string
	// Label is the label in the middle of the edge, or nil.
	Label *Label
	// Tail and Head are the labels at the start and at the end of the edge.
	// Only Layered places them. Mermaid draws the cardinality of a relation
	// with them.
	Tail, Head []Label
	// TailMarker and HeadMarker say that the edge draws a marker, such as
	// an arrowhead, at its start and at its end. ELK does not know of the
	// markers, so a label at such an end keeps room for the marker around
	// it, as Mermaid does.
	TailMarker, HeadMarker bool
}

// Point is a point in the coordinates of the whole graph.
type Point struct {
	X, Y float64
}

// Rect is a box in the coordinates of the whole graph.
type Rect struct {
	X, Y, Width, Height float64
}

// Result is the layout of a graph. Each list is in the order of the graph.
type Result struct {
	// Width and Height are the size of the whole graph.
	Width, Height float64
	Nodes         []NodePlace
	Edges         []EdgePlace
	// Issues names each part of the graph that ELK does not lay out and
	// that the layout leaves out (AGENTS.md, hard rule 4).
	Issues []string
}

// NodePlace is the place of a node or of a cluster.
type NodePlace struct {
	Rect

	// Label is the place of the title of a cluster, or nil.
	Label *Rect
	// Ports are the places of the ports, in their order.
	Ports []Rect
}

// EdgePlace is the route of an edge.
type EdgePlace struct {
	// Points runs from the start of the edge to its end, with each bend.
	// It is empty when the algorithm does not route the edges, as Box and
	// RectPacking do not.
	Points []Point
	// Label is the place of the label in the middle, or nil.
	Label *Rect
	// Tail and Head are the places of the end labels. An entry is nil when
	// ELK did not place the label.
	Tail, Head []*Rect
}

// Layout lays out a graph with elk-go.
func Layout(g *Graph) (*Result, error) {
	b, err := build(g, false)
	if err != nil {
		return nil, err
	}
	if err := elk.Layout(b.root); err != nil {
		return nil, fmt.Errorf("elk: %w", err)
	}
	return b.result(g), nil
}

// builder holds the graph of ELK while it is built, and the place of each
// part of the input in it.
type builder struct {
	root  *elk.Graph
	nodes []*elk.Node
	edges []*elk.Edge
	// reversed marks each edge that goes to ELK reversed.
	reversed []bool
	issues   []string
	layered  bool
	clusters map[string]*clusterState
	// mermaidTitles gives ELK the title of each cluster as a label, as
	// Mermaid does, in place of reserveTitles. Only a test sets it, to write
	// the graph that Mermaid gives ELK.js.
	mermaidTitles bool
}

// clusterState is what the cross-hierarchy pass of Mermaid reads and
// changes on a node of ELK.
type clusterState struct {
	node *elk.Node
	// own marks a cluster with an algorithm of its own.
	own bool
	// title is the title of the cluster.
	title Label
	// pad is the padding of a cluster that runs the layered algorithm.
	pad float64
	// minWidth is the width that the frame of the cluster needs for its
	// title, as groupTitleWidth of Mermaid gives it.
	minWidth float64
	// noTitle marks a cluster whose frame draws no title.
	noTitle bool
}

// build makes the graph of ELK, as buildElkGraphFromLayoutData of Mermaid
// does, up to the passes that this adapter leaves to the caller.
func build(g *Graph, mermaidTitles bool) (*builder, error) {
	algorithm := g.Algorithm
	if algorithm == "" {
		algorithm = Layered
	}
	if !known(algorithm, false) {
		return nil, fmt.Errorf("%w: %q", ErrAlgorithm, algorithm)
	}
	b := &builder{clusters: map[string]*clusterState{}, mermaidTitles: mermaidTitles}
	b.root = &elk.Graph{
		ID:            elk.StringID("root"),
		LayoutOptions: rootOptions(g, algorithm),
		Children:      []*elk.Node{},
		Edges:         []*elk.Edge{},
	}
	rootAlgorithm := algorithm
	if v, ok := g.Options["elk.algorithm"]; ok {
		rootAlgorithm = v
	}
	b.layered = rootAlgorithm == Layered

	if err := b.checkIDs(g); err != nil {
		return nil, err
	}
	b.nodes = make([]*elk.Node, len(g.Nodes))
	byID := map[string]int{}
	for i := range g.Nodes {
		byID[g.Nodes[i].ID] = i
	}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Parent == "" {
			continue
		}
		p, ok := byID[n.Parent]
		if !ok || !g.Nodes[p].Cluster {
			return nil, fmt.Errorf("%w: node %q is in %q, which is not a cluster", ErrGraph, n.ID, n.Parent)
		}
	}
	b.add(g, "", &b.root.Children, 0)
	for i, n := range b.nodes {
		if n == nil {
			return nil, fmt.Errorf("%w: node %q is in a loop of clusters", ErrGraph, g.Nodes[i].ID)
		}
	}
	for i := range g.Nodes {
		if g.Nodes[i].Cluster {
			b.configureCluster(g, &g.Nodes[i], algorithm)
		}
	}
	t := newTree(g, byID)
	es := make([]ends, len(g.Edges))
	for i := range g.Edges {
		from, fok := byID[g.Edges[i].From]
		to, tok := byID[g.Edges[i].To]
		if !fok || !tok {
			return nil, fmt.Errorf("%w: an end of the edge %q is not a node", ErrGraph, edgeID(g, i))
		}
		es[i] = ends{from, to}
	}
	b.reversed = make([]bool, len(g.Edges))
	if !g.Config.KeepFeedbackEdges {
		b.reversed = feedbackEdges(es, t)
	}
	if err := b.addEdges(g, byID); err != nil {
		return nil, err
	}
	b.crossHierarchy(g, byID)
	// ELK takes the edges in the order of the nesting of their ends, as
	// they are after the reverse, as orderEdgesByNesting of Mermaid gives
	// them.
	turned := slices.Clone(es)
	for i, r := range b.reversed {
		if r {
			turned[i] = ends{es[i].to, es[i].from}
		}
	}
	b.root.Edges = b.root.Edges[:0]
	for _, i := range nestedOrder(turned, t) {
		b.root.Edges = append(b.root.Edges, b.edges[i])
	}
	if g.Config.KeepEntryNodeOnTop {
		for _, i := range cyclicEntries(len(g.Nodes), es, t) {
			setOption(&b.nodes[i].LayoutOptions, "elk.layered.layering.layerConstraint", "FIRST")
		}
	}
	b.reserveTitles()
	return b, nil
}

// checkIDs makes sure that each ID of a node, a port and an edge is unique,
// and that no ID is root, the ID of the graph.
func (b *builder) checkIDs(g *Graph) error {
	seen := map[string]bool{"root": true}
	use := func(id, what string) error {
		if id == "" {
			return fmt.Errorf("%w: a %s has no ID", ErrGraph, what)
		}
		if seen[id] {
			return fmt.Errorf("%w: the ID %q of a %s is not unique", ErrGraph, id, what)
		}
		seen[id] = true
		return nil
	}
	for i := range g.Nodes {
		if err := use(g.Nodes[i].ID, "node"); err != nil {
			return err
		}
		for _, p := range g.Nodes[i].Ports {
			if err := use(p.ID, "port"); err != nil {
				return err
			}
		}
	}
	for i := range g.Edges {
		if err := use(edgeID(g, i), "edge"); err != nil {
			return err
		}
	}
	return nil
}

// edgeID returns the ID of edge i.
func edgeID(g *Graph, i int) string {
	if id := g.Edges[i].ID; id != "" {
		return id
	}
	return "e" + strconv.Itoa(i)
}

// add adds the nodes of a parent in their order, and the children of each
// cluster below it, as addVertices of Mermaid does. depth stops a loop of
// clusters.
func (b *builder) add(g *Graph, parent string, into *[]*elk.Node, depth int) {
	if depth > len(g.Nodes) {
		return
	}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Parent != parent {
			continue
		}
		en := &elk.Node{ID: elk.StringID(n.ID)}
		b.nodes[i] = en
		*into = append(*into, en)
		if n.Cluster {
			en.Children = []*elk.Node{}
			b.clusters[n.ID] = &clusterState{node: en}
			b.add(g, n.ID, &en.Children, depth+1)
			continue
		}
		en.Width, en.Height = elk.Float64(n.Width), elk.Float64(n.Height)
		if n.SpreadPorts {
			setOption(&en.LayoutOptions, "elk.portAlignment.default", "CENTER")
		}
		addPorts(en, n.Ports)
	}
}

// addPorts adds the ports of a node. A port with a place fixes the place of
// each port of its node, and else a port with a side fixes the side of each
// port of its node.
func addPorts(en *elk.Node, ports []Port) {
	sided, placed := false, false
	for _, p := range ports {
		ep := &elk.Port{ID: elk.StringID(p.ID), Width: elk.Float64(p.Width), Height: elk.Float64(p.Height)}
		if p.Side != "" {
			sided = true
			ep.LayoutOptions = elk.LayoutOptions{"elk.port.side": p.Side}
		}
		if p.At != nil {
			placed = true
			ep.X, ep.Y = elk.Float64(p.At.X), elk.Float64(p.At.Y)
		}
		en.Ports = append(en.Ports, ep)
	}
	switch {
	case placed:
		for _, ep := range en.Ports {
			if ep.X == nil {
				ep.X, ep.Y = elk.Float64(0), elk.Float64(0)
			}
		}
		setOption(&en.LayoutOptions, "elk.portConstraints", "FIXED_POS")
	case sided:
		setOption(&en.LayoutOptions, "elk.portConstraints", "FIXED_SIDE")
	}
}

// addEdges adds each edge to the root, as addEdgesToElkGraph of Mermaid
// does. ELK finds the cluster that holds an edge from its ends.
func (b *builder) addEdges(g *Graph, byID map[string]int) error {
	ports := map[string]string{}
	for i := range g.Nodes {
		for _, p := range g.Nodes[i].Ports {
			ports[p.ID] = g.Nodes[i].ID
		}
	}
	end := func(node, port, what string) (string, error) {
		if _, ok := byID[node]; !ok {
			return "", fmt.Errorf("%w: the %s %q of an edge is not a node", ErrGraph, what, node)
		}
		if port == "" {
			return node, nil
		}
		if ports[port] != node {
			return "", fmt.Errorf("%w: the port %q of an edge is not a port of %q", ErrGraph, port, node)
		}
		return port, nil
	}
	b.edges = make([]*elk.Edge, len(g.Edges))
	for i := range g.Edges {
		e := &g.Edges[i]
		from, err := end(e.From, e.FromPort, "start")
		if err != nil {
			return err
		}
		to, err := end(e.To, e.ToPort, "end")
		if err != nil {
			return err
		}
		tail, head := "TAIL", "HEAD"
		if b.reversed[i] {
			// ELK lays the edge out from its end to its start, so the
			// labels at its ends swap their places too.
			from, to = to, from
			tail, head = head, tail
		}
		ee := &elk.Edge{
			ID:      elk.StringID(edgeID(g, i)),
			Sources: []elk.ID{elk.StringID(from)},
			Targets: []elk.ID{elk.StringID(to)},
		}
		center := Label{}
		if e.Label != nil {
			center = *e.Label
		}
		ee.Labels = append(ee.Labels, &elk.Label{
			Text:   elk.String(center.Text),
			Width:  elk.Float64(center.Width),
			Height: elk.Float64(center.Height),
			LayoutOptions: elk.LayoutOptions{
				"edgeLabels.inline":    "true",
				"edgeLabels.placement": "CENTER",
			},
		})
		if len(e.Tail)+len(e.Head) > 0 && !b.layered {
			b.issues = append(b.issues, fmt.Sprintf(
				"the end labels of the edge %q need %s, so the layout leaves them out", edgeID(g, i), Layered))
		}
		if b.layered {
			for _, l := range e.Tail {
				ee.Labels = append(ee.Labels, endLabel(l, tail, markerPad(e.TailMarker)))
			}
			for _, l := range e.Head {
				ee.Labels = append(ee.Labels, endLabel(l, head, markerPad(e.HeadMarker)))
			}
		}
		b.edges[i] = ee
		b.root.Edges = append(b.root.Edges, ee)
	}
	return nil
}

// markerClearance is half the width of the markers of a relation of
// Mermaid, which a label at an end with a marker keeps around it, the
// TERMINAL_LABEL_MARKER_CLEARANCE of render.ts.
const markerClearance = 6

// markerPad returns the room that a label at an end of an edge keeps on
// each side, for a marker at that end or for none.
func markerPad(marker bool) float64 {
	if marker {
		return markerClearance
	}
	return 0
}

// endLabel returns the label of ELK for a label at an end of an edge, with
// pad more on each side, as terminalElkLabels of Mermaid gives it.
func endLabel(l Label, placement string, pad float64) *elk.Label {
	return &elk.Label{
		Text:          elk.String(l.Text),
		Width:         elk.Float64(l.Width + 2*pad),
		Height:        elk.Float64(l.Height + 2*pad),
		LayoutOptions: elk.LayoutOptions{"edgeLabels.placement": placement},
	}
}

// known reports whether an algorithm is one of Algorithms, or Radial for a
// cluster.
func known(algorithm string, cluster bool) bool {
	return slices.Contains(Algorithms, algorithm) || cluster && algorithm == Radial
}

// setOption sets a layout option of ELK, and makes the map if it is nil.
func setOption(opts *elk.LayoutOptions, key, value string) {
	if *opts == nil {
		*opts = elk.LayoutOptions{}
	}
	(*opts)[key] = value
}

// result reads the layout back from the graph of ELK.
func (b *builder) result(g *Graph) *Result {
	r := &Result{
		Width:  num(b.root.Width),
		Height: num(b.root.Height),
		Nodes:  make([]NodePlace, len(g.Nodes)),
		Edges:  make([]EdgePlace, len(g.Edges)),
		Issues: b.issues,
	}
	for i, en := range b.nodes {
		p := NodePlace{Rect: rect(en.X, en.Y, en.Width, en.Height)}
		if g.Nodes[i].Cluster {
			p.Label = titleRect(b.clusters[g.Nodes[i].ID], p.Rect)
		}
		for _, ep := range en.Ports {
			p.Ports = append(p.Ports, rect(ep.X, ep.Y, ep.Width, ep.Height))
		}
		r.Nodes[i] = p
	}
	for i, ee := range b.edges {
		var p EdgePlace
		for _, s := range ee.Sections {
			if s.StartPoint != nil {
				p.Points = appendPoint(p.Points, *s.StartPoint)
			}
			for _, bp := range s.BendPoints {
				p.Points = appendPoint(p.Points, bp)
			}
			if s.EndPoint != nil {
				p.Points = appendPoint(p.Points, *s.EndPoint)
			}
		}
		if b.reversed[i] {
			slices.Reverse(p.Points)
		}
		labels := ee.Labels
		if g.Edges[i].Label != nil && g.Edges[i].Label.Text != "" {
			p.Label = labelRect(labels[0], 0)
		}
		labels = labels[1:]
		if b.layered {
			for range g.Edges[i].Tail {
				p.Tail = append(p.Tail, labelRect(labels[0], markerPad(g.Edges[i].TailMarker)))
				labels = labels[1:]
			}
			for range g.Edges[i].Head {
				p.Head = append(p.Head, labelRect(labels[0], markerPad(g.Edges[i].HeadMarker)))
				labels = labels[1:]
			}
		}
		r.Edges[i] = p
	}
	return r
}

// appendPoint appends a point unless it repeats the last one, where two
// sections of an edge meet.
func appendPoint(pts []Point, p elk.Point) []Point {
	q := Point{X: p.X, Y: p.Y}
	if n := len(pts); n > 0 && pts[n-1] == q {
		return pts
	}
	return append(pts, q)
}

// labelRect returns the place of a label, without the room of pad on each
// side, or nil if ELK did not place it.
func labelRect(l *elk.Label, pad float64) *Rect {
	if l.X == nil || l.Y == nil {
		return nil
	}
	r := rect(l.X, l.Y, l.Width, l.Height)
	r.X, r.Y, r.Width, r.Height = r.X+pad, r.Y+pad, r.Width-2*pad, r.Height-2*pad
	return &r
}

// rect returns a box from the optional numbers of ELK.
func rect(x, y, w, h *float64) Rect {
	return Rect{X: num(x), Y: num(y), Width: num(w), Height: num(h)}
}

// num returns a number of ELK, or 0 if ELK gave none.
func num(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
