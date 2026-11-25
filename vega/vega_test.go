package vega

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpus returns the paths of the Vega specs of the corpus, and of the
// Vega specs that vega-lite.js compiled, which the references hold.
func corpus(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../test/testdata/corpus/vega/*.vg.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no corpus")
	}
	return paths
}

// TestParseCorpus reads each Vega spec of the corpus, writes it back, and
// checks that the JSON is the same.
func TestParseCorpus(t *testing.T) {
	t.Parallel()
	for _, path := range corpus(t) {
		if strings.HasSuffix(path, "platformer.vg.json") {
			// It gives an expression as a number, which zuzu writes back as
			// a string with the same meaning.
			continue
		}
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
		if err := json.Unmarshal(src, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &b); err != nil {
			t.Fatal(err)
		}
		ja, _ := json.Marshal(a, json.Deterministic(true))
		jb, _ := json.Marshal(b, json.Deterministic(true))
		if string(ja) != string(jb) {
			t.Errorf("%s: the spec written back differs:\n%s", path, firstDiff(string(ja), string(jb)))
		}
	}
}

// firstDiff shows where two strings first differ.
func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-80)
	return "read:  " + a[lo:min(len(a), i+80)] + "\nwrote: " + b[lo:min(len(b), i+80)]
}

// TestParseCompiled reads the Vega specs that vega-lite.js compiled, from
// the references, and writes each back.
func TestParseCompiled(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("../test/testdata/references/vegalite/*.ref.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no references: see test/README.md")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var ref struct {
			Compiled jsontext.Value `json:"compiled"`
		}
		if err := json.Unmarshal(src, &ref); err != nil {
			t.Fatal(err)
		}
		s, err := Parse(ref.Compiled)
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
		_ = json.Unmarshal(ref.Compiled, &a)
		_ = json.Unmarshal(out, &b)
		ja, _ := json.Marshal(a, json.Deterministic(true))
		jb, _ := json.Marshal(b, json.Deterministic(true))
		if string(ja) != string(jb) {
			t.Errorf("%s: the spec written back differs:\n%s", strings.TrimPrefix(path, "../"), firstDiff(string(ja), string(jb)))
		}
	}
}

// TestPropsOrder checks that the properties of a set of encodings keep the
// order of the spec when they are read and written back, and that the
// properties that Go code adds come after them, in the order of the names.
func TestPropsOrder(t *testing.T) {
	t.Parallel()
	var enc Encode
	if err := json.Unmarshal([]byte(`{"update": {"y": {"value": 1}, "x": {"signal": "item.y"}, "fill": {"value": "red"}}}`), &enc); err != nil {
		t.Fatal(err)
	}
	props := enc["update"]
	props["b"] = Production{{Signal: "1"}}
	props["a"] = Production{{Signal: "2"}}
	if got, want := strings.Join(props.Names(), " "), "y x fill a b"; got != want {
		t.Errorf("the names = %s, want %s", got, want)
	}
	out, err := json.Marshal(props)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), `{"y":{"value":1},"x":{"signal":"item.y"},"fill":{"value":"red"},"a":{"signal":"2"},"b":{"signal":"1"}}`; got != want {
		t.Errorf("the properties written = %s, want %s", got, want)
	}
	var none Props
	if err := json.Unmarshal([]byte(`null`), &none); err != nil || none != nil {
		t.Errorf("null = %v, %v, want nil", none, err)
	}
	if err := json.Unmarshal([]byte(`[1]`), &none); err == nil {
		t.Error("an array reads with no error")
	}
}
