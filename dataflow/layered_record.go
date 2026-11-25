package dataflow

import (
	"math"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
)

// record is a node with fields, as a record of DOT, a class of UML or an
// entity (D31). The fields lie side by side when across is true, and one
// under another otherwise. A field can be a group in braces of a record of
// DOT, a record of its own whose fields lie the other way.
type record struct {
	across, stack bool
	texts, ports  []string
	// cells holds the cells of each field, aligns its alignment, and fonts
	// the font of each line of its text, or of each of its cells, as b, i
	// and u.
	cells  [][]string
	aligns []string
	fonts  [][]string
	// groups holds the record of each field that is a group, or nil.
	groups []*record
	// sizes holds the size of each field along the axis of the fields, and
	// cols the width of each column of the cells, 0 for an empty column.
	sizes []float64
	cols  []float64
	// lines holds the size of each line of each field with fonts or an
	// alignment of its own, which draw one line at a time.
	lines [][][2]float64
	// math lays out a text with math of KaTeX in a weight, or returns nil
	// for a text that draws as text (D36). It is nil for a graph with no
	// math.
	math func(v any, weight string) *mathLabel
}

// mathOf returns the layout of a text with math in a weight, or nil.
func (rec *record) mathOf(s, weight string) *mathLabel {
	if rec.math == nil {
		return nil
	}
	return rec.math(s, weight)
}

// measureFunc measures a text at the size of the layout, in a weight.
type measureFunc func(v any, weight string) (float64, float64)

// readRecord reads the fields of a row of a node, or returns nil for a
// node with no fields.
func readRecord(r Row, across bool) *record {
	fs, ok := r["fields"].([]any)
	if !ok || len(fs) == 0 {
		return nil
	}
	stack, _ := r["stack"].(bool)
	rec := &record{across: across && !stack, stack: stack}
	ps, _ := r["ports"].([]any)
	cells, _ := r["cells"].([]any)
	aligns, _ := r["aligns"].([]any)
	fonts, _ := r["fonts"].([]any)
	groups, _ := r["groups"].([]any)
	at := func(list []any, j int) any {
		if j < len(list) {
			return list[j]
		}
		return nil
	}
	for j, f := range fs {
		rec.texts = append(rec.texts, expr.String(f))
		rec.ports = append(rec.ports, strOf(at(ps, j)))
		rec.aligns = append(rec.aligns, strOf(at(aligns, j)))
		var cs, ft []string
		if l, ok := at(cells, j).([]any); ok {
			for _, c := range l {
				cs = append(cs, strOf(c))
			}
		}
		if l, ok := at(fonts, j).([]any); ok {
			for _, c := range l {
				ft = append(ft, strOf(c))
			}
		}
		rec.cells = append(rec.cells, cs)
		rec.fonts = append(rec.fonts, ft)
		var sub *record
		if g, ok := at(groups, j).(map[string]any); ok {
			// A group lies the other way from the fields around it.
			sub = readRecord(g, !rec.across)
		}
		rec.groups = append(rec.groups, sub)
	}
	return rec
}

// oneByOne reports whether the field j draws one line at a time: it has
// fonts, or it puts a line at the left or at the right.
func (rec *record) oneByOne(j int) bool {
	return len(rec.fonts[j]) > 0 || strings.ContainsAny(rec.aligns[j], "lr")
}

// align returns the alignment of the line k of the field j: l, c or r.
// The last letter of the alignment of the field holds for the lines after
// it.
func (rec *record) align(j, k int) byte {
	a := rec.aligns[j]
	switch {
	case a == "":
		return 'c'
	case k < len(a):
		return a[k]
	}
	return a[len(a)-1]
}

// font returns the code of the font of the line or the cell k of the
// field j.
func (rec *record) font(j, k int) string {
	if k < len(rec.fonts[j]) {
		return rec.fonts[j][k]
	}
	return ""
}

// weight returns the weight of a font code, for the measure.
func weight(code string) string {
	if strings.Contains(code, "b") {
		return graph.BoldWeight
	}
	return ""
}

