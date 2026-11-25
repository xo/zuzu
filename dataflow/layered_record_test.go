package dataflow

import (
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// fixedMeasure measures a text 10 pixels for each letter of its longest
// line, and 10 pixels for each line.
func fixedMeasure(v any, _ string) (float64, float64) {
	s, _ := v.(string)
	if s == "" {
		return 0, 0
	}
	w := 0
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		w = max(w, len(l))
	}
	return 10 * float64(w), 10 * float64(len(lines))
}

// TestRecordGroups checks that a group in braces lies the other way from
// the fields around it, at every depth, that its fields share the room of
// the group, and that the ports of its fields have their boxes.
func TestRecordGroups(t *testing.T) {
	t.Parallel()
	// a|{b|<p>c}|d with the fields side by side, as in TB.
	row := Row{
		"fields": []any{"a", "", "d"},
		"ports":  []any{"", "", ""},
		"groups": []any{nil, map[string]any{"fields": []any{"b", "cc"}, "ports": []any{"", "p"}}, nil},
	}
	rec := readRecord(row, true)
	if rec == nil || rec.groups[1] == nil || rec.groups[1].across {
		t.Fatalf("expected a group of fields one under another, got %+v", rec)
	}
	w, h := rec.size(fixedMeasure, 8)
	// a and d are 26 wide, and the group is as wide as cc, 36. Each field
	// is 24 high, the least height of a field.
	if w != 26+36+26 || h != 48 {
		t.Errorf("expected the record 88 by 48, got %g by %g", w, h)
	}
	boxes := rec.boxes(w, h)
	if b := boxes["p"]; b != [4]float64{26 - w/2, 24 - h/2, 36, 24} {
		t.Errorf("expected the port p at the bottom of the group, got %v", b)
	}
	path, rows := rec.rows(Row{"key": "n"}, graph.NodePlace{X: 100, Y: 50, Width: w, Height: h}, "record", 8, 11)
	// The line between b and cc runs across the group, in the middle.
	if want := "M-18,0L18,0"; !strings.Contains(path, want) {
		t.Errorf("expected the line %s between b and cc, got %s", want, path)
	}
	if len(rows) != 4 {
		t.Errorf("expected a row for each of a, b, cc and d, got %d", len(rows))
	}
	// The same record with the fields one under another, as in LR, lays
	// the group side by side.
	rec = readRecord(row, false)
	w, h = rec.size(fixedMeasure, 8)
	if !rec.groups[1].across || w != 26+36 || h != 72 {
		t.Errorf("expected the group side by side, 62 by 72, got %t, %g by %g", rec.groups[1].across, w, h)
	}
}

// TestRecordAlign checks the place of each line of a field with an
// alignment for each line, as \l, \n and \r of DOT give it.
func TestRecordAlign(t *testing.T) {
	t.Parallel()
	row := Row{"fields": []any{"left\nmid\nright"}, "ports": []any{""}, "aligns": []any{"lcr"}}
	rec := readRecord(row, true)
	w, h := rec.size(fixedMeasure, 8)
	_, rows := rec.rows(Row{"key": "n"}, graph.NodePlace{Width: w, Height: h}, "record", 8, 11)
	var got []string
	for _, r := range rows {
		align, _ := r["align"].(string)
		got = append(got, align)
	}
	if strings.Join(got, " ") != "left center right" {
		t.Errorf("expected the lines left, center and right, got %v", got)
	}
	if x, _ := rows[2]["x"].(float64); x != w/2-8 {
		t.Errorf("expected the right line to end at the padding, got %g", x)
	}
}

// TestRecordRuns checks that a cell and a line with runs take the size of
// their runs, and that each run draws in its own font.
func TestRecordRuns(t *testing.T) {
	t.Parallel()
	// A bold letter is 12 pixels wide, and a plain one 10.
	size := func(text, _ string, _ float64, weight string) (float64, float64) {
		w := 10.0
		if weight == "bold" {
			w = 12
		}
		return w * float64(len(text)), 10
	}
	width := func(text, weight string) float64 {
		w, _ := size(text, "", 11, weight)
		return w
	}
	// runs returns runs that start bold and change font at each run.
	runs := func(texts ...string) []any {
		var out []any
		for i, s := range texts {
			out = append(out, map[string]any{"text": s, "bold": i%2 == 0, "italic": false})
		}
		return out
	}
	row := Row{
		"fields":    []any{"abc", ""},
		"ports":     []any{"", ""},
		"stack":     true,
		"cells":     []any{nil, []any{"x", "abcd"}},
		"fieldRuns": []any{[]any{runs("a", "bc")}, []any{nil, runs("ab", "cd")}},
	}
	rec := readRecord(row, false)
	rec.html = func(rs []any) *htmlLabel {
		h, err := layHTML(runsHTML(rs, 0, width), size, 11)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	w, h := rec.size(fixedMeasure, 8)
	// The cell abcd is 2 bold letters and 2 plain ones, 44 wide, where its
	// plain text measures 40.
	if len(rec.cols) != 2 || rec.cols[1] != 44+16 {
		t.Errorf("expected the second column 60 wide, got %v", rec.cols)
	}
	_, rows := rec.rows(Row{"key": "n"}, graph.NodePlace{Width: w, Height: h}, "record", 8, 11)
	var got []string
	for _, r := range rows {
		if r["kind"] != "htmlRun" {
			continue
		}
		s, _ := r["text"].(string)
		if r["fontWeight"] == "bold" {
			s = "*" + s
		}
		got = append(got, s)
	}
	if want := "*a bc *ab cd"; strings.Join(got, " ") != want {
		t.Errorf("expected the runs %s, got %v", want, got)
	}
}
