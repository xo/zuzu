package mermaid

import (
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// measureMath lays out the label of a block with math of KaTeX in a weight,
// and reports whether it did. A line with math lays out in one row, and
// each other line as text (D36). A label with no line that lays out as math
// lays out as text.
func (b *blockDiagram) measureMath(n *blockNode, weight int) bool {
	if text.MathOf(b.m) == nil || !text.HasMath(n.label) {
		return false
	}
	n.font = text.Font{Family: text.Family, Size: n.size, Weight: weight}
	lines, maths, w, h := b.layMath(n.label, n.font)
	if maths == nil {
		return false
	}
	n.lines, n.maths, n.tw, n.th = lines, maths, w, h
	return true
}

// layMath lays out the lines of a label with math of KaTeX in a font. It
// returns the lines, the layout of each line with math, or nil for a line
// of text, and the width and the height of the label. The layouts are nil
// when no line lays out as math.
func (b *blockDiagram) layMath(label string, f text.Font) ([]string, []*text.MathLine, float64, float64) {
	lh := f.Size * 1.3
	var lines []string
	var maths []*text.MathLine
	w, h, found := 0.0, 0.0, false
	for s := range strings.SplitSeq(label, "\n") {
		lines = append(lines, s)
		if l := b.math.line(f, s, lh); l != nil {
			maths = append(maths, l)
			w, h, found = max(w, l.Width), h+l.Height, true
			continue
		}
		maths = append(maths, nil)
		w, h = max(w, b.m.Width(f, s)), h+lh
	}
	if !found {
		return nil, nil, 0, 0
	}
	return lines, maths, w, h
}

// edgeMath lays out the label of an edge with math of KaTeX, as layMath
// does, in the font of the labels of the edges.
func (b *blockDiagram) edgeMath(label string) ([]string, []*text.MathLine, float64, float64) {
	if text.MathOf(b.m) == nil || !text.HasMath(label) {
		return nil, nil, 0, 0
	}
	return b.layMath(label, text.Font{Family: text.Family, Size: b.font, Weight: 400})
}

// mathLabel returns the rows of the label with math of a block, with its
// middle at y, from x, or centered on x when center is true.
func (b *blockDiagram) mathLabel(n *blockNode, x, y float64, center bool) ([]any, []any) {
	texts, paths := b.layLabel(n.lines, n.maths, n.font, x, y, center, n.row)
	return keyed(texts, "id", n.id), keyed(paths, "id", n.id)
}

// keyed sets a key of each row to a value, and returns the rows as values
// of a dataset.
func keyed(rows []map[string]any, key string, v any) []any {
	out := make([]any, len(rows))
	for i, r := range rows {
		r[key] = v
		out[i] = r
	}
	return out
}

// layLabel returns the rows of the lines of a label with math in a font,
// with the middle of the label at y. Each line starts at x, or is centered
// on x when center is true. A row of text is a line of text or a run of
// text of a line with math, which draws with the align left and the
// baseline middle, with the keys of the paint of the text of a row of a
// block. A row of a formula takes the color of the text of the block as
// its fill, unless it has a color of its own.
func (b *blockDiagram) layLabel(lines []string, maths []*text.MathLine, f text.Font, x, y float64, center bool, row map[string]any) ([]map[string]any, []map[string]any) {
	lh := f.Size * 1.3
	h := 0.0
	for _, ml := range maths {
		if ml != nil {
			h += ml.Height
		} else {
			h += lh
		}
	}
	top := y - h/2
	var texts, paths []map[string]any
	addText := func(s string, tx, ty float64) {
		t := map[string]any{"text": s, "x": tx, "y": ty, "fontSize": f.Size}
		copyKeys(t, row, "textColor", "fontWeight", "fontStyle")
		texts = append(texts, t)
	}
	for i, s := range lines {
		ml := maths[i]
		if ml == nil {
			x0 := x
			if center {
				x0 -= b.m.Width(f, s) / 2
			}
			addText(s, x0, top+lh/2)
			top += lh
			continue
		}
		mid := top + ml.Height/2
		x0 := x
		if center {
			x0 -= ml.Width / 2
		}
		runs, rows := mathPieces(b.m, f, ml, x0, mid)
		for _, r := range runs {
			addText(r.s, r.x, mid)
		}
		for _, p := range rows {
			if c, ok := row["textColor"]; ok {
				if _, own := p["fill"]; !own {
					p["fill"] = c
				}
			}
		}
		paths = append(paths, rows...)
		top += ml.Height
	}
	return texts, paths
}

// labelMathMarks returns the marks of the labels with math of KaTeX of a
// block diagram, and adds their datasets to data: the runs of text and the
// formulas of the labels of the blocks, the formulas of the titles of the
// composite blocks, whose runs of text are titles, and the runs of text
// and the formulas of the labels of the edges. A diagram with no math has
// none of them.
func (b *blockDiagram) labelMathMarks(data *[]any, labelRuns, labelMath, titleMath, edgeRuns, edgeMath []any) []any {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	var marks []any
	if len(labelRuns) > 0 {
		*data = append(*data, map[string]any{"name": "labelRuns", "values": labelRuns})
		marks = append(marks, sigMarks(data, "labelRuns", labelRuns, []string{"textColor", "fontWeight", "fontStyle"}, map[string]string{"textColor": "fill"}, func() map[string]any {
			return map[string]any{"type": "text", "style": []any{graph.StyleNodeLabel}, "encode": map[string]any{"update": map[string]any{
				"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
				"align": value("left"), "baseline": value("middle"),
			}}}
		})...)
	}
	if len(edgeRuns) > 0 {
		*data = append(*data, map[string]any{"name": "edgeLabelRuns", "values": edgeRuns})
		marks = append(marks, map[string]any{"type": "text", "name": "edgeLabelRuns", "from": map[string]any{"data": "edgeLabelRuns"}, "style": []any{graph.StyleEdgeLabel}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "fontSize": field("fontSize"),
			"align": value("left"), "baseline": value("middle"),
		}}})
	}
	for _, set := range []struct {
		name, style string
		rows        []any
	}{
		{"labelMath", graph.StyleNodeLabel, labelMath},
		{"titleMath", graph.StyleClusterTitle, titleMath},
		{"edgeLabelMath", graph.StyleEdgeLabel, edgeMath},
	} {
		if len(set.rows) > 0 {
			*data = append(*data, map[string]any{"name": set.name, "values": set.rows})
			marks = append(marks, mathMarks(data, set.name, set.rows, set.style)...)
		}
	}
	return marks
}
