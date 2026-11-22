// SPDX-License-Identifier: EPL-2.0

package adapter

// This file gives the layout options of ELK as render.ts of Mermaid 12.3.0
// gives them. The bundle of Mermaid and @mermaid-js/layout-elk 1.0.2 hold
// the same file. Each function names the function of render.ts that it
// follows. The list of options in elkOptionCatalogue.ts of Mermaid adds
// nothing, because each of its objects is empty in the release.

import (
	"fmt"
	"strconv"

	elk "github.com/xo/zuzu/elk/internal/elkgo"
)

// The constants of render.ts.
const (
	// portsSurrounding keeps a port off the corner of a node, the
	// PORTS_SURROUNDING of Mermaid.
	portsSurrounding = "[top=12,left=12,bottom=12,right=12]"
	// subgraphPadding is SUBGRAPH_PADDING, between a frame and its
	// children.
	subgraphPadding = 24
	// subgraphSpacingBase is DEFAULT_SUBGRAPH_SPACING_BASE_VALUE.
	subgraphSpacingBase = 24
	// subgraphNodeSpacing is DEFAULT_SUBGRAPH_NODE_SPACING.
	subgraphNodeSpacing = 50
	// containerPadding is CONTAINER_PADDING, inside a cluster with an
	// algorithm of its own.
	containerPadding = 15
	// rectPackingContainerPadding is RECTPACKING_CONTAINER_PADDING.
	rectPackingContainerPadding = 10
	// sizeConstraints sizes a cluster for its children, its title and
	// its minimum size.
	sizeConstraints = "[MINIMUM_SIZE, NODE_LABELS]"
	// rootSpacingBase is the spacing.baseValue of the root.
	rootSpacingBase = 40
	// elkPadding is the default padding of ELK, which a cluster has when
	// it loses its algorithm and its padding with it.
	elkPadding = 12
	// labelPadding is the default nodeLabels.padding of ELK, around the
	// title of a cluster.
	labelPadding = 5
)

// preset is one of the ELK_PRESETS of render.ts.
type preset struct {
	layering, placement, containerPlacement, alignment, cycleBreaking string
}

// presets are the ELK_PRESETS of render.ts.
var presets = map[string]preset{
	"default":    {"NETWORK_SIMPLEX", "BRANDES_KOEPF", "BRANDES_KOEPF", "BALANCED", "DEPTH_FIRST"},
	"legacy":     {"NETWORK_SIMPLEX", "BRANDES_KOEPF", "BRANDES_KOEPF", "NONE", "GREEDY"},
	"modelOrder": {"NETWORK_SIMPLEX", "NETWORK_SIMPLEX", "BRANDES_KOEPF", "NONE", "GREEDY_MODEL_ORDER"},
	"depthFirst": {"NETWORK_SIMPLEX", "NETWORK_SIMPLEX", "BRANDES_KOEPF", "NONE", "DEPTH_FIRST"},
}

// rectPackingOptions are the RECTPACKING_OPTIONS of render.ts.
var rectPackingOptions = map[string]string{
	"spacing.baseValue":      "15",
	"spacing.nodeNode":       "15",
	"elk.aspectRatio":        "1.6",
	"elk.expandNodes":        "true",
	"elk.rectpacking.trybox": "true",
	"elk.rectpacking.packing.compaction.rowHeightReevaluation": "true",
	"elk.rectpacking.packing.compaction.iterations":            "10",
	"elk.rectpacking.whiteSpaceElimination.strategy":           "EQUAL_BETWEEN_STRUCTURES",
	"elk.rectpacking.widthApproximation.strategy":              "SCANLINE",
}

// containerOverrides are the CONTAINER_ALGORITHM_OVERRIDES of render.ts:
// each option that a cluster sets because it has an algorithm of its own.
var containerOverrides = []string{
	"nodeSize.constraints",
	"nodeSize.minimum",
	"elk.algorithm",
	"elk.aspectRatio",
	"elk.contentAlignment",
	"elk.expandNodes",
	"elk.padding",
}

// resolvePreset returns the preset of a name, or the default preset, as
// resolveElkPreset does.
func resolvePreset(name string) preset {
	if p, ok := presets[name]; ok {
		return p
	}
	return presets["default"]
}

