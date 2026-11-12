package mermaid

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// mindmapNode is a node of a mindmap.
type mindmapNode struct {
	id    int
	label string
	// soft is true for a label that runs over more lines of the source.
	// Its new lines are spaces when the label fits on one line, as in
	// Mermaid.
	soft bool
	// shape is default, rect, rounded, circle, bang, cloud or hexagon.
	shape  string
	icon   string
	indent int
	line   int
	parent *mindmapNode
	kids   []*mindmapNode
	depth  int
	// color is the key of the color of the node in the scale: 0 for the
	// root, and then 1 and on for each branch, or for each depth.
	color int

	// The layout: the lines of the label, the size of the label, of the
	// icon and of the shape, the center, the side of the root, +1 for the
	// right and -1 for the left, and the height of the band of the subtree.
	lines        []textLine
	tw, th, isz  float64
	w, h, x, y   float64
	side         int
	band         float64
	size, weight float64
}

// mindmapIcon matches the icon of a node, ::icon(fa fa-book).
var mindmapIcon = regexp.MustCompile(`^::icon\((.*)\)$`)

// mindmapEnds are the tokens that end the text of a node.
var mindmapEnds = []string{"))", ")", "]", "}}", "((", "("}

// The sizes of the layout of a mindmap.
const (
	// mindmapFont and mindmapRootFont are the sizes of the labels.
	mindmapFont     = 11.0
	mindmapRootFont = 13.0
	// mindmapLevelGap is the space between a node and its children, and
	// mindmapRootGap that between the root and its children.
	mindmapLevelGap = 28.0
	mindmapRootGap  = 40.0
	// mindmapSiblingGap is the space between two subtrees of one parent,
	// and mindmapBranchGap that between two branches of the root.
	mindmapSiblingGap = 8.0
	mindmapBranchGap  = 18.0
	// mindmapColors is the number of the colors of the branches, as in
	// Mermaid.
	mindmapColors = 11
)

