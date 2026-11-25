// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/xo/zuzu/elk/internal/adapter"
	"github.com/xo/zuzu/graph"
)

// port adds a port to the node of an end of the edge i, at the place that
// the input gives from the center of the node, and returns its id. ELK
// places a port on the border of its node, so the port goes to the side
// that side names, or else to the side that faces the route: the side
// toward the next rank at the source, and the side toward the rank before
// at the target. It keeps the place of the input along that side. It
// returns no id and true for an end with no port, and false for a port at
// a cluster, which ELK does not lay out.
func (c *conversion) port(i int, id string, at, side []float64, source bool) (string, bool) {
	if len(at) < 2 {
		return "", true
	}
	j, ok := c.nodes[id]
	if !ok {
		return "", false
	}
	n := c.in.Nodes[j]
	w, h := n.Width, n.Height
	x := math.Max(-w/2, math.Min(w/2, at[0]))
	y := math.Max(-h/2, math.Min(h/2, at[1]))
	var name string
	switch {
	case len(side) >= 2 && (side[0] != 0 || side[1] != 0):
		name = sideOf(side[0], side[1])
	default:
		// The route leaves the source toward the next rank, and enters
		// the target from the rank before.
		dx, dy := 0.0, 1.0
		switch c.in.Direction {
		case graph.BT:
			dx, dy = 0, -1
		case graph.LR:
			dx, dy = 1, 0
		case graph.RL:
			dx, dy = -1, 0
		}
		if !source {
			dx, dy = -dx, -dy
		}
		name = sideOf(dx, dy)
	}
	switch name {
	case adapter.East:
		x = w / 2
	case adapter.West:
		x = -w / 2
	case adapter.South:
		y = h / 2
	default:
		y = -h / 2
	}
	end := "t"
	if source {
		end = "s"
	}
	p := adapter.Port{
		ID:   "p" + strconv.Itoa(i) + end,
		Side: name,
		At:   &adapter.Point{X: (x + w/2) * mermaidScale, Y: (y + h/2) * mermaidScale},
	}
	an := &c.g.Nodes[c.nodeAt[j]]
	an.Ports = append(an.Ports, p)
	return p.ID, true
}

// sideOf returns the side of a node in a direction out of it.
func sideOf(dx, dy float64) string {
	switch {
	case math.Abs(dx) > math.Abs(dy) && dx > 0:
		return adapter.East
	case math.Abs(dx) > math.Abs(dy):
		return adapter.West
	case dy > 0:
		return adapter.South
	}
	return adapter.North
}

// endLabels reports whether ELK places the labels at the ends of an edge:
// with the layered algorithm, which alone places them, and for a label
// with no fixed place.
func (c *conversion) endLabels(e graph.LayoutEdge) bool {
	return c.g.Algorithm == adapter.Layered && !(e.LabelDistance > 0)
}

// settings reads the settings of the section elk of the config of Mermaid
// into the config of the adapter. It names each setting that the engine
// does not read, and each value that it cannot read.
func (c *conversion) settings() {
	cfg := &c.g.Config
	for _, k := range slices.Sorted(maps.Keys(c.in.ELK)) {
		v := c.in.ELK[k]
		flag := func(set func(bool)) {
			b, err := strconv.ParseBool(v)
			if err != nil {
				c.unsupportedf("the setting elk.%s: %s", k, v)
				return
			}
			set(b)
		}
		switch k {
		case "preset":
			cfg.Preset = v
		case "nodePlacementStrategy":
			cfg.NodePlacementStrategy = v
		case "nodePlacementAlignment":
			cfg.NodePlacementAlignment = v
		case "cycleBreakingStrategy":
			cfg.CycleBreakingStrategy = v
		case "layeringStrategy":
			cfg.LayeringStrategy = v
		case "considerModelOrder":
			cfg.ConsiderModelOrder = v
		case "layeringLayerBound":
			bound, err := strconv.Atoi(v)
			if err != nil || bound < 1 {
				c.unsupportedf("the setting elk.%s: %s", k, v)
				continue
			}
			cfg.LayeringLayerBound = bound
		case "mergeEdges":
			flag(func(b bool) { cfg.MergeEdges = b })
		case "forceNodeModelOrder":
			flag(func(b bool) { cfg.ForceNodeModelOrder = b })
		case "keepEntryNodeOnTop":
			flag(func(b bool) { cfg.KeepEntryNodeOnTop = b })
		case "orientFeedbackEdges":
			flag(func(b bool) { cfg.KeepFeedbackEdges = !b })
		default:
			// straightenEdges and lineHops change how Mermaid draws a
			// route, not the layout.
			c.unsupportedf("the setting elk.%s", k)
		}
	}
}
