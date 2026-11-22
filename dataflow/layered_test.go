package dataflow

import (
	"slices"
	"testing"
	"unicode/utf8"
)

func TestWrapText(t *testing.T) {
	t.Parallel()
	// Each letter and each space is 1 wide.
	width := func(s string) float64 { return float64(utf8.RuneCountInString(s)) }
	tests := []struct {
		s          string
		limit      float64
		breakWords bool
		exp        []string
	}{
		{"short", 10, false, []string{"short"}},
		{"exactly 10", 10, false, []string{"exactly 10"}},
		{"one two three four", 9, false, []string{"one two", "three", "four"}},
		{"keep  two  spaces", 20, false, []string{"keep  two  spaces"}},
		{"a b\nc d e f g", 5, false, []string{"a b", "c d e", "f g"}},
		{"\nempty first", 20, false, []string{"", "empty first"}},
		{"a longwordhere b", 5, false, []string{"a", "longwordhere", "b"}},
		{"a longwordhere b", 5, true, []string{"a", "longw", "ordhe", "re b"}},
		{"xyz", 1, true, []string{"x", "y", "z"}},
		{"xyz", 0.5, true, []string{"x", "y", "z"}},
	}
	for _, test := range tests {
		if got := wrapText(test.s, test.limit, test.breakWords, width); !slices.Equal(got, test.exp) {
			t.Errorf("wrapText(%q, %v, %v): expected %q, got %q", test.s, test.limit, test.breakWords, test.exp, got)
		}
	}
}

// TestRunsHTML checks that the runs of a label in more than one font
// become a label of HTML with each run in its font, that the words wrap at
// the width limit, that a run with no space before it joins the word
// before it, and that the text is escaped.
func TestRunsHTML(t *testing.T) {
	t.Parallel()
	run := func(text string, bold, italic bool) any {
		return map[string]any{"text": text, "bold": bold, "italic": italic}
	}
	width := func(s, _ string) float64 { return float64(len(s)) }
	runs := []any{run("Bold", true, false), run(", and", false, false), run(" italic", false, true), run(" a<b", false, false)}
	if got, want := runsHTML(runs, 0, width), "<B>Bold</B>, and <I>italic</I> a&lt;b"; got != want {
		t.Errorf("runsHTML = %q, want %q", got, want)
	}
	if got, want := runsHTML(runs, 10, width), "<B>Bold</B>, and<BR/><I>italic</I> a&lt;b"; got != want {
		t.Errorf("runsHTML with a limit of 10 = %q, want %q", got, want)
	}
}
