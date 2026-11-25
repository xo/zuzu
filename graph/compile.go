package graph

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/shape"
	zlayout "github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// The styles of the marks of a diagram, which the config of a theme gives
// (D14, D31).
const (
	StyleNode         = theme.StyleNode
	StyleNodeMuted    = theme.StyleNodeMuted
	StyleNodeLabel    = theme.StyleNodeLabel
	StyleEdge         = theme.StyleEdge
	StyleArrow        = theme.StyleArrow
	StyleArrowOpen    = theme.StyleArrowOpen
	StyleEdgeLabel    = theme.StyleEdgeLabel
	StyleEdgeLabelBox = theme.StyleEdgeLabelBox
	StyleCluster      = theme.StyleCluster
	StyleClusterTitle = theme.StyleClusterTitle
	StyleIcon         = theme.StyleIcon
	StyleIconStroke   = theme.StyleIconStroke
	StyleGuide        = theme.StyleGuide
	StyleBand         = theme.StyleBand
)

// ArrowSymbolSize is the size of the symbol of an arrowhead at an
// arrowsize of 1. ArrowPaths draws the shape normal 2 long in a symbol of
// size 4, so that the shape is the square root of the size long, and this
// size makes it theme.ArrowLength long (docs/STYLE.md).
const ArrowSymbolSize = theme.ArrowLength * theme.ArrowLength

// paintKeys are the keys of a style of a node, an edge or a cluster that
// a mark encodes, by the name of their field. category comes first, so
// that a fill or a stroke wins over the color of the category.
var paintKeys = []string{"category", "fill", "fillOpacity", "stroke", "strokeWidth", "strokeDash", "note", "solid"}

// textKeys are the keys of a style of a label.
var textKeys = []string{"textColor", "fontSize", "fontWeight", "fontStyle"}

// TextSignature writes the keys of the style of a label that a row sets
// into its field _text, and returns them. The transform zuzu.layered calls
// it for the rows of the fields of a record, whose fonts can differ from
// the font of their node.
func TextSignature(r map[string]any) string {
	return signature(r, textKeys, "_text")
}

// fontCode writes a font of a line of a field as letters: b for bold, i
// for italic and u for a line under it.
func fontCode(f Font) string {
	var s string
	if f.Bold {
		s += "b"
	}
	if f.Italic {
		s += "i"
	}
	if f.Underline {
		s += "u"
	}
	return s
}

// BoldWeight is the weight of a bold line of a field, the weight of the
// title of a cluster.
const BoldWeight = "600"

// signature writes the keys of a list that a row holds, in order, and the
// row holds it in a field, so that a mark for each signature draws the
// rows that set the same keys.
func signature(r map[string]any, keys []string, field string) string {
	var set []string
	for _, k := range keys {
		if _, ok := r[k]; ok {
			set = append(set, k)
		}
	}
	s := strings.Join(set, ",")
	r[field] = s
	return s
}

// runRows returns the runs of a label as the values of a row.
func runRows(runs []Run) []any {
	out := make([]any, len(runs))
	for j, run := range runs {
		out[j] = map[string]any{"text": run.Text, "bold": run.Bold, "italic": run.Italic}
	}
	return out
}

// lines splits a label into its lines.
func lines(s string) []any {
	var out []any
	for l := range strings.SplitSeq(s, "\n") {
		out = append(out, l)
	}
	return out
}

// style copies the paint of a style into a row, with the names of Vega. A
// solid fill with no color of text gets the color of text of the light or
// the dark theme that reads on it, so that its label reads in both modes.
func style(r map[string]any, st map[string]string) {
	if _, ok := st["color"]; !ok && st["fill"] != "" && st["fillOpacity"] == "1" {
		fill := st["fill"]
		if g := st["gradient"]; g != "" {
			fill = g
		}
		if c := Contrast(fill); c != "" {
			r["textColor"] = c
		}
	}
	for k, v := range st {
		switch k {
		case "fill", "stroke", "color", "fontWeight", "fontStyle":
			if k == "color" {
				k = "textColor"
			}
			r[k] = v
		case "strokeWidth", "fillOpacity", "fontSize":
			var f float64
			if _, err := fmt.Sscan(v, &f); err == nil {
				r[k] = f
			}
		case "strokeDash":
			var dash []any
			for _, p := range strings.FieldsFunc(v, func(c rune) bool { return c == ',' || c == ' ' }) {
				var f float64
				if _, err := fmt.Sscan(p, &f); err == nil {
					dash = append(dash, f)
				}
			}
			if len(dash) > 0 {
				r[k] = dash
			}
		}
	}
	if dash, ok := r["strokeDash"].([]any); ok && st["strokeDashOffset"] != "" {
		var off float64
		if _, err := fmt.Sscan(st["strokeDashOffset"], &off); err == nil {
			r["strokeDash"] = shiftDash(dash, off)
		}
	}
	// A fill of two colors draws as a gradient.
	if list := st["gradient"]; list != "" {
		angle, _ := strconv.ParseFloat(st["gradientAngle"], 64)
		if g, ok := Gradient(list, angle, st["gradientType"] == "radial"); ok {
			r["fill"] = g
		}
	}
}

