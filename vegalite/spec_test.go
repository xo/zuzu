package vegalite

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

// TestParseCorpus reads each Vega-Lite spec of the corpus, writes it back,
// and checks that the JSON is the same.
func TestParseCorpus(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("../test/testdata/corpus/vegalite/*.vl.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no corpus")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s, err := Parse(src)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		out, err := Write(s)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		var a, b any
		_ = json.Unmarshal(src, &a)
		_ = json.Unmarshal(out, &b)
		ja, _ := json.Marshal(a, json.Deterministic(true))
		jb, _ := json.Marshal(b, json.Deterministic(true))
		if string(ja) != string(jb) {
			i := 0
			for i < len(ja) && i < len(jb) && ja[i] == jb[i] {
				i++
			}
			lo := max(0, i-80)
			t.Errorf("%s: the spec written back differs:\nread:  %s\nwrote: %s", path, ja[lo:min(len(ja), i+80)], jb[lo:min(len(jb), i+80)])
		}
	}
}
