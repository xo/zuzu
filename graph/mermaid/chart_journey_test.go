package mermaid_test

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph/mermaid"
	"github.com/xo/zuzu/scene"
)

// journeyTaskLine matches a task of a user journey: its name, its score
// and its actors.
var journeyTaskLine = regexp.MustCompile(`^\s*([^:]+?)\s*:\s*([0-9]+)\s*(?::\s*(.*))?$`)

// journeySection matches a section of a user journey.
var journeySection = regexp.MustCompile(`^\s*section\s+(.+?)\s*$`)

func TestJourneyFixtures(t *testing.T) {
	t.Parallel()
	for name, src := range mmdFiles(t, "journey") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			items := renderMermaid(t, src)
			n := 0
			for l := range strings.SplitSeq(src, "\n") {
				if m := journeySection.FindStringSubmatch(l); m != nil {
					if c := textCount(items, m[1]); c != 1 {
						t.Errorf("the section %q draws %d times", m[1], c)
					}
					continue
				}
				m := journeyTaskLine.FindStringSubmatch(l)
				if m == nil || strings.HasPrefix(strings.TrimSpace(l), "title") {
					continue
				}
				n++
				score, _ := strconv.ParseFloat(m[2], 64)
				if c := itemCount(items, scene.Symbol, "task", func(d map[string]any) bool {
					return d["label"] == m[1] && d["score"] == score
				}); c != 1 {
					t.Errorf("the task %s draws %d times", m[1], c)
				}
				if c := textCount(items, m[1]); c != 1 {
					t.Errorf("the label %q draws %d times", m[1], c)
				}
				if m[3] == "" {
					continue
				}
				var actors []string
				for a := range strings.SplitSeq(m[3], ",") {
					actors = append(actors, strings.TrimSpace(a))
				}
				if textCount(items, strings.Join(actors, ", ")) == 0 {
					t.Errorf("the actors %q of the task %s do not draw", m[3], m[1])
				}
			}
			if c := itemCount(items, scene.Symbol, "task", func(map[string]any) bool { return true }); c != n || n == 0 {
				t.Errorf("expected %d tasks, got %d", n, c)
			}
		})
	}
}

func TestJourneyErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		"journey",
		"journey\nsection A",
		"journey\nTask with no score",
		"journey\nTask: high: Me",
		"journey\nTask: 6: Me",
		"journey\nTask: 0",
		"journey\n: 3: Me",
		"journey LR\nTask: 3",
	}
	for _, src := range tests {
		if _, err := compileMermaid(t, src, false); !errors.Is(err, mermaid.ErrSyntax) {
			t.Errorf("%q: expected a syntax error, got %v", src, err)
		}
	}
}

func TestJourneyIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		src, part string
	}{
		{"---\nconfig:\n  journey:\n    taskFontSize: 20\n---\njourney\nTask: 3: Me", "journey.taskFontSize"},
	}
	for _, test := range tests {
		report, err := compileMermaid(t, test.src, false)
		if err != nil {
			t.Errorf("%q: %v", test.src, err)
			continue
		}
		if !reportHolds(report, test.part) {
			t.Errorf("%q: expected an issue of %q, got %v", test.src, test.part, report)
		}
		if _, err := compileMermaid(t, test.src, true); err == nil {
			t.Errorf("%q: expected an error in the strict mode", test.src)
		}
	}
}
