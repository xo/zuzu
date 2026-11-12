package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xo/zuzu"
)

// drawTimeout is the time that dot or mmdc can take on one source.
const drawTimeout = 2 * time.Minute

// drawOriginals draws the original of each source whose cached SVG is
// older than the source, and fills in the original of every source. A
// missing tool gives each of its sources an error, not a failed run.
func drawOriginals(ctx context.Context, cfg config, srcs []*source) {
	var corpus, diagrams []*source
	for _, s := range srcs {
		if !cfg.force && fresh(cfg, s) {
			continue
		}
		// An old SVG must not stand for a source that fails now.
		file := filepath.Join(cfg.out, originalFile(s))
		_ = os.Remove(file)
		_ = os.Remove(file + ".err")
		if s.spec != "" {
			corpus = append(corpus, s)
		} else {
			diagrams = append(diagrams, s)
		}
	}
	fmt.Fprintf(os.Stderr, "review: drawing %d originals, %d are cached\n", len(corpus)+len(diagrams), len(srcs)-len(corpus)-len(diagrams))
	// failed holds the reason for a source that gets no SVG and no error
	// from its original, such as a missing tool.
	failed := map[*source]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Go(func() {
		if msg := drawCorpus(ctx, cfg, corpus); msg != "" {
			mu.Lock()
			for _, s := range corpus {
				failed[s] = msg
			}
			mu.Unlock()
		}
	})
	wg.Go(func() {
		sem := make(chan struct{}, cfg.jobs)
		var inner sync.WaitGroup
		tools := map[string]string{}
		for _, name := range []string{toolDOT, toolMmdc} {
			if _, err := exec.LookPath(name); err != nil {
				tools[name] = name + " is not on PATH, so the original is missing"
			}
		}
		for _, s := range diagrams {
			tool := toolDOT
			if s.front == frontMermaid {
				tool = toolMmdc
			}
			if msg := tools[tool]; msg != "" {
				mu.Lock()
				failed[s] = msg
				mu.Unlock()
				continue
			}
			sem <- struct{}{}
			inner.Go(func() {
				defer func() { <-sem }()
				drawDiagram(ctx, cfg, s)
			})
		}
		inner.Wait()
	})
	wg.Wait()
	for _, s := range srcs {
		readOriginal(cfg, s, failed[s])
	}
}

// originalFile returns the SVG of the original, relative to the output
// folder. The same name with .err holds the error of a source that the
// original cannot draw.
func originalFile(s *source) string {
	return filepath.ToSlash(filepath.Join("original", s.front, s.slug+".svg"))
}

// fresh reports whether the cached SVG or error of a source is newer than
// the source.
func fresh(cfg config, s *source) bool {
	src, err := os.Stat(s.path)
	if err != nil {
		return false
	}
	file := filepath.Join(cfg.out, originalFile(s))
	for _, p := range []string{file, file + ".err"} {
		if fi, err := os.Stat(p); err == nil && fi.ModTime().After(src.ModTime()) {
			return true
		}
	}
	return false
}

// readOriginal fills in the original of a source from the cache. The reason
// is the error of a source that has nothing in the cache.
func readOriginal(cfg config, s *source, reason string) {
	file := filepath.Join(cfg.out, originalFile(s))
	if _, err := os.Stat(file); err == nil {
		s.original = originalFile(s)
		return
	}
	if b, err := os.ReadFile(file + ".err"); err == nil {
		s.originalErr = strings.TrimSpace(string(b))
		if s.originalErr == "" {
			s.originalErr = "the original failed with no message"
		}
		return
	}
	s.originalErr = reason
	if reason == "" {
		s.originalErr = "the original wrote no SVG"
	}
}

// corpusJob is a line of the input of oracle/svg.mjs.
type corpusJob struct {
	Spec    string `json:"spec"`
	Dialect string `json:"dialect"`
	Out     string `json:"out"`
}

