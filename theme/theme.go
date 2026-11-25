// Package theme holds the themes of zuzu: a mode, light or dark, and a color
// scheme, which a caller swaps apart from the mode (D14).
//
// The mode sets the neutral colors: the background, the text, the grid and the
// axes. The scheme sets the colors of the data. docs/STYLE.md holds the tokens
// of both, and a test checks that this package holds the same values.
//
// Besides the schemes of zuzu and of the themes of Mermaid, the package holds
// a scheme for each style of the package styles of transit, and for each
// family of a light and a dark style (D58). The command test/cmd/schemes of
// the test module writes them to schemes.go, with the license of each source
// in the folder licenses. docs/SCHEMES.md lists them.
package theme

import (
	"math"
	"slices"

	"github.com/xo/zuzu/internal/color"
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
	Muted      string // the subtitle, the labels and the title of an axis, a legend
	Faint      string // the source line, the title of a legend, a guide line
	Grid       string // the grid lines
	Axis       string // the line and the ticks of an axis, a base line, a border
	Edge       string // an edge of a diagram, its arrowheads, a message
	Band       string // a band behind a group of rows, as it shows over the background
}

// BandOpacity is the opacity of the text that draws a band behind a group
// of rows. Band is the text at this opacity over the background, and zuzu
// draws the band in the text color at this opacity, so that the grid lines
// and the labels under it still show.
const BandOpacity = 0.06

// The sizes of an edge of a diagram, from "Edges" in docs/STYLE.md.
const (
	// EdgeWidth is the width of the line of an edge, and of the outline of
	// an open arrowhead.
	EdgeWidth = 1.5
	// ArrowLength is the length of the arrowhead normal of DOT at an
	// arrowsize of 1. It is 0.8 of this length wide.
	ArrowLength = 8.0
)

// The sizes of an axis, from "Axes and grids" in docs/STYLE.md. The line
// and the ticks of an axis are in the color Axis (D46).
const (
	// TickSize is the length of a tick, from the line of the axis.
	TickSize = 5.0
	// TickWidth is the width of a tick.
	TickWidth = 1.0
	// DomainWidth is the width of the line of an axis.
	DomainWidth = 1.0
	// LabelPadding is the space between the end of a tick and its label.
	LabelPadding = 3.0
	// LabelGap is the least space between two flat labels of a discrete
	// axis. A label that does not fit the step of its band with this space
	// turns the labels of a discrete x axis (D58 part 4).
	LabelGap = 4.0
)

// The paint of a node of a diagram, from "Marks" in docs/STYLE.md: a tint
// of its color and an outline.
const (
	// NodeTint is how far the solid tint of a node goes from the
	// background to its color (D55).
	NodeTint = 0.14
	// NodeOutline is the width of the outline of a node.
	NodeOutline = 1.25
)

// neutrals returns the neutrals of a mode, from docs/STYLE.md.
func neutrals(m Mode) Neutrals {
	if m == Dark {
		return Neutrals{
			Background: "#10141b", Panel: "#171c25", Text: "#e7ebf2", Muted: "#9ba5b7",
			Faint: "#6a7487", Grid: "#1e242e", Axis: "#333c4a", Edge: "#bcc4d2", Band: "#1d2128",
		}
	}
	return Neutrals{
		Background: "#ffffff", Panel: "#f5f6f8", Text: "#1b2230", Muted: "#5f6b7d",
		Faint: "#98a2b3", Grid: "#eceff3", Axis: "#cfd5de", Edge: "#414a5b", Band: "#f1f2f3",
	}
}

// shares are the shares of the text in the background that make the
// neutrals other than the background and the text of a scheme that brings
// its own background (docs/STYLE.md).
type shares struct {
	panel, muted, faint, grid, axis, edge float64
}

// sharesOf returns the shares of a mode, from "Schemes from upstream themes"
// in docs/STYLE.md.
func sharesOf(m Mode) shares {
	if m == Dark {
		return shares{panel: 0.04, muted: 0.68, faint: 0.46, grid: 0.08, axis: 0.19, edge: 0.82}
	}
	return shares{panel: 0.04, muted: 0.67, faint: 0.42, grid: 0.07, axis: 0.19, edge: 0.82}
}

