// Command targets compares the SVG writer of zuzu with the targets (D13).
//
// It reads each target SVG that docs/targets/gen-targets.mjs writes, rebuilds
// it as a scene graph, writes that scene graph with the package svg, and
// renders both files with okage. It writes:
//
//	docs/targets/scene/<name>.json  the scene graph of each target
//	docs/targets/zuzu/<name>.svg    the SVG that zuzu writes from it
//	docs/targets/compare.html       the target beside the output of zuzu
//
// and prints the share of pixels that differ for each target. The scene
// graph is rebuilt from the SVG of the target, so the comparison measures how
// close the writer comes to the look of the targets. It does not test the
// runtime, which makes a scene graph from a spec in phase 3.
//
// With -testdata, it also copies the scene graphs of the tier 0 targets into
// a folder of golden tests, such as ../svg/testdata/targets.
//
// Run it from the test module, after the targets are drawn:
//
//	go run ./cmd/targets
package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
)

// tier0 are the targets of tier 0, whose scene graphs become golden tests.
var tier0 = []string{
	"line", "area", "stacked-area", "bar-grouped", "bar-stacked",
	"histogram", "bubble", "donut", "calendar", "dual-axis",
}

func main() {
	dir := flag.String("dir", filepath.Join("..", "docs", "targets"), "the folder of the targets")
	testdata := flag.String("testdata", "", "a folder to copy the scene graphs of tier 0 into")
	flag.Parse()
	if err := run(*dir, *testdata); err != nil {
		fmt.Fprintln(os.Stderr, "targets:", err)
		os.Exit(1)
	}
}

// result is the comparison of one target.
type result struct {
	name string
	diff float64
	err  error
}

// run compares every target in dir.
func run(dir, testdata string) error {
	files, err := filepath.Glob(filepath.Join(dir, "svg", "*.svg"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("finding targets in %s: none. Draw them first, as docs/targets/README.md says", dir)
	}
	for _, sub := range []string{"scene", "zuzu"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	r := okagesvg.New(okagesvg.WithBackground(color.White))
	var results []result
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".svg")
		results = append(results, compare(r, dir, name, f))
	}
	if testdata != "" {
		if err := copyTier0(dir, testdata); err != nil {
			return err
		}
	}
	return report(dir, results)
}

// compare rebuilds one target as a scene graph, writes it with svg, and
// measures the difference of the two images.
func compare(r *okagesvg.Renderer, dir, name, file string) result {
	res := result{name: name}
	target, err := os.ReadFile(file)
	if err != nil {
		res.err = err
		return res
	}
	s, err := parse(bytes.NewReader(target))
	if err != nil {
		res.err = err
		return res
	}
	var js bytes.Buffer
	if err := scene.Encode(&js, s); err != nil {
		res.err = err
		return res
	}
	var out bytes.Buffer
	if err := svg.Write(&out, s); err != nil {
		res.err = err
		return res
	}
	if err := os.WriteFile(filepath.Join(dir, "scene", name+".json"), js.Bytes(), 0o644); err != nil {
		res.err = err
		return res
	}
	if err := os.WriteFile(filepath.Join(dir, "zuzu", name+".svg"), out.Bytes(), 0o644); err != nil {
		res.err = err
		return res
	}
	a, err := r.Render(target)
	if err != nil {
		res.err = fmt.Errorf("rendering the target: %w", err)
		return res
	}
	b, err := r.Render(out.Bytes())
	if err != nil {
		res.err = fmt.Errorf("rendering the output of zuzu: %w", err)
		return res
	}
	res.diff = difference(a, b)
	return res
}

