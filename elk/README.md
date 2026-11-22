# elk

The module `github.com/xo/zuzu/elk` lays out the diagrams of zuzu with ELK,
the Eclipse Layout Kernel. It runs a copy of
[elk-go](https://github.com/d2lang/elk-go), the port of ELK.js 0.12.0 to Go
by the D2 team. ELK.js 0.12.0 is the version of ELK that Mermaid 12.3 uses
(D39 part 13, D44).

A program makes the engine available to zuzu as an option. zuzu then
draws with ELK each graph of Mermaid whose layout is ELK: a graph that sets
no layout, in a kind whose default layout in Mermaid 12.3 is ELK, and a
graph with `layout: elk` or another ELK layout, such as `elk.stress`
(D45):

```go
chart, report, err := zuzu.Parse(src, zuzu.WithLayoutEngine(elk.Layout()))
```

`zuzu.WithLayout("elk")` then forces ELK on every graph of the chart, DOT
too. Without the engine, an ELK layout stays a hint, and zuzu draws the
diagram with `zuzu.layered` (D32). "Layouts" in `docs/API.md` says how zuzu
picks a layout.

## The license

This module is under the Eclipse Public License 2.0 (EPL-2.0), in
[`LICENSE`](LICENSE). Two files of the copy are under the Apache License 2.0,
in [`LICENSE.apache-2.0`](LICENSE.apache-2.0), and one is under the BSD
3-Clause License, in [`LICENSE.v8-bsd`](LICENSE.v8-bsd). The line
`SPDX-License-Identifier` of each file names its license. The module
`github.com/xo/zuzu` stays under the MIT license and holds no file under the
EPL-2.0. The two modules are apart so that a program that does not import
this one ships no code under the EPL-2.0.

The EPL-2.0 is a copyleft at the level of the file. A program that imports
this module and ships a binary ships the files of this module in it. Whoever
ships such a binary must make the source code of those files available, and
must tell the people who get the binary how to get that source. The files of
the program itself can stay under their own license, and so can the files of
zuzu that only call this module. A program that ships the module also keeps
its notices: the license files above and the files `NOTICE.md`,
`NOTICE.eclipse-elk.md` and `PROVENANCE.md` in `internal/elkgo`.

This section says what the licenses ask, as the maintainers read them. It is
not legal advice. Read the licenses, and ask a lawyer about your own case.

## The packages

| Path | What it holds |
| --- | --- |
| `elk.go` | the package `elk`: `Layout`, which returns the engine `elk` of the interface `graph.Engine` of zuzu, and the conversion of a graph of zuzu to the graph of the adapter and back |
| `internal/adapter/` | the adapter, which builds the graph of ELK from a graph in plain Go with the layout options that Mermaid 12.3 gives, runs the layout and reads the places back, with its golden file in `internal/adapter/testdata` |
| `internal/elkgo/` | the copy of elk-go, with its licenses and its notices. Its `README.md` lists the files and the changes |
| `internal/wasm/` | a program that lays out a graph, so that CI can build the module with TinyGo for WebAssembly (D29) |

## The algorithms

The engine takes the algorithm from `graph.LayoutInput.Algorithm`, which
holds the name of the layout of Mermaid: `elk`, or empty, for
`elk.layered`, and `elk.stress`, `elk.force`, `elk.mrtree`,
`elk.sporeOverlap`, `elk.box` and `elk.rectpacking`. Another name is
unsupported, and the graph lays out with `elk.layered`. `adapter.Graph`
takes the names of ELK, and a cluster of the adapter can also take
`elk.radial`, as in Mermaid. zuzu gives a cluster no algorithm of its own
yet.

The adapter follows `render.ts` of Mermaid 12.3.0, which is the same file in
its bundle and in `@mermaid-js/layout-elk` 1.0.2. It gives the root and each
cluster the layout options of Mermaid, and `Graph.Config` holds the settings
of the configuration `elk` of Mermaid: the preset, the strategy and the
alignment of the placement of the nodes, the strategy that breaks the
cycles, the strategy of the layers, `mergeEdges` and the order of the model.
The spacing of the nodes is the spacing of Mermaid: `spacing.baseValue` 40 at
the root, and at each cluster 24, with a spacing of 50 between two nodes.
`Graph.Options` sets any other option of ELK at the root.

These passes of Mermaid are not in the adapter: the edges that turn back
into a cluster, the pin of the first node of a cycle to the first layer
(`keepEntryNodeOnTop`), the order of the edges by their depth, and the
room for the markers at the ends of an edge. They are in
`docs/BACKLOG.md`.

The same graph gives the same layout, as AGENTS.md, hard rule 6, asks. No
algorithm takes a seed from the clock, and the seed 0 is the seed 1.

## The binding

`elk.Layout` returns the engine as a `graph.Engine` of zuzu (D39 part 10,
D45). Its `Lay` does these steps:

1. It converts `graph.LayoutInput` to `adapter.Graph`. The clusters and the
   nodes are in one list, as Mermaid gives them to ELK: the clusters first,
   then the nodes, each in the order of the input. With
   `LayoutInput.ReverseClusters`, the sibling clusters are in the reverse
   of the order of the input, as Mermaid gives the subgraphs of a flowchart
   (D45 part 5). Each node has its measured size, and each cluster its
   title, its padding and its direction. Each part takes a new id, so that
   no id of the input can clash with another or with `root`.
2. It scales the graph up by 16/11, from the pixels of zuzu, whose text is
   11 pixels, to the pixels of Mermaid, whose text is 16 pixels. The
   adapter gives ELK the spaces of Mermaid in its pixels, so the spaces
   keep the ratio to the nodes that they have in Mermaid.
3. It calls `adapter.Layout`, and converts `adapter.Result` back to
   `graph.Layout`, scaled down by 11/16. An algorithm that routes no edge,
   such as `elk.box`, leaves each edge a line between the centers of its
   ends.
4. It puts `adapter.Result.Issues` in `graph.Layout.Unsupported`, after the
   parts of the input that the engine leaves out: the ports, the lanes, the
   groups of the same rank, the edges that take no part in the ranks, and
   the least length and the weight of an edge. The transform reports each
   one (AGENTS.md, hard rule 4).
5. It calls `graph.Finish`, which puts the ends of the edges on the
   outlines of the shapes and places the labels at the ends and the
   xlabels, as for every engine.

`go.mod` requires `github.com/xo/zuzu` with a `replace` of `../`, as
`test/go.mod` does, so the module builds against the zuzu of this
repository. The test module requires this module with a `replace` of
`../elk`, so that the position error and the review page draw with ELK.

## Tests

Run these in this folder:

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./...
golangci-lint run ./...
GOOS=wasip1 GOARCH=wasm tinygo build -o elk.wasm ./internal/wasm
```

The golden test in `internal/adapter` runs only on amd64 below the level v3
(D41). To write its golden file again, run
`go test ./internal/adapter -run TestGoldenLayouts -update` on such a
machine.

`TestWriteGraphsForELKjs` writes each graph of the tests as the JSON of ELK
when `ZUZU_ELK_GRAPHS` names a folder, so that a person can lay them out with
ELK.js and compare.
