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

// variant is a way to draw the original of a source: as the source asks,
// or in a config of mmdc, such as the look drawn by hand or the layout
// dagre of Mermaid.
type variant string

// The variants of an original.
const (
	// asSource draws the original as its source asks.
	asSource variant = ""
	// byHand draws the original of a diagram of Mermaid with the config
	// look: handDrawn and handDrawnSeed: 1, so that the page can show it
	// beside the looks of zuzu drawn by hand (D40).
	byHand variant = "handdrawn"
	// byDagre draws the original of a diagram of Mermaid with the config
	// layout: dagre. The default layout of Mermaid 12.3 is ELK, so this is
	// the original that zuzu.dagre follows (D44).
	byDagre variant = "dagre"
)

// variantConfigs are the configs of mmdc for the variants of an original.
var variantConfigs = map[variant]string{
	byHand:  `{"look": "handDrawn", "handDrawnSeed": 1}` + "\n",
	byDagre: `{"layout": "dagre"}` + "\n",
}

// job is an original to draw: a source in a variant.
type job struct {
	s *source
	v variant
}

// drawOriginals draws the original of each source whose cached SVG is
// older than the source, and fills in the original of every source, with
// the original drawn by hand of each diagram of Mermaid when the flag
// -handdrawn is on. A missing tool gives each of its sources an error, not
// a failed run.
func drawOriginals(ctx context.Context, cfg config, srcs []*source) {
	var all []job
	for _, s := range srcs {
		all = append(all, job{s, asSource})
		if cfg.handDrawn && hasHandOriginal(s) {
			all = append(all, job{s, byHand})
		}
		if cfg.layouts && s.front == frontMermaid {
			all = append(all, job{s, byDagre})
		}
	}
	var corpus []*source
	var diagrams []job
	for _, j := range all {
		if !cfg.force && fresh(cfg, j.s, j.v) {
			continue
		}
		// An old SVG must not stand for a source that fails now.
		file := filepath.Join(cfg.out, originalFile(j.s, j.v))
		_ = os.Remove(file)
		_ = os.Remove(file + ".err")
		if j.s.spec != "" {
			corpus = append(corpus, j.s)
		} else {
			diagrams = append(diagrams, j)
		}
	}
	fmt.Fprintf(os.Stderr, "review: drawing %d originals, %d are cached\n", len(corpus)+len(diagrams), len(all)-len(corpus)-len(diagrams))
	// failed holds the reason for an original that gets no SVG and no error,
	// such as a missing tool.
	failed := map[job]string{}
	var mu sync.Mutex
	configs := map[variant]string{}
	for v, text := range variantConfigs {
		file, err := writeConfig(cfg, v, text)
		if err != nil {
			for _, j := range diagrams {
				if j.v == v {
					failed[j] = err.Error()
				}
			}
			continue
		}
		configs[v] = file
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		if msg := drawCorpus(ctx, cfg, corpus); msg != "" {
			mu.Lock()
			for _, s := range corpus {
				failed[job{s, asSource}] = msg
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
		for _, j := range diagrams {
			tool := toolDOT
			if j.s.front == frontMermaid {
				tool = toolMmdc
			}
			mu.Lock()
			if msg := tools[tool]; msg != "" {
				failed[j] = msg
			}
			_, skip := failed[j]
			mu.Unlock()
			if skip {
				continue
			}
			sem <- struct{}{}
			inner.Go(func() {
				defer func() { <-sem }()
				drawDiagram(ctx, cfg, j, configs[j.v])
			})
		}
		inner.Wait()
	})
	wg.Wait()
	for _, j := range all {
		readOriginal(cfg, j, failed[j])
	}
}

// writeConfig writes the config of mmdc for the originals of a variant, and
// returns its path.
func writeConfig(cfg config, v variant, text string) (string, error) {
	file, err := filepath.Abs(filepath.Join(cfg.out, "original", string(v)+".json"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	return file, os.WriteFile(file, []byte(text), 0o644)
}

// originalFile returns the SVG of the original in a variant, relative to the
// output folder. The same name with .err holds the error of a source that
// the original cannot draw.
func originalFile(s *source, v variant) string {
	name := s.slug
	if v != asSource {
		name += "." + string(v)
	}
	return filepath.ToSlash(filepath.Join("original", s.front, name+".svg"))
}

// fresh reports whether the cached SVG or error of the original of a source
// in a variant is newer than the source. The original of a spec of the
// corpus must also be newer than the oracle that draws it.
func fresh(cfg config, s *source, v variant) bool {
	src, err := os.Stat(s.path)
	if err != nil {
		return false
	}
	since := src.ModTime()
	if s.spec != "" {
		if t := oracleTime(); t.After(since) {
			since = t
		}
	}
	file := filepath.Join(cfg.out, originalFile(s, v))
	for _, p := range []string{file, file + ".err"} {
		if fi, err := os.Stat(p); err == nil && fi.ModTime().After(since) {
			return true
		}
	}
	return false
}

// oracleTime returns the time of the newest file of the oracle that draws
// the originals of the corpus: its scripts, and the lock file that npm
// install writes in oracle/node_modules. Thus an original is drawn again
// after a change to a script or to the packages, such as the package
// canvas, which vega.js needs to draw a heatmap, a label or a word cloud.
func oracleTime() time.Time {
	files, _ := filepath.Glob(filepath.Join("oracle", "*.mjs"))
	files = append(files, filepath.Join("oracle", "node_modules", ".package-lock.json"))
	var newest time.Time
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

// readOriginal fills in the original of a job from the cache. The reason is
// the error of an original that has nothing in the cache.
func readOriginal(cfg config, j job, reason string) {
	svg, msg := &j.s.original, &j.s.originalErr
	switch j.v {
	case byHand:
		svg, msg = &j.s.handOriginal, &j.s.handOriginalErr
	case byDagre:
		svg, msg = &j.s.dagreOriginal, &j.s.dagreOriginalErr
	}
	file := filepath.Join(cfg.out, originalFile(j.s, j.v))
	if _, err := os.Stat(file); err == nil {
		*svg = originalFile(j.s, j.v)
		return
	}
	if b, err := os.ReadFile(file + ".err"); err == nil {
		*msg = strings.TrimSpace(string(b))
		if *msg == "" {
			*msg = "the original failed with no message"
		}
		return
	}
	*msg = reason
	if reason == "" {
		*msg = "the original wrote no SVG"
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
		out, err := filepath.Abs(filepath.Join(cfg.out, originalFile(s, asSource)))
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
// its SVG or its error to the cache. The original of a variant gives mmdc
// the config in the file conf.
func drawDiagram(ctx context.Context, cfg config, j job, conf string) {
	ctx, cancel := context.WithTimeout(ctx, drawTimeout)
	defer cancel()
	s := j.s
	file := filepath.Join(cfg.out, originalFile(s, j.v))
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
		if j.v != asSource {
			args = append(args, "-c", conf)
		}
		if cfg.puppeteer != "" {
			args = append(args, "-p", cfg.puppeteer)
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
