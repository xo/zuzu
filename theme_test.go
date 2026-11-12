package zuzu

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// TestMermaidTheme draws the theme of a Mermaid diagram (D32): a theme with
// dark in its name selects the dark mode, each other theme the scheme with
// its colors, and a mode or a scheme of the caller wins over the theme.
func TestMermaidTheme(t *testing.T) {
	t.Parallel()
	forest := theme.New(theme.Light, lookup(t, "mermaid-forest"))
	flow := func(name string) string {
		return "---\nconfig:\n  theme: " + name + "\n---\nflowchart LR\n  a --> b\n"
	}
	pie := "%%{init: {'theme': 'neutral'}}%%\npie\n  \"a\" : 2\n  \"b\" : 1\n"
	dark, light := `fill="#10141b"`, `fill="#ffffff"`
	for _, c := range []struct {
		name string
		src  string
		opts []RenderOption
		want []string
		not  []string
	}{
		{"forest", flow("forest"), nil, []string{light, `stroke="` + forest.Edge + `"`, `stroke="` + forest.Category[0] + `"`}, nil},
		{"dark", flow("dark"), nil, []string{dark}, nil},
		{"redux-dark-color", flow("redux-dark-color"), nil, []string{dark}, nil},
		{"the caller's mode", flow("neo-dark"), []RenderOption{WithMode(theme.Light)}, []string{light}, []string{dark}},
		{"the caller's scheme", flow("forest"), []RenderOption{WithScheme(theme.Zuzu)}, []string{`stroke="#5b6ee1"`}, []string{forest.Category[0]}},
		{"forest in the dark mode", flow("forest"), []RenderOption{WithMode(theme.Dark)}, []string{dark, theme.New(theme.Dark, lookup(t, "mermaid-forest")).Category[0]}, nil},
		{"a chart", pie, nil, []string{`fill="#777777"`}, nil},
	} {
		s, report, err := svgOf(t, c.src, true, c.opts...)
		if err != nil || len(report) != 0 {
			t.Errorf("%s: %v %v", c.name, err, report)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(s, w) {
				t.Errorf("%s: the SVG has no %s", c.name, w)
			}
		}
		for _, w := range c.not {
			if strings.Contains(s, w) {
				t.Errorf("%s: the SVG has %s", c.name, w)
			}
		}
	}
	// A theme that Mermaid does not have stays an issue.
	if _, _, err := Parse([]byte(flow("sepia")), WithStrict(true)); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("the theme sepia: got %v", err)
	}
}

// TestTheLookOfTheConfig draws a spec of Vega and of Vega-Lite that asks
// for a mode and a scheme in its config, and checks a value that zuzu does
// not have in both feature modes.
func TestTheLookOfTheConfig(t *testing.T) {
	t.Parallel()
	vg := `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 10, "height": 10,
	"config": {"zuzu.mode": "dark", "zuzu.scheme": "ember"},
	"marks": [{"type": "rect", "encode": {"enter": {"x": {"value": 0}, "y": {"value": 0}, "width": {"value": 5}, "height": {"value": 5}}}}]}`
	vl := `{"$schema": "https://vega.github.io/schema/vega-lite/v6.json", "config": {"zuzu.mode": "dark", "zuzu.scheme": "ember"},
	"data": {"values": [{"a": 1}]}, "mark": "bar", "encoding": {"y": {"field": "a", "type": "quantitative"}}}`
	ember := theme.New(theme.Dark, theme.Ember).Category[0]
	for _, src := range []string{vg, vl} {
		s, report, err := svgOf(t, src, true)
		if err != nil || len(report) != 0 {
			t.Fatalf("%v %v", err, report)
		}
		if !strings.Contains(s, `fill="#10141b"`) || !strings.Contains(s, ember) {
			t.Errorf("expected the dark mode and the scheme ember:\n%s", s)
		}
		if s, _, _ = svgOf(t, src, true, WithMode(theme.Light), WithScheme(theme.Zuzu)); strings.Contains(s, `fill="#10141b"`) || strings.Contains(s, ember) {
			t.Errorf("expected the look of the caller:\n%s", s)
		}
	}
	bad := strings.Replace(vg, `"ember"`, `"sepia"`, 1)
	if _, _, err := svgOf(t, bad, true); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("the scheme sepia in the strict mode: got %v", err)
	}
	s, report, err := svgOf(t, bad, false)
	if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "config.zuzu.scheme") || !strings.Contains(s, `fill="#10141b"`) {
		t.Errorf("the scheme sepia in the loose mode: %v %v", err, report)
	}
	bad = strings.Replace(vg, `"dark"`, `"dim"`, 1)
	if _, report, err := svgOf(t, bad, false); err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "config.zuzu.mode") {
		t.Errorf("the mode dim in the loose mode: %v %v", err, report)
	}
}

// lookup returns a scheme by its name.
func lookup(t *testing.T, name string) theme.Scheme {
	t.Helper()
	s, ok := theme.Lookup(name)
	if !ok {
		t.Fatalf("no scheme %s", name)
	}
	return s
}
