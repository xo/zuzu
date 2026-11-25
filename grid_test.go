package zuzu

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// gridCell is a cell of a test of the grid: its width and its height, and
// how far a rect inside it reaches past its left, top, right and bottom.
type gridCell struct {
	w, h, left, top, right, bottom float64
}

// gridScene renders a layout of cells, with a number of headers, footers
// and titles of each role, and returns the position of each group of the
// layout, as "role x,y", in the order of the spec. Each header holds a rect
// of 15 by 10, or 10 by 12 for a column, on its outer side.
func gridScene(t *testing.T, lay string, cells []gridCell, extra map[string]int) []string {
	t.Helper()
	var marks []any
	for _, role := range []string{"row-header", "column-header", "row-footer", "column-footer", "row-title", "column-title"} {
		row, foot := strings.HasPrefix(role, "row"), strings.Contains(role, "footer")
		for range extra[role] {
			w, h := 30.0, 0.0
			rect := map[string]any{"x": map[string]any{"value": 0}, "width": map[string]any{"value": 10}, "height": map[string]any{"value": 12}, "y": map[string]any{"value": -12}}
			if foot {
				rect["y"] = map[string]any{"value": 0}
			}
			if row {
				w, h = 0, 30
				rect = map[string]any{"y": map[string]any{"value": 0}, "height": map[string]any{"value": 10}, "width": map[string]any{"value": 15}, "x": map[string]any{"value": -15}}
				if foot {
					rect["x"] = map[string]any{"value": 0}
				}
			}
			marks = append(marks, map[string]any{
				"type": "group", "role": role,
				"encode": map[string]any{"update": map[string]any{"width": map[string]any{"value": w}, "height": map[string]any{"value": h}}},
				"marks":  []any{map[string]any{"type": "rect", "encode": map[string]any{"update": rect}}},
			})
		}
	}
	for _, c := range cells {
		marks = append(marks, map[string]any{
			"type":   "group",
			"encode": map[string]any{"update": map[string]any{"width": map[string]any{"value": c.w}, "height": map[string]any{"value": c.h}}},
			"marks": []any{map[string]any{"type": "rect", "encode": map[string]any{"update": map[string]any{
				"x": map[string]any{"value": -c.left}, "x2": map[string]any{"value": c.w + c.right},
				"y": map[string]any{"value": -c.top}, "y2": map[string]any{"value": c.h + c.bottom},
			}}}},
		})
	}
	m, err := json.Marshal(marks)
	if err != nil {
		t.Fatal(err)
	}
	s := specScene(t, `{"$schema": "https://vega.github.io/schema/vega/v5.json", "autosize": "pad",
		"layout": `+lay+`, "marks": `+string(m)+`}`)
	var out []string
	root := &s.Root.Items[0]
	for i := range root.Items {
		mk := &root.Items[i]
		for j := range mk.Items {
			out = append(out, fmt.Sprintf("%s %g,%g", mk.Role, mk.Items[j].X, mk.Items[j].Y))
		}
	}
	return out
}

// checkGrid runs gridScene and compares its positions with the positions
// that vega.js gives for the same spec.
func checkGrid(t *testing.T, lay string, cells []gridCell, extra map[string]int, want string) {
	t.Helper()
	got := strings.Join(gridScene(t, lay, cells, extra), "\n")
	parts := strings.Split(want, ";")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	if want = strings.Join(parts, "\n"); got != want {
		t.Errorf("the layout %s:\ngot:\n%s\nwant:\n%s", lay, got, want)
	}
}

// gridCells are five cells of more than one size, in the tests of the grid.
// Some of them draw past their box.
var gridCells = []gridCell{{50, 40, 5, 3, 0, 0}, {80, 30, 0, 0, 10, 0}, {60, 60, 7, 0, 0, 4}, {40, 20, 0, 6, 0, 0}, {30, 30, 2, 2, 0, 0}}

// TestGridAlign checks the align all, each and none, for both directions
// and for the columns apart from the rows, and the padding of the rows
// apart from that of the columns. The positions are those that vega.js
// 6.4 gives for the same spec.
func TestGridAlign(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		align, want string
	}{
		{`"each"`, `scope 0,0; scope 80,0; scope 0,56; scope 80,56; scope 0,132`},
		{`"all"`, `scope 0,0; scope 110,0; scope 0,80; scope 110,80; scope 0,160`},
		{`"none"`, `scope 0,0; scope 80,0; scope 0,50; scope 80,56; scope 0,126`},
		{`{"row": "all", "column": "none"}`, `scope 0,0; scope 80,0; scope 0,80; scope 80,80; scope 0,160`},
	} {
		lay := `{"columns": 2, "padding": {"row": 10, "column": 20}, "align": ` + c.align + `}`
		checkGrid(t, lay, gridCells, nil, c.want)
	}
}