// shiftDash returns a dash of the values of a spec that starts off along
// the dash, as shape.ShiftDash shifts it.
func shiftDash(dash []any, off float64) []any {
	d := make([]float64, 0, len(dash))
	for _, v := range dash {
		f, _ := v.(float64)
		d = append(d, f)
	}
	out := shape.ShiftDash(d, off)
	if len(out) == len(d) && slices.Equal(out, d) {
		return dash
	}
	res := make([]any, 0, len(out))
	for _, v := range out {
		res = append(res, v)
	}
	return res
}

// Compile writes a graph as a Vega spec that draws it with the transform
// zuzu.layered: its clusters, its edges with their arrowheads and labels,
// and its nodes with their labels (D31).
func Compile(g *Graph) (*vega.Spec, error) {
	dir := g.Direction
	if dir == "" {
		dir = TB
	}
	var nodes, edges, clusters []any
	// media says whether a node has an icon, an image or a figure, and
	// figures whether a node has a figure.
	media, figures, html := false, false, false
	// links says whether a node, an edge or a cluster has a link or a
	// tooltip, so that each mark encodes them (D32).
	links := false
	link := func(r map[string]any, href, tooltip string) {
		if href != "" {
			r["href"], links = href, true
		}
		if tooltip != "" {
			r["tooltip"], links = tooltip, true
		}
	}
	paints := map[string][]string{}
	// categories is the number of places of the category scheme that the
	// nodes and the clusters take (D37).
	categories := 0
	add := func(kind, sig string) {
		if !slices.Contains(paints[kind], sig) {
			paints[kind] = append(paints[kind], sig)
		}
	}
	for _, n := range g.Nodes {
		label := n.Label
		if label == "" && !n.NoLabel && (n.Width <= 0 || n.Height <= 0) {
			label = n.ID
		}
		r := map[string]any{"key": n.ID, "label": label, "lines": lines(label), "shape": n.Shape}
		if n.Cluster != "" {
			r["cluster"] = n.Cluster
		}
		if n.Rank != "" {
			r["rank"] = n.Rank
		}
		if len(n.Fields) > 0 {
			if fieldRow(r, n) {
				// The layout draws the runs of a line or a cell as a label
				// of HTML.
				html = true
			}
			if n.Stack {
				r["stack"] = true
			}
		}
		if n.HTML != "" {
			r["html"], r["lines"] = n.HTML, []any{}
			html = true
		} else if len(n.Runs) > 0 {
			// The layout wraps the runs and draws them as a label of HTML.
			r["runs"], r["lines"] = runRows(n.Runs), []any{}
			html = true
		}
		if len(n.Margin) == 2 {
			r["margin"] = []any{n.Margin[0], n.Margin[1]}
		}
		if n.Width > 0 && n.Height > 0 {
			r["width"], r["height"] = n.Width, n.Height
		}
		if n.MinWidth > 0 {
			r["minWidth"] = n.MinWidth
		}
		if n.MinHeight > 0 {
			r["minHeight"] = n.MinHeight
		}
		if n.MinLabelWidth > 0 {
			r["minLabelWidth"] = n.MinLabelWidth
		}
		if n.Wrap > 0 && len(n.Fields) == 0 && n.HTML == "" {
			r["wrap"] = n.Wrap
		}
		if m := n.Media; m != nil {
			media = true
			figures = figures || m.Path != ""
			media2row(r, m)
		}
		if labelRow(r, "x", n.XLabel, n.XHTML) {
			html = true
		}
		link(r, n.Href, n.Tooltip)
		style(r, n.Style)
		if _, ok := n.Style["color"]; !ok && LabelBelow(n.Shape) {
			// The label sits on the background under the figure, not on
			// its fill, so it keeps the color of text of the theme.
			delete(r, "textColor")
		}
		categories = categoryRow(r, n.Category, categories, false)
		if n.Note {
			r["note"] = true
		}
		if n.Solid {
			r["solid"] = true
		}
		if n.Boxed {
			r["boxed"] = true
		}
		add("node", signature(r, paintKeys, "_paint"))
		text := signature(r, textKeys, "_text")
		add("nodeText", text)
		if len(n.Fields) > 0 {
			// A bold line of a field has a weight of its own, an italic
			// line a style of its own, and a line under a line of text
			// takes the color of the text.
			add("fieldText", text)
			for _, f := range n.Fields {
				for _, ft := range f.Fonts {
					if ft.Bold || ft.Italic {
						c := maps.Clone(r)
						if ft.Bold {
							c["fontWeight"] = BoldWeight
						}
						if ft.Italic {
							c["fontStyle"] = "italic"
						}
						add("fieldText", signature(c, textKeys, "_text"))
					}
					if ft.Underline {
						add("fieldDeco", signature(r, []string{"textColor"}, "_deco"))
					}
				}
			}
		}
		if _, ok := r["xLines"]; ok {
			add("nodeXText", text)
		}
		nodes = append(nodes, r)
	}
	for i, e := range g.Edges {
		r := map[string]any{"key": strconv.Itoa(i), "source": e.Source, "target": e.Target}
		if e.Label != "" {
			r["label"], r["lines"] = e.Label, lines(e.Label)
		}
		if e.HTML != "" {
			r["html"], r["lines"] = e.HTML, []any{}
			html = true
		} else if len(e.Runs) > 0 {
			// The layout wraps the runs and draws them as a label of HTML.
			r["runs"], r["lines"] = runRows(e.Runs), []any{}
			html = true
		}
		if e.Wrap > 0 && e.Label != "" && e.HTML == "" {
			r["wrap"] = e.Wrap
		}
		for _, l := range []struct{ name, text, src string }{
			{"head", e.HeadLabel, e.HeadHTML}, {"tail", e.TailLabel, e.TailHTML}, {"x", e.XLabel, e.XHTML},
		} {
			if labelRow(r, l.name, l.text, l.src) {
				html = true
			}
		}
		if e.LabelDistance > 0 {
			r["labelDistance"], r["labelAngle"] = e.LabelDistance, e.LabelAngle
		}
		if e.SourcePort != "" {
			r["sourcePort"] = e.SourcePort
		}
		if e.TargetPort != "" {
			r["targetPort"] = e.TargetPort
		}
		if e.MinLen > 0 {
			r["minlen"] = float64(e.MinLen)
		}
		if e.Weight != nil {
			r["weight"] = *e.Weight
		}
		if e.NoConstraint {
			r["constraint"] = false
		}
		if e.Curve != "" {
			r["curve"] = e.Curve
		}
		head, tail := e.Head, e.Tail
		if head == "" {
			head = "normal"
			if !g.Directed {
				head = "none"
			}
		}
		if tail == "" {
			tail = "none"
		}
		r["head"], r["tail"] = head, tail
		r["headFill"], r["headOpen"] = ArrowPaths(head)
		r["tailFill"], r["tailOpen"] = ArrowPaths(tail)
		size := ArrowSymbolSize
		if e.ArrowSize > 0 {
			size *= e.ArrowSize * e.ArrowSize
		}
		r["arrowSize"] = size
		link(r, e.Href, e.Tooltip)
		style(r, e.Style)
		add("edge", signature(r, []string{"stroke", "strokeWidth", "strokeDash"}, "_paint"))
		if len(e.Strokes) > 1 {
			// Each color but the one of the edge draws a line of its own,
			// which the layout writes as a row of the kind edgeLine.
			strokes := make([]any, len(e.Strokes))
			for k, c := range e.Strokes {
				strokes[k] = c
			}
			r["strokes"] = strokes
			if e.Weights != nil {
				weights := make([]any, len(e.Weights))
				for k, w := range e.Weights {
					weights[k] = w
				}
				r["weights"] = weights
			}
			line := map[string]any{"stroke": e.Strokes[0]}
			for _, k := range []string{"strokeWidth", "strokeDash"} {
				if v, ok := r[k]; ok {
					line[k] = v
				}
			}
			sig := signature(line, []string{"stroke", "strokeWidth", "strokeDash"}, "_paint")
			r["_linePaint"] = sig
			add("edgeLine", sig)
		}
		text := signature(r, textKeys, "_text")
		add("edgeText", text)
		if _, ok := r["xLines"]; ok {
			add("edgeXText", text)
		}
		arrow := signature(r, []string{"stroke"}, "_arrow")
		for _, part := range arrowFields {
			if r[part] != "" {
				add(part, arrow)
			}
		}
		// The labels at the ends take the paint of the label of the edge,
		// and LabelStyle over it.
		if _, ok := r["headLines"]; ok || r["tailLines"] != nil {
			end := map[string]any{}
			style(end, e.Style)
			style(end, e.LabelStyle)
			if v, ok := end["textColor"]; ok {
				r["endColor"] = v
			}
			if v, ok := end["fontSize"]; ok {
				r["endFontSize"] = v
			}
			add("endText", signature(r, []string{"endColor", "endFontSize"}, "_end"))
		}
		edges = append(edges, r)
	}
	packages, states := false, false
	for _, c := range g.Clusters {
		r := map[string]any{"key": c.ID, "label": c.Label, "lines": lines(c.Label), "parent": nil}
		switch c.Shape {
		case ClusterPackage:
			r["shape"], packages = ClusterPackage, true
		case ClusterState:
			r["shape"], states = ClusterState, true
		}
		if c.Parent != "" {
			r["parent"] = c.Parent
		}
		if c.Direction != "" {
			r["direction"] = c.Direction
		}
		if c.Algorithm != "" {
			r["algorithm"] = c.Algorithm
		}
		if c.HTML != "" {
			r["html"], r["label"], r["lines"] = c.HTML, "", []any{}
			html = true
		}
		link(r, c.Href, c.Tooltip)
		style(r, c.Style)
		categories = categoryRow(r, c.Category, categories, true)
		add("cluster", signature(r, paintKeys, "_paint"))
		add("clusterText", signature(r, textKeys, "_text"))
		clusters = append(clusters, r)
	}
	layout := map[string]any{"type": "zuzu.layered", "edges": "edges", "clusters": "clusters", "direction": dir}
	if g.RankSep > 0 {
		layout["rankSep"] = g.RankSep
	}
	if g.NodeSep > 0 {
		layout["nodeSep"] = g.NodeSep
	}
	if g.ClusterTitleTop > 0 {
		layout["clusterTitleTop"] = g.ClusterTitleTop
	}
	if g.ClusterTitleBottom > 0 {
		layout["clusterTitleBottom"] = g.ClusterTitleBottom
	}
	if g.BreakWords {
		layout["breakWords"] = true
	}
	if g.Lanes {
		layout["lanes"] = true
	}
	if g.LaneEdges {
		layout["laneEdges"] = true
	}
	if g.ClusterDirection != "" {
		layout["clusterDirection"] = g.ClusterDirection
	}
	if g.ReverseClusters {
		layout["reverseClusters"] = true
	}
	if len(g.ELK) > 0 {
		settings := make(map[string]any, len(g.ELK))
		for k, v := range g.ELK {
			settings[k] = v
		}
		layout["elk"] = settings
	}
	if len(g.Order) > 0 {
		order := make([]any, len(g.Order))
		for i, id := range g.Order {
			order[i] = id
		}
		layout["order"] = order
	}
	if g.Math {
		layout["math"] = true
		if src := mathSource(g); len(src) > 0 {
			layout["mathSource"] = src
		}
	}
	if g.TitleHTML != "" {
		// The title of HTML takes the size of the title of a chart.
		layout["titleHTML"], layout["titleFontSize"] = g.TitleHTML, zlayout.TitleSize
		html = true
	}
	transforms := []any{layout}
	if media {
		transforms = []any{map[string]any{"type": "zuzu.icon"}, layout}
	}
	filter := func(name, source, test string) map[string]any {
		return map[string]any{"name": name, "source": source, "transform": []any{map[string]any{"type": "filter", "expr": test}}}
	}
	data := []any{
		map[string]any{"name": "nodes", "values": nodes},
		map[string]any{"name": "edges", "values": edges},
		map[string]any{"name": "clusters", "values": clusters},
		map[string]any{"name": "layout", "source": "nodes", "transform": transforms},
		filter("layoutNodes", "layout", "datum.kind === 'node'"),
		filter("layoutEdges", "layout", "datum.kind === 'edge'"),
		filter("layoutClusters", "layout", "datum.kind === 'cluster' && datum.width > 0"),
		filter("layoutFields", "layout", "datum.kind === 'field'"),
	}
	var marks []any
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	encodeKeys := func(enc map[string]any, sig string, rename map[string]string) {
		if sig == "" {
			return
		}
		for k := range strings.SplitSeq(sig, ",") {
			if k == "category" {
				categoryEncode(enc)
				continue
			}
			if k == "note" || k == "solid" {
				// A note and a solid node take a style, not a field.
				continue
			}
			name := k
			if r, ok := rename[k]; ok {
				name = r
			}
			enc[name] = field(k)
		}
		solidEncode(enc, strings.Split(sig, ","))
	}
	mark := func(typ, from, sigField, sig string, styles []string, enc map[string]any, rename map[string]string) map[string]any {
		name := fmt.Sprintf("%s%d", from, len(marks))
		src := from
		if sig != "" || sigField != "" {
			src = name + "Data"
			data = append(data, filter(src, from, fmt.Sprintf("datum.%s === %q", sigField, sig)))
		}
		encodeKeys(enc, sig, rename)
		return map[string]any{"type": typ, "from": map[string]any{"data": src}, "style": styles, "encode": map[string]any{"update": enc}}
	}
	// A package draws as a tab that holds the title and a panel under it,
	// as a package of UML (D32). The panel draws over the foot of the tab,
	// so the tab has round corners only at its top.
	panels := "layoutClusters"
	if packages {
		panels = "layoutPanels"
		data = append(data,
			filter(panels, "layoutClusters", "datum.shape !== 'package'"),
			filter("layoutPackages", "layoutClusters", "datum.shape === 'package'"))
	}
	cluster := func(m map[string]any) map[string]any {
		m["role"] = scene.RoleCluster
		return m
	}
	for _, sig := range paints["cluster"] {
		marks = append(marks, cluster(mark("rect", panels, "_paint", sig, []string{StyleCluster}, map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}, nil)))
	}
	if packages {
		const tab = "(datum.titleY - datum.y + datum.titleHeight + 4)"
		signal := func(s string) map[string]any { return map[string]any{"signal": s} }
		for _, sig := range paints["cluster"] {
			marks = append(marks, cluster(mark("rect", "layoutPackages", "_paint", sig, []string{StyleCluster}, map[string]any{
				"x": field("x"), "y": field("y"), "width": signal("datum.titleWidth + 2 * (datum.titleX - datum.x)"),
				"height": signal(tab + " + 6"),
			}, nil)))
			marks = append(marks, cluster(mark("rect", "layoutPackages", "_paint", sig, []string{StyleCluster}, map[string]any{
				"x": field("x"), "y": signal("datum.y + " + tab), "width": field("width"),
				"height": signal("datum.height - " + tab),
			}, nil)))
		}
	}
	if states {
		// A composite state draws a band across the top of its panel, in
		// the paint of the panel, so the band takes a stronger tint and
		// its edge draws a line under the title.
		const band = "(datum.titleY - datum.y + datum.titleHeight + 6)"
		data = append(data, filter("layoutStates", "layoutClusters", "datum.shape === 'state'"))
		for _, sig := range paints["cluster"] {
			marks = append(marks, cluster(mark("rect", "layoutStates", "_paint", sig, []string{StyleCluster}, map[string]any{
				"x": field("x"), "y": field("y"), "width": field("width"), "height": map[string]any{"signal": band},
			}, nil)))
		}
	}
	for _, sig := range paints["clusterText"] {
		marks = append(marks, mark("text", "layoutClusters", "_text", sig, []string{StyleClusterTitle}, map[string]any{
			"x": field("titleX"), "y": field("titleY"), "text": field("lines"), "align": map[string]any{"value": "left"}, "baseline": map[string]any{"value": "top"},
		}, map[string]string{"textColor": "fill"}))
	}
	if len(paints["edgeLine"]) > 0 {
		// The lines of the colors of an edge of more than one color.
		data = append(data, filter("layoutEdgeLines", "layout", "datum.kind === 'edgeLine'"))
	}
	for _, sig := range paints["edgeLine"] {
		m := mark("path", "layoutEdgeLines", "_paint", sig, []string{StyleEdge}, map[string]any{
			"x": map[string]any{"value": 0}, "y": map[string]any{"value": 0}, "path": field("path"),
		}, nil)
		m["role"] = scene.RoleEdge
		marks = append(marks, m)
	}
	for _, sig := range paints["edge"] {
		m := mark("path", "layoutEdges", "_paint", sig, []string{StyleEdge}, map[string]any{
			"x": map[string]any{"value": 0}, "y": map[string]any{"value": 0}, "path": field("path"),
		}, nil)
		m["role"] = scene.RoleEdge
		marks = append(marks, m)
	}
	data = append(data, filter("edgeLabels", "layoutEdges", "datum.labelWidth > 0"))
	marks = append(marks, map[string]any{"type": "rect", "from": map[string]any{"data": "edgeLabels"}, "style": []string{StyleEdgeLabelBox}, "encode": map[string]any{"update": map[string]any{
		"xc": field("labelX"), "yc": field("labelY"), "width": field("labelWidth"), "height": field("labelHeight"),
	}}})
	for _, sig := range paints["edgeText"] {
		marks = append(marks, mark("text", "edgeLabels", "_text", sig, []string{StyleEdgeLabel}, map[string]any{
			"x": field("labelX"), "y": field("labelLineY"), "text": field("lines"), "align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
		}, map[string]string{"textColor": "fill"}))
	}
	// The labels that take no space in the ranks: at the ends of an edge,
	// and the xlabels.
	labelMarks := func(source, kind, sigField string, sigs []string, styles []string, rename map[string]string) {
		if len(sigs) == 0 {
			return
		}
		name := kind + "Labels"
		if source == "layoutNodes" {
			name = "nodeXLabels"
		}
		data = append(data, filter(name, source, fmt.Sprintf("datum.%sLabelWidth > 0", kind)))
		for _, sig := range sigs {
			marks = append(marks, mark("text", name, sigField, sig, styles, map[string]any{
				"x": field(kind + "LabelX"), "y": field(kind + "LabelLineY"), "text": field(kind + "Lines"),
				"align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
			}, rename))
		}
	}
	endRename := map[string]string{"endColor": "fill", "endFontSize": "fontSize"}
	labelMarks("layoutEdges", "head", "_end", paints["endText"], []string{StyleEdgeLabel}, endRename)
	labelMarks("layoutEdges", "tail", "_end", paints["endText"], []string{StyleEdgeLabel}, endRename)
	labelMarks("layoutEdges", "x", "_text", paints["edgeXText"], []string{StyleEdgeLabel}, map[string]string{"textColor": "fill"})
	for _, sig := range paints["node"] {
		styles := []string{StyleNode}
		if slices.Contains(strings.Split(sig, ","), "note") {
			styles = append(styles, theme.StyleNote)
		}
		m := mark("path", "layoutNodes", "_paint", sig, styles, map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"),
		}, nil)
		if slices.Contains(strings.Split(sig, ","), "solid") {
			m["style"] = append(styles, theme.StyleNodeSolid)
			m["role"] = scene.RoleSolid
		}
		marks = append(marks, m)
	}
	if media {
		data = append(data,
			filter("layoutForms", "layoutNodes", "datum.formPath"),
			filter("iconFill", "layoutNodes", "datum.iconKind === 'fill'"),
			filter("iconStroke", "layoutNodes", "datum.iconKind === 'stroke'"),
			filter("iconImage", "layoutNodes", "datum.iconKind === 'image'"),
		)
		for _, sig := range paints["node"] {
			marks = append(marks, mark("path", "layoutForms", "_paint", sig, []string{StyleNode}, map[string]any{
				"x": field("mediaX"), "y": field("mediaY"), "path": field("formPath"),
			}, nil))
		}
		// A figure, such as an actor, draws in the paint of its node.
		if figures {
			data = append(data, filter("iconFigure", "layoutNodes", "datum.iconKind === 'figure'"))
			for _, sig := range paints["node"] {
				marks = append(marks, mark("path", "iconFigure", "_paint", sig, []string{StyleNode}, map[string]any{
					"x": field("mediaX"), "y": field("mediaY"), "path": field("iconPath"),
				}, nil))
			}
		}
		marks = append(marks,
			map[string]any{"type": "path", "from": map[string]any{"data": "iconFill"}, "style": []string{StyleIcon}, "encode": map[string]any{"update": map[string]any{
				"x": field("mediaX"), "y": field("mediaY"), "path": field("iconPath"),
			}}},
			map[string]any{"type": "path", "from": map[string]any{"data": "iconStroke"}, "style": []string{StyleIconStroke}, "encode": map[string]any{"update": map[string]any{
				"x": field("mediaX"), "y": field("mediaY"), "path": field("iconPath"), "strokeWidth": field("iconStrokeWidth"),
			}}},
			map[string]any{"type": "image", "from": map[string]any{"data": "iconImage"}, "encode": map[string]any{"update": map[string]any{
				"x": field("mediaLeft"), "y": field("mediaTop"), "width": field("mediaWidth"), "height": field("mediaHeight"),
				"url": field("iconURL"), "aspect": field("mediaAspect"),
			}}},
		)
	}
	var htmlText []any
	if html {
		shapes, texts := htmlMarks(&data)
		marks = append(marks, shapes...)
		htmlText = texts
	}
	// The arrowheads draw over the shapes of the nodes, so that the tip of
	// each one sits on the outline of its node. The lines of the edges
	// draw under the nodes, so a line that passes under a node stays
	// hidden.
	for _, part := range arrowFields {
		at := map[bool][3]string{true: {"endX", "endY", "endAngle"}, false: {"startX", "startY", "startAngle"}}[strings.HasPrefix(part, "head")]
		open := strings.HasSuffix(part, "Open")
		st := []string{StyleArrow}
		if open {
			st = []string{StyleArrowOpen}
		}
		for _, sig := range paints[part] {
			enc := map[string]any{
				"x": field(at[0]), "y": field(at[1]), "angle": field(at[2]), "shape": field(part), "size": field("arrowSize"),
			}
			if sig == "stroke" {
				enc["stroke"] = field("stroke")
				if !open {
					enc["fill"] = field("stroke")
				}
			}
			src := fmt.Sprintf("arrows%d", len(marks))
			data = append(data, filter(src, "layoutEdges", fmt.Sprintf("datum.%s !== '' && datum._arrow === %q", part, sig)))
			marks = append(marks, map[string]any{"type": "symbol", "role": scene.RoleEdge, "from": map[string]any{"data": src}, "style": st, "encode": map[string]any{"update": enc}})
		}
	}
	labelMarks("layoutNodes", "x", "_text", paints["nodeXText"], []string{StyleNodeLabel}, map[string]string{"textColor": "fill"})
	marks = append(marks, htmlText...)
	for _, sig := range paints["nodeText"] {
		marks = append(marks, mark("text", "layoutNodes", "_text", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("textX"), "y": field("textLineY"), "text": field("lines"), "align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
		}, map[string]string{"textColor": "fill"}))
	}
	// The text of the fields of a record: each row sets its alignment and
	// its style of font, and a line under a line of text is a thin box.
	for _, sig := range paints["fieldText"] {
		marks = append(marks, mark("text", "layoutFields", "_text", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("x"), "y": field("y"), "text": field("lines"), "align": field("align"), "baseline": map[string]any{"value": "middle"},
			"fontStyle": field("fontStyle"),
		}, map[string]string{"textColor": "fill"}))
	}
	if len(paints["fieldDeco"]) > 0 {
		data = append(data, filter("layoutFieldDecos", "layout", "datum.kind === 'fieldDeco'"))
	}
	for _, sig := range paints["fieldDeco"] {
		marks = append(marks, mark("rect", "layoutFieldDecos", "_deco", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}, map[string]string{"textColor": "fill"}))
	}
	marks = append(marks, mathMarks(&data, mathParts(g))...)
	if !links {
		links = htmlLinks(g)
	}
	if links {
		// Each mark draws a part of a node, an edge or a cluster, and each
		// row of the layout holds the link and the tooltip of its part.
		for _, m := range marks {
			mm, _ := m.(map[string]any)
			sets, _ := mm["encode"].(map[string]any)
			if enc, ok := sets["update"].(map[string]any); ok {
				enc["href"], enc["tooltip"] = field("href"), field("tooltip")
			}
		}
	}
	padding := 8.0
	if g.Padding != nil {
		padding = *g.Padding
	}
	spec := map[string]any{
		"$schema":  "https://vega.github.io/schema/vega/v5.json",
		"padding":  padding,
		"autosize": "pad",
		"data":     data,
		"marks":    marks,
	}
	if g.Title != "" && g.TitleHTML == "" {
		title := map[string]any{"text": g.Title, "anchor": "start"}
		for _, k := range []string{"fontWeight", "fontStyle"} {
			if v := g.TitleStyle[k]; v != "" {
				title[k] = v
			}
		}
		if g.TitleOffset != nil {
			title["offset"] = *g.TitleOffset
		}
		spec["title"] = title
	}
	if g.Description != "" {
		spec["description"] = g.Description
	}
	if g.Background != "" {
		spec["config"] = map[string]any{theme.ConfigBackground: g.Background}
	}
	if scales := categoryScales(categories); scales != nil {
		spec["scales"] = scales
	}
	// The link and the tooltip of the diagram go on the group of the whole
	// diagram.
	root := map[string]any{}
	if g.Href != "" {
		root["href"] = map[string]any{"value": g.Href}
	}
	if g.Tooltip != "" {
		root["tooltip"] = map[string]any{"value": g.Tooltip}
	}
	if len(root) > 0 {
		spec["encode"] = map[string]any{"update": root}
	}
	b, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("writing the spec of a graph: %w", err)
	}
	return vega.Parse(b)
}

