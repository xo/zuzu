package mermaid

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// The sizes of an event model in the pixels of Mermaid 12.3, which emPx
// turns into the pixels of zuzu at 11/16, because Mermaid draws the labels
// of an event model at 16 pixels (D32). A box is 80 by 80 at least, and 450
// by 750 at most, inside a padding of 10. The text of a box wraps at 430.
// A swimlane is 70 high at least, with a padding of 15 above and under its
// boxes, and 10 from the next. The first box starts 250 from the left.
// Two boxes in one swimlane are 10 apart, and a box in another swimlane
// than the box before it starts 70 before the end of that box.
const (
	emLaneMinH   = 70.0
	emLanePad    = 15.0
	emLaneGap    = 10.0
	emBoxPad     = 10.0
	emBoxOverlap = 90.0
	emBoxMinW    = 80.0
	emBoxMaxW    = 450.0
	emBoxMinH    = 80.0
	emBoxMaxH    = 750.0
	emStartX     = 250.0
	emTextMaxW   = 430.0
	emTextPad    = 10.0
	// emLabelX is the left of the label of a swimlane, and emLabelY the
	// middle of its first line under the top of the swimlane.
	emLabelX = 30.0
	emLabelY = 24.0
)

// The swimlanes of the kinds of a frame: the first index of the lanes of
// the kind, the label of the lane with no namespace, and the label before
// a namespace.
var emLanes = map[string]struct {
	index         int
	label, prefix string
}{
	"ui":        {0, "UI/Automation", "UI/A: "},
	"processor": {0, "UI/Automation", "UI/A: "},
	"readmodel": {100, "Command/Read Model", "C/RM: "},
	"command":   {100, "Command/Read Model", "C/RM: "},
	"event":     {200, "Events", "Stream: "},
}

// emColors are the kinds of a frame that take a color of the category
// scheme, in its order, so that a command is blue, a read model green, an
// event orange and a processor pink, near the colors of Mermaid. A frame
// ui draws in the neutral color muted, as Mermaid draws it white.
var emColors = []any{"command", "readmodel", "event", "processor"}

// The fonts of an event model: the name of a box is bold, and its data is
// not.
var (
	emNameFont = text.Font{Family: text.Family, Size: 11, Weight: boldWeight}
	emDataFont = text.Font{Family: text.Family, Size: 11, Weight: 400}
)

// emSpaces matches a run of spaces in a line of data.
var emSpaces = regexp.MustCompile(`[ \t]+`)

// emPx returns a size of Mermaid in the pixels of zuzu.
func emPx(v float64) float64 {
	return scaleWrap("eventmodeling", "", v)
}

// emLane is a swimlane of an event model.
type emLane struct {
	index int
	label string
	// r is the right of the last box of the lane.
	r       float64
	y, h    float64
	maxBoxH float64
}

// emBox is the box of a frame.
type emBox struct {
	f     *emFrame
	lane  *emLane
	x     float64
	w, h  float64
	names []string
	data  []string
}

// emRel is an arrow from one box to another.
type emRel struct {
	from, to *emBox
}

// emLayout lays out an event model.
type emLayout struct {
	m     *eventModel
	meas  *text.Estimate
	lanes map[int]*emLane
	boxes []*emBox
	rels  []emRel
	// right is the right of the widest box and its padding.
	right float64
}

