package dataflow

import "testing"

// TestHTMLCellLinks checks that each part of a label of HTML takes the link
// and the tooltip of the innermost table or cell with one that holds it,
// over the link of its node, and that a cell with a link and no fill gets
// a box that takes the link.
func TestHTMLCellLinks(t *testing.T) {
	t.Parallel()
	size := func(text, _ string, size float64, _ string) (float64, float64) {
		return 0.6 * size * float64(len(text)), size
	}
	src := `<table border="0" href="https://t.test" tooltip="table"><tr><td>a</td><td href="https://c.test" title="cell">b</td><td href="javascript:x">c</td></tr></table>`
	h, err := layHTML(src, size, 11)
	if err != nil {
		t.Fatal(err)
	}
	rows := h.rows(Row{"key": "n", "href": "https://node.test"}, 100, 100, "none")
	runs := map[string]Row{}
	boxes := 0
	for _, r := range rows {
		switch r["kind"] {
		case "htmlRun":
			text, _ := r["text"].(string)
			runs[text] = r
		case "htmlBox":
			boxes++
		}
	}
	if r := runs["a"]; r["href"] != "https://t.test" || r["tooltip"] != "table" {
		t.Errorf("expected a to take the link of the table, got %v and %v", r["href"], r["tooltip"])
	}
	if r := runs["b"]; r["href"] != "https://c.test" || r["tooltip"] != "cell" {
		t.Errorf("expected b to take the link of its cell, got %v and %v", r["href"], r["tooltip"])
	}
	// A link with a scheme that zuzu does not write draws no link, so c
	// keeps the link of the table.
	if r := runs["c"]; r["href"] != "https://t.test" {
		t.Errorf("expected c to keep the link of the table, got %v", r["href"])
	}
	// The table and the cell of b have no fill and no border, and each
	// gets a box for its link.
	if boxes < 2 {
		t.Errorf("expected a box for the table and for the cell of b, got %d boxes", boxes)
	}
}