// fieldRow writes the fields of a record into the row of its node: the
// text and the port of each field, and the cells, the alignment, the fonts,
// the runs and the groups of the fields when any field sets them. It
// reports whether a field has runs.
func fieldRow(r map[string]any, n *Node) bool {
	runs := writeFields(r, n.Fields)
	r["lines"] = []any{}
	return runs
}

// writeFields writes a list of fields into a row, as fieldRow says. The
// list groups holds, for each field, nil or the row of the fields of its
// group in braces, which writeFields writes the same way. The list
// fieldRuns holds, for each field, the runs of each line or cell, or nil
// for one with no runs. writeFields reports whether a field has runs.
func writeFields(r map[string]any, fs []Field) bool {
	var texts, ports, cells, aligns, fonts, runs, groups []any
	table, align, font, run, group := false, false, false, false, false
	for _, f := range fs {
		var rs []any
		for _, l := range f.Runs {
			var row any
			if len(l) > 0 {
				row, run = runRows(l), true
			}
			rs = append(rs, row)
		}
		runs = append(runs, rs)
		texts, ports = append(texts, f.Text), append(ports, f.Port)
		var cs []any
		for _, c := range f.Cells {
			cs = append(cs, c)
		}
		cells = append(cells, cs)
		aligns = append(aligns, f.Align)
		var fts []any
		for _, ft := range f.Fonts {
			fts = append(fts, fontCode(ft))
		}
		fonts = append(fonts, fts)
		var sub any
		if f.Fields != nil {
			m := map[string]any{}
			run = writeFields(m, f.Fields) || run
			sub, group = m, true
		}
		groups = append(groups, sub)
		table = table || len(f.Cells) > 0
		align = align || f.Align != ""
		font = font || len(f.Fonts) > 0
	}
	r["fields"], r["ports"] = texts, ports
	if table {
		r["cells"] = cells
	}
	if align {
		r["aligns"] = aligns
	}
	if font {
		r["fonts"] = fonts
	}
	if run {
		r["fieldRuns"] = runs
	}
	if group {
		r["groups"] = groups
	}
	return run
}

