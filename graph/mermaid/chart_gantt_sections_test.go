package mermaid_test

import (
	"testing"

	"github.com/xo/zuzu/scene"
)

func TestGanttSections(t *testing.T) {
	t.Parallel()
	items := renderMermaid(t, "gantt\ndateFormat YYYY-MM-DD\n"+
		"section Design\nSketch : 2024-01-01, 2d\nReview : 1d\n"+
		"section Build\nCode : 2024-01-04, 3d\n"+
		"section Ship\nRelease : 2024-01-08, 1d\nAnnounce : 1d\n")
	// Each name of a section draws once, at the left of the names of the
	// tasks, and no legend names them again.
	for _, s := range []string{"Design", "Build", "Ship"} {
		if n := textCount(items, s); n != 1 {
			t.Errorf("the section %q draws its name %d times", s, n)
		}
	}
	// A band runs behind every other section, from the left of its name to
	// the end of the time axis, in a tint of the color of the section.
	bands := map[string]*scene.Item{}
	var left float64
	for _, it := range items {
		if it.mark == scene.Text && it.text == "Design" {
			left = it.item.X
		}
	}
	for _, it := range items {
		if it.mark == scene.Rect && it.datum["y0"] != nil {
			name, _ := it.datum["section"].(string)
			bands[name] = it.item
		}
	}
	if len(bands) != 2 || bands["Design"] == nil || bands["Ship"] == nil {
		t.Fatalf("the bands are %v, not of Design and Ship", bands)
	}
	for name, b := range bands {
		if b.X >= left || b.X2 != 640 || b.FillOpacity == nil || *b.FillOpacity != 0.14 {
			t.Errorf("the band of %s runs from %v to %v with the opacity %v", name, b.X, b.X2, b.FillOpacity)
		}
	}
	// The bar of each task is inside the band of its section.
	for _, it := range items {
		if it.mark != scene.Rect || it.datum["task"] == nil {
			continue
		}
		name, _ := it.datum["section"].(string)
		b := bands[name]
		if b == nil {
			continue
		}
		if mid := it.item.Y + it.item.Height/2; mid < b.Y || mid > b.Y2 {
			t.Errorf("the task %v is at %v, outside its band from %v to %v", it.datum["task"], mid, b.Y, b.Y2)
		}
	}
	// A gantt with no section draws no band and no name.
	items = renderMermaid(t, "gantt\ndateFormat YYYY-MM-DD\nSketch : 2024-01-01, 2d\nReview : 1d\n")
	for _, it := range items {
		if it.datum["y0"] != nil || it.datum["y"] != nil {
			t.Errorf("a gantt with no section draws %s %q", it.mark, it.text)
		}
	}
}
