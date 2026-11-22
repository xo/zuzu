package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/elk"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
	"github.com/xo/zuzu/text/emoji"
	"github.com/xo/zuzu/text/excalifont"
	"github.com/xo/zuzu/text/math"
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
	chart, report, err := parse(src, s.dialect, cfg.look)
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
		lookFont,
	}
	look := []zuzu.RenderOption{}
	if cfg.look != "" {
		look = append(look, zuzu.WithLook(theme.Look(cfg.look)))
	}
	drawModes(ctx, cfg, s, chart, append(opts, look...), "", "", &s.light, &s.dark, addReport)
	if cfg.layouts && hasLayouts(s) {
		drawLayouts(ctx, cfg, s, src, opts)
	}
	if !cfg.handDrawn {
		return
	}
	// The looks drawn by hand parse the source again, because a look that
	// changes the font changes the layout (D40).
	for _, l := range handLooks {
		h := handDrawing{look: l}
		chart, report, err := parse(src, s.dialect, string(l))
		addReport(report)
		if err != nil {
			s.zuzuErr = append(s.zuzuErr, string(l)+": "+err.Error())
		} else {
			drawModes(ctx, cfg, s, chart, append(opts, zuzu.WithLook(l)), "."+string(l), string(l)+", ", &h.light, &h.dark, addReport)
		}
		s.hands = append(s.hands, h)
	}
}

// drawModes draws a chart in the light and the dark mode, and writes each
// SVG to the output folder, with the suffix in its name. It sets light and
// dark to the files it wrote, and adds each error to the source, after the
// prefix.
func drawModes(ctx context.Context, cfg config, s *source, chart *zuzu.Chart, opts []zuzu.RenderOption, suffix, prefix string, light, dark *string, addReport func(zuzu.Report)) {
	for _, m := range []struct {
		mode theme.Mode
		name string
		file *string
	}{{theme.Light, "light", light}, {theme.Dark, "dark", dark}} {
		rel := filepath.ToSlash(filepath.Join("zuzu", s.front, s.slug+suffix+"."+m.name+".svg"))
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
			s.zuzuErr = append(s.zuzuErr, prefix+m.name+" mode: "+err.Error())
			continue
		}
		*m.file = rel
	}
}

// lookFont gives Excalifont to the look like Excalidraw, which a source
// such as a Mermaid diagram with look: handDrawn asks for (D40).
var lookFont = zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())

// parse reads a source with zuzu, in a look or in the look of the source
// when it is empty, and turns a panic into an error, so that one source
// cannot stop the run.
func parse(src []byte, d zuzu.Dialect, look string) (chart *zuzu.Chart, report zuzu.Report, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("zuzu panicked in Parse: %v", r)
		}
	}()
	// The fonts of the math, of the emoji and of the look go to Parse, so
	// that a kind of Mermaid that lays out when it compiles measures them
	// (D36, D38, D40).
	opts := []zuzu.Option{zuzu.WithDialect(d), zuzu.WithMath(math.Font()), zuzu.WithEmoji(emoji.Font()), lookFont}
	if look != "" {
		opts = append(opts, zuzu.WithLook(theme.Look(look)))
	}
	return zuzu.Parse(src, opts...)
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

// drawLayouts draws a diagram in each layout of layoutNames, with the engine
// of ELK available, and forces each layout with zuzu.WithLayout, so that the
// page shows them side by side (D44, D45). The errors of a layout stay with
// its drawing, apart from the errors and the report of the source.
func drawLayouts(ctx context.Context, cfg config, s *source, src []byte, opts []zuzu.RenderOption) {
	for _, name := range layoutNames {
		d := layoutDrawing{name: name}
		chart, _, err := parseLayout(src, s.dialect, name)
		if err != nil {
			d.errs = append(d.errs, err.Error())
		} else {
			o := slices.Concat(opts, []zuzu.RenderOption{zuzu.WithLayoutEngine(elk.Layout()), zuzu.WithLayout(name)})
			var errs []string
			drawFiles(ctx, cfg, s, chart, o, ".layout-"+name, &d.light, &d.dark, &errs)
			d.errs = append(d.errs, errs...)
		}
		s.layouts = append(s.layouts, d)
	}
}

// parseLayout reads a source with zuzu, with the engine of ELK available
// and a layout forced, and turns a panic into an error.
func parseLayout(src []byte, d zuzu.Dialect, layout string) (chart *zuzu.Chart, report zuzu.Report, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("zuzu panicked in Parse: %v", r)
		}
	}()
	return zuzu.Parse(src, zuzu.WithDialect(d), zuzu.WithMath(math.Font()), zuzu.WithEmoji(emoji.Font()), lookFont,
		zuzu.WithLayoutEngine(elk.Layout()), zuzu.WithLayout(layout))
}

// drawFiles draws a chart in the light and the dark mode, and writes each
// SVG to the output folder, with the suffix in its name. It sets light and
// dark to the files it wrote, and adds each error to errs.
func drawFiles(ctx context.Context, cfg config, s *source, chart *zuzu.Chart, opts []zuzu.RenderOption, suffix string, light, dark *string, errs *[]string) {
	for _, m := range []struct {
		mode theme.Mode
		name string
		file *string
	}{{theme.Light, "light", light}, {theme.Dark, "dark", dark}} {
		rel := filepath.ToSlash(filepath.Join("zuzu", s.front, s.slug+suffix+"."+m.name+".svg"))
		file := filepath.Join(cfg.out, rel)
		_ = os.Remove(file)
		var b bytes.Buffer
		_, err := render(ctx, chart, &b, append(opts, zuzu.WithMode(m.mode)))
		if err == nil {
			if err = os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
				err = os.WriteFile(file, b.Bytes(), 0o644)
			}
		}
		if err != nil {
			*errs = append(*errs, m.name+" mode: "+err.Error())
			continue
		}
		*m.file = rel
	}
}
