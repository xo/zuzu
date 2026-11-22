package layout

import (
	"github.com/xo/zuzu/scene"
)

// titleClear is the least space between the title of an axis and a label
// or a title that it moves clear of.
const titleClear = 4

// PlacedByLook reports whether the look places the title of the axis: the
// spec gives no titleX, titleY, titleAngle, titleAnchor or titleBaseline,
// which win over the place of the look (D46).
func (a *Axis) PlacedByLook() bool {
	return a.Title != "" && a.TitleX == nil && a.TitleY == nil && a.TitleAngle == nil &&
		a.TitleAnchor == "" && a.TitleBaseline == ""
}

// ClearTitles moves the title of each axis that the look places away from
// the plot area until it covers no label of any of the axes, and no title
// before it, so that a title never covers a label (D46). Such a covered
// label is of an axis across, such as an axis at the top under the title
// of a vertical axis, which sits above the plot area. The title of a
// vertical axis and of an axis at the top moves up, and the title of an
// axis at the bottom moves down. marks are the marks that Axis drew for
// axes, in the same order, and a nil axis keeps its title where it is.
func (s Style) ClearTitles(marks []scene.Mark, axes []*Axis) {
	var labels []Bounds
	for i := range marks {
		for _, m := range axisParts(&marks[i], "axis-label") {
			for j := range m.Items {
				labels = append(labels, s.textBounds(&m.Items[j]))
			}
		}
	}
	var titles []*scene.Item
	for i := range marks {
		for _, m := range axisParts(&marks[i], "axis-title") {
			for j := range m.Items {
				titles = append(titles, &m.Items[j])
			}
		}
	}
	for i := range marks {
		if i >= len(axes) || axes[i] == nil || !axes[i].PlacedByLook() {
			continue
		}
		dir := -1.0
		if axes[i].Orient == "bottom" {
			dir = 1
		}
		for _, m := range axisParts(&marks[i], "axis-title") {
			for j := range m.Items {
				s.clearTitle(&m.Items[j], dir, labels, titles)
			}
		}
	}
}

// clearTitle moves a title in the direction dir of y until it covers none
// of the labels and none of the other titles.
func (s Style) clearTitle(it *scene.Item, dir float64, labels []Bounds, titles []*scene.Item) {
	for range 16 {
		b := s.textBounds(it)
		move := 0.0
		check := func(o Bounds) {
			if !overlaps(b, o) {
				return
			}
			if dir < 0 {
				move = max(move, b.Y2-(o.Y1-titleClear))
			} else {
				move = max(move, (o.Y2+titleClear)-b.Y1)
			}
		}
		for _, l := range labels {
			check(l)
		}
		for _, t := range titles {
			if t != it {
				check(s.textBounds(t))
			}
		}
		if move <= 0 {
			return
		}
		it.Y += dir * move
	}
}

// overlaps reports whether two bounds overlap by more than a hair.
func overlaps(a, b Bounds) bool {
	if a.IsEmpty() || b.IsEmpty() {
		return false
	}
	return a.X1 < b.X2-0.5 && b.X1 < a.X2-0.5 && a.Y1 < b.Y2-0.5 && b.Y1 < a.Y2-0.5
}

// axisParts returns the marks of a role in the mark of an axis.
func axisParts(m *scene.Mark, role string) []*scene.Mark {
	var out []*scene.Mark
	for i := range m.Items {
		for j := range m.Items[i].Items {
			if c := &m.Items[i].Items[j]; c.Role == role {
				out = append(out, c)
			}
		}
	}
	return out
}
