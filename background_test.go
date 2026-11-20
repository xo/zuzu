package zuzu

import (
	"errors"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// pageFill finds the fill of the background of an SVG, or empty for an SVG
// with no background.
var pageFill = regexp.MustCompile(`^<svg[^>]*>(?:<[^r][^>]*>)*<rect width="[^"]+" height="[^"]+" fill="([^"]+)"`)

// TestTheBackgroundOfTheConfig draws the bgcolor of a graph of DOT, which
// the spec carries as zuzu.background (D37). It fills the page and is the
// background of the theme, under the label of an edge and in an open
// arrowhead. The mode follows its lightness, over the mode of the caller.
// A background of the caller wins, and the mode is then that of the
// caller.
func TestTheBackgroundOfTheConfig(t *testing.T) {
	t.Parallel()
	light, dark := theme.New(theme.Light, theme.Zuzu).Text, theme.New(theme.Dark, theme.Zuzu).Text
	dot := func(bg string) string {
		return `digraph { bgcolor="` + bg + `"; a -> b [label="goes", arrowhead=empty] }`
	}
	tests := []struct {
		name string
		src  string
		opts []RenderOption
		// page is the fill of the page, and text the color of the text.
		page, text string
		// boxes is the least count of the fill of the page: the page,
		// the box under the label and the open arrowhead.
		boxes int
	}{
		{"a dark bgcolor", dot("#1e1e2e"), nil, "#1e1e2e", dark, 3},
		{"a dark bgcolor and a light caller", dot("#1e1e2e"), []RenderOption{WithMode(theme.Light)}, "#1e1e2e", dark, 3},
		{"a light bgcolor", dot("lightyellow"), nil, "#ffffe0", light, 3},
		{"a light bgcolor and a dark caller", dot("lightyellow"), []RenderOption{WithMode(theme.Dark)}, "#ffffe0", light, 3},
		{"a bgcolor of a scheme", dot("/blues9/9"), nil, "#08306b", dark, 3},
		{
			"a background of the caller", dot("#1e1e2e"),
			[]RenderOption{WithBackground(color.NRGBA{R: 0xfa, G: 0xfa, B: 0xf0, A: 255})}, "#fafaf0", light, 3,
		},
		{
			"a background and a mode of the caller", dot("lightyellow"),
			[]RenderOption{WithBackground(color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 255}), WithMode(theme.Dark)}, "#202020", dark, 3,
		},
		{"a transparent bgcolor", dot("transparent"), nil, "", light, 0},
		{
			"zuzu.background over zuzu.mode",
			`{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 10, "height": 10,
			"config": {"zuzu.mode": "light", "zuzu.background": "#1e1e2e"},
			"marks": [{"type": "text", "encode": {"enter": {"text": {"value": "x"}}}}]}`,
			nil, "#1e1e2e", dark, 1,
		},
		{
			"zuzu.background of Vega-Lite",
			`{"$schema": "https://vega.github.io/schema/vega-lite/v6.json", "config": {"zuzu.background": "rgb(30, 30, 46)"},
			"data": {"values": [{"a": 1}]}, "mark": "text", "encoding": {"text": {"field": "a"}}}`,
			nil, "#1e1e2e", dark, 1,
		},
	}
	for _, test := range tests {
		s, report, err := svgOf(t, test.src, true, test.opts...)
		if err != nil || len(report) != 0 {
			t.Fatalf("%s: %v %v", test.name, err, report)
		}
		page := ""
		if m := pageFill.FindStringSubmatch(s); m != nil {
			page = m[1]
		}
		if page != test.page {
			t.Errorf("%s: expected the page %q, got %q", test.name, test.page, page)
		}
		if !strings.Contains(s, `fill="`+test.text+`"`) {
			t.Errorf("%s: expected the text %s:\n%s", test.name, test.text, s)
		}
		if n := strings.Count(s, `fill="`+test.page+`"`); test.page != "" && n < test.boxes {
			t.Errorf("%s: expected the fill %s %d times or more, got %d:\n%s", test.name, test.page, test.boxes, n, s)
		}
	}
}

// TestABackgroundThatDoesNotRead checks a zuzu.background and a bgcolor
// that zuzu cannot read: an issue in the loose mode, which keeps the look
// of the theme, and an error in the strict mode.
func TestABackgroundThatDoesNotRead(t *testing.T) {
	t.Parallel()
	vg := func(bg string) string {
		return `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 10, "height": 10,
		"config": {"zuzu.background": ` + bg + `},
		"marks": [{"type": "text", "encode": {"enter": {"text": {"value": "x"}}}}]}`
	}
	for _, src := range []string{vg(`"nocolor"`), vg(`3`), `digraph { bgcolor="red:blue"; a }`, `digraph { bgcolor=nocolor; a }`} {
		s, report, err := svgOf(t, src, false)
		if err != nil || len(report) != 1 || !strings.Contains(report[0].Error(), "background") && !strings.Contains(report[0].Error(), "bgcolor") {
			t.Errorf("%s: expected one issue, got %v %v", src, err, report)
		}
		if m := pageFill.FindStringSubmatch(s); m == nil || m[1] != "#ffffff" {
			t.Errorf("%s: expected the background of the theme:\n%s", src, s)
		}
		if strings.HasPrefix(src, "{") {
			if _, _, err := svgOf(t, src, true); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("%s: expected an error in the strict mode, got %v", src, err)
			}
		} else if _, _, err := Parse([]byte(src), WithStrict(true)); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%s: expected an error in the strict mode, got %v", src, err)
		}
	}
}

// TestTheBackgroundIsNotAVegaConfig checks that zuzu.background does not go
// into the Vega config of the render, as zuzu.mode and zuzu.scheme do not.
func TestTheBackgroundIsNotAVegaConfig(t *testing.T) {
	t.Parallel()
	c, _, err := Parse([]byte(`digraph { bgcolor=navy; a }`))
	if err != nil {
		t.Fatal(err)
	}
	rt, err := newRuntime(t.Context(), c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.config[theme.ConfigBackground]; ok {
		t.Errorf("expected no %s in the config", theme.ConfigBackground)
	}
	if rt.theme.Background != "#000080" || rt.theme.Mode() != theme.Dark {
		t.Errorf("expected the dark theme on #000080, got %s in %v", rt.theme.Background, rt.theme.Mode())
	}
}
