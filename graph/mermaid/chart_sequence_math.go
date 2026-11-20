package mermaid

import (
	"math"

	"github.com/xo/zuzu/text"
)

// textMiddle returns how far the middle of the first line of a text of n
// lines, of a size and a line height, is from the y of the text, for a
// baseline of Vega, as the writers place it: the middle of a line is
// where a line with the baseline middle has its y.
func textMiddle(baseline string, n int, size, lineH float64) float64 {
	// The offsets of the baselines from the alphabetic baseline, as
	// fractions of the size, and how far up the first line of more lines
	// moves, in lines, as internal/shape has them.
	off, up := -0.21, 1.0
	switch baseline {
	case "top":
		off, up = 0.79, 0
	case "middle":
		off, up = 0.3, 0.5
	}
	return (off-0.3)*size - lineH*float64(n-1)*up
}

// mathRow adds the rows of a label with math to a set: a row of text for
// each line with no math and for each run of text of a line with math,
// and a row in the set of the set name and Math for each formula. The
// label takes the place that its lines of text take with the align and
// the baseline, and grows from the top for the baseline top, from the
// bottom for bottom, and from the middle for middle, by the height that
// its lines with math add (D36).
func (l *seqLayout) mathRow(set, kind string, lines []string, f text.Font, x, y float64, align, baseline string) {
	lineH := lh(f)
	n := len(lines)
	top := y + textMiddle(baseline, n, f.Size, lineH) - lineH/2
	_, h := l.size(f, lines)
	switch baseline {
	case "top":
	case "middle":
		top += (float64(n)*lineH - h) / 2
	default:
		top += float64(n)*lineH - h
	}
	for _, s := range lines {
		ml := l.mathLine(f, s)
		if ml == nil {
			l.plainRow(set, kind, []string{s}, f, x, top+lineH/2, align, "middle")
			top += lineH
			continue
		}
		mid := top + ml.Height/2
		x0 := x - ml.Width/2
		switch align {
		case "left":
			x0 = x
		case "right":
			x0 = x - ml.Width
		}
		texts, paths := mathPieces(l.m, f, ml, x0, mid)
		for _, t := range texts {
			l.plainRow(set, kind, []string{t.s}, f, t.x, mid, "left", "middle")
		}
		for _, p := range paths {
			p["kind"] = kind
			l.rows[set+"Math"] = append(l.rows[set+"Math"], p)
		}
		l.extent(x0, x0+ml.Width)
		top += ml.Height
		l.maxY = math.Max(l.maxY, top)
	}
}
