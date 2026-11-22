// Copied from github.com/d2lang/elk-go, file internal/common/nodespacing/options.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2009, 2020 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2012, 2020 Kiel University and others.
// Copyright (c) 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package nodespacing

import (
	"github.com/xo/zuzu/elk/internal/elkgo/internal/geometry"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
)

// These properties retain the exact CoreOptions identifiers used by ELK
// v0.8.1. They live here until the generated option metadata is ported.
var (
	Margins = graph.CloneablePropertyWithDefault("org.eclipse.elk.margins", func() geometry.Spacing {
		return geometry.Spacing{}
	})
	SpacingLabelNode = graph.PropertyWithDefault("org.eclipse.elk.spacing.labelNode", func() float64 { return 5 })
	SpacingPortPort  = graph.PropertyWithDefault("org.eclipse.elk.spacing.portPort", func() float64 { return 10 })

	NodeLabelsPlacement = graph.CloneablePropertyWithDefault("org.eclipse.elk.nodeLabels.placement", func() []string { return nil })
	PortLabelsPlacement = graph.CloneablePropertyWithDefault("org.eclipse.elk.portLabels.placement", func() []string {
		return []string{"OUTSIDE"}
	})
	EdgeLabelsPlacement = graph.PropertyWithDefault("org.eclipse.elk.edgeLabels.placement", func() string { return "CENTER" })
	NodeSizeOptions     = graph.CloneablePropertyWithDefault("org.eclipse.elk.nodeSize.options", func() []string {
		return []string{"DEFAULT_MINIMUM_SIZE"}
	})
	PortBorderOffset = graph.PropertyWithDefault("org.eclipse.elk.port.borderOffset", func() float64 { return 0 })
	PortAlignment    = graph.PropertyWithDefault("org.eclipse.elk.portAlignment.default", func() string { return "DISTRIBUTED" })
)

func has(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
