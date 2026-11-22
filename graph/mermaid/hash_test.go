package mermaid

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xo/zuzu/graph"
)

// TestHashSlots checks that a name takes the place of its FNV-1a hash, the
// next free place when an earlier name took it, never the place to avoid,
// and the place of its hash when no place is free (D56). Modulo 10, the
// hash of Animal and Fish is 3, of Duck 4 and of Zebra 7.
func TestHashSlots(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		ids   []string
		n     int
		avoid map[string]int
		want  string
	}{
		{[]string{"Animal", "Duck", "Fish", "Zebra"}, 10, nil, "Animal:3 Duck:4 Fish:5 Zebra:7"},
		// An id that comes again keeps its place.
		{[]string{"Fish", "Fish", "Animal"}, 10, nil, "Fish:3 Fish:3 Animal:4"},
		{[]string{"Animal"}, 10, map[string]int{"Animal": 3}, "Animal:4"},
		// With one place, every id takes it, and the place to avoid is
		// no place to go to.
		{[]string{"Animal", "Duck"}, 1, map[string]int{"Duck": 0}, "Animal:0 Duck:0"},
		// When no place is free, an id takes the place of its hash.
		{[]string{"Duck", "Animal", "Fish"}, 2, nil, "Duck:0 Animal:1 Fish:1"},
	} {
		slots := hashSlots(test.ids, test.n, test.avoid)
		var got []string
		for _, id := range test.ids {
			got = append(got, fmt.Sprintf("%s:%d", id, slots[id]))
		}
		if s := strings.Join(got, " "); s != test.want {
			t.Errorf("%v: got %s, want %s", test.ids, s, test.want)
		}
	}
}

// TestHashDomain checks that each key sits in the domain at the place of
// its color, ten places on for an earlier key of the same color, with gaps
// between (D56).
func TestHashDomain(t *testing.T) {
	t.Parallel()
	keys := []string{"Animal", "Duck", "Fish"}
	for range 10 {
		keys = append(keys, "Animal")
	}
	domain := hashDomain(append(keys, "k0", "k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8"))
	at := map[string]int{}
	for i, v := range domain {
		s, _ := v.(string)
		if !strings.HasPrefix(s, " zuzu.gap") {
			at[s] = i
		}
	}
	slots := hashSlots(append(keys, "k0", "k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8"), hashColors, nil)
	for k, i := range at {
		if i%hashColors != slots[k] {
			t.Errorf("%s at %d, want a place of color %d", k, i, slots[k])
		}
	}
	if len(at) != 12 {
		t.Errorf("%d keys in the domain, want 12", len(at))
	}
}

// TestNodeColors checks that a class in a namespace does not take the
// color of its namespace (D56).
func TestNodeColors(t *testing.T) {
	t.Parallel()
	// The hash of Animal is 3, so it takes the category 4, and its
	// namespace takes that category here.
	ns := &graph.Cluster{ID: "ns", Category: 4}
	n := &graph.Node{ID: "Animal", Cluster: "ns"}
	nodeColors([]*graph.Node{n}, []*graph.Cluster{ns})
	if n.Category != 5 {
		t.Errorf("Animal in a namespace of category 4 takes %d, want 5", n.Category)
	}
}
