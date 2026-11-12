package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// style returns the text of docs/STYLE.md.
func style(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "docs", "STYLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestTheModesMatchTheStyle checks the neutrals of each mode against the
// table of modes in docs/STYLE.md, so that the document and the code cannot
// drift apart.
func TestTheModesMatchTheStyle(t *testing.T) {
	t.Parallel()
	rows := regexp.MustCompile("(?m)^\\| `([a-z]+)` \\| `(#[0-9a-f]{6})` \\| `(#[0-9a-f]{6})` \\|").
		FindAllStringSubmatch(style(t), -1)
	if len(rows) != 9 {
		t.Fatalf("docs/STYLE.md has %d rows of modes, expected 9", len(rows))
	}
	for _, m := range []Mode{Light, Dark} {
		n := neutrals(m)
		got := map[string]string{
			"bg": n.Background, "panel": n.Panel, "text": n.Text, "muted": n.Muted,
			"faint": n.Faint, "grid": n.Grid, "axis": n.Axis, "edge": n.Edge, "band": n.Band,
		}
		for _, row := range rows {
			want := row[2]
			if m == Dark {
				want = row[3]
			}
			if got[row[1]] != want {
				t.Errorf("%s %s: the package has %s and docs/STYLE.md has %s", m, row[1], got[row[1]], want)
			}
		}
	}
}

// TestTheBandIsTheTextOverTheBackground checks that the band of each mode
// is the color that the text at BandOpacity gives over the background, as
// docs/STYLE.md says.
func TestTheBandIsTheTextOverTheBackground(t *testing.T) {
	t.Parallel()
	for _, m := range []Mode{Light, Dark} {
		n := neutrals(m)
		if got := mix(n.Background, n.Text, BandOpacity); got != n.Band {
			t.Errorf("%s: the text at %v over the background is %s, and the band is %s", m, BandOpacity, got, n.Band)
		}
	}
}

// TestTheZuzuSchemeMatchesTheStyle checks the colors of the default scheme
// against its table in docs/STYLE.md.
func TestTheZuzuSchemeMatchesTheStyle(t *testing.T) {
	t.Parallel()
	rows := regexp.MustCompile("(?m)^\\| ([0-9]+) \\| `(#[0-9a-f]{6})` \\| `(#[0-9a-f]{6})` \\|").
		FindAllStringSubmatch(style(t), -1)
	if len(rows) != 10 {
		t.Fatalf("docs/STYLE.md has %d colors of zuzu, expected 10", len(rows))
	}
	light, dark := New(Light, Zuzu), New(Dark, Zuzu)
	for i, row := range rows {
		if light.Category[i] != row[2] || dark.Category[i] != row[3] {
			t.Errorf("color %s: the package has %s and %s, and docs/STYLE.md has %s and %s",
				row[1], light.Category[i], dark.Category[i], row[2], row[3])
		}
	}
}

// TestEveryThemeHasTenColors checks each scheme in each mode.
func TestEveryThemeHasTenColors(t *testing.T) {
	t.Parallel()
	for _, name := range Schemes() {
		s, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) found nothing", name)
		}
		for _, m := range []Mode{Light, Dark} {
			th := New(m, s)
			if len(th.Category) != 10 || th.Up == "" || th.Down == "" || th.Mode() != m {
				t.Errorf("%s %s: got %+v", name, m, th)
			}
			for _, c := range th.Category {
				if !regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(c) {
					t.Errorf("%s %s: %q is not a color", name, m, c)
				}
			}
		}
	}
	if _, ok := Lookup("nonexistent"); ok {
		t.Error("Lookup found a scheme that does not exist")
	}
}

// TestTheDarkModeLiftsALightColor checks the lift against the value that
// the brighter method of d3-color gives for the same color.
func TestTheDarkModeLiftsALightColor(t *testing.T) {
	t.Parallel()
	// d3.hsl("#e8a87c").brighter(0.45).formatHex() in d3 7.9.0
	if got, want := brighter("#e8a87c", 0.45), "#f1cbb1"; got != want {
		t.Errorf("brighter: got %s, want %s", got, want)
	}
	if got := New(Dark, Pastel).Category[0]; got != brighter("#e8a87c", 0.45) {
		t.Errorf("the dark pastel scheme starts with %s", got)
	}
}

// TestTheSequentialRampRuns checks the ends and the middle of the ramp.
func TestTheSequentialRampRuns(t *testing.T) {
	t.Parallel()
	th := New(Light, Zuzu)
	if got := th.Sequential(0.5); got != th.Category[0] {
		t.Errorf("the middle of the ramp is %s, want the first color %s", got, th.Category[0])
	}
	if th.Sequential(0) == th.Sequential(1) {
		t.Error("the ramp has the same color at both ends")
	}
}

// TestTheEdgesTakeTheColorOfAnEdge checks that the styles of an edge and
// its arrowheads take the color edge, a guide line the color axis, and a
// band the text color (D32).
func TestTheEdgesTakeTheColorOfAnEdge(t *testing.T) {
	t.Parallel()
	for _, m := range []Mode{Light, Dark} {
		th := New(m, Zuzu)
		styles, _ := th.Config()["style"].(map[string]any)
		for _, c := range []struct {
			style, prop, want string
		}{
			{StyleEdge, "stroke", th.Edge},
			{StyleArrow, "fill", th.Edge},
			{StyleArrow, "stroke", th.Edge},
			{StyleArrowOpen, "stroke", th.Edge},
			{StyleGuide, "stroke", th.Axis},
			{StyleBand, "fill", th.Text},
		} {
			s, _ := styles[c.style].(map[string]any)
			if got := s[c.prop]; got != c.want {
				t.Errorf("%s: the %s of %s is %v, want %s", m, c.prop, c.style, got, c.want)
			}
		}
	}
}
