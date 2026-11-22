package dagro

import (
	"math"
	"testing"
)

// The tests of upstream share these helpers with layout_differential_test.go,
// which the copy leaves out, because it runs Dagre in Node.js and reads the
// corpus of D2. They are the helpers of that file, as upstream writes them.

func newLayoutTestGraph() *Graph {
	return NewGraph(GraphOptions{Multigraph: true, Compound: true}).
		SetGraph(Attrs{}).
		SetDefaultEdgeLabel(func(string, string, *string) any { return Attrs{} })
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("got %.17g, want %.17g", got, want)
	}
}
