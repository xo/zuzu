// Copied from github.com/d2lang/elk-go, file internal/core/codec_importer_errors_test.go, at commit
// 4e0a9977ed8b (D39, docs/UPSTREAM.md). The SPDX line below names its license,
// and elk/internal/elkgo holds the license texts and the notices. Its
// import paths name the copy (change 2).

// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"testing"

	"github.com/xo/zuzu/elk/internal/elkgo/graphjson"
)

func TestRejectUnconstructableJSONProperties(t *testing.T) {
	const enforcedGroupsID = "org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders"
	registry := NewOptionRegistry()
	if err := registry.Register(OptionData{ID: enforcedGroupsID, Kind: ObjectOption, Targets: Parents}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, id string
		value    any
		want     string
	}{
		{
			name:  "topdown size approximator",
			id:    TopdownSizeApproximator.ID,
			value: "COUNT_CHILDREN",
			want:  "Couldn't create new instance of property 'org.eclipse.elk.topdown.sizeApproximator'. Make sure its type is registered with the ElkReflect utility class.",
		},
		{
			name:  "crossing-minimization enforced groups",
			id:    enforcedGroupsID,
			value: []any{1, 2},
			want:  "Severe implementation error in the Json to ElkGraph importer.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCodec(registry).Import(&graphjson.Graph{
				ID:            graphjson.StringID("root"),
				LayoutOptions: graphjson.LayoutOptions{test.id: test.value},
			})
			if err == nil {
				t.Fatalf("Import succeeded, want %q", test.want)
			}
			if err.Error() != test.want {
				t.Fatalf("Import error = %q, want %q", err, test.want)
			}
		})
	}
}