// htmlMarks returns the marks that draw the labels of HTML that the
// transform zuzu.layered lays out (D31): the shapes, which are their boxes
// and their rules, and the texts, which are the lines of their text and
// their runs of text. The arrowheads draw between the two. A row of the
// layout leaves out each paint that the theme gives, and the field _paint
// lists the paints that it sets, so a mark for each list draws its rows.
func htmlMarks(data *[]any) (shapes, texts []any) {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	var marks []any
	add := func(typ, kind, sig string, styles []string, enc map[string]any, keys ...string) {
		name := fmt.Sprintf("html%d", len(marks))
		test := fmt.Sprintf("datum.kind === %q && datum._paint === %q", kind, sig)
		*data = append(*data, map[string]any{"name": name, "source": "layout", "transform": []any{map[string]any{"type": "filter", "expr": test}}})
		e := map[string]any{}
		maps.Copy(e, enc)
		for _, k := range keys {
			if slices.Contains(strings.Split(sig, ","), k) {
				e[k] = field(k)
			}
		}
		// A fill of a label of HTML is opaque, as the fill of a filled
		// node of DOT is.
		if typ == "rect" && e["fill"] != nil {
			e["fillOpacity"] = map[string]any{"value": 1}
		}
		marks = append(marks, map[string]any{"type": typ, "from": map[string]any{"data": name}, "style": styles, "encode": map[string]any{"update": e}})
	}
	both := []string{"", "fill", "stroke", "fill,stroke"}
	for _, sig := range both {
		add("rect", "htmlBox", sig, []string{StyleNode}, map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
			"strokeWidth": field("strokeWidth"), "strokeDash": field("strokeDash"), "cornerRadius": field("cornerRadius"),
		}, "fill", "stroke")
	}
	for _, sig := range []string{"", "stroke"} {
		add("rule", "htmlRule", sig, []string{StyleNode}, map[string]any{
			"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2"),
			"strokeWidth": field("strokeWidth"), "strokeDash": field("strokeDash"),
		}, "stroke")
	}
	n := len(marks)
	for _, sig := range []string{"", "fill"} {
		add("rect", "htmlDeco", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}, "fill")
	}
	for _, sig := range []string{"", "fill", "font", "fill,font"} {
		add("text", "htmlRun", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
			"fontWeight": field("fontWeight"), "fontStyle": field("fontStyle"),
			"align": map[string]any{"value": "left"}, "baseline": map[string]any{"value": "middle"},
		}, "fill", "font")
	}
	return marks[:n], marks[n:]
}

