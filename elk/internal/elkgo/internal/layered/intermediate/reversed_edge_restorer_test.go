// Copied from github.com/d2lang/elk-go, file internal/layered/intermediate/reversed_edge_restorer_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

func TestRestoreReversedEdges(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	a, b := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.SetNodeLayer(a, &layerID)
	arena.SetNodeLayer(b, &layerID)
	ap, bp := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(ap, &a)
	arena.SetPortNode(bp, &b)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &ap)
	arena.SetEdgeTarget(edgeID, &bp)
	arena.ReverseEdge(graphID, edgeID, false)

	RestoreReversedEdges(arena, graphID)

	edge := arena.Edge(edgeID)
	if edge.Source == nil || *edge.Source != ap || edge.Target == nil || *edge.Target != bp {
		t.Fatalf("restored endpoints = %v -> %v, want %v -> %v", edge.Source, edge.Target, ap, bp)
	}
	if basegraph.Get(&edge.Properties, layered.Reversed) {
		t.Fatal("REVERSED remains true")
	}
}
