package zuzu

import (
	"strings"
	"testing"
)

// groupTitleSpec is a spec with a group of width 100 and height 50, which
// draws a text from x -60 to 60, above the group, and a title that %s
// gives.
const groupTitleSpec = `{"$schema": "https://vega.github.io/schema/vega/v5.json",
	"width": 200, "height": 200,
	"marks": [{"type": "group",
		"encode": {"update": {"width": {"value": 100}, "height": {"value": 50}}},
		"title": %s,
		"marks": [{"type": "text", "role": "above", "encode": {"update": {
			"x": {"value": 0}, "y": {"value": -30}, "align": {"value": "center"},
			"fontSize": {"value": 10}, "text": {"value": "a label that is wide"}}}}]}]}`

// TestGroupTitleProps checks that the title of a group follows the
// properties that its spec sets, the frame, the anchor, the color, the
// size, the weight, the line height, the baseline and the dy, and that it
// stands above the marks of the group.
func TestGroupTitleProps(t *testing.T) {
	t.Parallel()
	s := specScene(t, strings.Replace(groupTitleSpec, "%s", `{"text": ["One", "Two"], "frame": "group",
		"color": "#d0d0d2", "fontSize": 12, "fontWeight": "600", "baseline": "line-bottom",
		"lineHeight": 20, "dy": -5}`, 1))
	above := roleItems(s, "above")
	texts := roleItems(s, "title-text")
	if len(above) != 1 || len(texts) != 1 {
		t.Fatalf("%d labels and %d items of the title, want 1 and 1", len(above), len(texts))
	}
	it := texts[0]
	if it.Fill.Color != "#d0d0d2" || it.FontSize != 12 || it.FontWeight != "600" ||
		it.Baseline != "line-bottom" || it.LineHeight != 20 || it.Dy != -5 {
		t.Errorf("the title = %+v, want the properties of the spec", it)
	}
	if strings.Join(it.Text, "|") != "One|Two" {
		t.Errorf("the lines of the title = %q, want One|Two in one item", it.Text)
	}
	// The frame group puts the middle of the title at the middle of the
	// width of the group.
	if it.X != 50 || it.Align != "center" {
		t.Errorf("the title is at x %v, aligned %q, want 50 and center", it.X, it.Align)
	}
	// The label reaches up to about -30 - 10.
	if it.Y > -40 {
		t.Errorf("the title is at y %v, want above the label at -30", it.Y)
	}
}

// TestGroupTitleFrame checks that the frame bounds, the default, puts the
// anchor of the title of a group relative to the bounds of what the group
// draws, and that the look applies where the spec sets nothing.
func TestGroupTitleFrame(t *testing.T) {
	t.Parallel()
	s := specScene(t, strings.Replace(groupTitleSpec, "%s", `{"text": "Name"}`, 1))
	texts := roleItems(s, "title-text")
	if len(texts) != 1 {
		t.Fatalf("%d items of the title, want 1", len(texts))
	}
	it := texts[0]
	// The label starts left of 0, so the middle of the bounds is left of
	// the middle of the group.
	if it.X >= 50 || it.Align != "center" {
		t.Errorf("the title is at x %v, aligned %q, want left of 50 and center", it.X, it.Align)
	}
	if it.FontSize != 16 || it.Fill.Color == "" {
		t.Errorf("the title = %+v, want the size and a color of the look", it)
	}
}

// TestTitleEncode checks that the encode of a title applies to its text,
// both as the encode of its parts and as sets of encodings at its top.
func TestTitleEncode(t *testing.T) {
	t.Parallel()
	for _, enc := range []string{
		`{"title": {"update": {"fill": {"value": "blue"}, "dx": {"value": -1}}}}`,
		`{"update": {"fill": {"value": "blue"}, "dx": {"value": -1}}}`,
	} {
		for _, at := range []string{"group", "root"} {
			title := `{"text": "Name", "encode": ` + enc + `}`
			src := strings.Replace(groupTitleSpec, "%s", title, 1)
			if at == "root" {
				src = `{"$schema": "https://vega.github.io/schema/vega/v5.json",
					"width": 200, "height": 200, "title": ` + title + `}`
			}
			texts := roleItems(specScene(t, src), "title-text")
			if len(texts) != 1 || texts[0].Fill.Color != "blue" || texts[0].Dx != -1 {
				t.Errorf("%s of the %s: the title = %+v, want blue with the dx -1", enc, at, texts)
			}
		}
	}
}