// mindmapChart compiles a mindmap: a tree of nodes by their indent, with
// shapes, icons and markdown, into a Vega spec of paths and text, laid out
// in Go (D31). The root is in the center, and its branches split to its
// two sides, each a tidy tree that grows away from the root.
func mindmapChart(c *chart) (map[string]any, bool, error) {
	if err := c.unreadSettings("mindmap.padding", "mindmap.maxNodeWidth", "mindmap.colorBy", "mindmap.layoutAlgorithm", "layout",
		"htmlLabels", "flowchart.htmlLabels"); err != nil {
		return nil, false, err
	}
	breakWords, err := c.mindmapHTML()
	if err != nil {
		return nil, false, err
	}
	for _, k := range []string{"layout", "mindmap.layoutAlgorithm"} {
		v := str(c.d.Config, strings.Split(k, ".")...)
		if v != "" && v != "cose-bilkent" && v != "tidy-tree" && !hint(k, v) {
			if err := c.add("the setting %s %s", k, v); err != nil {
				return nil, false, err
			}
		}
	}
	byDepth := false
	switch v := str(c.d.Config, "mindmap", "colorBy"); v {
	case "", "branch":
	case "depth":
		byDepth = true
	default:
		if err := c.add("the setting mindmap.colorBy %s", v); err != nil {
			return nil, false, err
		}
	}
	pad := 10.0
	if v, ok := configNumber(c.d.Config, "mindmap", "padding"); ok && v >= 0 {
		pad = v
	}
	maxWidth := 200.0
	if v, ok := configNumber(c.d.Config, "mindmap", "maxNodeWidth"); ok && v > 0 {
		maxWidth = v
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	nodes, err := c.mindmapNodes()
	if err != nil {
		return nil, false, err
	}
	c.line = 0
	root := nodes[0]
	for i, k := range root.kids {
		mindmapColor(k, i%mindmapColors+1, byDepth)
	}
	m := text.NewEstimate()
	for _, n := range nodes {
		mindmapMeasure(n, pad, maxWidth, breakWords, m)
	}
	mindmapPlace(root)
	return mindmapSpec(nodes), false, nil
}

// mindmapNodes reads the nodes of a mindmap, each with its parent, its
// icon and its classes. The first node is the root.
func (c *chart) mindmapNodes() ([]*mindmapNode, error) {
	var nodes []*mindmapNode
	lines := c.d.Lines
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		c.line = l.N
		t := l.Text
		var last *mindmapNode
		if len(nodes) > 0 {
			last = nodes[len(nodes)-1]
		}
		if rest, ok := strings.CutPrefix(t, ":::"); ok {
			if last == nil {
				return nil, c.errorf("a class before the first node")
			}
			// The classes of a mindmap are classes of CSS that the page of
			// a site gives, and zuzu reads no CSS.
			for cl := range strings.FieldsSeq(rest) {
				if err := c.add("the class %s of the node %q, which needs the CSS of a page", cl, last.label); err != nil {
					return nil, err
				}
			}
			continue
		}
		if m := mindmapIcon.FindStringSubmatch(t); m != nil {
			if last == nil {
				return nil, c.errorf("an icon before the first node")
			}
			name, ok := mindmapIconName(m[1])
			if !ok {
				if err := c.add("the icon %q, which names no pack", m[1]); err != nil {
					return nil, err
				}
				continue
			}
			last.icon = name
			continue
		}
		// A string of markdown can run over more lines.
		if open := strings.Index(t, "\"`"); open >= 0 && !strings.Contains(t[open+2:], "`\"") {
			parts := []string{t}
			for i+1 < len(lines) {
				i++
				parts = append(parts, lines[i].Text)
				if strings.Contains(lines[i].Text, "`\"") {
					break
				}
			}
			t = strings.Join(parts, "\n")
		}
		// The text of a shape can run over more lines, up to the line that
		// ends the shape.
		soft := false
		if mindmapOpen(t) {
			parts := []string{t}
			for i+1 < len(lines) {
				i++
				parts = append(parts, lines[i].Text)
				if strings.ContainsAny(lines[i].Text, ")](}") {
					break
				}
			}
			t, soft = strings.Join(parts, "\n"), len(parts) > 1
		}
		n, err := c.mindmapNode(t)
		if err != nil {
			return nil, err
		}
		n.id, n.indent, n.line, n.soft = len(nodes), l.Indent, l.N, soft
		if len(nodes) > 0 {
			for _, p := range slices.Backward(nodes) {
				if p.indent < n.indent {
					n.parent = p
					break
				}
			}
			if n.parent == nil {
				return nil, c.errorf("a second root %q, and a mindmap has one root", n.label)
			}
			n.parent.kids = append(n.parent.kids, n)
			n.depth = n.parent.depth + 1
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		c.line = c.d.N
		return nil, c.errorf("a mindmap with no nodes")
	}
	return nodes, nil
}

// mindmapOpen reports whether a line opens the shape of a node and does not
// end it: the text after the first (, ), [ or { holds no ), ], ( or }, and
// is not a string.
func mindmapOpen(t string) bool {
	i := strings.IndexAny(t, "([{)")
	if i < 0 {
		return false
	}
	rest := strings.TrimLeft(t[i:], "([{)")
	return !strings.HasPrefix(rest, `"`) && !strings.ContainsAny(rest, ")](}")
}

// mindmapNode reads a node: an id and a shape around its text, as
// id((text)), or text alone, which draws in the default shape.
func (c *chart) mindmapNode(s string) (*mindmapNode, error) {
	n := &mindmapNode{shape: "default"}
	i := strings.IndexAny(s, "([{)")
	if i < 0 {
		if strings.ContainsAny(s, "}") {
			return nil, c.errorf("the node %q", s)
		}
		n.label = labelText(s)
		return n, nil
	}
	rest := s[i:]
	open := ""
	for _, o := range []string{"((", "))", "{{", "(", ")", "["} {
		if strings.HasPrefix(rest, o) {
			open = o
			break
		}
	}
	if open == "" {
		return nil, c.errorf("the shape of the node %q", s)
	}
	rest = rest[len(open):]
	var body string
	switch {
	case strings.HasPrefix(rest, "\"`"):
		j := strings.Index(rest[2:], "`\"")
		if j < 0 {
			return nil, c.errorf("a string of markdown with no end in %q", s)
		}
		body, rest = rest[2:2+j], rest[2+j+2:]
	case strings.HasPrefix(rest, `"`):
		j := strings.Index(rest[1:], `"`)
		if j < 0 {
			return nil, c.errorf("a string with no end in %q", s)
		}
		body, rest = rest[1:1+j], rest[1+j+1:]
	default:
		j := strings.IndexAny(rest, ")](}")
		if j < 0 {
			return nil, c.errorf("the node %q has no end", s)
		}
		body, rest = rest[:j], rest[j:]
	}
	end := strings.TrimSpace(rest)
	if !slices.Contains(mindmapEnds, end) {
		return nil, c.errorf("the end %q of the node %q", end, s)
	}
	switch open {
	case "[":
		n.shape = "rect"
	case "(":
		n.shape = "cloud"
		if end == ")" {
			n.shape = "rounded"
		}
	case "((":
		n.shape = "circle"
	case ")":
		n.shape = "cloud"
	case "))":
		n.shape = "bang"
	case "{{":
		n.shape = "hexagon"
	}
	if strings.Contains(body, "\n") {
		// Mermaid drops the space at the start and the end of each line of
		// a text that runs over more lines.
		var lines []string
		for l := range strings.SplitSeq(body, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
		body = strings.Join(lines, "\n")
	}
	n.label = labelText(body)
	return n, nil
}

// mindmapIconName reads the icon of a node, as the classes of a font of
// icons, fa fa-book or mdi mdi-home, into pack:name, fa:book or mdi:home.
// A name with a colon is a name of zuzu already.
func mindmapIconName(s string) (string, bool) {
	f := strings.Fields(s)
	if len(f) == 1 && strings.Contains(f[0], ":") {
		return f[0], true
	}
	if len(f) < 2 {
		return "", false
	}
	prefix := f[0] + "-"
	switch f[0] {
	case "fa", "fas", "far", "fab", "fa-solid", "fa-regular", "fa-brands":
		prefix = "fa-"
	}
	pack := strings.TrimPrefix(f[0], "fa-")
	switch pack {
	case "solid":
		pack = "fas"
	case "regular":
		pack = "far"
	case "brands":
		pack = "fab"
	}
	for _, w := range f[1:] {
		if name, ok := strings.CutPrefix(w, prefix); ok && name != "" {
			return pack + ":" + name, true
		}
	}
	return "", false
}

// mindmapColor gives a branch its color: the color of the branch, or one
// for each depth.
func mindmapColor(n *mindmapNode, color int, byDepth bool) {
	n.color = color
	if byDepth {
		n.color = (n.depth-1)%mindmapColors + 1
	}
	for _, k := range n.kids {
		mindmapColor(k, color, byDepth)
	}
}

// mindmapHTML reads htmlLabels, or flowchart.htmlLabels when it is not set,
// as Mermaid does, and reports whether a long word of a label breaks. With
// htmlLabels: false, Mermaid draws a label in SVG and breaks a word that is
// wider than the label. A value that is not true or false is an issue.
func (c *chart) mindmapHTML() (bool, error) {
	for _, keys := range [][]string{{"htmlLabels"}, {"flowchart", "htmlLabels"}} {
		var v any = c.d.Config
		for _, k := range keys {
			m, _ := v.(map[string]any)
			v = m[k]
		}
		if v == nil {
			continue
		}
		html, ok := v.(bool)
		if !ok {
			return false, c.add("the setting %s: %v", strings.Join(keys, "."), v)
		}
		return !html, nil
	}
	return false, nil
}

// mindmapMeasure measures the label and the icon of a node, and sets the
// size of its shape. When breakWords is true, a word that is wider than
// the label breaks.
func mindmapMeasure(n *mindmapNode, pad, maxWidth float64, breakWords bool, m text.Measurer) {
	n.size, n.weight = mindmapFont, 400
	if n.parent == nil {
		n.size, n.weight = mindmapRootFont, boldWeight
	}
	if n.icon != "" {
		n.isz = math.Round(n.size * 1.6)
	}
	limit := scaleWrap(maxWidth - 2*pad)
	if n.isz > 0 {
		limit -= n.isz + 6
	}
	label := n.label
	if n.soft {
		// Mermaid draws the text on one line, with no break, when it fits.
		one := strings.ReplaceAll(label, "\n", " ")
		if l := wrapRuns(markdownRuns(one), n.size, int(n.weight), 0, m); len(l) == 1 && l[0].w <= max(limit, 40) {
			label = one
		}
	}
	runs := markdownRuns(label)
	if breakWords {
		runs = breakRuns(runs, n.size, int(n.weight), max(limit, 40), m)
	}
	n.lines = wrapRuns(runs, n.size, int(n.weight), max(limit, 40), m)
	for _, l := range n.lines {
		n.tw = max(n.tw, l.w)
	}
	n.th = float64(len(n.lines)) * n.size * 1.3
	w, h := n.tw, n.th
	if n.isz > 0 {
		w += n.isz + 6
		h = max(h, n.isz)
	}
	padY := math.Round(pad * 0.6)
	switch n.shape {
	case "circle":
		d := math.Hypot(w, h) + pad
		n.w, n.h = d, d
	case "hexagon":
		n.h = h + 2*padY
		n.w = w + 2*pad + n.h
	case "cloud", "bang":
		n.w, n.h = (w+pad)*math.Sqrt2+12, (h+padY)*math.Sqrt2+12
	default:
		n.w, n.h = w+2*pad, h+2*padY
	}
}

// mindmapPlace lays a mindmap out. The root is at 0, 0. Its branches split
// in their order to its right side, from the top down, and then to its
// left side, from the bottom up, so that they read clockwise. Each side
// holds about half of the height. In a branch, each subtree takes a band
// of the height, and a node is in the middle of the band of its subtree,
// so that no two nodes overlap.
func mindmapPlace(root *mindmapNode) {
	mindmapBand(root)
	total := 0.0
	for _, k := range root.kids {
		total += k.band
	}
	// The first split branches go to the right: as many as make the two
	// sides closest in height, and the most of them on a tie.
	split, best, sum := 0, math.Inf(1), 0.0
	for i, k := range root.kids {
		sum += k.band
		if d := math.Abs(2*sum - total); d <= best+0.5 {
			split, best = i+1, min(best, d)
		}
	}
	right, left := root.kids[:split], slices.Clone(root.kids[split:])
	slices.Reverse(left)
	for _, side := range []struct {
		kids []*mindmapNode
		dir  int
	}{{right, 1}, {left, -1}} {
		h := 0.0
		for i, k := range side.kids {
			if i > 0 {
				h += mindmapBranchGap
			}
			h += k.band
		}
		top := -h / 2
		for _, k := range side.kids {
			mindmapPlaceAt(k, side.dir, top, root.x+float64(side.dir)*(root.w/2+mindmapRootGap))
			top += k.band + mindmapBranchGap
		}
	}
}

// mindmapBand sets the height of the band of each subtree: the height of
// the node, or the bands of its children, whichever is more.
func mindmapBand(n *mindmapNode) float64 {
	sum := 0.0
	for i, k := range n.kids {
		if i > 0 {
			sum += mindmapSiblingGap
		}
		sum += mindmapBand(k)
	}
	n.band = max(n.h, sum)
	return n.band
}

// mindmapPlaceAt places a subtree in its band, which starts at top, with
// the near edge of the node at x.
func mindmapPlaceAt(n *mindmapNode, dir int, top, x float64) {
	n.side = dir
	n.x = x + float64(dir)*n.w/2
	n.y = top + n.band/2
	h := 0.0
	for i, k := range n.kids {
		if i > 0 {
			h += mindmapSiblingGap
		}
		h += k.band
	}
	t := n.y - h/2
	for _, k := range n.kids {
		mindmapPlaceAt(k, dir, t, n.x+float64(dir)*(n.w/2+mindmapLevelGap))
		t += k.band + mindmapSiblingGap
	}
}

// anchor returns the point where an edge meets a node on a side: the
// middle of the side, or for the default shape, the end of the line under
// it.
func (n *mindmapNode) anchor(dir int) (float64, float64) {
	x := n.x + float64(dir)*n.w/2
	if n.shape == "default" && n.parent != nil {
		return x, n.y + n.h/2
	}
	return x, n.y
}

// mindmapSpec writes the Vega spec of a mindmap that mindmapPlace laid
// out.
func mindmapSpec(nodes []*mindmapNode) map[string]any {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, n := range nodes {
		minX, minY = min(minX, n.x-n.w/2), min(minY, n.y-n.h/2)
	}
	for _, n := range nodes {
		n.x, n.y = n.x-minX, n.y-minY
	}
	var rows, edges, runs, icons []any
	colors := []any{}
	for _, n := range nodes {
		if !slices.Contains(colors, any(n.color)) {
			colors = append(colors, n.color)
		}
		var label []string
		for _, l := range n.lines {
			label = append(label, l.String())
		}
		row := map[string]any{
			"id": n.id, "label": strings.Join(label, " "), "depth": n.depth, "shape": n.shape, "color": n.color,
			"x": n.x, "y": n.y, "width": n.w, "height": n.h, "side": n.side, "path": mindmapShape(n.shape, n.w, n.h),
			"strokeWidth": 1.25, "parent": nil,
		}
		if n.parent == nil {
			row["strokeWidth"] = 2.0
		} else {
			row["parent"] = n.parent.id
		}
		rows = append(rows, row)
		width := 1.25
		switch n.depth {
		case 1:
			width = 2.5
		case 2:
			width = 1.75
		}
		if n.parent != nil {
			x0, y0 := n.parent.anchor(n.side)
			x1, y1 := n.anchor(-n.side)
			mx := (x0 + x1) / 2
			edges = append(edges, map[string]any{
				"source": n.parent.id, "target": n.id, "color": n.color, "width": width,
				"path": fmt.Sprintf("M%s,%sC%s,%s %s,%s %s,%s", num(x0), num(y0), num(mx), num(y0), num(mx), num(y1), num(x1), num(y1)),
			})
		}
		if n.shape == "default" && n.parent != nil {
			y := n.y + n.h/2
			edges = append(edges, map[string]any{
				"source": n.id, "target": n.id, "color": n.color, "width": width,
				"path": fmt.Sprintf("M%s,%sL%s,%s", num(n.x-n.w/2), num(y), num(n.x+n.w/2), num(y)),
			})
		}
		// The label and the icon are a block in the middle of the shape,
		// the icon at the left, and each line in the middle of the text.
		bw := n.tw
		if n.isz > 0 {
			bw += n.isz + 6
		}
		left := n.x - bw/2
		if n.isz > 0 {
			icons = append(icons, map[string]any{"node": n.id, "icon": n.icon, "iconSize": n.isz, "x": left + n.isz/2, "y": n.y, "color": n.color})
			left += n.isz + 6
		}
		lh := n.size * 1.3
		y := n.y - n.th/2 + lh/2
		for i, l := range n.lines {
			x := left + (n.tw-l.w)/2
			for _, r := range l.runs {
				weight, style := n.weight, "normal"
				if r.bold {
					weight = max(weight, boldWeight)
				}
				if r.italic {
					style = "italic"
				}
				runs = append(runs, map[string]any{
					"node": n.id, "line": i, "x": x + r.x, "y": y, "text": r.text,
					"fontWeight": weight, "fontStyle": style, "fontSize": n.size,
				})
			}
			y += lh
		}
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	color := map[string]any{"scale": "color", "field": "color"}
	data := []any{
		map[string]any{"name": "nodes", "values": rows},
		filteredData("outlined", "nodes", "datum.shape !== 'default' || datum.depth === 0"),
		filteredData("plain", "nodes", "datum.shape === 'default' && datum.depth > 0"),
		map[string]any{"name": "edges", "values": edges},
		map[string]any{"name": "labels", "values": runs},
	}
	marks := []any{
		map[string]any{"type": "path", "name": "edges", "from": map[string]any{"data": "edges"}, "style": []any{graph.StyleEdge}, "encode": map[string]any{"update": map[string]any{
			"x": map[string]any{"value": 0}, "y": map[string]any{"value": 0}, "path": field("path"),
			"stroke": color, "strokeWidth": field("width"),
		}}},
		map[string]any{"type": "path", "name": "shapes", "from": map[string]any{"data": "outlined"}, "style": []any{graph.StyleNode}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"), "fill": color, "stroke": color, "strokeWidth": field("strokeWidth"),
		}}},
		map[string]any{"type": "path", "name": "plainShapes", "from": map[string]any{"data": "plain"}, "style": []any{graph.StyleNode}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"), "fill": color, "strokeOpacity": map[string]any{"value": 0},
		}}},
	}
	if len(icons) > 0 {
		data = append(data, iconData("icons", icons)...)
		marks = append(marks, iconMarks("icons", color)...)
	}
	marks = append(marks, map[string]any{"type": "text", "name": "labels", "from": map[string]any{"data": "labels"}, "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
		"fontWeight": field("fontWeight"), "fontStyle": field("fontStyle"),
		"align": map[string]any{"value": "left"}, "baseline": map[string]any{"value": "middle"},
	}}})
	return map[string]any{
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": colors, "range": "category"}},
		"marks":    marks,
	}
}

