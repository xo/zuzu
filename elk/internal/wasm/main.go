// SPDX-License-Identifier: EPL-2.0

// Command wasm lays out one small graph with the package elk, so that CI can
// build the module with TinyGo for WebAssembly (D29). TinyGo builds a
// program and not a library, so this program exists to be built and run.
package main

import (
	"fmt"
	"os"

	"github.com/xo/zuzu/elk"
	"github.com/xo/zuzu/graph"
)

func main() {
	in := graph.LayoutInput{
		Nodes: []graph.LayoutNode{
			{ID: "a", Width: 80, Height: 40},
			{ID: "b", Cluster: "c", Width: 60, Height: 40},
		},
		Clusters: []graph.LayoutCluster{{ID: "c", TitleWidth: 10, TitleHeight: 20}},
		Edges:    []graph.LayoutEdge{{Source: "a", Target: "b", LabelWidth: 20, LabelHeight: 20}},
	}
	l, err := elk.Layout().Lay(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wasm:", err)
		os.Exit(1)
	}
	for i, p := range l.Nodes {
		fmt.Printf("%s %g %g %g %g\n", in.Nodes[i].ID, p.X, p.Y, p.Width, p.Height)
	}
	for i, p := range l.Clusters {
		fmt.Printf("%s %g %g %g %g\n", in.Clusters[i].ID, p.X, p.Y, p.Width, p.Height)
	}
}
