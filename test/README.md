# Test module

This folder is a Go module of its own. It holds the corpus, the tools that
read it and the oracle, which a user of zuzu never imports (D5, D17).

| Path | What it holds |
| --- | --- |
| `testdata/corpus/` | the specs of the corpus and the data they read, with the license of each source beside its files |
| `testdata/corpus/inventory.tsv` | the features that each spec uses, which `cmd/inventory` writes |
| `testdata/references/` | the references of vega.js, which the oracle writes and git ignores |
| `cmd/inventory/` | the tool that writes the inventory |
| `cmd/wasm/` | a program that imports zuzu, so that CI can build it with TinyGo (D29) |
| `oracle/` | the oracle, which runs vega.js and vega-lite.js in Node.js |

## The corpus

The specs came from the testdata of `github.com/xo/vegagoja`: 580 specs of
the examples of Vega-Lite, 88 of Vega, and 11 of Deneb. Each folder holds the
license of its source. The data folder holds the 53 files that the specs
read, except three large files that only specs that bind events read.
`testdata/corpus/data/LICENSE-data.txt` says which.

## Write the inventory

Run this in this folder:

```bash
go run ./cmd/inventory
```

On 2026-11-10 it listed 679 specs. 540 of them are in the first target: they
bind no event and the corpus holds all of their data (D7).

## Write the references

The oracle needs Node.js. Run this in this folder:

```bash
(cd oracle && npm install)
TZ=UTC node oracle/oracle.mjs testdata
```

The time zone is UTC, so that the references are the same on every machine.
On 2026-11-10 it wrote 540 references in about 8 seconds, 174 MB in all, and
no spec failed in vega.js. Each reference holds what D17 compares: the
compiled Vega spec, the rows of each dataset, the type and the domain of each
scale, and the data of each item of each mark.
