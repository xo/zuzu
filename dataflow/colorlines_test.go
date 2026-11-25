package dataflow

import "testing"

// TestColorLines checks the rows of an edge of more than one color: the
// edge keeps the line of the color of its arrowheads, and each other color
// is a row of the kind edgeLine with the paint of the lines.
func TestColorLines(t *testing.T) {
	t.Parallel()
	o := Row{
		"key": "e", "path": "M0,0L0,100", "strokes": []any{"red", "green", "blue"}, "stroke": "red",
		"head": "none", "tail": "none", "_linePaint": "stroke", "href": "https://example.com",
	}
	rows := colorLines(o)
	if len(rows) != 2 || o["path"] != "M-2,0L-2,100" {
		t.Fatalf("expected the edge on the first line and two more rows, got %v and %v", o["path"], rows)
	}
	if r := rows[1]; r["kind"] != "edgeLine" || r["stroke"] != "blue" || r["path"] != "M2,0L2,100" || r["_paint"] != "stroke" || r["href"] != "https://example.com" {
		t.Errorf("unexpected row of blue %v", r)
	}
	// With weights, the edge keeps the last color, at the head.
	o = Row{"key": "e", "path": "M0,0L0,100", "strokes": []any{"red", "blue"}, "weights": []any{0.3, -1.0}, "stroke": "blue"}
	rows = colorLines(o)
	if len(rows) != 1 || o["path"] != "M0,30L0,100" || rows[0]["path"] != "M0,0L0,30" || rows[0]["stroke"] != "red" {
		t.Errorf("expected red then blue, got %v and %v", o["path"], rows)
	}
	if colorLines(Row{"path": "M0,0L1,1"}) != nil {
		t.Error("expected no rows for an edge of one color")
	}
}