// over returns the neutrals of a mode with each field that own sets in
// place of its own. When own sets the background, each other neutral that
// it does not set mixes the text into the background by the share of the
// mode. A band that own does not set is the text at BandOpacity over the
// background, as docs/STYLE.md says.
func (n Neutrals) over(own Neutrals, m Mode) Neutrals {
	if own == (Neutrals{}) {
		return n
	}
	out := n
	if own.Background != "" {
		out.Background = own.Background
	}
	if own.Text != "" {
		out.Text = own.Text
	}
	sh := sharesOf(m)
	for _, f := range []struct {
		dst   *string
		src   string
		share float64
	}{
		{&out.Panel, own.Panel, sh.panel}, {&out.Muted, own.Muted, sh.muted},
		{&out.Faint, own.Faint, sh.faint}, {&out.Grid, own.Grid, sh.grid},
		{&out.Axis, own.Axis, sh.axis}, {&out.Edge, own.Edge, sh.edge},
	} {
		switch {
		case f.src != "":
			*f.dst = f.src
		case own.Background != "":
			*f.dst = mix(out.Background, out.Text, f.share)
		}
	}
	out.Band = own.Band
	if own.Band == "" {
		out.Band = mix(out.Background, out.Text, BandOpacity)
	}
	return out
}

// Scheme is a color scheme: the colors of the data. A scheme from an
// upstream theme can also bring neutrals of its own for a mode (D15).
type Scheme struct {
	name string
	// light and dark hold the colors of each mode. A scheme from a style of
	// transit has one of the two, or both for a family (D58).
	light []string
	dark  []string
	// up and down are the indices of the colors of a rise and a fall.
	up   int
	down int
	// darkUp and darkDown are the indices of the colors of a rise and a fall
	// among the dark colors of a family, which come from another style. A
	// rise and a fall are never one color, so when the two are equal, up
	// and down hold in both modes.
	darkUp   int
	darkDown int
	// own holds the neutrals of the scheme for each mode, by the mode. An
	// empty field keeps the neutral of the mode.
	own [2]Neutrals
	// licenses names the license files of the sources of a scheme from a
	// style of transit, in the folder licenses of this package.
	licenses []string
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

// The text and the line colors that the themes of Mermaid share: a dark
// gray, black, and the ink of redux.
const (
	mermaidGray  = "#333333"
	mermaidBlack = "#000000"
	mermaidInk   = "#28253d"
)

// The schemes of the light themes of Mermaid 12.3.0, which the theme of a
// Mermaid diagram selects (D32). docs/STYLE.md says how their colors come
// from the theme variables of Mermaid, under the MIT license. Each brings
// the text and the line color of its theme for the light mode.
var (
	mermaidDefault = Scheme{
		name:  "mermaid-default",
		light: []string{"#8683de", "#8e8f2e", "#bf6fbe", "#bb7d39", "#09a069", "#d4659d", "#6a983d", "#d86769", "#00a1a4", "#1a94e1"},
		up:    4,
		down:  7,
		own:   [2]Neutrals{{Text: mermaidGray, Edge: mermaidGray}},
	}
	mermaidBase = Scheme{
		name:  "mermaid-base",
		light: []string{"#ab8530", "#b075ca", "#829332", "#5f8ce3", "#609a43", "#da6386", "#00a176", "#d26e57", "#009fc1"},
		up:    4,
		down:  5,
		own:   [2]Neutrals{{Text: mermaidGray, Edge: "#0b0b0b"}},
	}
	mermaidNeutral = Scheme{
		name:  "mermaid-neutral",
		light: []string{"#777777", "#b0b0b0", "#868686", "#c6c6c6", "#989898", "#d1d1d1", "#7f7f7f", "#bbbbbb", "#919191", "#a3a3a3"},
		dark:  []string{"#5e5e5e", "#969696", "#6d6d6d", "#ababab", "#7f7f7f", "#b6b6b6", "#666666", "#a0a0a0", "#777777", "#898989"},
		up:    0,
		down:  1,
		own:   [2]Neutrals{{Text: mermaidBlack, Edge: "#666666"}},
	}
	mermaidForest = Scheme{
		name:  "mermaid-forest",
		light: []string{"#789536", "#009cd1", "#da647d", "#359e5b", "#7188e2", "#00a189", "#b672c6", "#9d8a2d", "#cc724c"},
		up:    3,
		down:  2,
		own:   [2]Neutrals{{Text: mermaidBlack, Edge: mermaidBlack}},
	}
	mermaidNeo = Scheme{
		name:  "mermaid-neo",
		light: []string{"#8982dd", "#8d902e", "#bf6ebe", "#bb7d3a", "#129f68", "#d4659c", "#6a983d", "#d86867", "#00a1a4", "#2493e1"},
		up:    4,
		down:  7,
		own:   [2]Neutrals{{Text: mermaidGray, Edge: mermaidBlack}},
	}
	mermaidRedux = Scheme{
		name:  "mermaid-redux",
		light: []string{"#787583", "#b2afbe", "#878593", "#c8c5d4", "#9a97a6", "#d3d0df", "#807d8b", "#bdbac9", "#928f9e", "#a4a1b0"},
		dark:  []string{"#5f5d6a", "#9794a3", "#6e6b79", "#aca9b8", "#807d8b", "#b7b4c3", "#676472", "#a29fae", "#787583", "#8a8796"},
		up:    0,
		down:  1,
		own:   [2]Neutrals{{Text: mermaidInk, Edge: mermaidBlack}},
	}
	mermaidReduxColor = Scheme{
		name:  "mermaid-redux-color",
		light: []string{"#f4a8ff", "#46ecd5", "#ffb86a", "#dab2ff", "#7bf1a8", "#c4b4ff", "#ffa2a2", "#ffdf20", "#a3b3ff", "#bbf451"},
		up:    4,
		down:  6,
		own:   [2]Neutrals{{Text: mermaidInk, Edge: mermaidBlack}},
	}
)

// builtin returns the schemes of zuzu and of the themes of Mermaid, in the
// order of docs/STYLE.md.
func builtin() []Scheme {
	return []Scheme{
		Zuzu, Pastel, Harbor, Ember,
		mermaidDefault, mermaidBase, mermaidNeutral, mermaidForest, mermaidNeo, mermaidRedux, mermaidReduxColor,
	}
}

// schemes returns the schemes that zuzu ships: those of builtin, then those
// from the styles of transit, sorted by their names.
func schemes() []Scheme {
	return append(builtin(), upstream[:]...)
}

// Lookup returns the scheme with a name.
func Lookup(name string) (Scheme, bool) {
	for _, s := range builtin() {
		if s.name == name {
			return s, true
		}
	}
	for i := range upstream {
		if upstream[i].name == name {
			return upstream[i], true
		}
	}
	return Scheme{}, false
}

// Schemes returns the names of the schemes: those of zuzu and of the themes
// of Mermaid in the order of docs/STYLE.md, then those from the styles of
// transit, sorted by their names, as docs/SCHEMES.md lists them.
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
	// Note is the color of a note of a diagram: the categorical color whose
	// hue is nearest the yellow of a note on paper, NoteHue.
	Note string

	mode Mode
}

