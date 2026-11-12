package mermaid

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// swimlaneOf reads a source in the strict and the loose mode, and returns
// the graph of the loose mode and the error of the strict mode.
func swimlaneOf(t *testing.T, src string) (*graph.Graph, error) {
	t.Helper()
	d, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, _, serr := Graph(d, true)
	g, _, err := Graph(d, false)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return g, serr
}

func TestSwimlaneLanes(t *testing.T) {
	t.Parallel()
	g, err := swimlaneOf(t, `swimlane-beta LR
  start[Start] --> a
  subgraph Owned [Owned lane]
    a[Owned node]
    subgraph Inner
      b
    end
  end
  subgraph Other
    c
  end
  b --> c --> done`)
	if err != nil {
		t.Fatalf("expected no issue, got %v", err)
	}
	if !g.Lanes || g.LaneEdges || g.Direction != graph.LR {
		t.Fatalf("expected lanes from left to right, got %v %v %s", g.Lanes, g.LaneEdges, g.Direction)
	}
	var ids []string
	for _, c := range g.Clusters {
		ids = append(ids, c.ID+":"+c.Label)
	}
	if got := strings.Join(ids, " "); got != "__swimlane_default__: Owned:Owned lane Inner:Inner Other:Other" {
		t.Errorf("expected the lane of the loose nodes first, with no title, got %q", got)
	}
	for id, c := range map[string]string{"start": defaultLane, "done": defaultLane, "a": "Owned", "b": "Inner", "c": "Other"} {
		if n := node(t, g, id); n.Cluster != c {
			t.Errorf("node %s: expected the cluster %s, got %s", id, c, n.Cluster)
		}
	}
	// With no loose node, there is no lane of its own.
	g, _ = swimlaneOf(t, "swimlane-beta\n  subgraph A\n    a\n  end")
	if len(g.Clusters) != 1 || g.Direction != graph.TB {
		t.Errorf("expected one lane from top to bottom, got %v %s", g.Clusters, g.Direction)
	}
}

func TestSwimlaneLayout(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		src   string
		lanes bool
		issue string
	}{
		{"swimlane-beta\n a", true, ""},
		{"---\nconfig:\n  layout: swimlane\n---\nswimlane-beta\n a", true, ""},
		{"---\nconfig:\n  layout: dagre\n---\nswimlane-beta\n a", false, ""},
		{"---\nconfig:\n  layout: elk\n---\nswimlane-beta\n a", false, ""},
		{"---\nconfig:\n  layout: swimlane\n  swimlane:\n    layout: dagre\n---\nswimlane-beta\n a", false, ""},
		{"---\nconfig:\n  swimlane:\n    layout: dagre\n---\nswimlane-beta\n a", false, ""},
		{"---\nconfig:\n  layout: tidy-tree\n---\nswimlane-beta\n a", true, "the layout tidy-tree"},
		{"---\nconfig:\n  layout: swimlane\n---\nflowchart LR\n a", true, ""},
		{"---\nconfig:\n  layout: swimlane\n---\ngraph TD\n a", true, ""},
		{"flowchart LR\n a", false, ""},
		{"---\nconfig:\n  layout: swimlane\n---\nstateDiagram-v2\n a", false, "the layout swimlane"},
	} {
		d, err := Read(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, false)
		if err != nil {
			t.Fatalf("%q: %v", tt.src, err)
		}
		if g.Lanes != tt.lanes {
			t.Errorf("%q: expected lanes %v, got %v", tt.src, tt.lanes, g.Lanes)
		}
		got := fmt.Sprint(report)
		if tt.issue == "" && len(report) > 0 || tt.issue != "" && !strings.Contains(got, tt.issue) {
			t.Errorf("%q: expected the issue %q, got %v", tt.src, tt.issue, report)
		}
	}
}

func TestSwimlaneSettings(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		settings  string
		issue     string
		laneEdges bool
	}{
		{"ignoreCrossLaneEdges: true\n    optimizeRanksByCrossings: true", "", false},
		{"ignoreCrossLaneEdges: false\n    optimizeRanksByCrossings: false", "", true},
		{"ignoreCrossLaneEdges: false", "", true},
		{"ignoreCrossLaneEdges: false\n    optimizeRanksByCrossings: true", "swimlane.optimizeRanksByCrossings", true},
		{"ignoreCrossLaneEdges: 1", "swimlane.ignoreCrossLaneEdges", false},
		{"automaticLaneOrdering: false", "", false},
		{"automaticLaneOrdering: true", "swimlane.automaticLaneOrdering", false},
		{"lineHops: false", "", false},
		{"lineHops: arc", "swimlane.lineHops", false},
		{"look: neo", "", false},
		{"useMaxWidth: false", "", false},
		{"theme: redux-color", "swimlane.theme", false},
		{"curve: linear", "swimlane.curve", false},
	} {
		src := "---\nconfig:\n  swimlane:\n    " + tt.settings + "\n---\nswimlane-beta\n  subgraph A\n    a --> b\n  end"
		d, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		g, report, err := Graph(d, false)
		if err != nil {
			t.Fatalf("%s: %v", tt.settings, err)
		}
		got := fmt.Sprint(report)
		if tt.issue == "" && len(report) > 0 || tt.issue != "" && !strings.Contains(got, tt.issue) {
			t.Errorf("%s: expected the issue %q, got %v", tt.settings, tt.issue, report)
		}
		if g.LaneEdges != tt.laneEdges {
			t.Errorf("%s: expected LaneEdges %v, got %v", tt.settings, tt.laneEdges, g.LaneEdges)
		}
	}
	// A swimlane diagram reads the section flowchart, as Mermaid does.
	g, err := swimlaneOf(t, "---\nconfig:\n  flowchart:\n    nodeSpacing: 100\n    curve: linear\n---\nswimlane-beta\n a --> b")
	if err != nil || g.NodeSep != 48 || g.Edges[0].Curve != "linear" {
		t.Errorf("expected the spaces and the curve of the section flowchart, got %v %v %q", err, g.NodeSep, g.Edges[0].Curve)
	}
	// A swimlane diagram that does not lay out in lanes reads no setting of
	// the lanes.
	if _, err := swimlaneOf(t, "---\nconfig:\n  layout: dagre\n  swimlane:\n    ignoreCrossLaneEdges: false\n---\nswimlane-beta\n a"); err == nil {
		t.Error("expected an issue for a setting of the lanes with the layout dagre")
	}
}

// TestSwimlaneErrors checks that a swimlane diagram fails where mmdc fails,
// and draws where mmdc draws.
func TestSwimlaneErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		src string
		bad bool
	}{
		{"swimlane-beta XY\n  subgraph A\n    a --> b\n  end", true},
		{"swimlane-beta LR\n  subgraph A\n    a --> b", true},
		{"swimlane-beta TB\n  a --> b\n  end", true},
		{"swimlane-beta TD\n  a --> b", false},
		{"swimlane-beta", false},
		{"swimlane-beta LR; a --> b\n  subgraph S; c; end\n  b --> c", false},
		{"swimlane-beta LR\n  subgraph A\n    a\n  end\n  subgraph Empty\n  end\n  a --> a", false},
	} {
		d, err := Read(tt.src)
		if err == nil {
			_, _, err = Graph(d, true)
		}
		if bad := errors.Is(err, ErrSyntax); bad != tt.bad || !bad && err != nil {
			t.Errorf("%q: expected an error of syntax %v, got %v", tt.src, tt.bad, err)
		}
	}
}
