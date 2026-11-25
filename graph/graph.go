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
	// TitleStyle holds the fontWeight and the fontStyle of Title, or nil.
	TitleStyle map[string]string
	Nodes      []*Node
	Edges      []*Edge
	Clusters   []*Cluster
	// RankSep and NodeSep are the space between the ranks and between the
	// nodes of a rank, or 0 for the defaults.
	RankSep, NodeSep float64
	// ClusterTitleTop and ClusterTitleBottom are the space above and below
	// the title of each cluster, 0 by default.
	ClusterTitleTop, ClusterTitleBottom float64
	// Padding is the space around the diagram, or nil for 8 pixels.
	Padding *float64
	// TitleOffset is the space between the title and the diagram, or nil
	// for the space of the title of a chart.
	TitleOffset *float64
	// Lanes lays the graph out in lanes, as LayoutInput.Lanes says, and
	// LaneEdges makes an edge between two lanes move its target to the next
	// rank, as LayoutInput.LaneEdges says.
	Lanes, LaneEdges bool
	// ClusterDirection is the direction of a cluster with no direction of
	// its own that lays out on its own in zuzu.dagre, as
	// LayoutInput.ClusterDirection says, or empty. A state diagram of
	// Mermaid gives TB.
	ClusterDirection string
	// ReverseClusters sets the sibling clusters in zuzu.dagre in the
	// reverse of their order, as LayoutInput.ReverseClusters says. The
	// reader of a flowchart of Mermaid sets it.
	ReverseClusters bool
	// Order holds the ids of the nodes and the clusters in the order in
	// which Mermaid 12.3 gives them to its layouts, as LayoutInput.Order
	// says, or nil. The reader of a state diagram sets it, because Mermaid
	// gives a composite state and a note among the states, in the order of
	// the source.
	Order []string
	// ELK holds the settings of the section elk of the config of Mermaid,
	// as LayoutInput.ELK says, or nil.
	ELK map[string]string
	// BreakWords breaks a word that is wider than the wrap width of its
	// label between two letters, as the SVG labels of Mermaid do. Otherwise
	// the word stays whole on a line of its own, and the label is wider
	// than the wrap width, as the HTML labels of Mermaid are.
	BreakWords bool
	// Math draws the math of KaTeX in the labels, each text between two
	// $$, with the font of math of the render (D36). A label with math is
	// one row of its runs of text and of its formulas, with no wrap, and
	// each newline in it is a space, as Mermaid 12.3 draws it. Each line
	// of a field, such as a member of a class, is a label of its own.
	// MathSource holds the labels with math that draw their source as
	// text, because a formula in them does not lay out.
	Math       bool
	MathSource []string

	// Description is the text of the diagram for a screen reader, or
	// empty. Compile writes it as the description of the spec.
	Description string
	// Href and Tooltip are the link and the tooltip of the whole diagram,
	// or empty (D32).
	Href, Tooltip string
	// Background is a color of CSS that fills the page and is the
	// background of the theme, such as the bgcolor of a graph of DOT, or
	// empty (D37). Compile writes it in the config of the spec as
	// zuzu.background.
	Background string
}

// Node is a node of a graph.
type Node struct {
	ID string
	// Label is the text of the node. An empty label draws the id, except
	// on a node with a Width and a Height, or with NoLabel, which draws no
	// text.
	Label string
	// NoLabel draws no text for an empty Label, as label="" of DOT does.
	NoLabel bool
	// Shape names the shape of the node, such as rect or ellipse. The
	// shape polygon of DOT has the name that Polygon writes.
	Shape string
	// Cluster is the id of the cluster that holds the node, or empty.
	Cluster string
	// Rank names a group of nodes of the same rank, or empty.
	Rank string
	// Style holds the paint of the node: fill, stroke, color, strokeWidth,
	// strokeDash, strokeDashOffset, fontWeight, fontStyle and so on, with
	// the names of Vega. A fill of
	// two colors of DOT, such as a:b, sets gradient to the list of colors,
	// gradientAngle to its angle in degrees, and gradientType to radial
	// for a radial gradient. Fill then holds the first color.
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
	// Note says that the node is a note of Mermaid, such as a note of a
	// state diagram, which draws in the color of a note of the theme, its
	// yellow, and not in the color of a node.
	Note bool
	// Boxed puts the node in a box of its own for the engine of ELK, as
	// LayoutNode.Boxed says. Mermaid 12.3 puts each note of a state
	// diagram in such a box.
	Boxed bool
	// Solid says that the node fills with the color of its outline, not
	// with its tint, such as the start of a state diagram, a fork bar or a
	// junction point of a flowchart (D55).
	Solid bool
	// Runs are the parts of the label of a node in one font each, bold,
	// italic or plain, such as the runs of a string of markdown of
	// Mermaid, or nil for a label in one font. A run with the text "\n"
	// starts a new line. The layout wraps the runs and draws each in its
	// font, as a label of HTML.
	Runs []Run
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
	// MinLabelWidth is the least width of the box of the label, or 0 for
	// none. A narrower label takes the middle of a box of this width, and
	// the shape grows around the box, as Mermaid sizes a node with
	// minNodeWidth. A node with no label, a node with fields or HTML, and
	// a node with an icon or an image outside the line of its label do not
	// read it.
	MinLabelWidth float64
	// Wrap is the width in pixels at which the label wraps, or 0 for a
	// label that does not wrap. A line of the label that is wider breaks at
	// its spaces into lines that are not wider. The label of a node with
	// fields or HTML does not wrap.
	Wrap float64
	// Href is the link of the node and Tooltip its tooltip, or empty
	// (D32). Its shape and its labels take them.
	Href, Tooltip string
	// Category is the place of a color in the category scheme of the
	// theme, from 1, or 0 for none. The node then draws in that color, a
	// tint and an outline, and its figure too. A fill or a stroke of Style
	// wins over it. After the last color of the scheme, the colors start
	// again (D37).
	Category int
}

