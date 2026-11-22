package zuzu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
)

// TestUsecaseRotate checks that colorScheme rotate draws each actor and
// use case, and each system boundary, in a color of the category scheme of
// the theme of the render, so that the first ten actors take the ten
// colors (D37, D56). A node fills with the solid tint of its color (D55).
func TestUsecaseRotate(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("---\nconfig:\n  usecase:\n    colorScheme: rotate\n---\nusecase-beta\n")
	for i := range 11 {
		fmt.Fprintf(&b, "actor A%d\n", i)
	}
	b.WriteString("systemBoundary S\n  U\nend\n")
	for _, test := range []struct {
		name string
		opts []RenderOption
		th   theme.Theme
	}{
		// A source with no theme takes the zuzu scheme (D55).
		{"default", nil, theme.New(theme.Light, theme.Zuzu)},
		{"dark", []RenderOption{WithMode(theme.Dark)}, theme.New(theme.Dark, theme.Zuzu)},
		{"zuzu", []RenderOption{WithScheme(theme.Zuzu)}, theme.New(theme.Light, theme.Zuzu)},
	} {
		s, report, err := svgOf(t, b.String(), true, test.opts...)
		if err != nil {
			t.Fatalf("%s: %v (%v)", test.name, err, report)
		}
		c := test.th.Category
		// The first ten actors take the ten colors, each by the hash of
		// its id or the next free color. The boundary S takes the color
		// of its hash, 3 modulo 9, the fifth.
		for i, want := range c {
			paint := fmt.Sprintf(`fill="%s" stroke="%s" stroke-width="1.25"`, test.th.Tint(want), want)
			if !strings.Contains(s, paint) {
				t.Errorf("%s: %d: the SVG has no node in %s", test.name, i, want)
			}
		}
		if n := strings.Count(s, `rx="6" fill="`+test.th.Tint(c[4])+`" stroke="`+c[4]+`"`); n != 1 {
			t.Errorf("%s: expected the boundary in %s, got %d", test.name, c[4], n)
		}
	}
	// With role, or in a theme that is not a color theme of Mermaid,
	// every node draws in the first color, as before.
	for _, src := range []string{
		strings.Replace(b.String(), "rotate", "role", 1),
		strings.Replace(b.String(), "config:\n", "config:\n  theme: forest\n", 1),
	} {
		s, report, err := svgOf(t, src, true)
		if err != nil {
			t.Fatalf("%v (%v)", err, report)
		}
		sc := theme.Zuzu
		if strings.Contains(src, "forest") {
			sc, _ = theme.Lookup("mermaid-forest")
		}
		th := theme.New(theme.Light, sc)
		c := th.Category
		if strings.Contains(s, `fill="`+c[1]+`"`) || strings.Contains(s, `fill="`+th.Tint(c[1])+`"`) {
			t.Errorf("%q: a node draws in the second color %s", src, c[1])
		}
	}
}
