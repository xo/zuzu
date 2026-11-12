// Command review draws each source of the corpus and of the diagram
// fixtures with its original renderer and with zuzu, and writes a page that
// shows the two side by side, so that the maintainers can look at where
// zuzu differs. The originals are vega.js, vega-lite.js, Graphviz dot and
// mmdc of mermaid-cli. Run it in the test folder:
//
//	go run ./cmd/review [-out dir] [-only text] [-front list] [-force] [-j n]
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
	fmt.Fprintf(os.Stderr, "review: %d sources\n", len(srcs))
	done := make(chan struct{})
	go func() {
		drawZuzu(ctx, cfg, srcs)
		close(done)
	}()
	drawOriginals(ctx, cfg, srcs)
	<-done
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
