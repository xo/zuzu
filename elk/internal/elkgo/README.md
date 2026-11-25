# elkgo

This folder is a copy of [elk-go](https://github.com/d2lang/elk-go) at the
commit `4e0a9977ed8b`, under the Eclipse Public License 2.0 in `LICENSE`
(D39 part 13, D44). The rules for a copy are in
[`docs/UPSTREAM.md`](../../../docs/UPSTREAM.md).

elk-go is a port to Go of ELK.js 0.12.0, the version of ELK that Mermaid 12.3
uses, by the D2 team. It lays out a graph with 11 algorithms of ELK: fixed,
box, random, layered, stress, mrtree, radial, force, sporeOverlap,
sporeCompaction and rectpacking. The layout `zuzu.elk` of D39 runs it through
the package `internal/adapter` of this module.

## The licenses

The folder keeps these files of upstream, byte for byte:

1. `LICENSE` is the Eclipse Public License 2.0, the license of elk-go and of
   most of its files.
2. `LICENSE.apache-2.0` is the Apache License 2.0 of two adaptations of the
   runtime of GWT, `internal/javacompat/gwt_priority_queue.go` and
   `internal/javacompat/gwt_stable_sort.go`, and of the test of the second,
   `internal/javacompat/gwt_stable_sort_test.go`.
3. `LICENSE.v8-bsd` is the BSD 3-Clause License of the routines of V8 and
   fdlibm in `internal/javacompat/jsmath.go`, which also keeps the notice of
   Sun Microsystems.
4. `NOTICE.md`, `NOTICE.eclipse-elk.md` and `PROVENANCE.md` name the sources
   of the port and their licenses.

The line `SPDX-License-Identifier` of each copied file names its license.
The folder name is `elkgo`, after the project, because the package `elk` of
this module is the one that a caller imports. The root package of the copy
keeps its upstream name `elk`, and the package `internal/adapter` imports it
as `elk`.

## The files

The copy holds the root package of upstream, `elk.go` and `layout.go`, and
the packages `graphjson` and `internal/...` with their tests. That is 192
files of source and 103 files of tests, with the same names and the same
folders as in upstream. The import paths start with
`github.com/xo/zuzu/elk/internal/elkgo` in place of
`github.com/d2lang/elk-go`.

The copy keeps the 11 algorithms, because `internal/builtins` registers each
of them. Mermaid names seven of them, and a cluster of Mermaid can also take
radial.

These files are files of zuzu, not of upstream:

1. `zuzu_layout_test.go` holds the checks of the tests `layout_test.go` and
   `inside_self_loop_test.go` of upstream that do not need the JSON form of a
   graph, written with the types of Go, with the expected values of upstream
   (change 4).
2. `zuzu_seed_test.go` holds the tests of change 3.

The golden test of the copy is in `internal/adapter`, with its golden file
in `internal/adapter/testdata`. It lays out each algorithm through the
adapter, so it checks the output of the copy too.

The copy leaves out these files of upstream:

1. `cmd/`, the command line tool and the program that runs ELK.js as an
   oracle.
2. `internal/oracle/`, the oracle that runs ELK.js in Node.js, and each test
   that imports it: `internal/force/oracle_test.go`,
   `internal/mrtree/provider_oracle_test.go`,
   `internal/radial/provider_oracle_test.go`,
   `internal/rectpacking/provider_oracle_test.go`,
   `internal/spore/provider_oracle_test.go`,
   `internal/stress/oracle_test.go`, and in `internal/layeredlayout` the
   files `advanced_processors_oracle_test.go`,
   `ancestor_descendant_oracle_test.go`, `audit_all_enums_test.go`,
   `connected_components_oracle_test.go`,
   `external_port_ns_oracle_test.go`, `free_ports_oracle_test.go`,
   `invalid_enum_fallback_test.go`, `isolated_external_port_oracle_test.go`,
   `network_simplex_oracle_test.go`, `polyline_selfloop_oracle_test.go`,
   `post_compaction_nonfinite_oracle_test.go`, `strategy_oracle_test.go` and
   `undefined_routing_oracle_test.go`.
3. `testdata/`, which holds the fixtures that only the oracle reads, two of
   them from `moon_elk` under the EPL-2.0, and the notes of the upgrade to
   0.12.
4. `graphjson/model_json.go` and its test `graphjson/model_test.go`, and the
   tests `layout_test.go` and `inside_self_loop_test.go`, which read and
   write the JSON form of a graph (change 4).
5. The files of the project: `go.mod`, `README.md`, `.gitignore` and
   `.github`.

The copy imports only the standard library. It builds with TinyGo for
`wasip1`.

## The changes

Changes 3 and 4 are marked in the code with a comment that starts with
`zuzu:`, which names the number of the change.

1. Each copied file opens with a comment that names the project, the file
   and the commit, and that starts with `Copied from`.
2. The import paths of the packages of elk-go start with
   `github.com/xo/zuzu/elk/internal/elkgo`. The comment of change 1 says so
   in each file, so the change has no comment `zuzu:` of its own.
3. No algorithm takes a seed from the clock or draws from the global
   `math/rand` (AGENTS.md, hard rule 6). In `internal/layered/configurator.go`
   and `internal/force/importer.go`, the seed 0 of `org.eclipse.elk.randomSeed`
   takes the seed 1, its default, where upstream takes the time. In
   `internal/random/provider.go`, the seed 0 takes the seed 1, where upstream
   takes the time and a global counter, as the new `Random()` of Java does.
   The counter `seedUniquifier`, its `init` and `defaultSeed` are left out. In
   `internal/spore/importer.go`, the jitter of a center that another node
   holds draws from a `javacompat.Random` of the layout with the seed 1, where
   upstream draws from the global `math/rand`. The output of each other seed
   is the output of upstream.
4. The copy reads and writes no JSON. zuzu reads JSON only with
   `encoding/json/v2` (D29), and it builds the graph of ELK in Go, so the
   JSON form of upstream, which reads `encoding/json` v1, is left out. In
   `layout.go`, `LayoutJSON` is left out. In `graphjson/model.go`, the
   methods `MarshalJSON` and `UnmarshalJSON` of `ID` and the field
   `AdditionalFields` of each type are left out, and gofmt aligns the fields
   of each type again. In `internal/core/codec.go`, `optionString` has no
   case for `json.Number`, and it writes any other value with `fmt.Sprint`.
   A move of the JSON form to `encoding/json/v2` was the other choice. It
   changes about 400 lines, and v2 differs from v1 in the order of the keys
   of a map, in duplicate names and in the text of a number, which the tests
   of the JSON form check.

## What differs from ELK.js

The adapter compared its graphs with ELK.js 0.12.0, the bundle of Mermaid
12.3.0, on 21 graphs. Twelve gave the same places, and elk-go differs in
these places. Each is in `docs/BACKLOG.md`, to offer upstream:

1. elk-go places the label of a cluster with `nodeLabels.placement` set to
   `[H_CENTER V_TOP, INSIDE]` at the left and in the middle of the height
   when the direction is DOWN or UP, and it keeps no room for it at the top
   in any direction. ELK.js places it at the top in the middle and moves the
   children down. The adapter works around it (`reserveTitles` in
   `internal/adapter`). The work-around does not cover a title that is wider
   than its cluster when the direction is DOWN or UP, or a long title of a
   cluster whose frame draws no title. ELK.js keeps room beside the cluster
   for such a title, up to 92 pixels in the tests.
2. elk-go spreads the ports of a node with `spacing.portsSurrounding` over
   the side less the two margins, and ELK.js spreads them a little wider. A
   port moves by less than one pixel, and a node by up to one pixel.
3. A cluster with the algorithm radial is larger in elk-go than in ELK.js,
   and its nodes move by up to 35 pixels.
