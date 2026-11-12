// Package theme holds the themes of zuzu: a mode, light or dark, and a color
// scheme, which a caller swaps apart from the mode (D14).
//
// The mode sets the neutral colors: the background, the text, the grid and the
// axes. The scheme sets the colors of the data. docs/STYLE.md holds the tokens
// of both, and a test checks that this package holds the same values.
package theme

import (
	"slices"
)

// Mode is the light or the dark mode.
type Mode int

// The modes.
const (
	Light Mode = iota
	Dark
)

// String returns the name of the mode.
func (m Mode) String() string {
	if m == Dark {
		return "dark"
	}
	return "light"
}

// Neutrals are the colors of a mode that do not carry data.
type Neutrals struct {
	Background string // the background of the chart
	Panel      string // a panel inside the chart, such as a tooltip
	Text       string // the title, the labels on the data, a value
	Muted      string // the subtitle, the labels of an axis, a legend
	Faint      string // the source line, the title of an axis, a guide line
	Grid       string // the grid lines
	Axis       string // a base line, a border, a guide line of a diagram
	Edge       string // an edge of a diagram, its arrowheads, a message
	Band       string // a band behind a group of rows, as it shows over the background
}

// BandOpacity is the opacity of the text that draws a band behind a group
// of rows. Band is the text at this opacity over the background, and zuzu
// draws the band in the text color at this opacity, so that the grid lines
// and the labels under it still show.
const BandOpacity = 0.06

// neutrals returns the neutrals of a mode, from docs/STYLE.md.
func neutrals(m Mode) Neutrals {
	if m == Dark {
		return Neutrals{
			Background: "#10141b", Panel: "#171c25", Text: "#e7ebf2", Muted: "#9ba5b7",
			Faint: "#6a7487", Grid: "#1e242e", Axis: "#333c4a", Edge: "#707a8d", Band: "#1d2128",
		}
	}
	return Neutrals{
		Background: "#ffffff", Panel: "#f5f6f8", Text: "#1b2230", Muted: "#5f6b7d",
		Faint: "#98a2b3", Grid: "#eceff3", Axis: "#cfd5de", Edge: "#848e9f", Band: "#f1f2f3",
	}
}

// Scheme is a color scheme: the colors of the data.
type Scheme struct {
	name  string
	light []string
	dark  []string
	up    int
	down  int
}

// Name returns the name of the scheme, such as zuzu.
func (s Scheme) Name() string {
	return s.name
}

// The schemes that zuzu ships (D14). The maintainers accepted these four on
// 2026-11-10.
var (
	Zuzu = Scheme{
		name:  "zuzu",
		light: []string{"#5b6ee1", "#1fa187", "#f0a830", "#e2577a", "#3aa2d6", "#9a6fd1", "#7fb04a", "#e2733b", "#c2569f", "#5d8f9b"},
		dark:  []string{"#7d8cf0", "#3cc4a6", "#f6be55", "#f07a97", "#5cbdea", "#b38ef0", "#9ccb66", "#f08f5c", "#df7cc0", "#7fb2bd"},
		up:    1,
		down:  3,
	}
	Pastel = Scheme{
		name:  "pastel",
		light: []string{"#e8a87c", "#7fc8b8", "#e6cf6a", "#e7838f", "#8fb3e0", "#b7a1d8", "#a9cf8c", "#d9a3c4", "#8ccbd6", "#c9b49a"},
		up:    1,
		down:  3,
	}
	Harbor = Scheme{
		name:  "harbor",
		light: []string{"#1f6f9f", "#3fa7b5", "#7cc3a4", "#e3b04b", "#d0684d", "#5b4e8c", "#8aa8c8", "#2f9c7a", "#b98b5a", "#6c7a89"},
		up:    7,
		down:  4,
	}
	Ember = Scheme{
		name:  "ember",
		light: []string{"#d9480f", "#f08c00", "#e8590c", "#c2255c", "#f59f00", "#862e9c", "#a61e4d", "#e67700", "#5f3dc4", "#94d82d"},
		up:    9,
		down:  3,
	}
)

// schemes returns the schemes that zuzu ships, in the order of docs/STYLE.md.
func schemes() []Scheme {
	return []Scheme{Zuzu, Pastel, Harbor, Ember}
}

// Lookup returns the scheme with a name.
func Lookup(name string) (Scheme, bool) {
	i := slices.IndexFunc(schemes(), func(s Scheme) bool { return s.name == name })
	if i < 0 {
		return Scheme{}, false
	}
	return schemes()[i], true
}

// Schemes returns the names of the schemes, in the order of docs/STYLE.md.
func Schemes() []string {
	var names []string
	for _, s := range schemes() {
		names = append(names, s.name)
	}
	return names
}

// Theme is a mode and a scheme together: every color that a chart uses.
type Theme struct {
	Neutrals

	// Category holds ten categorical colors for the mode.
	Category []string
	// Up and Down are the colors of a rise and a fall.
	Up   string
	Down string
	// FillOpacity is the opacity of a tinted fill, such as a box of a box
	// plot.
	FillOpacity float64

	mode Mode
}

// New returns the theme of a mode and a scheme. A scheme without its own
// dark colors lifts each light color for the dark mode, and a scheme with
// fewer than ten colors gets the rest mixed toward the text color.
func New(m Mode, s Scheme) Theme {
	if s.name == "" {
		s = Zuzu
	}
	n := neutrals(m)
	cat := slices.Clone(s.light)
	if m == Dark {
		if s.dark != nil {
			cat = slices.Clone(s.dark)
		} else {
			for i, c := range cat {
				cat[i] = brighter(c, 0.45)
			}
		}
	}
	for i := len(cat); i < 10; i++ {
		cat = append(cat, mix(cat[i%len(cat)], n.Text, 0.4))
	}
	opacity := 0.2
	if m == Dark {
		opacity = 0.24
	}
	return Theme{
		Neutrals:    n,
		Category:    cat,
		Up:          cat[s.up],
		Down:        cat[s.down],
		FillOpacity: opacity,
		mode:        m,
	}
}

// Mode returns the mode of the theme.
func (t Theme) Mode() Mode {
	return t.mode
}

// Sequential returns the color at v, from 0 to 1, on the sequential ramp of
// the theme. The ramp runs from the background through the first color, so
// that it reads in both modes.
func (t Theme) Sequential(v float64) string {
	base := t.Category[0]
	var stops []string
	if t.mode == Dark {
		stops = []string{
			mix(t.Background, base, 0.12), mix(t.Background, base, 0.45), base,
			mix(base, "#ffffff", 0.45), mix(base, "#ffffff", 0.85),
		}
	} else {
		stops = []string{
			mix(t.Background, base, 0.1), mix(t.Background, base, 0.4), base,
			mix(base, "#000000", 0.35), mix(base, "#000000", 0.6),
		}
	}
	return ramp(stops, v)
}
