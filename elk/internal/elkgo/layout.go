// Copied from github.com/d2lang/elk-go, file layout.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

// zuzu: change 4. The imports of bytes, encoding/json, io and graphjson
// served LayoutJSON only.
import (
	"fmt"

	"github.com/xo/zuzu/elk/internal/elkgo/internal/builtins"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/core"
)

// Layout computes layout in place. Like elkjs, it preserves the input graph's
// element and slice order and adds computed shape and edge-route fields.
func Layout(input *Graph) error {
	options, algorithms := defaultRegistries()
	codec := core.NewCodec(options)
	model, err := codec.Import(input)
	if err != nil {
		return fmt.Errorf("import ELK graph: %w", err)
	}
	if err := core.NewEngine(options, algorithms).Layout(model); err != nil {
		return fmt.Errorf("layout ELK graph: %w", err)
	}
	if err := codec.TransferLayout(model); err != nil {
		return fmt.Errorf("export ELK layout: %w", err)
	}
	return nil
}

// zuzu: change 4. LayoutJSON, which read and wrote the JSON form of a graph
// with encoding/json, is left out. zuzu builds the graph in Go and calls
// Layout, and it reads JSON only with encoding/json/v2 (D29).

// defaultRegistries is the single integration point for native providers.
// Algorithm packages register D2's eleven ELK.js 0.12.0 provider IDs and the
// official option metadata here. The executable compatibility contract is
// D2's documented profile, not every expert option combination.
func defaultRegistries() (*core.OptionRegistry, *core.AlgorithmRegistry) {
	options := core.NewOptionRegistry()
	builtins.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := builtins.Register(algorithms); err != nil {
		panic(err)
	}
	return options, algorithms
}
