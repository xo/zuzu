// Command rasterbench measures how long each step takes to draw the specs
// of the first target as an image through SVG: the scene, the SVG writer,
// the parse of okage and its rasterization. It times the direct renderer
// too, and counts the pixels that differ between the two images (D11). Run
// it in the test folder:
//
//	go run ./cmd/rasterbench [-n runs] [-only text]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/raster"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/test/conform"
	"github.com/xo/zuzu/text"
)

func main() {
	runs := flag.Int("n", 5, "the runs of each step, of which the fastest counts")
	only := flag.String("only", "", "measure only the specs whose path holds this text")
	dir := flag.String("testdata", "testdata", "the testdata folder")
	dump := flag.String("dump", "", "write the two images of each spec and their difference as PNG to this folder")
	flag.Parse()
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	dumpDir = *dump
	if err := run(ctx, *dir, *only, *runs); err != nil {
		fmt.Fprintln(os.Stderr, "rasterbench:", err)
		os.Exit(1)
	}
}

// textRE matches a text element of the writer, which holds no other
// element.
var textRE = regexp.MustCompile(`<text[^>]*>.*?</text>`)

// arcRE matches the radii, the rotation and the flags of an arc of a path,
// so that the arc becomes a line to its end point, and rxRE matches a round
// corner of a rect. okage parses an arc slowly, and these estimate the
// parse with a fast arc.
var (
	arcRE = regexp.MustCompile(`A[-0-9.e]+,[-0-9.e]+ [-0-9.e]+ [01],[01] `)
	rxRE  = regexp.MustCompile(` r[xy]="[^"]*"`)
)

// dumpDir is the folder of -dump, empty for none.
var dumpDir string

// step is the time of each step for one spec.
type step struct {
	spec                        string
	scene, write, parse, render time.Duration
	// bare is the parse of the SVG without its text, and flat the parse
	// with each arc a line and no round corners.
	bare, flat time.Duration
	// image is the time of the image through SVG, and direct the time of
	// raster.Draw.
	image, direct time.Duration
	// diff is the share of the pixels that differ between the two images,
	// by the metric of okage D31.
	diff float64
}

// fastest runs fn n times and returns the least time.
func fastest(n int, fn func() error) (time.Duration, error) {
	best := time.Duration(-1)
	for range n {
		start := time.Now()
		if err := fn(); err != nil {
			return 0, err
		}
		if d := time.Since(start); best < 0 || d < best {
			best = d
		}
	}
	return best, nil
}

func run(ctx context.Context, dir, only string, runs int) error {
	paths, err := filepath.Glob(filepath.Join(dir, "references", "*", "*.ref.json"))
	if err != nil {
		return err
	}
	slices.Sort(paths)
	// The bundled fonts, so that the measure is the same on every machine.
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
	// The first parse loads the system fonts. It happens once in a program,
	// so it does not count.
	if _, err := r.Render([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"><text>a</text></svg>`)); err != nil {
		return err
	}
	opts := []zuzu.RenderOption{
		zuzu.WithLoader(zuzu.FSLoader(os.DirFS(filepath.Join(dir, "corpus")))),
		zuzu.WithLocation(time.UTC),
		zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }),
	}
	var steps []step
	for _, p := range paths {
		if only != "" && !strings.Contains(p, only) {
			continue
		}
		ref, err := conform.Read(p)
		if err != nil {
			return err
		}
		src := []byte(ref.Compiled)
		if len(src) == 0 {
			if src, err = os.ReadFile(filepath.Join(dir, "corpus", ref.Spec)); err != nil {
				return err
			}
		}
		chart, _, err := zuzu.Parse(src, zuzu.WithDialect(zuzu.Vega))
		if err != nil {
			continue
		}
		var st step
		var view *zuzu.View
		if st.scene, err = fastest(runs, func() error {
			var err error
			view, _, err = chart.NewView(ctx, opts...)
			return err
		}); err != nil {
			continue
		}
		var b bytes.Buffer
		if st.write, err = fastest(runs, func() error {
			b.Reset()
			return svg.Write(&b, view.Scene())
		}); err != nil {
			return fmt.Errorf("%s: %w", ref.Spec, err)
		}
		var doc *okagesvg.Document
		if st.parse, err = fastest(runs, func() error {
			var err error
			doc, err = r.Parse(b.Bytes())
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", ref.Spec, err)
			continue
		}
		if st.render, err = fastest(runs, func() error {
			_, err := doc.Render()
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", ref.Spec, err)
			continue
		}
		bare := textRE.ReplaceAll(b.Bytes(), nil)
		if st.bare, err = fastest(runs, func() error {
			_, err := r.Parse(bare)
			return err
		}); err != nil {
			return fmt.Errorf("%s: %w", ref.Spec, err)
		}
		flat := rxRE.ReplaceAll(arcRE.ReplaceAll(b.Bytes(), []byte("L")), nil)
		if st.flat, err = fastest(runs, func() error {
			_, err := r.Parse(flat)
			return err
		}); err != nil {
			return fmt.Errorf("%s: %w", ref.Spec, err)
		}
		st.spec = ref.Spec
		if err := compare(r, view.Scene(), runs, &st); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", ref.Spec, err)
			continue
		}
		st.spec = ref.Spec
		steps = append(steps, st)
	}
	report(steps)
	return nil
}

