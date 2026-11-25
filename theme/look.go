package theme

import "slices"

// Look is a way to draw the marks of a chart or a diagram: the standard look
// of zuzu, or a look drawn by hand (D39, D40). A look changes the shapes and
// the font, and keeps the colors of the mode and the scheme. "The look like
// Excalidraw" in docs/STYLE.md holds its rules.
type Look string

// The looks. The look like Excalidraw is the only look drawn by hand (D57).
const (
	// LookStandard draws each mark as docs/STYLE.md says. It is the
	// default.
	LookStandard Look = "standard"
	// LookExcalidraw draws each mark as if by hand, as Excalidraw and the
	// look handDrawn of Mermaid draw it, with the options of rough.js that
	// Mermaid 12.3 gives, and draws its text in Excalifont.
	LookExcalidraw Look = "excalidraw"
)

// looks are the looks that zuzu draws.
var looks = []Look{LookStandard, LookExcalidraw}

// Known reports whether zuzu draws a look.
func (l Look) Known() bool {
	return slices.Contains(looks, l)
}

// HandDrawn reports whether a look draws its marks as if by hand.
func (l Look) HandDrawn() bool {
	return l == LookExcalidraw
}

// The tokens of the look like Excalidraw, from "The look like Excalidraw"
// in docs/STYLE.md. They are the options of rough.js that Mermaid 12.3
// gives for look: handDrawn (D40 part 5).
const (
	// SketchRoughness is how far a line strays from its path.
	SketchRoughness = 0.7
	// SketchEdgeRoughness is the roughness of the lines and the
	// arrowheads of an edge of a graph.
	SketchEdgeRoughness = 0.3
	// SketchStroke is the width of a stroke that the item does not set.
	SketchStroke = 1.3
	// SketchFillWeight is the width of each line of a hachure.
	SketchFillWeight = 1.5
	// SketchClusterFillWeight is the width of each line of the hachure of
	// the box of a cluster of a graph.
	SketchClusterFillWeight = 3.0
	// SketchHachureGap is the space between the lines of a hachure.
	SketchHachureGap = 1.5
	// SketchHachureAngle is the angle of the lines of a hachure, in
	// degrees, the default of rough.js.
	SketchHachureAngle = -41.0
	// SketchGradientGap is the space between the lines of the hachure of
	// a gradient, the gap of Excalidraw (D50).
	SketchGradientGap = 5.0
	// SketchGradientTint is the opacity of the tint under the hachure of a
	// gradient, times the opacity of the fill.
	SketchGradientTint = 0.25
	// SketchGradientInset is how far the hachure of a gradient stays
	// inside the outline.
	SketchGradientInset = 2.0
	// SketchGradientLeast is the least width and height of a shape whose
	// gradient draws over a tint. A thinner shape, such as the bar of a
	// legend, keeps the hachure of SketchHachureGap.
	SketchGradientLeast = 20.0
	// SketchSlack is the most that the canvas grows at a side for a line
	// that wobbles out of it. A mark that reaches further past the side
	// stays cut there, as in the standard look.
	SketchSlack = 10.0
	// SketchSeed is the seed of a chart that sets none (D40 part 6).
	SketchSeed = 1
)
