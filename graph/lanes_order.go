package graph

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// This file holds the order of the lanes that the setting
// automaticLaneOrdering of the swimlane layout of Mermaid 12.3 finds.

// laneRestarts is the number of the starts from a shuffled order of the
// lanes, after the start from the order of the input.
const laneRestarts = 8

// laneWeight is the number of the edges between two lanes, a before b in
// the order of the input.
type laneWeight struct {
	a, b   int
	weight int
}

// orderLanes orders the lanes as the setting
// swimlane.automaticLaneOrdering of Mermaid 12.3 does. The cost of an
// order is the sum, over the edges between two lanes, of the number of
// lanes from one end to the other. Swaps of two neighbor lanes lower the
// cost, from the order of the input and from 8 shuffled orders, and the
// order of the least cost wins. Of two orders of the same cost, the one
// closer to the order of the input wins. A lane that holds no node takes
// part in the order, as in Mermaid, and then takes no place.
func (l *laneLayout) orderLanes() {
	// The lanes of Mermaid: the lane of the nodes outside every cluster,
	// then each cluster at the top, held or not, with its id.
	var source []int
	if slices.Contains(l.lanes, -1) {
		source = append(source, -1)
	}
	for c := range l.in.Clusters {
		if l.cparent[c] < 0 {
			source = append(source, c)
		}
	}
	if len(source) < 2 {
		return
	}
	id := func(lane int) string {
		if lane < 0 {
			return ""
		}
		return l.in.Clusters[lane].ID
	}
	at := make(map[int]int, len(source))
	for i, lane := range source {
		at[lane] = i
	}
	// laneOf returns the lane of an end of an edge: the lane of a node, or
	// the lane of a cluster inside a lane. A lane itself is in no lane.
	laneOf := func(end int) (int, bool) {
		if end >= 0 {
			return l.lanes[l.lane[end]], true
		}
		c := -1 - end
		if l.cparent[c] < 0 {
			return 0, false
		}
		t := l.top(c)
		return t, t >= 0
	}
	weights := map[[2]int]*laneWeight{}
	for _, end := range l.ends {
		la, ok1 := laneOf(end[0])
		lb, ok2 := laneOf(end[1])
		if !ok1 || !ok2 || la == lb {
			continue
		}
		if at[la] > at[lb] {
			la, lb = lb, la
		}
		k := [2]int{la, lb}
		if w, ok := weights[k]; ok {
			w.weight++
			continue
		}
		weights[k] = &laneWeight{la, lb, 1}
	}
	if len(weights) == 0 {
		return
	}
	ws := slices.Collect(maps.Values(weights))
	slices.SortFunc(ws, func(x, y *laneWeight) int {
		if x.a == y.a {
			return collate(id(x.b), id(y.b))
		}
		return collate(id(x.a), id(y.a))
	})
	cost := func(order []int) int {
		pos := make(map[int]int, len(order))
		for i, lane := range order {
			pos[lane] = i
		}
		c := 0
		for _, w := range ws {
			d := pos[w.a] - pos[w.b]
			c += w.weight * max(d, -d)
		}
		return c
	}
	distance := func(order []int) int {
		d := 0
		for i, lane := range order {
			d += max(i-at[lane], at[lane]-i)
		}
		return d
	}
	// greedy swaps neighbor lanes while a swap lowers the cost, for as many
	// sweeps as there are lanes at most.
	greedy := func(start []int) ([]int, int) {
		order := slices.Clone(start)
		c := cost(order)
		for sweep, changed := 0, true; changed && sweep < len(order); sweep++ {
			changed = false
			for i := 0; i+1 < len(order); i++ {
				order[i], order[i+1] = order[i+1], order[i]
				if next := cost(order); next < c {
					c, changed = next, true
				} else {
					order[i], order[i+1] = order[i+1], order[i]
				}
			}
		}
		return order, c
	}
	ids := make([]string, len(source))
	for i, lane := range source {
		ids[i] = id(lane)
	}
	sig := make([]string, len(ws))
	for i, w := range ws {
		sig[i] = id(w.a) + ":" + id(w.b) + ":" + strconv.Itoa(w.weight)
	}
	best, bestCost := greedy(source)
	bestDist := distance(best)
	for i := range laneRestarts {
		seed := fnv1a(strings.Join(ids, "|") + "#" + strings.Join(sig, "|") + "#" + strconv.Itoa(i))
		order, c := greedy(shuffleLanes(source, seed))
		if d := distance(order); c < bestCost || c == bestCost && d < bestDist {
			best, bestCost, bestDist = order, c, d
		}
	}
	var lanes []int
	for _, lane := range best {
		if slices.Contains(l.lanes, lane) {
			lanes = append(lanes, lane)
		}
	}
	for i, li := range l.lane {
		l.lane[i] = slices.Index(lanes, l.lanes[li])
	}
	l.lanes = lanes
}

// fnv1a returns the hash FNV-1a of 32 bits of the code units of UTF-16 of
// a string, as the hash of the lane order of Mermaid 12.3 reads a string
// of JavaScript.
func fnv1a(s string) uint32 {
	h := uint32(2166136261)
	for _, u := range utf16.Encode([]rune(s)) {
		h ^= uint32(u)
		h *= 16777619
	}
	return h
}

// shuffleLanes returns a shuffle of an order by Fisher and Yates, from the
// last place down, with the numbers of mulberry32 from a seed, as
// Mermaid 12.3 shuffles the lanes.
func shuffleLanes(order []int, seed uint32) []int {
	out := slices.Clone(order)
	state := seed
	random := func() float64 {
		state += 0x6d2b79f5
		t := state
		t = (t ^ t>>15) * (t | 1)
		t ^= t + (t^t>>7)*(t|61)
		return float64(t^t>>14) / 4294967296
	}
	for i := len(out) - 1; i > 0; i-- {
		j := int(math.Floor(random() * float64(i+1)))
		out[i], out[j] = out[j], out[i]
	}
	return out
}
