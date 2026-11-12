package mermaid

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
)

// ishScale turns the pixels of Mermaid 12.3 into the pixels of zuzu in an
// ishikawa diagram. Mermaid draws its labels at 16 pixels, and zuzu at 11,
// so each length of Mermaid scales by 11/16 and a label keeps its place
// along its bone (D32).
const ishScale = 11.0 / 16

// The sizes of an ishikawa diagram in the pixels of Mermaid 12.3, which
// ishScale turns into the pixels of zuzu. The spine is 250 pixels high on
// each side at least. A bone of a cause leans 82 degrees from the spine. A
// bone along the spine is 30 pixels long, or 60 and 5 for each cause that
// it has. A cause with no causes has a bone of 0.2 of the length of its
// side. The spine starts 20 pixels left of the head.
const (
	ishSpine    = 250.0
	ishStub     = 30.0
	ishLong     = 60.0
	ishPerChild = 5.0
	ishShort    = 0.2
	ishStart    = 20.0
	ishAngle    = 82 * math.Pi / 180
	// ishWrap is the number of characters at which Mermaid wraps the
	// label of a cause of a cause.
	ishWrap = 15
)

// The sizes of the look of zuzu in an ishikawa diagram, in pixels: the
// space between a label and the end of its bone, the width of the spine
// and of the bone of a cause, and the length of the arrowhead of a smaller
// bone.
const (
	ishGap        = 3.0
	ishSpineWidth = 2.5
	ishSmallArrow = 0.75 * theme.ArrowLength
)

// The fonts of an ishikawa diagram: the problem is bold, and the causes are
// not.
var (
	ishHeadFont = text.Font{Family: text.Family, Size: 11, Weight: boldWeight}
	ishFont     = text.Font{Family: text.Family, Size: 11, Weight: 400}
)

// ishBreak matches a break between two lines of a label: br, or a new line
// that the wrap of the label makes.
var ishBreak = regexp.MustCompile(`(?i)<br\s*/?>|\n`)

// ishLayout lays out an ishikawa diagram and writes its rows.
type ishLayout struct {
	k *ishikawa
	m *text.Estimate

	rows                   map[string][]any
	minX, minY, maxX, maxY float64
	// left is the least x of the labels of the pair of causes that the
	// layout draws, and boxes holds the left of the box of each of the two
	// causes, above and under the spine.
	left  float64
	boxes [2]float64
}

// ishBone is a bone that other bones start on: its ends, the number of
// its causes, and the number of them that start on it so far.
type ishBone struct {
	x0, y0, x1, y1 float64
	kids, drawn    int
}

// ishEntry is a cause of a cause, in the order in which Mermaid places it.
type ishEntry struct {
	depth  int
	lines  []string
	parent int
	kids   int
}

