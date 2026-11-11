package layout

import (
	"math"
	"strings"

	"github.com/xo/zuzu/expr"

	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
)

// Axis is an axis, with the settings that the view resolved from the spec.
type Axis struct {
	// Orient is top, bottom, left or right.
	Orient string
	Scale  scale.Scale
	// Length is the size of the plot area along the axis, and Cross the
	// size across it, which the grid lines span.
	Length, Cross float64
	// Values are the values of the ticks, or nil for those of the scale.
	Values []any
	// Count is the number of ticks the spec asks for, or 0.
	Count float64
	// Format writes the label of a value.
	Format func(any) string
	// Grid, Labels and Domain say which parts the spec asks for. A domain
	// line is drawn only along a band axis.
	Grid, Labels, Domain bool
	Title                string
	// LabelAngle turns the labels, in degrees.
	LabelAngle float64
	// Overlap drops every other label, in turns, until no two overlap.
	Overlap bool
	// Offset moves the axis away from the plot area.
	Offset float64
	// BandPosition is where in a band a tick sits, from 0 to 1.
	BandPosition float64
	// LabelFlush is the distance from an end of the axis within which a
	// label aligns flush with that end, so that it stays inside the axis,
	// or a negative number for none.
	LabelFlush float64
	// LabelLimit is the widest that a label draws, in pixels, before it is
	// cut with an ellipsis, or 0 for no limit.
	LabelLimit float64
}

// vertical reports whether the axis runs up and down.
func (a *Axis) vertical() bool {
	return a.Orient == "left" || a.Orient == "right"
}

// ticks returns the values of the ticks and their positions along the
// axis. A scale of numbers has no more than MaxLabels ticks.
func (a *Axis) ticks() ([]any, []float64) {
	vals := a.Values
	if vals == nil {
		count := a.Count
		if count <= 0 {
			count = math.Max(2, math.Min(MaxLabels, a.Length/40))
			if a.Scale.Type() == scale.Trading {
				// A label of a trading scale is short and marks a period,
				// such as a week, so the axis takes one each 50 pixels.
				count = math.Max(2, a.Length/50)
			}
		}
		if !scale.Discrete(a.Scale.Type()) {
			count = math.Min(count, MaxLabels)
		}
		vals = a.Scale.Ticks(count)
	}
	bw := a.Scale.Bandwidth()
	var out []any
	var pos []float64
	for _, v := range vals {
		p, ok := a.Scale.Map(v).(float64)
		if !ok || math.IsNaN(p) {
			continue
		}
		p += bw * a.BandPosition
		if p < -0.5 || p > a.Length+0.5 {
			continue
		}
		out = append(out, v)
		pos = append(pos, p)
	}
	return out, pos
}

// zero returns the position of zero along the axis, for a linear scale
// whose domain holds zero.
func (a *Axis) zero() (float64, bool) {
	typ := strings.TrimPrefix(a.Scale.Type(), "sequential-")
	if typ != "linear" && typ != "pow" && typ != "sqrt" && typ != "symlog" {
		return 0, false
	}
	dom := a.Scale.Domain()
	if len(dom) < 2 {
		return 0, false
	}
	lo, hi := expr.Number(dom[0]), expr.Number(dom[len(dom)-1])
	if math.Min(lo, hi) > 0 || math.Max(lo, hi) < 0 {
		return 0, false
	}
	p, ok := a.Scale.Map(0.0).(float64)
	if !ok || math.IsNaN(p) {
		return 0, false
	}
	return p, true
}

// Axis draws an axis as a group of the role axis, with its grid, its
// labels, its base line and its title. The group sits at the origin of the
// plot area.
func (s Style) Axis(a *Axis) scene.Mark {
	t := s.Theme
	group := scene.Item{}
	vals, pos := a.ticks()
	if a.Grid {
		grid := scene.Mark{Type: scene.Rule, Role: "axis-grid"}
		for _, p := range pos {
			it := scene.Item{Stroke: scene.Paint{Color: t.Grid}, StrokeWidth: ptr(1)}
			if a.vertical() {
				it.X, it.X2, it.Y, it.Y2 = 0, a.Cross, p, p
			} else {
				it.X, it.X2, it.Y, it.Y2 = p, p, 0, a.Cross
			}
			grid.Items = append(grid.Items, it)
		}
		group.Items = append(group.Items, grid)
		if z, ok := a.zero(); ok {
			// The base line at zero of a scale of numbers is one line in
			// the color of an axis (docs/STYLE.md).
			it := scene.Item{Stroke: scene.Paint{Color: t.Axis}, StrokeWidth: ptr(1)}
			if a.vertical() {
				it.X, it.X2, it.Y, it.Y2 = 0, a.Cross, z, z
			} else {
				it.X, it.X2, it.Y, it.Y2 = z, z, 0, a.Cross
			}
			group.Items = append(group.Items, scene.Mark{Type: scene.Rule, Role: "axis-zero", Items: []scene.Item{it}})
		}
	}
	if a.Labels && len(vals) > 0 {
		group.Items = append(group.Items, s.axisLabels(a, vals, pos))
	}
	// The look of zuzu draws a base line along a band axis, and none along
	// an axis of numbers (docs/STYLE.md).
	if a.Domain && scale.Discrete(a.Scale.Type()) {
		dom := scene.Mark{Type: scene.Rule, Role: "axis-domain"}
		it := scene.Item{Stroke: scene.Paint{Color: t.Axis}, StrokeWidth: ptr(1)}
		switch a.Orient {
		case "bottom":
			it.X, it.X2, it.Y, it.Y2 = 0, a.Length, a.Cross+a.Offset, a.Cross+a.Offset
		case "top":
			it.X, it.X2, it.Y, it.Y2 = 0, a.Length, -a.Offset, -a.Offset
		case "left":
			it.X, it.X2, it.Y, it.Y2 = -a.Offset, -a.Offset, 0, a.Length
		default:
			it.X, it.X2, it.Y, it.Y2 = a.Cross+a.Offset, a.Cross+a.Offset, 0, a.Length
		}
		dom.Items = append(dom.Items, it)
		group.Items = append(group.Items, dom)
	}
	if a.Title != "" {
		group.Items = append(group.Items, s.axisTitle(a))
	}
	return scene.Mark{Type: scene.Group, Role: "axis", Items: []scene.Item{group}}
}

