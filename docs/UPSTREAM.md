# Copies of other projects

This document holds the rules for Go code that zuzu copies from another
project (D39). A copy takes the Go code of a project as it is, with the
changes that zuzu needs, under the license of the project. zuzu makes no
port: no file of zuzu translates code from another language (D6). The rules
come from ivy, which keeps copies the same way.

## The list of upstreams

[`upstream/upstreams.tsv`](upstream/upstreams.tsv) lists each project that
zuzu copies. An upstream is such a project. The file has one row for each
upstream, with these columns, separated by a tab:

| Column | What it holds |
| --- | --- |
| `name` | the name of the upstream, and the name of its folder under `upstream/` in the root |
| `url` | the URL of its repository |
| `kind` | `copy`, the only kind that zuzu has |
| `license` | the SPDX identifier of its license |
| `base` | the full hash of the commit that zuzu took the code from |
| `matched` | the full hash of the newest commit whose changes zuzu has, or the base if there is none yet |
| `path` | the folder in zuzu that holds the copy. Two folders are separated by a comma and a space |

Add the row before the first commit of the copy. Each new upstream needs the
approval of the maintainers.

On 2026-11-22 the list holds three upstreams:

1. `rough-go`, the port of Rough.js 4.6.6 to Go by the D2 team, in
   `internal/rough`, for the look drawn by hand (D39, D40).
2. `dagro`, the Go implementation of dagre 3.1.1 by the D2 team, in
   `internal/dagre`, for the transform `zuzu.dagre` (D39, D44). The copy
   sweeps the crossings as dagre 0.8 does, which Mermaid 12.3 bundles
   (D45 part 1).
3. `elk-go`, the port of ELK.js 0.12.0 to Go by the D2 team, in
   `elk/internal/elkgo`, for the layout of ELK. It is in the module
   `github.com/xo/zuzu/elk`, under the EPL-2.0, and not in the module
   `github.com/xo/zuzu` (D39 part 13, D44).

## The checkouts

The work is made from a checkout of each upstream in `upstream/<name>/`, in
the root of the repository. Git ignores the folder. Do not edit a file in a
checkout. If a checkout is missing, clone it at the base commit, from the
root of the repository:

```bash
git clone https://github.com/d2lang/rough-go.git upstream/rough-go
```

```bash
git -C upstream/rough-go checkout e7169d56ddc67985d9397e11035a0e9e4f6de2a6
```

Clone elk-go and dagro the same way:

```bash
git clone https://github.com/d2lang/elk-go.git upstream/elk-go
```

```bash
git -C upstream/elk-go checkout 4e0a9977ed8b9de6820c6f448bd50536cc33a683
```

```bash
git clone https://github.com/d2lang/dagro.git upstream/dagro
```

```bash
git -C upstream/dagro checkout 30476a97eb110dfc64e32a5691e37a2e6f2dc175
```

A checkout of Go code holds a `go.mod` of its own, so `go build ./...` in
zuzu does not build it. The checkouts of rough-go and dagro have one in
their root. The
tests of the documents skip the folder `upstream`, because no file in it is
a file of zuzu.

## A copy

1. The copy goes in a folder of its own under `internal/`, named after the
   project, such as `internal/rough`. A copy in another module of this
   repository goes under the `internal/` of that module, such as
   `elk/internal/elkgo`.
2. The folder holds the `LICENSE` file and the `NOTICE` file of the
   upstream, byte for byte, if the upstream has them. If it has another file
   of notices, the folder holds that file too.
3. Each copied file opens with a comment that names the project, the file
   and the base commit, before the comments and the package clause of
   upstream.
4. A change to a copied file is marked with a comment that starts with
   `zuzu:`, and it says why. A copy stays as close to its upstream as it
   can, so that a later commit of the upstream still applies. zuzu changes
   a copy only where it must.
5. The copy leaves out what zuzu does not use: the commands, the debug code,
   the files of the project such as `go.mod`, and each test that runs
   Node.js or another program as an oracle. It also leaves out test data
   that comes from D2 or from another project under another license. It
   keeps the other tests of upstream, and they pass.
6. Each change that zuzu needs for determinism, for TinyGo or for D29 goes
   in the copy as a marked change. Offer each such change to the upstream.
   A copy can also take a marked change that makes it follow the version
   that Mermaid 12.3 bundles, such as the crossing sweep of dagre 0.8 in
   `internal/dagre` (D45 part 1). Such a change stays in zuzu.
7. A file that zuzu adds to the folder has a name that starts with `zuzu_`,
   such as `zuzu_test.go`, or is the `README.md` or a file in `testdata`.
8. The `README.md` of the folder names the upstream, the commit and the
   license. It lists the files that the copy holds, the files that it
   leaves out, and the changes, with a number for each. A comment `zuzu:`
   can name its change by that number.
9. Each copy has a golden test of its output. Its golden file holds the
   bytes of amd64 below the level v3, and the test has the build constraint
   `amd64 && !amd64.v3`, so it runs only there. Write the golden file again
   on such a machine.

## What does not check a copy

A copy keeps the code and the comments of its upstream, which follow the
rules of that project and not those of zuzu (D40 part 9).

1. golangci-lint does not check a copy. `.golangci.yml` excludes each folder
   in the column `path`, for the linters and for the formatters. A copy in
   another module is excluded by the `.golangci.yml` of that module, such as
   `elk/.golangci.yml`.
2. The tests of the documents in the root package read each document, each
   comment and each message of the repository. They skip each file in a
   folder of the column `path`, except the files that rule 7 of "A copy"
   names, which are files of zuzu. The tests that skip a copy are
   `TestProseIsSimpleEnglish`, `TestEveryMarkdownLinkResolves` and
   `TestEveryDecisionReferenceExists`. `TestEveryCopyFollowsTheRules` checks
   that each folder in the list exists, holds a `LICENSE` and a `README.md`,
   and that each copied Go file opens with the comment of rule 3.
3. `gofmt -l .` and `go vet ./...` still check a copy, and the tests of the
   copy still run.

## Determinism on one architecture

The output of zuzu is the same bytes on one architecture, not across
machines (D41). A copy follows that rule and no stricter one. The package
`math` of Go gives other bits for `Sin`, `Cos` and their kind on arm64 than
on amd64, and the compiler of Go fuses `x*y + z` into one instruction on
arm64, ppc64le, s390x, riscv64 and loong64, and on amd64 at the level v3.
So the output of a copy can differ between machines in the last digits. A
copy does not change its upstream to stop that, and it keeps the code of its
upstream. This is why the golden file of a copy is pinned to amd64 below the
level v3.

The tests of a copy other than its golden test run on every architecture,
and CI runs the tests of the root module on arm64 too.

## A new commit of an upstream

1. Read the commits of the upstream after `matched`, in its checkout.
2. Apply each change that zuzu needs, and keep the changes of zuzu. A change
   to a line that zuzu changed needs the change of zuzu again.
3. Run the tests of the copy, with its golden test, on amd64 below the level
   v3.
4. If the output changes on purpose, write the golden file again on such a
   machine, and say in the `README.md` of the copy why it changed.
5. Update the list of files and the changes in the `README.md` of the copy.
6. Move `matched` in `upstream/upstreams.tsv` to the newest commit that zuzu
   has.
