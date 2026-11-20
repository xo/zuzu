package mermaid

import (
	"strings"
	"unicode"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// The kinds of Mermaid that lay out when they compile, the sequence
// diagram, the mindmap and the block diagram, draw the math of KaTeX in
// their labels with the font of zuzu.WithMath (D36). This file holds what
// the three kinds share: the check of a label, the layout of a line of a
// label with math, and the marks that draw the formulas.

// checkMath reports a label of a chart with math of KaTeX that does not
// draw as math: each label when the parse has no font of math, as
// mathIssue, and a label with a formula that does not lay out, with the
// command that stops it. The caller sets the line of the report.
func (c *chart) checkMath(label string) error {
	if !text.HasMath(label) {
		return nil
	}
	if c.d.Math == nil {
		return c.add(mathIssue, label)
	}
	if format, args := labelMathIssue(c.d.Math, label); format != "" {
		return c.add(format, args...)
	}
	return nil
}

// mathKey is a line of a label in a font, with the height of a line of
// text.
type mathKey struct {
	f  text.Font
	s  string
	lh float64
}

// mathLines lays out the lines of the labels of a chart with math, and
// keeps each line, so that a layout that measures a label more than once
// lays out each formula once.
type mathLines struct {
	m    text.Measurer
	done map[mathKey]*text.MathLine
}

// newMathLines returns the lines with math of a measurer.
func newMathLines(m text.Measurer) *mathLines {
	return &mathLines{m: m, done: map[mathKey]*text.MathLine{}}
}

// line lays out a line of a label in a font, with a line of text lh high.
// It returns nil when the line draws as text: it holds no math, the
// measurer has no font of math, or a formula does not lay out, which
// checkMath reported.
func (ml *mathLines) line(f text.Font, s string, lh float64) *text.MathLine {
	if text.MathOf(ml.m) == nil || !text.HasMath(s) {
		return nil
	}
	k := mathKey{f, s, lh}
	if l, ok := ml.done[k]; ok {
		return l
	}
	l, err := text.LayMathLine(ml.m, f, text.SplitMath(s), lh)
	if err != nil {
		l = nil
	}
	ml.done[k] = l
	return l
}

// mathText is a run of text of a line of a label with math, as it draws:
// its text with no space at its ends, and the x where that text starts.
type mathText struct {
	x float64
	s string
}

// mathPieces returns the runs of text and the rows of the formulas of a
// line of a label with math, for a line that starts at x and has its
// middle at y. A run of text loses the spaces at its ends, and starts
// after the width of the spaces at its start, so that it draws where the
// layout put it. Each formula gives a row for each of its paths, at the
// origin of the formula on its baseline, with its TeX as its text, and the
// color of the path as its fill when the path has its own color.
func mathPieces(m text.Measurer, f text.Font, l *text.MathLine, x, y float64) ([]mathText, []map[string]any) {
	var texts []mathText
	var rows []map[string]any
	for _, p := range l.Pieces {
		if p.Formula == nil {
			s := strings.TrimRightFunc(p.Text, unicode.IsSpace)
			lead := s[:len(s)-len(strings.TrimLeftFunc(s, unicode.IsSpace))]
			if s = s[len(lead):]; s == "" {
				continue
			}
			lx := x + p.X
			if lead != "" {
				lx += m.Width(f, lead)
			}
			texts = append(texts, mathText{x: lx, s: s})
			continue
		}
		for _, mp := range p.Formula.Paths {
			row := map[string]any{"x": x + p.X, "y": y + p.Baseline, "path": mp.Path, "tex": p.Text}
			if mp.Color != "" {
				row["fill"] = mp.Color
			}
			rows = append(rows, row)
		}
	}
	return texts, rows
}

// mathMarks returns the marks of type path that draw the formulas of a
// dataset of rows of mathPieces, in a style of a label, which gives the
// color of the text of the label. A row with a fill of its own draws in
// it. The dataset of the rows must be in data. Each mark has the role
// math, and each of its items holds the TeX of its formula as its text,
// which the SVG writes as its label (D36).
func mathMarks(data *[]any, name string, rows []any, style string) []any {
	field := func(f string) map[string]any { return map[string]any{"field": f} }
	return sigMarks(data, name, rows, []string{"fill"}, nil, func() map[string]any {
		return map[string]any{"type": "path", "role": scene.RoleMath, "style": []any{style}, "encode": map[string]any{"update": map[string]any{
			"x": field("x"), "y": field("y"), "path": field("path"), "text": field("tex"),
			"stroke": map[string]any{"value": nil},
		}}}
	})
}

// mathSource returns the source of a line of a label with math: its runs
// of text, and its formulas between two $$.
func mathSource(l *text.MathLine) string {
	var b strings.Builder
	for _, p := range l.Pieces {
		if p.Math {
			b.WriteString("$$" + p.Text + "$$")
			continue
		}
		b.WriteString(p.Text)
	}
	return b.String()
}
