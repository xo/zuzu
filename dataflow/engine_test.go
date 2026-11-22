package dataflow

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// layoutEnv is an env that picks a layout engine and keeps the issues.
type layoutEnv struct {
	*env

	engine    graph.Engine
	algorithm string
	issues    []string
}

func (e *layoutEnv) Layout() (graph.Engine, string, error) { return e.engine, e.algorithm, nil }

func (e *layoutEnv) Issue(_ error, path, format string, args ...any) error {
	e.issues = append(e.issues, path+": "+fmt.Sprintf(format, args...))
	return nil
}

// recordEngine is an engine that keeps the input of its last call and lays
// out with zuzu.layered.
type recordEngine struct {
	in *graph.LayoutInput
}

func (r *recordEngine) Name() string { return "record" }

func (r *recordEngine) Lay(in graph.LayoutInput) (*graph.Layout, error) {
	*r.in = in
	return graph.Lay(in)
}

// graphEnv returns an env with the edges of a small graph, one of them with
// a port.
func graphEnv() *env {
	e := newEnv()
	e.data["edges"] = []Row{{"source": "a", "target": "b", "sourcePort": "e"}, {"source": "b", "target": "c"}}
	return e
}

const graphNodes = `[{"key":"a","width":40,"height":20},{"key":"b","width":40,"height":20},{"key":"c","width":40,"height":20}]`

func TestLayoutTransforms(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"zuzu.layered", "zuzu.dagre"} {
		if !Supported(typ) {
			t.Errorf("expected the transform %s", typ)
		}
	}
	e := &layoutEnv{env: graphEnv()}
	out, err := Run(e, rowsOf(t, graphNodes), transformsOf(t, `[{"type":"zuzu.dagre","edges":"edges"}]`), "/transform")
	if err != nil {
		t.Fatal(err)
	}
	if path, _ := out[3]["path"].(string); len(out) != 5 || out[3]["kind"] != "edge" || !strings.HasPrefix(path, "M") {
		t.Errorf("expected the rows of a layout, got %v", out)
	}
	e.issues = nil
	// Without an env that takes issues, a part that the engine leaves out
	// is an error.
	_, err = Run(e.env, rowsOf(t, graphNodes), transformsOf(t, `[{"type":"zuzu.dagre","edges":"edges"}]`), "/transform")
	if !errors.Is(err, vega.ErrUnsupported) || !strings.Contains(err.Error(), "the port of the edge a -> b") {
		t.Errorf("expected the port as an error, got %v", err)
	}
	if _, err := Run(e, rowsOf(t, graphNodes), transformsOf(t, `[{"type":"zuzu.dagre","edges":"edges"}]`), "/transform"); err != nil {
		t.Fatal(err)
	}
	want := []string{"transform zuzu.dagre: the port of the edge a -> b, which zuzu.dagre leaves out"}
	if strings.Join(e.issues, "\n") != strings.Join(want, "\n") {
		t.Errorf("expected the issues %q, got %q", want, e.issues)
	}
	// zuzu.layered lays out the port, and reports nothing.
	e.issues = nil
	if _, err := Run(e, rowsOf(t, graphNodes), transformsOf(t, `[{"type":"zuzu.layered","edges":"edges"}]`), "/transform"); err != nil {
		t.Fatal(err)
	}
	if len(e.issues) > 0 {
		t.Errorf("expected no issue, got %q", e.issues)
	}
}

func TestLayoutEnvPicksTheEngine(t *testing.T) {
	t.Parallel()
	var in graph.LayoutInput
	e := &layoutEnv{env: graphEnv(), engine: &recordEngine{in: &in}, algorithm: "elk.stress"}
	if _, err := Run(e, rowsOf(t, graphNodes), transformsOf(t, `[{"type":"zuzu.dagre","edges":"edges","direction":"LR"}]`), "/transform"); err != nil {
		t.Fatal(err)
	}
	if len(in.Nodes) != 3 || len(in.Edges) != 2 || in.Direction != graph.LR {
		t.Errorf("expected the engine of the env to lay out the graph, got %+v", in)
	}
	if in.Algorithm != "elk.stress" {
		t.Errorf("expected the algorithm elk.stress, got %q", in.Algorithm)
	}
	if len(e.issues) > 0 {
		t.Errorf("expected no issue from the engine of the env, got %q", e.issues)
	}
}
