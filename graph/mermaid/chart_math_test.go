package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text/math"
	"github.com/xo/zuzu/vega"
)

// mathSpec compiles a chart of Mermaid, with the font of math when font is
// true, and returns its spec as JSON values and its report. An error fails
// the test.
func mathSpec(t *testing.T, src string, font bool) (map[string]any, []string) {
	t.Helper()
	b, report, err := mathCompile(t, src, font, false)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, is := range report {
		out = append(out, is.Error())
	}
	return spec, out
}

// mathCompile compiles a chart of Mermaid, with the font of math when font
// is true, in the strict mode when strict is true.
func mathCompile(t *testing.T, src string, font, strict bool) ([]byte, vega.Report, error) {
	t.Helper()
	d, err := mermaid.Read(src)
	if err != nil {
		t.Fatalf("%q: read: %v", src, err)
	}
	if font {
		d.Math = math.Font()
	}
	b, _, report, err := mermaid.Chart(d, strict)
	return b, report, err
}

// specData returns the rows of a dataset of a spec, through the source of
// a dataset that filters another.
func specData(spec map[string]any, name string) []map[string]any {
	data, _ := spec["data"].([]any)
	for _, d := range data {
		m, _ := d.(map[string]any)
		if m["name"] != name {
			continue
		}
		if src, ok := m["source"].(string); ok {
			return specData(spec, src)
		}
		var out []map[string]any
		vals, _ := m["values"].([]any)
		for _, v := range vals {
			r, _ := v.(map[string]any)
			out = append(out, r)
		}
		return out
	}
	return nil
}

// specMarks returns the marks of a spec of a type, and of a role when
// role is not empty.
func specMarks(spec map[string]any, typ, role string) []map[string]any {
	marks, _ := spec["marks"].([]any)
	var out []map[string]any
	for _, mk := range marks {
		m, _ := mk.(map[string]any)
		if m["type"] == typ && (role == "" || m["role"] == role) {
			out = append(out, m)
		}
	}
	return out
}

// markRows returns the rows that a mark draws.
func markRows(spec map[string]any, m map[string]any) []map[string]any {
	from, _ := m["from"].(map[string]any)
	name, _ := from["data"].(string)
	return specData(spec, name)
}

// mathTeX returns the TeX of each row of the marks of math of a spec, and
// the texts of its marks of text, each once.
func mathTeX(spec map[string]any) ([]string, []string) {
	var tex, texts []string
	for _, m := range specMarks(spec, "path", scene.RoleMath) {
		for _, r := range markRows(spec, m) {
			if s, _ := r["tex"].(string); !slices.Contains(tex, s) {
				tex = append(tex, s)
			}
		}
	}
	for _, m := range specMarks(spec, "text", "") {
		for _, r := range markRows(spec, m) {
			for _, k := range []string{"text", "lines"} {
				switch v := r[k].(type) {
				case string:
					texts = append(texts, v)
				case []any:
					for _, s := range v {
						str, _ := s.(string)
						texts = append(texts, str)
					}
				}
			}
		}
	}
	return tex, texts
}

// mathSources are a diagram of each kind that lays out when it compiles,
// with math of KaTeX in each label that draws it, and the TeX of each
// formula.
var mathSources = []struct {
	name, src string
	tex       []string
}{
	{
		"sequence",
		"sequenceDiagram\n  participant A as $$\\alpha$$\n  participant B\n  A->>B: send $$\\frac{a}{b}$$ now\n" +
			"  Note over A: $$x^2$$\n  loop $$\\sum_i x_i$$\n    B->>A: back\n  end\n  alt $$y$$\n    A->>B: one\n  else $$z$$\n    A->>B: two\n  end\n",
		[]string{`\alpha`, `\frac{a}{b}`, `x^2`, `\sum_i x_i`, `y`, `z`},
	},
	{"mindmap", "mindmap\n  root((Root $$r$$))\n    a[\"$$\\frac{a}{b}$$\"]\n    b[plain]\n", []string{`r`, `\frac{a}{b}`}},
	{
		"block",
		"block-beta\n  columns 2\n  block:g[\"group $$g$$\"]:2\n    a[\"$$\\frac{a}{b}$$\"] b\n  end\n  c d\n  c -- \"$$e$$\" --> d\n",
		[]string{`g`, `\frac{a}{b}`, `e`},
	},
}

