// Command survey draws the zuzu cards of docs/survey/compare.html. It reads
// each spec in docs/survey/zuzu, draws it in the light and the dark mode,
// and writes docs/survey/static/zuzu-<row>-<light|dark>.svg. A spec names
// its data by url, and the loader reads it from the corpus of the test
// module. The issues of each spec go to standard error. Run it in the test
// folder:
//
//	go run ./cmd/survey [-survey ../docs/survey] [-data testdata/corpus]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/color"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/theme"
)

func main() {
	survey := flag.String("survey", filepath.Join("..", "docs", "survey"), "the survey folder, which holds zuzu and static")
	data := flag.String("data", filepath.Join("testdata", "corpus"), "the folder that the url of a spec is relative to")
	flag.Parse()
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	if err := run(ctx, *survey, *data); err != nil {
		fmt.Fprintln(os.Stderr, "survey:", err)
		os.Exit(1)
	}
}

// run draws each spec of the folder zuzu in both modes.
func run(ctx context.Context, survey, data string) error {
	paths, err := filepath.Glob(filepath.Join(survey, "zuzu", "*"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	loader := zuzu.FSLoader(os.DirFS(data))
	n := 0
	for _, p := range paths {
		base := filepath.Base(p)
		var opts []zuzu.Option
		switch {
		case strings.HasSuffix(base, ".dot"):
			opts = append(opts, zuzu.WithDialect(zuzu.DOT))
		case strings.HasSuffix(base, ".json"):
		default:
			continue
		}
		row, _, _ := strings.Cut(base, ".")
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		chart, report, err := zuzu.Parse(src, opts...)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", p, err)
		}
		for _, is := range report {
			fmt.Fprintf(os.Stderr, "%s: %v\n", row, is)
		}
		for _, m := range []struct {
			name string
			mode theme.Mode
		}{{"light", theme.Light}, {"dark", theme.Dark}} {
			var b bytes.Buffer
			// The background is transparent, as the Vega cards set it, so
			// that the color of the card shows behind the chart.
			report, err := chart.SVG(ctx, &b,
				zuzu.WithLoader(loader),
				zuzu.WithMode(m.mode),
				zuzu.WithBackground(color.Transparent),
				zuzu.WithLocation(time.UTC),
				zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }))
			if err != nil {
				return fmt.Errorf("drawing %s in the %s mode: %w", p, m.name, err)
			}
			if m.mode == theme.Light {
				for _, is := range report {
					fmt.Fprintf(os.Stderr, "%s: %v\n", row, is)
				}
			}
			out := filepath.Join(survey, "static", "zuzu-"+row+"-"+m.name+".svg")
			if err := os.WriteFile(out, b.Bytes(), 0o644); err != nil {
				return err
			}
		}
		n++
	}
	if n == 0 {
		return fmt.Errorf("no spec in %s", filepath.Join(survey, "zuzu"))
	}
	return nil
}