// newEMLayout lays out an event model with the rules of Mermaid 12.3: each
// frame in order, in the swimlane of its kind, and an arrow to each frame
// from the frames that it names, or from the last frame of another
// swimlane.
func newEMLayout(m *eventModel) *emLayout {
	l := &emLayout{m: m, meas: text.NewEstimate(), lanes: map[int]*emLane{}}
	// The first box starts at 250 pixels of Mermaid, or right of the
	// widest label of a swimlane when it is wider.
	start := emPx(emStartX)
	for _, f := range m.frames {
		b := l.box(f)
		b.lane = l.lane(f)
		start = max(start, l.meas.Width(emNameFont, b.lane.label)+emPx(emLabelX+20))
		l.boxes = append(l.boxes, b)
	}
	var prev *emBox
	for _, b := range l.boxes {
		switch {
		case prev == nil:
			b.x = start
		case prev.lane == b.lane && b.lane.r != 0:
			b.x = b.lane.r + emPx(emBoxPad)
		default:
			b.x = prev.x + prev.w + emPx(emBoxPad) - emPx(emBoxOverlap) + emPx(emBoxPad)
			// A box starts after the last box of its lane, which Mermaid
			// can draw over.
			if b.lane.r != 0 {
				b.x = max(b.x, b.lane.r+emPx(emBoxPad))
			}
		}
		b.lane.r = b.x + b.w
		b.lane.maxBoxH = max(b.lane.maxBoxH, b.h)
		b.lane.h = max(emPx(emLaneMinH), b.lane.maxBoxH) + 2*emPx(emLanePad)
		l.right = max(l.right, b.x+b.w+emPx(emBoxPad))
		prev = b
	}
	y := 0.0
	for _, ln := range l.sortedLanes() {
		ln.y = y
		y += ln.h + emPx(emLaneGap)
	}
	l.relations()
	return l
}

// sortedLanes returns the swimlanes by their index.
func (l *emLayout) sortedLanes() []*emLane {
	var out []*emLane
	for _, ln := range l.lanes {
		out = append(out, ln)
	}
	slices.SortFunc(out, func(a, b *emLane) int { return a.index - b.index })
	return out
}

// lane returns the swimlane of a frame, and adds it when it is new. A
// frame with no namespace goes in the lane of its kind. A frame with a
// namespace, as Cart.ItemAdded, goes in a new lane under the lanes of its
// kind, labeled with the namespace, as in Mermaid 12.3, which does not
// find the lane of a namespace again.
func (l *emLayout) lane(f *emFrame) *emLane {
	k := emLanes[f.kind]
	index, label := k.index, k.label
	if parts := strings.Split(f.ident, "."); len(parts) == 2 {
		index = k.index
		for i := range l.lanes {
			if i > k.index && i < k.index+100 {
				index = max(index, i)
			}
		}
		index++
		label = k.prefix + parts[0]
	}
	ln, ok := l.lanes[index]
	if !ok {
		ln = &emLane{index: index, label: label, maxBoxH: emPx(emLaneMinH)}
		l.lanes[index] = ln
	}
	return ln
}

// box measures the box of a frame: its name, which drops a namespace, and
// its data.
func (l *emLayout) box(f *emFrame) *emBox {
	name := f.ident
	if parts := strings.Split(name, "."); len(parts) == 2 {
		name = parts[1]
	}
	limit := emPx(emTextMaxW)
	b := &emBox{f: f, names: emWrap(name, emNameFont, limit, l.meas)}
	if data, ok := l.data(f); ok {
		for line := range strings.SplitSeq(decodeEntities(lineBreak.ReplaceAllString(data, "\n")), "\n") {
			line = strings.TrimSpace(emSpaces.ReplaceAllString(line, " "))
			if line != "" {
				b.data = append(b.data, emWrap(line, emDataFont, limit, l.meas)...)
			}
		}
	}
	// Mermaid measures the name with its tags of HTML, <b> and </b>, so
	// that a box is wider than its name. zuzu measures the same text, so
	// that the boxes keep the widths and the places of Mermaid.
	tw := 0.0
	for _, s := range b.names {
		tw = max(tw, l.meas.Width(emNameFont, "<b>"+s+"</b>"))
	}
	for _, s := range b.data {
		tw = max(tw, l.meas.Width(emDataFont, s))
	}
	b.w = max(emPx(emBoxMinW), min(emPx(emBoxMaxW), tw+2*emPx(emTextPad))) + 2*emPx(emBoxPad)
	b.h = max(emPx(emBoxMinH), min(emPx(emBoxMaxH), l.textHeight(b)+2*emPx(emTextPad))) + 2*emPx(emBoxPad)
	return b
}

// textHeight returns the height of the text of a box: its name, and its
// data under an empty line.
func (l *emLayout) textHeight(b *emBox) float64 {
	n := len(b.names)
	if len(b.data) > 0 {
		n += 1 + len(b.data)
	}
	return float64(n) * lh(emNameFont)
}

