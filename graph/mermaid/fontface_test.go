package mermaid_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/zuzu"
	"github.com/xo/zuzu/icons/carbon"
	"github.com/xo/zuzu/icons/fa"
	"github.com/xo/zuzu/icons/k8s"
	"github.com/xo/zuzu/icons/mdi"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/theme"
)

// withFamily returns a source of Mermaid with a directive that sets
// fontFamily, after the front matter of the source when it has one.
func withFamily(src, family string) string {
	directive := "%%{init: {\"fontFamily\": \"" + family + "\"}}%%\n"
	if rest, ok := strings.CutPrefix(src, "---\n"); ok {
		if i := strings.Index(rest, "\n---\n"); i >= 0 {
			return "---\n" + rest[:i+5] + directive + rest[i+5:]
		}
	}
	return directive + src
}

// drawScene renders a source of Mermaid in the loose mode and returns its
// scene and the report.
func drawScene(t *testing.T, src string) (*scene.Scene, zuzu.Report) {
	t.Helper()
	c, report, err := zuzu.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s, more, err := c.Render(context.Background(),
		zuzu.WithLocation(time.UTC), zuzu.WithLook(theme.LookStandard),
		zuzu.WithIcons("fa", fa.Pack()), zuzu.WithIcons("mdi", mdi.Pack()),
		zuzu.WithIcons("carbon", carbon.Pack()), zuzu.WithIcons("k8s", k8s.Pack()))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return s, append(report, more...)
}

// sceneTexts returns the items of the text marks of a scene, titles too,
// by their text. It leaves out the labels that the look of zuzu adds, such
// as the total of a donut, whose roles start with zuzu-, because the
// runtime draws them and not the spec.
func sceneTexts(s *scene.Scene) map[string][]*scene.Item {
	out := map[string][]*scene.Item{}
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		if strings.HasPrefix(m.Role, "zuzu-") {
			return
		}
		for i := range m.Items {
			it := &m.Items[i]
			if s := strings.TrimSpace(strings.Join(it.Text, " ")); m.Type == scene.Text && s != "" {
				out[s] = append(out[s], it)
			}
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(&s.Root)
	return out
}

// noFamilyIssue fails a test for each issue of a report that names
// fontFamily.
func noFamilyIssue(t *testing.T, report zuzu.Report) {
	t.Helper()
	for _, is := range report {
		if strings.Contains(is.Message, "fontFamily") {
			t.Errorf("the issue %v", is)
		}
	}
}

// TestChartFontFamily checks that each kind of chart, in each of its
// fixtures, gives the weight and the style of fontFamily to all of its
// text, and that fontFamily is no issue (D58). Text at the normal weight
// takes the light weight of the family, and a bold heading stays bold.
func TestChartFontFamily(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "*.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		switch filepath.Base(filepath.Dir(path)) {
		case "sequence", "swimlane", "usecase":
			// The graphs and the sequence diagram read fontFamily
			// themselves, and TestFontFamily and TestSequenceFontFamily
			// check them.
			continue
		}
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			s, report := drawScene(t, withFamily(string(b), "Helvetica-Light-Oblique"))
			noFamilyIssue(t, report)
			texts := sceneTexts(s)
			if len(texts) == 0 {
				t.Fatal("no text")
			}
			for str, items := range texts {
				for _, it := range items {
					if it.FontStyle != "italic" {
						t.Errorf("%q has the style %q, want italic", str, it.FontStyle)
					}
					if w := string(it.FontWeight); w == "" || w == "400" || w == "normal" {
						t.Errorf("%q has the weight %q, want 300 or bold", str, w)
					}
				}
			}
		})
	}
}

// TestChartFontFamilyWins checks that the settings of a kind that give a
// weight or a style win over fontFamily: a style of a block, and the
// family of the participants of a sequence diagram. The text that they do
// not set takes fontFamily.
func TestChartFontFamilyWins(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, src string
		// want maps a text to its weight and its style.
		want map[string][2]string
	}{
		{
			"block",
			"block\n  a[\"Plain\"] b[\"Styled\"]\n  style b font-weight:normal,font-style:normal\n",
			map[string][2]string{"Plain": {"600", "italic"}, "Styled": {"normal", "normal"}},
		},
		{
			"sequence",
			"---\nconfig:\n  sequence:\n    actorFontFamily: Helvetica-Light\n---\nsequenceDiagram\n  Alice->>Bob: Hello\n",
			map[string][2]string{"Alice": {"300", ""}, "Hello": {"600", "italic"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s, report := drawScene(t, withFamily(test.src, "Helvetica-BoldOblique"))
			noFamilyIssue(t, report)
			texts := sceneTexts(s)
			for str, want := range test.want {
				if len(texts[str]) == 0 {
					t.Fatalf("no text %q", str)
				}
				for _, it := range texts[str] {
					if got := [2]string{string(it.FontWeight), it.FontStyle}; got != want {
						t.Errorf("%q has the weight and the style %q, want %q", str, got, want)
					}
				}
			}
		})
	}
}

// TestChartFontFamilyMeasures checks that a kind that lays out when it
// compiles measures its text in the weight of fontFamily, so that a box
// grows to hold a bold label.
func TestChartFontFamilyMeasures(t *testing.T) {
	t.Parallel()
	const src = "railroad-beta\nrule = terminal(\"a long terminal of many letters\") ;\n"
	plain, _ := drawScene(t, src)
	bold, _ := drawScene(t, withFamily(src, "Helvetica-Bold"))
	if bold.Width <= plain.Width {
		t.Errorf("the width %g in a bold family, want more than %g", bold.Width, plain.Width)
	}
}

// TestChartFontFamilyIssues checks that fontFamily at the top of the config
// is no issue in a chart, even in the strict mode, and that a family of a
// section that zuzu does not read stays an issue when it names a weight or
// a style (D32, D58).
func TestChartFontFamilyIssues(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		"---\nconfig:\n  fontFamily: Helvetica-Bold\n---\npie\n\"a\": 1\n",
		"---\nconfig:\n  fontFamily: Times-Italic\n---\ntimeline\n2002 : LinkedIn\n",
		"---\nconfig:\n  journey:\n    taskFontFamily: courier\n---\njourney\nsection A\n  Task: 5: Me\n",
	} {
		if report, err := chartOf(src, true); err != nil || len(report) != 0 {
			t.Errorf("%q: the report %v and the error %v, want none", src, report, err)
		}
	}
	for _, src := range []string{
		"---\nconfig:\n  journey:\n    taskFontFamily: Times-Bold\n---\njourney\nsection A\n  Task: 5: Me\n",
		"---\nconfig:\n  fontFamily: 12\n---\npie\n\"a\": 1\n",
	} {
		report, err := chartOf(src, false)
		if err != nil || len(report) != 1 || !strings.Contains(report[0].Message, "ontFamily") {
			t.Errorf("%q: the report %v and the error %v, want one issue of the family", src, report, err)
		}
	}
}
