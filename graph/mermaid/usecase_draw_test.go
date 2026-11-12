package mermaid_test

import (
	"testing"

	"github.com/xo/zuzu/scene"
)

// TestUsecasePackageDraws checks that a boundary of the type package draws
// as a package of UML: a tab at its top left that holds the title, over a
// panel as wide as the boundary (D32). A boundary of the type rect draws as
// one panel.
func TestUsecasePackageDraws(t *testing.T) {
	t.Parallel()
	src := "usecase-beta\nsystemBoundary pay[\"Payment service\"]@{ type: package }\n  actor Clerk\n  Authorize\nend\n" +
		"systemBoundary ship[\"Shipping\"]@{ type: rect }\n  Send\nend\nClerk --> Authorize\nClerk --> Send\n"
	items, report, err := drawDiagram(t, src, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	boxes := map[string][]*scene.Item{}
	titles := map[string]*scene.Item{}
	for _, it := range items {
		key, _ := it.datum["key"].(string)
		if it.datum["kind"] != "cluster" {
			continue
		}
		switch it.mark {
		case scene.Rect:
			boxes[key] = append(boxes[key], it.item)
		case scene.Text:
			titles[key] = it.item
		}
	}
	if len(boxes["ship"]) != 1 {
		t.Errorf("the boundary ship draws %d boxes, not 1", len(boxes["ship"]))
	}
	if len(boxes["pay"]) != 2 {
		t.Fatalf("the package pay draws %d boxes, not a tab and a panel", len(boxes["pay"]))
	}
	tab, panel := boxes["pay"][0], boxes["pay"][1]
	title := titles["pay"]
	if title == nil {
		t.Fatal("the package pay draws no title")
	}
	tabBottom := tab.Y + tab.Height
	switch {
	case tab.X != panel.X || tab.Y >= panel.Y:
		t.Errorf("the tab at %v, %v is not at the top left of the panel at %v, %v", tab.X, tab.Y, panel.X, panel.Y)
	case tab.Width >= panel.Width:
		t.Errorf("the tab is %v wide, and the panel %v", tab.Width, panel.Width)
	case title.Y < tab.Y || title.Y > panel.Y || title.X < tab.X || title.X > tab.X+tab.Width:
		t.Errorf("the title at %v, %v is not in the tab", title.X, title.Y)
	case tabBottom <= panel.Y:
		t.Errorf("the tab ends at %v, above the top of the panel at %v", tabBottom, panel.Y)
	}
}
