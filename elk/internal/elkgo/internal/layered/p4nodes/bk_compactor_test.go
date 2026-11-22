// Copied from github.com/d2lang/elk-go, file internal/layered/p4nodes/bk_compactor_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import "testing"

func TestBKCompactorPlacesTwoIndependentBlocksWithSpacing(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(1)
	upper := addPlacementNode(arena, graphID, layers[0], 10)
	lower := addPlacementNode(arena, graphID, layers[0], 10)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, info.NodeCount, VerticalDown, HorizontalRight)
	aligner := NewBKAligner(arena, graphID, info)
	aligner.VerticalAlignment(layout, nil)
	aligner.InsideBlockShift(layout)
	NewBKCompactor(arena, graphID).HorizontalCompaction(arena, graphID, info, layout)
	upperY := requireFloat(layout.Y[arena.Node(upper).ID], "y")
	lowerY := requireFloat(layout.Y[arena.Node(lower).ID], "y")
	if lowerY-upperY != 30 { // 10 high + default 20 node-node spacing
		t.Fatalf("positions = %v, %v; want delta 30", upperY, lowerY)
	}
}
