package zuzu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/theme"
)

// TestUsecaseRotate checks that colorScheme rotate draws each actor and
// use case, and each system boundary, as a node of the next color of the
// category scheme of the theme of the render, and that the colors start
// again after the last one (D37).
func TestUsecaseRotate(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("---\nconfig:\n  usecase:\n    colorScheme: rotate\n---\nusecase-beta\n")
	for i := range 11 {
		fmt.Fprintf(&b, "actor A%d\n", i)
	}
	b.WriteString("systemBoundary S\n  U\nend\n")
	redux, ok := theme.Lookup("mermaid-redux-color")
	if !ok {
		t.Fatal("no scheme mermaid-redux-color")
	}
	for _, test := range []struct {
		name string
		opts []RenderOption
		th   theme.Theme
	}{
		{"default", nil, theme.New(theme.Light, redux)},
		{"dark", []RenderOption{WithMode(theme.Dark)}, theme.New(theme.Dark, redux)},
		{"zuzu", []RenderOption{WithScheme(theme.Zuzu)}, theme.New(theme.Light, theme.Zuzu)},
	} {
		s, report, err := svgOf(t, b.String(), true, test.opts...)
		if err != nil {
			t.Fatalf("%s: %v (%v)", test.name, err, report)
		}
		c := test.th.Category
		// Each actor takes the next color, the eleventh the first again,
		// and the use case the second. The boundary takes the first.
		for i, want := range []string{c[0], c[1], c[9], c[0], c[1]} {
			paint := fmt.Sprintf(`fill="%s" stroke="%s" stroke-width="1.25" fill-opacity="0.14"`, want, want)
			if !strings.Contains(s, paint) {
				t.Errorf("%s: %d: the SVG has no node in %s", test.name, i, want)
			}
		}
		if n := strings.Count(s, `rx="6" fill="`+c[0]+`" stroke="`+c[0]+`"`); n != 1 {
			t.Errorf("%s: expected the boundary in %s, got %d", test.name, c[0], n)
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
		scheme := "mermaid-redux-color"
		if strings.Contains(src, "forest") {
			scheme = "mermaid-forest"
		}
		sc, _ := theme.Lookup(scheme)
		c := theme.New(theme.Light, sc).Category
		if strings.Contains(s, `fill="`+c[1]+`"`) {
			t.Errorf("%q: a node draws in the second color %s", src, c[1])
		}
	}
}
