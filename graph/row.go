package graph

// StyleRow copies the paint of a style of a node into a row of a chart that
// lays itself out, with the names of Vega, as Compile does for a node: fill,
// fillOpacity, stroke, strokeWidth, strokeDash, and color as textColor. A
// solid fill with no color of text gets the color of text that reads on it.
func StyleRow(r map[string]any, st map[string]string) {
	style(r, st)
}

// PaintKeys returns the keys of the paint of a node that a row can hold.
func PaintKeys() []string {
	return append([]string(nil), paintKeys...)
}

// TextKeys returns the keys of the style of a label that a row can hold.
func TextKeys() []string {
	return append([]string(nil), textKeys...)
}
