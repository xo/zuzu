// Copied from github.com/d2lang/elk-go, file internal/layered/p3order/layer_sweep_type_decider_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

func TestLayerSweepTypeDeciderAlwaysProcessesRootBottomUp(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	nodeID := addCountingNode(arena, graphID, layerID, layered.EastSide)
	order := [][]layered.LNodeID{{nodeID}}
	decider := NewLayerSweepTypeDecider(1)
	decider.InitAtLayerLevel(arena, 0, order)
	decider.InitAtNodeLevel(arena, 0, 0, order)
	if !decider.UseBottomUp(arena, graphID, nil, false, order) {
		t.Fatal("root graph must be a bottom-up sweep root")
	}
}
