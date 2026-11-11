package graph

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

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
)

// paintKeys are the keys of a style of a node, an edge or a cluster that
// a mark encodes, by the name of their field.
var paintKeys = []string{"fill", "fillOpacity", "stroke", "strokeWidth", "strokeDash"}

// textKeys are the keys of a style of a label.
var textKeys = []string{"textColor", "fontSize", "fontWeight"}

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

// style copies the paint of a style into a row, with the names of Vega.
func style(r map[string]any, st map[string]string) {
	for k, v := range st {
		switch k {
		case "fill", "stroke", "color", "fontWeight":
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
		if label == "" && n.Label == "" {
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
			var texts, ports []any
			for _, f := range n.Fields {
				texts, ports = append(texts, f.Text), append(ports, f.Port)
			}
			r["fields"], r["ports"], r["lines"] = texts, ports, []any{}
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
		if m := n.Media; m != nil {
			media = true
			media2row(r, m)
		}
		style(r, n.Style)
		add("node", signature(r, paintKeys, "_paint"))
		add("nodeText", signature(r, textKeys, "_text"))
		nodes = append(nodes, r)
	}
	for i, e := range g.Edges {
		r := map[string]any{"key": strconv.Itoa(i), "source": e.Source, "target": e.Target}
		if e.Label != "" {
			r["label"], r["lines"] = e.Label, lines(e.Label)
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
		hp, hf := ArrowPath(head)
		tp, tf := ArrowPath(tail)
		r["headPath"], r["tailPath"] = hp, tp
		style(r, e.Style)
		add("edge", signature(r, []string{"stroke", "strokeWidth", "strokeDash"}, "_paint"))
		add("edgeText", signature(r, textKeys, "_text"))
		for _, end := range []struct {
			name, kind string
			filled     bool
		}{{"_head", head, hf}, {"_tail", tail, tf}} {
			sig := "open"
			if end.filled {
				sig = "filled"
			}
			if _, ok := r["stroke"]; ok {
				sig += ",stroke"
			}
			r[end.name] = sig
			if end.kind != "none" {
				add("arrow", sig)
			}
		}
		edges = append(edges, r)
	}
	for _, c := range g.Clusters {
		r := map[string]any{"key": c.ID, "label": c.Label, "parent": nil}
		if c.Parent != "" {
			r["parent"] = c.Parent
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
	for _, sig := range paints["arrow"] {
		for _, end := range []struct{ name, x, y, angle, path, kind string }{
			{"_head", "endX", "endY", "endAngle", "headPath", "head"}, {"_tail", "startX", "startY", "startAngle", "tailPath", "tail"},
		} {
			st := []string{StyleArrow}
			if strings.HasPrefix(sig, "open") {
				st = []string{StyleArrowOpen}
			}
			enc := map[string]any{
				"x": field(end.x), "y": field(end.y), "angle": field(end.angle), "shape": field(end.path),
				"size": map[string]any{"value": 36},
			}
			if strings.HasSuffix(sig, ",stroke") {
				enc["stroke"] = field("stroke")
				if !strings.HasPrefix(sig, "open") {
					enc["fill"] = field("stroke")
				}
			}
			src := fmt.Sprintf("arrows%d", len(marks))
			data = append(data, filter(src, "layoutEdges", fmt.Sprintf("datum.%s !== 'none' && datum.%s === %q", end.kind, end.name, sig)))
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
	if html {
		marks = append(marks, htmlMarks(&data)...)
	}
	records := slices.ContainsFunc(g.Nodes, func(n *Node) bool { return len(n.Fields) > 0 })
	for _, sig := range paints["nodeText"] {
		for _, from := range []string{"layoutNodes", "layoutFields"} {
			if from == "layoutFields" && !records {
				continue
			}
			x, y := "textX", "textY"
			if from == "layoutFields" {
				x, y = "x", "y"
			}
			marks = append(marks, mark("text", from, "_text", sig, []string{StyleNodeLabel}, map[string]any{
				"x": field(x), "y": field(y), "text": field("lines"), "align": map[string]any{"value": "center"}, "baseline": map[string]any{"value": "middle"},
			}, map[string]string{"textColor": "fill"}))
		}
	}
	spec := map[string]any{
		"$schema":  "https://vega.github.io/schema/vega/v5.json",
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"marks":    marks,
	}
	if g.Title != "" {
		spec["title"] = map[string]any{"text": g.Title, "anchor": "start"}
	}
	b, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("writing the spec of a graph: %w", err)
	}
	return vega.Parse(b)
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