// iconData returns the datasets of the icons of a chart: the rows, each
// with an icon, a size and a center, through the transform zuzu.icon, and
// the rows of each kind of icon.
func iconData(name string, rows []any) []any {
	return []any{
		map[string]any{"name": name, "values": rows, "transform": []any{
			map[string]any{"type": "zuzu.icon"},
			map[string]any{"type": "formula", "as": "mediaLeft", "expr": "datum.x - datum.mediaWidth / 2"},
			map[string]any{"type": "formula", "as": "mediaTop", "expr": "datum.y - datum.mediaHeight / 2"},
		}},
		filteredData(name+"Fill", name, "datum.iconKind === 'fill'"),
		filteredData(name+"Stroke", name, "datum.iconKind === 'stroke'"),
		filteredData(name+"Image", name, "datum.iconKind === 'image'"),
	}
}

// iconMarks returns the marks of the icons of iconData: a path for an
// icon of one color, filled or stroked in the color, or with the style of
// an icon for nil, and an image for another icon.
func iconMarks(name string, color map[string]any) []any {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	fill := map[string]any{"x": field("x"), "y": field("y"), "path": field("iconPath")}
	stroke := map[string]any{"x": field("x"), "y": field("y"), "path": field("iconPath"), "strokeWidth": field("iconStrokeWidth")}
	if color != nil {
		fill["fill"] = color
		stroke["stroke"] = color
	}
	return []any{
		map[string]any{"type": "path", "from": map[string]any{"data": name + "Fill"}, "style": []any{graph.StyleIcon}, "encode": map[string]any{"update": fill}},
		map[string]any{"type": "path", "from": map[string]any{"data": name + "Stroke"}, "style": []any{graph.StyleIconStroke}, "encode": map[string]any{"update": stroke}},
		map[string]any{"type": "image", "from": map[string]any{"data": name + "Image"}, "encode": map[string]any{"update": map[string]any{
			"x": field("mediaLeft"), "y": field("mediaTop"), "width": field("mediaWidth"), "height": field("mediaHeight"),
			"url": field("iconURL"), "aspect": field("mediaAspect"),
		}}},
	}
}

