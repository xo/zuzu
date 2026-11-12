package mermaid_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/graph/mermaid"
)

// gitWant is what a fixture of a git graph draws: the labels of its
// branches in their order, the types of its commits in their order, the
// labels of its commits, with * for an id that zuzu makes, and its tags.
type gitWant struct {
	branches []string
	types    []string
	labels   []string
	tags     []string
}

// randomID matches an id that zuzu makes for a commit with no id.
var randomID = regexp.MustCompile(`^[0-9]+-[0-9a-f]{7}$`)

func TestGitGraphFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]gitWant{
		"01-basic.mmd": {
			branches: []string{"main", "develop"},
			types:    []string{"normal", "normal", "normal", "normal", "merge", "normal", "normal"},
			labels:   []string{"*", "*", "*", "*", "*", "*"},
		},
		"02-types.mmd": {
			branches: []string{"main", "hotfix"},
			types:    []string{"normal", "normal", "reverse", "normal", "highlight", "normal", "normal", "highlight"},
			labels:   []string{"Normal", "*", "Reverse", "*", "Highlight", "*", "fix", "release"},
			tags:     []string{"v1.0.0", "RC_1", "8.8.4", "latest", "v1.1.0"},
		},
		"03-cherry-pick.mmd": {
			branches: []string{"main", "develop", "release"},
			types: []string{
				"normal", "normal", "normal", "normal", "merge", "normal", "cherry-pick", "normal", "normal",
				"cherry-pick", "cherry-pick",
			},
			labels: []string{"ZERO", "A", "ONE", "B", "MERGE", "TWO", "THREE", "C"},
			tags:   []string{"cherry-pick:MERGE|parent:B", "picked"},
		},
		"04-order.mmd": {
			branches: []string{"test2", "test3", "trunk", "test1"},
			types:    []string{"normal", "normal", "normal", "normal", "merge"},
			labels:   []string{"1", "2", "3", "4", "5"},
		},
		"05-tb.mmd": {
			branches: []string{"main", "develop", "feature"},
			types:    []string{"normal", "normal", "highlight", "normal", "merge", "normal", "normal", "merge"},
			labels:   []string{"init", "d1", "d2", "m1", "m2", "f1", "d3"},
			tags:     []string{"v0.1", "stable", "v1.0"},
		},
		"06-bt.mmd": {
			branches: []string{"main", "develop"},
			types:    []string{"normal", "normal", "normal", "normal", "normal", "merge"},
			labels:   []string{"a", "b", "c", "d", "e", "f"},
		},
		"07-parallel.mmd": {
			branches: []string{"main", "develop", "feature"},
			types:    []string{"normal", "normal", "normal", "normal", "normal", "normal", "normal", "merge", "merge"},
			labels:   []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
		},
		"08-parallel-bt.mmd": {
			branches: []string{"main", "develop", "feature"},
			types:    []string{"normal", "normal", "normal", "normal", "normal", "normal", "normal", "merge"},
			labels:   []string{"1", "2", "3", "4", "5", "6", "7", "8"},
		},
		"09-config.mmd": {
			types:  []string{"normal", "normal", "normal", "merge"},
			labels: []string{"one", "two", "three", "four"},
		},
		"10-reroute.mmd": {
			branches: []string{"main", "develop", "feature"},
			types: []string{
				"normal", "normal", "normal", "normal", "normal", "normal", "merge", "normal", "merge",
				"normal", "normal", "normal", "merge", "merge",
			},
			labels: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"},
		},
		"11-header.mmd": {
			branches: []string{"main", "Feature A (ongoing)"},
			types:    []string{"normal", "normal", "normal", "merge"},
			labels:   []string{"1", "2", "3", "4"},
		},
		"12-branches.mmd": {
			branches: []string{"main", "b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10"},
			types:    slices.Repeat([]string{"normal"}, 11),
			labels:   slices.Repeat([]string{"*"}, 11),
		},
	}
	dir := filepath.Join("testdata", "gitgraph")
	files, err := filepath.Glob(filepath.Join(dir, "*.mmd"))
	if err != nil || len(files) != len(cases) {
		t.Fatalf("expected %d fixtures, got %d: %v", len(cases), len(files), err)
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			items, report, err := drawDiagram(t, string(src), true)
			if err != nil || len(report) > 0 {
				t.Fatalf("draw: %v %v", err, report)
			}
			texts := func(mark string) []string {
				var out []string
				for _, it := range drawnOf(items, mark) {
					s := it.text
					if randomID.MatchString(s) {
						s = "*"
					}
					out = append(out, s)
				}
				return out
			}
			var types []string
			for _, it := range drawnOf(items, "commits") {
				typ, _ := it.datum["type"].(string)
				types = append(types, typ)
			}
			for _, c := range []struct {
				what      string
				got, want []string
			}{
				{"branches", texts("branchLabels"), want.branches},
				{"types", types, want.types},
				{"labels", texts("commitLabels"), want.labels},
				{"tags", texts("tagLabels"), want.tags},
			} {
				if !slices.Equal(c.got, c.want) {
					t.Errorf("%s: expected %q, got %q", c.what, c.want, c.got)
				}
			}
			// Each branch has its lane, and each link and each commit takes
			// the color of a branch.
			if n := len(drawnOf(items, "lanes")); n != len(want.branches) {
				t.Errorf("expected %d lanes, got %d", len(want.branches), n)
			}
			for _, mark := range []string{"links", "commits", "lanes", "branchBoxes"} {
				for _, it := range drawnOf(items, mark) {
					if it.item.Stroke.Color == "" {
						t.Errorf("%s: an item with no color", mark)
					}
				}
			}
		})
	}
}

