package dataflow

import (
	"math"
	"strings"

	"github.com/xo/okage/html"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
)

// htmlLabel is a label of HTML of a node, laid out by the package html of
// okage (D31): the label, its layout, and the margin around it.
type htmlLabel struct {
	label  *html.Label
	lay    *html.Layout
	mx, my float64
	// grads holds the gradient of each table and each cell, by the
	// *html.Table or the *html.Cell, as gradientangle and style=radial
	// set it.
	grads map[any]graph.HTMLGradient
}

// sizeFunc measures a text in a font, a size and a weight.
type sizeFunc func(text, font string, size float64, weight string) (float64, float64)

// htmlOf lays out the label of HTML of a row of a node, or returns false
// for none. The parser of DOT already reported the parts that the label
// does not draw, so they are not reported again.
func htmlOf(r Row, size sizeFunc, fontSize float64) (*htmlLabel, bool, error) {
	src, ok := r["html"].(string)
	if !ok || src == "" {
		return nil, false, nil
	}
	if v := expr.Number(r["fontSize"]); v > 0 {
		fontSize = v
	}
	h, err := layHTML(src, size, fontSize)
	if err != nil {
		return nil, false, err
	}
	if ms, ok := r["margin"].([]any); ok && len(ms) == 2 {
		h.mx, h.my = expr.Number(ms[0]), expr.Number(ms[1])
	}
	return h, true, nil
}

// layHTML lays out a label of HTML in a size of font.
func layHTML(src string, size sizeFunc, fontSize float64) (*htmlLabel, error) {
	src, tables, cells := graph.HTMLGradients(src)
	l, _, err := html.Parse(src)
	if err != nil {
		return nil, err
	}
	grads := map[any]graph.HTMLGradient{}
	// The tables and the cells come in the order of the source, as a walk
	// of the tree finds them.
	var walk func(t *html.Table)
	walk = func(t *html.Table) {
		if len(tables) > 0 {
			grads[t], tables = tables[0], tables[1:]
		}
		for _, r := range t.Rows {
			for _, c := range r.Cells {
				if len(cells) > 0 {
					grads[c], cells = cells[0], cells[1:]
				}
				if c.Table != nil {
					walk(c.Table)
				}
			}
		}
	}
	if l.Table != nil {
		walk(l.Table)
	}
	m := func(text string, st html.TextStyle) (html.Metrics, error) {
		weight := ""
		if st.Bold {
			weight = "bold"
		}
		w, h := size(text, st.Family, st.Size, weight)
		return html.Metrics{Width: w, Height: h, Ascent: 0.8 * h}, nil
	}
	lay, err := l.Layout(html.WithMeasure(m), html.WithTextStyle(html.TextStyle{Size: fontSize}))
	if err != nil {
		return nil, err
	}
	return &htmlLabel{label: l, lay: lay, mx: 8, my: 4, grads: grads}, nil
}

// size returns the size of a node of a shape around the label, with the
// margin around a node with no shape, and the padding of a shape around
// another.
func (h *htmlLabel) size(kind string, padding float64) (float64, float64) {
	if kind == "none" {
		return h.lay.Width + 2*h.mx, h.lay.Height + 2*h.my
	}
	return graph.NodeSize(kind, h.lay.Width, h.lay.Height, padding)
}

// ports returns the box of each port of the label, as x, y, width and
// height from the center of the node.
func (h *htmlLabel) ports() map[string][4]float64 {
	out := map[string][4]float64{}
	for name, b := range h.lay.Ports {
		out[name] = [4]float64{b.X - h.lay.Width/2, b.Y - h.lay.Height/2, b.Width, b.Height}
	}
	return out
}

// lineDash returns the dash of a line style.
func lineDash(s html.LineStyle) []any {
	switch s {
	case html.LineDashed:
		return []any{5.0, 4.0}
	case html.LineDotted:
		return []any{1.0, 3.0}
	}
	return []any{}
}

// paintSig sets the signature of the keys of a list that a row holds, in
// the field _paint, as the compile of a graph lists them.
func paintSig(o Row, keys ...string) {
	var set []string
	for _, k := range keys {
		if _, ok := o[k]; ok {
			set = append(set, k)
		}
	}
	o["_paint"] = strings.Join(set, ",")
}

// colorOf returns a raw color of a label as CSS, or false for an empty or
// unknown one.
func colorOf(v string) (string, bool) {
	if v == "" {
		return "", false
	}
	return graph.Color(v)
}

// gradient returns the gradient of the fill of a box, or false for a fill
// of one color.
func (h *htmlLabel) gradient(b html.Box) (map[string]any, bool) {
	if !strings.Contains(b.Fill, ":") {
		return nil, false
	}
	var g graph.HTMLGradient
	if b.Cell != nil {
		g = h.grads[b.Cell]
	} else if b.Table != nil {
		g = h.grads[b.Table]
	}
	return graph.Gradient(b.Fill, g.Angle, g.Radial)
}

