package zuzu

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	stdcolor "image/color"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// schemeScale runs a spec with one scale of colors, color, and returns it.
func schemeScale(t *testing.T, src string, opts ...RenderOption) scale.Scale {
	t.Helper()
	c, _, err := Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := c.NewView(t.Context(), append([]RenderOption{WithLocation(time.UTC)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	s, err := v.Scale("color")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// colorSpec returns a spec with a scale of colors, color, of a type, a
// domain and a range, and more keys of the top level.
func colorSpec(typ, domain, rng, more string) string {
	return `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 10, "height": 10,` + more +
		`"scales": [{"name": "color", "type": "` + typ + `", "domain": ` + domain + `, "range": ` + rng + `}]}`
}

// schemeCases are the colors of the named schemes of vega.js, and what
// some scales with a scheme map values to, which test/oracle/schemes.mjs
// writes.
type schemeCases struct {
	Schemes map[string]struct {
		Colors []string            `json:"colors"`
		At     [][2]jsontext.Value `json:"at"`
	} `json:"schemes"`
	Scales []struct {
		Scale   map[string]any `json:"scale"`
		Inputs  []any          `json:"inputs"`
		Outputs []string       `json:"outputs"`
	} `json:"scales"`
}

// readSchemeCases reads testdata/schemes.json.
func readSchemeCases(t *testing.T) schemeCases {
	t.Helper()
	b, err := os.ReadFile("testdata/schemes.json")
	if err != nil {
		t.Fatal(err)
	}
	var c schemeCases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// sameColor reports whether two colors of CSS are the same color.
func sameColor(a, b string) bool {
	x, ok1 := color.Parse(a)
	y, ok2 := color.Parse(b)
	return ok1 && ok2 && x.Hex() == y.Hex()
}

// TestTheNamedSchemesAreThoseOfVega checks that zuzu has each scheme of
// vega.js and no other, and that each one has the colors of vega.js: each
// color of a discrete scheme, and the color of a continuous scheme at some
// points (D46).
func TestTheNamedSchemesAreThoseOfVega(t *testing.T) {
	t.Parallel()
	c := readSchemeCases(t)
	for name, want := range c.Schemes {
		got, ok := scale.NamedScheme(strings.ToUpper(name))
		switch {
		case !ok:
			t.Errorf("%s: zuzu does not have the scheme", name)
			continue
		case got.Continuous != (want.At != nil):
			t.Errorf("%s: continuous is %v, want %v", name, got.Continuous, want.At != nil)
			continue
		}
		if !got.Continuous {
			if !slices.EqualFunc(got.Colors, want.Colors, sameColor) {
				t.Errorf("%s: the colors are %v, want %v", name, got.Colors, want.Colors)
			}
			continue
		}
		for _, p := range want.At {
			var at float64
			var col string
			if json.Unmarshal(p[0], &at) != nil || json.Unmarshal(p[1], &col) != nil {
				t.Fatalf("%s: the point %s", name, p)
			}
			if g := got.At(at); !sameColor(g, col) {
				t.Errorf("%s: at %v the color is %s, want %s", name, at, g, col)
			}
		}
	}
	for _, name := range []string{"tableau10", "viridis", "redblue", "rainbow", "darkmulti"} {
		if _, ok := c.Schemes[name]; !ok {
			t.Errorf("vega.js does not have %s", name)
		}
	}
}

// TestASchemeMapsAsInVega checks that a scale with a scheme maps values to
// the colors of vega.js, in both modes: a sequential or a diverging scale
// through the interpolation of the scheme, with its extent and reverse,
// and a discrete scale through the colors that it takes of the scheme
// (D46).
func TestASchemeMapsAsInVega(t *testing.T) {
	t.Parallel()
	for _, c := range readSchemeCases(t).Scales {
		sc := maps.Clone(c.Scale)
		sc["name"] = "color"
		def, err := json.Marshal(map[string]any{
			"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 10, "height": 10, "scales": []any{sc},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range []theme.Mode{theme.Light, theme.Dark} {
			s := schemeScale(t, string(def), WithMode(m))
			for i, v := range c.Inputs {
				if got := expr.String(s.Map(v)); !sameColor(got, c.Outputs[i]) {
					t.Errorf("%s, %s: %v maps to %s, want %s", c.Scale, m, v, got, c.Outputs[i])
				}
			}
		}
	}
}

// TestAnUnknownSchemeIsAnIssue checks that a scheme name that Vega does not
// have is an error in the strict mode, and in the loose mode an issue with
// the colors of the theme.
func TestAnUnknownSchemeIsAnIssue(t *testing.T) {
	t.Parallel()
	// The legend builds the scale while the view runs.
	src := colorSpec("ordinal", `["a", "b"]`, `{"scheme": "nosuchscheme"}`, `"legends": [{"fill": "color"}],`)
	c, _, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	v, report, err := c.NewView(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(report, func(is vega.Issue) bool { return errors.Is(is.Err, vega.ErrInvalidSpec) }) {
		t.Errorf("the report %v names no invalid scheme", report)
	}
	s, err := v.Scale("color")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Map("b"), theme.New(theme.Light, theme.Zuzu).Category[1]; got != want {
		t.Errorf("b maps to %v, want the color of the theme %s", got, want)
	}
	c, _, err = Parse([]byte(src), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	v, _, err = c.NewView(t.Context())
	if err == nil {
		_, err = v.Scale("color")
	}
	if !errors.Is(err, vega.ErrInvalidSpec) {
		t.Errorf("the strict mode gives %v, want %v", err, vega.ErrInvalidSpec)
	}
}

// TestANamedRangeKeepsTheTheme checks that a scale that names no scheme
// takes the colors of the theme, and that a scheme in the range of the
// config of the spec draws in its own colors (D46).
func TestANamedRangeKeepsTheTheme(t *testing.T) {
	t.Parallel()
	th := theme.New(theme.Dark, theme.Zuzu)
	s := schemeScale(t, colorSpec("ordinal", `["a", "b"]`, `"category"`, ""), WithMode(theme.Dark))
	if got := s.Map("b"); got != th.Category[1] {
		t.Errorf("category maps b to %v, want %s", got, th.Category[1])
	}
	s = schemeScale(t, colorSpec("linear", "[0, 1]", `"diverging"`, ""), WithMode(theme.Dark))
	if got := s.Map(0.0); got != th.Down {
		t.Errorf("diverging maps 0 to %v, want %s", got, th.Down)
	}
	s = schemeScale(t, colorSpec("ordinal", `["a", "b"]`, `"category"`, `"config": {"range": {"category": {"scheme": "category10"}}},`))
	if got := s.Map("b"); got != "#ff7f0e" {
		t.Errorf("a config of category10 maps b to %v, want #ff7f0e", got)
	}
}

// TestARampStartsFromTheBackgroundOfTheChart checks that a sequential ramp
// starts from the background of the spec or of its config, and that the
// background of the caller wins over both (D35).
func TestARampStartsFromTheBackgroundOfTheChart(t *testing.T) {
	t.Parallel()
	start := func(bg string) string {
		th := theme.New(theme.Light, theme.Zuzu)
		th.Background = bg
		return th.Sequential(0)
	}
	for _, c := range []struct {
		name, more string
		opts       []RenderOption
		want       string
	}{
		{"no background", "", nil, start("#ffffff")},
		{"the background of the spec", `"background": "#111",`, nil, start("#111111")},
		{"a named background", `"background": "black",`, nil, start("#000000")},
		{"a background of a signal", `"background": {"signal": "'#222431'"},`, nil, start("#222431")},
		{"the background of the config", `"config": {"background": "#dae3f3"},`, nil, start("#dae3f3")},
		{"a transparent background", `"background": "transparent",`, nil, start("#ffffff")},
		{"half of black", `"background": "rgba(0, 0, 0, 0.5)",`, nil, start("#808080")},
		{"the caller", `"background": "#111",`, []RenderOption{WithBackground(stdcolor.RGBA{R: 255, G: 255, A: 255})}, start("#ffff00")},
	} {
		s := schemeScale(t, colorSpec("linear", "[0, 1]", `"ramp"`, c.more), c.opts...)
		if got := s.Map(0.0); got != c.want {
			t.Errorf("%s: the ramp starts at %v, want %s", c.name, got, c.want)
		}
	}
}
