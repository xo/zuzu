package mermaid

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// The sizes of a tree view in the pixels of Mermaid 12.3, which scaleWrap
// turns into the pixels of zuzu at 11/16, because Mermaid draws the labels
// of a tree view at 16 pixels (D32): the size of an icon, the space
// between an icon and its label, the space between the widest label and
// the descriptions, the space at the right of a band of highlight and
// past its end, and the space above and under a band.
const (
	tvIconSize  = 14.0
	tvIconGap   = 4.0
	tvDescGap   = 16.0
	tvHighRight = 8.0
	tvHighPast  = 2.0
	tvHighInset = 1.0
)

// tvLayout lays out a tree view and writes its rows.
type tvLayout struct {
	t *treeView
	m *text.Estimate
	// font is the font of a label, and size its size in the pixels of
	// zuzu.
	font text.Font
	// rowH is the height of a row, and the other sizes are those of the
	// settings in the pixels of zuzu.
	rowH, indent, padX, step float64

	labels, descs, lines, highs, glyphs, icons []any
	// rows are the rows of the items in the order of the source, with the
	// right end of the label of each.
	rows []tvRow
	// rowY is the top of the row of each item.
	rowY          map[*tvNode]float64
	width, height float64
}

// tvRow is a row of a tree view as the layout places it.
type tvRow struct {
	n          *tvNode
	id         string
	x, y       float64
	labelRight float64
}

// newTreeViewLayout lays out a tree view as Mermaid 12.3 does: each item
// on a row of its own, in the order of the source, indented by its depth.
// A line joins each item to the line of its parent, which runs down from
// the parent to its last child.
func newTreeViewLayout(t *treeView) *tvLayout {
	k := func(v float64) float64 { return scaleWrap(treeViewKind, "", v) }
	size := wrapSize * t.labelSize / 16
	l := &tvLayout{
		t: t, m: text.NewEstimate(),
		font:   text.Font{Family: text.Family, Size: size, Weight: 400},
		indent: k(t.rowIndent), padX: k(t.padX), rowY: map[*tvNode]float64{},
	}
	l.rowH = lh(l.font) + 2*k(t.padY)
	l.step = k(t.rowIndent + t.padX)
	l.walk(t.root, 0)
	l.annotate()
	return l
}

// treeViewKind is the kind of a tree view, by which mermaidSizes gives the
// size of its labels.
const treeViewKind = "treeView-beta"

// walk places an item and its children, and draws the lines that join
// them.
func (l *tvLayout) walk(n *tvNode, depth int) {
	k := func(v float64) float64 { return scaleWrap(treeViewKind, "", v) }
	id := "n" + strconv.Itoa(len(l.rows))
	x, y := float64(depth)*l.step, l.height
	cy := y + l.rowH/2
	f := l.font
	if n.dir {
		f.Weight = boldWeight
	}
	labelX := x + l.padX
	if ic := l.t.iconOf(n); ic != "" {
		size := k(tvIconSize)
		l.icon(id, ic, labelX+size/2, cy, size)
		labelX += size + k(tvIconGap)
	}
	w := l.m.Width(f, n.name)
	l.labels = append(l.labels, map[string]any{
		"id": id, "depth": depth, "x": round2(labelX), "y": round2(cy), "text": n.name, "fontWeight": f.Weight,
	})
	l.rows = append(l.rows, tvRow{n: n, id: id, x: x, y: y, labelRight: labelX + w})
	l.rowY[n] = y
	// The root has no line to a parent.
	if depth > 0 {
		l.line(x-l.indent, cy, x, cy)
	}
	l.width = math.Max(l.width, labelX+w+l.padX)
	l.height += l.rowH
	if len(n.children) == 0 {
		return
	}
	for _, c := range n.children {
		l.walk(c, depth+1)
	}
	// The line of the children runs from the bottom of the row of the
	// item to the middle of the row of its last child, where the line to
	// that child starts, so that the two meet.
	l.line(x+l.padX, y+l.rowH, x+l.padX, l.rowY[n.children[len(n.children)-1]]+l.rowH/2)
}

// annotate places the descriptions in a column after the widest label,
// then the bands of the items with the class highlight, each as wide as
// the tree at its turn, as in Mermaid.
func (l *tvLayout) annotate() {
	k := func(v float64) float64 { return scaleWrap(treeViewKind, "", v) }
	right := 0.0
	described := false
	for _, r := range l.rows {
		right = math.Max(right, r.labelRight)
		described = described || r.n.desc != ""
	}
	if described {
		x := right + k(tvDescGap)
		f := l.font
		f.Italic = true
		for _, r := range l.rows {
			if r.n.desc == "" {
				continue
			}
			l.descs = append(l.descs, map[string]any{
				"id": r.id, "x": round2(x), "y": round2(r.y + l.rowH/2), "text": r.n.desc,
			})
			l.width = math.Max(l.width, x+l.m.Width(f, r.n.desc)+l.padX)
		}
	}
	for _, r := range l.rows {
		if !hasClass(r.n.class, "highlight") {
			continue
		}
		w := l.width - r.x + k(tvHighRight)
		l.highs = append(l.highs, map[string]any{
			"id": r.id, "x": round2(r.x), "y": round2(r.y + k(tvHighInset)),
			"x2": round2(r.x + w), "y2": round2(r.y + l.rowH - k(tvHighInset)),
		})
		l.width = math.Max(l.width, r.x+w+k(tvHighPast))
	}
}

