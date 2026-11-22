package zuzu

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// layoutFlow is a flowchart with nested clusters, an edge across the border
// of a cluster, a cluster with no such edge, and labels.
const layoutFlow = `flowchart LR
  x[Start] -->|go| a
  subgraph outer [Outer]
    subgraph inner [Inner]
      a --> b
    end
    b --> c
  end
  subgraph alone [Alone]
    d --> e
  end
  c -->|done| y[End]
  y --> y
`

// drawLayout draws a source and returns its SVG and its report.
func drawLayout(t *testing.T, src string, parse []Option, render ...RenderOption) (string, Report) {
	t.Helper()
	c, report, err := Parse([]byte(src), parse...)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	more, err := c.SVG(t.Context(), &b, render...)
	if err != nil {
		t.Fatal(err)
	}
	return b.String(), append(report, more...)
}

func TestWithLayout(t *testing.T) {
	t.Parallel()
	plain, report := drawLayout(t, layoutFlow, nil)
	if len(report) > 0 {
		t.Errorf("expected no issue, got %v", report)
	}
	dagre, report := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")})
	if len(report) > 0 {
		t.Errorf("expected no issue from zuzu.dagre, got %v", report)
	}
	if dagre == plain {
		t.Fatal("expected zuzu.dagre to draw other than zuzu.layered")
	}
	if got, _ := drawLayout(t, layoutFlow, nil, WithLayout("dagre")); got != dagre {
		t.Error("expected WithLayout of the render to draw as WithLayout of the parse")
	}
	if got, _ := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")}, WithLayout("layered")); got != plain {
		t.Error("expected WithLayout of the render to win over WithLayout of the parse")
	}
	if got, _ := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")}, WithLayout("")); got != plain {
		t.Error("expected an empty layout of the render to force no layout")
	}
	if got, _ := drawLayout(t, layoutFlow, []Option{WithLayout("layered")}); got != plain {
		t.Error("expected the layout layered to draw as no layout")
	}
	// A layout of the source gives way to the forced layout.
	if got, _ := drawLayout(t, "---\nconfig:\n  layout: dagre\n---\n"+layoutFlow, []Option{WithLayout("layered")}); got != plain {
		t.Error("expected WithLayout to win over layout: dagre")
	}
}

func TestWithLayoutNoEngine(t *testing.T) {
	t.Parallel()
	plain, _ := drawLayout(t, layoutFlow, nil)
	for _, name := range []string{"elk", "elk.stress", "circle"} {
		got, report := drawLayout(t, layoutFlow, nil, WithLayout(name))
		if got != plain || len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupported) || !strings.Contains(report[0].Error(), fmt.Sprintf("the layout %q of zuzu.WithLayout", name)) {
			t.Errorf("%s: expected the drawing of zuzu.layered and one issue, got %v", name, report)
		}
		c, _, err := Parse([]byte(layoutFlow), WithLayout(name), WithStrict(true))
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if _, err := c.SVG(t.Context(), &b); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%s: expected a forced layout with no engine to be an error in the strict mode, got %v", name, err)
		}
	}
	// A chart with no graph lays out nothing, so it reports nothing.
	if _, report := drawLayout(t, "pie\n  \"a\": 1\n  \"b\": 2\n", nil, WithLayout("elk")); len(report) > 0 {
		t.Errorf("expected no issue for a chart with no graph, got %v", report)
	}
}

func TestWithLayoutEngine(t *testing.T) {
	t.Parallel()
	plain, _ := drawLayout(t, layoutFlow, nil)
	dagre, _ := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")})
	tests := []struct {
		name, src string
		// algorithm is the algorithm that the engine of ELK gets, or empty
		// when the engine of ELK does not lay out the graph.
		algorithm string
		// dagre says that the graph draws with zuzu.dagre.
		dagre bool
	}{
		{"no layout", layoutFlow, "elk", false},
		{"elk", "---\nconfig:\n  layout: elk\n---\n" + layoutFlow, "elk", false},
		{"elk.stress", "---\nconfig:\n  layout: elk.stress\n---\n" + layoutFlow, "elk.stress", false},
		{"section", "---\nconfig:\n  flowchart:\n    layout: elk.mrtree\n---\n" + layoutFlow, "elk.mrtree", false},
		{"dagre", "---\nconfig:\n  layout: dagre\n---\n" + layoutFlow, "", true},
		{"lanes", "---\nconfig:\n  layout: swimlane\n---\n" + layoutFlow, "", false},
		{"dot", "digraph { a -> b; b -> c }", "", false},
	}
	for _, tt := range tests {
		var algorithm string
		called := false
		e := algorithmEngine{got: &algorithm, called: &called}
		got, report := drawLayout(t, tt.src, []Option{WithLayoutEngine(e)})
		if len(report) > 0 {
			t.Errorf("%s: expected no issue, got %v", tt.name, report)
		}
		if called != (tt.algorithm != "") || algorithm != tt.algorithm {
			t.Errorf("%s: expected the engine of ELK to get the algorithm %q, got %q", tt.name, tt.algorithm, algorithm)
		}
		if tt.dagre && got != dagre {
			t.Errorf("%s: expected the drawing of zuzu.dagre", tt.name)
		}
		// Without the engine of ELK, the graph draws as it does with no
		// option, and the engine of the render draws as that of Parse.
		without, _ := drawLayout(t, tt.src, nil)
		if tt.algorithm != "" && without != plain {
			t.Errorf("%s: expected the drawing of zuzu.layered without the engine of ELK", tt.name)
		}
		if again, _ := drawLayout(t, tt.src, nil, WithLayoutEngine(e)); again != got {
			t.Errorf("%s: expected WithLayoutEngine of the render to draw as that of Parse", tt.name)
		}
	}
	// WithLayout forces the engine that WithLayoutEngine makes available,
	// on DOT too.
	var algorithm string
	called := false
	drawLayout(t, "digraph { a -> b }", []Option{WithLayoutEngine(algorithmEngine{got: &algorithm, called: &called}), WithLayout("elk.force")})
	if !called || algorithm != "elk.force" {
		t.Errorf("expected WithLayout to force the engine of ELK with elk.force, got %v %q", called, algorithm)
	}
	// The engine of the render wins over the engine of Parse with the same
	// name, and WithLayoutEngine(nil) makes nothing available.
	var first, second string
	var c1, c2 bool
	drawLayout(t, layoutFlow, []Option{WithLayoutEngine(algorithmEngine{got: &first, called: &c1})},
		WithLayoutEngine(nil), WithLayoutEngine(algorithmEngine{got: &second, called: &c2}))
	if c1 || !c2 {
		t.Errorf("expected the engine of the render to lay out, got the parse %v and the render %v", c1, c2)
	}
}

