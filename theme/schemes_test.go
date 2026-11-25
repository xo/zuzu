package theme

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// TestEverySchemeFromAStyleIsWhole checks each scheme from a style of
// transit: it has 4 to 10 colors of the form #rrggbb in each mode that it
// brings, its own background there, a rise and a fall among its
// colors, a name that Lookup finds and no other scheme has, and a license
// file in the folder licenses for its source (D58).
func TestEverySchemeFromAStyleIsWhole(t *testing.T) {
	t.Parallel()
	if len(upstream) == 0 {
		t.Fatal("the package holds no scheme from a style of transit")
	}
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	names := make(map[string]bool)
	for _, s := range schemes() {
		if names[s.name] {
			t.Errorf("two schemes have the name %s", s.name)
		}
		names[s.name] = true
	}
	for _, s := range upstream {
		if got, ok := Lookup(s.name); !ok || got.name != s.name {
			t.Errorf("Lookup(%q) does not find the scheme", s.name)
		}
		if s.light == nil && s.dark == nil {
			t.Errorf("%s: the scheme has no colors", s.name)
		}
		for m, colors := range [][]string{s.light, s.dark} {
			if colors == nil {
				if s.own[m] != (Neutrals{}) {
					t.Errorf("%s %s: the scheme brings neutrals for a mode without colors", s.name, Mode(m))
				}
				continue
			}
			if len(colors) < 4 || len(colors) > 10 {
				t.Errorf("%s %s: %d colors, want 4 to 10", s.name, Mode(m), len(colors))
			}
			for _, c := range colors {
				if !hex.MatchString(c) {
					t.Errorf("%s %s: %q is not a color", s.name, Mode(m), c)
				}
			}
			// A style with no text color of its own takes the text of zuzu.
			own := s.own[m]
			if !hex.MatchString(own.Background) || (own.Text != "" && !hex.MatchString(own.Text)) {
				t.Errorf("%s %s: the background %q or the text %q is not a color", s.name, Mode(m), own.Background, own.Text)
			}
			if IsDark(own.Background) != (Mode(m) == Dark) {
				t.Errorf("%s %s: the background %s is not of the mode", s.name, Mode(m), own.Background)
			}
		}
		for _, m := range []Mode{Light, Dark} {
			th := New(m, s)
			if th.Up == th.Down {
				t.Errorf("%s %s: the rise and the fall are one color, %s", s.name, m, th.Up)
			}
		}
		if len(s.licenses) == 0 {
			t.Errorf("%s: the scheme names no license file", s.name)
		}
		for _, f := range s.licenses {
			if _, err := os.Stat(filepath.Join("licenses", filepath.FromSlash(f))); err != nil {
				t.Errorf("%s: the license file %s: %v", s.name, f, err)
			}
		}
	}
}

