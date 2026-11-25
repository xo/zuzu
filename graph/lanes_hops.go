package graph

import (
	"math"
	"slices"
	"strings"
)

// The hops of the routes of a layout in lanes, as the setting
// swimlane.lineHops of Mermaid 12.3 draws them.
const (
	// hopRadius is the radius of a hop.
	hopRadius = 6.0
	// hopClear is the least space between a hop and a corner.
	hopClear = 2.0
	// hopLeast is the least radius of a hop, as a part of hopRadius. A
	// hop with no room for it draws no hop.
	hopLeast = 0.6
	// hopEnds is how close to an end of a part of a route a crossing can be
	// and still not count.
	hopEnds = 1e-6
)

// laneHop is a hop on a part of a route: the index of the part, and the
// place of the crossing along the part, from 0 to 1, and its point.
type laneHop struct {
	part  int
	t     float64
	point [2]float64
}

// hopCurve reports whether a route of a curve can take a hop, as in
// Mermaid: the curve basis, which draws round corners, and the curves that
// draw straight lines.
func hopCurve(curve string) bool {
	switch curve {
	case "", "basis", "linear", "step", "stepBefore", "stepAfter":
		return true
	}
	return false
}

// cornerCut returns how far the round corner at the point k of a route
// reaches along the parts on each side of it, or 0 for no corner, as
// lanePath draws it.
func cornerCut(pts [][2]float64, k int) float64 {
	if k <= 0 || k >= len(pts)-1 {
		return 0
	}
	p, c, n := pts[k-1], pts[k], pts[k+1]
	d1, d2 := math.Hypot(c[0]-p[0], c[1]-p[1]), math.Hypot(n[0]-c[0], n[1]-c[1])
	if d1 == 0 || d2 == 0 {
		return 0
	}
	return math.Min(laneCorner, math.Min(d1, d2)/2)
}

// inCorner reports whether a place on a part of a route lies in a round
// corner at either end of the part.
func inCorner(pts [][2]float64, curve string, part int, t float64) bool {
	if curve != "" && curve != "basis" {
		return false
	}
	a, b := pts[part], pts[part+1]
	length := math.Hypot(b[0]-a[0], b[1]-a[1])
	d := t * length
	return d < cornerCut(pts, part) || length-d < cornerCut(pts, part+1)
}

// crossAt returns where two parts of routes cross: the place along each,
// from 0 to 1, when they cross away from their ends.
func crossAt(a1, a2, b1, b2 [2]float64) (float64, float64, bool) {
	dxa, dya := a2[0]-a1[0], a2[1]-a1[1]
	dxb, dyb := b2[0]-b1[0], b2[1]-b1[1]
	den := dxa*dyb - dya*dxb
	if den == 0 {
		return 0, 0, false
	}
	dx, dy := b1[0]-a1[0], b1[1]-a1[1]
	ta, tb := (dx*dyb-dy*dxb)/den, (dx*dya-dy*dxa)/den
	if ta <= hopEnds || ta >= 1-hopEnds || tb <= hopEnds || tb >= 1-hopEnds {
		return 0, 0, false
	}
	return ta, tb, true
}

// flat reports whether a part of a route runs more across than down.
func flat(a, b [2]float64) bool {
	return math.Abs(b[0]-a[0]) >= math.Abs(b[1]-a[1])
}

// laneHops finds the hops of the routes of a layout, as Mermaid 12.3 finds
// them: where two routes cross away from the ends of their parts and from
// their round corners, the route whose part runs across takes the hop. Of
// two parts that run the same way, the later route takes it.
func laneHops(edges []EdgePlace, curves []string) [][]laneHop {
	out := make([][]laneHop, len(edges))
	for i := range edges {
		a := edges[i].Points
		for j := i + 1; j < len(edges); j++ {
			b := edges[j].Points
			for si := 0; si+1 < len(a); si++ {
				for sj := 0; sj+1 < len(b); sj++ {
					ta, tb, ok := crossAt(a[si], a[si+1], b[sj], b[sj+1])
					if !ok || inCorner(a, curves[i], si, ta) || inCorner(b, curves[j], sj, tb) {
						continue
					}
					p := [2]float64{a[si][0] + ta*(a[si+1][0]-a[si][0]), a[si][1] + ta*(a[si+1][1]-a[si][1])}
					fa, fb := flat(a[si], a[si+1]), flat(b[sj], b[sj+1])
					if fa != fb && fa {
						out[i] = append(out[i], laneHop{si, ta, p})
					} else {
						out[j] = append(out[j], laneHop{sj, tb, p})
					}
				}
			}
		}
	}
	return out
}

