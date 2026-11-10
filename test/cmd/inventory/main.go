// Command inventory lists the features that each spec of the corpus uses, so
// that each phase can say which specs it claims to support (D7, D17).
//
// It reads the specs in testdata/corpus and writes testdata/corpus/inventory.tsv,
// one row for each spec:
//
//	spec      the path of the spec in the corpus
//	dialect   vega or vegalite
//	marks     the kinds of mark, sorted and joined by commas
//	transforms the kinds of transform
//	functions the functions that its expressions call
//	events    yes if the spec binds an event, a selection or a parameter
//	data      the data files that it names and the corpus does not hold
//	first     yes if the spec is in the first target: no events and all data
//
// Run it from the test module:
//
//	go run ./cmd/inventory
package main

import (
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func main() {
	dir := flag.String("dir", filepath.Join("testdata", "corpus"), "the folder of the corpus")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "inventory:", err)
		os.Exit(1)
	}
}

// row is the inventory of one spec.
type row struct {
	spec       string
	dialect    string
	marks      []string
	transforms []string
	functions  []string
	events     bool
	missing    []string
}

// run reads every spec in dir and writes the inventory.
func run(dir string) error {
	var rows []row
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isSpec(path) {
			return nil
		}
		r, err := inspect(dir, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		rows = append(rows, r)
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortFunc(rows, func(a, b row) int { return strings.Compare(a.spec, b.spec) })
	var sb strings.Builder
	sb.WriteString("spec\tdialect\tmarks\ttransforms\tfunctions\tevents\tdata\tfirst\n")
	var first int
	for _, r := range rows {
		ok := !r.events && len(r.missing) == 0
		if ok {
			first++
		}
		fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.spec, r.dialect,
			strings.Join(r.marks, ","), strings.Join(r.transforms, ","),
			strings.Join(r.functions, ","), yes(r.events), strings.Join(r.missing, ","), yes(ok))
	}
	out := filepath.Join(dir, "inventory.tsv")
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", out, err)
	}
	fmt.Printf("%d specs, %d in the first target, written to %s\n", len(rows), first, out)
	return nil
}

// isSpec reports whether a file of the corpus is a spec rather than data.
func isSpec(path string) bool {
	return strings.HasSuffix(path, ".vl.json") || strings.HasSuffix(path, ".vg.json")
}

// yes writes a bool as the inventory writes it.
func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// errNotObject is the error of a spec whose top level is not a JSON object.
var errNotObject = errors.New("spec is not a JSON object")

// inspect reads one spec and lists its features.
func inspect(dir, path string) (row, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return row{}, err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return row{}, err
	}
	spec, ok := v.(map[string]any)
	if !ok {
		return row{}, errNotObject
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return row{}, err
	}
	f := features{
		marks:      map[string]bool{},
		transforms: map[string]bool{},
		functions:  map[string]bool{},
		urls:       map[string]bool{},
	}
	r := row{spec: filepath.ToSlash(rel), dialect: "vegalite"}
	if strings.HasSuffix(path, ".vg.json") {
		r.dialect = "vega"
		f.vega(spec)
	} else {
		f.vegalite(spec)
	}
	for url := range f.urls {
		if _, err := os.Stat(filepath.Join(dir, url)); err != nil {
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), url)); err != nil {
				r.missing = append(r.missing, url)
			}
		}
	}
	slices.Sort(r.missing)
	r.marks = slices.Sorted(maps.Keys(f.marks))
	r.transforms = slices.Sorted(maps.Keys(f.transforms))
	r.functions = slices.Sorted(maps.Keys(f.functions))
	r.events = f.events
	return r, nil
}

// features collects what a spec uses as the walk goes.
type features struct {
	marks      map[string]bool
	transforms map[string]bool
	functions  map[string]bool
	urls       map[string]bool
	events     bool
}

// exprKeys are the properties whose string value is an expression.
var exprKeys = map[string]bool{
	"expr": true, "signal": true, "update": true, "test": true,
	"calculate": true, "filter": true, "labelExpr": true, "init": true,
}

// call matches the name of a function in an expression.
var call = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// vega walks a Vega spec.
func (f *features) vega(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			switch k {
			case "marks":
				for _, m := range list(val) {
					if t, ok := field(m, "type"); ok {
						f.marks[t] = true
					}
				}
			case "transform":
				for _, t := range list(val) {
					if name, ok := field(t, "type"); ok {
						f.transforms[name] = true
					}
				}
			case "on":
				f.events = true
			case "url":
				if s, ok := val.(string); ok {
					f.urls[s] = true
				}
			}
			f.expression(k, val)
			f.vega(val)
		}
	case []any:
		for _, e := range x {
			f.vega(e)
		}
	}
}

// vegalite walks a Vega-Lite spec.
func (f *features) vegalite(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			switch k {
			case "mark":
				if s, ok := val.(string); ok {
					f.marks[s] = true
				} else if t, ok := field(val, "type"); ok {
					f.marks[t] = true
				}
			case "transform":
				for _, t := range list(val) {
					if m, ok := t.(map[string]any); ok {
						for name := range m {
							if name != "as" && name != "groupby" {
								f.transforms[name] = true
								break
							}
						}
					}
				}
			case "params", "selection":
				if hasSelectOrBind(val) {
					f.events = true
				}
			case "url":
				if s, ok := val.(string); ok {
					f.urls[s] = true
				}
			}
			f.expression(k, val)
			f.vegalite(val)
		}
	case []any:
		for _, e := range x {
			f.vegalite(e)
		}
	}
}

// expression records the functions of a property that holds an expression.
func (f *features) expression(key string, val any) {
	s, ok := val.(string)
	if !ok || !exprKeys[key] {
		return
	}
	for _, m := range call.FindAllStringSubmatch(s, -1) {
		f.functions[m[1]] = true
	}
}

// hasSelectOrBind reports whether a Vega-Lite params or selection value
// defines a selection or binds an input, which a person changes.
func hasSelectOrBind(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if _, ok := x["select"]; ok {
			return true
		}
		if _, ok := x["bind"]; ok {
			return true
		}
		if _, ok := x["type"]; ok {
			// a selection of Vega-Lite 4, keyed by its name
			return true
		}
		for _, e := range x {
			if hasSelectOrBind(e) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(x, hasSelectOrBind)
	}
	return false
}

// list returns v as a slice, or nil.
func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// field returns the string field name of an object.
func field(v any, name string) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	s, ok := m[name].(string)
	return s, ok
}