// rows writes the rows that draw the label of a node at its center: its
// boxes, its rules, the lines under, over and through its text, and its
// runs of text. A node with no shape fills its outer table with the fill
// of a node of the theme, and the stroke and the font color of the node
// go to the borders and the text that set none.
func (h *htmlLabel) rows(r Row, x, y float64, kind string) []Row {
	x0, y0 := x-h.lay.Width/2, y-h.lay.Height/2
	stroke, _ := r["stroke"].(string)
	text, _ := r["textColor"].(string)
	var out []Row
	for _, b := range h.lay.Boxes {
		o := Row{"kind": "htmlBox", "key": r["key"]}
		sw := b.StrokeWidth
		o["x"], o["y"] = x0+b.X+sw/2, y0+b.Y+sw/2
		o["width"], o["height"] = math.Max(b.Width-sw, 0), math.Max(b.Height-sw, 0)
		o["strokeWidth"], o["strokeDash"] = sw, lineDash(b.Style.Line)
		o["cornerRadius"] = 0.0
		if b.Style.Rounded {
			o["cornerRadius"] = 6.0
		}
		if g, ok := h.gradient(b); ok {
			o["fill"] = g
		} else if c, ok := colorOf(b.Fill); ok {
			o["fill"] = c
		} else if b.Table != h.label.Table || kind != "none" {
			o["fill"] = "transparent"
		}
		switch c, ok := colorOf(b.Stroke); {
		case sw <= 0:
			o["stroke"] = "transparent"
		case ok:
			o["stroke"] = c
		case stroke != "":
			o["stroke"] = stroke
		}
		paintSig(o, "fill", "stroke")
		out = append(out, o)
	}
	for _, ru := range h.lay.Rules {
		o := Row{"kind": "htmlRule", "key": r["key"]}
		o["x"], o["y"], o["x2"], o["y2"] = x0+ru.X1, y0+ru.Y1, x0+ru.X2, y0+ru.Y2
		o["strokeWidth"], o["strokeDash"] = ru.Width, lineDash(ru.Line)
		if c, ok := colorOf(ru.Stroke); ok {
			o["stroke"] = c
		} else if stroke != "" {
			o["stroke"] = stroke
		}
		paintSig(o, "stroke")
		out = append(out, o)
	}
	for _, run := range h.lay.Runs {
		color, ok := colorOf(run.Color)
		if !ok {
			color = graph.Contrast(run.Under)
		}
		if color == "" {
			color = text
		}
		size := run.Style.Size
		mid := y0 + run.Top + run.Height/2 - run.Shift
		o := Row{"kind": "htmlRun", "key": r["key"], "x": x0 + run.X, "y": mid, "text": keepSpaces(run.Text), "fontSize": size}
		o["fontWeight"], o["fontStyle"] = "normal", "normal"
		if run.Style.Bold {
			o["fontWeight"] = "bold"
		}
		if run.Style.Italic {
			o["fontStyle"] = "italic"
		}
		if color != "" {
			o["fill"] = color
		}
		if run.Style.Family != "" {
			o["font"] = run.Style.Family
		}
		paintSig(o, "fill", "font")
		out = append(out, o)
		// zuzu draws no decoration of text, so a line under, over or
		// through a run is a thin box in the color of the text.
		for _, d := range []struct {
			on bool
			at float64
		}{{run.Underline, 0.5}, {run.Strike, 0.05}, {run.Overline, -0.55}} {
			if !d.on {
				continue
			}
			t := math.Max(size/14, 1)
			o := Row{"kind": "htmlDeco", "key": r["key"], "x": x0 + run.X, "y": mid + d.at*size - t/2, "width": run.Width, "height": t}
			if color != "" {
				o["fill"] = color
			}
			paintSig(o, "fill")
			out = append(out, o)
		}
	}
	for _, o := range out {
		copyLink(o, r)
	}
	return out
}

// copyLink copies the link and the tooltip of a node, an edge or a cluster
// into a row that draws a part of it (D32).
func copyLink(dst, src Row) {
	for _, k := range []string{"href", "tooltip"} {
		if v, ok := src[k]; ok && v != nil {
			dst[k] = v
		}
	}
}

// keepSpaces turns the spaces at the ends of a run, and each space after
// another, into no-break spaces, which SVG does not collapse. The run keeps
// the width that the layout measured.
func keepSpaces(s string) string {
	if !strings.Contains(s, " ") {
		return s
	}
	r := []rune(s)
	for i := range r {
		if r[i] != ' ' {
			continue
		}
		if i == 0 || i == len(r)-1 || r[i-1] == ' ' || r[i-1] == '\u00a0' {
			r[i] = '\u00a0'
		}
	}
	return string(r)
}

// compass returns the place of a side of a box, as x and y from the
// center of the node, and the direction out of the box at it, or false for
// a word that is not a side. The center, c or _, has no direction.
func compass(side string, b [4]float64) ([]float64, []float64, bool) {
	l, t, w, h := b[0], b[1], b[2], b[3]
	cx, cy := l+w/2, t+h/2
	d := 1 / math.Sqrt2
	switch strings.ToLower(side) {
	case "n":
		return []float64{cx, t}, []float64{0, -1}, true
	case "ne":
		return []float64{l + w, t}, []float64{d, -d}, true
	case "e":
		return []float64{l + w, cy}, []float64{1, 0}, true
	case "se":
		return []float64{l + w, t + h}, []float64{d, d}, true
	case "s":
		return []float64{cx, t + h}, []float64{0, 1}, true
	case "sw":
		return []float64{l, t + h}, []float64{-d, d}, true
	case "w":
		return []float64{l, cy}, []float64{-1, 0}, true
	case "nw":
		return []float64{l, t}, []float64{-d, -d}, true
	case "c", "_":
		return []float64{cx, cy}, nil, true
	}
	return nil, nil, false
}
