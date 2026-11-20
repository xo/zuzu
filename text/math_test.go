package text_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/text/math"
)

// TestMathLaysOutAFormula checks that STIX Two Math lays out a formula in
// the display style, with a size and one path in the color of the text
// (D36).
func TestMathLaysOutAFormula(t *testing.T) {
	t.Parallel()
	f, err := math.Font().Layout(`x^2`, 16)
	if err != nil {
		t.Fatal(err)
	}
	if f.Width <= 0 || f.Height <= 0 || f.Depth < 0 {
		t.Errorf("x^2 measures %v by %v and %v", f.Width, f.Height, f.Depth)
	}
	if len(f.Paths) != 1 || f.Paths[0].Color != "" || !strings.HasPrefix(f.Paths[0].Path, "M") {
		t.Errorf("x^2 gives the paths %+v, want one path in the color of the text", f.Paths)
	}
	frac, err := math.Font().Layout(`\frac{a}{b}`, 16)
	if err != nil {
		t.Fatal(err)
	}
	if frac.Height+frac.Depth <= f.Height+f.Depth {
		t.Errorf("a fraction is %v high, and x^2 %v", frac.Height+frac.Depth, f.Height+f.Depth)
	}
	big, err := math.Font().Layout(`x^2`, 32)
	if err != nil {
		t.Fatal(err)
	}
	if d := big.Width - 2*f.Width; d > 0.01 || d < -0.01 {
		t.Errorf("x^2 at 32 is %v wide, want twice %v", big.Width, f.Width)
	}
}

// TestMathKeepsItsColors checks that a formula with \color gives a path for
// each color, so that each part keeps its color (D36 part 5).
func TestMathKeepsItsColors(t *testing.T) {
	t.Parallel()
	f, err := math.Font().Layout(`a + \color{red}{b} + c`, 16)
	if err != nil {
		t.Fatal(err)
	}
	var colors []string
	for _, p := range f.Paths {
		colors = append(colors, p.Color)
	}
	if !slices.Contains(colors, "#ff0000") || !slices.Contains(colors, "") {
		t.Errorf("the colors %q, want red and the color of the text", colors)
	}
}

// TestMathNamesTheUnknownCommand checks that a formula with a command that
// the layout does not know is an error that names the command.
func TestMathNamesTheUnknownCommand(t *testing.T) {
	t.Parallel()
	_, err := math.Font().Layout(`\nosuchcommand{x}`, 16)
	if !errors.Is(err, okage.ErrTeXMathSyntax) {
		t.Fatalf("got %v, want an error of the syntax", err)
	}
	if c := text.MathCommand(err); c != `\nosuchcommand` {
		t.Errorf("the command %q, want \\nosuchcommand", c)
	}
}

// TestMathIsSafeForConcurrentUse lays out formulas from many goroutines on
// one font, which shares one face (D36 part 10).
func TestMathIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	want, err := math.Font().Layout(`\sqrt{x+3}`, 14)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				got, err := math.Font().Layout(`\sqrt{x+3}`, 14)
				if err != nil || got.Paths[0].Path != want.Paths[0].Path {
					t.Error("a formula laid out at the same time differs")
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestSplitMath splits a line into runs of text and of math, as Mermaid
// 12.3 finds math between two $$.
func TestSplitMath(t *testing.T) {
	t.Parallel()
	for s, want := range map[string][]text.Run{
		"plain":                {{Text: "plain"}},
		"$$x^2$$":              {{Text: "x^2", Math: true}},
		"Area $$\\pi r^2$$ ok": {{Text: "Area "}, {Text: "\\pi r^2", Math: true}, {Text: " ok"}},
		"$$a$$ and $$b$$":      {{Text: "a", Math: true}, {Text: " and "}, {Text: "b", Math: true}},
		"cost $$5":             {{Text: "cost $$5"}},
	} {
		if got := text.SplitMath(s); !slices.Equal(got, want) {
			t.Errorf("%q splits into %+v, want %+v", s, got, want)
		}
	}
}

// TestMathOf checks that an Estimate gives the font of math of SetMath.
func TestMathOf(t *testing.T) {
	t.Parallel()
	e := text.NewEstimate()
	if text.MathOf(e) != nil {
		t.Error("an Estimate with no font of math gives one")
	}
	e.SetMath(math.Font())
	if text.MathOf(e) != math.Font() {
		t.Error("an Estimate does not give its font of math")
	}
}

// TestLayMathLine checks the row of a line with math as Mermaid 12.3 lays
// it out: the spaces at the ends of a run of text drop, and a line of
// math alone is as high as its formula.
func TestLayMathLine(t *testing.T) {
	t.Parallel()
	e := text.NewEstimate()
	e.SetMath(math.Font())
	f := text.Font{Family: "IBM Plex Sans", Size: 16, Weight: 400}
	spaced, err := text.LayMathLine(e, f, text.SplitMath("area   $$\\pi r^2$$   here"), 24)
	if err != nil {
		t.Fatal(err)
	}
	tight, err := text.LayMathLine(e, f, text.SplitMath("area$$\\pi r^2$$here"), 24)
	if err != nil {
		t.Fatal(err)
	}
	if spaced.Width != tight.Width || len(spaced.Pieces) != 3 || spaced.Pieces[0].Text != "area" {
		t.Errorf("the spaced line is %v wide with %+v, and the tight one %v", spaced.Width, spaced.Pieces, tight.Width)
	}
	if spaced.Height != 24 {
		t.Errorf("a line of text and math is %v high, want the line of text, 24", spaced.Height)
	}
	alone, err := text.LayMathLine(e, f, text.SplitMath("$$x^2$$"), 24)
	if err != nil {
		t.Fatal(err)
	}
	fm := alone.Pieces[0].Formula
	if alone.Height != fm.Height+fm.Depth || alone.Height >= 24 {
		t.Errorf("a line of x^2 alone is %v high, want its formula, %v", alone.Height, fm.Height+fm.Depth)
	}
	if _, err := text.LayMathLine(text.NewEstimate(), f, text.SplitMath("$$x$$"), 24); !errors.Is(err, text.ErrNoMathFont) {
		t.Errorf("a line of math with no font gives %v, want ErrNoMathFont", err)
	}
}
