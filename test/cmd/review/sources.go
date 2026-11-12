package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
)

// The names of the front ends, which the flag -front takes.
const (
	frontVega     = "vega"
	frontVegaLite = "vegalite"
	frontDeneb    = "deneb"
	frontDOT      = "dot"
	frontMermaid  = "mermaid"
)

// The commands that draw the original of a diagram.
const (
	toolDOT  = "dot"
	toolMmdc = "mmdc"
)

// front is a front end of zuzu, which the page shows in a tab of its own.
type front struct {
	id    string
	title string
	// original names the renderer that draws the original.
	original string
}

// fronts are the front ends, in the order of the tabs.
var fronts = []front{
	{frontVega, "Vega", "vega.js"},
	{frontVegaLite, "Vega-Lite", "vega-lite.js and vega.js"},
	{frontDeneb, "Deneb", "vega.js, and vega-lite.js for a Vega-Lite spec"},
	{frontDOT, "DOT", "Graphviz dot"},
	{frontMermaid, "Mermaid", "mmdc of mermaid-cli"},
}

// frontIDs returns the names of the front ends.
func frontIDs() []string {
	ids := make([]string, len(fronts))
	for i, f := range fronts {
		ids[i] = f.id
	}
	return ids
}

// source is one spec or diagram, and what each side drew from it.
type source struct {
	front string
	// name is the name on the page, which -only matches.
	name string
	// kind is the kind of a Mermaid diagram, such as flowchart.
	kind string
	// path is the file, relative to the folder the command runs in.
	path string
	// spec is the path of a spec in the corpus.
	spec    string
	dialect zuzu.Dialect
	// slug names the files that the command writes for the source.
	slug string

	// original is the SVG of the original, relative to the output folder,
	// or empty with the reason in originalErr.
	original    string
	originalErr string

	// light and dark are the SVG files of zuzu, relative to the output
	// folder. zuzuErr holds each error, and report the issues of the loose
	// mode.
	light, dark string
	zuzuErr     []string
	report      []string
}

// collect returns the sources of the front ends that cfg selects.
func collect(cfg config) ([]*source, error) {
	var all []*source
	for _, f := range cfg.fronts {
		var srcs []*source
		var err error
		switch f {
		case frontVega, frontVegaLite, frontDeneb:
			srcs, err = corpusSources(cfg.testdata, f)
		case frontDOT:
			srcs, err = dotSources(cfg.testdata)
		case frontMermaid:
			srcs, err = mermaidSources(cfg)
		}
		if err != nil {
			return nil, err
		}
		for _, s := range srcs {
			if cfg.only != "" && !strings.Contains(s.name, cfg.only) {
				continue
			}
			s.slug = strings.ReplaceAll(s.name, "/", "_")
			all = append(all, s)
		}
	}
	return all, nil
}

// corpusSources returns the specs of a folder of the corpus that the
// inventory puts in the first target.
func corpusSources(dir, folder string) ([]*source, error) {
	corpus := filepath.Join(dir, "corpus")
	f, err := os.Open(filepath.Join(corpus, "inventory.tsv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var srcs []*source
	sc := bufio.NewScanner(f)
	for n := 0; sc.Scan(); n++ {
		cols := strings.Split(sc.Text(), "\t")
		if n == 0 || len(cols) < 8 {
			continue
		}
		spec, dialect, first := cols[0], cols[1], cols[7]
		if first != "yes" || !strings.HasPrefix(spec, folder+"/") {
			continue
		}
		s := &source{front: folder, name: spec, path: filepath.Join(corpus, spec), spec: spec, dialect: zuzu.Vega}
		if dialect == frontVegaLite {
			s.dialect = zuzu.VegaLite
		}
		srcs = append(srcs, s)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading the inventory: %w", err)
	}
	return srcs, nil
}

// dotSources returns the fixtures of DOT.
func dotSources(dir string) ([]*source, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "diagrams", "dot", "*.dot"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	srcs := make([]*source, 0, len(paths))
	for _, p := range paths {
		srcs = append(srcs, &source{front: frontDOT, name: "dot/" + filepath.Base(p), path: p, dialect: zuzu.DOT})
	}
	return srcs, nil
}

// mermaidSources returns the fixtures of Mermaid of the test module, and
// the fixtures of the package graph/mermaid, which holds a folder for each
// kind of diagram.
func mermaidSources(cfg config) ([]*source, error) {
	fixtures, err := filepath.Glob(filepath.Join(cfg.testdata, "diagrams", "mermaid", "*.mmd"))
	if err != nil {
		return nil, err
	}
	sort.Strings(fixtures)
	var srcs []*source
	for _, p := range fixtures {
		kind := "unknown"
		if b, err := os.ReadFile(p); err == nil {
			if d, err := mermaid.Read(string(b)); err == nil {
				kind = d.Kind
			}
		}
		srcs = append(srcs, &source{front: frontMermaid, name: "diagrams/" + filepath.Base(p), kind: kind, path: p, dialect: zuzu.Mermaid})
	}
	kinds, err := filepath.Glob(filepath.Join(cfg.root, "graph", "mermaid", "testdata", "*", "*.mmd"))
	if err != nil {
		return nil, err
	}
	sort.Strings(kinds)
	for _, p := range kinds {
		kind := filepath.Base(filepath.Dir(p))
		srcs = append(srcs, &source{front: frontMermaid, name: kind + "/" + filepath.Base(p), kind: kind, path: p, dialect: zuzu.Mermaid})
	}
	return srcs, nil
}

// counts are the counts of a front end that the page and the summary show.
type counts struct {
	sources        int
	zuzuErrors     int
	originalErrors int
	reported       int
}

// countFront counts the sources of a front end.
func countFront(srcs []*source, id string) counts {
	var c counts
	for _, s := range srcs {
		if s.front != id {
			continue
		}
		c.sources++
		if len(s.zuzuErr) > 0 {
			c.zuzuErrors++
		}
		if s.originalErr != "" {
			c.originalErrors++
		}
		if len(s.report) > 0 {
			c.reported++
		}
	}
	return c
}

// copySources copies each source to the output folder, so that the page
// can link to its text when a server serves only that folder.
func copySources(cfg config, srcs []*source) error {
	for _, s := range srcs {
		b, err := os.ReadFile(s.path)
		if err != nil {
			return err
		}
		dst := filepath.Join(cfg.out, sourceFile(s))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// sourceFile returns the copy of the source, relative to the output folder.
// The extension .txt makes a browser show it as text.
func sourceFile(s *source) string {
	return filepath.ToSlash(filepath.Join("source", s.front, s.slug+".txt"))
}
