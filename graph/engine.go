package graph

import "math"

// ConfigLayout is the key of the config of a spec that names the layout of
// its graphs, such as dagre or elk.stress (D44, D45). The built in engines
// are layered and dagre. A layout of ELK, elk or a name that starts with
// elk., lays out with the engine named elk that the caller makes available
// with zuzu.WithLayoutEngine, and without it is a hint that the transform
// of the spec draws. A graph of Mermaid fills it: elk for a kind whose
// default layout is ELK when the diagram sets no layout, and else the
// layout that the diagram sets, such as dagre. The layout that the caller
// forces with zuzu.WithLayout wins over it.
const ConfigLayout = "zuzu.layout"

// Engine is a layout engine (D39 part 10). A graph goes in, as LayoutInput
// holds it: the nodes with their sizes, the clusters nested by their
// parents, the edges with the boxes of their labels and of the labels at
// their ends, the ports, the direction and the spaces. The places of the
// nodes and the clusters, the points of each edge and the places of its
// labels come out, as Layout holds them, in the order of the input.
//
// An engine that does not lay out a part of the input as the input asks,
// such as a port, names that part in Layout.Unsupported. The transform
// that runs the engine reports each such part, so that zuzu never drops a
// part without a word (hard rule 4). An engine that does not lay out a
// graph at all returns an error.
//
// An engine is safe for concurrent use. It keeps no state between two
// calls of Lay.
type Engine interface {
	// Name returns the name of the engine, such as layered or dagre. The
	// transform of the engine is zuzu. and the name, such as zuzu.dagre.
	Name() string
	// Lay lays out a graph.
	Lay(in LayoutInput) (*Layout, error)
}

// The names of the built in engines, and of the engine of ELK, which the
// module github.com/xo/zuzu/elk gives. The engine of ELK lays out the
// layout elk and each layout whose name starts with elk., such as
// elk.stress (D45).
const (
	EngineLayered = "layered"
	EngineDagre   = "dagre"
	EngineELK     = "elk"
)

// Layered returns the engine of zuzu.layered, the layered layout of zuzu
// (D31). It lays out the whole of LayoutInput, and names nothing in
// Layout.Unsupported.
func Layered() Engine {
	return layeredEngine{}
}

// layeredEngine is the engine of Layered.
type layeredEngine struct{}

// Name satisfies Engine.
func (layeredEngine) Name() string {
	return EngineLayered
}

// Lay satisfies Engine.
func (layeredEngine) Lay(in LayoutInput) (*Layout, error) {
	return Lay(in)
}

// BuiltinEngine returns the built in engine of a name, layered or dagre,
// or nil for another name.
func BuiltinEngine(name string) Engine {
	switch name {
	case EngineLayered:
		return Layered()
	case EngineDagre:
		return Dagre()
	}
	return nil
}

// Finish completes a layout that an engine made from the centers of its
// nodes, the boxes of its clusters, the points of its edges and the
// centers of the labels of its edges. An end of an edge at a node that does
// not lie on the outline of the shape of the node, such as an end on the
// box of a diamond or at the center of a node, moves along the line from
// the next point of the edge to where that line meets the outline. A loop
// keeps its ends. For each edge with two points or more, it then sets the
// place and the angle of each end from the first two and the last two
// points. It places the labels at the ends of the edges and
// the xlabels of the nodes and the edges, as zuzu.layered does. It then
// moves the layout so that it starts at 0, 0, sets its size, and writes the
// path of each edge with its curve. A loop, an edge from a node to itself,
// takes the path of a loop of zuzu.layered: a cubic curve from its first
// point to its last, which the two points between pull out.
//
// An engine outside the package graph can call it after it sets the
// places, so that each engine draws the ends and the labels the same way.
func Finish(in *LayoutInput, out *Layout) {
	clipEnds(in, out)
	for i := range out.Edges {
		ep := &out.Edges[i]
		pts := ep.Points
		if len(pts) < 2 {
			continue
		}
		last := len(pts) - 1
		ep.EndX, ep.EndY = pts[last][0], pts[last][1]
		ep.EndAngle = math.Atan2(pts[last][1]-pts[last-1][1], pts[last][0]-pts[last-1][0]) * 180 / math.Pi
		ep.StartX, ep.StartY = pts[0][0], pts[0][1]
		ep.StartAngle = math.Atan2(pts[0][1]-pts[1][1], pts[0][0]-pts[1][0]) * 180 / math.Pi
	}
	placeLabels(in, out)
	normalizeLayout(in, out)
}

// clipEnds moves each end of an edge at a node onto the outline of the
// shape of the node, as Finish says.
func clipEnds(in *LayoutInput, out *Layout) {
	index := make(map[string]int, len(in.Nodes))
	for i, n := range in.Nodes {
		index[n.ID] = i
	}
	for i, e := range in.Edges {
		pts := out.Edges[i].Points
		if len(pts) < 2 || e.Source == e.Target {
			continue
		}
		last := len(pts) - 1
		if a, ok := index[e.Source]; ok {
			pts[0] = onOutline(out.Nodes[a], in.Nodes[a].Shape, pts[1], pts[0])
		}
		if b, ok := index[e.Target]; ok {
			pts[last] = onOutline(out.Nodes[b], in.Nodes[b].Shape, pts[last-1], pts[last])
		}
	}
}

// onOutline returns the end of an edge at a node, moved onto the outline of
// the shape of the node along the line from the point from: the end itself
// when it lies on the outline, and else the first point of the line from
// from through the end that lies on the outline. When from lies inside the
// shape, the end stays. When the line misses the shape, the end moves to
// where the line from the center of the node toward from meets the
// outline.
func onOutline(n NodePlace, kind string, from, end [2]float64) [2]float64 {
	// at returns the place of a point toward the outline: below 1
	// outside the shape, 1 on it and above 1 inside it.
	at := func(p [2]float64) float64 {
		dx, dy := p[0]-n.X, p[1]-n.Y
		if dx == 0 && dy == 0 {
			return math.Inf(1)
		}
		return outline(n, kind, dx, dy)
	}
	if t := at(end); math.Abs(t-1) < 1e-9 {
		return end
	}
	if at(from) >= 1 {
		return end
	}
	dx, dy := end[0]-from[0], end[1]-from[1]
	length := math.Hypot(dx, dy)
	if length == 0 {
		return end
	}
	// The line runs from from, through the end, and on across the whole
	// node, in steps, up to the first point inside the shape.
	far := 1 + 2*math.Hypot(n.Width, n.Height)/length
	point := func(u float64) [2]float64 { return [2]float64{from[0] + u*dx, from[1] + u*dy} }
	const steps = 64
	lo := 0.0
	for k := 1; k <= steps; k++ {
		hi := far * float64(k) / steps
		if at(point(hi)) < 1 {
			lo = hi
			continue
		}
		for range 60 {
			mid := (lo + hi) / 2
			if at(point(mid)) < 1 {
				lo = mid
			} else {
				hi = mid
			}
		}
		return point(hi)
	}
	x, y := clip(n, kind, from[0], from[1])
	return [2]float64{x, y}
}
