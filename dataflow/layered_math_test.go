package dataflow

import (
	"errors"
	"testing"

	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/math"
)

// newTestMath returns the labels with math of a graph, laid out with the
// font of math, with the labels of source.
func newTestMath(source ...string) *mathLabels {
	e := text.NewEstimate()
	e.SetMath(math.Font())
	ml := &mathLabels{m: e, source: map[string]bool{}, done: map[mathKey]*mathLabel{}}
	for _, s := range source {
		ml.source[s] = true
	}
	return ml
}

// TestMathLabelSize checks the size of a label with math: one row of its
// text and its formulas, with each newline a space, as high as its
// tallest formula or its line of text.
func TestMathLabelSize(t *testing.T) {
	t.Parallel()
	ml := newTestMath("$$bad$$")
	e := text.NewEstimate()
	f := text.Font{Family: text.Family, Size: 11, Weight: 400}
	for _, s := range []any{nil, "plain", "$$bad$$", "cost $5"} {
		if l, err := ml.of(s, 11, ""); l != nil || err != nil {
			t.Errorf("%q: expected no math, got %v and %v", s, l, err)
		}
	}
	l, err := ml.of("of\na whole $$\\frac{a}{b}$$", 11, "")
	if err != nil {
		t.Fatal(err)
	}
	frac, err := math.Font().Layout(`\frac{a}{b}`, 11)
	if err != nil {
		t.Fatal(err)
	}
	if w := e.Width(f, "of a whole") + 11*text.MathGap + frac.Width; abs(l.w-w) > 1e-9 || l.h != frac.Height+frac.Depth || len(l.lines) != 1 {
		t.Errorf("the label is %v by %v in %d lines, want %v by %v in one", l.w, l.h, len(l.lines), w, frac.Height+frac.Depth)
	}
	if again, _ := ml.of("of\na whole $$\\frac{a}{b}$$", 11, ""); again != l {
		t.Error("the label is laid out again")
	}
	// A field draws each line on its own, a line of source as text.
	l, err = ml.linesOf("plain\n$$x$$ n\n$$bad$$", 11, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.lines) != 3 || l.h < 3*11 || l.w != e.Width(f, "$$bad$$") {
		t.Errorf("the field is %v by %v in %d lines, want three lines as wide as $$bad$$", l.w, l.h, len(l.lines))
	}
	if l, err := ml.linesOf("plain\n$$bad$$", 11, ""); l != nil || err != nil {
		t.Errorf("a field with only source drew as math: %v, %v", l, err)
	}
	// With no font of math, a label with math is an error.
	none := &mathLabels{m: text.NewEstimate(), done: map[mathKey]*mathLabel{}}
	if _, err := none.of("$$x$$", 11, ""); !errors.Is(err, text.ErrNoMathFont) {
		t.Errorf("with no font, expected ErrNoMathFont, got %v", err)
	}
}

// TestMathLabelRows checks the rows that draw a label with math: a run of
// text at the middle of its line, and a path of each formula at its
// baseline, in the color of the label or in its own.
func TestMathLabelRows(t *testing.T) {
	t.Parallel()
	ml := newTestMath()
	l, err := ml.of(`Area $$x^2$$ and $$\color{red}{y}$$`, 11, "")
	if err != nil {
		t.Fatal(err)
	}
	rows := l.rows(Row{"key": "a", "textColor": "#123456", "href": "u"}, "node", 100, 50, false, false)
	var texts, paths []Row
	for _, r := range rows {
		switch r["kind"] {
		case "mathText":
			texts = append(texts, r)
		case "mathPath":
			paths = append(paths, r)
		}
		if r["key"] != "a" || r["part"] != "node" || r["href"] != "u" {
			t.Errorf("the row %v lost its key, its part or its link", r)
		}
	}
	if len(texts) != 2 || texts[0]["text"] != "Area" || texts[1]["text"] != "and" {
		t.Fatalf("expected the runs Area and and, got %v", texts)
	}
	left := 100 - l.w/2
	if texts[0]["x"] != left || texts[0]["y"] != 50.0 || texts[0]["fill"] != "#123456" || texts[0]["_paint"] != "fill" {
		t.Errorf("the first run at %v, %v in %v, want %v, 50 in #123456", texts[0]["x"], texts[0]["y"], texts[0]["fill"], left)
	}
	if len(paths) != 2 || paths[0]["tex"] != "x^2" || paths[0]["fill"] != "#123456" || paths[1]["fill"] != "#ff0000" {
		t.Fatalf("expected x^2 in the color of the label and y in red, got %v", paths)
	}
	p := l.lines[0].Pieces[1]
	if paths[0]["x"] != left+p.X || paths[0]["y"] != 50+p.Baseline {
		t.Errorf("x^2 at %v, %v, want %v, %v", paths[0]["x"], paths[0]["y"], left+p.X, 50+p.Baseline)
	}
	// With no color of the label, a row takes the color of its style, and
	// a title at the top left starts at its corner.
	rows = l.rows(Row{"key": "c"}, "cluster", 10, 20, true, true)
	if rows[0]["x"] != 10.0 || rows[0]["y"] != 20+l.h/2 || rows[0]["_paint"] != "" || rows[0]["fill"] != nil {
		t.Errorf("the first row of the title %v", rows[0])
	}
}

// abs returns the absolute value of a number.
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
