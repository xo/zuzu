package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
	"github.com/xo/zuzu/text/emoji"
	"github.com/xo/zuzu/theme"
)

// drawZuzu draws each source with zuzu, in the light and the dark mode, with
// cfg.jobs sources at the same time.
func drawZuzu(ctx context.Context, cfg config, srcs []*source) {
	sem := make(chan struct{}, cfg.jobs)
	var wg sync.WaitGroup
	for _, s := range srcs {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			drawOne(ctx, cfg, s)
		})
	}
	wg.Wait()
}

// drawOne draws one source with zuzu in both modes, and keeps the errors
// and the report.
func drawOne(ctx context.Context, cfg config, s *source) {
	seen := map[string]bool{}
	addReport := func(r zuzu.Report) {
		for _, is := range r {
			if msg := is.Error(); !seen[msg] {
				seen[msg] = true
				s.report = append(s.report, msg)
			}
		}
	}
	src, err := os.ReadFile(s.path)
	if err != nil {
		s.zuzuErr = append(s.zuzuErr, err.Error())
		return
	}
	chart, report, err := parse(src, s.dialect)
	addReport(report)
	if err != nil {
		s.zuzuErr = append(s.zuzuErr, err.Error())
		return
	}
	opts := []zuzu.RenderOption{
		zuzu.WithLoader(zuzu.FSLoader(os.DirFS(filepath.Join(cfg.testdata, "corpus")))),
		zuzu.WithLocation(time.UTC),
		zuzu.WithRandom(func() rand.Source { return rand.NewPCG(1, 2) }),
		zuzu.WithIcons("fa", fa.Pack()),
		zuzu.WithIcons("mdi", mdi.Pack()),
		zuzu.WithIcons("carbon", carbon.Pack()),
		zuzu.WithIcons("k8s", k8s.Pack()),
		zuzu.WithEmoji(emoji.Font()),
	}
	for _, m := range []struct {
		mode theme.Mode
		name string
		file *string
	}{{theme.Light, "light", &s.light}, {theme.Dark, "dark", &s.dark}} {
		rel := filepath.ToSlash(filepath.Join("zuzu", s.front, s.slug+"."+m.name+".svg"))
		file := filepath.Join(cfg.out, rel)
		_ = os.Remove(file)
		var b bytes.Buffer
		report, err := render(ctx, chart, &b, append(opts, zuzu.WithMode(m.mode)))
		addReport(report)
		if err == nil {
			if err = os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
				err = os.WriteFile(file, b.Bytes(), 0o644)
			}
		}
		if err != nil {
			s.zuzuErr = append(s.zuzuErr, m.name+" mode: "+err.Error())
			continue
		}
		*m.file = rel
	}
}

// parse reads a source with zuzu, and turns a panic into an error, so that
// one source cannot stop the run.
func parse(src []byte, d zuzu.Dialect) (chart *zuzu.Chart, report zuzu.Report, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("zuzu panicked in Parse: %v", r)
		}
	}()
	return zuzu.Parse(src, zuzu.WithDialect(d))
}

// render writes the SVG of a chart, and turns a panic into an error.
func render(ctx context.Context, chart *zuzu.Chart, b *bytes.Buffer, opts []zuzu.RenderOption) (report zuzu.Report, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("zuzu panicked in SVG: %v", r)
		}
	}()
	return chart.SVG(ctx, b, opts...)
}
