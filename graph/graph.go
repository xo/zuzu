// Package graph holds the model of a diagram, a graph of nodes, edges and
// clusters, and its layered layout (D18, D31). The packages dot and mermaid
// below it read a source into a graph, and Compile writes a graph as a Vega
// spec that draws with the transform zuzu.layered.
package graph

// Error is an error of the package graph.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// ErrGraph is the error of a graph that does not lay out, such as an edge
// to a node that does not exist.
const ErrGraph Error = "invalid graph"

// The directions of a layout: the ranks go from the top to the bottom, from
// the bottom to the top, from the left to the right, or from the right to
// the left.
const (
	TB = "TB"
	BT = "BT"
	LR = "LR"
	RL = "RL"
)

// Graph is a diagram: its nodes, its edges and its clusters, in the order
// of its source.
type Graph struct {
	// Directed says whether an edge has a direction, as a digraph of DOT
	// does and a graph does not.
	Directed bool
	// Direction is TB, BT, LR or RL.
	Direction string
	// Title is the title of the diagram, or empty.
	Title    string
	Nodes    []*Node
	Edges    []*Edge
	Clusters []*Cluster
	// RankSep and NodeSep are the space between the ranks and between the
	// nodes of a rank, or 0 for the defaults.
	RankSep, NodeSep float64
}

// Node is a node of a graph.
type Node struct {
	ID    string
	Label string
	// Shape names the shape of the node, such as rect or ellipse.
	Shape string
	// Cluster is the id of the cluster that holds the node, or empty.
	Cluster string
	// Rank names a group of nodes of the same rank, or empty.
	Rank string
	// Style holds the paint of the node: fill, stroke, color, strokeWidth,
	// strokeDash, fontWeight and so on, with the names of Vega.
	Style map[string]string
	// Media is the icon or the image of the node, or nil.
	Media *Media
	// Fields are the fields of a record, or nil.
	Fields []Field
	// Width and Height are the size of the node, or 0 to measure the label.
	Width, Height float64
}

// Media is the icon or the image of a node (D31).
type Media struct {
	// Icon names an icon, as pack:name, and Image the URL of an image.
	Icon, Image string
	// Form is the box around an icon: square, circle, rounded, or empty
	// for none.
	Form string
	// Pos puts the label above the icon, t, or below it, b, the default.
	Pos string
	// Width and Height are the size of the icon or the image, or 0 for the
	// default.
	Width, Height float64
	// Keep keeps the ratio of the width to the height of an image.
	Keep bool
	// Inline draws the icon before the label, inside the shape of the node,
	// as an icon in a label does.
	Inline bool
}

// Field is a field of a record: its text and its port.
type Field struct {
	Text, Port string
}

// Edge is an edge of a graph.
type Edge struct {
	ID             string
	Source, Target string
	// SourcePort and TargetPort name a field or a side of the end.
	SourcePort, TargetPort string
	Label                  string
	// MinLen is the least number of ranks that the edge spans, 1 by
	// default, and Weight how much a short edge counts, 1 by default.
	MinLen int
	Weight float64
	// NoConstraint takes the edge out of the ranks.
	NoConstraint bool
	// Head and Tail are the arrowheads at the ends: normal, none, vee, dot,
	// empty, circle or cross.
	Head, Tail string
	// Style holds the paint of the edge, as Node.Style does.
	Style map[string]string
	// Curve is the interpolation of the route, or empty for basis.
	Curve string
}

// Cluster is a cluster of nodes, as a subgraph of DOT or Mermaid.
type Cluster struct {
	ID, Label string
	// Parent is the id of the cluster that holds this one, or empty.
	Parent string
	// Direction is the direction of the cluster, or empty for that of the
	// graph.
	Direction string
	Style     map[string]string
}
