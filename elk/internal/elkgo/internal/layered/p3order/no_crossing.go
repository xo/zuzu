// Copied from github.com/d2lang/elk-go, file internal/layered/p3order/no_crossing.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2022 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/xo/zuzu/elk/internal/elkgo/internal/layered"

// NoCrossingMinimizer deliberately preserves the node and port order produced
// by layering and the required long-edge preprocessing.
type NoCrossingMinimizer struct{}

func (*NoCrossingMinimizer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseNodeOrdering, layered.LongEdgeSplitter).
		AddBefore(layered.PhaseNodeOrdering, layered.PortListSorter).
		AddBefore(layered.PhaseNodePlacement, layered.InLayerConstraintProcessor).
		AddAfter(layered.PhaseEdgeRouting, layered.LongEdgeJoiner)
	return configuration
}

func (*NoCrossingMinimizer) Process(*layered.LGraphArena, layered.LGraphID) error { return nil }
