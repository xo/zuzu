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
	Title string
	// TitleHTML is the source of a label of HTML of DOT for the title, with
	// no < and > around it, or empty. It draws in the place of Title, at
	// the top left of the diagram.
	TitleHTML string
	Nodes     []*Node
	Edges     []*Edge
	Clusters  []*Cluster
	// RankSep and NodeSep are the space between the ranks and between the
	// nodes of a rank, or 0 for the defaults.
	RankSep, NodeSep float64

	// Description is the text of the diagram for a screen reader, or
	// empty. Compile writes it as the description of the spec.
	Description string
}

// Node is a node of a graph.
type Node struct {
	ID string
	// Label is the text of the node. An empty label draws the id, except
	// on a node with a Width and a Height, which draws no text.
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
	// Stack lays the fields of a record one under another in every
	// direction, as the title and the descriptions of a state of Mermaid.
	// Otherwise the fields lie across the ranks. A field of a stacked
	// record with no text and no cells is a thin empty band, as an empty
	// compartment of a class is.
	Stack bool
	// HTML is the source of a label of HTML of DOT, with no < and > around
	// it, or empty. The package html of okage reads it and lays it out
	// (D31).
	HTML string
	// Margin is the space around a label of HTML, x and y, or nil for the
	// default of the shape.
	Margin []float64
	// XLabel is a label outside the node, at a corner of the node, and
	// XHTML the source of a label of HTML there, or empty. An xlabel takes
	// no space in the layout: it goes where it covers no node and no other
	// label, if it can.
	XLabel, XHTML string
	// Width and Height are the size of the node, or 0 to measure the label.
	Width, Height float64
	// MinWidth and MinHeight are the least size of the node, which grows to
	// fit its label, or 0 for none.
	MinWidth, MinHeight float64
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
	// Cells are the cells of a row of a table, in the place of Text. The
	// cells of a column of a record line up, a line divides two columns,
	// and a column with no text in any row takes no space. A cell starts
	// at the left of its column.
	Cells []string
	// Align puts each line of the text at the left of the field, l, or at
	// its center, the default.
	Align string
	// Fonts holds the font of each line of the text, or nil for the font
	// of the node.
	Fonts []Font
}

// Font is the font of a line of a field: bold, italic, or with a line
// under it.
type Font struct {
	Bold, Italic, Underline bool
}

// Edge is an edge of a graph.
type Edge struct {
	ID             string
	Source, Target string
	// SourcePort and TargetPort name a field or a cell, a side such as e,
	// or both, as p:e.
	SourcePort, TargetPort string
	Label                  string
	// HTML is the source of a label of HTML of DOT, with no < and > around
	// it, or empty. It draws in the place of Label, and Label holds its
	// text.
	HTML string
	// HeadLabel and TailLabel are the labels at the head and at the tail of
	// the edge, such as the cardinality of an association, or empty. Each
	// sits beside the edge, near its end. XLabel is a label beside the
	// middle of the edge. These three take no space in the ranks.
	HeadLabel, TailLabel, XLabel string
	// HeadHTML, TailHTML and XHTML are the sources of labels of HTML that
	// draw in the place of HeadLabel, TailLabel and XLabel, or empty.
	HeadHTML, TailHTML, XHTML string
	// LabelStyle holds the paint of the head and the tail labels: color
	// and fontSize. A key that it does not set takes the one of Style.
	LabelStyle map[string]string
	// LabelDistance and LabelAngle fix the places of the head and the tail
	// labels, as labeldistance and labelangle of DOT do: the center of a
	// label is 10 times LabelDistance pixels from the end, at LabelAngle
	// degrees counterclockwise from the edge. When LabelDistance is 0, the
	// layout places them itself, beside the edge.
	LabelDistance, LabelAngle float64
	// MinLen is the least number of ranks that the edge spans, 1 by
	// default, and Weight how much a short edge counts, 1 by default.
	MinLen int
	Weight float64
	// NoConstraint takes the edge out of the ranks.
	NoConstraint bool
	// Head and Tail are the arrowheads at the ends: a name of the arrowType
	// of Graphviz, such as normal, vee, empty, odiamond, crowodot or
	// teetee, or circle or cross of Mermaid. ArrowOf says which names zuzu
	// draws.
	Head, Tail string
	// ArrowSize scales the arrowheads, 1 for 0.
	ArrowSize float64
	// Style holds the paint of the edge, as Node.Style does.
	Style map[string]string
	// Curve is the interpolation of the route, or empty for basis.
	Curve string
}

// Cluster is a cluster of nodes, as a subgraph of DOT or Mermaid.
type Cluster struct {
	ID, Label string
	// HTML is the source of a label of HTML of DOT for the title, or
	// empty. It draws in the place of Label.
	HTML string
	// Parent is the id of the cluster that holds this one, or empty.
	Parent string
	// Direction is the direction of the cluster, or empty for that of the
	// graph.
	Direction string
	Style     map[string]string
}