func TestLayoutConfig(t *testing.T) {
	t.Parallel()
	c, _, err := Parse([]byte(layoutFlow))
	if err != nil {
		t.Fatal(err)
	}
	dagre, _ := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")})
	plain, _ := drawLayout(t, layoutFlow, nil)
	spec := c.Spec()
	if spec.Config == nil {
		spec.Config = vega.Config{}
	}
	spec.Config[graph.ConfigLayout] = jsontext.Value(`"dagre"`)
	byConfig, _, err := FromVega(spec)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if _, err := byConfig.SVG(t.Context(), &b); err != nil {
		t.Fatal(err)
	}
	if b.String() != dagre {
		t.Error("expected zuzu.layout dagre to draw as WithLayout dagre")
	}
	b.Reset()
	if _, err := byConfig.SVG(t.Context(), &b, WithLayout("layered")); err != nil {
		t.Fatal(err)
	}
	if b.String() != plain {
		t.Error("expected WithLayout to win over zuzu.layout")
	}
	// A layout with no engine is an issue, and draws with the transform of
	// the spec.
	spec.Config[graph.ConfigLayout] = jsontext.Value(`"circle"`)
	unknown, _, err := FromVega(spec)
	if err != nil {
		t.Fatal(err)
	}
	b.Reset()
	report, err := unknown.SVG(t.Context(), &b)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != plain || len(report) != 1 || !strings.Contains(report[0].Error(), `the layout "circle"`) {
		t.Errorf("expected the drawing of zuzu.layered and one issue, got %v", report)
	}
	strict, _, err := FromVega(spec, WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strict.SVG(t.Context(), &b); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("expected an unknown layout to be an error in the strict mode, got %v", err)
	}
}

// algorithmEngine is an engine named elk that keeps the algorithm that it
// gets, says that it was called, and lays out with zuzu.layered.
type algorithmEngine struct {
	got    *string
	called *bool
}

func (a algorithmEngine) Name() string { return graph.EngineELK }

func (a algorithmEngine) Lay(in graph.LayoutInput) (*graph.Layout, error) {
	*a.got, *a.called = in.Algorithm, true
	return graph.Lay(in)
}

func TestLayoutELKHint(t *testing.T) {
	t.Parallel()
	plain, _ := drawLayout(t, layoutFlow, nil)
	for _, name := range []string{"elk", "elk.stress"} {
		src := "---\nconfig:\n  layout: " + name + "\n---\n" + layoutFlow
		// Without an engine of ELK, the layout is a hint that zuzu.layered
		// draws, with no issue (D32).
		got, report := drawLayout(t, src, nil)
		if got != plain || len(report) > 0 {
			t.Errorf("%s: expected the drawing of zuzu.layered with no issue, got %v", name, report)
		}
	}
	// The layout dagre draws with zuzu.dagre (D45 part 2).
	dagre, _ := drawLayout(t, layoutFlow, []Option{WithLayout("dagre")})
	if got, report := drawLayout(t, "---\nconfig:\n  layout: dagre\n---\n"+layoutFlow, nil); got != dagre || len(report) > 0 {
		t.Errorf("expected layout: dagre to draw with zuzu.dagre with no issue, got %v", report)
	}
}

func TestLayoutUnsupportedPart(t *testing.T) {
	t.Parallel()
	src := "digraph { a -> b:w; b -> c }"
	_, report := drawLayout(t, src, []Option{WithLayout("dagre")})
	if len(report) != 1 || !errors.Is(report[0], vega.ErrUnsupported) || !strings.Contains(report[0].Error(), "the port of the edge a -> b, which zuzu.dagre leaves out") {
		t.Errorf("expected the port in the report, got %v", report)
	}
	c, _, err := Parse([]byte(src), WithLayout("dagre"), WithStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if _, err := c.SVG(t.Context(), &b); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("expected the port to be an error in the strict mode, got %v", err)
	}
	if _, report := drawLayout(t, src, nil); len(report) > 0 {
		t.Errorf("expected zuzu.layered to lay out the port, got %v", report)
	}
}