// data returns the data of a frame, as Mermaid 12.3 cuts it from its
// source: the text after the first { and its new line, up to the
// character before the last }. The data block that the frame names wins
// over the data in its line. Data in quotes holds no }, so it is empty, as
// in Mermaid.
func (l *emLayout) data(f *emFrame) (string, bool) {
	// cut drops the character before the last } and the rest.
	cut := func(s string) string {
		end := strings.LastIndex(s, "}")
		if end <= 0 {
			return ""
		}
		s = s[:end]
		_, size := utf8.DecodeLastRuneInString(s)
		return s[:len(s)-size]
	}
	if block, ok := l.m.data[f.ref]; ok && f.ref != "" {
		// With no new line right after the {, Mermaid starts after the
		// character after the {.
		start := 1
		if i := strings.Index(block, "{\n"); i >= 0 {
			start = i + 2
		}
		return cut(block[start:]), true
	}
	if f.hasInline {
		return cut(f.inline[strings.Index(f.inline, "{")+1:]), true
	}
	return "", false
}

// relations finds the arrows of the boxes, as Mermaid 12.3 does. A frame
// that names frames with ->> takes an arrow from each that is before it or
// is itself, in the order of the source. Else the first frame and a reset
// frame take none, and each other frame takes one from the last frame
// before it in another swimlane.
func (l *emLayout) relations() {
	for i, b := range l.boxes {
		if len(b.f.sources) > 0 {
			for _, src := range l.boxes[:i+1] {
				if slices.Contains(b.f.sources, src.f.name) {
					l.rels = append(l.rels, emRel{src, b})
				}
			}
			continue
		}
		if i == 0 || b.f.reset {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if l.boxes[j].lane != b.lane {
				l.rels = append(l.rels, emRel{l.boxes[j], b})
				break
			}
		}
	}
}

// emWrap wraps a text into lines no wider than a width, as Mermaid wraps
// the label of a box of an event model. A word goes on the line before it
// while the line holds it with a space after it, and on a new line when
// not. A word wider than the width breaks into parts that each end with a
// hyphen.
func emWrap(s string, f text.Font, limit float64, m text.Measurer) []string {
	var lines []string
	next := ""
	for word := range strings.SplitSeq(s, " ") {
		if word == "" {
			continue
		}
		wl := m.Width(f, word+" ")
		switch {
		case wl > limit:
			lines = append(lines, next)
			part := ""
			runes := []rune(word)
			for k, r := range runes {
				test := part + string(r)
				if m.Width(f, test) < limit {
					part = test
					continue
				}
				if k == len(runes)-1 {
					lines = append(lines, test)
				} else {
					lines = append(lines, test+"-")
				}
				part = ""
			}
			next = part
		case m.Width(f, next)+wl >= limit:
			lines = append(lines, next)
			next = word
		case next == "":
			next = word
		default:
			next += " " + word
		}
	}
	lines = append(lines, next)
	return slices.DeleteFunc(lines, func(s string) bool { return s == "" })
}