// axisLabels draws the labels of an axis, 8 from the plot area, without
// those that overlap when the axis asks.
func (s Style) axisLabels(a *Axis, vals []any, pos []float64) scene.Mark {
	m := scene.Mark{Type: scene.Text, Role: "axis-label"}
	color := s.Theme.Muted
	for i, v := range vals {
		str := a.Format(v)
		var it scene.Item
		switch a.Orient {
		case "bottom":
			it = s.label(pos[i], a.Cross+a.Offset+LabelPad, str, LabelSize, 400, color, "center", "top")
		case "top":
			it = s.label(pos[i], -a.Offset-LabelPad, str, LabelSize, 400, color, "center", "bottom")
		case "left":
			it = s.label(-a.Offset-LabelPad, pos[i], str, LabelSize, 400, color, "right", "middle")
		default:
			it = s.label(a.Cross+a.Offset+LabelPad, pos[i], str, LabelSize, 400, color, "left", "middle")
		}
		if a.LabelAngle != 0 {
			it.Angle = a.LabelAngle
			turnLabel(&it, a.Orient, a.LabelAngle)
		} else if a.LabelFlush >= 0 {
			flush(&it, a, pos[i])
		}
		it.Limit = a.LabelLimit
		it.Datum = map[string]any{"value": v, "label": str}
		m.Items = append(m.Items, it)
	}
	if a.Overlap {
		m.Items = s.dropOverlaps(m.Items)
	}
	return m
}

// flush aligns a label at an end of its axis with that end: the first label
// of a horizontal axis starts at the start, the last ends at the end, and
// the labels of a vertical axis hang below the top and sit on the bottom.
func flush(it *scene.Item, a *Axis, pos float64) {
	near := func(end float64) bool { return math.Abs(pos-end) <= a.LabelFlush }
	if a.vertical() {
		switch {
		case near(0):
			it.Baseline = "top"
		case near(a.Length):
			it.Baseline = "bottom"
		}
		return
	}
	switch {
	case near(0):
		it.Align = "left"
	case near(a.Length):
		it.Align = "right"
	}
}

// turnLabel sets the align and the baseline of a turned label, so that its
// end meets the axis.
func turnLabel(it *scene.Item, orient string, angle float64) {
	a := math.Mod(angle+360, 360)
	if orient == "bottom" || orient == "top" {
		switch {
		case a > 0 && a < 180:
			it.Align, it.Baseline = "left", "middle"
		case a > 180 && a < 360:
			it.Align, it.Baseline = "right", "middle"
		}
		if orient == "top" {
			switch it.Align {
			case "left":
				it.Align = "right"
			case "right":
				it.Align = "left"
			}
		}
	}
}

// dropOverlaps keeps every other label until no two labels overlap, and
// always keeps the first.
func (s Style) dropOverlaps(items []scene.Item) []scene.Item {
	overlaps := func(list []scene.Item) bool {
		for i := 1; i < len(list); i++ {
			a, b := s.textBounds(&list[i-1]), s.textBounds(&list[i])
			if a.X1 < b.X2+2 && b.X1 < a.X2+2 && a.Y1 < b.Y2 && b.Y1 < a.Y2 {
				return true
			}
		}
		return false
	}
	for len(items) > 2 && overlaps(items) {
		var next []scene.Item
		for i := 0; i < len(items); i += 2 {
			next = append(next, items[i])
		}
		items = next
	}
	return items
}

// axisTitle draws the title of an axis at the end of the axis, rather than
// along it: above a vertical axis, and at the right of a horizontal one,
// below its labels.
func (s Style) axisTitle(a *Axis) scene.Mark {
	m := scene.Mark{Type: scene.Text, Role: "axis-title"}
	color := s.Theme.Faint
	var it scene.Item
	switch a.Orient {
	case "bottom":
		it = s.label(a.Length, a.Cross+a.Offset+LabelPad+LabelSize+6, a.Title, SmallSize, 400, color, "right", "top")
	case "top":
		it = s.label(a.Length, -a.Offset-LabelPad-LabelSize-6, a.Title, SmallSize, 400, color, "right", "bottom")
	case "left":
		it = s.label(-a.Offset, -LabelPad-4, a.Title, SmallSize, 400, color, "left", "bottom")
	default:
		it = s.label(a.Cross+a.Offset, -LabelPad-4, a.Title, SmallSize, 400, color, "right", "bottom")
	}
	m.Items = []scene.Item{it}
	return m
}

// ptr returns a pointer to a number.
func ptr(f float64) *float64 {
	return new(f)
}
