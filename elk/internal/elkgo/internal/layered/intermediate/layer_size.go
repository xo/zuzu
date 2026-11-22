// Copied from github.com/d2lang/elk-go, file internal/layered/intermediate/layer_size.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	basegraph "github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

// CalculateLayerSizesAndGraphHeight sets every layer's bounding size and the
// graph's vertical size and offset after node placement.
func CalculateLayerSizesAndGraphHeight(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	minY, maxY := math.Inf(1), math.Inf(-1)
	foundNodes := false
	for _, layerID := range graph.Layers {
		layer := arena.Layer(layerID)
		layer.Size.X, layer.Size.Y = 0, 0
		if len(layer.Nodes) == 0 {
			continue
		}
		foundNodes = true
		for _, nodeID := range layer.Nodes {
			node := arena.Node(nodeID)
			layer.Size.X = math.Max(layer.Size.X, node.Size.X+node.Margin.Left+node.Margin.Right)
		}
		first := arena.Node(layer.Nodes[0])
		top := first.Position.Y - first.Margin.Top
		if first.Type == layered.ExternalPortNode {
			top -= basegraph.Get(&graph.Properties, layered.SpacingPortsSurrounding).Top
		}
		last := arena.Node(layer.Nodes[len(layer.Nodes)-1])
		bottom := last.Position.Y + last.Size.Y + last.Margin.Bottom
		if last.Type == layered.ExternalPortNode {
			bottom += basegraph.Get(&graph.Properties, layered.SpacingPortsSurrounding).Bottom
		}
		layer.Size.Y = bottom - top
		minY = math.Min(minY, top)
		maxY = math.Max(maxY, bottom)
	}
	if !foundNodes {
		minY, maxY = 0, 0
	}
	graph.Size.Y = maxY - minY
	graph.Offset.Y -= minY
}
