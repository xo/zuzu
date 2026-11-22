// Copied from github.com/d2lang/elk-go, file internal/layered/p4nodes/threshold_strategy_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"testing"
)

func TestNullThresholdStrategyUsesDirectionInfinity(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(1)
	nodeID := addPlacementNode(arena, graphID, layers[0], 10)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, info.NodeCount, VerticalDown, HorizontalLeft)
	layout.Align[arena.Node(nodeID).ID] = nodePointer(nodeID)
	strategy := NewThresholdStrategy(false)
	if got := strategy.CalculateThreshold(arena, layout, 42, nodeID, nodeID); !math.IsInf(got, -1) {
		t.Fatalf("down threshold = %v", got)
	}
	layout.VDirection = VerticalUp
	if got := strategy.CalculateThreshold(arena, layout, 42, nodeID, nodeID); !math.IsInf(got, 1) {
		t.Fatalf("up threshold = %v", got)
	}
}
