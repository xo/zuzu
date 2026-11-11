package dataflow

import (
	"math"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
)

// record is a node with fields, as a record of DOT, a class of UML or an
// entity (D31). The fields lie side by side when across is true, and one
// under another otherwise.
type record struct {
	across, stack bool
	texts, ports  []string
	// cells holds the cells of each field, aligns its alignment, and fonts
	// the font of each line of its text, as b, i and u.
	cells  [][]string
	aligns []string
	fonts  [][]string
	// sizes holds the size of each field along the axis of the fields, and
	// cols the width of each column of the cells, 0 for an empty column.
	sizes []float64
	cols  []float64
	// lines holds the size of each line of each field with fonts or an
	// alignment of its own, which draw one line at a time.
	lines [][][2]float64
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
	}
	return rec
}

// oneByOne reports whether the field j draws one line at a time: it has
// fonts, or it starts its lines at the left.
func (rec *record) oneByOne(j int) bool {
	return len(rec.fonts[j]) > 0 || rec.aligns[j] == "l"
}

// font returns the code of the font of the line k of the field j.
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
// in columns.
func (rec *record) size(measure measureFunc, padding float64) (float64, float64) {
	var w, h float64
	// Each column takes the width of its widest cell.
	for j, cs := range rec.cells {
		if rec.across {
			// Fields side by side have no columns, so the cells join
			// into one text.
			if len(cs) > 0 && rec.texts[j] == "" {
				rec.texts[j] = strings.Join(cs, " ")
			}
			rec.cells[j] = nil
			continue
		}
		for k, c := range cs {
			for len(rec.cols) <= k {
				rec.cols = append(rec.cols, 0)
			}
			if tw, _ := measure(c, ""); tw > 0 {
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
		var tw, th float64
		switch {
		case len(rec.cells[j]) > 0:
			tw = table - 2*padding
			for _, c := range rec.cells[j] {
				_, ch := measure(c, "")
				th = math.Max(th, ch)
			}
		case rec.oneByOne(j):
			for k, l := range strings.Split(t, "\n") {
				lw, lh := measure(l, weight(rec.font(j, k)))
				if lh == 0 {
					_, lh = measure("X", "")
				}
				// A line that draws on its own takes a gap of a fifth
				// of its height, so that a line under it stays clear of
				// the next line.
				lh *= 1.2
				rec.lines[j] = append(rec.lines[j], [2]float64{lw, lh})
				tw, th = math.Max(tw, lw), th+lh
			}
		default:
			tw, th = measure(t, "")
		}
		fw, fh := math.Max(tw+2*padding, 24), math.Max(th+1.6*padding, 24)
		if rec.stack && t == "" && len(rec.cells[j]) == 0 {
			// An empty field of a stacked record is a thin band.
			fh = padding
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

// boxes returns the box of each port of the record, as x, y, width and
// height from the center of a node of a size.
func (rec *record) boxes(w, h float64) map[string][4]float64 {
	m := map[string][4]float64{}
	at := 0.0
	for j, name := range rec.ports {
		if name != "" {
			if rec.across {
				m[name] = [4]float64{at - w/2, -h / 2, rec.sizes[j], h}
			} else {
				m[name] = [4]float64{-w / 2, at - h/2, w, rec.sizes[j]}
			}
		}
		at += rec.sizes[j]
	}
	return m
}

// rows returns the path of the record of a node and the rows that draw
// its text: a row for each field, for each line of a field that draws one
// line at a time, and for each cell, and a row for each line under a line
// of text. fontSize is the size of the font of the layout.
func (rec *record) rows(r Row, np graph.NodePlace, kind string, padding, fontSize float64) (string, []Row) {
	var path strings.Builder
	path.WriteString(graph.RecordPath(kind, np.Width, np.Height, rec.across, rec.sizes))
	var out []Row
	text := func(x, y float64, lines []any, align, code string) Row {
		f := clone(r)
		f["kind"] = "field"
		f["x"], f["y"], f["lines"], f["align"] = x, y, lines, align
		f["fontStyle"] = "normal"
		if strings.Contains(code, "i") {
			f["fontStyle"] = "italic"
		}
		if strings.Contains(code, "b") {
			f["fontWeight"] = graph.BoldWeight
		}
		graph.TextSignature(f)
		return f
	}
	split := func(t string) []any {
		lines := []any{}
		for l := range strings.SplitSeq(t, "\n") {
			lines = append(lines, l)
		}
		return lines
	}
	left, top := np.X-np.Width/2, np.Y-np.Height/2
	at := 0.0
	for j, t := range rec.texts {
		// The box of the field.
		fx, fy, fw, fh := left+at, top, rec.sizes[j], np.Height
		if !rec.across {
			fx, fy, fw, fh = left, top+at, np.Width, rec.sizes[j]
		}
		at += rec.sizes[j]
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
				if c != "" {
					out = append(out, text(x+padding, cy, split(c), "left", ""))
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
				if rec.aligns[j] == "l" {
					x, x0, align = fx+padding, fx+padding, "left"
				}
				code := rec.font(j, k)
				out = append(out, text(x, y+lh/2, []any{l}, align, code))
				if strings.Contains(code, "u") && lw > 0 {
					// zuzu draws no decoration of text, so the line under
					// it is a thin box in the color of the text.
					th := math.Max(fontSize/14, 1)
					d := Row{"kind": "fieldDeco", "key": r["key"], "x": x0, "y": y + lh/2 + 0.5*fontSize - th/2, "width": lw, "height": th}
					if c, ok := r["textColor"]; ok {
						d["textColor"] = c
					}
					d["_deco"] = r["_deco"]
					out = append(out, d)
				}
				y += lh
			}
		default:
			out = append(out, text(cx, cy, split(t), "center", ""))
		}
	}
	return path.String(), out
}