// difference returns the share of pixels whose color differs by more than a
// small amount in any channel, which ignores the noise of anti-aliasing.
func difference(a, b *image.RGBA) float64 {
	if a.Bounds() != b.Bounds() {
		return 1
	}
	var n, total int
	for i := 0; i+3 < len(a.Pix); i += 4 {
		total++
		for c := range 3 {
			if d := int(a.Pix[i+c]) - int(b.Pix[i+c]); d > 24 || d < -24 {
				n++
				break
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

// copyTier0 copies the scene graphs of the tier 0 targets, in both modes of
// the default scheme, into a folder of golden tests.
func copyTier0(dir, testdata string) error {
	if err := os.MkdirAll(testdata, 0o755); err != nil {
		return err
	}
	for _, name := range tier0 {
		for _, mode := range []string{"light", "dark"} {
			file := name + "-" + mode + ".json"
			b, err := os.ReadFile(filepath.Join(dir, "scene", file))
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(testdata, file), b, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// report prints the results and writes the page that compares them.
func report(dir string, results []result) error {
	var page strings.Builder
	page.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>zuzu Against the Targets</title>
<!-- Generated by test/cmd/targets. Do not edit. -->
<style>
body { margin: 0; background: #e9ebef; color: #1b2230; font: 14px/1.5 'IBM Plex Sans', sans-serif; }
main { max-width: 1440px; margin: 0 auto; padding: 24px 16px; display: grid; gap: 20px; }
figure { margin: 0; display: grid; gap: 6px; }
.pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.pair img { width: 100%; height: auto; border-radius: 10px; box-shadow: 0 1px 2px rgb(0 0 0 / .1); }
figcaption { font-size: .85rem; color: #5f6b7d; }
figcaption b { color: #1b2230; }
.bad { color: #c43d5f; }
</style>
</head>
<body>
<main>
<h1>zuzu against the targets</h1>
<p>Each row is a target, on the left, and the SVG that zuzu writes from its scene graph, on the right. The number is the share of pixels that differ.</p>
`)
	var failed int
	for _, r := range results {
		if r.err != nil {
			failed++
			fmt.Printf("%-32s error: %v\n", r.name, r.err)
			fmt.Fprintf(&page, "<figure><figcaption><b>%s</b> <span class=\"bad\">%s</span></figcaption></figure>\n",
				html.EscapeString(r.name), html.EscapeString(r.err.Error()))
			continue
		}
		fmt.Printf("%-32s %6.2f%% of pixels differ\n", r.name, 100*r.diff)
		fmt.Fprintf(&page, "<figure><div class=\"pair\"><img src=\"svg/%[1]s.svg\" alt=\"target %[1]s\" loading=\"lazy\"><img src=\"zuzu/%[1]s.svg\" alt=\"zuzu %[1]s\" loading=\"lazy\"></div><figcaption><b>%[1]s</b> %.2f%% of pixels differ</figcaption></figure>\n",
			html.EscapeString(r.name), 100*r.diff)
	}
	page.WriteString("</main>\n</body>\n</html>\n")
	fmt.Printf("%d targets, %d not rebuilt\n", len(results), failed)
	return os.WriteFile(filepath.Join(dir, "compare.html"), []byte(page.String()), 0o644)
}

// errUnsupported is the error of a part of SVG that a scene graph does not
// hold.
var errUnsupported = errors.New("not held by a scene graph")

// builder rebuilds a scene graph from the elements of an SVG file.
type builder struct {
	scene     *scene.Scene
	grads     map[string]*scene.Gradient
	grad      *scene.Gradient
	stack     []*[]scene.Mark
	text      *scene.Item
	skip      int
	inDefs    bool
	inTitle   bool
	hasBackgr bool
}

// parse reads a target SVG into a scene graph.
func parse(r io.Reader) (*scene.Scene, error) {
	b := &builder{scene: &scene.Scene{}, grads: map[string]*scene.Gradient{}}
	root := &b.scene.Root
	root.Type = scene.Group
	root.Items = []scene.Item{{}}
	b.stack = []*[]scene.Mark{&root.Items[0].Items}
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := b.start(t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			b.end(t)
		case xml.CharData:
			if b.text != nil && b.skip == 0 {
				b.text.Text = scene.Lines{string(t)}
			}
		}
	}
	root.Items[0].Width, root.Items[0].Height = b.scene.Width, b.scene.Height
	return b.scene, nil
}

// start handles the start of an element.
func (b *builder) start(t xml.StartElement) error {
	a := attrs(t)
	if b.skip > 0 {
		b.skip++
		return nil
	}
	switch t.Name.Local {
	case "svg":
		b.scene.Width, b.scene.Height = num(a["width"]), num(a["height"])
	case "title":
		b.inTitle = true
	case "defs":
		b.inDefs = true
	case "marker":
		b.skip = 1
	case "linearGradient":
		b.grad = &scene.Gradient{Type: scene.Linear, X1: num(a["x1"]), Y1: num(a["y1"]), X2: num(a["x2"]), Y2: num(a["y2"])}
		b.grads[a["id"]] = b.grad
	case "stop":
		if b.grad != nil {
			b.grad.Stops = append(b.grad.Stops, scene.Stop{Offset: num(a["offset"]), Color: withOpacity(a["stop-color"], a["stop-opacity"])})
		}
	case "g":
		x, y, angle, scale := transform(a["transform"])
		if scale != 1 || angle != 0 {
			return fmt.Errorf("reading a group with %q: %w", a["transform"], errUnsupported)
		}
		b.add(scene.Group, scene.Item{X: x, Y: y})
		marks := *b.stack[len(b.stack)-1]
		item := &marks[len(marks)-1].Items[len(marks[len(marks)-1].Items)-1]
		b.stack = append(b.stack, &item.Items)
	case "rect":
		it := b.paint(a)
		it.X, it.Y, it.Width, it.Height = num(a["x"]), num(a["y"]), num(a["width"]), num(a["height"])
		it.CornerRadius = num(a["rx"])
		if !b.hasBackgr && a["x"] == "" && it.Width == b.scene.Width && it.Height == b.scene.Height {
			b.hasBackgr = true
			b.scene.Background = it.Fill
			return nil
		}
		b.add(scene.Rect, it)
	case "path":
		it := b.paint(a)
		it.Path = a["d"]
		x, y, angle, _ := transform(a["transform"])
		it.X, it.Y, it.Angle = x, y, angle
		b.add(scene.Path, it)
	case "polygon":
		it := b.paint(a)
		it.Path = "M" + strings.ReplaceAll(strings.TrimSpace(a["points"]), " ", "L") + "Z"
		b.add(scene.Path, it)
	case "circle":
		it := b.paint(a)
		r := num(a["r"])
		size := 4 * r * r
		it.X, it.Y, it.Size, it.Shape = num(a["cx"]), num(a["cy"]), &size, "circle"
		b.add(scene.Symbol, it)
	case "line":
		it := b.paint(a)
		it.Fill = scene.Paint{}
		it.X, it.Y, it.X2, it.Y2 = num(a["x1"]), num(a["y1"]), num(a["x2"]), num(a["y2"])
		b.add(scene.Rule, it)
	case "text":
		it := b.paint(a)
		it.X, it.Y = num(a["x"]), num(a["y"])
		if tr := a["transform"]; tr != "" {
			x, y, angle, _ := transform(tr)
			it.X, it.Y, it.Angle = it.X+x, it.Y+y, angle
		}
		it.FontSize = num(a["font-size"])
		it.FontWeight = scene.Weight(a["font-weight"])
		it.Align = map[string]string{"middle": "center", "end": "right"}[a["text-anchor"]]
		it.Baseline = "alphabetic"
		b.add(scene.Text, it)
		marks := *b.stack[len(b.stack)-1]
		m := &marks[len(marks)-1]
		b.text = &m.Items[len(m.Items)-1]
	}
	return nil
}

// end handles the end of an element.
func (b *builder) end(t xml.EndElement) {
	if b.skip > 0 {
		b.skip--
		return
	}
	switch t.Name.Local {
	case "title":
		b.inTitle = false
	case "defs":
		b.inDefs = false
	case "linearGradient":
		b.grad = nil
	case "g":
		b.stack = b.stack[:len(b.stack)-1]
	case "text":
		b.text = nil
	}
}

// add appends an item to the current list of marks. It joins an item to the
// last mark when that mark has the same type, so that a row of bars is one
// mark, as in Vega.
func (b *builder) add(kind string, it scene.Item) {
	marks := b.stack[len(b.stack)-1]
	if n := len(*marks); n > 0 && (*marks)[n-1].Type == kind && kind != scene.Group {
		(*marks)[n-1].Items = append((*marks)[n-1].Items, it)
		return
	}
	*marks = append(*marks, scene.Mark{Type: kind, Items: []scene.Item{it}})
}

// paint reads the fill, the stroke and the opacities of an element. A shape
// with no fill attribute is black, as in SVG.
func (b *builder) paint(a map[string]string) scene.Item {
	var it scene.Item
	switch f := a["fill"]; {
	case f == "":
		it.Fill = scene.Paint{Color: "#000"}
	case f == "none" || f == "transparent":
	case strings.HasPrefix(f, "url(#"):
		if g, ok := b.grads[strings.TrimSuffix(strings.TrimPrefix(f, "url(#"), ")")]; ok {
			it.Fill = scene.Paint{Gradient: g}
		}
	default:
		it.Fill = scene.Paint{Color: f}
	}
	if s := a["stroke"]; s != "" && s != "none" {
		it.Stroke = scene.Paint{Color: s}
		w := 1.0
		if v, ok := a["stroke-width"]; ok {
			w = num(v)
		}
		it.StrokeWidth = &w
	}
	for attr, dst := range map[string]**float64{"opacity": &it.Opacity, "fill-opacity": &it.FillOpacity, "stroke-opacity": &it.StrokeOpacity} {
		if v, ok := a[attr]; ok {
			n := num(v)
			*dst = &n
		}
	}
	if d := a["stroke-dasharray"]; d != "" {
		for f := range strings.FieldsSeq(strings.ReplaceAll(d, ",", " ")) {
			it.StrokeDash = append(it.StrokeDash, num(f))
		}
	}
	it.StrokeCap, it.StrokeJoin = a["stroke-linecap"], a["stroke-linejoin"]
	return it
}

// attrs returns the attributes of an element by name.
func attrs(t xml.StartElement) map[string]string {
	m := make(map[string]string, len(t.Attr))
	for _, a := range t.Attr {
		m[a.Name.Local] = a.Value
	}
	return m
}

// transformPart matches one function of a transform attribute.
var transformPart = regexp.MustCompile(`(translate|rotate|scale)\(([^)]*)\)`)

// transform reads a transform attribute of translations, a rotation and a
// scale.
func transform(s string) (x, y, angle, scale float64) {
	scale = 1
	for _, m := range transformPart.FindAllStringSubmatch(s, -1) {
		args := strings.FieldsFunc(m[2], func(r rune) bool { return r == ',' || r == ' ' })
		switch m[1] {
		case "translate":
			if len(args) > 0 {
				x += num(args[0])
			}
			if len(args) > 1 {
				y += num(args[1])
			}
		case "rotate":
			if len(args) > 0 {
				angle = num(args[0])
			}
		case "scale":
			if len(args) > 0 {
				scale = num(args[0])
			}
		}
	}
	return x, y, angle, scale
}

// withOpacity returns a color with an opacity as rgba(), or the color as it
// is when the opacity is 1 or absent.
func withOpacity(c, opacity string) string {
	if opacity == "" || num(opacity) == 1 || len(c) != 7 || c[0] != '#' {
		return c
	}
	v, err := strconv.ParseUint(c[1:], 16, 32)
	if err != nil {
		return c
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", v>>16&0xff, v>>8&0xff, v&0xff,
		strconv.FormatFloat(math.Round(num(opacity)*1000)/1000, 'f', -1, 64))
}

// num reads a number, and reads anything else as zero.
func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}