func TestGitGraphColors(t *testing.T) {
	t.Parallel()
	// The branches take the colors of the scheme in their order, and the
	// eleventh branch takes the first color again.
	src, err := os.ReadFile(filepath.Join("testdata", "gitgraph", "12-branches.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := drawDiagram(t, string(src), true)
	if err != nil {
		t.Fatal(err)
	}
	var colors []string
	for _, it := range drawnOf(items, "branchBoxes") {
		colors = append(colors, it.item.Stroke.Color)
	}
	if len(colors) != 11 || colors[10] != colors[0] || len(slices.Compact(slices.Clone(colors[:10]))) != 10 {
		t.Errorf("expected ten colors, then the first again, got %v", colors)
	}
}

func TestGitGraphErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"merge into itself":            "gitGraph\n  commit\n  merge main\n",
		"merge of the branch it is on": "gitGraph\n  commit\n  branch b\n  merge main\n",
		"merge with the same head":     "gitGraph\n  commit\n  branch b\n  checkout main\n  merge b\n",
		"merge of an unknown branch":   "gitGraph\n  commit\n  merge nope\n",
		"merge into an empty branch":   "gitGraph\n  branch b\n  commit\n  checkout main\n  merge b\n",
		"merge of an empty branch":     "gitGraph\n  branch b\n  checkout main\n  commit\n  merge b\n",
		"merge with a used id":         "gitGraph\n  commit id: \"a\"\n  branch b\n  commit\n  checkout main\n  merge b id: \"a\"\n",
		"checkout of an unknown":       "gitGraph\n  commit\n  checkout nope\n",
		"branch that exists":           "gitGraph\n  commit\n  branch main\n",
		"cherry-pick of an unknown":    "gitGraph\n  commit\n  branch b\n  cherry-pick id: \"x\"\n",
		"cherry-pick with no id":       "gitGraph\n  commit\n  branch b\n  cherry-pick\n",
		"cherry-pick on its branch":    "gitGraph\n  commit id: \"a\"\n  cherry-pick id: \"a\"\n",
		"cherry-pick of a merge":       "gitGraph\n  commit\n  branch b\n  commit\n  checkout main\n  merge b id: \"m\"\n  branch c\n  cherry-pick id: \"m\"\n",
		"cherry-pick with a parent":    "gitGraph\n  commit id: \"a\"\n  branch b\n  commit id: \"c\"\n  checkout main\n  cherry-pick id: \"c\" parent: \"z\"\n",
		"cherry-pick into no commits":  "gitGraph\n  branch b\n  commit id: \"a\"\n  checkout main\n  cherry-pick id: \"a\"\n",
		"an id with no quotes":         "gitGraph\n  commit id: abc\n",
		"a space before the colon":     "gitGraph\n  commit id :\"a\"\n",
		"a direction with no colon":    "gitGraph TB\n  commit\n",
		"a direction in lower case":    "gitGraph tb:\n  commit\n",
		"an order with a fraction":     "gitGraph\n  branch b order: 2.5\n",
		"a negative order":             "gitGraph\n  branch b order: -1\n",
		"a type in lower case":         "gitGraph\n  commit type: reverse\n",
		"a key in upper case":          "gitGraph\n  commit TYPE: REVERSE\n",
		"a keyword in another case":    "gitGraph\n  Commit\n",
		"two statements on a line":     "gitGraph\n  commit; commit\n",
		"a branch named as a number":   "gitGraph\n  commit\n  branch 2024\n",
		"a branch named as a keyword":  "gitGraph\n  commit\n  branch commit\n",
		"a name that is not ASCII":     "gitGraph\n  commit\n  branch café\n",
		"a string with no end":         "gitGraph\n  commit id: \"a\n",
		"an unknown statement":         "gitGraph\n  push\n",
		"a merge type of a cherry":     "gitGraph\n  commit id: \"a\"\n  branch b\n  cherry-pick id: \"a\" type: NORMAL\n",
		"a title with a colon":         "gitGraph\n  title: x\n  commit\n",
	}
	for name, src := range tests {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, _, _, err := mermaid.Chart(d, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%s: expected an error of the syntax, got %v", name, err)
		}
	}
}