// mindmapShape returns the path of a shape of a mindmap of a size,
// centered at 0, 0.
func mindmapShape(shape string, w, h float64) string {
	switch shape {
	case "circle":
		r := w / 2
		return fmt.Sprintf("M%s,0A%s,%s 0 1,1 %s,0A%s,%s 0 1,1 %s,0Z", num(-r), num(r), num(r), num(r), num(r), num(r), num(-r))
	case "hexagon":
		a := h / 2
		return polygon([][2]float64{{-w / 2, 0}, {-w/2 + a, -h / 2}, {w/2 - a, -h / 2}, {w / 2, 0}, {w/2 - a, h / 2}, {-w/2 + a, h / 2}})
	case "cloud":
		return cloudPath(w, h)
	case "bang":
		return bangPath(w, h)
	case "rect":
		return roundedRect(w, h, 2)
	case "rounded":
		return roundedRect(w, h, min(h/2, 12))
	}
	return roundedRect(w, h, 4)
}

// roundedRect returns the path of a rectangle with round corners,
// centered at 0, 0.
func roundedRect(w, h, r float64) string {
	x, y := -w/2, -h/2
	return fmt.Sprintf("M%s,%sH%sA%s,%s 0 0,1 %s,%sV%sA%s,%s 0 0,1 %s,%sH%sA%s,%s 0 0,1 %s,%sV%sA%s,%s 0 0,1 %s,%sZ",
		num(x+r), num(y), num(x+w-r), num(r), num(r), num(x+w), num(y+r), num(y+h-r), num(r), num(r), num(x+w-r), num(y+h),
		num(x+r), num(r), num(r), num(x), num(y+h-r), num(y+r), num(r), num(r), num(x+r), num(y))
}

