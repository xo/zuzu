package shape

import (
	"math"
	"strings"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// DefaultFont is the font family of a text item that names none. zuzu
// bundles IBM Plex Sans (D16).
const DefaultFont = "'IBM Plex Sans', sans-serif"

// The baselines of Vega that the writers read.
const (
	baselineTop    = "top"
	baselineMiddle = "middle"
	baselineBottom = "bottom"
)

// baselines are the offsets of the baselines of Vega, as fractions of the
// size of the font, from the alphabetic baseline that SVG draws on.
var baselines = map[string]float64{
	baselineTop:    0.79,
	baselineMiddle: 0.3,
	baselineBottom: -0.21,
}

// Text is the layout of a text item: where its first line starts, on its
// baseline, before the angle of the item, and the lines that it draws, one
// line height apart.
type Text struct {
	X, Y       float64
	Angle      float64
	Size       float64
	LineHeight float64
	// Anchor is the text-anchor of SVG: start, middle or end.
	Anchor string
	Lines  []string
}

// TextOf lays out a text item: its size, 11 by default, its place from its
// radius and its angle theta, its offsets and its baseline, and its lines,
// each cut to its limit with its ellipsis, as Vega cuts them. A text whose
// direction is rtl loses its start rather than its end. m measures a line
// for the limit.
func TextOf(it *scene.Item, m text.Measurer) Text {
	size := it.FontSize
	if size == 0 {
		size = 11
	}
	x, y := it.X, it.Y
	if it.Radius != 0 {
		x += it.Radius * math.Sin(it.Theta)
		y -= it.Radius * math.Cos(it.Theta)
	}
	lh := it.LineHeight
	if lh == 0 {
		lh = size
	}
	y += it.Dy + baselines[it.Baseline]*size - lh*float64(len(it.Text)-1)*multiline(it.Baseline)
	x += it.Dx
	anchor := "start"
	switch it.Align {
	case "center":
		anchor = "middle"
	case "right":
		anchor = "end"
	}
	return Text{X: x, Y: y, Angle: it.Angle, Size: size, LineHeight: lh, Anchor: anchor, Lines: limit(it, size, m)}
}

// limit cuts each line of a text item to its limit, in pixels.
func limit(it *scene.Item, size float64, m text.Measurer) []string {
	if it.Limit <= 0 {
		return it.Text
	}
	f := text.Font{Family: it.Font, Size: size, Weight: text.Weight(string(it.FontWeight)), Italic: it.FontStyle == "italic"}
	out := make([]string, len(it.Text))
	for i, line := range it.Text {
		if it.Dir == "rtl" {
			out[i] = reverse(text.Truncate(m, f, reverse(line), it.Limit))
		} else {
			out[i] = text.Truncate(m, f, line, it.Limit)
		}
		if it.Ellipsis != "" && it.Ellipsis != text.Ellipsis {
			if s, ok := strings.CutSuffix(out[i], text.Ellipsis); ok && out[i] != line {
				out[i] = s + it.Ellipsis
			}
		}
	}
	return out
}

// reverse reverses the runes of a string.
func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// multiline returns how far the first line of a text of several lines moves
// up, in lines, for a baseline: none for the top, half for the middle and all
// of them for the bottom.
func multiline(baseline string) float64 {
	switch baseline {
	case baselineMiddle:
		return 0.5
	case baselineBottom, "alphabetic", "":
		return 1
	}
	return 0
}

// Collapse collapses the white space of a line as SVG collapses the text of
// an element: each tab and each new line becomes a space, a run of spaces
// becomes one, and the line loses its spaces at the start and at the end.
func Collapse(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}), " ")
}