// TestGridNoneFollows checks that the align none places a cell after the
// cell before it in its column, with its own reach above its origin, as
// vega.js does.
func TestGridNoneFollows(t *testing.T) {
	t.Parallel()
	cells := []gridCell{{50, 40, 0, 0, 0, 0}, {30, 10, 0, 0, 0, 0}, {20, 20, 0, 0, 0, 0}, {60, 50, 0, 6, 0, 0}, {40, 30, 3, 0, 0, 0}, {10, 30, 5, 2, 0, 0}}
	checkGrid(t, `{"columns": 2, "padding": 10, "align": "none"}`, cells, nil,
		`scope 0,0; scope 60,0; scope 0,50; scope 60,56; scope 0,110; scope 65,118`)
	checkGrid(t, `{"columns": 2, "padding": 10, "align": "each"}`, cells, nil,
		`scope 0,0; scope 65,0; scope 0,56; scope 65,56; scope 0,118; scope 65,118`)
	checkGrid(t, `{"columns": 2, "padding": 10, "align": "all"}`, cells, nil,
		`scope 0,0; scope 75,0; scope 0,66; scope 75,66; scope 0,132; scope 75,132`)
}

// TestGridDefaults checks the defaults of Vega: the align none, no padding,
// and one row when the layout gives no columns.
func TestGridDefaults(t *testing.T) {
	t.Parallel()
	checkGrid(t, `{"columns": 2}`, gridCells, nil,
		`scope 0,0; scope 60,0; scope 0,40; scope 60,46; scope 0,106`)
	cells := []gridCell{{50, 40, 0, 0, 0, 0}, {30, 10, 0, 0, 0, 0}, {20, 20, 0, 0, 0, 0}, {60, 50, 0, 6, 0, 0}, {40, 30, 4, 0, 0, 0}, {10, 30, 0, 0, 0, 0}}
	for _, lay := range []string{`{"padding": 10, "align": "each"}`, `{"padding": 10, "align": "each", "columns": 0}`, `{"padding": 10, "align": "each", "columns": 10}`} {
		checkGrid(t, lay, cells, nil, `scope 0,0; scope 60,0; scope 100,0; scope 130,0; scope 204,0; scope 254,0`)
	}
}

// TestGridRounding checks that the grid rounds the reach of each cell up
// to whole pixels, and not the padding, as vega.js does.
func TestGridRounding(t *testing.T) {
	t.Parallel()
	cells := []gridCell{{60.4, 10.2, 0, 0, 0, 0}, {1, 1, 0, 0, 0, 0}, {50, 10.7, 0, 0, 0, 0}, {1, 1, 0, 0, 0, 0}}
	checkGrid(t, `{"columns": 2, "padding": 10.5, "align": "each"}`, cells, nil,
		`scope 0,0; scope 71.5,0; scope 0,21.5; scope 71.5,21.5`)
	checkGrid(t, `{"columns": 2, "padding": 10, "align": "each", "bounds": "flush"}`, cells, nil,
		`scope 0,0; scope 71,0; scope 0,21; scope 71,21`)
	cells = []gridCell{{60, 10, 0.3, 0.2, 0.4, 0}, {1, 1, 1.2, 1.7, 0, 0}, {50, 10, 0, 0, 0, 0}, {1, 1, 0, 0, 0, 0}}
	checkGrid(t, `{"columns": 2, "padding": 10, "align": "each"}`, cells, nil,
		`scope 0,0; scope 73,0; scope 0,20; scope 73,20`)
}

// TestGridCenter checks that the center moves each cell to the middle of
// its column and of its row, as the documentation of Vega says, for both
// directions or for one, and that the align none centers nothing. vega.js
// 6.4 centers only the cells of the first column and of the first row, so
// its positions differ for the other cells.
func TestGridCenter(t *testing.T) {
	t.Parallel()
	cells := []gridCell{{50, 40, 0, 0, 0, 0}, {80, 30, 0, 0, 0, 0}, {20, 10, 0, 0, 0, 0}, {60, 60, 0, 0, 0, 0}, {40, 20, 0, 0, 0, 0}, {10, 70, 0, 0, 0, 0}, {30, 30, 0, 0, 0, 0}, {70, 50, 0, 0, 0, 0}}
	for _, c := range []struct {
		lay, want string
	}{
		{`"align": "each", "center": true`, `scope 5,0; scope 70,5; scope 160,15; scope 0,55; scope 90,75; scope 165,50; scope 15,140; scope 75,130`},
		{`"align": "all", "center": true`, `scope 15,15; scope 90,20; scope 210,30; scope 10,85; scope 110,105; scope 215,80; scope 25,180; scope 95,170`},
		{`"align": "each", "center": {"column": true}`, `scope 5,0; scope 70,0; scope 160,0; scope 0,50; scope 90,50; scope 165,50; scope 15,130; scope 75,130`},
		{`"align": "none", "center": true`, `scope 0,0; scope 70,0; scope 160,0; scope 0,50; scope 70,50; scope 160,50; scope 0,130; scope 70,130`},
	} {
		checkGrid(t, `{"columns": 3, "padding": 10, `+c.lay+`}`, cells, nil, c.want)
	}
}

