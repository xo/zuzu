// Copied from github.com/d2lang/elk-go, file internal/rectpacking/options.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package rectpacking

import (
	"github.com/xo/zuzu/elk/internal/elkgo/internal/basicoptions"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/core"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/geometry"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/graph"
)

var (
	AspectRatio = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.3 })
	Padding     = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(15)
	})
	SpacingNodeNode     = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 15 })
	Interactive         = graph.PropertyWithDefault("org.eclipse.elk.interactive", func() bool { return false })
	OmitNodeMicroLayout = graph.PropertyWithDefault("org.eclipse.elk.omitNodeMicroLayout", func() bool { return false })

	OptimizationGoal              = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.widthApproximation.optimizationGoal", func() string { return "MAX_SCALE_DRIVEN" })
	LastPlaceShift                = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.widthApproximation.lastPlaceShift", func() bool { return true })
	WidthApproximationStrategy    = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.widthApproximation.strategy", func() string { return "GREEDY" })
	PackingStrategy               = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.packing.strategy", func() string { return "COMPACTION" })
	RowHeightReevaluation         = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.packing.compaction.rowHeightReevaluation", func() bool { return false })
	CompactionIterations          = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.packing.compaction.iterations", func() int { return 1 })
	WhiteSpaceEliminationStrategy = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.whiteSpaceElimination.strategy", func() string { return "NONE" })
	CurrentPosition               = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.currentPosition", func() int { return -1 })
	DesiredPosition               = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.desiredPosition", func() int { return -1 })
	InNewRow                      = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.inNewRow", func() bool { return false })
	TryBox                        = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.trybox", func() bool { return false })
	OrderBySize                   = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.orderBySize", func() bool { return false })
	TargetWidth                   = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.widthApproximation.targetWidth", func() float64 { return -1 })
)

// RegisterOptions registers the metadata declared by RectPacking.melk in ELK
// 0.12.0. Shared core identifiers are deliberately registered idempotently.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Padding.ID, Kind: core.ObjectOption, Targets: core.Parents, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: SpacingNodeNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Interactive.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OmitNodeMicroLayout.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OptimizationGoal.ID, Group: "widthApproximation", Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"MAX_SCALE_DRIVEN", "ASPECT_RATIO_DRIVEN", "AREA_DRIVEN"}},
		{ID: LastPlaceShift.ID, Group: "widthApproximation", Kind: core.BooleanOption, Targets: core.Parents},
		{ID: WidthApproximationStrategy.ID, Group: "widthApproximation", Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"GREEDY", "TARGET_WIDTH"}},
		{ID: PackingStrategy.ID, Group: "packing", Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"COMPACTION", "SIMPLE", "NONE"}},
		{ID: RowHeightReevaluation.ID, Group: "packing.compaction", Kind: core.BooleanOption, Targets: core.Parents},
		{ID: CompactionIterations.ID, Group: "packing.compaction", Kind: core.IntegerOption, Targets: core.Parents},
		{ID: WhiteSpaceEliminationStrategy.ID, Group: "whiteSpaceElimination", Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"EQUAL_BETWEEN_STRUCTURES", "TO_ASPECT_RATIO", "NONE"}},
		{ID: CurrentPosition.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: DesiredPosition.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: InNewRow.ID, Kind: core.BooleanOption, Targets: core.Nodes},
		{ID: TryBox.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OrderBySize.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: TargetWidth.ID, Group: "widthApproximation", Kind: core.DoubleOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) == nil {
			if err := registry.Register(option); err != nil {
				panic(err)
			}
		}
	}
	// The node-size option is shared with the utility providers and is needed
	// by effectiveMinSizeConstraintFor.
	basicoptions.RegisterOptions(registry)
}
