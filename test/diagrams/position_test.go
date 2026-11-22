package diagrams

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/elk"
	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/graph/mermaid"
)

func TestPositionErrorScale(t *testing.T) {
	t.Parallel()
	ref := Places{"a": {0, 0}, "b": {100, 0}, "c": {50, 80}}
	scaled := Places{"a": {10, 5}, "b": {60, 5}, "c": {35, 45}, "d": {1, 1}}
	if e, n, ok := PositionError(scaled, ref); !ok || n != 3 || e > 1e-12 {
		t.Errorf("expected no error for the same places at another scale, got %g over %d nodes", e, n)
	}
	// a and b swap, and each moves across the box, 100 of its diagonal of
	// 128.06.
	swapped := Places{"a": {100, 0}, "b": {0, 0}, "c": {50, 80}}
	e, _, _ := PositionError(swapped, ref)
	if want := 2.0 / 3 * 100 / math.Hypot(100, 80); math.Abs(e-want) > 1e-12 {
		t.Errorf("expected %g for two nodes that swap, got %g", want, e)
	}
	if _, _, ok := PositionError(Places{"a": {0, 0}}, ref); ok {
		t.Error("expected no error over one node")
	}
	if _, _, ok := PositionError(Places{"a": {3, 3}, "b": {3, 3}}, ref); ok {
		t.Error("expected no error for nodes on one point")
	}
}

// positionKinds are the kinds of Mermaid whose fixtures D39 part 12 compares.
var positionKinds = map[string]string{
	"flowchart": "flowchart", "graph": "flowchart", "flowchart-elk": "flowchart",
	"stateDiagram": "state", "stateDiagram-v2": "state",
	"classDiagram": "class", "classDiagram-v2": "class", "erDiagram": "er",
}

// TestPositionError reports the error of the places of the nodes of
// zuzu.layered, zuzu.dagre and zuzu.elk against the references of mmdc,
// for each fixture of a flowchart, a state diagram, a class diagram and an
// ER diagram (D40 part 11, D44 part 4). The engine of ELK is available to
// each column. The columns of zuzu.layered and zuzu.dagre force their
// layout with zuzu.WithLayout, and the column of zuzu.elk forces none, so
// that each fixture draws with ELK as its source asks (D45). It never fails,
// because the number informs the choice of a layout and is not a test. Run
// it with -v to see the numbers.
//
// The references in testdata come from mmdc with its default layout, which
// is ELK in Mermaid 12.3. Set ZUZU_DAGRE_REFS to a folder of references that
// cmd/mmdcref wrote from mmdc with the config {"layout": "dagre"}, one file
// for each fixture with the same name, to compare with dagre too.
func TestPositionError(t *testing.T) {
	t.Parallel()
	files, _ := filepath.Glob("../testdata/diagrams/mermaid/*.mmd")
	dagreRefs := os.Getenv("ZUZU_DAGRE_REFS")
	type column struct {
		name, layout string
		elk          bool
	}
	cols := []column{
		{"layered/elk", graph.EngineLayered, true},
		{"dagre/elk", graph.EngineDagre, true},
		{"elk/elk", "", true},
	}
	if dagreRefs != "" {
		cols = append(cols, column{"layered/dagre", graph.EngineLayered, false}, column{"dagre/dagre", graph.EngineDagre, false})
	}
	errs := make([][]float64, len(cols))
	var lines []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		d, err := mermaid.Read(string(src))
		if err != nil {
			t.Fatal(err)
		}
		kind, ok := positionKinds[d.Kind]
		if !ok {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(f), ".mmd")
		refs := map[bool]Places{}
		if refs[true], err = ReadPlaces(strings.TrimSuffix(f, ".mmd") + ".json"); err != nil {
			t.Fatal(err)
		}
		if dagreRefs != "" {
			if refs[false], err = ReadPlaces(filepath.Join(dagreRefs, base+".json")); err != nil {
				t.Logf("%s: no reference of dagre: %v", base, err)
			}
		}
		places := map[string]Places{}
		var line strings.Builder
		fmt.Fprintf(&line, "%-32s %-9s", base, kind)
		row := make([]float64, len(cols))
		complete := true
		for k, c := range cols {
			p, ok := places[c.layout]
			if !ok {
				opts := []zuzu.Option{zuzu.WithLayoutEngine(elk.Layout())}
				if c.layout != "" {
					opts = append(opts, zuzu.WithLayout(c.layout))
				}
				if p, _, err = ZuzuPlaces(t.Context(), src, opts...); err != nil {
					t.Errorf("%s: %s: %v", base, c.layout, err)
				}
				places[c.layout] = p
			}
			e, _, ok := PositionError(p, refs[c.elk])
			if !ok {
				complete = false
				fmt.Fprintf(&line, " %13s", "-")
				continue
			}
			row[k] = e
			fmt.Fprintf(&line, " %13.3f", e)
		}
		lines = append(lines, line.String())
		if complete {
			for k := range cols {
				errs[k] = append(errs[k], row[k])
			}
		}
	}
	var header strings.Builder
	fmt.Fprintf(&header, "%-32s %-9s", "fixture", "kind")
	for _, c := range cols {
		fmt.Fprintf(&header, " %13s", c.name)
	}
	t.Log(header.String())
	for _, l := range lines {
		t.Log(l)
	}
	for k, c := range cols {
		t.Logf("%-13s over %d fixtures: mean %.3f, median %.3f", c.name, len(errs[k]), mean(errs[k]), median(errs[k]))
	}
	// Each pair of columns against the same reference counts the fixtures
	// that each one wins.
	for a := range cols {
		for b := a + 1; b < len(cols); b++ {
			if cols[a].elk != cols[b].elk {
				continue
			}
			wa, wb, tie := 0, 0, 0
			for i := range errs[a] {
				switch {
				case errs[a][i] < errs[b][i]-1e-9:
					wa++
				case errs[b][i] < errs[a][i]-1e-9:
					wb++
				default:
					tie++
				}
			}
			t.Logf("%s wins %d, %s wins %d, %d ties", cols[a].name, wa, cols[b].name, wb, tie)
		}
	}
}

// mean returns the mean of a list, or 0 for none.
func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// median returns the median of a list, or 0 for none.
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := slices.Clone(v)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}
