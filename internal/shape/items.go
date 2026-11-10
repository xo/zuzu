package shape

import (
	"math"
	"strings"

	"github.com/xo/zuzu/scene"
)

// Box returns the position and the size of a rect, from x2 and y2 where the
// item has no width or height, and with no negative size.
func Box(it *scene.Item) (x, y, wd, ht float64) {
	x, y, wd, ht = it.X, it.Y, it.Width, it.Height
	if wd == 0 && it.X2 != 0 {
		wd = it.X2 - it.X
	}
	if ht == 0 && it.Y2 != 0 {
		ht = it.Y2 - it.Y
	}
	if wd < 0 {
		x, wd = x+wd, -wd
	}
	if ht < 0 {
		y, ht = y+ht, -ht
	}
	return x, y, wd, ht
}

// Corners returns the radius of each corner: top left, top right, bottom
// right and bottom left. A corner with its own radius overrides the radius of
// every corner.
func Corners(it *scene.Item) (tl, tr, br, bl float64) {
	pick := func(own float64) float64 {
		if own != 0 {
			return own
		}
		return it.CornerRadius
	}
	return pick(it.CornerRadiusTopLeft), pick(it.CornerRadiusTopRight),
		pick(it.CornerRadiusBottomRight), pick(it.CornerRadiusBottomLeft)
}

// RoundRect returns the path of a rect whose corners have radii of their
// own, each a quadratic curve. A radius is at most half the shorter side.
func RoundRect(x, y, wd, ht, tl, tr, br, bl float64) string {
	limit := func(r float64) float64 { return math.Max(0, math.Min(r, math.Min(wd, ht)/2)) }
	tl, tr, br, bl = limit(tl), limit(tr), limit(br), limit(bl)
	return "M" + pt(x+tl, y) + "H" + num(x+wd-tr) +
		quad(x+wd, y, x+wd, y+tr, tr) + "V" + num(y+ht-br) +
		quad(x+wd, y+ht, x+wd-br, y+ht, br) + "H" + num(x+bl) +
		quad(x, y+ht, x, y+ht-bl, bl) + "V" + num(y+tl) +
		quad(x, y, x+tl, y, tl) + "Z"
}

// quad returns a quadratic curve through the corner (cx, cy) to (x, y), or a
// line when the radius is zero.
func quad(cx, cy, x, y, r float64) string {
	if r == 0 {
		return "L" + pt(x, y)
	}
	return "Q" + pt(cx, cy) + " " + pt(x, y)
}

// RuleEnd returns the end of a rule, from (x, y) to (x2, y2). An end that
// the item does not set is the start.
func RuleEnd(it *scene.Item) (float64, float64) {
	x2, y2 := it.X2, it.Y2
	if x2 == 0 && it.Width == 0 {
		x2 = it.X
	}
	if y2 == 0 && it.Height == 0 {
		y2 = it.Y
	}
	return x2, y2
}

// Runs splits the items of a line or an area where an item is not defined.
func Runs(items []scene.Item) [][]scene.Item {
	var out [][]scene.Item
	start := -1
	for i, it := range items {
		if it.Defined != nil && !*it.Defined {
			if start >= 0 {
				out = append(out, items[start:i])
			}
			start = -1
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, items[start:])
	}
	return out
}

// Line returns the path of a line mark: one path through its items, with
// the interpolation of the first item. An item that is not defined breaks
// the line. It returns false for an interpolation that it does not know.
func Line(items []scene.Item) (string, bool) {
	first := &items[0]
	var d strings.Builder
	for _, run := range Runs(items) {
		pts := make([][2]float64, len(run))
		for i, it := range run {
			pts[i] = [2]float64{it.X, it.Y}
		}
		seg, ok := Curve(first.Interpolate, pts, true, first.Tension)
		if !ok {
			return "", false
		}
		d.WriteString(seg)
	}
	return d.String(), true
}

// Area returns the path of an area mark: from the line of x and y along its
// items, back along the line of x and y2, or of x2 and y for an area whose
// orient is horizontal.
func Area(items []scene.Item) (string, bool) {
	first := &items[0]
	var d strings.Builder
	for _, run := range Runs(items) {
		top := make([][2]float64, len(run))
		base := make([][2]float64, len(run))
		for i, it := range run {
			top[i] = [2]float64{it.X, it.Y}
			if first.Orient == "horizontal" {
				base[len(run)-1-i] = [2]float64{it.X2, it.Y}
			} else {
				base[len(run)-1-i] = [2]float64{it.X, it.Y2}
			}
		}
		a, ok := Curve(first.Interpolate, top, true, first.Tension)
		if !ok {
			return "", false
		}
		b, ok := Curve(first.Interpolate, base, false, first.Tension)
		if !ok {
			return "", false
		}
		d.WriteString(a + b + "Z")
	}
	return d.String(), true
}

// TrailOf returns the path of a trail mark: a line through its items whose
// width is the size of each item.
func TrailOf(items []scene.Item) string {
	var d strings.Builder
	for _, run := range Runs(items) {
		pts := make([][3]float64, len(run))
		for i, it := range run {
			size := 2.0
			if it.Size != nil {
				size = *it.Size
			}
			pts[i] = [3]float64{it.X, it.Y, size}
		}
		d.WriteString(Trail(pts))
	}
	return d.String()
}

// SymbolOf returns the path of a symbol item around the origin, from its
// shape and its size, and false for a shape that it does not know.
func SymbolOf(it *scene.Item) (string, bool) {
	size := 64.0
	if it.Size != nil {
		size = *it.Size
	}
	return Symbol(it.Shape, size)
}

// ArcOf returns the path of an arc item around its position.
func ArcOf(it *scene.Item) string {
	return Arc(it.InnerRadius, it.OuterRadius, it.StartAngle, it.EndAngle, it.PadAngle, it.CornerRadius)
}
