package zuzu

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
)

// A dashed line keeps its gaps under the round cap of the look, and a
// legend of the dashes draws each dash.
func TestDashedLine(t *testing.T) {
	t.Parallel()
	s := renderVL(t, `"mark": "line",
		"encoding": {"x": {"field": "a", "type": "quantitative"}, "y": {"field": "b", "type": "quantitative"}, "strokeDash": {"field": "c"}}`)
	var dashed int
	for _, m := range marksOf(s, "mark") {
		if m.Type != scene.Line {
			continue
		}
		for _, it := range m.Items {
			if len(it.StrokeDash) == 2 && it.StrokeDash[1] > 0 {
				dashed++
				if it.StrokeCap != "butt" {
					t.Errorf("a line with the dashes %v has the cap %q, which closes its gaps", it.StrokeDash, it.StrokeCap)
				}
			}
		}
	}
	if dashed == 0 {
		t.Fatal("no line is dashed")
	}
	symbols := marksOf(s, "legend-symbol")
	if len(symbols) != 1 || len(symbols[0].Items) != 2 {
		t.Fatalf("the legend of the dashes has %d marks of symbols, want 1 of 2", len(symbols))
	}
	if d := symbols[0].Items[1].StrokeDash; len(d) != 2 || d[0] != 4 || d[1] != 2 {
		t.Errorf("the second entry of the legend has the dashes %v, want [4 2]", d)
	}
	// A gap longer than the width keeps the round cap.
	v := renderVega(t, `"width": 100, "height": 100, "marks": [{"type": "line",
		"from": {"data": "d"},
		"encode": {"update": {"x": {"field": "x"}, "y": {"value": 10}, "strokeDash": {"value": [5, 4]}, "strokeWidth": {"value": 1.5}, "strokeCap": {"value": "round"}}}}],
		"data": [{"name": "d", "values": [{"x": 0}, {"x": 90}]}]`)
	for _, m := range marksOf(v, "mark") {
		if m.Type == scene.Line && m.Items[0].StrokeCap != "round" {
			t.Errorf("a line that encodes its cap has the cap %q", m.Items[0].StrokeCap)
		}
	}
}

// A mark that clips draws inside its group, and stays out of the size of
// the chart, as in Vega.
func TestMarkClip(t *testing.T) {
	t.Parallel()
	s := renderVega(t, `"width": 100, "height": 80, "padding": 0, "marks": [
		{"type": "rect", "clip": true, "encode": {"update": {"x": {"value": 50}, "y": {"value": 10}, "width": {"value": 500}, "height": {"value": 20}}}}]`)
	if s.Width != 100 || s.Height != 80 {
		t.Errorf("the chart is %vx%v, want 100x80", s.Width, s.Height)
	}
	var b bytes.Buffer
	if err := svg.Write(&b, s); err != nil {
		t.Fatal(err)
	}
	if out := b.String(); !strings.Contains(out, `<clipPath id="clip1"><rect width="100" height="80"/></clipPath>`) || !strings.Contains(out, `clip-path="url(#clip1)"`) {
		t.Errorf("the SVG does not clip the mark:\n%s", out)
	}
}

// The radius of a text moves it at its angle theta, and the size of the
// chart holds it there.
func TestTextRadius(t *testing.T) {
	t.Parallel()
	s := renderVega(t, `"width": 100, "height": 100, "padding": 0, "marks": [
		{"type": "text", "encode": {"update": {"x": {"value": 50}, "y": {"value": 50}, "text": {"value": "far"},
			"radius": {"value": 200}, "theta": {"value": 1.5707963267948966}}}}]`)
	if s.Width < 250 {
		t.Errorf("the chart is %v wide, want room for the text 200 right of x 50", s.Width)
	}
}
