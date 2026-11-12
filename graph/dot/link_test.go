package dot

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/zuzu/vega"
)

// TestLinks reads URL, href and tooltip on the graph, a cluster, a node and
// an edge, with their escapes, and reports a link with a scheme that zuzu
// does not write and a target.
func TestLinks(t *testing.T) {
	t.Parallel()
	src := `digraph G {
	URL="https://example.com/\G"; tooltip="the graph"
	subgraph cluster_a { label="A"; href="#\G"; tooltip="cluster \G"; a }
	a [URL="https://example.com/a" href="/nodes/\N" tooltip="node \N\nline two"]
	b [URL="javascript:alert(1)" target="_blank"]
	a -> b [URL="https://example.com/\T-\H" tooltip="\E"]
}`
	g, report, err := Parse(src, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Href != "https://example.com/G" || g.Tooltip != "the graph" {
		t.Errorf("graph: got %q and %q", g.Href, g.Tooltip)
	}
	if c := g.Clusters[0]; c.Href != "#cluster_a" || c.Tooltip != "cluster cluster_a" {
		t.Errorf("cluster: got %q and %q", c.Href, c.Tooltip)
	}
	nodes := map[string][2]string{}
	for _, n := range g.Nodes {
		nodes[n.ID] = [2]string{n.Href, n.Tooltip}
	}
	if got := nodes["a"]; got != [2]string{"/nodes/a", "node a\nline two"} {
		t.Errorf("node a: got %q", got)
	}
	if got := nodes["b"]; got != [2]string{} {
		t.Errorf("node b: got %q, want no link", got)
	}
	if e := g.Edges[0]; e.Href != "https://example.com/a-b" || e.Tooltip != "a->b" {
		t.Errorf("edge: got %q and %q", e.Href, e.Tooltip)
	}
	var msgs []string
	for _, is := range report {
		msgs = append(msgs, is.Error())
	}
	all := strings.Join(msgs, "\n")
	if len(report) != 2 || !strings.Contains(all, "javascript:alert(1)") || !strings.Contains(all, "the attribute target") {
		t.Errorf("report = %v, want the link of b and its target", report)
	}
	if _, _, err := Parse(src, true); !errors.Is(err, vega.ErrUnsupported) {
		t.Errorf("strict: got %v, want %v", err, vega.ErrUnsupported)
	}
}
