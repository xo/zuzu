package mermaid_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/vega"
)

// tvWant is what a fixture of a tree view draws: each item as its name and
// its depth, in the order of the rows, then the descriptions, the items
// with a band of highlight, the icons of the packs and the number of
// icons of a folder or a file.
type tvWant struct {
	items      []string
	descs      []string
	highlights []string
	icons      []string
	glyphs     int
}

func TestTreeViewFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]tvWant{
		"01-indents.mmd": {items: []string{
			"0 /", "1 packages", "2 mermaid", "3 src", "4 parser.ts", "4 renderer.ts", "3 package.json",
			"2 parser", "3 src", "1 README.md",
		}},
		"02-boxes.mmd": {items: []string{
			"0 /", "1 src", "2 index.ts", "2 utils", "3 strings.ts", "1 package.json", "1 README.md",
		}},
		"03-annotations.mmd": {
			items: []string{
				"0 /", "1 my-project", "2 src", "3 App.tsx", "3 index.js", "2 .env", "2 Dockerfile", "2 package.json",
			},
			descs:      []string{"main component", "entry point", "environment variables"},
			highlights: []string{"App.tsx", "Dockerfile"},
			icons:      []string{"fa:bell"},
		},
		"04-icons.mmd": {
			items: []string{
				"0 /", "1 src", "2 index.js", "2 alarm.ts", "2 empty.js", "1 Dockerfile", "1 notes.txt", "1 README.md",
			},
			icons:  []string{"fa:bell", "fa:gear", "fa:book"},
			glyphs: 3,
		},
		"05-config.mmd": {
			items: []string{
				"0 /", "1 folder1", "1 file1.js", "2 file2.ts", "1 folder2", "2 folder3", "3 file4.ts",
			},
			descs: []string{"a file"},
		},
	}
	files := fixtures(t, "treeview")
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
			checkTreeView(t, items, want)
		})
	}
}

// checkTreeView checks the rows, the lines, the descriptions, the bands
// and the icons of a tree view that drew.
func checkTreeView(t *testing.T, items []drawnItem, want tvWant) {
	t.Helper()
	var got []string
	names := map[string]string{}
	ys := map[string]float64{}
	for _, it := range drawnOf(items, "labels") {
		depth, _ := it.datum["depth"].(float64)
		got = append(got, strconv.Itoa(int(depth))+" "+it.text)
		names[datumID(it)] = it.text
		ys[datumID(it)] = it.item.Y
	}
	if !slices.Equal(got, want.items) {
		t.Errorf("the items %q, want %q", got, want.items)
	}
	// Each row is under the one before it.
	labels := drawnOf(items, "labels")
	for i := 1; i < len(labels); i++ {
		if labels[i].item.Y <= labels[i-1].item.Y {
			t.Errorf("the row of %s is not under the row of %s", labels[i].text, labels[i-1].text)
		}
	}
	// A line joins each item to its parent, and a line runs down from each
	// item with children.
	depth := func(item string) int {
		d, _ := strconv.Atoi(item[:strings.IndexByte(item, ' ')])
		return d
	}
	parents := 0
	for i := 1; i < len(want.items); i++ {
		if depth(want.items[i]) > depth(want.items[i-1]) {
			parents++
		}
	}
	if n := len(drawnOf(items, "lines")); n != len(want.items)-1+parents {
		t.Errorf("%d lines, want %d", n, len(want.items)-1+parents)
	}
	var descs []string
	for _, it := range drawnOf(items, "descriptions") {
		descs = append(descs, it.text)
		if ys[datumID(it)] != it.item.Y {
			t.Errorf("the description %q is not on the row of %s", it.text, names[datumID(it)])
		}
	}
	if !slices.Equal(descs, want.descs) {
		t.Errorf("the descriptions %q, want %q", descs, want.descs)
	}
	var highs []string
	for _, it := range drawnOf(items, "highlights") {
		highs = append(highs, names[datumID(it)])
	}
	if !slices.Equal(highs, want.highlights) {
		t.Errorf("the highlights %q, want %q", highs, want.highlights)
	}
	var icons []string
	for _, it := range items {
		if ic, ok := it.datum["icon"].(string); ok && it.role == "" {
			icons = append(icons, ic)
		}
	}
	if !slices.Equal(icons, want.icons) {
		t.Errorf("the icons %q, want %q", icons, want.icons)
	}
	if n := len(drawnOf(items, "glyphs")); n != want.glyphs {
		t.Errorf("%d icons of a folder or a file, want %d", n, want.glyphs)
	}
}