// drawCorpus draws the specs of the corpus with one process of Node.js,
// which is much faster than one process for each spec. It returns the
// reason for a spec that gets no SVG, or an empty string.
func drawCorpus(ctx context.Context, cfg config, srcs []*source) string {
	if len(srcs) == 0 {
		return ""
	}
	if _, err := exec.LookPath("node"); err != nil {
		return "node is not on PATH, so the original is missing"
	}
	for _, pkg := range []string{"vega", "vega-lite"} {
		if _, err := os.Stat(filepath.Join("oracle", "node_modules", pkg)); err != nil {
			return "the package " + pkg + " is not in oracle/node_modules: run npm install in test/oracle"
		}
	}
	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	for _, s := range srcs {
		out, err := filepath.Abs(filepath.Join(cfg.out, originalFile(s)))
		if err != nil {
			return err.Error()
		}
		dialect := frontVega
		if s.dialect == zuzu.VegaLite {
			dialect = frontVegaLite
		}
		if err := enc.Encode(corpusJob{Spec: s.spec, Dialect: dialect, Out: out}); err != nil {
			return err.Error()
		}
	}
	var stderr bytes.Buffer
	//nolint:gosec // the script is in this repository, and the folder is the corpus of the flag -testdata.
	cmd := exec.CommandContext(ctx, "node", filepath.Join("oracle", "svg.mjs"), filepath.Join(cfg.testdata, "corpus"))
	cmd.Env = append(os.Environ(), "TZ=UTC")
	cmd.Stdin = &in
	cmd.Stdout = os.Stderr
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		// The specs that node drew before it stopped keep their SVG.
		msg := fmt.Sprintf("node oracle/svg.mjs stopped before it drew the spec: %v", err)
		if tail := lastLines(stderr.String(), 5); tail != "" {
			msg += "\n" + tail
		}
		return msg
	}
	return ""
}

// drawDiagram draws the original of a diagram with dot or mmdc, and writes
// its SVG or its error to the cache.
func drawDiagram(ctx context.Context, cfg config, s *source) {
	ctx, cancel := context.WithTimeout(ctx, drawTimeout)
	defer cancel()
	file := filepath.Join(cfg.out, originalFile(s))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "review:", err)
		return
	}
	tmp := strings.TrimSuffix(file, ".svg") + ".tmp.svg"
	defer func() { _ = os.Remove(tmp) }()
	var cmd *exec.Cmd
	if s.front == frontDOT {
		cmd = exec.CommandContext(ctx, toolDOT, "-Tsvg", "-o", tmp, s.path) //nolint:gosec // the command is fixed, and the file is a fixture of this repository.
	} else {
		args := []string{"-q", "-i", s.path, "-o", tmp}
		if pc := os.Getenv("PUPPETEER_CONFIG"); pc != "" {
			args = append(args, "-p", pc)
		}
		cmd = exec.CommandContext(ctx, toolMmdc, args...) //nolint:gosec // the command is fixed, and the file is a fixture of this repository.
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		err = finishSVG(tmp, file, s.front == frontMermaid)
	}
	if err != nil {
		msg := strings.TrimSpace(out.String())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			msg = fmt.Sprintf("%s took more than %s", cmd.Path, drawTimeout)
		}
		if msg == "" {
			msg = err.Error()
		}
		writeErr(file, msg)
	}
}

// writeErr writes the error of an original to the cache, and removes its
// SVG.
func writeErr(file, msg string) {
	if len(msg) > 4000 {
		msg = msg[:4000] + "\n..."
	}
	if err := os.WriteFile(file+".err", []byte(msg+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "review:", err)
	}
	_ = os.Remove(file)
}

// rootTag matches the root element of an SVG, and viewBox matches its view
// box.
var (
	rootTag = regexp.MustCompile(`^<svg[^>]*>`)
	viewBox = regexp.MustCompile(`viewBox="[-\d.]+ [-\d.]+ ([\d.]+) ([\d.]+)"`)
)

// finishSVG moves a drawn SVG to its place in the cache. The root element
// of mmdc often has a width of 100%, which an img element cannot size, so
// such an SVG gets the width of its view box, and its height if it has
// none.
func finishSVG(tmp, file string, mermaid bool) error {
	b, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return errors.New("the SVG is empty")
	}
	if root := rootTag.Find(b); mermaid && root != nil && bytes.Contains(root, []byte(` width="100%"`)) {
		if m := viewBox.FindSubmatch(root); m != nil {
			size := ` width="` + string(m[1]) + `"`
			if !bytes.Contains(root, []byte(` height="`)) {
				size += ` height="` + string(m[2]) + `"`
			}
			fixed := bytes.Replace(root, []byte(` width="100%"`), []byte(size), 1)
			b = append(fixed, b[len(root):]...)
		}
	}
	if err := os.WriteFile(file, b, 0o644); err != nil {
		return err
	}
	_ = os.Remove(file + ".err")
	return nil
}

// lastLines returns the last n lines of a text.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
