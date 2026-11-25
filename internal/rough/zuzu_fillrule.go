package rough

// This file is a file of zuzu, not of rough-go. It holds change 3 of
// README.md: the hachure of a polygon follows the rule nonzero of SVG when
// the option FillRule is "nonzero".

// edgeDir returns the direction of an edge of a polygon for the winding
// number: 1 when it runs down, to a larger y, and -1 when it runs up.
func edgeDir(p1, p2 Point) int {
	if p2[1] > p1[1] {
		return 1
	}
	return -1
}

// nonzeroLines adds to lines the spans of one scan line at y that the rule
// nonzero fills. The active edges are sorted by x. A span starts where the
// winding number leaves 0 and ends where it comes back to 0. So a part of
// the polygon that two of its rings cover in the same direction is filled,
// where the rule evenodd leaves it empty. A bridge that goes out and comes
// back along the same line adds 1 and takes 1 away, so it does not split a
// span. Outside the polygon, such a bridge gives a span of no length, as
// the rule evenodd of upstream gives it. So a polygon of one ring that does
// not cross itself gets the same lines with both rules.
func nonzeroLines(lines []Line, active []activeHachureEdgeEntry, y float64) []Line {
	winding := 0
	start := 0.0
	for _, ae := range active {
		if winding == 0 {
			start = ae.edge.x
		}
		winding += ae.edge.dir
		if winding != 0 {
			continue
		}
		lines = append(lines, Line{{jsRound(start), y}, {jsRound(ae.edge.x), y}})
	}
	return lines
}