// New returns the theme of a mode and a scheme. A scheme without its own
// dark colors lifts each light color for the dark mode, and a scheme with
// only dark colors darkens each for the light mode. A scheme with fewer
// than ten colors gets the rest mixed toward the text color. A neutral that
// the scheme brings for the mode replaces that of the mode.
func New(m Mode, s Scheme) Theme {
	if s.name == "" {
		s = Zuzu
	}
	n := neutrals(m).over(s.own[m], m)
	cat, up, down := s.colors(m)
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
		Up:          cat[up],
		Down:        cat[down],
		FillOpacity: opacity,
		Note:        nearestHue(cat, NoteHue),
		mode:        m,
	}
}

// colors returns the categorical colors of the scheme for a mode, and the
// indices of the colors of a rise and a fall among them. The dark mode
// lifts the light colors of a scheme without dark colors, and the light
// mode darkens the dark colors of a scheme without light colors, by the
// same factor (docs/STYLE.md).
func (s Scheme) colors(m Mode) ([]string, int, int) {
	switch {
	case m == Dark && s.dark != nil:
		if s.darkUp != s.darkDown {
			return slices.Clone(s.dark), s.darkUp, s.darkDown
		}
		return slices.Clone(s.dark), s.up, s.down
	case m == Dark:
		cat := make([]string, len(s.light))
		for i, c := range s.light {
			cat[i] = brighter(c)
		}
		return cat, s.up, s.down
	case s.light == nil:
		cat := make([]string, len(s.dark))
		for i, c := range s.dark {
			cat[i] = darker(c)
		}
		return cat, s.up, s.down
	}
	return slices.Clone(s.light), s.up, s.down
}

// Tint returns the solid tint of a color: the color mixed into the
// background at NodeTint, with no opacity, so that it reads the same over
// a panel as over the background (D55).
func (t Theme) Tint(c string) string {
	return mix(t.Background, c, NodeTint)
}