// report prints the total of each step and the share that a direct
// renderer can save, over the sum and per spec.
func report(steps []step) {
	var sum step
	shares := make([]float64, 0, len(steps))
	for _, s := range steps {
		sum.scene += s.scene
		sum.write += s.write
		sum.parse += s.parse
		sum.render += s.render
		sum.bare += s.bare
		sum.flat += s.flat
		sum.image += s.image
		sum.direct += s.direct
		image := s.write + s.parse + s.render
		if image > 0 {
			shares = append(shares, float64(s.write+s.parse)/float64(image))
		}
	}
	slices.Sort(shares)
	total := sum.scene + sum.write + sum.parse + sum.render
	pct := func(d time.Duration) float64 { return 100 * float64(d) / float64(total) }
	fmt.Printf("specs: %d\n", len(steps))
	fmt.Printf("scene:  %10v  %5.1f%%\n", sum.scene.Round(time.Microsecond), pct(sum.scene))
	fmt.Printf("write:  %10v  %5.1f%%\n", sum.write.Round(time.Microsecond), pct(sum.write))
	fmt.Printf("parse:  %10v  %5.1f%%\n", sum.parse.Round(time.Microsecond), pct(sum.parse))
	fmt.Printf("render: %10v  %5.1f%%\n", sum.render.Round(time.Microsecond), pct(sum.render))
	fmt.Printf("parse without text: %v, %.1f%% of the parse\n", sum.bare.Round(time.Microsecond), 100*float64(sum.bare)/float64(sum.parse))
	fmt.Printf("image: through SVG %v, direct %v, %.1f%% saved\n", sum.image.Round(time.Microsecond), sum.direct.Round(time.Microsecond), 100*(1-float64(sum.direct)/float64(sum.image)))
	over := 0
	byDiff := slices.Clone(steps)
	slices.SortFunc(byDiff, func(a, b step) int { return cmpFloat(b.diff, a.diff) })
	for _, s := range byDiff {
		if s.diff > 0.01 {
			over++
		}
	}
	fmt.Printf("%d of %d specs differ in more than 1%% of the pixels\n", over, len(steps))
	for _, s := range byDiff[:min(12, len(byDiff))] {
		fmt.Printf("  %-60s %6.2f%%\n", s.spec, 100*s.diff)
	}
	byDirect := slices.Clone(steps)
	slices.SortFunc(byDirect, func(a, b step) int { return int(b.direct - a.direct) })
	for _, s := range byDirect[:min(6, len(byDirect))] {
		fmt.Printf("  slow %-55s direct %8v  through SVG %8v\n", s.spec, s.direct.Round(time.Microsecond), s.image.Round(time.Microsecond))
	}
	fmt.Printf("parse with arcs as lines: %v, %.1f%% of the parse\n", sum.flat.Round(time.Microsecond), 100*float64(sum.flat)/float64(sum.parse))
	flatImage := sum.write + sum.flat + sum.render
	fmt.Printf("with arcs as lines, write and parse of the image steps: %.1f%%\n", 100*float64(sum.write+sum.flat)/float64(flatImage))
	slices.SortFunc(steps, func(a, b step) int { return int(b.parse - a.parse) })
	for _, s := range steps[:min(8, len(steps))] {
		fmt.Printf("  %-60s parse %8v  without text %8v  render %8v\n", s.spec, s.parse.Round(time.Microsecond), s.bare.Round(time.Microsecond), s.render.Round(time.Microsecond))
	}
	image := sum.write + sum.parse + sum.render
	fmt.Printf("write and parse of the image steps: %.1f%%\n", 100*float64(sum.write+sum.parse)/float64(image))
	if len(shares) > 0 {
		at := func(q float64) float64 { return 100 * shares[int(q*float64(len(shares)-1))] }
		fmt.Printf("per spec, write and parse of the image steps: median %.1f%%, p10 %.1f%%, p90 %.1f%%\n", at(0.5), at(0.1), at(0.9))
	}
}

