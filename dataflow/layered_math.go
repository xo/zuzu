package dataflow

import (
	"fmt"
	"math"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/text"
)

// MeasureEnv is an Env that gives the Measurer of the render. The
// transform zuzu.layered lays out the math of KaTeX in a label with it,
// in the font of math that text.MathOf gives (D36).
type MeasureEnv interface {
	Measurer() text.Measurer
}

// mathLabel is a label with math of KaTeX laid out in a size and a weight
// of font: its lines, one under another, each a row of its runs of text
// and its formulas, and its width and its height.
type mathLabel struct {
	lines  []*text.MathLine
	w, h   float64
	size   float64
	weight string
}

// mathKey is a label with math in a size and a weight of font, laid out
// as one row or as lines.
type mathKey struct {
	s      string
	size   float64
	weight string
	lines  bool
}

// mathLabels lays out the labels with math of a graph, once each. It is
// nil for a graph with no math.
type mathLabels struct {
	m      text.Measurer
	source map[string]bool
	done   map[mathKey]*mathLabel
}

// newMathLabels reads the parameters math and mathSource of the transform
// zuzu.layered, which graph.Compile writes. It returns nil when math is
// not true.
func newMathLabels(p *params) (*mathLabels, error) {
	on, err := p.boolean("math", false)
	if err != nil || !on {
		return nil, err
	}
	ml := &mathLabels{source: map[string]bool{}, done: map[mathKey]*mathLabel{}}
	if me, ok := p.env.(MeasureEnv); ok {
		ml.m = me.Measurer()
	}
	src, err := p.strings("mathSource")
	if err != nil {
		return nil, err
	}
	for _, s := range src {
		ml.source[s] = true
	}
	return ml, nil
}

// draws reports whether a label draws as math: it holds math, and
// MathSource does not name it.
func (ml *mathLabels) draws(s string) bool {
	return text.HasMath(s) && !ml.source[s]
}

// of lays out a label of a node, an edge or a cluster in a size and a
// weight of font, or returns nil for a label that draws as text. Mermaid
// 12.3 draws such a label with math as one row, so each newline of it is
// a space.
func (ml *mathLabels) of(v any, size float64, weight string) (*mathLabel, error) {
	if ml == nil || v == nil || expr.IsUndefined(v) {
		return nil, nil
	}
	s := expr.String(v)
	if !ml.draws(s) {
		return nil, nil
	}
	return ml.lay(mathKey{s, size, weight, false})
}

// linesOf lays out a text of a field of a record, such as the members of
// a class, or returns nil for a text that draws as text. Mermaid 12.3
// draws each line of a field as a label of its own, so each line draws as
// math or as text on its own. A line that MathSource names draws as text.
func (ml *mathLabels) linesOf(v any, size float64, weight string) (*mathLabel, error) {
	if ml == nil || v == nil || expr.IsUndefined(v) {
		return nil, nil
	}
	s := expr.String(v)
	for l := range strings.SplitSeq(s, "\n") {
		if ml.draws(l) {
			return ml.lay(mathKey{s, size, weight, true})
		}
	}
	return nil, nil
}

// lay lays out a label once.
func (ml *mathLabels) lay(k mathKey) (*mathLabel, error) {
	if l, ok := ml.done[k]; ok {
		return l, nil
	}
	if ml.m == nil {
		return nil, fmt.Errorf("the math of the label %q: %w", k.s, text.ErrNoMathFont)
	}
	f := text.Font{Family: text.Family, Size: k.size, Weight: text.Weight(k.weight)}
	l := &mathLabel{size: k.size, weight: k.weight}
	// A label of one row is a row of HTML in Mermaid, where a newline is
	// a space.
	rows := []string{strings.ReplaceAll(k.s, "\n", " ")}
	if k.lines {
		rows = strings.Split(k.s, "\n")
	}
	for _, r := range rows {
		// A line of a field with no math, or with math that draws its
		// source, draws as a line of text, with its spaces, as high as a
		// line of text of zuzu, which is as high as its font.
		piece := text.Piece{Text: r, W: ml.m.Width(f, r)}
		line := &text.MathLine{Pieces: []text.Piece{piece}, Width: piece.W, Height: k.size}
		if !k.lines || ml.draws(r) {
			var err error
			if line, err = text.LayMathLine(ml.m, f, text.SplitMath(r), k.size); err != nil {
				return nil, fmt.Errorf("the math of the label %q: %w", k.s, err)
			}
		}
		l.lines = append(l.lines, line)
		l.w, l.h = max(l.w, line.Width), l.h+line.Height
	}
	ml.done[k] = l
	return l, nil
}

