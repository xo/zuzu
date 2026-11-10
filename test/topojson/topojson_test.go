// Package topojson tests the mesh of a TopoJSON topology against the rows
// that the reader of vega.js gives, which oracle/topojson.mjs writes.
package topojson

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xo/zuzu/dataflow"
)

// meshCase is a case of testdata/topojson/mesh.json.
type meshCase struct {
	File   string         `json:"file"`
	Mesh   string         `json:"mesh"`
	Filter string         `json:"filter"`
	Rows   []dataflow.Row `json:"rows"`
}

func TestMesh(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../testdata/topojson/mesh.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []meshCase
	if err := json.Unmarshal(src, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join("../testdata", c.File))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := dataflow.Read(data, dataflow.Format{Type: "topojson", Mesh: c.Mesh, Filter: c.Filter}, nil)
		if err != nil {
			t.Errorf("%s %s %q: %v", c.File, c.Mesh, c.Filter, err)
			continue
		}
		if !reflect.DeepEqual(rows, c.Rows) {
			t.Errorf("%s %s %q: the mesh differs from that of vega.js: %s", c.File, c.Mesh, c.Filter, diff(rows, c.Rows))
		}
	}
}

// diff describes the first difference between two meshes.
func diff(got, want []dataflow.Row) string {
	if len(got) != 1 || len(want) != 1 {
		return "the number of rows differs"
	}
	g, _ := got[0]["coordinates"].([]any)
	w, _ := want[0]["coordinates"].([]any)
	if len(g) != len(w) {
		b, _ := json.Marshal(g)
		return "the number of lines differs: " + string(rune('0'+len(g)%10)) + " " + string(b[:min(len(b), 300)])
	}
	for i := range g {
		if !reflect.DeepEqual(g[i], w[i]) {
			gb, _ := json.Marshal(g[i])
			wb, _ := json.Marshal(w[i])
			return "line " + string(rune('0'+i%10)) + ":\ngot  " + string(gb[:min(len(gb), 200)]) + "\nwant " + string(wb[:min(len(wb), 200)])
		}
	}
	return "the rows differ outside the lines"
}
