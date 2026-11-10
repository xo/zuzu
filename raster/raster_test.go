package raster

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
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

// TestDrawMatchesOkage draws each test scene without its text directly and
// through SVG and okage, and checks that at most 1 percent of the pixels
// differ, the gate of okage D31 for a file with text (D11).
func TestDrawMatchesOkage(t *testing.T) {
	t.Parallel()
	r := okagesvg.New(okagesvg.WithLoadSystemFonts(false))
	for name, s := range scenes(t) {
		bare := withoutText(s)
		var b bytes.Buffer
		if err := svg.Write(&b, bare); err != nil {
			t.Fatal(err)
		}
		want, err := r.Render(b.Bytes())
		if err != nil {
			t.Fatalf("%s: okage: %v", name, err)
		}
		got, err := Draw(bare)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Bounds() != want.Bounds() {
			t.Errorf("%s: the size is %v, want %v", name, got.Bounds(), want.Bounds())
			continue
		}
		if d := differ(got, want); d > 0.01 {
			t.Errorf("%s: %.2f%% of the pixels differ, more than 1%%", name, 100*d)
		}
	}
}

// textRenderer counts the text items that it draws.
type textRenderer struct {
	n int
}

func (r *textRenderer) DrawText(_ *okage.Pixmap, _ *scene.Item, _ okage.Transform) error {
	r.n++
	return nil
}

func TestDrawText(t *testing.T) {
	t.Parallel()
	s := &scene.Scene{Width: 10, Height: 10, Root: scene.Mark{Type: scene.Group, Items: []scene.Item{{
		Items: []scene.Mark{{Type: scene.Text, Items: []scene.Item{{Text: scene.Lines{"a"}}, {Text: scene.Lines{"b"}}}}},
	}}}}
	if _, err := Draw(s); !errors.Is(err, ErrText) {
		t.Errorf("a text with no text renderer gives %v, want %v", err, ErrText)
	}
	tr := &textRenderer{}
	if _, err := Draw(s, WithText(tr)); err != nil {
		t.Fatal(err)
	}
	if tr.n != 2 {
		t.Errorf("the text renderer drew %d items, want 2", tr.n)
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
