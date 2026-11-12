package mermaid

import (
	"maps"
	"slices"

	"github.com/xo/zuzu/graph"
)

// defaultLane is the id of the lane that holds the nodes outside every
// subgraph of a swimlane diagram, the id that Mermaid 12.3 gives it.
const defaultLane = "__swimlane_default__"

// laneKind reports whether a kind of diagram can lay out in lanes: the
// swimlane diagram, and the flowchart with the layout swimlane.
func laneKind(kind string) bool {
	switch kind {
	case "swimlane-beta", "flowchart", "graph":
		return true
	}
	return false
}

// laneLayout returns the layout of a diagram: the setting layout of the
// section of its kind, or else the setting layout at the top of the
// config, or else swimlane for a swimlane diagram and dagre for the others,
// as Mermaid 12.3 picks it.
func laneLayout(d *Diagram) string {
	if s := str(d.Config, configSection(d.Kind), "layout"); s != "" {
		return s
	}
	if s := str(d.Config, "layout"); s != "" {
		return s
	}
	if d.Kind == "swimlane-beta" {
		return "swimlane"
	}
	return "dagre"
}

// lanesOn reports whether a diagram lays out in lanes: a swimlane diagram
// with no other layout that zuzu draws, or a flowchart with the layout
// swimlane. The layouts dagre and elk draw a swimlane diagram as a
// flowchart, with its subgraphs, as Mermaid does.
func lanesOn(d *Diagram) bool {
	if !laneKind(d.Kind) {
		return false
	}
	switch laneLayout(d) {
	case "swimlane":
		return true
	case "dagre", "elk":
		return false
	}
	return d.Kind == "swimlane-beta"
}

// swimlaneSettings reads the section swimlane of the config of a swimlane
// diagram, or of a flowchart that lays out in lanes. laneLayout reads its
// layout, and its look draws in the look of zuzu. ignoreCrossLaneEdges
// false makes an edge between two lanes move its target to the next rank.
// zuzu does not move the ranks to fewer crossings, so
// optimizeRanksByCrossings true with that setting is an issue. zuzu does
// not order the lanes, so automaticLaneOrdering true is an issue. Mermaid
// draws a hop where two edges cross with lineHops, and zuzu draws none, so
// a hop is an issue too. A diagram that does not lay out in lanes reads
// only the layout and the look.
func (f *flowchart) swimlaneSettings(v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return f.issues.Add("config", "the setting swimlane")
	}
	on := lanesOn(f.d)
	ignore := true
	if b, ok := m["ignoreCrossLaneEdges"].(bool); ok {
		ignore = b
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		b, isBool := m[k].(bool)
		var good bool
		switch k {
		case "layout":
			s, isText := m[k].(string)
			good = isText && (s == "swimlane" || s == "dagre" || hint("layout", s))
		case "look":
			good = true
		case "ignoreCrossLaneEdges":
			good = on && isBool
		case "optimizeRanksByCrossings":
			good = on && isBool && (!b || ignore)
		case "automaticLaneOrdering", "lineHops":
			good = on && isBool && !b
		default:
			good = accepted(f.d.Kind, "swimlane."+k, m[k])
		}
		if !good {
			if err := f.issues.Add("config", "the setting swimlane.%s: %v", k, m[k]); err != nil {
				return err
			}
		}
	}
	f.g.LaneEdges = on && !ignore
	return nil
}

// lanes makes the graph of a diagram that lays out in lanes: each subgraph
// at the top is a lane, and the nodes outside every subgraph go into a lane
// of their own, with no title, before the others, as in Mermaid 12.3.
func (f *flowchart) lanes() {
	f.g.Lanes = true
	loose := false
	for _, n := range f.g.Nodes {
		if n.Cluster == "" {
			loose = true
			break
		}
	}
	if !loose {
		return
	}
	if !slices.ContainsFunc(f.g.Clusters, func(c *graph.Cluster) bool { return c.ID == defaultLane }) {
		c := &graph.Cluster{ID: defaultLane, Style: map[string]string{}}
		f.g.Clusters = append([]*graph.Cluster{c}, f.g.Clusters...)
	}
	for _, n := range f.g.Nodes {
		if n.Cluster == "" {
			n.Cluster = defaultLane
		}
	}
}
