package theme

// The styles that the compiler of Vega-Lite names for the color of a mark
// that a spec does not set (D6, D14). The config of a theme gives them the
// first color of the categories, or the text color.
const (
	// StyleFill is the style of a mark whose fill is the color of the mark.
	StyleFill = "zuzu.fill"
	// StyleStroke is the style of a mark whose stroke is the color of the
	// mark.
	StyleStroke = "zuzu.stroke"
	// StyleText is the style of a text whose fill is the text color.
	StyleText = "zuzu.text"
	// StyleBar is the prefix of the style of a bar, which its orient,
	// vertical or horizontal, ends.
	StyleBar = "zuzu.bar."
	// StylePoint is the style of a point, which gives it a ring in the
	// background color.
	StylePoint = "zuzu.point"
	// CornerRadiusEnd is the property of a style of a bar that rounds the
	// end of the bar away from zero, and only the last bar of a stack. It
	// is a rule of the look of zuzu that Vega has no config for, and 0
	// turns it off (D14).
	CornerRadiusEnd = "zuzu.cornerRadiusEnd"
	// LargestFirst is the property of the config of a symbol that draws the
	// largest first, when the symbols have sizes, so that the small ones
	// stay on top.
	LargestFirst = "zuzu.largestFirst"
	// EndLabel is the property of the config of a line that names its
	// series at its last point, with a dot.
	EndLabel = "zuzu.endLabel"
	// MarkHigh is the property of a style of an area that marks its highest
	// point with its value.
	MarkHigh = "zuzu.markHigh"
	// Reference is the property of a style of a rule that dashes it and
	// names its value where it starts.
	Reference = "zuzu.reference"
	// StyleReference is the style of a rule of Vega-Lite across the whole
	// plot, a reference line.
	StyleReference = "zuzu.reference"
	// Hierarchy is the property of the config of a rect, a symbol and an
	// arc that draws the nodes of a hierarchy: a group as a tint of its
	// color, with its name.
	Hierarchy = "zuzu.hierarchy"
	// Total is the property of the config of an arc that writes the total
	// of a donut in its center.
	Total = "zuzu.total"
	// Sankey is the property of the config of a path and a rect that draws
	// the links of a sankey at 0.32 opacity and its nodes with round
	// corners.
	Sankey = "zuzu.sankey"
	// StyleNode and the styles after it are the styles of the marks of a
	// diagram of DOT or Mermaid (D31): a node,
	// its label, an edge, its arrowheads, filled and open, its label and the
	// box under the label, a cluster and its title, and an icon of one
	// color, filled and stroked.
	StyleNode         = "zuzu.node"
	StyleNodeLabel    = "zuzu.nodeLabel"
	StyleEdge         = "zuzu.edge"
	StyleArrow        = "zuzu.arrow"
	StyleArrowOpen    = "zuzu.arrow.open"
	StyleEdgeLabel    = "zuzu.edgeLabel"
	StyleEdgeLabelBox = "zuzu.edgeLabelBox"
	StyleCluster      = "zuzu.cluster"
	StyleClusterTitle = "zuzu.clusterTitle"
	StyleIcon         = "zuzu.icon"
	StyleIconStroke   = "zuzu.icon.stroke"
	// StackGap is the property of a style of a bar that opens a gap of that
	// width between the bars of a stack.
	StackGap = "zuzu.stackGap"
	// StyleArea is the style of an area that stacks on no other, which
	// draws its color as a gradient with a line on top.
	StyleArea = "zuzu.area"
	// StyleAreaStacked is the style of the layer of a stacked area.
	StyleAreaStacked = "zuzu.area.stacked"
	// StyleCurve is the style of a line or an area along x, which draws as
	// a monotone curve.
	StyleCurve = "zuzu.curve"
	// Gradient is the property of a style of an area that fills it from an
	// opacity at its line to an opacity at its base.
	Gradient = "zuzu.gradient"
	// TopLine is the property of a style of an area that draws a line of
	// that width along its top.
	TopLine = "zuzu.topLine"
)

