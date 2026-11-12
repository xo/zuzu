package raster

import (
	"bytes"
	"errors"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/text"
)

// scenes reads the test scenes of the SVG writer: the marks, and the tier 0
// targets of D13.
func scenes(t *testing.T) map[string]*scene.Scene {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.json", filepath.Join("targets", "*.json")} {
		m, err := filepath.Glob(filepath.Join("..", "svg", "testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("found no test scene")
	}
	out := map[string]*scene.Scene{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s, err := scene.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = s
	}
	return out
}

// withoutText returns a copy of a scene with no text mark.
func withoutText(s *scene.Scene) *scene.Scene {
	out := *s
	out.Root = stripText(s.Root)
	return &out
}

func stripText(m scene.Mark) scene.Mark {
	if m.Type != scene.Group {
		return m
	}
	items := make([]scene.Item, len(m.Items))
	for i, it := range m.Items {
		marks := make([]scene.Mark, 0, len(it.Items))
		for _, c := range it.Items {
			if c.Type != scene.Text {
				marks = append(marks, stripText(c))
			}
		}
		it.Items = marks
		items[i] = it
	}
	m.Items = items
	return m
}

// differ returns the share of the pixels that differ, by the metric of
// okage D31: both images demultiplied, and a pixel differs when a channel
// differs by more than 1 or when only one image has it.
func differ(a, b *image.RGBA) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
	n := 0
	for y := range h {
		for x := range w {
			p := image.Pt(x, y)
			if !p.In(ab) || !p.In(bb) {
				n++
				continue
			}
			pa, pb := demultiply(a, x, y), demultiply(b, x, y)
			for c := range 4 {
				if d := pa[c] - pb[c]; d > 1 || d < -1 {
					n++
					break
				}
			}
		}
	}
	return float64(n) / float64(w*h)
}

func demultiply(img *image.RGBA, x, y int) [4]int {
	i := img.PixOffset(x, y)
	p := img.Pix[i : i+4]
	a := int(p[3])
	if a == 0 {
		return [4]int{}
	}
	f := func(c uint8) int { return (int(c)*255 + a/2) / a }
	return [4]int{f(p[0]), f(p[1]), f(p[2]), a}
}

// fonts returns a renderer of okage with only the bundled fonts, so that a
// test draws the same text on every machine.
func fonts() *okagesvg.Renderer {
	return okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
}

// TestDrawMatchesOkage draws each test scene directly and through SVG and
// okage, with and without its text, and checks that at most 1 percent of
// the pixels differ, the gate of okage D31 for a file with text (D11).
func TestDrawMatchesOkage(t *testing.T) {
	t.Parallel()
	r := fonts()
	for name, s := range scenes(t) {
		for _, c := range []struct {
			kind string
			s    *scene.Scene
		}{{"with text", s}, {"without text", withoutText(s)}} {
			var b bytes.Buffer
			if err := svg.Write(&b, c.s); err != nil {
				t.Fatal(err)
			}
			want, err := r.Render(b.Bytes())
			if err != nil {
				t.Fatalf("%s %s: okage: %v", name, c.kind, err)
			}
			got, err := Draw(c.s, WithRenderer(r))
			if err != nil {
				t.Fatalf("%s %s: %v", name, c.kind, err)
			}
			if got.Bounds() != want.Bounds() {
				t.Errorf("%s %s: the size is %v, want %v", name, c.kind, got.Bounds(), want.Bounds())
				continue
			}
			if d := differ(got, want); d > 0.01 {
				t.Errorf("%s %s: %.2f%% of the pixels differ, more than 1%%", name, c.kind, 100*d)
			}
		}
	}
}

// TestDrawText checks that a text draws, with the renderer of the option or
// with a renderer of its own.
func TestDrawText(t *testing.T) {
	t.Parallel()
	s := &scene.Scene{Width: 40, Height: 20, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{
		Items: []scene.Mark{{Type: scene.Text, Items: []scene.Item{{X: 2, Y: 15, Text: scene.Lines{"Ab"}, FontSize: 14}}}},
	}}}}
	for _, opts := range [][]Option{{WithRenderer(fonts())}, nil} {
		img, err := Draw(s, opts...)
		if err != nil {
			t.Fatal(err)
		}
		inked := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				inked++
			}
		}
		if inked == 0 {
			t.Errorf("the text with %d options draws nothing", len(opts))
		}
	}
}

// TestDrawItalic checks that a text in the italic style draws with the
// bundled italic face, and not with the upright face.
func TestDrawItalic(t *testing.T) {
	t.Parallel()
	draw := func(style string, r *okagesvg.Renderer) *image.RGBA {
		s := &scene.Scene{Width: 160, Height: 30, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{
			Items: []scene.Mark{{Type: scene.Text, Items: []scene.Item{{X: 4, Y: 20, Text: scene.Lines{"Italic flag"}, FontSize: 16, FontStyle: style}}}},
		}}}}
		img, err := Draw(s, WithRenderer(r))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	if d := differ(draw("italic", fonts()), draw("", fonts())); d < 0.01 {
		t.Errorf("the italic text differs from the upright text in only %.4f of the pixels", d)
	}
	// Without the italic faces, okage draws the text upright or slants the
	// upright face. Either differs from the italic face.
	upright := fstest.MapFS{}
	for _, f := range []string{"IBMPlexSans-Regular.ttf", "IBMPlexSans-Medium.ttf", "IBMPlexSans-SemiBold.ttf"} {
		b, err := fs.ReadFile(text.Fonts(), "fonts/"+f)
		if err != nil {
			t.Fatal(err)
		}
		upright[f] = &fstest.MapFile{Data: b}
	}
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(upright),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family))
	if d := differ(draw("italic", fonts()), draw("italic", r)); d < 0.01 {
		t.Errorf("the italic text with the italic faces differs from the italic text without them in only %.4f of the pixels", d)
	}
}

func TestDrawRefusesWhatItCannotDraw(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		m    scene.Mark
		want error
	}{
		{scene.Mark{Type: "nosuch", Items: []scene.Item{{}}}, ErrUnsupportedMark},
		{scene.Mark{Type: scene.Line, Items: []scene.Item{{Interpolate: "nosuch"}}}, ErrUnsupportedCurve},
		{scene.Mark{Type: scene.Symbol, Items: []scene.Item{{Shape: "nosuch"}}}, ErrUnsupportedShape},
	} {
		s := &scene.Scene{Width: 10, Height: 10, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{Items: []scene.Mark{c.m}}}}}
		if _, err := Draw(s); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.m.Type, err, c.want)
		}
	}
}

// TestDrawMarkClip checks that a mark that clips draws inside the box of
// its group, as the SVG writer clips it.
func TestDrawMarkClip(t *testing.T) {
	t.Parallel()
	s := &scene.Scene{Width: 40, Height: 20, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{
		Width: 20, Height: 20,
		Items: []scene.Mark{{Type: scene.Rect, Clip: true, Items: []scene.Item{{Width: 40, Height: 20, Fill: scene.Paint{Color: "#000000"}}}}},
	}}}}
	img, err := Draw(s, WithRenderer(fonts()))
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(10, 10).A == 0 {
		t.Error("the rect draws nothing inside its group")
	}
	if img.RGBAAt(30, 10).A != 0 {
		t.Error("the rect draws outside its group")
	}
	var b bytes.Buffer
	if err := svg.Write(&b, s); err != nil {
		t.Fatal(err)
	}
	want, err := fonts().Render(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if d := differ(img, want); d > 0.01 {
		t.Errorf("%.2f%% of the pixels differ from okage, more than 1%%", 100*d)
	}
}