// Media is the icon or the image of a node (D31).
type Media struct {
	// Icon names an icon, as pack:name, and Image the URL of an image.
	Icon, Image string
	// Path is a figure that zuzu draws in the place of an icon, such as
	// an actor of a use case diagram: the path data of SVG around 0, 0,
	// inside Width and Height. It takes the paint of its node.
	Path string
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
	// its center, the default. It can also hold one letter for each line,
	// l, c or r for the left, the center or the right, as the \l, \n and
	// \r of a record of DOT. The last letter holds for the lines after it.
	Align string
	// Fields are the fields of a group in braces of a record of DOT, in the
	// place of Text, or nil. They lie the other way from the fields around
	// the group: one under another in a record whose fields lie side by
	// side, and side by side in a record whose fields lie one under
	// another. A group can hold groups.
	Fields []Field
	// Fonts holds the font of each line of the text, or of each cell of
	// a row of cells, or nil for the font of the node. A cell draws no
	// line under it.
	Fonts []Font
	// Runs holds the runs of each line of the text, or of each cell of a
	// row of cells, as Fonts holds their fonts, or nil. A line or a cell
	// with runs draws each run in its own font, in the place of its font
	// in Fonts, as a label of HTML. A line or a cell with no runs draws
	// in its font.
	Runs [][]Run
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
	// Runs are the parts of the label in one font each, as Node.Runs
	// says, or nil for a label in one font. Label holds their text.
	Runs []Run
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
	// default, and Weight how much a short edge counts, 1 when it is nil.
	// A Weight of 0 counts for nothing, as weight=0 of DOT does.
	MinLen int
	Weight *float64
	// NoConstraint takes the edge out of the ranks.
	NoConstraint bool
	// Head and Tail are the arrowheads at the ends: a name of the arrowType
	// of Graphviz, such as normal, vee, empty, odiamond, crowodot or
	// teetee, or circle, cross or circleplus of Mermaid. ArrowOf says
	// which names zuzu draws.
	Head, Tail string
	// ArrowSize scales the arrowheads, 1 for 0.
	ArrowSize float64
	// Style holds the paint of the edge, as Node.Style does.
	Style map[string]string
	// Strokes are the colors of an edge of more than one color, as a list
	// of colors of DOT, such as red:blue, gives them, or nil. Each color
	// draws a line of its own, as ColorLines says. Weights holds the weight
	// of each color, as `a:b;0.3` gives it, -1 for a color with no weight,
	// or nil for a list with no weight, whose lines lie side by side. The
	// stroke of Style is the color of the arrowheads: the first color, or
	// the last of colors one after another.
	Strokes []string
	Weights []float64
	// Curve is the interpolation of the route, or empty for basis.
	Curve string
	// Wrap is the width in pixels at which the label wraps, as Node.Wrap,
	// or 0 for a label that does not wrap.
	Wrap float64
	// Href and Tooltip are the link and the tooltip of the edge, or empty
	// (D32). Its line, its arrowheads and its labels take them.
	Href, Tooltip string
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
	// Href and Tooltip are the link and the tooltip of the cluster, or
	// empty (D32). Its box and its title take them.
	Href, Tooltip string
	// Category is the place of a color in the category scheme, as
	// Node.Category. The panel then draws as a node of that color draws: a
	// tint and an outline.
	Category int
	// Shape is the shape of the cluster: empty for a panel, or
	// ClusterPackage.
	Shape string
	// Algorithm is the layout of ELK of the cluster alone, as
	// LayoutCluster.Algorithm says, such as elk.box, or empty.
	Algorithm string
}

// ClusterPackage is the shape of a cluster that draws as a package of UML:
// a panel with a tab at its top left that holds the title (D32).
const ClusterPackage = "package"

// ClusterState is the shape of a cluster that draws as a composite state
// of a state diagram: a panel with a band across its top that holds the
// title, in a stronger tint of its color, with a line under the band.
const ClusterState = "state"

// Run is a part of a label in one font: its text, and whether it is bold
// or italic.
type Run struct {
	Text         string
	Bold, Italic bool
}