// newIshLayout lays out an ishikawa diagram as Mermaid 12.3 does. The head
// is at the right end of the spine. The causes alternate above and under
// the spine, from right to left, in pairs. A pair starts left of the
// labels of the pair before it. A box of a cause can reach right of the
// start of its bone, and Mermaid draws it over the box of the cause before
// it on its side. zuzu moves the pair left so that the two boxes do not
// overlap.
func newIshLayout(k *ishikawa) *ishLayout {
	l := &ishLayout{
		k: k, m: text.NewEstimate(), rows: map[string][]any{},
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	causes := k.root.kids
	if len(causes) == 0 {
		l.head(0)
		return l
	}
	var upper, lower []*ishNode
	for i, n := range causes {
		if i%2 == 0 {
			upper = append(upper, n)
		} else {
			lower = append(lower, n)
		}
	}
	upTotal, upMax := ishStats(upper)
	lowTotal, lowMax := ishStats(lower)
	upLen, lowLen := ishSpine, ishSpine
	if total := upTotal + lowTotal; total > 0 {
		pool, least := 2*ishSpine, 0.3*ishSpine
		upLen = math.Max(least, pool*float64(upTotal)/float64(total))
		lowLen = math.Max(least, pool*float64(lowTotal)/float64(total))
	}
	spacing := 2 * k.fontSize
	upLen = math.Max(upLen, float64(upMax)*spacing) * ishScale
	lowLen = math.Max(lowLen, float64(lowMax)*spacing) * ishScale
	spineY := math.Max(upLen, ishSpine*ishScale)
	l.head(spineY)
	x := -ishStart * ishScale
	for p := 0; p*2 < len(causes); p++ {
		pair := []*ishNode{causes[p*2]}
		lens := []float64{upLen, lowLen}
		if p*2+1 < len(causes) {
			pair = append(pair, causes[p*2+1])
		}
		if p > 0 {
			shift := 0.0
			for i, n := range pair {
				reach, bw := l.cause(n, lens[i])
				shift = math.Max(shift, x-reach+bw/2+ishGap-l.boxes[i])
			}
			x -= shift
		}
		l.left = math.Inf(1)
		for i, n := range pair {
			l.branch(n, x, spineY, 2*i-1, lens[i])
		}
		x = l.left
	}
	l.add("spine", map[string]any{"x": x, "y": spineY, "x2": 0.0, "y2": spineY}, x, spineY, 0, spineY)
	return l
}

// ishStats returns the number of the causes under the causes of one side,
// and the most under one of them.
func ishStats(nodes []*ishNode) (int, int) {
	var count func(n *ishNode) int
	count = func(n *ishNode) int {
		c := 0
		for _, k := range n.kids {
			c += 1 + count(k)
		}
		return c
	}
	total, most := 0, 0
	for _, n := range nodes {
		c := count(n)
		total += c
		most = max(most, c)
	}
	return total, most
}

// ishLines returns the lines of a label. A label with more characters than
// limit wraps at its spaces, by the count of its characters, as in Mermaid.
// A limit of 0 does not wrap. br starts a new line, and the entities read
// as their characters.
func ishLines(s string, limit int) []string {
	if limit > 0 && utf8.RuneCountInString(s) > limit {
		var lines []string
		for w := range strings.FieldsSeq(s) {
			if n := len(lines) - 1; n >= 0 && utf8.RuneCountInString(lines[n])+1+utf8.RuneCountInString(w) <= limit {
				lines[n] += " " + w
			} else {
				lines = append(lines, w)
			}
		}
		s = strings.Join(lines, "\n")
	}
	var out []string
	for _, p := range ishBreak.Split(s, -1) {
		out = append(out, strings.TrimSpace(decodeEntities(p)))
	}
	return out
}

// width returns the width of the widest line of a label.
func (l *ishLayout) width(lines []string, f text.Font) float64 {
	w := 0.0
	for _, s := range lines {
		w = math.Max(w, l.m.Width(f, s))
	}
	return w
}

// head writes the head at the right end of the spine: the problem in a
// shape like the head of a fish. Mermaid wraps the problem at 110 pixels
// of its font size, in characters at 0.6 of the size each.
func (l *ishLayout) head(y float64) {
	limit := max(6, int(math.Floor(110/(l.k.fontSize*0.6))))
	lines := ishLines(l.k.root.text, limit)
	tw, th := l.width(lines, ishHeadFont), float64(len(lines))*lh(ishHeadFont)
	w := math.Max(60*ishScale, tw+6*ishScale)
	h := math.Max(40*ishScale, 2*th+40*ishScale)
	path := "M0," + num(-h/2) + "L0," + num(h/2) + "Q" + num(2.4*w) + ",0 0," + num(-h/2) + "Z"
	l.add("head", map[string]any{"x": 0.0, "y": y, "path": path}, 0, y-h/2, 1.2*w, y+h/2)
	l.labelRows("headLabels", lines, w/2+3*ishScale, y, "middle", ishHeadFont)
}

// branch writes a cause on a side of the spine, -1 above and 1 under, its
// bone of a length from a point of the spine, its label at the end of the
// bone, and the bones of its causes.
func (l *ishLayout) branch(n *ishNode, x, y float64, dir int, length float64) {
	if len(n.kids) == 0 {
		length *= ishShort
	}
	dirF := float64(dir)
	dx, dy := -math.Cos(ishAngle)*length, math.Sin(ishAngle)*length*dirF
	ex, ey := x+dx, y+dy
	l.bone("bones", x, y, ex, ey, theme.ArrowLength)
	// The label of a cause is in a box at the end of its bone. Mermaid
	// does not wrap it.
	lines := ishLines(n.text, 0)
	bw := l.width(lines, ishFont) + 40*ishScale
	th := float64(len(lines)) * lh(ishFont)
	ly := ey + dirF*(lh(ishFont)/2+2)
	bh := th + 4
	l.add("causes", map[string]any{
		"text": strings.Join(lines, " "), "x": ex - bw/2, "y": ly - bh/2, "x2": ex + bw/2, "y2": ly + bh/2,
	}, ex-bw/2, ly-bh/2, ex+bw/2, ly+bh/2)
	l.labelRows("causeLabels", lines, ex, ly, "middle", ishFont)
	l.left = math.Min(l.left, ex-(bw-40*ishScale)/2)
	l.boxes[(dir+1)/2] = ex - bw/2
	if len(n.kids) == 0 {
		return
	}
	entries, order := ishFlatten(n.kids, dir)
	ys := make([]float64, len(entries))
	for slot, i := range order {
		ys[i] = y + dy*float64(slot+1)/float64(len(entries)+1)
	}
	bones := map[int]*ishBone{-1: {x0: x, y0: y, x1: ex, y1: ey, kids: len(n.kids)}}
	diagX, diagY := -math.Cos(ishAngle), math.Sin(ishAngle)*dirF
	baseline := "top"
	if dir < 0 {
		baseline = "alphabetic"
	}
	for i, e := range entries {
		y1 := ys[i]
		p := bones[e.parent]
		var bx0, by0, bx1 float64
		if e.depth%2 == 0 {
			// A bone along the spine, from a bone that leans.
			t := 0.5
			if d := p.y1 - p.y0; d != 0 {
				t = (y1 - p.y0) / d
			}
			bx0, by0 = p.x0+(p.x1-p.x0)*t, y1
			long := ishStub
			if e.kids > 0 {
				long = ishLong + float64(e.kids)*ishPerChild
			}
			bx1 = bx0 - long*ishScale
			l.bone("subBones", bx0, by0, bx1, y1, ishSmallArrow)
			l.labelRows("subLabels", e.lines, bx1-ishGap, y1, "middle", ishFont)
		} else {
			// A bone that leans as the bone of a cause, from a bone along
			// the spine. The bones start from the left of their bone to
			// its right, in the order of the source on each side.
			k := p.drawn
			p.drawn++
			bx0 = p.x0 + (p.x1-p.x0)*float64(p.kids-k)/float64(p.kids+1)
			by0 = p.y0
			bx1 = bx0 + diagX*((y1-by0)/diagY)
			l.bone("subBones", bx0, by0, bx1, y1, ishSmallArrow)
			l.labelRows("subLabels", e.lines, bx1-ishGap, y1, baseline, ishFont)
		}
		l.left = math.Min(l.left, bx1-ishGap-l.width(e.lines, ishFont))
		if e.kids > 0 {
			bones[i] = &ishBone{x0: bx0, y0: by0, x1: bx1, y1: y1, kids: e.kids}
		}
	}
}

// cause returns how far left of the start of its bone the bone of a
// cause of a length ends, and the width of the box of its label.
func (l *ishLayout) cause(n *ishNode, length float64) (float64, float64) {
	if len(n.kids) == 0 {
		length *= ishShort
	}
	return math.Cos(ishAngle) * length, l.width(ishLines(n.text, 0), ishFont) + 40*ishScale
}

// ishFlatten returns the causes under a cause in the order in which they
// start on their bones, and the order of their places along the bone of
// the cause, from the spine out. Above the spine, each list of causes
// goes in the other order, as in Mermaid. A cause on a bone along the
// spine comes before its causes, and a cause on a bone that leans comes
// after them.
func ishFlatten(kids []*ishNode, dir int) ([]ishEntry, []int) {
	var entries []ishEntry
	var order []int
	var walk func(nodes []*ishNode, parent, depth int)
	walk = func(nodes []*ishNode, parent, depth int) {
		if dir < 0 {
			nodes = slices.Clone(nodes)
			slices.Reverse(nodes)
		}
		for _, n := range nodes {
			i := len(entries)
			entries = append(entries, ishEntry{depth: depth, lines: ishLines(n.text, ishWrap), parent: parent, kids: len(n.kids)})
			if depth%2 == 0 {
				order = append(order, i)
				walk(n.kids, i, depth+1)
			} else {
				walk(n.kids, i, depth+1)
				order = append(order, i)
			}
		}
	}
	walk(kids, -1, 2)
	return entries, order
}

// bone writes a bone from a point to its end, with an arrowhead of a length
// at the point. The line stops inside the arrowhead.
func (l *ishLayout) bone(set string, x0, y0, x1, y1, arrow float64) {
	d := math.Hypot(x1-x0, y1-y0)
	if d == 0 {
		return
	}
	ux, uy := (x1-x0)/d, (y1-y0)/d
	sx, sy := x0+ux*arrow/2, y0+uy*arrow/2
	l.add(set, map[string]any{"x": sx, "y": sy, "x2": x1, "y2": y1}, x0, y0, x1, y1)
	fill, _ := graph.ArrowPaths("normal")
	l.rows["arrows"] = append(l.rows["arrows"], map[string]any{
		"x": x0, "y": y0, "angle": math.Round(math.Atan2(-uy, -ux)*180/math.Pi*100) / 100,
		"shape": fill, "size": arrow * arrow,
	})
}

// labelRows writes a label as a row for each line, the lines around y. The
// label ends at x, or is in the middle of x for the problem and a cause.
// The baseline is middle, alphabetic for a label over the end of its bone,
// or top for a label under it.
func (l *ishLayout) labelRows(set string, lines []string, x, y float64, baseline string, f text.Font) {
	h := lh(f)
	align := "right"
	if set != "subLabels" {
		align = "center"
	}
	y0 := y - float64(len(lines)-1)*h/2
	for i, s := range lines {
		ly := y0 + float64(i)*h
		w := l.m.Width(f, s)
		x0, x1 := x-w, x
		if align == "center" {
			x0, x1 = x-w/2, x+w/2
		}
		top, bottom := ly-h/2, ly+h/2
		switch baseline {
		case "alphabetic":
			top, bottom = ly-h*0.8, ly+h*0.2
		case "top":
			top, bottom = ly, ly+h
		}
		l.add(set, map[string]any{
			"text": s, "line": i, "x": x, "y": ly, "align": align, "baseline": baseline,
		}, x0, top, x1, bottom)
	}
}

// add appends a row to a set and grows the bounds by a box.
func (l *ishLayout) add(set string, row map[string]any, x0, y0, x1, y1 float64) {
	l.rows[set] = append(l.rows[set], row)
	l.minX, l.maxX = math.Min(l.minX, math.Min(x0, x1)), math.Max(l.maxX, math.Max(x0, x1))
	l.minY, l.maxY = math.Min(l.minY, math.Min(y0, y1)), math.Max(l.maxY, math.Max(y0, y1))
}

// spec writes the rows of the layout as a Vega spec. Each set of rows is a
// data set, and each mark draws one set in a style of a diagram (D31). The
// rows move so that the diagram starts at 0. The problem and the causes
// take the first color of the scheme, as Mermaid draws them in one color.
func (l *ishLayout) spec() map[string]any {
	dx, dy := -l.minX, -l.minY
	var data []any
	for _, set := range slices.Sorted(maps.Keys(l.rows)) {
		rows := l.rows[set]
		for _, r := range rows {
			m, _ := r.(map[string]any)
			for k, d := range map[string]float64{"x": dx, "x2": dx, "y": dy, "y2": dy} {
				if v, ok := m[k].(float64); ok {
					m[k] = math.Round((v+d)*100) / 100
				}
			}
		}
		data = append(data, map[string]any{"name": set, "values": rows})
	}
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	value := func(v any) map[string]any { return map[string]any{"value": v} }
	var marks []any
	mark := func(typ, name, set, style string, enc map[string]any) {
		if len(l.rows[set]) == 0 {
			return
		}
		marks = append(marks, map[string]any{
			"type": typ, "name": name, "from": map[string]any{"data": set},
			"style": []any{style}, "encode": map[string]any{"update": enc},
		})
	}
	rule := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["x2"], enc["y2"] = field("x"), field("y"), field("x2"), field("y2")
		return enc
	}
	path := func(enc map[string]any) map[string]any {
		enc["x"], enc["y"], enc["path"] = field("x"), field("y"), field("path")
		return enc
	}
	mark("rule", "spine", "spine", graph.StyleEdge, rule(map[string]any{"strokeWidth": value(ishSpineWidth)}))
	mark("rule", "bones", "bones", graph.StyleEdge, rule(map[string]any{"strokeWidth": value(ishSpineWidth)}))
	mark("rule", "subBones", "subBones", graph.StyleEdge, rule(map[string]any{}))
	mark("symbol", "arrows", "arrows", graph.StyleArrow, map[string]any{
		"x": field("x"), "y": field("y"), "angle": field("angle"), "shape": field("shape"), "size": field("size"),
	})
	// The head and the box of each cause have an opaque back under their
	// tint, so that a bone does not show through them.
	mark("path", "headBack", "head", graph.StyleEdgeLabelBox, path(map[string]any{"fillOpacity": value(1), "strokeOpacity": value(0)}))
	mark("path", "head", "head", graph.StyleNode, path(map[string]any{}))
	mark("rect", "causeBacks", "causes", graph.StyleEdgeLabelBox, rule(map[string]any{
		"fillOpacity": value(1), "strokeOpacity": value(0), "cornerRadius": value(4),
	}))
	mark("rect", "causes", "causes", graph.StyleNode, rule(map[string]any{"cornerRadius": value(4)}))
	label := func(f text.Font) map[string]any {
		return map[string]any{
			"x": field("x"), "y": field("y"), "text": field("text"),
			"align": field("align"), "baseline": field("baseline"),
			"fontSize": value(f.Size), "fontWeight": value(f.Weight),
		}
	}
	mark("text", "headLabels", "headLabels", graph.StyleNodeLabel, label(ishHeadFont))
	mark("text", "causeLabels", "causeLabels", graph.StyleNodeLabel, label(ishFont))
	mark("text", "subLabels", "subLabels", graph.StyleNodeLabel, label(ishFont))
	pad := l.k.padding * 8 / 20
	return map[string]any{
		"width":    math.Ceil(l.maxX - l.minX),
		"height":   math.Ceil(l.maxY - l.minY),
		"autosize": "pad",
		"padding":  map[string]any{"left": pad, "right": pad, "top": pad, "bottom": pad},
		"data":     data,
		"marks":    marks,
	}
}