// TestGridFlush checks that the bounds flush of a layout give each cell
// only its width and its height, so that a label past the width of a cell
// does not push the next cell away.
func TestGridFlush(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		bounds string
		flush  bool
	}{{"flush", true}, {"full", false}} {
		s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
			"width": 200, "height": 200,
			"layout": {"columns": 2, "padding": 10, "bounds": "`+c.bounds+`"},
			"data": [{"name": "t", "values": [{"k": "a"}, {"k": "b"}]}],
			"marks": [{"type": "group", "from": {"facet": {"data": "t", "name": "f", "groupby": "k"}},
				"encode": {"update": {"width": {"value": 50}, "height": {"value": 50}}},
				"marks": [{"type": "text", "encode": {"update": {"x": {"value": 40},
					"text": {"value": "a label that is wide"}}}}]}]}`)
		cells := roleItems(s, "scope")
		if len(cells) != 2 {
			t.Fatalf("%d cells, want 2", len(cells))
		}
		if got := cells[1].X - cells[0].X; (got == 60) != c.flush {
			t.Errorf("the bounds %s: the second cell is %v past the first, want 60 only for flush", c.bounds, got)
		}
	}
}

// TestGridIssues checks that the grid reports an align and a center of a
// layout that it does not follow, and that the align all of cells of one
// size, as in a facet, is not an issue (hard rule 4).
func TestGridIssues(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		layout string
		want   string
	}{
		{`"align": "all"`, ""},
		{`"align": "each"`, ""},
		{`"align": "none"`, "align"},
		{`"align": "all", "center": true`, "center"},
	} {
		src := `{"$schema": "https://vega.github.io/schema/vega/v5.json", "width": 200, "height": 200,
			"layout": {"columns": 2, ` + c.layout + `},
			"data": [{"name": "t", "values": [{"k": "a"}, {"k": "b"}]}],
			"marks": [{"type": "group", "from": {"facet": {"data": "t", "name": "f", "groupby": "k"}},
				"encode": {"update": {"width": {"value": 50}, "height": {"value": 50}}}}]}`
		_, report, err := svgOf(t, src, false)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, is := range report {
			b.WriteString(is.Error())
		}
		got := b.String()
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: the report is %q, want %q", c.layout, got, c.want)
		}
	}
}

// TestGridFlushKeepsRoomForAxisTitle checks that a grid with the bounds
// flush keeps room above a cell for the title of its vertical axis, which
// the look puts above the plot, so that the title does not cover the cell
// above, as in concat_marginal_histograms.
func TestGridFlushKeepsRoomForAxisTitle(t *testing.T) {
	t.Parallel()
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json",
		"layout": {"columns": 1, "padding": 10, "bounds": "flush"},
		"marks": [
			{"type": "group", "encode": {"update": {"width": {"value": 100}, "height": {"value": 40}}}},
			{"type": "group", "encode": {"update": {"width": {"value": 100}, "height": {"value": 80}}},
				"scales": [{"name": "y", "type": "linear", "domain": [0, 10], "range": [80, 0]}],
				"axes": [{"scale": "y", "orient": "left", "title": "A title"}]}]}`)
	cells := roleItems(s, "scope")
	if len(cells) != 2 {
		t.Fatalf("%d cells, want 2", len(cells))
	}
	if gap := cells[1].Y - (cells[0].Y + cells[0].Height); gap <= 10 {
		t.Errorf("the second cell is %v below the first, want more than the padding of 10 for the title", gap)
	}
}