// treeViewSpec compiles a tree view and returns its spec of Vega.
func treeViewSpec(t *testing.T, src string) map[string]any {
	t.Helper()
	d, err := mermaid.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _, report, err := mermaid.Chart(d, true)
	if err != nil || len(report) != 0 {
		t.Fatalf("the report %v and the error %v", report, err)
	}
	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

// treeViewNames returns the names of the items of a spec of a tree view,
// each after its depth.
func treeViewNames(spec map[string]any) []string {
	var out []string
	for _, v := range treeViewData(spec)["labels"] {
		row, _ := v.(map[string]any)
		depth, _ := row["depth"].(float64)
		name, _ := row["text"].(string)
		out = append(out, strconv.Itoa(int(depth))+" "+name)
	}
	return out
}

// treeViewData returns the rows of each data set of a spec of a tree view.
func treeViewData(spec map[string]any) map[string][]any {
	out := map[string][]any{}
	sets, _ := spec["data"].([]any)
	for _, d := range sets {
		m, _ := d.(map[string]any)
		name, _ := m["name"].(string)
		out[name], _ = m["values"].([]any)
	}
	return out
}

// TestTreeViewGrammar reads sources that Mermaid 12.3 reads, and checks
// the items that each gives, as mmdc draws them.
func TestTreeViewGrammar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src  string
		want []string
	}{
		// The root alone.
		{"treeView-beta\n", []string{"0 /"}},
		// An indent counts its characters, so a tab is one.
		{"treeView-beta\n\ta\n    b\n\t c\n", []string{"0 /", "1 a", "2 b", "2 c"}},
		// The word title starts a title, so titles.txt is the title
		// "title" and the item s.txt, with no indent.
		{"treeView-beta\n  titles.txt\n  b\n", []string{"0 /", "1 s.txt", "2 b"}},
		// Two names in quotes on one line are two items.
		{"treeView-beta\n  \"a\" \"b\"\n  c\n", []string{"0 /", "1 a", "1 b", "2 c"}},
		// %% after a name with no quotes is part of the name.
		{"treeView-beta\n  a.txt %% c\n  b\n", []string{"0 /", "1 a.txt %% c", "1 b"}},
		// A comment on a line of its own drops.
		{"treeView-beta\n  a\n  %% a note\n  b\n", []string{"0 /", "1 a", "1 b"}},
		// A name keeps its spaces, and a slash at its end makes a
		// directory.
		{"treeView-beta\n  my  file  \n  'dir/'\n", []string{"0 /", "1 my  file", "1 dir"}},
		// An item with less indent than the first goes to the root.
		{"treeView-beta\n    a\n      b\n  c\n", []string{"0 /", "1 a", "2 b", "1 c"}},
		// Spaces after a name in quotes are the indent of the next item.
		{"treeView-beta\n\"a\"  \nb\n", []string{"0 /", "1 a", "2 b"}},
		// A tree of boxes takes its depth from the column of the branch.
		{"treeView-beta\n├── a\n│   └── b\n    └── c\n└── d\n", []string{"0 /", "1 a", "2 b", "2 c", "1 d"}},
		// A tree of boxes with branches of one dash and a line of only │.
		{"treeView-beta\n├─ a\n│\n│  └─ b\n└─ c\n", []string{"0 /", "1 a", "2 b", "1 c"}},
	}
	for _, c := range cases {
		spec := treeViewSpec(t, c.src)
		if got := treeViewNames(spec); !slices.Equal(got, c.want) {
			t.Errorf("%q: the items %q, want %q", c.src, got, c.want)
		}
	}
}

// TestTreeViewAnnotations checks the annotations of an item: the last of
// each kind wins, as in Mermaid, and the text of a description runs to the
// end of the line.
func TestTreeViewAnnotations(t *testing.T) {
	t.Parallel()
	spec := treeViewSpec(t, "treeView-beta\n  \"a\" icon(fa:bell) :::x ## d :::y\n  b :::x :::highlight\n  c ## one ## two\n")
	data := treeViewData(spec)
	var descs []string
	for _, v := range data["descriptions"] {
		row, _ := v.(map[string]any)
		text, _ := row["text"].(string)
		descs = append(descs, text)
	}
	if !slices.Equal(descs, []string{"d :::y", "one ## two"}) {
		t.Errorf("the descriptions %q", descs)
	}
	first := func(set, key string) any {
		if len(data[set]) != 1 {
			return nil
		}
		row, _ := data[set][0].(map[string]any)
		return row[key]
	}
	if first("highlights", "id") != "n2" {
		t.Errorf("the highlights %v, want the item b", data["highlights"])
	}
	if first("icons", "icon") != "fa:bell" {
		t.Errorf("the icons %v, want fa:bell", data["icons"])
	}
}