func TestGitGraphReads(t *testing.T) {
	t.Parallel()
	// Each source draws, as it does in Mermaid.
	tests := map[string]string{
		"a colon after the keyword":       "gitGraph:\n  commit\n",
		"a statement after the keyword":   "gitGraph commit\n",
		"a statement after the colon":     "gitGraph: commit id: \"x\"\n",
		"a direction and a spaced colon":  "gitGraph TB :\n  commit\n",
		"a message with no key":           "gitGraph\n  commit \"hello\"\n",
		"options with no spaces":          "gitGraph\n  commit id:\"a\"type:HIGHLIGHT tag:\"t1\"tag:\"t2\"\n",
		"single quotes and an escape":     "gitGraph\n  commit id:'a b' tag:'x\\\"y'\n",
		"a branch with dots and a slash":  "gitGraph\n  commit\n  branch release/v1.0\n  commit\n",
		"a branch that ends in a dash":    "gitGraph\n  commit\n  branch _x-\n  commit\n",
		"switch":                          "gitGraph\n  commit\n  branch b\n  switch main\n  commit\n",
		"a branch with no commit under":   "gitGraph\n  branch b\n  commit\n",
		"a merge of a branch twice":       "gitGraph\n  commit\n  branch b\n  commit\n  checkout main\n  merge b\n  checkout b\n  commit\n  checkout main\n  merge b\n",
		"a title and accessibility":       "gitGraph\n  title Hello\n  accTitle: Hi\n  accDescr: there\n  commit\n",
		"a commit with the same id":       "gitGraph\n  commit id:\"a\"\n  commit id:\"a\"\n",
		"an empty id":                     "gitGraph\n  commit id:\"\"\n",
		"a cherry-pick of a merge":        "gitGraph\n  commit id:\"a\"\n  branch b\n  commit id:\"c\"\n  checkout main\n  commit\n  merge b id:\"m\"\n  branch d\n  commit\n  cherry-pick id:\"m\" parent:\"c\"\n",
		"a comment at the end of a line":  "gitGraph\n  commit id: \"a\" %% a comment\n",
		"a merge with the same name head": "gitGraph\n  commit\n  branch b\n  commit\n  branch c\n  commit\n  checkout b\n  merge c\n  checkout main\n  merge b\n",
	}
	for name, src := range tests {
		c, report, err := zuzu.Parse([]byte(src), zuzu.WithStrict(true))
		if err != nil || len(report) > 0 {
			t.Errorf("%s: %v %v", name, err, report)
			continue
		}
		var b bytes.Buffer
		if _, err := c.SVG(context.Background(), &b); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestGitGraphDescription(t *testing.T) {
	t.Parallel()
	d, err := mermaid.Read("gitGraph\n  title Hello\n  accTitle: Hi\n  accDescr {\n    there\n    again\n  }\n  commit\n")
	if err != nil {
		t.Fatal(err)
	}
	b, _, report, err := mermaid.Chart(d, true)
	if err != nil || len(report) > 0 {
		t.Fatalf("%v %v", err, report)
	}
	for _, want := range []string{`"description":"Hi there again"`, `"text":"Hello"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("expected %s in the spec", want)
		}
	}
}

func TestGitGraphSameIDs(t *testing.T) {
	t.Parallel()
	// The ids that zuzu makes are the same on each run, and a commit with
	// the id of an earlier commit replaces it, as in Mermaid.
	src := "gitGraph\n  commit\n  commit id: \"a\"\n  commit id: \"a\"\n  branch b\n  commit\n  checkout main\n  merge b\n"
	var first string
	for range 3 {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		b, _, _, err := mermaid.Chart(d, true)
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = string(b)
		} else if string(b) != first {
			t.Fatal("expected the same spec on each run")
		}
	}
	items, _, err := drawDiagram(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(drawnOf(items, "commits")); n != 4 {
		t.Errorf("expected 4 commits, got %d", n)
	}
}

func TestGitGraphSettings(t *testing.T) {
	t.Parallel()
	// A setting that a git graph does not read, or a value of the wrong
	// type, is an issue.
	for _, cfg := range []string{
		"  gitGraph:\n    titleTopMargin: 10\n",
		"  gitGraph:\n    showBranches: maybe\n",
		"  gitGraph:\n    mainBranchOrder: x\n",
		"  parallelCommits: true\n",
	} {
		src := "---\nconfig:\n" + cfg + "---\ngitGraph\n  commit\n"
		if _, _, err := zuzu.Parse([]byte(src), zuzu.WithStrict(true)); err == nil {
			t.Errorf("%q: expected an error in the strict mode", cfg)
		}
		_, report, err := zuzu.Parse([]byte(src))
		if err != nil || len(report) != 1 {
			t.Errorf("%q: expected one issue, got %v %v", cfg, err, report)
		}
	}
	// showCommitLabel: false draws no label.
	items, _, err := drawDiagram(t, "%%{init: {'gitGraph': {'showCommitLabel': false}}}%%\ngitGraph\n  commit id: \"a\"\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(drawnOf(items, "commitLabels")); n != 0 {
		t.Errorf("expected no label, got %d", n)
	}
}