// TestEveryLicenseFileHasAScheme checks that each file in the folder
// licenses is the license of a scheme, so that no file stays after its
// scheme goes.
func TestEveryLicenseFileHasAScheme(t *testing.T) {
	t.Parallel()
	used := make(map[string]bool)
	for _, s := range upstream {
		for _, f := range s.licenses {
			used[f] = true
		}
	}
	var found int
	err := filepath.WalkDir("licenses", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel("licenses", path)
		if err != nil {
			return err
		}
		found++
		if !used[filepath.ToSlash(rel)] {
			t.Errorf("%s is the license of no scheme", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found != len(used) {
		t.Errorf("the folder licenses holds %d files, and the schemes name %d", found, len(used))
	}
}

// TestTheSharesMatchTheStyle checks the shares of the text in the
// background against their table in docs/STYLE.md.
func TestTheSharesMatchTheStyle(t *testing.T) {
	t.Parallel()
	rows := regexp.MustCompile("(?m)^\\| `(panel|muted|faint|grid|axis|edge)` \\| ([0-9.]+) \\| ([0-9.]+) \\|$").
		FindAllStringSubmatch(style(t), -1)
	if len(rows) != 6 {
		t.Fatalf("docs/STYLE.md has %d rows of shares, expected 6", len(rows))
	}
	for _, row := range rows {
		for i, m := range []Mode{Light, Dark} {
			sh := sharesOf(m)
			got := map[string]float64{
				"panel": sh.panel, "muted": sh.muted, "faint": sh.faint,
				"grid": sh.grid, "axis": sh.axis, "edge": sh.edge,
			}[row[1]]
			if want, _ := strconv.ParseFloat(row[2+i], 64); got != want {
				t.Errorf("%s %s: the package has %v and docs/STYLE.md has %v", m, row[1], got, want)
			}
		}
	}
}

// TestTheSharesAreThoseOfZuzu checks that each share gives a neutral near
// that of zuzu, when it mixes the text of zuzu into its background, as
// docs/STYLE.md says the shares come from the neutrals of zuzu.
func TestTheSharesAreThoseOfZuzu(t *testing.T) {
	t.Parallel()
	for _, m := range []Mode{Light, Dark} {
		n, sh := neutrals(m), sharesOf(m)
		for _, c := range []struct {
			name, want string
			share      float64
		}{
			{"panel", n.Panel, sh.panel}, {"muted", n.Muted, sh.muted}, {"faint", n.Faint, sh.faint},
			{"grid", n.Grid, sh.grid}, {"axis", n.Axis, sh.axis}, {"edge", n.Edge, sh.edge},
		} {
			got, want := parse(mix(n.Background, n.Text, c.share)), parse(c.want)
			if d := math.Max(math.Abs(got.r-want.r), math.Max(math.Abs(got.g-want.g), math.Abs(got.b-want.b))); d > 12 {
				t.Errorf("%s %s: the share %v gives %s, which is %v from %s", m, c.name, c.share, got.hex(), d, c.want)
			}
		}
	}
}

// TestASchemeWithItsOwnBackgroundMixesItsNeutrals checks that a scheme
// that brings a background and a text mixes the other neutrals of that
// mode from them, and keeps the neutrals of zuzu in the mode it does not
// bring.
func TestASchemeWithItsOwnBackgroundMixesItsNeutrals(t *testing.T) {
	t.Parallel()
	s := Scheme{
		name: "test",
		dark: []string{"#89b4fa", "#a6e3a1", "#f9e2af", "#f38ba8"},
		up:   1,
		down: 3,
		own:  [2]Neutrals{1: {Background: "#1e1e2e", Text: "#cdd6f4"}},
	}
	dark := New(Dark, s)
	sh := sharesOf(Dark)
	want := Neutrals{
		Background: "#1e1e2e", Text: "#cdd6f4",
		Panel: mix("#1e1e2e", "#cdd6f4", sh.panel), Muted: mix("#1e1e2e", "#cdd6f4", sh.muted),
		Faint: mix("#1e1e2e", "#cdd6f4", sh.faint), Grid: mix("#1e1e2e", "#cdd6f4", sh.grid),
		Axis: mix("#1e1e2e", "#cdd6f4", sh.axis), Edge: mix("#1e1e2e", "#cdd6f4", sh.edge),
		Band: mix("#1e1e2e", "#cdd6f4", BandOpacity),
	}
	if dark.Neutrals != want {
		t.Errorf("the dark neutrals are %+v, want %+v", dark.Neutrals, want)
	}
	if dark.Up != "#a6e3a1" || dark.Down != "#f38ba8" {
		t.Errorf("the rise and the fall are %s and %s", dark.Up, dark.Down)
	}
	light := New(Light, s)
	if light.Neutrals != neutrals(Light) {
		t.Errorf("the light neutrals are %+v, want those of zuzu", light.Neutrals)
	}
	for i, c := range s.dark {
		if got := light.Category[i]; got != darker(c) {
			t.Errorf("the light color %d is %s, want %s darkened", i, got, c)
		}
	}
}

// TestDarkerIsTheInverseOfBrighter checks that darker lowers the lightness
// by the factor that brighter raises it by, as in d3-color.
func TestDarkerIsTheInverseOfBrighter(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"#5b6ee1", "#e8a87c", "#1fa187", "#f38ba8"} {
		_, _, l := toHSL(parse(c))
		_, _, dl := toHSL(parse(darker(c)))
		if want := l * math.Pow(0.7, lift); math.Abs(dl-want) > 1.0/255 {
			t.Errorf("darker(%s) has the lightness %v, want %v", c, dl, want)
		}
		back := parse(brighter(darker(c)))
		orig := parse(c)
		if math.Abs(back.r-orig.r) > 2 || math.Abs(back.g-orig.g) > 2 || math.Abs(back.b-orig.b) > 2 {
			t.Errorf("brighter(darker(%s)) is %s", c, back.hex())
		}
	}
}

// TestAFamilyTakesTheRiseOfEachMode checks that a family takes the rise and
// the fall of its dark style in the dark mode.
func TestAFamilyTakesTheRiseOfEachMode(t *testing.T) {
	t.Parallel()
	i := slices.IndexFunc(upstream[:], func(s Scheme) bool { return s.darkUp != s.darkDown })
	if i < 0 {
		t.Skip("no family has a rise or a fall of its own in the dark mode")
	}
	s := upstream[i]
	if th := New(Dark, s); th.Up != s.dark[s.darkUp] || th.Down != s.dark[s.darkDown] {
		t.Errorf("%s: the dark rise and fall are %s and %s, want %s and %s",
			s.name, th.Up, th.Down, s.dark[s.darkUp], s.dark[s.darkDown])
	}
	if th := New(Light, s); th.Up != s.light[s.up] || th.Down != s.light[s.down] {
		t.Errorf("%s: the light rise and fall are %s and %s, want %s and %s",
			s.name, th.Up, th.Down, s.light[s.up], s.light[s.down])
	}
}
