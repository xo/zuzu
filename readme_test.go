package zuzu_test

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/text/excalifont"
	"github.com/xo/zuzu/theme"
)

// readmeScale is the scale of the images of README.md. README.md shows each
// image at half its size in pixels, so that it is sharp on a screen of high
// density.
const readmeScale = 2

// readmeImage is one image of README.md, in docs/readme: the example whose
// source it draws, and the options of the parse and of the render.
type readmeImage struct {
	file    string
	example string
	parse   []zuzu.Option
	render  []zuzu.RenderOption
}

// readmeImages returns the images of README.md. The options of each are the
// options of the code that README.md shows next to the image.
func readmeImages() []readmeImage {
	look := zuzu.WithLook(theme.LookExcalidraw)
	font := zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())
	light, dark := zuzu.WithMode(theme.Light), zuzu.WithMode(theme.Dark)
	return []readmeImage{
		{file: "bar.png", example: "ExampleChart_Image"},
		{file: "bar-dark-ember.png", example: "ExampleChart_Image", render: []zuzu.RenderOption{dark, zuzu.WithScheme(theme.Ember)}},
		{file: "flowchart-light.png", example: "ExampleParse_mermaid", render: []zuzu.RenderOption{light}},
		{file: "flowchart-dark.png", example: "ExampleParse_mermaid", render: []zuzu.RenderOption{dark}},
		{file: "graph-light.png", example: "ExampleParse_dot", render: []zuzu.RenderOption{light}},
		{file: "graph-dark.png", example: "ExampleParse_dot", render: []zuzu.RenderOption{dark}},
		{file: "sketch-light.png", example: "ExampleWithLook", parse: []zuzu.Option{look, font}, render: []zuzu.RenderOption{light}},
		{file: "sketch-dark.png", example: "ExampleWithLook", parse: []zuzu.Option{look, font}, render: []zuzu.RenderOption{dark}},
		{file: "line-light.png", example: "ExampleWithLoader", render: []zuzu.RenderOption{light}},
		{file: "line-dark.png", example: "ExampleWithLoader", render: []zuzu.RenderOption{dark}},
	}
}

// TestTheReadmeImagesAreDrawn draws each image of README.md from the source
// of its example in example_test.go, and checks that the file in docs/readme
// has the size of the drawing. Run go test -run TestTheReadmeImagesAreDrawn
// -update to draw the files again. The test does not compare the pixels,
// because the pixels of the look drawn by hand differ from one architecture
// to another (D41), and the images only show what zuzu draws.
func TestTheReadmeImagesAreDrawn(t *testing.T) {
	t.Parallel()
	update := flag.Lookup("update")
	if update == nil {
		t.Fatal("the flag update is not defined. It is in render_test.go")
	}
	sources := exampleSources(t)
	for _, test := range readmeImages() {
		t.Run(test.file, func(t *testing.T) {
			t.Parallel()
			lits := sources[test.example]
			if len(lits) == 0 {
				t.Fatalf("%s holds no raw string, so there is no source to draw", test.example)
			}
			render := append([]zuzu.RenderOption{zuzu.WithScale(readmeScale)}, test.render...)
			if len(lits) > 1 {
				fsys := fstest.MapFS{"temps.csv": {Data: []byte(lits[1])}}
				render = append(render, zuzu.WithLoader(zuzu.FSLoader(fsys)))
			}
			chart, _, err := zuzu.Parse([]byte(lits[0]), test.parse...)
			if err != nil {
				t.Fatal(err)
			}
			img, report, err := chart.Image(t.Context(), render...)
			if err != nil {
				t.Fatal(err)
			}
			if len(report) != 0 {
				t.Errorf("report = %v, want none", report)
			}
			path := filepath.Join("docs", "readme", test.file)
			if update.Value.String() == "true" {
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			cfg, err := png.DecodeConfig(f)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if cfg.Width != img.Bounds().Dx() || cfg.Height != img.Bounds().Dy() {
				t.Errorf("%s is %dx%d, and the chart draws %dx%d: run go test -run TestTheReadmeImagesAreDrawn -update",
					path, cfg.Width, cfg.Height, img.Bounds().Dx(), img.Bounds().Dy())
			}
		})
	}
}

// exampleSources returns the raw strings of each example of example_test.go,
// in order, by the name of the example. The first is the source of the chart,
// and the second, if there is one, is the file that its loader reads.
func exampleSources(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "example_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]string)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "Example") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			out[fn.Name.Name] = append(out[fn.Name.Name], s)
			return true
		})
	}
	return out
}

// goBlock matches a block of Go code in Markdown.
var goBlock = regexp.MustCompile("(?s)```go\n(.*?)```")

// TestTheReadmeCodeIsAnExample checks that each block of Go code in README.md
// is code of example_test.go, where go test compiles and runs it. The
// indentation of a line does not count, because a block of README.md is not
// inside a function.
func TestTheReadmeCodeIsAnExample(t *testing.T) {
	t.Parallel()
	examples := unindent(read(t, "example_test.go"))
	blocks := goBlock.FindAllStringSubmatch(read(t, "README.md"), -1)
	if len(blocks) == 0 {
		t.Fatal("README.md holds no block of Go code, so this guards nothing")
	}
	for _, m := range blocks {
		if !strings.Contains(examples, unindent(m[1])) {
			t.Errorf("README.md shows this code, and example_test.go does not hold it:\n%s", m[1])
		}
	}
}

// unindent removes the space at the start of each line.
func unindent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " \t")
	}
	return strings.Join(lines, "\n")
}