// Config returns the theme as a Vega config (D14): the defaults of each
// kind of mark, in the colors of the theme and with the rules of the marks
// of docs/STYLE.md, and the styles that the compiler of Vega-Lite names. The
// config of a spec goes over it, and the encoding of a mark goes over both.
// Each call returns a new value, which the caller owns.
func (t Theme) Config() map[string]any {
	c := t.Category[0]
	return map[string]any{
		"arc":  map[string]any{"fill": c, "padAngle": 0.01, "cornerRadius": 4, Hierarchy: true, Total: true},
		"area": map[string]any{"fill": c},
		"line": map[string]any{
			"stroke": c, "strokeWidth": 2.0, "strokeCap": "round", "strokeJoin": "round", EndLabel: true,
		},
		"path":  map[string]any{"stroke": c, Sankey: true},
		"rect":  map[string]any{"fill": c, Hierarchy: true, Sankey: true},
		"rule":  map[string]any{"stroke": t.Text},
		"shape": map[string]any{"stroke": c},
		// A point has a ring of 1 in the background color.
		"symbol": map[string]any{"fill": c, "size": 64.0, "stroke": t.Background, "strokeWidth": 1.0, LargestFirst: true, Hierarchy: true},
		"text":   map[string]any{"fill": t.Text},
		"trail":  map[string]any{"fill": c, "size": 2.0},
		"style": map[string]any{
			StyleFill:               map[string]any{"fill": c},
			StyleStroke:             map[string]any{"stroke": c},
			StyleText:               map[string]any{"fill": t.Text},
			StylePoint:              map[string]any{"stroke": t.Background, "strokeWidth": 1.0},
			StyleBar + "vertical":   map[string]any{CornerRadiusEnd: 3.0, StackGap: 1.5},
			StyleBar + "horizontal": map[string]any{CornerRadiusEnd: 3.0, StackGap: 1.5},
			StyleArea:               map[string]any{Gradient: []any{0.32, 0.02}, TopLine: 2.0, MarkHigh: true},
			StyleReference:          map[string]any{Reference: true},
			// The layers of a stacked area are full colors, with a line of
			// the background between them.
			StyleAreaStacked: map[string]any{"fillOpacity": 0.88, "stroke": t.Background, "strokeWidth": 1.0},
			StyleCurve:       map[string]any{"interpolate": "monotone"},
			// A node is a tint of its color with an outline of 1.25, and an
			// edge a line of 1.25 in the color of the axis (D31).
			StyleNode:         map[string]any{"fill": c, "fillOpacity": 0.14, "stroke": c, "strokeWidth": 1.25},
			StyleNodeLabel:    map[string]any{"fill": t.Text, "fontSize": 11.0},
			StyleEdge:         map[string]any{"stroke": t.Axis, "strokeWidth": 1.25, "strokeCap": "round", "strokeJoin": "round"},
			StyleArrow:        map[string]any{"fill": t.Axis, "stroke": t.Axis, "strokeWidth": 1.0},
			StyleArrowOpen:    map[string]any{"fill": t.Background, "stroke": t.Axis, "strokeWidth": 1.25},
			StyleEdgeLabel:    map[string]any{"fill": t.Muted, "fontSize": 11.0},
			StyleEdgeLabelBox: map[string]any{"fill": t.Background, "fillOpacity": 0.9, "cornerRadius": 3.0},
			StyleCluster:      map[string]any{"fill": t.Panel, "stroke": t.Axis, "strokeWidth": 1.0, "cornerRadius": 6.0},
			StyleClusterTitle: map[string]any{"fill": t.Muted, "fontSize": 11.0, "fontWeight": "600"},
			StyleIcon:         map[string]any{"fill": c},
			StyleIconStroke:   map[string]any{"stroke": c, "strokeCap": "round", "strokeJoin": "round"},
		},
	}
}