// hasClass reports whether a list of classes, split by spaces, holds a
// class.
func hasClass(list, class string) bool {
	return slices.Contains(strings.Fields(list), class)
}

// icon writes the icon of an item, centered at a point: the folder or the
// file of Mermaid as a shape of zuzu, or else an icon of a pack.
func (l *tvLayout) icon(id, name string, cx, cy, size float64) {
	switch name {
	case tvIconPack + ":folder":
		l.glyphs = append(l.glyphs, map[string]any{"id": id, "x": round2(cx), "y": round2(cy), "path": tvFolder(size)})
	case tvIconPack + ":file":
		l.glyphs = append(l.glyphs, map[string]any{"id": id, "x": round2(cx), "y": round2(cy), "path": tvFile(size)})
	default:
		l.icons = append(l.icons, map[string]any{"id": id, "icon": name, "iconSize": round2(size), "x": round2(cx), "y": round2(cy)})
	}
}

// tvFolder returns the path of a folder of a size, centered at 0, 0: a
// body with a tab at its top left.
func tvFolder(size float64) string {
	u := size / 24
	p := func(x, y float64) string { return num(x*u-size/2) + "," + num(y*u-size/2) }
	return "M" + p(2, 5) + "L" + p(9, 5) + "L" + p(11, 7) + "L" + p(22, 7) + "L" + p(22, 20) + "L" + p(2, 20) + "Z"
}

// tvFile returns the path of a file of a size, centered at 0, 0: a page
// with its top right corner folded. The fold winds the other way, so that
// it is a hole in the fill.
func tvFile(size float64) string {
	u := size / 24
	p := func(x, y float64) string { return num(x*u-size/2) + "," + num(y*u-size/2) }
	return "M" + p(5, 2) + "L" + p(14, 2) + "L" + p(20, 8) + "L" + p(20, 22) + "L" + p(5, 22) + "Z" +
		"M" + p(14, 2) + "L" + p(14, 8) + "L" + p(20, 8)
}

// line writes a line of the tree.
func (l *tvLayout) line(x0, y0, x1, y1 float64) {
	l.lines = append(l.lines, map[string]any{"x": round2(x0), "y": round2(y0), "x2": round2(x1), "y2": round2(y1)})
}

// round2 rounds a value to two places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// spec writes the rows of the layout as a Vega spec. The lines take the
// style of an edge, a label the style of a label of a node, a description
// the muted style of the label of an edge, in italic, and a band the
// style of a node. A color of the theme variables of treeView replaces
// the color of the theme.
func (l *tvLayout) spec() map[string]any {
	t := l.t
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	data := []any{
		map[string]any{"name": "lines", "values": orEmpty(l.lines)},
		map[string]any{"name": "highlights", "values": orEmpty(l.highs)},
		map[string]any{"name": "glyphs", "values": orEmpty(l.glyphs)},
		map[string]any{"name": "labels", "values": orEmpty(l.labels)},
		map[string]any{"name": "descriptions", "values": orEmpty(l.descs)},
	}
	from := func(name string) map[string]any { return map[string]any{"data": name} }
	lineEnc := map[string]any{"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2")}
	if t.thickness != 1 {
		lineEnc["strokeWidth"] = value(round2(theme.EdgeWidth * t.thickness))
	}
	if t.lineColor != "" {
		lineEnc["stroke"] = value(t.lineColor)
	}
	highEnc := map[string]any{"x": field("x"), "y": field("y"), "x2": field("x2"), "y2": field("y2"), "cornerRadius": value(3)}
	if t.highBg != "" {
		highEnc["fill"], highEnc["fillOpacity"] = value(t.highBg), value(1)
	}
	if t.highStroke != "" {
		highEnc["stroke"] = value(t.highStroke)
	}
	glyphEnc := map[string]any{"x": field("x"), "y": field("y"), "path": field("path")}
	var iconColor map[string]any
	if t.iconColor != "" {
		iconColor = value(t.iconColor)
		glyphEnc["fill"] = iconColor
	}
	labelEnc := map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "fontWeight": field("fontWeight"),
		"fontSize": value(l.font.Size), "align": value("left"), "baseline": value("middle"),
	}
	if t.labelColor != "" {
		labelEnc["fill"] = value(t.labelColor)
	}
	descEnc := map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"), "fontStyle": value("italic"),
		"fontSize": value(l.font.Size), "align": value("left"), "baseline": value("middle"),
	}
	if t.descColor != "" {
		descEnc["fill"] = value(t.descColor)
	}
	mark := func(typ, name string, style string, enc map[string]any) map[string]any {
		return map[string]any{
			"type": typ, "name": name, "from": from(name), "style": []any{style},
			"encode": map[string]any{"update": enc},
		}
	}
	marks := []any{
		mark("rect", "highlights", graph.StyleNode, highEnc),
		mark("rule", "lines", graph.StyleEdge, lineEnc),
		mark("path", "glyphs", graph.StyleIcon, glyphEnc),
	}
	if len(l.icons) > 0 {
		data = append(data, iconData("icons", l.icons)...)
		marks = append(marks, iconMarks("icons", iconColor)...)
	}
	marks = append(marks,
		mark("text", "labels", graph.StyleNodeLabel, labelEnc),
		mark("text", "descriptions", graph.StyleEdgeLabel, descEnc),
	)
	const pad = 8.0
	return map[string]any{
		"width":    math.Ceil(l.width),
		"height":   math.Ceil(l.height),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"marks":    marks,
	}
}
