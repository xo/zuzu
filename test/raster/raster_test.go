// Package raster checks the direct renderer of zuzu against okage on the
// corpus, with the text (D11).
package raster

import (
	"bytes"
	"image"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/raster"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/test/conform"
	"github.com/xo/zuzu/text"
)

// TestCorpusMatchesOkage draws each spec of the first target directly and
// through SVG and okage, with the bundled fonts, and checks that at most 1
// percent of the pixels differ, the gate of okage D31 for a file with text.
func TestCorpusMatchesOkage(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the corpus takes a while")
	}
	paths, err := filepath.Glob(filepath.Join("..", "testdata", "references", "*", "*.ref.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no references: run the oracle first")
	}
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
	corpus := filepath.Join("..", "testdata", "corpus")
	for _, p := range paths {
		ref, err := conform.Read(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(ref.Spec, func(t *testing.T) {
			t.Parallel()
			src := []byte(ref.Compiled)
			if len(src) == 0 {
				if src, err = os.ReadFile(filepath.Join(corpus, ref.Spec)); err != nil {
					t.Fatal(err)
				}
			}
			chart, _, err := zuzu.Parse(src, zuzu.WithDialect(zuzu.Vega))
			if err != nil {
				t.Skip(err)
			}
			s, _, err := chart.Render(t.Context(),
				zuzu.WithLoader(zuzu.FSLoader(os.DirFS(corpus))),
				zuzu.WithLocation(time.UTC),
				zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }))
			if err != nil {
				t.Skip(err)
			}
			var b bytes.Buffer
			if err := svg.Write(&b, s); err != nil {
				t.Fatal(err)
			}
			want, err := r.Render(b.Bytes())
			if err != nil {
				t.Fatalf("okage: %v", err)
			}
			got, err := raster.Draw(s, raster.WithRenderer(r))
			if err != nil {
				t.Fatal(err)
			}
			if d := differ(got, want); d > 0.01 {
				t.Errorf("%.2f%% of the pixels differ, more than 1%%", 100*d)
			}
		})
	}
}

// differ returns the share of the pixels that differ, by the metric of
// okage D31.
func differ(a, b *image.RGBA) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
	if w == 0 || h == 0 {
		return 0
	}
	n := 0
	for y := range h {
		for x := range w {
			p := image.Pt(x, y)
			if !p.In(ab) || !p.In(bb) {
				n++
				continue
			}
			pa, pb := demultiply(a, x, y), demultiply(b, x, y)
			for c := range 4 {
				if d := pa[c] - pb[c]; d > 1 || d < -1 {
					n++
					break
				}
			}
		}
	}
	return float64(n) / float64(w*h)
}

func demultiply(img *image.RGBA, x, y int) [4]int {
	i := img.PixOffset(x, y)
	p := img.Pix[i : i+4]
	a := int(p[3])
	if a == 0 {
		return [4]int{}
	}
	f := func(c uint8) int { return (int(c)*255 + a/2) / a }
	return [4]int{f(p[0]), f(p[1]), f(p[2]), a}
}