// mathSource returns the labels of a graph that draw their source in
// place of their math, sorted and once each.
func mathSource(g *Graph) []any {
	src := slices.Clone(g.MathSource)
	slices.Sort(src)
	var out []any
	for _, s := range slices.Compact(src) {
		out = append(out, s)
	}
	return out
}

// mathParts returns the parts of a graph that hold a label that draws as
// math, in the order that they draw: cluster for the title of a cluster,
// edge for a label of an edge, and node for a label of a node or of a
// field. A label draws as math when it holds math of KaTeX and is not in
// MathSource, as the transform zuzu.layered decides (D36).
func mathParts(g *Graph) []string {
	if !g.Math {
		return nil
	}
	draws := func(texts ...string) bool {
		for _, s := range texts {
			if text.HasMath(s) && !slices.Contains(g.MathSource, s) {
				return true
			}
		}
		return false
	}
	var parts []string
	for _, c := range g.Clusters {
		if draws(c.Label) {
			parts = append(parts, "cluster")
			break
		}
	}
	for _, e := range g.Edges {
		if draws(e.Label, e.HeadLabel, e.TailLabel, e.XLabel) {
			parts = append(parts, "edge")
			break
		}
	}
	for _, n := range g.Nodes {
		// Each line of a field is a label of its own.
		texts := []string{n.Label, n.ID, n.XLabel}
		for _, f := range n.Fields {
			for _, s := range append([]string{f.Text}, f.Cells...) {
				texts = append(texts, strings.Split(s, "\n")...)
			}
		}
		if draws(texts...) {
			parts = append(parts, "node")
			break
		}
	}
	return parts
}