// rows writes the rows that draw the label: a row of kind mathText for
// each run of text, and a row of kind mathPath for each path of each
// formula, both with the field part, which names the style of the label.
// x is the center of the label, or its left when left is true, and y its
// middle, or its top when top is true. Each line is at the center of the
// label, or at its left. base holds the key of the row, its link, and the
// color of the text, the weight and the style of the font of the label,
// when they are set. The weight of the layout wins.
func (l *mathLabel) rows(base Row, part string, x, y float64, left, top bool) []Row {
	if !top {
		y -= l.h / 2
	}
	color, _ := base["textColor"].(string)
	weight, style := l.weight, "normal"
	if weight == "" {
		weight = "normal"
		if w, ok := base["fontWeight"].(string); ok && w != "" {
			weight = w
		}
	}
	if s, ok := base["fontStyle"].(string); ok && s != "" {
		style = s
	}
	paint := func(o Row, c string) Row {
		o["_paint"] = ""
		if c != "" {
			o["fill"], o["_paint"] = c, "fill"
		}
		copyLink(o, base)
		return o
	}
	var out []Row
	for _, line := range l.lines {
		x0, mid := x, y+line.Height/2
		if !left {
			x0 -= line.Width / 2
		}
		y += line.Height
		for _, p := range line.Pieces {
			if p.Formula == nil {
				if strings.TrimSpace(p.Text) == "" {
					continue
				}
				out = append(out, paint(Row{
					"kind": "mathText", "part": part, "key": base["key"], "x": x0 + p.X, "y": mid, "text": keepSpaces(p.Text),
					"fontSize": l.size, "fontWeight": weight, "fontStyle": style,
				}, color))
				continue
			}
			for _, mp := range p.Formula.Paths {
				c := mp.Color
				if c == "" {
					c = color
				}
				out = append(out, paint(Row{
					"kind": "mathPath", "part": part, "key": base["key"], "x": x0 + p.X, "y": mid + p.Baseline,
					"path": mp.Path, "tex": p.Text,
				}, c))
			}
		}
	}
	return out
}

// turnRows turns the rows of a label with math by angle degrees around the
// point x, y. Each row moves around the point and takes the field angle,
// so that its run of text or its path turns around its own x and y. The
// label then lies where a text at x, y with that angle lies. An angle of 0
// leaves the rows as they are.
func turnRows(rows []Row, x, y, angle float64) {
	if angle == 0 {
		return
	}
	sin, cos := math.Sincos(angle * math.Pi / 180)
	if math.Mod(angle, 90) == 0 {
		// A turn by a multiple of 90 degrees moves a row to a place that
		// is exact.
		sin, cos = math.Round(sin), math.Round(cos)
	}
	for _, r := range rows {
		dx, dy := expr.Number(r["x"])-x, expr.Number(r["y"])-y
		r["x"], r["y"], r["angle"] = x+dx*cos-dy*sin, y+dx*sin+dy*cos, angle
	}
}

// mathBase returns the base of the rows of a label with math of a row: its
// key, its link, the color of a field of it, or else its color of text,
// and its weight and its style of font.
func mathBase(r Row, color string) Row {
	b := Row{"key": r["key"]}
	if c, ok := r[color]; ok && c != nil {
		b["textColor"] = c
	} else if c, ok := r["textColor"]; ok && c != nil {
		b["textColor"] = c
	}
	for _, k := range []string{"fontWeight", "fontStyle"} {
		if v, ok := r[k]; ok && v != nil {
			b[k] = v
		}
	}
	copyLink(b, r)
	return b
}
