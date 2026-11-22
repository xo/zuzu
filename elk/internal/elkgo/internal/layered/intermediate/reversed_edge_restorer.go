// Copied from github.com/d2lang/elk-go, file internal/layered/intermediate/reversed_edge_restorer.go, at commit
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
	basegraph "github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

// RestoreReversedEdges restores every cycle-breaking reversal. Iteration uses
// a copy of each outgoing-edge list because ReverseEdge mutates adjacency.
func RestoreReversedEdges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, portID := range arena.Node(nodeID).Ports {
				edges := append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...)
				for _, edgeID := range edges {
					if basegraph.Get(&arena.Edge(edgeID).Properties, layered.Reversed) {
						arena.ReverseEdge(graphID, edgeID, false)
					}
				}
			}
		}
	}
}
