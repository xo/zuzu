package conform

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"maps"
	"slices"
	"testing"
)

// TestPNGDigest makes sure that two PNGs of the same pixels match, though
// one hides a color under a pixel with no opacity, and that a pixel of
// another color does not match.
func TestPNGDigest(t *testing.T) {
	t.Parallel()
	url := func(img image.Image) string {
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		return pngPrefix + base64.StdEncoding.EncodeToString(b.Bytes())
	}
	a := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	a.SetNRGBA(0, 0, color.NRGBA{R: 68, G: 1, B: 84, A: 255})
	a.SetNRGBA(1, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 0})
	b := image.NewRGBA(image.Rect(0, 0, 2, 1))
	b.SetRGBA(0, 0, color.RGBA{R: 68, G: 1, B: 84, A: 255})
	da, ok := pngDigest(url(a))
	if !ok {
		t.Fatal("pngDigest does not read the PNG")
	}
	if db, _ := pngDigest(url(b)); db != da {
		t.Errorf("pngDigest = %s, want %s", db, da)
	}
	b.SetRGBA(0, 0, color.RGBA{R: 68, G: 1, B: 85, A: 255})
	if db, _ := pngDigest(url(b)); db == da {
		t.Errorf("pngDigest of another pixel = %s, the same", db)
	}
	if _, ok := pngDigest("data:text/plain,png"); ok {
		t.Error("pngDigest reads a URL that holds no PNG")
	}
}

// TestPaddedScales makes sure that only a linear, a time or a utc scale
// with a padding, in a spec with the autosize fit, has a domain that
// depends on the layout of the text.
func TestPaddedScales(t *testing.T) {
	t.Parallel()
	scales := `"scales": [
		{"name": "x", "type": "time", "padding": 24},
		{"name": "y", "type": "linear"},
		{"name": "b", "type": "band", "padding": 0.1}
	]`
	for _, c := range []struct {
		autosize string
		want     []string
	}{
		{`"fit"`, []string{"x"}},
		{`{"type": "fit-x", "contains": "padding"}`, []string{"x"}},
		{`"pad"`, nil},
	} {
		got := slices.Sorted(maps.Keys(paddedScales([]byte(`{"autosize": ` + c.autosize + `, ` + scales + `}`))))
		if !slices.Equal(got, c.want) {
			t.Errorf("autosize %s: paddedScales = %v, want %v", c.autosize, got, c.want)
		}
	}
}

// The exit of phase 3, as docs/PLAN.md states it, in percent.
const (
	minSpecs    = 80
	minDatasets = 85
	minScales   = 90
	minMarks    = 90
)

// The exit of phase 4, as docs/PLAN.md states it, in percent.
const (
	minVLSpecs     = 60
	minVLDatasets  = 70
	minVLScales    = 80
	minVLMarks     = 75
	minVLStructure = 50
)

// TestPhase4Share checks that zuzu compiles the Vega-Lite specs itself
// and that the share of the specs, the datasets, the scales, the marks and
// the compiled structures that match the references stays at or above the
// exit of phase 4. It skips without the references, as TestPhase3Share
// does.
func TestPhase4Share(t *testing.T) {
	t.Parallel()
	s, err := Summarize(t.Context(), "../testdata", "", true)
	if errors.Is(err, ErrNoReferences) {
		t.Skip("no references: run the oracle, as test/README.md says")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		n, all  int
		minimum float64
	}{
		{"specs", s.SpecsOK, s.Specs, minVLSpecs},
		{"datasets", s.DatasetsOK, s.Datasets, minVLDatasets},
		{"scales", s.ScalesOK, s.Scales, minVLScales},
		{"marks", s.MarksOK, s.Marks, minVLMarks},
		{"structure", s.Structure, s.Specs, minVLStructure},
	} {
		share := Share(c.n, c.all)
		t.Logf("%s: %d of %d match (%.1f%%)", c.name, c.n, c.all, share)
		if share < c.minimum {
			t.Errorf("%s: %.1f%% match, below the %v%% of phase 4", c.name, share, c.minimum)
		}
	}
	if s.Errors > 0 {
		t.Errorf("%d specs did not run", s.Errors)
	}
}

// TestPhase3Share checks that the share of the specs, the datasets, the
// scales and the marks that match the references stays at or above the
// exit of phase 3. It needs the references, which the oracle writes, so it
// skips without them. The conformance job of CI runs the oracle first.
func TestPhase3Share(t *testing.T) {
	t.Parallel()
	s, err := Summarize(t.Context(), "../testdata", "", false)
	if errors.Is(err, ErrNoReferences) {
		t.Skip("no references: run the oracle, as test/README.md says")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		n, all  int
		minimum float64
	}{
		{"specs", s.SpecsOK, s.Specs, minSpecs},
		{"datasets", s.DatasetsOK, s.Datasets, minDatasets},
		{"scales", s.ScalesOK, s.Scales, minScales},
		{"marks", s.MarksOK, s.Marks, minMarks},
	} {
		share := Share(c.n, c.all)
		t.Logf("%s: %d of %d match (%.1f%%)", c.name, c.n, c.all, share)
		if share < c.minimum {
			t.Errorf("%s: %.1f%% match, below the %v%% of phase 3", c.name, share, c.minimum)
		}
	}
	if s.Errors > 0 {
		t.Errorf("%d specs did not run", s.Errors)
	}
}
