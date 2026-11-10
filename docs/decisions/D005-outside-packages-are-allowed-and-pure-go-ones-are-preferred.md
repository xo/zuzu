# D5. Outside packages are allowed, and pure Go ones are preferred

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu has no limit on imports,
unlike the other `xo` repositories, and that it prefers packages that are
pure Go. A pure Go package uses no cgo, links no C or Rust library, and runs
no WebAssembly.

The module that a user imports uses no cgo. Where two packages do the same
work, zuzu takes the pure Go one. Each outside package gets a row in the
table of dependencies in `docs/PLAN.md` when it is added, with the reason.

The test module can use anything that a test needs, such as Node.js and
vega.js as an oracle (D17), because a user never imports it.
