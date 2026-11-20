package zuzu

import (
	stdcolor "image/color"
	"testing"
	"time"

	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/theme"
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

// TestADivergingSchemeTakesTheDivergingRamp checks that a diverging scheme
// name of Vega takes the diverging ramp of the theme, from the color of a
// fall at the low end to the color of a rise at the high end (D35).
func TestADivergingSchemeTakesTheDivergingRamp(t *testing.T) {
	t.Parallel()
	th := theme.New(theme.Light, theme.Zuzu)
	for _, name := range []string{"pinkyellowgreen", "redblue", "spectral"} {
		s := schemeScale(t, colorSpec("linear", "[-1, 0, 1]", `{"scheme": "`+name+`"}`, ""))
		for _, c := range []struct {
			v    float64
			want string
		}{{-1, th.Down}, {0, th.Grid}, {1, th.Up}} {
			if got := s.Map(c.v); got != c.want {
				t.Errorf("%s: %v maps to %v, want %s", name, c.v, got, c.want)
			}
		}
	}
	s := schemeScale(t, colorSpec("ordinal", `["a", "b", "c"]`, `{"scheme": "redblue"}`, ""))
	if got := s.Map("b"); got != th.Grid {
		t.Errorf("an ordinal scale of redblue maps b to %v, want %s", got, th.Grid)
	}
}

// TestACyclicalSchemeTakesTheWheel checks that rainbow and sinebow take the
// wheel of the ten categorical colors, with the last joined to the first,
// and that a discrete scale spreads its colors round the wheel (D35).
func TestACyclicalSchemeTakesTheWheel(t *testing.T) {
	t.Parallel()
	th := theme.New(theme.Dark, theme.Zuzu)
	for _, name := range []string{"rainbow", "sinebow"} {
		s := schemeScale(t, colorSpec("linear", "[0, 360]", `{"scheme": "`+name+`"}`, ""), WithMode(theme.Dark))
		for i, c := range th.Category {
			if got := s.Map(float64(i) * 36); got != c {
				t.Errorf("%s: %v maps to %v, want the color %d, %s", name, i*36, got, i, c)
			}
		}
		if got := s.Map(360.0); got != th.Category[0] {
			t.Errorf("%s: 360 maps to %v, want the first color %s", name, got, th.Category[0])
		}
	}
	s := schemeScale(t, colorSpec("ordinal", `["a", "b", "c", "d", "e"]`, `{"scheme": "rainbow"}`, ""), WithMode(theme.Dark))
	for i, k := range []string{"a", "b", "c", "d", "e"} {
		if got, want := s.Map(k), th.Category[2*i]; got != want {
			t.Errorf("an ordinal scale of rainbow maps %s to %v, want %s", k, got, want)
		}
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
		s := schemeScale(t, colorSpec("linear", "[0, 1]", `{"scheme": "blues"}`, c.more), c.opts...)
		if got := s.Map(0.0); got != c.want {
			t.Errorf("%s: the ramp starts at %v, want %s", c.name, got, c.want)
		}
	}
}