// mathMarks returns the marks that draw the labels with math that the
// transform zuzu.layered lays out (D36), for each part of mathParts: the
// runs of text of each label in its font, and its formulas, as a mark of
// type path with the role math. Each path holds the TeX of its formula as
// its text, which a path does not draw, and which the SVG writes as its
// label. A row sets fill when its label or its formula has a color of its
// own, and its field _paint says so. The other rows take the color of the
// style of their label.
func mathMarks(data *[]any, parts []string) []any {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	styles := map[string]string{"cluster": StyleClusterTitle, "edge": StyleEdgeLabel, "node": StyleNodeLabel}
	var marks []any
	for _, part := range parts {
		for _, sig := range []string{"", "fill"} {
			for _, kind := range []string{"mathText", "mathPath"} {
				name := fmt.Sprintf("math%d", len(marks))
				test := fmt.Sprintf("datum.kind === %q && datum.part === %q && datum._paint === %q", kind, part, sig)
				*data = append(*data, map[string]any{"name": name, "source": "layout", "transform": []any{map[string]any{"type": "filter", "expr": test}}})
				enc := map[string]any{"x": field("x"), "y": field("y")}
				typ := "text"
				if kind == "mathText" {
					enc["text"], enc["fontSize"], enc["fontWeight"], enc["fontStyle"] = field("text"), field("fontSize"), field("fontWeight"), field("fontStyle")
					enc["align"], enc["baseline"] = map[string]any{"value": "left"}, map[string]any{"value": "middle"}
				} else {
					typ = "path"
					// A formula is a filled outline, with no stroke of the
					// config of a path.
					enc["path"], enc["text"], enc["stroke"] = field("path"), field("tex"), map[string]any{"value": nil}
				}
				if sig == "fill" {
					enc["fill"] = field("fill")
				}
				m := map[string]any{"type": typ, "from": map[string]any{"data": name}, "style": []string{styles[part]}, "encode": map[string]any{"update": enc}}
				if typ == "path" {
					m["role"] = scene.RoleMath
				}
				marks = append(marks, m)
			}
		}
	}
	return marks
}

