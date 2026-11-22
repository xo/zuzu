// Copied from github.com/d2lang/elk-go, file internal/layered/p3order/cross_min_util_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"reflect"
	"testing"

	"github.com/xo/zuzu/elk/internal/elkgo/internal/layered"
)

func TestPortsInNorthSouthEastWestOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	for _, side := range []layered.PortSide{
		layered.NorthSide, layered.NorthSide,
		layered.EastSide, layered.EastSide,
		layered.SouthSide, layered.SouthSide,
		layered.WestSide, layered.WestSide,
	} {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
	}

	checks := []struct {
		side layered.PortSide
		want []layered.LPortID
	}{
		{layered.NorthSide, []layered.LPortID{0, 1}},
		{layered.EastSide, []layered.LPortID{2, 3}},
		{layered.SouthSide, []layered.LPortID{5, 4}},
		{layered.WestSide, []layered.LPortID{7, 6}},
		{layered.UndefinedSide, nil},
	}
	for _, check := range checks {
		if got := portsInNorthSouthEastWestOrder(arena, nodeID, check.side); !reflect.DeepEqual(got, check.want) {
			t.Errorf("side %s: got %v, want %v", check.side, got, check.want)
		}
	}
}