// or returns v, or def if v is empty, as the ?? of TypeScript does.
func or(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// direction returns the direction of ELK of a direction of Mermaid, as
// dir2ElkDirection does.
func direction(dir string) string {
	switch dir {
	case "LR":
		return "RIGHT"
	case "RL":
		return "LEFT"
	case "BT":
		return "UP"
	default:
		return "DOWN"
	}
}

// formatNumber writes a number as JavaScript writes it in a template
// string, for the numbers that Mermaid builds into an option.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// rootOptions returns the layout options of the root, as createRootElkGraph
// and buildElkGraphFromLayoutData do. It also asks ELK for the coordinates
// of the whole graph, which changes the output and not the layout.
func rootOptions(g *Graph, algorithm string) elk.LayoutOptions {
	c := g.Config
	p := resolvePreset(c.Preset)
	bound := c.LayeringLayerBound
	if bound == 0 {
		bound = 4
	}
	opts := elk.LayoutOptions{
		"elk.hierarchyHandling":                                "INCLUDE_CHILDREN",
		"elk.algorithm":                                        algorithm,
		"elk.layered.nodePlacement.strategy":                   or(c.NodePlacementStrategy, p.placement),
		"elk.layered.nodePlacement.bk.fixedAlignment":          or(c.NodePlacementAlignment, p.alignment),
		"elk.layered.mergeEdges":                               strconv.FormatBool(c.MergeEdges),
		"elk.direction":                                        direction(g.Direction),
		"spacing.baseValue":                                    strconv.Itoa(rootSpacingBase),
		"elk.layered.crossingMinimization.forceNodeModelOrder": strconv.FormatBool(c.ForceNodeModelOrder),
		"elk.layered.considerModelOrder.strategy":              or(c.ConsiderModelOrder, "NODES_AND_EDGES"),
		"elk.layered.unnecessaryBendpoints":                    "true",
		"elk.layered.cycleBreaking.strategy":                   or(c.CycleBreakingStrategy, p.cycleBreaking),
		"elk.layered.layering.strategy":                        or(c.LayeringStrategy, p.layering),
		"elk.layered.layering.coffmanGraham.layerBound":        strconv.Itoa(bound),
		"elk.layered.wrapping.multiEdge.improveCuts":           "true",
		"elk.layered.wrapping.multiEdge.improveWrappedEdges":   "true",
		"elk.layered.edgeRouting.selfLoopDistribution":         "EQUALLY",
		"elk.layered.mergeHierarchyEdges":                      "true",
		"elk.spacing.portsSurrounding":                         portsSurrounding,
		// These two are of zuzu, not of Mermaid. ELK then gives each place
		// in the coordinates of the whole graph.
		"org.eclipse.elk.json.shapeCoords": "ROOT",
		"org.eclipse.elk.json.edgeCoords":  "ROOT",
	}
	if algorithm == RectPacking {
		for k, v := range rectPackingOptions {
			opts[k] = v
		}
		opts["elk.contentAlignment"] = "H_CENTER V_TOP"
		opts["elk.padding"] = "[top=15,left=15,bottom=15,right=15]"
	}
	for k, v := range g.Options {
		opts[k] = v
	}
	return opts
}

// titleWidth returns the width that the frame of a cluster needs for its
// title, as groupTitleWidth does.
func titleWidth(n *Node) float64 {
	if n.NoTitle {
		return 0
	}
	w := 0.0
	if n.Label != nil {
		w = n.Label.Width
	}
	return w + n.Padding
}

// titleSizeOptions returns the options that keep the width of the title of
// a cluster, as groupTitleSizeOptions does.
func titleSizeOptions(n *Node) map[string]string {
	if n.NoTitle {
		return nil
	}
	return map[string]string{
		"nodeSize.constraints": sizeConstraints,
		"nodeSize.minimum":     "(" + formatNumber(titleWidth(n)) + ", 0)",
	}
}

// configureCluster gives a cluster its title and its layout options, as
// configureSubgraphNodes and buildSubgraphLayoutOptions do.
func (b *builder) configureCluster(g *Graph, n *Node, algorithm string) {
	st := b.clusters[n.ID]
	en := st.node
	label := Label{Width: 50, Height: 50}
	if n.Label != nil {
		label = *n.Label
	}
	st.title = label
	st.pad = subgraphPadding
	st.minWidth = titleWidth(n)
	st.noTitle = n.NoTitle
	if b.mermaidTitles {
		en.Labels = []*elk.Label{{
			Text:   elk.String(label.Text),
			Width:  elk.Float64(label.Width),
			Height: elk.Float64(label.Height),
		}}
	}
	c := g.Config
	p := resolvePreset(c.Preset)
	opts := elk.LayoutOptions{}
	for k, v := range titleSizeOptions(n) {
		opts[k] = v
	}
	opts["spacing.baseValue"] = strconv.Itoa(subgraphSpacingBase)
	opts["elk.layered.spacing.edgeNodeBetweenLayers"] = "30"
	opts["elk.spacing.edgeEdge"] = "20"
	opts["spacing.nodeNode"] = strconv.Itoa(subgraphNodeSpacing)
	pad := formatNumber(subgraphPadding)
	opts["elk.padding"] = "[top=" + pad + ",left=" + pad + ",bottom=" + pad + ",right=" + pad + "]"
	opts["nodeLabels.placement"] = "[H_CENTER V_TOP, INSIDE]"
	opts["elk.layered.mergeEdges"] = strconv.FormatBool(c.MergeEdges)
	opts["elk.layered.nodePlacement.bk.fixedAlignment"] = or(c.NodePlacementAlignment, p.alignment)
	opts["elk.layered.nodePlacement.strategy"] = or(c.NodePlacementStrategy, p.containerPlacement)
	opts["elk.layered.cycleBreaking.strategy"] = or(c.CycleBreakingStrategy, p.cycleBreaking)
	opts["elk.layered.nodePlacement.networkSimplex.nodeFlexibility"] = "PORT_POSITION"
	opts["elk.spacing.portsSurrounding"] = portsSurrounding

	own := n.Algorithm
	if own != "" && !known(own, true) {
		b.issues = append(b.issues, fmt.Sprintf(
			"the cluster %q asks for the algorithm %q, which ELK does not have, so it takes the algorithm of the graph", n.ID, own))
		own = ""
	}
	labelW, labelH := 0.0, 0.0
	if n.Label != nil {
		labelW, labelH = n.Label.Width, n.Label.Height
	}
	minWidth := labelW + 2*n.Padding
	switch {
	case own != "":
		st.own = true
		padTop := labelH + containerPadding
		cp := formatNumber(containerPadding)
		opts["nodeSize.constraints"] = sizeConstraints
		opts["nodeSize.minimum"] = "(" + formatNumber(minWidth) + ", " + formatNumber(padTop+containerPadding) + ")"
		opts["elk.algorithm"] = own
		opts["elk.hierarchyHandling"] = "SEPARATE_CHILDREN"
		opts["elk.aspectRatio"] = "2.0"
		opts["elk.contentAlignment"] = "H_CENTER V_TOP"
		opts["elk.expandNodes"] = "true"
		opts["elk.padding"] = "[top=" + formatNumber(padTop) + ",left=" + cp + ",bottom=" + cp + ",right=" + cp + "]"
		if own == RectPacking {
			rpTop := labelH + rectPackingContainerPadding
			rp := formatNumber(rectPackingContainerPadding)
			for k, v := range rectPackingOptions {
				opts[k] = v
			}
			opts["elk.padding"] = "[top=" + formatNumber(rpTop) + ",left=" + rp + ",bottom=" + rp + ",right=" + rp + "]"
			opts["nodeSize.minimum"] = "(" + formatNumber(minWidth) + ", " + formatNumber(rpTop+rectPackingContainerPadding) + ")"
		}
	case n.Direction != "":
		opts["elk.algorithm"] = algorithm
		opts["elk.direction"] = direction(n.Direction)
		opts["elk.hierarchyHandling"] = "SEPARATE_CHILDREN"
	}
	en.LayoutOptions = opts
}

// crossHierarchy makes each cluster on the way of an edge between two
// clusters lay out with the graph, as configureCrossHierarchyEdges does.
func (b *builder) crossHierarchy(g *Graph, byID map[string]int) {
	parent := func(id string) string { return g.Nodes[byID[id]].Parent }
	for i := range g.Edges {
		e := &g.Edges[i]
		if parent(e.From) == parent(e.To) {
			continue
		}
		ancestor := b.commonAncestor(e.From, e.To, parent)
		b.includeChildren(g, byID, e.From, ancestor)
		b.includeChildren(g, byID, e.To, ancestor)
	}
}

// commonAncestor returns the first cluster that holds both nodes, or
// root, as findCommonAncestor does.
func (b *builder) commonAncestor(from, to string, parent func(string) string) string {
	seen := map[string]bool{}
	for id := from; id != ""; id = parent(id) {
		seen[id] = true
	}
	for id := to; id != ""; id = parent(id) {
		if seen[id] {
			return id
		}
	}
	return "root"
}

// includeChildren sets the hierarchy handling INCLUDE_CHILDREN on a node
// and on each cluster above it up to an ancestor, as
// setIncludeChildrenPolicy does. A cluster with an algorithm of its own
// loses it.
func (b *builder) includeChildren(g *Graph, byID map[string]int, id, ancestor string) {
	for depth := 0; id != "" && depth <= len(g.Nodes); depth++ {
		i := byID[id]
		n := &g.Nodes[i]
		en := b.nodes[i]
		if st := b.clusters[id]; st != nil && st.own {
			b.issues = append(b.issues, fmt.Sprintf(
				"an edge crosses the cluster %q, so it takes the algorithm of the graph", id))
			for _, k := range containerOverrides {
				delete(en.LayoutOptions, k)
			}
			for k := range rectPackingOptions {
				delete(en.LayoutOptions, k)
			}
			st.pad = elkPadding
			en.LayoutOptions["spacing.baseValue"] = strconv.Itoa(subgraphSpacingBase)
			en.LayoutOptions["spacing.nodeNode"] = strconv.Itoa(subgraphNodeSpacing)
			for k, v := range titleSizeOptions(n) {
				en.LayoutOptions[k] = v
			}
			st.own = false
		}
		setOption(&en.LayoutOptions, "elk.hierarchyHandling", "INCLUDE_CHILDREN")
		if id == ancestor {
			return
		}
		id = n.Parent
	}
}

// reserveTitles keeps room for the title of each cluster at its top, in
// place of the label that Mermaid gives ELK.
//
// ELK.js 0.12.0 places the title of a cluster that runs the layered
// algorithm at its top, in the middle, and moves its children down by the
// height of the title and the padding of the label. elk-go places it at the
// left and in the middle of the height when the direction is DOWN or UP, and
// keeps no room for it (BACKLOG). So the adapter gives ELK no title, adds
// its room to the padding at the top, and places the title itself, as
// result does. A cluster with an algorithm of its own keeps the room for its
// title in its padding already, as Mermaid sets it, and ELK.js adds none.
func (b *builder) reserveTitles() {
	if b.mermaidTitles {
		return
	}
	for _, st := range b.clusters {
		if st.own || st.title.Text == "" {
			continue
		}
		top := formatNumber(st.pad + labelPadding + st.title.Height)
		side := formatNumber(st.pad)
		st.node.LayoutOptions["elk.padding"] = "[top=" + top + ",left=" + side + ",bottom=" + side + ",right=" + side + "]"
		// ELK.js makes a cluster whose frame draws its title as wide as the
		// title and the padding of the label on both sides, at least.
		if st.noTitle {
			continue
		}
		width := max(st.title.Width+2*labelPadding, st.minWidth)
		st.node.LayoutOptions["nodeSize.constraints"] = sizeConstraints
		st.node.LayoutOptions["nodeSize.minimum"] = "(" + formatNumber(width) + ", 0)"
	}
}

// titleRect returns the place of the title of a cluster, at its top and in
// the middle, as ELK.js places it. It returns nil for a cluster with no
// title, which ELK.js does not place.
func titleRect(st *clusterState, r Rect) *Rect {
	if st.title.Text == "" {
		return nil
	}
	return &Rect{
		X:      r.X + (r.Width-st.title.Width)/2,
		Y:      r.Y + labelPadding,
		Width:  st.title.Width,
		Height: st.title.Height,
	}
}
