package mermaid

import (
	"math"
	"strconv"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// The sizes of a kanban in the pixels of Mermaid 12.3, which scaleWrap
// turns into the pixels of zuzu at 11/16, because Mermaid draws the labels
// of a kanban at 16 pixels (D32). A column is kanban.sectionWidth wide, and
// its name wraps at that width. A card is 15 pixels narrower than its
// column, and its text wraps at 10 pixels less than the card.
const (
	kbCardLess = 15.0
	kbWrapLess = 25.0
)

// The spaces of the look of zuzu in a kanban, in pixels: between two
// columns, around the name of a column, around the text of a card, between
// two cards, and under the last card of a column.
const (
	kbColumnGap = 6.0
	kbHeadPad   = 6.0
	kbCardPadX  = 7.0
	kbCardPadY  = 6.0
	kbCardGap   = 4.0
	kbBottom    = 7.0
)

// The fonts of a kanban: the name of a column is bold, the text of a card
// is not, and the ticket and the assignee of a card are smaller.
var (
	kbHeadFont = text.Font{Family: text.Family, Size: 11, Weight: boldWeight}
	kbCardFont = text.Font{Family: text.Family, Size: 11, Weight: 400}
	kbMetaFont = text.Font{Family: text.Family, Size: 10, Weight: 400}
)

// kbLayout lays out a kanban and writes its rows.
type kbLayout struct {
	k    *kanban
	m    *text.Estimate
	rows map[string][]any
	w, h float64
	err  error
}

// newKanbanLayout lays out a kanban: the columns side by side, each with
// its name at its top and its cards under it. The cards of every column
// start under the tallest name, as in Mermaid, and each column is as tall
// as its cards.
func newKanbanLayout(k *kanban) *kbLayout {
	l := &kbLayout{k: k, m: text.NewEstimate(), rows: map[string][]any{}}
	colW := scaleWrap("kanban", "", k.width)
	cardW := scaleWrap("kanban", "", k.width-kbCardLess)
	inset := (colW - cardW) / 2
	heads := make([][]string, len(k.columns))
	head := 0.0
	for i, col := range k.columns {
		heads[i], _ = plainLines(col.label, kbHeadFont.Size, kbHeadFont.Weight, colW, l.m)
		head = max(head, float64(len(heads[i]))*lh(kbHeadFont))
	}
	head += 2 * kbHeadPad
	for i, col := range k.columns {
		x := float64(i) * (colW + kbColumnGap)
		id := "c" + strconv.Itoa(i)
		key := strconv.Itoa(i)
		y := head
		for j, card := range col.cards {
			y += l.card(card, id+"i"+strconv.Itoa(j), key, x+inset, y, cardW) + kbCardGap
		}
		bottom := y - kbCardGap + kbBottom
		if len(col.cards) == 0 {
			bottom = head + scaleWrap("kanban", "", 25)
		}
		l.rows["columns"] = append(l.rows["columns"], map[string]any{
			"id": id, "key": key, "x": x, "y": 0.0, "x2": x + colW, "y2": bottom,
		})
		l.rows["columnLabels"] = append(l.rows["columnLabels"], map[string]any{
			"id": id, "key": key, "x": x + colW/2, "y": head / 2, "lines": anyStrings(heads[i]),
		})
		l.w, l.h = x+colW, max(l.h, bottom)
	}
	return l
}

// card writes a card with its text, its ticket, its assignee and its
// priority at x and y, and returns its height. The text wraps at the width
// of Mermaid, scaled. The ticket is at the bottom left, under the text, and
// the assignee at the bottom right.
func (l *kbLayout) card(n *kbNode, id, key string, x, y, w float64) float64 {
	limit := scaleWrap("kanban", "", l.k.width-kbWrapLess)
	lines := wrapRuns(markdownRuns(n.label), kbCardFont.Size, kbCardFont.Weight, limit, l.m)
	th := float64(len(lines)) * lh(kbCardFont)
	h := th + 2*kbCardPadY
	meta := n.ticket != "" || n.assigned != ""
	if meta {
		h += lh(kbMetaFont)
	}
	l.rows["cards"] = append(l.rows["cards"], map[string]any{
		"id": id, "key": key, "label": n.label, "x": x, "y": y, "x2": x + w, "y2": y + h,
	})
	ly := y + kbCardPadY + lh(kbCardFont)/2
	for i, line := range lines {
		for _, r := range line.runs {
			weight, style := kbCardFont.Weight, "normal"
			if r.bold {
				weight = boldWeight
			}
			if r.italic {
				style = "italic"
			}
			l.rows["cardLabels"] = append(l.rows["cardLabels"], map[string]any{
				"id": id, "line": i, "x": x + kbCardPadX + r.x, "y": ly, "text": r.text,
				"fontWeight": weight, "fontStyle": style,
			})
		}
		ly += lh(kbCardFont)
	}
	my := y + kbCardPadY + th + lh(kbMetaFont)/2
	if n.ticket != "" {
		row := map[string]any{"id": id, "x": x + kbCardPadX, "y": my, "text": n.ticket}
		href, err := l.k.link(n)
		if err != nil && l.err == nil {
			l.err = err
		}
		if href != "" {
			row["href"] = href
		}
		l.rows["tickets"] = append(l.rows["tickets"], row)
	}
	if n.assigned != "" {
		l.rows["assignees"] = append(l.rows["assignees"], map[string]any{
			"id": id, "x": x + w - kbCardPadX, "y": my, "text": n.assigned,
		})
	}
	if color, ok := kbPriorities[n.priority]; ok {
		l.rows["priorities"] = append(l.rows["priorities"], map[string]any{
			"id": id, "priority": n.priority, "color": color, "x": x + 2.5, "y": y + 3, "y2": y + h - 3,
		})
	}
	return h
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31).
// Each column takes the next color of the scheme, and its cards take its
// color.
func (l *kbLayout) spec() map[string]any {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	color := map[string]any{"scale": "color", "field": "key"}
	var data, marks []any
	mark := func(typ, set string, style string, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": set, "from": map[string]any{"data": set},
			"style": []any{style}, "encode": map[string]any{"update": enc},
		})
	}
	rect := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["x2"], enc["y2"] = field("x"), field("y"), field("x2"), field("y2")
		return enc
	}
	for _, set := range []string{"columns", "columnLabels", "cards", "priorities", "cardLabels", "tickets", "assignees"} {
		if rows := l.rows[set]; len(rows) > 0 {
			for _, r := range rows {
				m, _ := r.(map[string]any)
				for _, k := range []string{"x", "y", "x2", "y2"} {
					if v, ok := m[k].(float64); ok {
						m[k] = math.Round(v*100) / 100
					}
				}
			}
			data = append(data, map[string]any{"name": set, "values": rows})
		}
	}
	mark("rect", "columns", graph.StyleNode, rect(map[string]any{
		"fill": color, "stroke": color, "fillOpacity": value(0.12), "strokeOpacity": value(0.5), "cornerRadius": value(6),
	}))
	mark("text", "columnLabels", graph.StyleNodeLabel, map[string]any{
		"x": field("x"), "y": field("y"), "text": field("lines"), "fill": color,
		"align": value("center"), "baseline": value("middle"),
		"fontSize": value(kbHeadFont.Size), "fontWeight": value(kbHeadFont.Weight), "lineHeight": value(lh(kbHeadFont)),
	})
	mark("rect", "cards", graph.StyleEdgeLabelBox, rect(map[string]any{
		"fillOpacity": value(1), "stroke": color, "strokeWidth": value(1.25), "cornerRadius": value(4),
	}))
	mark("rule", "priorities", graph.StyleEdge, map[string]any{
		"x": field("x"), "y": field("y"), "x2": field("x"), "y2": field("y2"),
		"stroke": field("color"), "strokeWidth": value(3), "strokeCap": value("round"),
	})
	mark("text", "cardLabels", graph.StyleNodeLabel, map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"),
		"fontSize": value(kbCardFont.Size), "fontWeight": field("fontWeight"), "fontStyle": field("fontStyle"),
		"align": value("left"), "baseline": value("middle"),
	})
	ticket := map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"),
		"fontSize": value(kbMetaFont.Size), "align": value("left"), "baseline": value("middle"),
	}
	for _, r := range l.rows["tickets"] {
		if _, ok := r.(map[string]any)["href"]; ok {
			// A ticket with a link carries it, as Mermaid draws the
			// ticket as a link (D32).
			ticket["href"] = field("href")
			break
		}
	}
	mark("text", "tickets", graph.StyleEdgeLabel, ticket)
	mark("text", "assignees", graph.StyleEdgeLabel, map[string]any{
		"x": field("x"), "y": field("y"), "text": field("text"),
		"fontSize": value(kbMetaFont.Size), "align": value("right"), "baseline": value("middle"),
	})
	domain := make([]any, len(l.k.columns))
	for i := range domain {
		domain[i] = strconv.Itoa(i)
	}
	return map[string]any{
		"width":    math.Ceil(l.w),
		"height":   math.Ceil(l.h),
		"autosize": "pad",
		"padding":  8,
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": domain, "range": "category"}},
		"marks":    marks,
	}
}
