package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/xo/transit/styles"

	"github.com/xo/zuzu/internal/color"
	"github.com/xo/zuzu/theme"
)

// The limits of the categorical colors of a style, from "Schemes from
// upstream themes" in docs/STYLE.md.
const (
	// upstreamChroma is the chroma in CIE LCh below which a color is a gray.
	upstreamChroma = 18.0
	// upstreamContrast is the least contrast ratio of WCAG 2.1 that a color has
	// with the background.
	upstreamContrast = 1.8
	// upstreamDistance is the least distance in CIELAB, as CIE76 measures it,
	// between a color and a color that the scheme already took.
	upstreamDistance = 15.0
	// upstreamHueStep is the least difference of hue, in degrees, that the next
	// color has with the color before it, when a color has it.
	upstreamHueStep = 90.0
	// maxColors and minColors bound the count of the colors of a scheme.
	maxColors = 10
	minColors = 4
	// riseColor and fallColor are green and crimson of CSS. The color of a
	// rise is the color nearest riseColor in the plane of a* and b* of
	// CIELAB, and the color of a fall the other color nearest fallColor.
	riseColor = "#008000"
	fallColor = "#dc143c"
)

// white is the background of a style that gives none, as Pygments draws
// such a style on white.
const white = "#ffffff"

// candidate is a foreground color of a style and how many entries use it.
type candidate struct {
	hex  string
	uses int
	lab  color.Lab
	hue  float64
}

// mode is the part of a scheme that one style gives: its background, its
// text and its categorical colors, with the color of a rise and of a fall.
type mode struct {
	style  string
	source string
	// ownText is true when the style gives its text color. Otherwise the
	// text is that of zuzu, which the scheme does not repeat.
	ownText bool
	dark    bool
	bg      string
	text    string
	colors  []string
	up      int
	down    int
}

// hexOf returns a color of a style as #rrggbb. An ANSI color takes the
// value of the default palette of xterm.
func hexOf(c styles.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// rgba reads a color in the form #rrggbb.
func rgba(hex string) color.RGBA {
	c, _ := color.Parse(hex)
	return c
}

// contrast returns the contrast ratio of two colors, as WCAG 2.1 defines
// it.
func contrast(a, b string) float64 {
	la, lb := rgba(a).Luminance(), rgba(b).Luminance()
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// hueDiff returns the difference of two hues in degrees, from 0 to 180.
func hueDiff(a, b float64) float64 {
	d := math.Abs(a - b)
	return min(d, 360-d)
}

// distance returns the distance of two colors in CIELAB, as CIE76
// measures it.
func distance(a, b color.Lab) float64 {
	return math.Sqrt((a.L-b.L)*(a.L-b.L) + (a.A-b.A)*(a.A-b.A) + (a.B-b.B)*(a.B-b.B))
}

// modeOf reads the background, the text and the categorical colors of a
// style, by the rules of docs/STYLE.md. It returns false when the style
// keeps fewer than minColors colors.
func modeOf(s *styles.Style) (mode, bool) {
	m := mode{style: s.Name, source: s.Source, bg: white}
	if s.Default.Bg.IsSet() {
		m.bg = hexOf(s.Default.Bg)
	}
	m.dark = theme.IsDark(m.bg)
	m.text = zuzuText(m.dark)
	if s.Default.Fg.IsSet() {
		m.text, m.ownText = hexOf(s.Default.Fg), true
	}
	uses := make(map[string]int)
	for _, e := range s.Captures {
		if e.Fg.IsSet() {
			uses[hexOf(e.Fg)]++
		}
	}
	text := rgba(m.text).Lab()
	all := make([]candidate, 0, len(uses))
	for hex, n := range uses {
		lab := rgba(hex).Lab()
		h, ch, _ := lab.HCL()
		if ch < upstreamChroma || contrast(hex, m.bg) < upstreamContrast || distance(lab, text) < upstreamDistance {
			continue
		}
		all = append(all, candidate{hex: hex, uses: n, lab: lab, hue: h})
	}
	slices.SortFunc(all, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(b.uses, a.uses), cmp.Compare(a.hex, b.hex))
	})
	var taken []candidate
	for _, c := range all {
		if len(taken) == maxColors {
			break
		}
		if !slices.ContainsFunc(taken, func(t candidate) bool { return distance(t.lab, c.lab) < upstreamDistance }) {
			taken = append(taken, c)
		}
	}
	ordered := order(taken)
	for _, c := range ordered {
		m.colors = append(m.colors, c.hex)
	}
	if len(m.colors) < minColors {
		return m, false
	}
	m.up, m.down = marks(ordered)
	return m, true
}

// order orders the colors, which come sorted by their uses. The first is
// the most used. Each next color is the most used one whose hue differs
// from the hue of the color before it by upstreamHueStep or more. If none does,
// it is the one whose hue differs most.
func order(colors []candidate) []candidate {
	if len(colors) == 0 {
		return nil
	}
	out := []candidate{colors[0]}
	rest := slices.Clone(colors[1:])
	for len(rest) != 0 {
		prev := out[len(out)-1].hue
		next := slices.IndexFunc(rest, func(c candidate) bool { return hueDiff(c.hue, prev) >= upstreamHueStep })
		if next < 0 {
			next = 0
			for i, c := range rest {
				if hueDiff(c.hue, prev) > hueDiff(rest[next].hue, prev) {
					next = i
				}
			}
		}
		out = append(out, rest[next])
		rest = slices.Delete(rest, next, next+1)
	}
	return out
}

// marks returns the index of the color of a rise, the color nearest
// riseColor, and of the color of a fall, the other color nearest
// fallColor. The distance leaves out the lightness, so that a light and a
// dark style take the same hue.
func marks(colors []candidate) (up, down int) {
	nearest := func(ref string, skip int) int {
		r := rgba(ref).Lab()
		best, bestD := -1, math.Inf(1)
		for i, c := range colors {
			if d := math.Hypot(c.lab.A-r.A, c.lab.B-r.B); i != skip && d < bestD {
				best, bestD = i, d
			}
		}
		return best
	}
	up = nearest(riseColor, -1)
	return up, nearest(fallColor, up)
}

// zuzuText returns the text color of zuzu in the light or the dark mode.
func zuzuText(dark bool) string {
	if dark {
		return theme.New(theme.Dark, theme.Zuzu).Text
	}
	return theme.New(theme.Light, theme.Zuzu).Text
}
