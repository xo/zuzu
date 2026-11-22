// Copied from github.com/d2lang/elk-go, file internal/layered/intermediate/selfloop_preprocessor.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2018, 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered/intermediate/loops"
)

// PreprocessSelfLoops installs ELK's temporary self-loop model and hides
// self-loop edges and freely orderable pure self-loop ports from phases 1-3.
func PreprocessSelfLoops(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if !loops.NeedsProcessing(arena, nodeID) {
			continue
		}
		holder, err := loops.Install(arena, nodeID)
		if err != nil {
			return err
		}
		hideSelfLoopEdges(arena, holder)
		hideSelfLoopPorts(arena, holder)
	}
	return nil
}

func hideSelfLoopEdges(arena *layered.LGraphArena, holder *loops.Holder) {
	for _, loop := range holder.HyperLoops {
		for _, edge := range loop.Edges {
			arena.SetEdgeSource(edge.LEdge, nil)
			arena.SetEdgeTarget(edge.LEdge, nil)
		}
	}
}

func hideSelfLoopPorts(arena *layered.LGraphArena, holder *loops.Holder) {
	node := arena.Node(holder.LNode)
	orderFixed := basegraph.Get(&node.Properties, layered.PortConstraintsOption).IsOrderFixed()
	hierarchyMode := false
	if node.NestedGraph != nil {
		properties := basegraph.Get(&arena.Graph(*node.NestedGraph).Properties, layered.GraphProperties)
		hierarchyMode = properties&layered.GraphPropertyExternalPorts != 0
	}
	if orderFixed || hierarchyMode {
		return
	}

	for _, portID := range holder.PortOrder {
		port := holder.Ports[portID]
		if !port.HadOnlySelfLoops {
			continue
		}
		arena.SetPortNode(portID, nil)
		port.Hidden = true
		holder.PortsHidden = true
	}
}
