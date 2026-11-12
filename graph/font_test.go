package graph

import "testing"

func TestFontFace(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, weight, style string
	}{
		{"Helvetica", "", ""},
		{"Helvetica-Bold", BoldWeight, ""},
		{"Times-BoldItalic", BoldWeight, "italic"},
		{"Helvetica-BoldOblique", BoldWeight, "italic"},
		{"Times-Roman", "", ""},
		{"Times-Italic", "", "italic"},
		{"Courier-Oblique", "", "italic"},
		{"Arial Light", "300", ""},
		{"Source Sans Pro ExtraLight", "200", ""},
		{"Segoe UI Semibold", BoldWeight, ""},
		{"Open Sans SemiBold Italic", BoldWeight, "italic"},
		{"Roboto Medium", "500", ""},
		{"Arial Black", BoldWeight, ""},
		{"Helvetica:bold", BoldWeight, ""},
		{"Arial:style=Bold Italic", BoldWeight, "italic"},
		{"Lucida Bright", "", ""},
		{"Trebuchet MS", "", ""},
		{"'Times New Roman', serif", "", ""},
		{"", "", ""},
	} {
		w, s := FontFace(c.name)
		if w != c.weight || s != c.style {
			t.Errorf("FontFace(%q) = %q, %q, want %q, %q", c.name, w, s, c.weight, c.style)
		}
	}
}
