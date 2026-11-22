// Command review draws each source of the corpus and of the diagram
// fixtures with its original renderer and with zuzu, and writes a page that
// shows the two side by side, so that the maintainers can look at where
// zuzu differs. The originals are vega.js, vega-lite.js, Graphviz dot and
// mmdc of mermaid-cli. Run it in the test folder:
//
//	go run ./cmd/review [-out dir] [-only text] [-front list] [-force] [-j n] [-look name] [-handdrawn=false] [-layouts=false]
//
// With -handdrawn, on by default, each row also shows the source drawn by
// hand: by mmdc with look: handDrawn for a diagram of Mermaid, and by zuzu
// in each of its looks drawn by hand. With -layouts, on by default, each
// diagram also shows its layouts: the original, and the original with
// layout: dagre for Mermaid, beside zuzu.layered, zuzu.dagre and zuzu.elk,
// each with its error of the places of the nodes (D44).
//
// It keeps the SVG of each original in the output folder, and draws an
// original again only when its source is newer, or with -force.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// config holds the flags of the command.
type config struct {
	testdata string
	root     string
	out      string
	only     string
	fronts   []string
	force    bool
	jobs     int
	// look is the look of the flag -look, or empty for the look of each
	// source.
	look string
	// handDrawn draws each source in each look drawn by hand too, and the
	// original of a diagram of Mermaid with look: handDrawn, side by side.
	handDrawn bool
	// layouts draws each diagram in each layout of zuzu too, and the
	// original of a diagram of Mermaid with layout: dagre, side by side.
	layouts bool
	// puppeteer is the file of the config of puppeteer that mmdc reads, or
	// empty for its own search.
	puppeteer string
}

func main() {
	var cfg config
	var frontList string
	flag.StringVar(&cfg.out, "out", "../docs/review", "the folder to write the page and the SVG files to")
	flag.StringVar(&cfg.only, "only", "", "draw only the sources whose name holds this text")
	flag.StringVar(&frontList, "front", strings.Join(frontIDs(), ","), "the front ends to draw, a comma list of "+strings.Join(frontIDs(), ","))
	flag.BoolVar(&cfg.force, "force", false, "draw each original again, even when its SVG is newer than its source")
	flag.IntVar(&cfg.jobs, "j", max(1, runtime.NumCPU()/2), "the number of sources to draw at the same time")
	flag.StringVar(&cfg.testdata, "testdata", "testdata", "the testdata folder of the test module")
	flag.StringVar(&cfg.root, "root", "..", "the root folder of the repository")
	flag.StringVar(&cfg.look, "look", "", "draw each source of zuzu in this look, such as excalidraw, rather than in the look of the source")
	flag.BoolVar(&cfg.handDrawn, "handdrawn", true, "draw each source in each look drawn by hand too, beside the original of Mermaid with look: handDrawn")
	flag.BoolVar(&cfg.layouts, "layouts", true, "draw each diagram in each layout of zuzu too, beside the original of Mermaid with layout: dagre")
	flag.Parse()
	for f := range strings.SplitSeq(frontList, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !slices.Contains(frontIDs(), f) {
			fmt.Fprintf(os.Stderr, "review: unknown front end %q, want one of %s\n", f, strings.Join(frontIDs(), ","))
			os.Exit(2)
		}
		cfg.fronts = append(cfg.fronts, f)
	}
	cfg.jobs = max(1, cfg.jobs)
	ctx := context.Background() //nolint:forbidigo // main owns the context of the program.
	if err := run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "review:", err)
		os.Exit(1)
	}
}

// run collects the sources, draws them on both sides and writes the page.
func run(ctx context.Context, cfg config) error {
	start := time.Now()
	srcs, err := collect(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	if cfg.puppeteer, err = puppeteerConfig(cfg.out); err != nil {
		return err
	}
	if cfg.puppeteer != "" {
		fmt.Fprintf(os.Stderr, "review: mmdc reads the config of puppeteer %s\n", cfg.puppeteer)
	}
	fmt.Fprintf(os.Stderr, "review: %d sources\n", len(srcs))
	done := make(chan struct{})
	go func() {
		drawZuzu(ctx, cfg, srcs)
		close(done)
	}()
	drawOriginals(ctx, cfg, srcs)
	<-done
	if cfg.layouts {
		measureLayouts(ctx, cfg, srcs)
	}
	if err := copySources(cfg, srcs); err != nil {
		return err
	}
	if err := writePage(cfg, srcs); err != nil {
		return err
	}
	for _, f := range fronts {
		c := countFront(srcs, f.id)
		if c.sources == 0 {
			continue
		}
		fmt.Printf("%-9s %4d sources, %3d zuzu errors, %3d original errors, %3d with report lines\n",
			f.id, c.sources, c.zuzuErrors, c.originalErrors, c.reported)
	}
	fmt.Printf("wrote %s in %s\n", filepath.Join(cfg.out, "index.html"), time.Since(start).Round(time.Millisecond))
	return nil
}
