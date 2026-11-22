// SPDX-License-Identifier: EPL-2.0

package elk

// This file is a file of zuzu, not of elk-go. It holds the tests of change
// 3: no algorithm takes a seed from the clock or draws from the global
// math/rand, so the same graph gives the same layout, and the seed 0 gives
// the layout of the seed 1.

import (
	"reflect"
	"strconv"
	"testing"
)

// seedGraph returns a graph of five nodes at one place, so that an
// algorithm that jitters or draws at random has to, with an algorithm and a
// seed. An empty seed sets none.
func seedGraph(algorithm, seed string) *Graph {
	g := &Graph{
		ID:            StringID("root"),
		LayoutOptions: LayoutOptions{"elk.algorithm": algorithm},
	}
	if seed != "" {
		g.LayoutOptions["elk.randomSeed"] = seed
	}
	for i := range 5 {
		g.Children = append(g.Children, &Node{
			ID:    StringID("n" + strconv.Itoa(i)),
			Width: Float64(40), Height: Float64(30),
		})
	}
	for i := range 5 {
		g.Edges = append(g.Edges, &Edge{
			ID:      StringID("e" + strconv.Itoa(i)),
			Sources: []ID{StringID("n" + strconv.Itoa(i))},
			Targets: []ID{StringID("n" + strconv.Itoa((i+2)%5))},
		})
	}
	return g
}

// seedAlgorithms are the algorithms that draw random numbers or jitter.
var seedAlgorithms = []string{"layered", "force", "random", "sporeOverlap", "sporeCompaction"}

func TestZuzuSeedZeroIsSeedOne(t *testing.T) {
	t.Parallel()
	for _, algorithm := range seedAlgorithms {
		zero, one := seedGraph(algorithm, "0"), seedGraph(algorithm, "1")
		if err := Layout(zero); err != nil {
			t.Fatalf("%s: %v", algorithm, err)
		}
		if err := Layout(one); err != nil {
			t.Fatalf("%s: %v", algorithm, err)
		}
		zero.LayoutOptions, one.LayoutOptions = nil, nil
		if !reflect.DeepEqual(zero, one) {
			t.Errorf("%s: the seed 0 gives another layout than the seed 1", algorithm)
		}
	}
}

func TestZuzuSameGraphSameLayout(t *testing.T) {
	t.Parallel()
	for _, algorithm := range seedAlgorithms {
		for _, seed := range []string{"", "0", "7"} {
			want := seedGraph(algorithm, seed)
			if err := Layout(want); err != nil {
				t.Fatalf("%s %q: %v", algorithm, seed, err)
			}
			for range 3 {
				got := seedGraph(algorithm, seed)
				if err := Layout(got); err != nil {
					t.Fatalf("%s %q: %v", algorithm, seed, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s with the seed %q gives another layout on another run", algorithm, seed)
				}
			}
		}
	}
}
