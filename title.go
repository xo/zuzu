package zuzu

import (
	"encoding/json/v2"
	"maps"
	"math"
	"slices"

	"github.com/xo/zuzu/layout"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// The title of a group follows the properties of a title of Vega that its
// spec sets: its frame, its anchor, its orient and its offset, and the
// color, the font, the size, the weight, the alignment, the baseline, the
// angle, the line height, the limit and the dx and dy of its text. The look
// of zuzu applies where the spec sets nothing. The encode of a title, of
// the title of the chart too, applies to its text, its subtitle and its
// group.

// contentBounds returns the bounds of the marks and the guides that a group
// drew, in the frame of the group, but those of a mark that clips, which
// stay inside the group.
func (s *scope) contentBounds(drawn []scene.Mark) layout.Bounds {
	b := layout.Empty()
	st := s.rt.style()
	for i := range drawn {
		if drawn[i].Clip {
			continue
		}
		mb := st.MarkBounds(&drawn[i])
		if !mb.IsEmpty() {
			b.X1, b.Y1 = math.Min(b.X1, mb.X1), math.Min(b.Y1, mb.Y1)
			b.X2, b.Y2 = math.Max(b.X2, mb.X2), math.Max(b.Y2, mb.Y2)
		}
	}
	return b
}

// titleFrame applies the frame of the title of a group, which the anchor
// is relative to. The frame bounds, the default, is the box of the group
// with the marks and the guides that it drew. The frame group is the width
// or the height of the group. titleFrame sets the length of the edge in lt
// and returns where the edge starts.
func (s *scope) titleFrame(t *vega.Title, lt *layout.Title, b layout.Bounds) (float64, error) {
	frame, err := s.str(t.Frame, "bounds")
	if err != nil || frame != "bounds" || b.IsEmpty() {
		return 0, err
	}
	switch lt.Orient {
	case "left", "right":
		y1, y2 := math.Min(0, b.Y1), math.Max(lt.Height, b.Y2)
		lt.Height = y2 - y1
		return y1, nil
	}
	x1, x2 := math.Min(0, b.X1), math.Max(lt.Width, b.X2)
	lt.Width = x2 - x1
	return x1, nil
}

// moveTitle moves the text of a title along its edge.
func moveTitle(m *scene.Mark, orient string, d float64) {
	if d == 0 {
		return
	}
	for i := range m.Items {
		for j := range m.Items[i].Items {
			tm := &m.Items[i].Items[j]
			for k := range tm.Items {
				if orient == "left" || orient == "right" {
					tm.Items[k].Y += d
				} else {
					tm.Items[k].X += d
				}
			}
		}
	}
}

// titleLineHeight is the line height of a title, in sizes of its font, as
// the layout spaces its lines.
const titleLineHeight = 1.2

// titleTextProps are the properties of a title of Vega that set a property
// of its text of the same name, beyond those that styleGuide applies.
var titleTextProps = []string{"align", "angle", "baseline", "dx", "dy", "limit", "lineHeight"}

// styleGroupTitle applies the properties that the spec of the title of a
// group sets to its text and its subtitle.
func (s *scope) styleGroupTitle(m *scene.Mark, t *vega.Title) error {
	props := s.guideProps(nil, t.Extra)
	for k, v := range map[string]vega.Value{
		"color": t.Color, "font": t.Font, "fontSize": t.FontSize,
		"align": t.Align, "baseline": t.Baseline, "angle": t.Angle,
	} {
		if v.IsZero() {
			continue
		}
		x, err := s.value(v)
		if err != nil {
			return err
		}
		props[k] = x
	}
	if len(props) == 0 {
		return nil
	}
	_, size := props["fontSize"]
	_, lh := props["lineHeight"]
	_, base := props["baseline"]
	if size || lh || base {
		joinTitleLines(m)
	}
	s.styleGuide(m, props, titleParts)
	for _, tm := range titleMarks(m, "title-text") {
		for i := range tm.Items {
			st := &itemState{it: &tm.Items[i], set: map[string]bool{}}
			for _, k := range titleTextProps {
				if v, ok := props[k]; ok {
					setProp(st, k, v)
				}
			}
			if !lh && len(tm.Items[i].Text) > 1 {
				tm.Items[i].LineHeight = tm.Items[i].FontSize * titleLineHeight
			}
		}
	}
	return nil
}

// joinTitleLines makes the lines of the text of a title one item of text
// with a line for each, as Vega draws a title of several lines, so that a
// size, a line height and a baseline of the spec apply to the whole text.
// The item stays where the line nearest the group was.
func joinTitleLines(m *scene.Mark) {
	for _, tm := range titleMarks(m, "title-text") {
		if len(tm.Items) < 2 {
			continue
		}
		// The layout puts the lines from the farthest to the nearest, but
		// for a title at the bottom, whose first line is the nearest.
		near := tm.Items[len(tm.Items)-1]
		if near.Baseline == "top" {
			near = tm.Items[0]
		}
		var lines scene.Lines
		for _, it := range tm.Items {
			lines = append(lines, it.Text...)
		}
		near.Text = lines
		near.LineHeight = near.FontSize * titleLineHeight
		tm.Items = []scene.Item{near}
	}
}

// titleMarks returns the marks of a role inside a title.
func titleMarks(m *scene.Mark, role string) []*scene.Mark {
	var out []*scene.Mark
	for i := range m.Items {
		for j := range m.Items[i].Items {
			if tm := &m.Items[i].Items[j]; tm.Role == role {
				out = append(out, tm)
			}
		}
	}
	return out
}

// titleEncodeParts are the roles of the parts of a title that its encode
// names.
var titleEncodeParts = map[string]string{
	"title": "title-text", "subtitle": "title-subtitle", "group": "title",
}

// titleEncode applies the encode of a title to its parts. An encode that
// gives the sets of encodings, such as update, at its top encodes the text
// of the title, as Vega reads it.
func (s *scope) titleEncode(m *scene.Mark, t *vega.Title, path string) error {
	if len(t.Encode) == 0 {
		return nil
	}
	var top map[string]any
	if err := json.Unmarshal(t.Encode, &top); err != nil {
		return err
	}
	enc := vega.GuideEncode{}
	sets := slices.ContainsFunc(slices.Collect(maps.Keys(top)), func(k string) bool {
		return k == "enter" || k == "update" || k == "hover" || k == "exit"
	})
	if sets {
		var el vega.GuideElement
		if err := json.Unmarshal(t.Encode, &el); err != nil {
			return err
		}
		enc["title"] = el
	} else if err := json.Unmarshal(t.Encode, &enc); err != nil {
		return err
	}
	return s.guideEncode(m, enc, titleEncodeParts, nil, path+"/title", "a title")
}