// TestTreeViewDescription checks that accTitle and accDescr go in the
// description of the spec, after the description of the front matter, and
// that the title draws.
func TestTreeViewDescription(t *testing.T) {
	t.Parallel()
	spec := treeViewSpec(t, "---\ndescription: Files\n---\ntreeView-beta\n  title The tree\n  accDescr {\n    the lines\n    of a block\n  }\n  accTitle: A tree\n  a\n")
	if got := spec["description"]; got != "Files the lines\nof a block A tree" {
		t.Errorf("the description %q", got)
	}
	title, _ := spec["title"].(map[string]any)
	if title["text"] != "The tree" {
		t.Errorf("the title %v", spec["title"])
	}
}

// TestTreeViewErrors checks that zuzu does not read the sources that
// Mermaid 12.3 does not read.
func TestTreeViewErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		// Text after the keyword, even a space.
		"treeView-beta x\n  a\n",
		"treeView-beta  \n  a\n",
		// Spaces after a name in quotes, or after an annotation.
		"treeView-beta\n  \"a\"  \n  c\n",
		"treeView-beta\n  a :::hi  \n  c\n",
		// A line of only spaces.
		"treeView-beta\n  a\n   \n  c\n",
		// %% alone on a line does not drop, and then the indent before it
		// has no name.
		"treeView-beta\n  a\n  %%\n  c\n",
		// Spaces at the end of the source.
		"treeView-beta\n  'a' \n",
		// A title after the first item.
		"treeView-beta\n  a\n  title x\n",
		// An icon with two colons starts no token.
		"treeView-beta\n  a icon(x:y:z)\n",
		// A class right after a name in quotes.
		"treeView-beta\n  \"a\":::x\n",
		// An indent with no branch in a tree of boxes, and a branch with
		// no name.
		"treeView-beta\n├── a\n  b\n",
		"treeView-beta\n├──\n",
		// The keyword again.
		"treeView-beta\n  a\ntreeView-beta\n",
	} {
		d, err := mermaid.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := mermaid.Chart(d, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected an error of the syntax, got %v", src, err)
		}
	}
}

// TestTreeViewSettings checks the settings that a tree view reads, and
// that a value of the wrong type, another setting and another theme
// variable are issues.
func TestTreeViewSettings(t *testing.T) {
	t.Parallel()
	ok := "---\nconfig:\n  treeView:\n    useMaxWidth: false\n    rowIndent: 20\n    showIcons: true\n" +
		"  themeVariables:\n    treeView:\n      iconColor: '#123456'\n      highlightBg: 'rgba(255, 193, 7, 0.15)'\n      highlightStroke: '#ffc107'\n" +
		"---\ntreeView-beta\n  a.txt :::highlight\n"
	if _, err := chartOf(ok, true); err != nil {
		t.Errorf("%q: %v", ok, err)
	}
	for _, src := range []string{
		"---\nconfig:\n  treeView:\n    rowIndent: wide\n---\ntreeView-beta\n  a\n",
		"---\nconfig:\n  treeView:\n    showIcons: yes please\n---\ntreeView-beta\n  a\n",
		"---\nconfig:\n  treeView:\n    extensionIcons: none\n---\ntreeView-beta\n  a\n",
		"---\nconfig:\n  treeView:\n    nodeSpacing: 3\n---\ntreeView-beta\n  a\n",
		"---\nconfig:\n  themeVariables:\n    primaryColor: '#ff0000'\n---\ntreeView-beta\n  a\n",
		"---\nconfig:\n  themeVariables:\n    treeView:\n      labelFontSize: big\n---\ntreeView-beta\n  a\n",
	} {
		if _, err := chartOf(src, true); !errors.Is(err, vega.ErrUnsupported) {
			t.Errorf("%q: expected an unsupported setting, got %v", src, err)
		}
	}
}
