// Copied from github.com/d2lang/elk-go, file internal/layered/intermediate/selfloop_router.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"

	basegraph "github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered/intermediate/loops"
	looprouting "github.com/xo/zuzu/elk/internal/elkgo/internal/layered/intermediate/loops/routing"
)

// RouteSelfLoops runs ELK's routing-director, label-placement, slot-assignment,
// and orthogonal routing chain for every normal self-loop node.
func RouteSelfLoops(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	routing := basegraph.Get(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption)
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.NormalNode {
				continue
			}
			holder, found := basegraph.TryGet(&node.Properties, loops.HolderProperty)
			if !found {
				continue
			}
			if err := looprouting.DetermineLoopRoutes(arena, holder); err != nil {
				return err
			}
			if err := looprouting.PlaceLabels(arena, holder); err != nil {
				return err
			}
			looprouting.AssignRoutingSlots(arena, holder)
			switch routing {
			case layered.EdgeRoutingOrthogonal, layered.EdgeRoutingUndefined:
				looprouting.RouteOrthogonal(arena, holder)
			case layered.EdgeRoutingPolyline:
				looprouting.RoutePolyline(arena, holder)
			case layered.EdgeRoutingSplines:
				looprouting.RouteSpline(arena, holder)
			default:
				return fmt.Errorf("self-loop router: routing mode %d is not implemented", routing)
			}
		}
	}
	return nil
}
