package svg

import (
	"bytes"
	"errors"
	"flag"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/scene"
)

// update writes the golden files again from the output of the writer. A
// person reviews the change before it is committed.
var update = flag.Bool("update", false, "write the golden files again")

// scenes returns the names of the test scenes, without .json, relative to
// testdata. The scenes in testdata/targets are the tier 0 targets of D13,
// which test/cmd/targets rebuilds as scene graphs.
func scenes(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.json", filepath.Join("targets", "*.json")} {
		m, err := filepath.Glob(filepath.Join("testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("found no test scene in testdata")
	}
	for i, f := range files {
		rel, err := filepath.Rel("testdata", f)
		if err != nil {
			t.Fatal(err)
		}
		files[i] = strings.TrimSuffix(rel, ".json")
	}
	return files
}

// render writes one test scene as SVG.
func render(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := scene.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestWriteMatchesTheGoldenFiles writes each test scene and compares the SVG
// with its golden file. Run the test with -update to write the golden files
// again.
func TestWriteMatchesTheGoldenFiles(t *testing.T) {
	t.Parallel()
	for _, name := range scenes(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := render(t, name)
			golden := filepath.Join("testdata", name+".svg")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v. Run the test with -update to write it", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("the SVG of %s differs from %s. Run the test with -update, and review the change", name, golden)
			}
			if again := render(t, name); !bytes.Equal(got, again) {
				t.Errorf("the SVG of %s differs between two writes", name)
			}
		})
	}
}

// TestOkageRendersEachGoldenFile renders each golden file with okage, so
// that the writer uses only the SVG that okage renders (D10).
func TestOkageRendersEachGoldenFile(t *testing.T) {
	t.Parallel()
	r := okagesvg.New(okagesvg.WithBackground(color.White))
	for _, name := range scenes(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			img, err := r.Render(render(t, name))
			if err != nil {
				t.Fatalf("okage did not render %s: %v", name, err)
			}
			if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
				t.Errorf("okage rendered %s as an empty image", name)
			}
		})
	}
}

// TestWriteRefusesWhatItCannotDraw checks the errors of a mark, a curve and a
// shape that the writer does not draw.
func TestWriteRefusesWhatItCannotDraw(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mark scene.Mark
		want error
	}{
		{scene.Mark{Type: "unknown", Items: []scene.Item{{}}}, ErrUnsupportedMark},
		{scene.Mark{Type: scene.Line, Items: []scene.Item{{Interpolate: "basis-closed"}, {X: 1}}}, ErrUnsupportedCurve},
		{scene.Mark{Type: scene.Symbol, Items: []scene.Item{{Shape: "not a path"}}}, ErrUnsupportedShape},
	} {
		err := Write(&bytes.Buffer{}, &scene.Scene{Width: 1, Height: 1, Root: c.mark})
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.mark.Type, err, c.want)
		}
	}
}

// TestLinksAndTooltipsDrawNothing renders the scene with links and tooltips
// with okage, and again without them, and checks that the images are the
// same, so that the a and title elements change nothing that draws.
func TestLinksAndTooltipsDrawNothing(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("testdata", "links.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := scene.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	var with, without bytes.Buffer
	if err := Write(&with, s); err != nil {
		t.Fatal(err)
	}
	var strip func(m *scene.Mark)
	strip = func(m *scene.Mark) {
		for i := range m.Items {
			m.Items[i].Href, m.Items[i].Tooltip = "", ""
			for j := range m.Items[i].Items {
				strip(&m.Items[i].Items[j])
			}
		}
	}
	strip(&s.Root)
	if err := Write(&without, s); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(with.Bytes(), without.Bytes()) {
		t.Fatal("the scene wrote no link and no tooltip")
	}
	if strings.Contains(with.String(), "javascript") {
		t.Errorf("the SVG holds a link with the scheme javascript:\n%s", with.String())
	}
	r := okagesvg.New(okagesvg.WithBackground(color.White))
	a, err := r.Render(with.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Render(without.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("the links and the tooltips change the image")
	}
}