// size measures the record, with a padding around each field, and returns
// the size of its node. The cells of the fields one under another line up
// in columns. A group takes the size of its fields.
func (rec *record) size(measure measureFunc, padding float64) (float64, float64) {
	var w, h float64
	// Each column takes the width of its widest cell.
	for j, cs := range rec.cells {
		if rec.across {
			// Fields side by side have no columns, so the cells join
			// into one text.
			if len(cs) > 0 && rec.texts[j] == "" {
				rec.texts[j] = strings.Join(cs, " ")
				rec.fonts[j] = nil
			}
			rec.cells[j] = nil
			continue
		}
		for k, c := range cs {
			for len(rec.cols) <= k {
				rec.cols = append(rec.cols, 0)
			}
			if tw, _ := measure(c, weight(rec.font(j, k))); tw > 0 {
				rec.cols[k] = math.Max(rec.cols[k], tw+2*padding)
			}
		}
	}
	table := 0.0
	for _, c := range rec.cols {
		table += c
	}
	rec.lines = make([][][2]float64, len(rec.texts))
	for j, t := range rec.texts {
		var fw, fh float64
		if sub := rec.groups[j]; sub != nil {
			sub.math = rec.math
			fw, fh = sub.size(measure, padding)
		} else {
			var tw, th float64
			switch {
			case len(rec.cells[j]) > 0:
				tw = table - 2*padding
				for k, c := range rec.cells[j] {
					_, ch := measure(c, weight(rec.font(j, k)))
					th = math.Max(th, ch)
				}
			case rec.oneByOne(j):
				for k, l := range strings.Split(t, "\n") {
					lw, lh := measure(l, weight(rec.font(j, k)))
					if lh == 0 {
						_, lh = measure("X", "")
					}
					// A line that draws on its own takes a gap of a fifth
					// of its height, so that a line under it stays clear
					// of the next line.
					lh *= 1.2
					rec.lines[j] = append(rec.lines[j], [2]float64{lw, lh})
					tw, th = math.Max(tw, lw), th+lh
				}
			default:
				tw, th = measure(t, "")
			}
			fw, fh = math.Max(tw+2*padding, 24), math.Max(th+1.6*padding, 24)
			if rec.stack && t == "" && len(rec.cells[j]) == 0 {
				// An empty field of a stacked record is a thin band.
				fh = padding
			}
		}
		if rec.across {
			rec.sizes = append(rec.sizes, fw)
			w += fw
			h = math.Max(h, fh)
		} else {
			rec.sizes = append(rec.sizes, fh)
			h += fh
			w = math.Max(w, fw)
		}
	}
	// The last column that holds text takes the rest of the width.
	for k := len(rec.cols) - 1; k >= 0; k-- {
		if rec.cols[k] > 0 {
			rec.cols[k] += w - table
			break
		}
	}
	return w, h
}

// fieldBoxes returns the box of each field of the record in a box, as x,
// y, width and height. With spread, the room that the box has along the
// axis of the fields past their sizes goes to the fields in equal parts,
// as Graphviz gives it to the fields of a group.
func (rec *record) fieldBoxes(x, y, w, h float64, spread bool) [][4]float64 {
	extra := 0.0
	if spread && len(rec.sizes) > 0 {
		total := 0.0
		for _, s := range rec.sizes {
			total += s
		}
		along := h
		if rec.across {
			along = w
		}
		extra = math.Max(along-total, 0) / float64(len(rec.sizes))
	}
	out := make([][4]float64, len(rec.sizes))
	at := 0.0
	for j, s := range rec.sizes {
		s += extra
		if rec.across {
			out[j] = [4]float64{x + at, y, s, h}
		} else {
			out[j] = [4]float64{x, y + at, w, s}
		}
		at += s
	}
	return out
}

// boxes returns the box of each port of the record, as x, y, width and
// height from the center of a node of a size.
func (rec *record) boxes(w, h float64) map[string][4]float64 {
	m := map[string][4]float64{}
	rec.portBoxes(m, -w/2, -h/2, w, h, false)
	return m
}

// portBoxes adds the box of each port of the record in a box to m, and of
// each port of its groups.
func (rec *record) portBoxes(m map[string][4]float64, x, y, w, h float64, spread bool) {
	for j, b := range rec.fieldBoxes(x, y, w, h, spread) {
		if sub := rec.groups[j]; sub != nil {
			sub.portBoxes(m, b[0], b[1], b[2], b[3], true)
			continue
		}
		if name := rec.ports[j]; name != "" {
			m[name] = b
		}
	}
}

