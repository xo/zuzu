package theme

// The styles that the compiler of Vega-Lite names for the color of a mark
// that a spec does not set (D6, D14). The config of a theme gives them the
// first color of the categories, or the text color.
const (
	// StyleFill is the style of a mark whose fill is the color of the mark.
	StyleFill = "zuzu.fill"
	// StyleStroke is the style of a mark whose stroke is the color of the
	// mark.
	StyleStroke = "zuzu.stroke"
	// StyleText is the style of a text whose fill is the text color.
	StyleText = "zuzu.text"
	// StyleBar is the prefix of the style of a bar, which its orient,
	// vertical or horizontal, ends.
	StyleBar = "zuzu.bar."
	// StylePoint is the style of a point, which gives it a ring in the
	// background color.
	StylePoint = "zuzu.point"
	// CornerRadiusEnd is the property of a style of a bar that rounds the
	// end of the bar away from zero, and only the last bar of a stack. It
	// is a rule of the look of zuzu that Vega has no config for, and 0
	// turns it off (D14).
	CornerRadiusEnd = "zuzu.cornerRadiusEnd"
)

// Config returns the theme as a Vega config (D14): the defaults of each
// kind of mark, in the colors of the theme and with the rules of the marks
// of docs/STYLE.md, and the styles that the compiler of Vega-Lite names. The
// config of a spec goes over it, and the encoding of a mark goes over both.
// Each call returns a new value, which the caller owns.
func (t Theme) Config() map[string]any {
	c := t.Category[0]
	return map[string]any{
		"arc":  map[string]any{"fill": c, "padAngle": 0.01, "cornerRadius": 4},
		"area": map[string]any{"fill": c},
		"line": map[string]any{
			"stroke": c, "strokeWidth": 2.0, "strokeCap": "round", "strokeJoin": "round",
		},
		"path":  map[string]any{"stroke": c},
		"rect":  map[string]any{"fill": c},
		"rule":  map[string]any{"stroke": t.Text},
		"shape": map[string]any{"stroke": c},
		// A point has a ring of 1 in the background color.
		"symbol": map[string]any{"fill": c, "size": 64.0, "stroke": t.Background, "strokeWidth": 1.0},
		"text":   map[string]any{"fill": t.Text},
		"trail":  map[string]any{"fill": c, "size": 2.0},
		"style": map[string]any{
			StyleFill:               map[string]any{"fill": c},
			StyleStroke:             map[string]any{"stroke": c},
			StyleText:               map[string]any{"fill": t.Text},
			StylePoint:              map[string]any{"stroke": t.Background, "strokeWidth": 1.0},
			StyleBar + "vertical":   map[string]any{CornerRadiusEnd: 3.0},
			StyleBar + "horizontal": map[string]any{CornerRadiusEnd: 3.0},
		},
	}
}