// cmpFloat compares two numbers.
func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compare draws a scene through SVG and directly, with the fonts of one
// renderer, times both, and counts the pixels that differ.
func compare(r *okagesvg.Renderer, s *scene.Scene, runs int, st *step) error {
	var viaSVG, direct *image.RGBA
	var err error
	if st.image, err = fastest(runs, func() error {
		var b bytes.Buffer
		if err := svg.Write(&b, s); err != nil {
			return err
		}
		viaSVG, err = r.Render(b.Bytes())
		return err
	}); err != nil {
		return err
	}
	if st.direct, err = fastest(runs, func() error {
		direct, err = raster.Draw(s, raster.WithRenderer(r))
		return err
	}); err != nil {
		return err
	}
	st.diff = pixelDiff(viaSVG, direct)
	if dumpDir != "" {
		return dumpImages(st.spec, viaSVG, direct)
	}
	return nil
}

// pixelDiff returns the share of the pixels that differ, by the metric of
// okage D31: both images demultiplied, a pixel differs when a channel
// differs by more than 1, and a pixel that only one image has differs.
func pixelDiff(a, b *image.RGBA) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
	if w == 0 || h == 0 {
		return 0
	}
	n := 0
	for y := range h {
		for x := range w {
			if !image.Pt(x, y).In(ab) || !image.Pt(x, y).In(bb) {
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

// demultiply returns a pixel with its color divided by its alpha.
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

// dumpImages writes the two images and a map of the pixels that differ.
func dumpImages(spec string, a, b *image.RGBA) error {
	name := strings.NewReplacer("/", "_", ".json", "").Replace(spec)
	diff := image.NewRGBA(a.Bounds())
	for y := range a.Bounds().Dy() {
		for x := range a.Bounds().Dx() {
			c := color.RGBA{255, 255, 255, 255}
			if image.Pt(x, y).In(b.Bounds()) {
				pa, pb := demultiply(a, x, y), demultiply(b, x, y)
				for k := range 4 {
					if d := pa[k] - pb[k]; d > 1 || d < -1 {
						c = color.RGBA{255, 0, 0, 255}
					}
				}
			}
			diff.Set(x, y, c)
		}
	}
	for suffix, img := range map[string]image.Image{"svg": a, "direct": b, "diff": diff} {
		f, err := os.Create(filepath.Join(dumpDir, name+"."+suffix+".png"))
		if err != nil {
			return err
		}
		if err := png.Encode(f, img); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}