// spec writes the layout as a Vega spec: the swimlanes as panels with
// their labels, the boxes in the colors of their kinds with their texts,
// and the arrows over them, as Mermaid draws them.
func (l *emLayout) spec() map[string]any {
	var lanes, laneLabels, boxes, uis, names, datas, rels, arrows []any
	width := l.right + emPx(emLanePad)
	line := lh(emNameFont)
	for _, ln := range l.sortedLanes() {
		id := "lane" + itoa(ln.index)
		lanes = append(lanes, map[string]any{"id": id, "label": ln.label, "x": 0.0, "y": ln.y, "width": width, "height": ln.h})
		laneLabels = append(laneLabels, map[string]any{"id": id, "text": ln.label, "x": emPx(emLabelX), "y": ln.y + emPx(emLabelY)})
	}
	top := func(b *emBox) float64 { return b.lane.y + emPx(emLanePad) }
	for _, b := range l.boxes {
		y := top(b)
		r := map[string]any{"id": b.f.name, "kind": b.f.kind, "label": b.f.ident, "lane": "lane" + itoa(b.lane.index),
			"x": b.x, "y": y, "width": b.w, "height": b.h}
		if b.f.kind == "ui" {
			uis = append(uis, r)
		} else {
			boxes = append(boxes, r)
		}
		// The text sits in the middle of the box: the name in the middle
		// of each line, and the data at the left, under an empty line.
		ty := y + b.h/2 - l.textHeight(b)/2 + line/2
		for _, s := range b.names {
			names = append(names, map[string]any{"id": b.f.name, "text": s, "x": b.x + b.w/2, "y": ty})
			ty += line
		}
		ty += line
		for _, s := range b.data {
			datas = append(datas, map[string]any{"id": b.f.name, "text": s, "x": b.x + emPx(emBoxPad+emTextPad), "y": ty})
			ty += line
		}
	}
	glyph, _ := graph.ArrowPaths("normal")
	for _, r := range l.rels {
		sy, ty := top(r.from), top(r.to)
		x0, x1 := r.from.x+r.from.w*2/3, r.to.x+r.to.w/3
		y0, y1 := sy+r.from.h, ty
		if sy > ty {
			// The arrow goes up, from the top of its source to the bottom
			// of its target.
			y0, y1 = sy, ty+r.to.h
		}
		angle := math.Atan2(y1-y0, x1-x0)
		// The line stops inside the arrowhead, so that its round end does
		// not show past the tip.
		back := theme.ArrowLength / 2
		ex, ey := x1-back*math.Cos(angle), y1-back*math.Sin(angle)
		rels = append(rels, map[string]any{
			"id": r.from.f.name + "-" + r.to.f.name, "source": r.from.f.name, "target": r.to.f.name,
			"path": "M" + num(x0) + "," + num(y0) + "L" + num(ex) + "," + num(ey),
		})
		arrows = append(arrows, map[string]any{
			"id": r.from.f.name + "-" + r.to.f.name, "x": x1, "y": y1, "angle": angle * 180 / math.Pi, "glyph": glyph,
		})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	rect := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["width"], enc["height"] = field("x"), field("y"), field("width"), field("height")
		return enc
	}
	label := func(align string, f text.Font) map[string]any {
		return map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"), "align": value(align), "baseline": value("middle"),
			"fontSize": value(f.Size), "fontWeight": value(f.Weight),
		}
	}
	color := map[string]any{"scale": "color", "field": "kind"}
	data := []any{}
	var marks []any
	mark := func(typ, set string, rows []any, style string, enc map[string]any) {
		data = append(data, map[string]any{"name": set, "values": orEmpty(rows)})
		marks = append(marks, map[string]any{
			"type": typ, "name": set, "from": map[string]any{"data": set},
			"style": []any{style}, "encode": map[string]any{"update": enc},
		})
	}
	mark("rect", "lanes", lanes, graph.StyleCluster, rect(map[string]any{}))
	mark("text", "laneLabels", laneLabels, graph.StyleClusterTitle, label("left", emNameFont))
	mark("rect", "boxes", boxes, graph.StyleNode, rect(map[string]any{"fill": color, "stroke": color, "cornerRadius": value(4)}))
	mark("rect", "uiBoxes", uis, graph.StyleNodeMuted, rect(map[string]any{"cornerRadius": value(4)}))
	mark("text", "names", names, graph.StyleNodeLabel, label("center", emNameFont))
	mark("text", "data", datas, graph.StyleNodeLabel, label("left", emDataFont))
	mark("path", "relations", rels, graph.StyleEdge, map[string]any{"x": value(0), "y": value(0), "path": field("path")})
	mark("symbol", "arrows", arrows, graph.StyleArrow, map[string]any{
		"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("glyph"), "size": value(graph.ArrowSymbolSize),
	})
	return map[string]any{
		"padding":  8,
		"autosize": "pad",
		"data":     data,
		"scales":   []any{map[string]any{"name": "color", "type": "ordinal", "domain": emColors, "range": "category"}},
		"marks":    marks,
	}
}