// TextOn returns the color of a text that sits on a colored fill, such as
// the name of a leaf of a treemap (D59): the text of the light mode or the
// text of the dark mode, whichever has the higher contrast ratio of WCAG
// 2.1 with the fill. The text of the theme stands for its own mode, and
// wins a tie. A fill with some opacity counts as drawn over the background
// of the theme. A fill that does not parse gives the text of the theme.
func (t Theme) TextOn(fill string) string {
	c, ok := color.Parse(fill)
	if !ok {
		return t.Text
	}
	if c.A < 1 {
		if bg, ok := color.Parse(t.Background); ok {
			c = color.Mix(color.RGBA{R: bg.R, G: bg.G, B: bg.B, A: 1}, color.RGBA{R: c.R, G: c.G, B: c.B, A: 1}, c.A, "rgb")
		}
	}
	other := neutrals(Dark).Text
	if t.mode == Dark {
		other = neutrals(Light).Text
	}
	if contrastRatio(c, other) > contrastRatio(c, t.Text) {
		return other
	}
	return t.Text
}

// contrastRatio returns the contrast ratio of WCAG 2.1 of a color and a color
// of CSS, or 0 when the second does not parse.
func contrastRatio(c color.RGBA, s string) float64 {
	d, ok := color.Parse(s)
	if !ok {
		return 0
	}
	la, lb := c.Luminance(), d.Luminance()
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// NoteHue is the hue, in degrees, of the yellow of a note on paper, which
// the color of a note of a diagram comes nearest to.
const NoteHue = 52.0

// nearestHue returns the color whose hue is nearest a hue, in degrees. A
// gray, with no hue, comes last.
func nearestHue(colors []string, hue float64) string {
	best, bestD := colors[0], math.Inf(1)
	for _, c := range colors {
		rgb, ok := color.Parse(c)
		if !ok {
			continue
		}
		hi, lo := math.Max(rgb.R, math.Max(rgb.G, rgb.B)), math.Min(rgb.R, math.Min(rgb.G, rgb.B))
		if hi-lo < 1e-9 {
			continue
		}
		var h float64
		switch hi {
		case rgb.R:
			h = math.Mod((rgb.G-rgb.B)/(hi-lo), 6)
		case rgb.G:
			h = (rgb.B-rgb.R)/(hi-lo) + 2
		default:
			h = (rgb.R-rgb.G)/(hi-lo) + 4
		}
		h = math.Mod(h*60+360, 360)
		d := math.Abs(h - hue)
		d = math.Min(d, 360-d)
		if d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// Mode returns the mode of the theme.
func (t Theme) Mode() Mode {
	return t.mode
}

// RampStart is how far the sequential ramp of the theme starts from the
// background toward its first color, so that the lowest value of a scale
// shows in both modes (D48 part 4).
const RampStart = 0.4

// Sequential returns the color at v, from 0 to 1, on the sequential ramp of
// the theme. The ramp starts at a tint of the first color, RampStart of the
// way from the background, and runs through a tint halfway from there to
// the first color, then through the first color, so that it reads in both
// modes. It runs on to a lighter color on a dark background, and to a
// darker color on a light one. A caller that draws another background,
// such as the background of a spec, sets Background first (D35, D48).
func (t Theme) Sequential(v float64) string {
	base := t.Category[0]
	var stops []string
	if IsDark(t.Background) {
		stops = []string{
			mix(t.Background, base, RampStart), mix(t.Background, base, (1+RampStart)/2), base,
			mix(base, "#ffffff", 0.45), mix(base, "#ffffff", 0.85),
		}
	} else {
		stops = []string{
			mix(t.Background, base, RampStart), mix(t.Background, base, (1+RampStart)/2), base,
			mix(base, "#000000", 0.35), mix(base, "#000000", 0.6),
		}
	}
	return ramp(stops, v)
}

// IsDark reports whether a background in the form #rrggbb is dark: its
// lightness in HSL is below 0.5 (D35, D37). A color in another form reads
// as black, and so as dark.
func IsDark(background string) bool {
	_, _, l := toHSL(parse(background))
	return l < 0.5
}

// Diverging returns the color at v, from 0 to 1, on the diverging ramp of
// the theme. The ramp runs from the color of a fall through the color
// muted to the color of a rise, so that a mark near the middle still shows
// (docs/STYLE.md).
func (t Theme) Diverging(v float64) string {
	return ramp([]string{t.Down, t.Muted, t.Up}, v)
}
