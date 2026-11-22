package mermaid

import (
	"fmt"
	"hash/fnv"

	"github.com/xo/zuzu/graph"
)

// hashColors is the number of the categorical colors that the hash of a
// name chooses from, the ten colors of each scheme of zuzu (D56).
const hashColors = 10

// hashSlots returns the place, from 0, of the color of each id among n
// places, by the FNV-1a hash of 32 bits of the id, modulo n (D56). The ids
// take their places in their order. An id whose place an earlier id took
// takes the next free place, and an id never takes the place that avoid
// gives it, if any. When no place is free, an id takes the place of its
// hash, or the next one after a place to avoid. An id that comes again
// keeps its first place.
func hashSlots(ids []string, n int, avoid map[string]int) map[string]int {
	out := make(map[string]int, len(ids))
	taken := make([]bool, n)
	free := n
	for _, id := range ids {
		if _, ok := out[id]; ok {
			continue
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(id))
		// A uint32 fits in an int64, so the hash takes the modulo with no
		// overflow on any architecture.
		start := int(int64(h.Sum32()) % int64(n))
		no, hasNo := avoid[id]
		slot := -1
		for i := range n {
			s := (start + i) % n
			if (taken[s] && free > 0) || (hasNo && s == no && n > 1) {
				continue
			}
			slot = s
			break
		}
		if slot < 0 {
			slot = start
			if hasNo && slot == no && n > 1 {
				slot = (start + 1) % n
			}
		}
		if !taken[slot] {
			taken[slot] = true
			free--
		}
		out[id] = slot
	}
	return out
}

// nodeColors gives each node of nodes the category of the color of the
// hash of its id among the ten colors, in the order of nodes (D56). A
// node in a cluster does not take the color of its cluster.
func nodeColors(nodes []*graph.Node, clusters []*graph.Cluster) {
	cats := map[string]int{}
	for _, c := range clusters {
		cats[c.ID] = c.Category
	}
	ids := make([]string, len(nodes))
	avoid := map[string]int{}
	for i, n := range nodes {
		ids[i] = n.ID
		if c := cats[n.Cluster]; c > 0 {
			avoid[n.ID] = c - 1
		}
	}
	slots := hashSlots(ids, hashColors, avoid)
	for _, n := range nodes {
		n.Category = slots[n.ID] + 1
	}
}

// panelColors gives each cluster of clusters the category of the color of
// the hash of its id among the second to the tenth colors, so that a
// panel never takes the first color, the color of the nodes in it (D56).
func panelColors(clusters []*graph.Cluster) {
	ids := make([]string, len(clusters))
	for i, c := range clusters {
		ids[i] = c.ID
	}
	slots := hashSlots(ids, hashColors-1, nil)
	for _, c := range clusters {
		c.Category = slots[c.ID] + 2
	}
}

// hashDomain returns the domain of an ordinal scale over the range of the
// categorical colors that maps each key to the color of the hash of the
// key, in the order of keys (D56). The scale maps the value at a place i
// of its domain to the color at i modulo ten, so each key sits at the
// place of its color, ten places on for each earlier key of that color,
// and a value that no key takes fills each place between.
func hashDomain(keys []string) []any {
	slots := hashSlots(keys, hashColors, nil)
	var domain []any
	used := map[int]bool{}
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			continue
		}
		seen[k] = true
		i := slots[k]
		for used[i] {
			i += hashColors
		}
		used[i] = true
		for len(domain) <= i {
			// An id of Mermaid has no space at its start, so no key is
			// a gap.
			domain = append(domain, fmt.Sprintf(" zuzu.gap%d", len(domain)))
		}
		domain[i] = k
	}
	return domain
}
