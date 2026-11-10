package text

import (
	"encoding/json/v2"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	okagesvg "github.com/xo/okage/svg"
)

// label is a text of a target, with its font.
type label struct {
	text string
	font Font
}

// labels returns the texts of the tier 0 targets in the light mode, which
// test/cmd/targets copies into the golden tests of svg.
func labels(t *testing.T) []label {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "svg", "testdata", "targets", "*-light.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("finding the scene graphs of the targets: %v", err)
	}
	seen := map[label]bool{}
	var out []label
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if s, ok := x["text"].(string); ok {
				size, _ := x["fontSize"].(float64)
				weight, _ := x["fontWeight"].(string)
				l := label{s, Font{Family: Family, Size: size, Weight: Weight(weight)}}
				if size > 0 && !seen[l] {
					seen[l] = true
					out = append(out, l)
				}
			}
			for _, e := range x {
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		walk(v)
	}
	return out
}

// okageDB returns a font database of okage that holds the bundled fonts.
func okageDB(t *testing.T) *okagesvg.FontDatabase {
	t.Helper()
	db := okagesvg.NewFontDatabase()
	err := fs.WalkDir(Fonts(), "fonts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".ttf") {
			return err
		}
		b, err := fs.ReadFile(Fonts(), path)
		if err != nil {
			return err
		}
		if len(db.LoadFontData(b)) == 0 {
			t.Errorf("okage loaded no face from %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// okageWidth measures a string as okage lays it out: the position of a
// marker drawn after the string is the width of the string.
func okageWidth(t *testing.T, db *okagesvg.FontDatabase, l label) float64 {
	t.Helper()
	src := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="4000" height="100">`+
		`<text x="0" y="50" font-family="%s" font-size="%g" font-weight="%d">%s|</text></svg>`,
		Family, l.font.Size, l.font.Weight, strings.NewReplacer("&", "&amp;", "<", "&lt;").Replace(l.text))
	tree, err := okagesvg.FromData([]byte(src), okagesvg.WithFontDatabase(db))
	if err != nil {
		t.Fatalf("parsing %q in okage: %v", l.text, err)
	}
	for _, n := range tree.Root().Children() {
		txt, ok := n.(*okagesvg.Text)
		if !ok {
			continue
		}
		spans := txt.Layouted()
		if len(spans) == 0 || len(spans[len(spans)-1].PositionedGlyphs) == 0 {
			t.Fatalf("okage laid out no glyph for %q", l.text)
		}
		glyphs := spans[len(spans)-1].PositionedGlyphs
		return float64(glyphs[len(glyphs)-1].Transform().TX)
	}
	t.Fatalf("okage made no text node for %q", l.text)
	return 0
}

// TestEstimateIsCloseToOkage measures every label of the tier 0 targets
// with Estimate and with the text layout of okage, which shapes and kerns.
// The exit of phase 2 holds the estimate within a stated tolerance: one
// pixel, or two percent of the width, whichever is more.
func TestEstimateIsCloseToOkage(t *testing.T) {
	t.Parallel()
	db := okageDB(t)
	e := NewEstimate()
	var worst, sum float64
	ls := labels(t)
	if len(ls) < 50 {
		t.Fatalf("found %d labels in the targets, expected many more", len(ls))
	}
	for _, l := range ls {
		got, want := e.Width(l.font, l.text), okageWidth(t, db, l)
		d := math.Abs(got - want)
		sum += d
		worst = max(worst, d)
		if d > math.Max(1, 0.02*want) {
			t.Errorf("%q at %g, weight %d: Estimate gives %.2f, okage gives %.2f",
				l.text, l.font.Size, l.font.Weight, got, want)
		}
	}
	t.Logf("%d labels: mean difference %.3f px, worst %.3f px", len(ls), sum/float64(len(ls)), worst)
}

// TestTruncateCutsToTheLimit checks that a cut string fits its limit, ends
// in the ellipsis, and that a string that fits is not cut.
func TestTruncateCutsToTheLimit(t *testing.T) {
	t.Parallel()
	e := NewEstimate()
	f := Font{Family: Family, Size: 11, Weight: 400}
	s := "Monthly closing price in US dollars"
	if got := Truncate(e, f, s, 1000); got != s {
		t.Errorf("a string that fits came back as %q", got)
	}
	for _, limit := range []float64{20, 60, 120} {
		got := Truncate(e, f, s, limit)
		if !strings.HasSuffix(got, Ellipsis) || e.Width(f, got) > limit {
			t.Errorf("limit %g: got %q, %.2f wide", limit, got, e.Width(f, got))
		}
	}
	if got := Truncate(e, f, s, 1); got != "" {
		t.Errorf("a limit narrower than the ellipsis gave %q", got)
	}
}

// TestWeightReadsEachForm reads the weights that Vega writes.
func TestWeightReadsEachForm(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"": 400, "normal": 400, "bold": 700, "600": 600, "x": 400} {
		if got := Weight(in); got != want {
			t.Errorf("Weight(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestTheFontsHoldTheirLicense checks that the license travels with the
// fonts, as the Open Font License asks (D16).
func TestTheFontsHoldTheirLicense(t *testing.T) {
	t.Parallel()
	b, err := fs.ReadFile(Fonts(), "fonts/LICENSE.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "SIL Open Font License") {
		t.Error("fonts/LICENSE.txt is not the Open Font License")
	}
}

// TestEstimateIsSafeForConcurrentUse measures from many goroutines at once,
// for the race detector.
func TestEstimateIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	e := NewEstimate()
	f := Font{Family: Family, Size: 11, Weight: 400}
	want := e.Width(f, "zuzu")
	done := make(chan float64)
	for i := range 8 {
		go func() { done <- e.Width(f, "zuzu"+strconv.Itoa(i)) - e.Width(f, strconv.Itoa(i)) }()
	}
	for range 8 {
		if got := <-done; math.Abs(got-want) > 1e-9 {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
