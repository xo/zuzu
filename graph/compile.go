package graph

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	zlayout "github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// The styles of the marks of a diagram, which the config of a theme gives
// (D14, D31).
const (
	StyleNode         = theme.StyleNode
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

// paintKeys are the keys of a style of a node, an edge or a cluster that
// a mark encodes, by the name of their field.
var paintKeys = []string{"fill", "fillOpacity", "stroke", "strokeWidth", "strokeDash"}

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

// shiftDash returns a dash that starts off along a dash, as the offset of
// a dash of SVG does, since the scene has no offset of a dash. When the
// offset falls in a dash, the dash starts with the rest of that dash and
// ends with its first part and a gap of 0, so that the two parts join.
// When it falls in a gap, the dash starts at the next dash, since a dash
// of 0 draws a dot with round ends: the dashes come up to a gap early.
func shiftDash(dash []any, off float64) []any {
	var d []float64
	for _, v := range dash {
		f, _ := v.(float64)
		d = append(d, f)
	}
	if len(d)%2 == 1 {
		// SVG repeats a dash of an odd length to make it even.
		d = append(d, d...)
	}
	var period float64
	for _, v := range d {
		period += v
	}
	if !(period > 0) {
		return dash
	}
	off = math.Mod(off, period)
	if off < 0 {
		off += period
	}
	if off == 0 {
		return dash
	}
	// Find the dash or the gap where the offset falls.
	i := 0
	for ; i < len(d)-1 && off >= d[i]; i++ {
		off -= d[i]
	}
	var out []float64
	if i%2 == 0 {
		out = append(out, d[i]-off)
		out = append(out, d[i+1:]...)
		out = append(out, d[:i]...)
		out = append(out, off, 0)
	} else {
		out = append(out, d[i+1:]...)
		out = append(out, d[:i+1]...)
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
	media, html := false, false
	paints := map[string][]string{}
	add := func(kind, sig string) {
		if !slices.Contains(paints[kind], sig) {
			paints[kind] = append(paints[kind], sig)
		}
	}
	for _, n := range g.Nodes {
		label := n.Label
		if label == "" && (n.Width <= 0 || n.Height <= 0) {
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
			fieldRow(r, n)
			if n.Stack {
				r["stack"] = true
			}
		}
		if n.HTML != "" {
			r["html"], r["lines"] = n.HTML, []any{}
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
		if n.Wrap > 0 && len(n.Fields) == 0 && n.HTML == "" {
			r["wrap"] = n.Wrap
		}
		if m := n.Media; m != nil {
			media = true
			media2row(r, m)
		}
		if labelRow(r, "x", n.XLabel, n.XHTML) {
			html = true
		}
		style(r, n.Style)
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
		if e.Weight > 0 {
			r["weight"] = e.Weight
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
		size := 36.0
		if e.ArrowSize > 0 {
			size *= e.ArrowSize * e.ArrowSize
		}
		r["arrowSize"] = size
		style(r, e.Style)
		add("edge", signature(r, []string{"stroke", "strokeWidth", "strokeDash"}, "_paint"))
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
	for _, c := range g.Clusters {
		r := map[string]any{"key": c.ID, "label": c.Label, "parent": nil}
		if c.Parent != "" {
			r["parent"] = c.Parent
		}
		if c.Direction != "" {
			r["direction"] = c.Direction
		}
		if c.HTML != "" {
			r["html"], r["label"] = c.HTML, ""
			html = true
		}
		style(r, c.Style)
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
			name := k
			if r, ok := rename[k]; ok {
				name = r
			}
			enc[name] = field(k)
		}
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
	for _, sig := range paints["cluster"] {
		marks = append(marks, mark("rect", "layoutClusters", "_paint", sig, []string{StyleCluster}, map[string]any{
			"x": field("x"), "y": field("y"), "width": field("width"), "height": field("height"),
		}, nil))
	}
	for _, sig := range paints["clusterText"] {
		marks = append(marks, mark("text", "layoutClusters", "_text", sig, []string{StyleClusterTitle}, map[string]any{
			"x": field("titleX"), "y": field("titleY"), "text": field("label"), "align": map[string]any{"value": "left"}, "baseline": map[string]any{"value": "top"},
		}, map[string]string{"textColor": "fill"}))
	}
	for _, sig := range paints["edge"] {
		marks = append(marks, mark("path", "layoutEdges", "_paint", sig, []string{StyleEdge}, map[string]any{
			"x": map[string]any{"value": 0}, "y": map[string]any{"value": 0}, "path": field("path"),
		}, nil))
	}
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
			marks = append(marks, map[string]any{"type": "symbol", "from": map[string]any{"data": src}, "style": st, "encode": map[string]any{"update": enc}})
		}
	}
	data = append(data, filter("edgeLabels", "layoutEdges", "datum.labelWidth > 0"))
	marks = append(marks, map[string]any{"type": "rect", "from": map[string]any{"data": "edgeLabels"}, "style": []string{StyleEdgeLabelBox}, "encode": map[string]any{"update": map[string]any{
		"xc": field("labelX"), "yc": field("labelY"), "width": field("labelWidth"), "height": field("labelHeight"),
	}}})
	for _, sig := range paints["edgeText"] {
		marks = append(marks, mark("text", "edgeLabels", "_text", sig, []string{StyleEdgeLabel}, map[string]any{
			"x": field("labelX"), "y": field("labelY"), "text": field("lines"), "align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
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
				"x": field(kind + "LabelX"), "y": field(kind + "LabelY"), "text": field(kind + "Lines"),
				"align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
			}, rename))
		}
	}
	endRename := map[string]string{"endColor": "fill", "endFontSize": "fontSize"}
	labelMarks("layoutEdges", "head", "_end", paints["endText"], []string{StyleEdgeLabel}, endRename)
	labelMarks("layoutEdges", "tail", "_end", paints["endText"], []string{StyleEdgeLabel}, endRename)
	labelMarks("layoutEdges", "x", "_text", paints["edgeXText"], []string{StyleEdgeLabel}, map[string]string{"textColor": "fill"})
	for _, sig := range paints["node"] {
		marks = append(marks, mark("path", "layoutNodes", "_paint", sig, []string{StyleNode}, map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"),
		}, nil))
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
	labelMarks("layoutNodes", "x", "_text", paints["nodeXText"], []string{StyleNodeLabel}, map[string]string{"textColor": "fill"})
	if html {
		marks = append(marks, htmlMarks(&data)...)
	}
	for _, sig := range paints["nodeText"] {
		marks = append(marks, mark("text", "layoutNodes", "_text", sig, []string{StyleNodeLabel}, map[string]any{
			"x": field("textX"), "y": field("textY"), "text": field("lines"), "align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
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
		if g.TitleOffset != nil {
			title["offset"] = *g.TitleOffset
		}
		spec["title"] = title
	}
	if g.Description != "" {
		spec["description"] = g.Description
	}
	b, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("writing the spec of a graph: %w", err)
	}
	return vega.Parse(b)
}

// fieldRow writes the fields of a record into the row of its node: the
// text and the port of each field, and the cells, the alignment and the
// fonts of the fields when any field sets them.
func fieldRow(r map[string]any, n *Node) {
	var texts, ports, cells, aligns, fonts []any
	table, align, font := false, false, false
	for _, f := range n.Fields {
		texts, ports = append(texts, f.Text), append(ports, f.Port)
		var cs []any
		for _, c := range f.Cells {
			cs = append(cs, c)
		}
		cells = append(cells, cs)
		aligns = append(aligns, f.Align)
		var fs []any
		for _, ft := range f.Fonts {
			fs = append(fs, fontCode(ft))
		}
		fonts = append(fonts, fs)
		table = table || len(f.Cells) > 0
		align = align || f.Align != ""
		font = font || len(f.Fonts) > 0
	}
	r["fields"], r["ports"], r["lines"] = texts, ports, []any{}
	if table {
		r["cells"] = cells
	}
	if align {
		r["aligns"] = aligns
	}
	if font {
		r["fonts"] = fonts
	}
}

// htmlMarks returns the marks that draw the labels of HTML that the
// transform zuzu.layered lays out (D31): their boxes, their rules, the lines
// of their text, and their runs of text. A row of the layout leaves out
// each paint that the theme gives, and the field _paint lists the paints
// that it sets, so a mark for each list draws its rows.
func htmlMarks(data *[]any) []any {
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

// media2row writes the icon or the image of a node into its row, for the
// transform zuzu.icon and the layout.
func media2row(r map[string]any, m *Media) {
	if m.Icon != "" {
		r["icon"] = m.Icon
		size := m.Height
		if m.Inline {
			size = 14
		}
		if size > 0 {
			r["iconSize"] = size
		}
	} else {
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
