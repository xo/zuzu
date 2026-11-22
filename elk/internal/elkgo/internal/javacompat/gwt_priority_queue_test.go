// Copied from github.com/d2lang/elk-go, file internal/javacompat/gwt_priority_queue_test.go, at commit
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

package javacompat

import (
	"reflect"
	"testing"
)

func TestGWTPriorityQueueBulkHeapAndEqualSiftOrder(t *testing.T) {
	type item struct {
		priority int
		name     string
	}
	queue := NewGWTPriorityQueue(func(first, second item) int {
		return intComparator(first.priority, second.priority)
	})
	queue.AddAll([]item{{3, "a"}, {1, "b"}, {1, "c"}, {2, "d"}, {1, "e"}})
	var got []string
	for !queue.IsEmpty() {
		value, _ := queue.Poll()
		got = append(got, value.name)
	}
	// Frozen from GWT 2.11.0's recursive makeHeap and strict mergeHeaps
	// comparison. The vector is unchanged from GWT 2.9.0; OpenJDK's
	// equal-element order is different.
	want := []string{"e", "b", "c", "d", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("poll order = %v, want %v", got, want)
	}
}
