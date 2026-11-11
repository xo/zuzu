package graph

// Crossings counts the pairs of edges of a layout whose routes cross, by
// the lines through their points. Two edges that share an end do not count
// at that end.
func Crossings(l *Layout) int {
	n := 0
	for i := range l.Edges {
		for j := range i {
			if routesCross(l.Edges[i].Points, l.Edges[j].Points) {
				n++
			}
		}
	}
	return n
}

// routesCross reports whether two polylines cross away from their ends.
func routesCross(a, b [][2]float64) bool {
	for i := 1; i < len(a); i++ {
		for j := 1; j < len(b); j++ {
			if segmentsCross(a[i-1], a[i], b[j-1], b[j]) {
				return true
			}
		}
	}
	return false
}

// segmentsCross reports whether two segments cross at a point inside both,
// not at an end of either.
func segmentsCross(p, q, r, s [2]float64) bool {
	d := (q[0]-p[0])*(s[1]-r[1]) - (q[1]-p[1])*(s[0]-r[0])
	if d == 0 {
		return false
	}
	t := ((r[0]-p[0])*(s[1]-r[1]) - (r[1]-p[1])*(s[0]-r[0])) / d
	u := ((r[0]-p[0])*(q[1]-p[1]) - (r[1]-p[1])*(q[0]-p[0])) / d
	const eps = 1e-6
	return t > eps && t < 1-eps && u > eps && u < 1-eps
}
