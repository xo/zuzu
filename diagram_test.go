package zuzu

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/icons/fa"
)

func TestSniffDiagram(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src string
		exp Dialect
	}{
		{"digraph { a -> b }", DOT},
		{"strict graph { a -- b }", DOT},
		{"graph {\n a -- b }", DOT},
		{"graph G { a -- b }", DOT},
		{"// comment\ndigraph { a }", DOT},
		{"/* c */ digraph { a }", DOT},
		{"graph TD\n A --> B", Mermaid},
		{"flowchart LR\n A --> B", Mermaid},
		{"%% comment\nflowchart LR\n A", Mermaid},
		{"---\ntitle: x\n---\nflowchart LR\n A", Mermaid},
		{"pie\n \"a\": 1", Mermaid},
		{`{"$schema": "x"}`, 0},
		{"hello", 0},
	}
	for _, test := range tests {
		if d := sniffDiagram([]byte(test.src)); d != test.exp {
			t.Errorf("%q: expected %v, got %v", test.src, test.exp, d)
		}
	}
}

// svgOf parses a source and renders it as SVG.
func svgOf(t *testing.T, src string, strict bool, opts ...RenderOption) (string, Report, error) {
	t.Helper()
	c, _, err := Parse([]byte(src), WithStrict(strict))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	report, err := c.SVG(t.Context(), &b, append([]RenderOption{WithLocation(time.UTC)}, opts...)...)
	return b.String(), report, err
}

func TestDiagrams(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"digraph { a [label=\"First\"]; a -> b [label=\"goes\"]; subgraph cluster_x { label=\"Box\"; b } }",
		"flowchart LR\n  a[First] -->|goes| b\n  subgraph x [Box]\n    b\n  end",
	} {
		s, report, err := svgOf(t, src, true)
		if err != nil {
			t.Fatalf("%q: %v (%v)", src, err, report)
		}
		for _, want := range []string{">First<", ">goes<", ">Box<", ">b<"} {
			if !strings.Contains(s, want) {
				t.Errorf("%q: the SVG has no %s", src, want)
			}
		}
	}
}

func TestIcons(t *testing.T) {
	t.Parallel()
	src := "flowchart LR\n  a@{ icon: \"fa:user\", label: \"User\" } --> b@{ icon: \"database\", form: square } --> c[fa:fa-gear Gear]"
	s, report, err := svgOf(t, src, true, WithIcons("fa", fa.Pack()))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if !strings.Contains(s, "<metadata>Icons: Font Awesome Free") {
		t.Error("the SVG names no credit of Font Awesome")
	}
	// The icon of a pack draws as a path, and the label of the inline icon
	// stays.
	if !strings.Contains(s, ">Gear<") || strings.Contains(s, "fa-gear") {
		t.Error("the inline icon is not drawn apart from its label")
	}
	// Without the pack, the icon is missing: an error in the strict mode,
	// and a line of the report in the loose mode.
	if _, _, err := svgOf(t, src, true); !errors.Is(err, icon.ErrMissing) {
		t.Errorf("expected ErrMissing in the strict mode, got %v", err)
	}
	_, report, err = svgOf(t, src, false)
	if err != nil || len(report) != 2 {
		t.Errorf("expected 2 lines in the report, got %v, %v", report, err)
	}
}

func TestIconLoader(t *testing.T) {
	t.Parallel()
	calls := 0
	loader := IconLoaderFunc(func(_ context.Context, req IconRequest) (io.ReadCloser, error) {
		calls++
		if req.Prefix != "aws" || req.Name != "lambda" {
			return nil, icon.ErrMissing
		}
		svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" onload="x()"><script>x()</script>` +
			`<rect width="10" height="10" fill="#f80"/><image href="https://example.com/a.png"/></svg>`
		return io.NopCloser(strings.NewReader(svg)), nil
	})
	src := "flowchart LR\n  a@{ icon: \"aws:lambda\" } --> b@{ icon: \"aws:lambda\" }"
	s, report, err := svgOf(t, src, true, WithIconLoader(loader))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if calls != 1 {
		t.Errorf("expected 1 call of the loader, got %d", calls)
	}
	// The icon of several colors draws as an image, written once in the
	// definitions and used twice.
	if n := strings.Count(s, "<use "); n != 2 || strings.Count(s, "data:image/svg+xml") != 1 {
		t.Errorf("expected 2 uses of 1 image, got %d uses", n)
	}
	if strings.Contains(s, "example.com") {
		t.Error("the SVG holds a URL from outside")
	}
	// FSIconLoader reads an SVG, and else a pack.
	fsys := fstest.MapFS{
		"k/one.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><path fill="currentColor" d="M0 0h4v4z"/></svg>`)},
		"p.json":    {Data: []byte(`{"prefix":"p","icons":{"two":{"body":"<path fill=\"currentColor\" d=\"M0 0h1v1z\"/>"}}}`)},
	}
	src = "flowchart LR\n  a@{ icon: \"k:one\" } --> b@{ icon: \"p:two\" }"
	if _, report, err := svgOf(t, src, true, WithIconLoader(FSIconLoader(fsys))); err != nil {
		t.Errorf("%v (%v)", err, report)
	}
}

func TestImageNode(t *testing.T) {
	t.Parallel()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		img.Set(x, 0, color.NRGBA{R: 255, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"a.png": {Data: b.Bytes()}}
	src := "flowchart LR\n  a@{ img: \"a.png\", label: \"Pic\", w: 40, constraint: on } --> b"
	s, report, err := svgOf(t, src, true, WithLoader(FSLoader(fsys)))
	if err != nil {
		t.Fatalf("%v (%v)", err, report)
	}
	if !strings.Contains(s, `width="40" height="20" href="data:image/png;base64,`) {
		t.Error("the image does not draw at 40 by 20")
	}
	// With no loader, the node draws a box with a question mark.
	if _, report, err := svgOf(t, src, false); err != nil || len(report) != 1 {
		t.Errorf("expected 1 line in the report, got %v, %v", report, err)
	}
}
