// Copied from github.com/d2lang/elk-go, file internal/layered/p2layers/coffman_graham_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"testing"

	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

func TestCoffmanGrahamBreaksSourceTiesByNodeID(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	arena.Node(first).ID = 3
	arena.Node(second).ID = 7
	layerer := &CoffmanGrahamLayerer{
		inTopologicalOrder: make(map[layered.LNodeID][]int),
	}

	if got := layerer.compareNodesInTopologicalOrder(arena, first, second); got != -1 {
		t.Fatalf("compare(first, second) = %d, want -1", got)
	}
	if got := layerer.compareNodesInTopologicalOrder(arena, second, first); got != 1 {
		t.Fatalf("compare(second, first) = %d, want 1", got)
	}
	if got := layerer.compareNodesInTopologicalOrder(arena, first, first); got != 0 {
		t.Fatalf("compare(first, first) = %d, want 0", got)
	}
}
