// Copied from github.com/d2lang/elk-go, file internal/builtins/register.go, at commit
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

// Package builtins registers the layout providers compiled into ELK.js 0.12.0.
package builtins

import (
	"github.com/xo/zuzu/elk/internal/elkgo/internal/basic"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/core"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/force"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/layeredlayout"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/mrtree"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/radial"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/rectpacking"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/spore"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/stress"
)

// RegisterOptions installs the option metadata needed by the native provider
// set. Individual registration functions are idempotent for shared core IDs.
func RegisterOptions(registry *core.OptionRegistry) {
	basic.RegisterOptions(registry)
	layered.RegisterOptions(registry)
	force.RegisterOptions(registry)
	stress.RegisterOptions(registry)
	mrtree.RegisterOptions(registry)
	radial.RegisterOptions(registry)
	spore.RegisterOptions(registry)
	rectpacking.RegisterOptions(registry)
	registerCompatibilityOptions(registry)
}

// Register installs algorithms in the same relative order as the official
// ELK.js 0.12.0 registry. That order is observable during suffix resolution.
func Register(registry *core.AlgorithmRegistry) error {
	for _, register := range []func(*core.AlgorithmRegistry) error{
		basic.Register,
		layeredlayout.Register,
		stress.Register,
		mrtree.Register,
		radial.Register,
		force.Register,
		spore.Register,
		rectpacking.Register,
	} {
		if err := register(registry); err != nil {
			return err
		}
	}
	return nil
}