// polygon returns the path of a polygon.
func polygon(pts [][2]float64) string {
	var b strings.Builder
	for i, p := range pts {
		if i == 0 {
			b.WriteString("M")
		} else {
			b.WriteString("L")
		}
		b.WriteString(num(p[0]) + "," + num(p[1]))
	}
	b.WriteString("Z")
	return b.String()
}

// cloudPath returns the path of a cloud of a size: arcs that bulge out
// from points on an ellipse inside the size.
func cloudPath(w, h float64) string {
	const bump = 6.0
	rx, ry := w/2-bump, h/2-bump
	n := max(8, int(math.Round(math.Pi*(rx+ry)/18)))
	var b strings.Builder
	var px, py float64
	for i := 0; i <= n; i++ {
		a := 2*math.Pi*float64(i)/float64(n) - math.Pi
		x, y := rx*math.Cos(a), ry*math.Sin(a)
		if i == 0 {
			b.WriteString("M" + num(x) + "," + num(y))
		} else {
			r := math.Hypot(x-px, y-py) * 0.6
			fmt.Fprintf(&b, "A%s,%s 0 0,1 %s,%s", num(r), num(r), num(x), num(y))
		}
		px, py = x, y
	}
	b.WriteString("Z")
	return b.String()
}

// bangPath returns the path of a bang of a size: a star of spikes between
// an ellipse of the size and one inside it.
func bangPath(w, h float64) string {
	rx, ry := w/2, h/2
	n := max(10, int(math.Round(math.Pi*(rx+ry)/16)))
	pts := make([][2]float64, 0, 2*n)
	for i := range 2 * n {
		a := math.Pi*float64(i)/float64(n) - math.Pi
		k := 1.0
		if i%2 == 1 {
			k = 0.82
		}
		pts = append(pts, [2]float64{k * rx * math.Cos(a), k * ry * math.Sin(a)})
	}
	return polygon(pts)
}

// num writes a number of a path with two decimals at most.
func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}