// TestChartMath checks that the sequence diagram, the mindmap and the
// block diagram draw the math of KaTeX with the font of math as paths of
// the role math with their TeX, with no issue and no $$ in a text, and
// report each label with math as its source with no font.
func TestChartMath(t *testing.T) {
	t.Parallel()
	for _, test := range mathSources {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec, report := mathSpec(t, test.src, true)
			if len(report) > 0 {
				t.Errorf("with the font, the report %q", report)
			}
			if _, _, err := mathCompile(t, test.src, true, true); err != nil {
				t.Errorf("with the font, in the strict mode: %v", err)
			}
			tex, texts := mathTeX(spec)
			for _, w := range test.tex {
				if !slices.Contains(tex, w) {
					t.Errorf("no path of math with the TeX %q in %q", w, tex)
				}
			}
			for _, s := range texts {
				if strings.Contains(s, "$$") {
					t.Errorf("the text %q holds $$", s)
				}
			}
			for _, m := range specMarks(spec, "path", scene.RoleMath) {
				enc, _ := m["encode"].(map[string]any)["update"].(map[string]any)
				if _, ok := enc["text"]; !ok {
					t.Errorf("the mark of math %v does not encode its text", m["name"])
				}
			}
			spec, report = mathSpec(t, test.src, false)
			if len(report) == 0 {
				t.Error("with no font, no issue")
			}
			for _, is := range report {
				if !strings.Contains(is, "drawn as its source") {
					t.Errorf("with no font, the issue %q", is)
				}
			}
			if tex, _ := mathTeX(spec); len(tex) > 0 {
				t.Errorf("with no font, math %q", tex)
			}
			if _, _, err := mathCompile(t, test.src, false, true); !errors.Is(err, vega.ErrUnsupported) {
				t.Errorf("with no font, in the strict mode, the error %v", err)
			}
		})
	}
}

// TestChartMathCommand checks that a formula with a command that zuzu
// cannot lay out is an issue that names the command, and an error in the
// strict mode, and that its label draws its source.
func TestChartMathCommand(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"sequenceDiagram\n  A->>B: bad $$\\foo{x}$$\n",
		"mindmap\n  root[\"bad $$\\foo{x}$$\"]\n",
		"block-beta\n  a[\"bad $$\\foo{x}$$\"]\n",
	} {
		spec, report := mathSpec(t, src, true)
		if len(report) != 1 || !strings.Contains(report[0], `\foo`) || !strings.Contains(report[0], "drawn as its source") {
			t.Errorf("%q: the report %q, want one issue that names \\foo", src, report)
		}
		tex, texts := mathTeX(spec)
		if len(tex) > 0 {
			t.Errorf("%q: math %q", src, tex)
		}
		if !slices.ContainsFunc(texts, func(s string) bool { return strings.Contains(s, `$$\foo{x}$$`) }) {
			t.Errorf("%q: no text of the source in %q", src, texts)
		}
		if _, _, err := mathCompile(t, src, true, true); !errors.Is(err, vega.ErrUnsupported) || !strings.Contains(err.Error(), `\foo`) {
			t.Errorf("%q: in the strict mode, the error %v", src, err)
		}
	}
}

