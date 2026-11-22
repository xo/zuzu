// Copied from github.com/d2lang/elk-go, file internal/basic/register.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package basic registers ELK's three core utility layout providers.
package basic

import (
	"github.com/xo/zuzu/elk/internal/elkgo/internal/basicoptions"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/box"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/core"
	"github.com/xo/zuzu/elk/internal/elkgo/internal/fixed"
	randomlayout "github.com/xo/zuzu/elk/internal/elkgo/internal/random"
)

func RegisterOptions(registry *core.OptionRegistry) {
	basicoptions.RegisterOptions(registry)
}

func Register(registry *core.AlgorithmRegistry) error {
	for _, register := range []func(*core.AlgorithmRegistry) error{
		fixed.Register,
		box.Register,
		randomlayout.Register,
	} {
		if err := register(registry); err != nil {
			return err
		}
	}
	return nil
}