// gridHeaders are one of each kind of header, footer and title for each
// row and column of gridCells in two columns.
var gridHeaders = map[string]int{"row-header": 3, "column-header": 2, "row-footer": 3, "column-footer": 2, "row-title": 1, "column-title": 1}

// gridPlaced are the positions of the cells of gridCells in two columns
// with the align each.
const gridPlaced = `scope 0,0; scope 80,0; scope 0,56; scope 80,56; scope 0,132`

// TestGridHeaders checks the headers, the footers and the titles of a
// grid, with each property that moves them: offset, headerBand,
// footerBand, titleBand and titleAnchor. The positions are those that
// vega.js 6.4 gives for the same spec.
func TestGridHeaders(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		lay, want string
	}{
		{``, `row-header -7,0; row-header -7,56; row-header -7,132; column-header 0,-3; column-header 80,-3;
			row-footer 170,0; row-footer 170,56; row-footer 170,132; column-footer 0,162; column-footer 80,162;
			row-title -22,80; column-title 82,-15`},
		{`, "offset": {"rowHeader": 1, "columnHeader": 2, "rowFooter": 3, "columnFooter": 4, "rowTitle": 5, "columnTitle": 6}`,
			`row-header -8,0; row-header -8,56; row-header -8,132; column-header 0,-5; column-header 80,-5;
			row-footer 173,0; row-footer 173,56; row-footer 173,132; column-footer 0,166; column-footer 80,166;
			row-title -28,80; column-title 82,-23`},
		{`, "offset": 7`, `row-header -14,0; row-header -14,56; row-header -14,132; column-header 0,-10; column-header 80,-10;
			row-footer 177,0; row-footer 177,56; row-footer 177,132; column-footer 0,169; column-footer 80,169;
			row-title -36,80; column-title 82,-29`},
		{`, "headerBand": {"row": 0.5, "column": 1}, "footerBand": 0.25, "titleBand": {"row": 0, "column": 1}`,
			`row-header -7,19; row-header -7,88; row-header -7,146; column-header 50,-3; column-header 170,-3;
			row-footer 170,8; row-footer 170,57; row-footer 170,138; column-footer 6,162; column-footer 90,162;
			row-title -22,-3; column-title 170,-15`},
		{`, "titleAnchor": "end", "offset": {"rowTitle": 5, "columnTitle": 6}`,
			`row-header -7,0; row-header -7,56; row-header -7,132; column-header 0,-3; column-header 80,-3;
			row-footer 170,0; row-footer 170,56; row-footer 170,132; column-footer 0,162; column-footer 80,162;
			row-title 190,80; column-title 82,180`},
	} {
		lay := `{"columns": 2, "padding": {"row": 10, "column": 20}, "align": "each"` + c.lay + `}`
		checkGrid(t, lay, gridCells, gridHeaders, c.want+"; "+gridPlaced)
	}
}

// TestGridHeaderPerRow checks that each header serves its own row or
// column, so that a layout with fewer headers than rows leaves the other
// rows without one, as in Vega.
func TestGridHeaderPerRow(t *testing.T) {
	t.Parallel()
	checkGrid(t, `{"columns": 1, "padding": 10, "align": "each"}`,
		[]gridCell{{60, 30, 0, 0, 0, 0}, {50, 30, 0, 0, 0, 0}}, map[string]int{"row-header": 1, "column-footer": 1},
		`row-header 0,0; column-footer 0,70; scope 0,0; scope 0,40`)
}

// TestGridRowHeaderRoom checks that a row keeps room for its row header
// above and below its cells, so that the labels of an axis in the header
// do not meet those of the next row. Vega does not count the headers in
// the size of a row, so here vega.js puts the second cell at 20.
func TestGridRowHeaderRoom(t *testing.T) {
	t.Parallel()
	checkGrid(t, `{"columns": 1, "padding": 10, "align": "each"}`,
		[]gridCell{{60, 10, 0, 0, 0, 0}, {50, 10, 0, 0, 0, 0}}, map[string]int{"row-header": 1},
		`row-header 0,0; scope 0,0; scope 0,40`)
}

// TestGridSignals checks that a layout can give the layout and each of its
// properties by a signal.
func TestGridSignals(t *testing.T) {
	t.Parallel()
	want := strings.Join(gridScene(t, `{"columns": 2, "padding": {"row": 10, "column": 4}, "align": "all", "center": true}`, gridCells, nil), "; ")
	for _, lay := range []string{
		`{"columns": {"signal": "1 + 1"}, "padding": {"row": {"signal": "5 * 2"}, "column": 4}, "align": {"signal": "'all'"}, "center": {"signal": "true"}}`,
		`{"signal": "{columns: 2, padding: {row: 10, column: 4}, align: 'all', center: true}"}`,
	} {
		if got := strings.Join(gridScene(t, lay, gridCells, nil), "; "); got != want {
			t.Errorf("the layout %s: %s, want %s", lay, got, want)
		}
	}
}