// hopPath returns the path of a route with its hops: an arc over each
// crossing for LaneHopArc, and a gap for LaneHopGap. A hop takes the room
// between the corners of its part, and two hops close together share the
// room between them. A route with no hop, or of a curve that takes none,
// draws as lanePath draws it.
func hopPath(curve, style string, pts [][2]float64, hops []laneHop) string {
	if len(hops) == 0 || !hopCurve(curve) || len(pts) < 2 {
		return lanePath(curve, pts)
	}
	round := curve == "" || curve == "basis"
	type hop struct {
		d, r float64
		at   [2]float64
	}
	var b strings.Builder
	b.WriteString("M" + num(pts[0][0]) + "," + num(pts[0][1]))
	for k := 0; k+1 < len(pts); k++ {
		p, q := pts[k], pts[k+1]
		length := math.Hypot(q[0]-p[0], q[1]-p[1])
		var ux, uy float64
		if length > 0 {
			ux, uy = (q[0]-p[0])/length, (q[1]-p[1])/length
		}
		from, to := 0.0, length
		if round {
			from, to = cornerCut(pts, k), length-cornerCut(pts, k+1)
		}
		var on []hop
		for _, h := range hops {
			if h.part != k {
				continue
			}
			d := h.t * length
			r := math.Min(hopRadius, math.Min(d-from, to-d)-hopClear)
			if r >= hopRadius*hopLeast {
				on = append(on, hop{d, r, h.point})
			}
		}
		slices.SortStableFunc(on, func(x, y hop) int {
			switch {
			case x.d < y.d:
				return -1
			case x.d > y.d:
				return 1
			}
			return 0
		})
		for i := 0; i+1 < len(on); i++ {
			if gap := on[i+1].d - on[i].d; on[i].r+on[i+1].r > gap {
				on[i].r, on[i+1].r = math.Min(on[i].r, gap/2), math.Min(on[i+1].r, gap/2)
			}
		}
		sweep := "0"
		if flat(p, q) && q[0] >= p[0] || !flat(p, q) && q[1] >= p[1] {
			sweep = "1"
		}
		for _, h := range on {
			if h.r < hopRadius*hopLeast {
				continue
			}
			pre := [2]float64{h.at[0] - ux*h.r, h.at[1] - uy*h.r}
			post := [2]float64{h.at[0] + ux*h.r, h.at[1] + uy*h.r}
			b.WriteString("L" + num(pre[0]) + "," + num(pre[1]))
			if style == LaneHopGap {
				b.WriteString("M" + num(post[0]) + "," + num(post[1]))
				continue
			}
			b.WriteString("A" + num(h.r) + "," + num(h.r) + " 0 0 " + sweep + " " + num(post[0]) + "," + num(post[1]))
		}
		if c := cornerCut(pts, k+1); round && c > 0 {
			n := pts[k+2]
			d2 := math.Hypot(n[0]-q[0], n[1]-q[1])
			s := [2]float64{q[0] - ux*c, q[1] - uy*c}
			e := [2]float64{q[0] + (n[0]-q[0])/d2*c, q[1] + (n[1]-q[1])/d2*c}
			b.WriteString("L" + num(s[0]) + "," + num(s[1]) + "Q" + num(q[0]) + "," + num(q[1]) + " " + num(e[0]) + "," + num(e[1]))
			continue
		}
		b.WriteString("L" + num(q[0]) + "," + num(q[1]))
	}
	return b.String()
}
