package mermaid_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/scene"
)

func TestGanttCompact(t *testing.T) {
	t.Parallel()
	items, report := drawFixture(t, "testdata/gantt/17-compact.mmd")
	if len(report) != 0 {
		t.Errorf("report: %v", report)
	}
	// The tasks of each section pack into rows. Design, then Test after it,
	// and Ship share a row, and Code takes a second. In Run, Monitor and
	// Report overlap, so they take a row each.
	rows := map[string]any{}
	for _, it := range items {
		if it.mark == scene.Rect || it.mark == scene.Symbol {
			if task, ok := it.datum["task"].(string); ok {
				rows[task] = it.datum["key"]
			}
		}
	}
	if rows["Design"] != rows["Test"] || rows["Design"] != rows["Ship"] || rows["Code"] == rows["Design"] ||
		rows["Monitor"] == rows["Report"] || rows["Monitor"] == rows["Design"] || rows["Monitor"] == rows["Code"] {
		t.Errorf("the rows of the tasks are %v", rows)
	}
	keys := map[any]bool{}
	for _, k := range rows {
		keys[k] = true
	}
	if len(rows) != 6 || len(keys) != 4 {
		t.Errorf("%d tasks in %d rows, not 6 in 4", len(rows), len(keys))
	}
	// Each task has its name once, on or beside its bar, with a halo, and
	// the axis writes no key of a row.
	for task := range rows {
		n := countDrawn(items, scene.Text, func(it drawnItem) bool {
			return it.text == task && !it.item.Stroke.IsZero() && it.role != "axis-label"
		})
		if n != 1 {
			t.Errorf("the name of %s draws %d times", task, n)
		}
	}
	if n := countDrawn(items, scene.Text, func(it drawnItem) bool { return it.role == "axis-label" && keys[it.text] }); n != 0 {
		t.Errorf("the axis writes %d keys of rows", n)
	}
}

func TestGanttSectionStyles(t *testing.T) {
	t.Parallel()
	const path = "testdata/gantt/18-section-styles.mmd"
	items, report := drawFixture(t, path)
	if len(report) != 0 {
		t.Errorf("report: %v", report)
	}
	// With numberSectionStyles 2, the sections take two colors in turn.
	colors := map[string]string{}
	for _, it := range items {
		if task, ok := it.datum["task"].(string); ok && it.mark == scene.Rect {
			colors[task] = it.item.Fill.Color
		}
	}
	if colors["First"] == "" || colors["First"] != colors["Third"] || colors["Second"] != colors["Fourth"] || colors["First"] == colors["Second"] {
		t.Errorf("the colors of the sections are %v", colors)
	}
	// rightPadding 0 leaves no space at the right of the time axis.
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := zuzu.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := c.Render(context.Background(), zuzu.WithLocation(time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	plot := s.Root.Items[0]
	if s.Width != math.Ceil(plot.X+640) {
		t.Errorf("the chart is %v wide, and the time axis ends at %v", s.Width, plot.X+640)
	}
}

func TestGanttLong(t *testing.T) {
	t.Parallel()
	src := "gantt\ndateFormat YYYY-MM-DD\ntickInterval 1day\nexcludes weekends\n" +
		"section Early\nStart : 2022-10-20, 12d\nsection Late\nFinish : 9999-10-01, 30d\n"
	report, err := compileMermaid(t, src, false)
	if err != nil {
		t.Fatal(err)
	}
	// Past 5 years, Mermaid draws no excluded days, and past 10000 ticks
	// its axis takes ticks of its own. Both are issues.
	for _, part := range []string{"so it draws no excluded days", "so the axis takes ticks of its own"} {
		if !reportHolds(report, part) {
			t.Errorf("expected an issue of %q, got %v", part, report)
		}
	}
	// Each bar draws at least 2 pixels wide, though its time is less than
	// a pixel of the axis.
	path := filepath.Join(t.TempDir(), "long.mmd")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	items, _ := drawFixture(t, path)
	if n := countDrawn(items, scene.Rect, func(it drawnItem) bool { return it.datum["task"] != nil && it.item.Width >= 1.99 }); n != 2 {
		t.Errorf("%d bars draw 2 pixels wide or more, not 2", n)
	}
	if n := countDrawn(items, scene.Rect, func(it drawnItem) bool {
		return it.datum["task"] == nil && it.datum["y0"] == nil && it.datum["start"] != nil
	}); n != 0 {
		t.Errorf("%d bands of excluded days draw", n)
	}
}
