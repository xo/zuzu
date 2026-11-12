# D33. vega.js is the oracle of expressions, and schemes are Go values

Status: Decided. Amends D9 and D15.

On 2026-11-12 a sweep of the documents found two decisions that the code
does not follow. The maintainers chose to change the decisions, because
the code works and passes its tests.

## The oracle of the expression language

1. vega.js in Node.js is the oracle of the expression language. The test
   module runs `test/oracle/exprs.mjs` over the cases of
   `expr/testdata/exprs.txt`, and a test of the package `expr` compares
   each result with the result of vega.js.
2. goja is not an oracle in the test module. This amends item 3 of D9. The
   other use of goja in D9 stays: scripting beyond the expression language
   runs in goja, in an optional package, which is not built yet.

## The form of the color schemes

3. A color scheme is a Go value in `theme/theme.go`. The package `theme`
   embeds no JSON file of a scheme. This amends item 6 of D15.
4. The tool that converts upstream themes into schemes, and the schemes
   `catppuccin` and `solarized`, which exist only in
   `docs/targets/gen-targets.mjs`, are work for later in
   `docs/BACKLOG.md`. When the tool comes, it writes Go values, or the
   maintainers change this decision.