// rows returns the path of the record of a node and the rows that draw
// its text: a row for each field, for each line of a field that draws one
// line at a time, and for each cell, and a row for each line under a line
// of text. fontSize is the size of the font of the layout.
func (rec *record) rows(r Row, np graph.NodePlace, kind string, padding, fontSize float64) (string, []Row) {
	var path strings.Builder
	path.WriteString(graph.RecordPath(kind, np.Width, np.Height, rec.across, rec.sizes))
	var out []Row
	size := fontSize
	if v := expr.Number(r["fontSize"]); v > 0 {
		size = v
	}
	// text writes the row of a text whose lines are in the middle of y. The
	// row takes the y of the first line, which the text draws on.
	text := func(x, y float64, lines []any, align, code string) Row {
		f := clone(r)
		f["kind"] = "field"
		f["x"], f["y"], f["lines"], f["align"] = x, graph.FirstLine(y, len(lines), size, "middle"), lines, align
		// A line takes the style of the font of its node, or italic.
		if strings.Contains(code, "i") {
			f["fontStyle"] = "italic"
		}
		if strings.Contains(code, "b") {
			f["fontWeight"] = graph.BoldWeight
		}
		graph.TextSignature(f)
		return f
	}
	// maths writes the rows of a text with math at x and y, at its left
	// or at its center, in the font of a code, or returns false for a
	// text with no math.
	maths := func(rec *record, t string, x, y float64, left bool, code string) bool {
		ml := rec.mathOf(t, weight(code))
		if ml == nil {
			return false
		}
		base := mathBase(r, "textColor")
		if strings.Contains(code, "i") {
			base["fontStyle"] = "italic"
		}
		out = append(out, ml.rows(base, "node", x, y, left, false)...)
		return true
	}
	split := func(t string) []any {
		lines := []any{}
		for l := range strings.SplitSeq(t, "\n") {
			lines = append(lines, l)
		}
		return lines
	}
	// field writes the rows of the field j of a record in its box.
	field := func(rec *record, j int, fx, fy, fw, fh float64) {
		t := rec.texts[j]
		cx, cy := fx+fw/2, fy+fh/2
		switch {
		case len(rec.cells[j]) > 0:
			x := fx
			var xs []float64
			for k, c := range rec.cells[j] {
				if k >= len(rec.cols) || rec.cols[k] == 0 {
					continue
				}
				if x > fx {
					xs = append(xs, x-np.X)
				}
				if c != "" && !maths(rec, c, x+padding, cy, true, rec.font(j, k)) {
					out = append(out, text(x+padding, cy, split(c), "left", rec.font(j, k)))
				}
				x += rec.cols[k]
			}
			path.WriteString(graph.ColumnPath(xs, fy-np.Y, fy+fh-np.Y))
		case rec.oneByOne(j):
			var block float64
			for _, l := range rec.lines[j] {
				block += l[1]
			}
			y := cy - block/2
			for k, l := range strings.Split(t, "\n") {
				lw, lh := rec.lines[j][k][0], rec.lines[j][k][1]
				x, x0, align := cx, cx-lw/2, "center"
				switch rec.align(j, k) {
				case 'l':
					x, x0, align = fx+padding, fx+padding, "left"
				case 'r':
					x, x0, align = fx+fw-padding, fx+fw-padding-lw, "right"
				}
				code := rec.font(j, k)
				if !maths(rec, l, x, y+lh/2, align == "left", code) {
					out = append(out, text(x, y+lh/2, []any{l}, align, code))
				}
				if strings.Contains(code, "u") && lw > 0 {
					// zuzu draws no decoration of text, so the line under
					// it is a thin box in the color of the text.
					th := math.Max(fontSize/14, 1)
					d := Row{"kind": "fieldDeco", "key": r["key"], "x": x0, "y": y + lh/2 + 0.5*fontSize - th/2, "width": lw, "height": th}
					copyLink(d, r)
					if c, ok := r["textColor"]; ok {
						d["textColor"] = c
					}
					d["_deco"] = r["_deco"]
					out = append(out, d)
				}
				y += lh
			}
		default:
			if !maths(rec, t, cx, cy, false, "") {
				out = append(out, text(cx, cy, split(t), "center", ""))
			}
		}
	}
	// group writes the fields of a record in a box, and the lines between
	// the fields of a group.
	var group func(rec *record, x, y, w, h float64, spread bool)
	group = func(rec *record, x, y, w, h float64, spread bool) {
		bs := rec.fieldBoxes(x, y, w, h, spread)
		for j, b := range bs {
			sub := rec.groups[j]
			if sub == nil {
				field(rec, j, b[0], b[1], b[2], b[3])
				continue
			}
			inner := sub.fieldBoxes(b[0], b[1], b[2], b[3], true)
			for _, ib := range inner[:len(inner)-1] {
				// The line after each field of the group but the last.
				if sub.across {
					at := ib[0] + ib[2] - np.X
					path.WriteString(graph.ColumnPath([]float64{at}, b[1]-np.Y, b[1]+b[3]-np.Y))
				} else {
					at := ib[1] + ib[3] - np.Y
					path.WriteString(graph.RowPath([]float64{at}, b[0]-np.X, b[0]+b[2]-np.X))
				}
			}
			group(sub, b[0], b[1], b[2], b[3], true)
		}
	}
	group(rec, np.X-np.Width/2, np.Y-np.Height/2, np.Width, np.Height, false)
	return path.String(), out
}