// TestChartMathHTMLLabels checks that a mindmap and a block diagram with
// htmlLabels false draw the source of the math with no issue, as Mermaid
// does, and that a sequence diagram draws the math whatever htmlLabels is.
func TestChartMathHTMLLabels(t *testing.T) {
	t.Parallel()
	front := "---\nconfig:\n  htmlLabels: false\n---\n"
	for _, src := range []string{
		front + "mindmap\n  root[\"$$\\frac{a}{b}$$\"]\n",
		front + "block-beta\n  a[\"$$\\frac{a}{b}$$\"] b\n  a -- \"$$e$$\" --> b\n",
		"---\nconfig:\n  flowchart:\n    htmlLabels: false\n---\nblock-beta\n  a[\"$$\\frac{a}{b}$$\"]\n",
	} {
		for _, font := range []bool{false, true} {
			spec, report := mathSpec(t, src, font)
			if len(report) > 0 {
				t.Errorf("%q: the report %q", src, report)
			}
			tex, texts := mathTeX(spec)
			if len(tex) > 0 {
				t.Errorf("%q: math %q", src, tex)
			}
			if !slices.ContainsFunc(texts, func(s string) bool { return strings.Contains(s, `$$\frac{a}{b}$$`) }) {
				t.Errorf("%q: no text of the source in %q", src, texts)
			}
		}
	}
	src := front + "sequenceDiagram\n  A->>B: $$\\frac{a}{b}$$\n"
	spec, report := mathSpec(t, src, true)
	if len(report) > 0 {
		t.Errorf("the report %q", report)
	}
	if tex, _ := mathTeX(spec); !slices.Equal(tex, []string{`\frac{a}{b}`}) {
		t.Errorf("a sequence diagram with htmlLabels false draws the math %q", tex)
	}
}

// TestChartMathBox checks that the title of a box of a sequence diagram
// draws its source, as in Mermaid, with no issue, and that a participant
// named by an id with math draws it.
func TestChartMathBox(t *testing.T) {
	t.Parallel()
	src := "sequenceDiagram\n  box Aqua $$b$$\n    participant $$p$$\n  end\n  $$p$$->>C: hi\n"
	for _, font := range []bool{false, true} {
		spec, report := mathSpec(t, src, font)
		_, texts := mathTeX(spec)
		if !slices.Contains(texts, "$$b$$") {
			t.Errorf("font %v: no title of the box as its source in %q", font, texts)
		}
		want := 0
		if !font {
			want = 1
		}
		if len(report) != want {
			t.Errorf("font %v: the report %q, want %d issues", font, report, want)
		}
	}
	spec, _ := mathSpec(t, src, true)
	if tex, _ := mathTeX(spec); !slices.Equal(tex, []string{"p"}) {
		t.Errorf("the math %q, want the id of the participant", tex)
	}
}

// TestChartMathGrows checks that a fraction makes the label that holds it
// higher than a label of text, so that the participant, the note, the
// message, the node and the block that hold it grow.
func TestChartMathGrows(t *testing.T) {
	t.Parallel()
	frac := `$$\frac{a}{b}$$`
	tests := []struct {
		name, src, set string
		size           func(r map[string]any) float64
	}{
		{"participant", "sequenceDiagram\n  participant A as %s\n  A->>A: hi\n", "lifelines", func(r map[string]any) float64 {
			return num(r["y"])
		}},
		{"note", "sequenceDiagram\n  participant A\n  Note over A: %s\n", "notes", func(r map[string]any) float64 {
			return num(r["y2"]) - num(r["y"])
		}},
		{"message", "sequenceDiagram\n  A->>B: hi\n  A->>B: %s\n", "messages", func(r map[string]any) float64 {
			return num(r["y"])
		}},
		{"mindmap", "mindmap\n  root[\"%s\"]\n", "nodes", func(r map[string]any) float64 { return num(r["height"]) }},
		{"block", "block-beta\n  a[\"%s\"]\n", "blocks", func(r map[string]any) float64 { return num(r["height"]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			last := func(label string) float64 {
				spec, report := mathSpec(t, strings.Replace(test.src, "%s", label, 1), true)
				if len(report) > 0 {
					t.Errorf("the report %q", report)
				}
				rows := specData(spec, test.set)
				if len(rows) == 0 {
					t.Fatalf("no rows of %s", test.set)
				}
				return test.size(rows[len(rows)-1])
			}
			text, math := last("ab"), last(frac)
			t.Logf("with text %v, with a fraction %v", text, math)
			if math <= text {
				t.Errorf("with a fraction %v, with text %v, want more", math, text)
			}
		})
	}
}
