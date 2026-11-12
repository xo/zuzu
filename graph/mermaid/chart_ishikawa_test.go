package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
)

// ishWant is what a fixture of an ishikawa diagram draws: the problem and
// the number of its lines, each cause in the order of the source, and the
// label of each cause of a cause.
type ishWant struct {
	head  string
	lines int
	// causes alternate above and under the spine.
	causes []string
	subs   []string
}

func TestIshikawaFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]ishWant{
		"01-docs.mmd": {
			head: "Blurry Photo", lines: 2,
			causes: []string{"Process", "User", "Equipment", "Environment"},
			subs: []string{
				"Out of focus", "Shutter speed too slow", "Protective film not removed", "Beautification filter applied",
				"Shaky hands", "LENS", "Inappropriate lens", "Damaged lens", "Dirty lens", "SENSOR", "Damaged sensor",
				"Dirty sensor", "Subject moved too quickly", "Too dark",
			},
		},
		"02-nested.mmd": {
			head: "Server Outage", lines: 2,
			causes: []string{"Hardware", "Software", "Network", "People"},
			subs: []string{
				"Disk", "Bad sectors", "Full capacity", "Memory", "Leak detected", "Old driver", "Bug", "Race condition",
				"On call",
			},
		},
		"03-flat.mmd": {
			head: "Problem", lines: 1,
			causes: []string{"Cause A", "Subcause A1", "Subcause A2", "Cause B", "Subcause B1", "Cause C"},
		},
		"04-root.mmd": {head: "Problem", lines: 1},
		"05-grammar.mmd": {
			head: "Late delivery", lines: 2,
			causes: []string{"Supplier", "Transport"},
			subs: []string{
				"Parts arrive late from the other plant", `Wrong "parts"`, "Truck broken",
				"accTitle: a cause, as Mermaid reads it",
			},
		},
		"06-settings.mmd": {
			head: "A blurry photo from the phone", lines: 6,
			causes: []string{"Process", "User"},
			subs:   []string{"Out of focus", "Shaky hands"},
		},
	}
	files := fixtures(t, "ishikawa")
	if len(files) != len(cases) {
		t.Fatalf("%d fixtures and %d cases", len(files), len(cases))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			want, ok := cases[filepath.Base(path)]
			if !ok {
				t.Fatal("no case")
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			items, report, err := drawDiagram(t, string(src), true)
			if err != nil || len(report) != 0 {
				t.Fatalf("the report %v and the error %v", report, err)
			}
			checkIshikawa(t, items, want)
		})
	}
}

// ishLabels joins the lines of the labels of a mark: a row with the line 0
// starts a label.
func ishLabels(items []drawnItem, mark string) [][]string {
	var out [][]string
	for _, it := range drawnOf(items, mark) {
		if n, _ := it.datum["line"].(float64); n == 0 || len(out) == 0 {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], it.text)
	}
	return out
}

// checkIshikawa checks that each node of a fixture draws once with its
// label, each cause with its bone on its side of the spine, and each cause
// of a cause with its bone and its arrowhead.
func checkIshikawa(t *testing.T, items []drawnItem, want ishWant) {
	t.Helper()
	head := ishLabels(items, "headLabels")
	if len(head) != 1 || strings.Join(head[0], " ") != want.head || len(head[0]) != want.lines {
		t.Errorf("the problem %q, want %q in %d lines", head, want.head, want.lines)
	}
	if n := len(drawnOf(items, "head")); n != 1 {
		t.Errorf("%d heads", n)
	}
	var causes []string
	for _, l := range ishLabels(items, "causeLabels") {
		causes = append(causes, strings.Join(l, " "))
	}
	if !slices.Equal(causes, want.causes) {
		t.Errorf("the causes %q, want %q", causes, want.causes)
	}
	var subs []string
	for _, l := range ishLabels(items, "subLabels") {
		subs = append(subs, strings.Join(l, " "))
	}
	slices.Sort(subs)
	wantSubs := slices.Sorted(slices.Values(want.subs))
	if !slices.Equal(subs, wantSubs) {
		t.Errorf("the causes of the causes %q, want %q", subs, wantSubs)
	}
	bones, small, arrows := drawnOf(items, "bones"), drawnOf(items, "subBones"), drawnOf(items, "arrows")
	if len(bones) != len(want.causes) || len(small) != len(want.subs) || len(arrows) != len(bones)+len(small) {
		t.Errorf("%d bones, %d smaller bones and %d arrowheads", len(bones), len(small), len(arrows))
	}
	spine := drawnOf(items, "spine")
	if len(want.causes) == 0 {
		if len(spine) != 0 {
			t.Error("a spine with no cause")
		}
		return
	}
	if len(spine) != 1 {
		t.Fatalf("%d spines", len(spine))
	}
	y := spine[0].item.Y
	for i, b := range drawnOf(items, "causes") {
		box := boxOf(b)
		if above := box.y1 < y; above != (i%2 == 0) {
			t.Errorf("the cause %q is on the wrong side of the spine", b.datum["text"])
		}
	}
}

func TestIshikawaWrap(t *testing.T) {
	t.Parallel()
	// Mermaid wraps the label of a cause of a cause at 15 characters, and
	// does not wrap the label of a cause.
	items, _, err := drawDiagram(t, "ishikawa-beta\n  P\n  A cause that does not wrap\n    Parts arrive late from the other plant\n", true)
	if err != nil {
		t.Fatal(err)
	}
	subs, causes := ishLabels(items, "subLabels"), ishLabels(items, "causeLabels")
	if want := [][]string{{"Parts arrive", "late from the", "other plant"}}; !slices.EqualFunc(subs, want, slices.Equal) {
		t.Errorf("the lines %q, want %q", subs, want)
	}
	if want := [][]string{{"A cause that does not wrap"}}; !slices.EqualFunc(causes, want, slices.Equal) {
		t.Errorf("the lines %q, want %q", causes, want)
	}
}

func TestIshikawaErrors(t *testing.T) {
	t.Parallel()
	// The lexer of Mermaid reads the keyword at the start of each line.
	for _, src := range []string{
		"ishikawa-beta\n  Root\n  ishikawa x\n",
		"ishikawa-beta\n  Root\n  Ishikawa-beta\n",
		"ishikawa-beta ishikawa\n",
	} {
		if _, err := chartOf(src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: the error %v", src, err)
		}
	}
	for _, src := range []string{
		"---\nconfig:\n  fontSize: big\n---\nishikawa-beta\n  Root\n",
		"---\nconfig:\n  ishikawa:\n    diagramPadding: -1\n---\nishikawa-beta\n  Root\n",
		"---\nconfig:\n  ishikawa:\n    padding: 3\n---\nishikawa-beta\n  Root\n",
	} {
		if _, err := chartOf(src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: the strict error %v", src, err)
		}
		if report, err := chartOf(src, false); err != nil || len(report) != 1 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
	// A diagram with no statement draws nothing, as in Mermaid, and the
	// math of KaTeX draws as its source, as Mermaid draws it.
	for _, src := range []string{
		"ishikawa-beta\n",
		"ishikawa\n  $$a$$\n",
		"---\nconfig:\n  fontSize: 20px\n---\nishikawa-beta\n  Root\n",
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v", src, report, err)
		}
	}
}

func TestIshikawaDescription(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("testdata", "ishikawa", "06-settings.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := mermaid.Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := mermaid.Chart(d, true)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Description != "The causes of a blurry photo." {
		t.Errorf("the description %q", spec.Description)
	}
}