// arrowFields are the fields of a row of an edge that hold the paths of its
// arrowheads: the filled shapes and the open shapes at each end.
var arrowFields = []string{"headFill", "tailFill", "headOpen", "tailOpen"}

// labelRow writes a label that takes no space in the ranks into a row: the
// text in the field name+"Label" and its lines in name+"Lines", or the
// source of HTML in name+"HTML" with no lines. It reports whether the
// label is HTML.
func labelRow(r map[string]any, name, text, src string) bool {
	switch {
	case src != "":
		r[name+"HTML"], r[name+"Lines"] = src, []any{}
		return true
	case text != "":
		r[name+"Label"], r[name+"Lines"] = text, lines(text)
	}
	return false
}

// media2row writes the icon, the image or the figure of a node into its
// row, for the transform zuzu.icon and the layout.
func media2row(r map[string]any, m *Media) {
	switch {
	case m.Path != "":
		// The transform zuzu.icon leaves a figure as it is.
		r["iconKind"], r["iconPath"], r["mediaWidth"], r["mediaHeight"] = "figure", m.Path, m.Width, m.Height
	case m.Icon != "":
		r["icon"] = m.Icon
		size := m.Height
		if m.Inline {
			size = 14
		}
		if size > 0 {
			r["iconSize"] = size
		}
	default:
		r["image"] = m.Image
		if m.Width > 0 {
			r["imageWidth"] = m.Width
		}
		if m.Height > 0 {
			r["imageHeight"] = m.Height
		}
		r["imageAspect"] = m.Keep
	}
	if m.Form != "" {
		r["iconForm"] = m.Form
	}
	if m.Pos != "" {
		r["iconPos"] = m.Pos
	}
	if m.Inline {
		r["iconInline"] = true
	}
}
