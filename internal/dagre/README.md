# dagre

This folder is a copy of [dagro](https://github.com/d2lang/dagro) at the
commit `30476a97eb11`, under the MIT license in `LICENSE` (D39, D44).
`NOTICE` names the upstream projects of dagro: dagre 3.1.1 and graphlib
4.0.5. Both files are the files of upstream, byte for byte. The rules for a
copy are in [`docs/UPSTREAM.md`](../../docs/UPSTREAM.md).

dagro is a Go implementation of the layout of dagre 3.1.1 by the D2 team.
It lays out a directed graph in ranks, with compound nodes, named edges
between the same two nodes, the boxes of the labels of the edges, and the
four directions. The transform `zuzu.dagre` lays out a graph with it,
through the engine `graph.Dagre`, which puts the wrap of Mermaid 12.3 around
it. Mermaid draws with dagre when a diagram sets `layout: dagre`, and so does
zuzu (D45). The default layout of Mermaid 12.3 is ELK.

Mermaid 12.3 bundles dagre-d3-es 7.0.14, which is of the line of dagre 0.8.
The copy takes the crossing sweep of dagre 0.8 as change 4 below, so that it
places the nodes as Mermaid does (D45 part 1).

The folder is `internal/dagre`, and the package keeps the name `dagro` of
upstream, so that each copied file keeps its package clause.

## The files

Each copied file has the same name as in upstream: `acyclic.go`,
`add_border_segments.go`, `attrs.go`, `callback.go`,
`coordinate_system.go`, `feasible_tree.go`, `graph.go`, `greedy_fas.go`,
`layout.go`, `list.go`, `nesting_graph.go`, `network_simplex.go`,
`normalize.go`, `numeric_semantics.go`, `order.go`,
`order_add_subgraph_constraints.go`, `order_barycenter.go`,
`order_build_layer_graph.go`, `order_cross_count.go`, `order_init.go`,
`order_resolve_conflicts.go`, `order_sort.go`, `order_sort_subgraph.go`,
`parent_dummy_chains.go`, `position.go`, `position_bk.go`, `rank.go`,
`rank_util.go`, `util.go` and `version.go`, with the tests
`acyclic_test.go`, `add_border_segments_test.go`, `attrs_test.go`,
`bundle_test.go`, `coordinate_system_test.go`, `graph_adjacency_test.go`,
`graph_test.go`, `greedy_fas_test.go`, `layout_test.go`, `list_test.go`,
`nesting_graph_test.go`, `normalize_test.go`, `numeric_semantics_test.go`,
`order_add_subgraph_constraints_test.go`, `order_barycenter_test.go`,
`order_build_layer_graph_test.go`, `order_cross_count_test.go`,
`order_init_test.go`, `order_resolve_conflicts_test.go`,
`order_sort_subgraph_test.go`, `order_sort_test.go`, `order_test.go`,
`parent_dummy_chains_test.go`, `performance_test.go`, `position_bk_test.go`,
`position_test.go`, `rank_feasible_tree_test.go`,
`rank_network_simplex_test.go`, `rank_test.go`, `rank_tree_order_test.go`,
`rank_util_test.go`, `util_test.go` and `version_test.go`.

These files are files of zuzu, not of upstream:

1. `zuzu_doc.go` holds the comment of the package.
2. `zuzu_helpers_test.go` holds `newLayoutTestGraph` and `assertNear`, the
   two helpers that the tests of upstream share with
   `layout_differential_test.go`, as upstream writes them.
3. `zuzu_golden_test.go` and `testdata/golden.txt` hold the golden test of
   the copy. It lays out three graphs: a chain with a cycle and a minlen of
   2, a graph from left to right with the labels of its edges, and a graph
   from the bottom to the top with two nested clusters, a label and a loop.
   It compares the place of each node and the points and the label of each
   edge with the golden file, byte for byte.
4. `zuzu_order_test.go` holds the test of change 4. It lays out four graphs
   and compares the center of each node with the center that dagre-d3-es
   7.0.14 gives for the same graph.

The golden file holds the bytes of amd64 below the level v3. The test runs
only on such a machine. On another machine the compiler of Go can fuse a
multiply and an add into one instruction, and the places can differ in the
last digits. zuzu promises the same bytes on one architecture, not across
machines (D41). To write the file again, run
`go test ./internal/dagre -run TestGoldenLayouts -update` on amd64 below
the level v3.

The copy leaves out these files of upstream:

1. `layout_differential_test.go` and the folder `testdata`. The test runs
   dagre in Node.js as an oracle, and reads the corpus of D2, which is under
   the MPL-2.0 of D2.
2. `debug.go` and `debug_test.go`, the debug helper `DebugOrdering`.
3. The files of the project: `go.mod`, `README.md`, `UPSTREAM.md`,
   `.gitignore` and `.github`.

The copy imports only the standard library. It builds with TinyGo for
`wasip1`.

## The changes

Changes 2, 3 and 4 are marked in the code with a comment that starts with
`zuzu:`, which names the number of the change.

1. Each copied file opens with a comment that names the project, the file
   and the commit, and that starts with `Copied from`.
2. The copy leaves out the timing of the debug code. `LayoutOptions` has no
   field `DebugTiming`, and `Layout` runs each step with no timing. `Time`
   and `NoTime` of `util.go` are left out, and so is the test of
   their output in `util_test.go`, with its helper `captureTestStdout` and
   its imports `io`, `os` and `regexp`. zuzu prints
   nothing to the standard output.
3. `list.go` imports `encoding/json/v2` in place of `encoding/json`, which
   D29 allows and TinyGo builds. Its `String` passes the option
   `json.Deterministic(true)`, so that the keys of a map stay sorted, as
   `encoding/json` sorts them.
4. `order.go` sweeps the layers for the fewest crossings as dagre 0.8 does,
   and not as dagre 3.1.1 does (D45 part 1). Of two orders with the same
   count of crossings, `order` keeps the first, where dagre 3.1.1 keeps the
   last. `sweepLayerGraphs` sweeps every layer with the bias that its
   argument gives, where dagre 3.1.1 starts each sweep with the bias to the
   right and switches it after each layer that used it. The test
   `TestLayoutPreservesMultipleReversedParallelPartners` of `layout_test.go`
   expects the height 213 of the sweep of dagre 0.8, which dagre-d3-es
   7.0.14 gives, in place of the height 200.5 of dagre 3.1.1.

## What zuzu does not change

1. `SetDefaultNodeLabel` and `SetDefaultEdgeLabel` call a function that a
   caller gives as a label through `reflect.Value.Call` in `callback.go`,
   which TinyGo does not support well. zuzu never calls them. The engine of
   `zuzu.dagre` gives each node and each edge its own attributes, and the
   copy calls them nowhere.
2. `uniqueID` of `util.go` counts in a variable of the package, as dagre
   does. It names the reversed edges and the root of each layer graph. The
   names never reach the places, so the output stays the same from call
   to call.
