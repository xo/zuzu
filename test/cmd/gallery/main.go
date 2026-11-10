// Command gallery draws the specs of the corpus with zuzu and writes a page
// that shows each SVG, so that a person can look at what the runtime draws.
// It reads the compiled Vega spec of a Vega-Lite spec from its reference.
// Run it in the test folder:
//
//	go run ./cmd/gallery [-only text] [-dark] [-out dir]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/test/conform"
	"github.com/xo/zuzu/theme"
)

func main() {
	only := flag.String("only", "", "draw only the specs whose path holds this text")
	dark := flag.Bool("dark", false, "draw in the dark mode")
	out := flag.String("out", "gallery", "the folder to write")
	dir := flag.String("testdata", "testdata", "the testdata folder")
	limit := flag.Int("n", 0, "draw at most this many specs")
	pngs := flag.Bool("png", false, "also render each SVG to PNG with okage")
	vl := flag.Bool("vl", false, "compile each Vega-Lite spec with zuzu, rather than read the Vega spec that vega-lite.js compiled")
	flag.Parse()
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	if err := run(ctx, *dir, *out, *only, *dark, *limit, *pngs, *vl); err != nil {
		fmt.Fprintln(os.Stderr, "gallery:", err)
		os.Exit(1)
	}
}

// run draws each spec and writes the page.
func run(ctx context.Context, dir, out, only string, dark bool, limit int, pngs, vl bool) error {
	paths, err := filepath.Glob(filepath.Join(dir, "references", "*", "*.ref.json"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	mode := theme.Light
	bg := "#ffffff"
	if dark {
		mode, bg = theme.Dark, "#111111"
	}
	var page strings.Builder
	fmt.Fprintf(&page, "<!doctype html><meta charset=utf-8><title>zuzu gallery</title><style>body{font-family:sans-serif;background:%s;color:#888;margin:16px}figure{display:inline-block;margin:8px;vertical-align:top;border:1px solid #8884;padding:8px}figcaption{font-size:11px;max-width:400px;overflow-wrap:anywhere}img{max-width:600px}</style>\n", bg)
	n := 0
	for _, p := range paths {
		if only != "" && !strings.Contains(p, only) {
			continue
		}
		if limit > 0 && n >= limit {
			break
		}
		ref, err := conform.Read(p)
		if err != nil {
			return err
		}
		src := []byte(ref.Compiled)
		dialect := zuzu.Vega
		if vl && len(src) > 0 {
			src = nil
			dialect = zuzu.VegaLite
		}
		if len(src) == 0 {
			src, err = os.ReadFile(filepath.Join(dir, "corpus", ref.Spec))
			if err != nil {
				return err
			}
		}
		name := strings.NewReplacer("/", "_", ".json", ".svg").Replace(ref.Spec)
		var caption strings.Builder
		caption.WriteString(html.EscapeString(ref.Spec))
		chart, _, err := zuzu.Parse(src, zuzu.WithDialect(dialect))
		if err == nil {
			var b bytes.Buffer
			var report zuzu.Report
			report, err = chart.SVG(ctx, &b,
				zuzu.WithLoader(zuzu.FSLoader(os.DirFS(filepath.Join(dir, "corpus")))),
				zuzu.WithLocation(time.UTC), zuzu.WithMode(mode),
				zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }))
			if err == nil {
				err = os.WriteFile(filepath.Join(out, name), b.Bytes(), 0o644)
			}
			if err == nil && pngs {
				err = writePNG(filepath.Join(out, strings.TrimSuffix(name, ".svg")+".png"), b.Bytes())
			}
			for _, is := range report {
				caption.WriteString("<br>" + html.EscapeString(is.Error()))
			}
		}
		if err != nil {
			fmt.Fprintf(&page, "<figure><figcaption>%s<br><b>%s</b></figcaption></figure>\n", caption.String(), html.EscapeString(err.Error()))
		} else {
			fmt.Fprintf(&page, "<figure><img src=%q><figcaption>%s</figcaption></figure>\n", name, caption.String())
		}
		n++
	}
	return os.WriteFile(filepath.Join(out, "index.html"), []byte(page.String()), 0o644)
}

// writePNG renders SVG with okage and writes it as PNG.
func writePNG(path string, svg []byte) error {
	img, err := okagesvg.New(okagesvg.WithBackground(color.White)).Render(svg)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
