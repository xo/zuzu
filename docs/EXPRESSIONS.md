# Expressions and scripting

This document holds the design of the expression language of zuzu, and the
choices for scripting beyond it. The maintainers accepted the design on
2026-11-10, with choice C (D9).

## What Vega does

Vega has an expression language. It is a subset of JavaScript expressions.
An expression reads values and returns a value. It cannot assign a variable,
define a function, make an object with `new`, or reach anything outside the
names that Vega gives it. These are examples:

```text
datum.price * 2
datum.open < datum.close ? 'up' : 'down'
timeFormat(datum.date, '%Y')
scale('x', datum.year) + bandwidth('x') / 2
```

A spec uses expressions in these places:

| Place | Example |
| --- | --- |
| the `filter` and `formula` transforms | `{"type": "filter", "expr": "datum.value > 0"}` |
| a signal, as its `update` | `{"name": "w", "update": "width / 2"}` |
| an encoding, as a `signal` | `"x": {"signal": "datum.x0 + 4"}` |
| a test in an encoding | `{"test": "datum.open < datum.close", "value": "green"}` |
| a label of an axis or a legend | `"labelExpr": "datum.label + ' km'"` |
| a tooltip | `"tooltip": {"signal": "datum.name"}` |

Vega-Lite writes the same expressions, in `filter`, `calculate`,
`condition` and `labelExpr`, and compiles them into the Vega spec unchanged.

The parser of Vega, vega-expression, reads the text into a tree. Vega then
generates JavaScript from the tree and runs it. The names an expression can
reach are `datum`, `event`, `item`, `parent`, the signals, a list of
constants such as `PI`, and a list of more than 100 functions. The functions
cover math, strings, arrays, dates and times, number and time formats,
regular expressions, colors, and access to the scales and the data of the
view.

## What zuzu needs

zuzu must evaluate those expressions in Go, for every spec in the first
target (D7). An expression runs once for each row of data in a transform,
and once for each item of a mark in an encoding, so it must be fast.

The browser component needs the same expressions in JavaScript (D19).

## The choices

### A. A native evaluator

zuzu parses the grammar of vega-expression with a parser written by hand,
into a tree. It compiles the tree into Go closures, one for each node, and
calls them. The function library is written in Go.

1. It is pure Go, small, and has no dependency.
2. It is fast. A closure tree runs without the conversion of values between
   Go and a JavaScript engine.
3. It is safe for a spec from an untrusted source. It reaches only the names
   it is given, and the `regexp` package of Go runs in linear time, so a
   regular expression cannot stop the program.
4. It must copy the rules of JavaScript exactly: `+` adds numbers and joins
   strings, `==` converts types and `===` does not, and `NaN` and `null`
   behave as they do in JavaScript. A test against a JavaScript engine finds
   each difference.
5. The regular expressions of Go have no lookahead, no lookbehind and no
   backreference, which JavaScript has. A spec that uses one of them gets
   an error. A regular
   expression can be a literal, such as `/^h/i`, or a call to `regexp`.
6. The tree can be written back as JavaScript, so the browser component runs
   the same expression.

### B. goja

goja is a JavaScript engine in pure Go. `vegagoja` and `echartsgoja` used it
to run all of vega.js and ECharts.

1. It gives the rules of JavaScript exactly, with no work.
2. It can run any script, not only the expression language.
3. It is large. `usql` measured goja at 1.69 MB, and the collation tables of
   `golang.org/x/text` that come with it at 1.27 MB (usql backlog W13).
4. A call from Go into goja for each row converts each value, which costs
   more than a closure.
5. A script from an untrusted source must lose the globals of the engine,
   and a timeout must stop it with `Interrupt`.

### C. Both, with A as the default

zuzu evaluates every expression with A. A small interface sits between the
runtime and the evaluator, so that a second implementation can stand in.

goja has two uses behind that interface:

1. In the test module, goja is an oracle for A. A test generates expressions
   and data, evaluates each one with A and with goja, and fails on any
   difference. This finds the differences in item 4 of A before a user does.
2. Scripting beyond the expression language runs in goja, in an optional
   package. A program that does not import that package does not link goja.

The maintainers chose C on 2026-11-10 (D9).

On 2026-11-12 D33 changed the first use. The oracle of the package `expr`
is vega.js in Node.js, not goja: `test/oracle/exprs.mjs` evaluates the
expressions of `expr/testdata/exprs.txt`, and a test compares A with the
result. The second use stays, and is not built yet: no package runs
scripts in goja.

## The functions of the view

The package `zuzu` gives the functions that read the view: its data, its
scales, its projections and its trees. They follow the documentation of
Vega, and the result of vega.js where the documentation says less. The
test `TestFuncsOracle` compares them with vega.js on the cases of
`testdata/funcs/cases.json`, which `test/oracle/funcs.mjs` evaluates.

zuzu draws a chart once, with no events. Each function reads the state
that the spec gives, and nothing changes it after that. These are the
choices for the functions that need one:

| Function | What zuzu does |
| --- | --- |
| `gradient(scale, p0, p1, count)` | It returns a linear gradient that samples a continuous scale at the ticks of its domain for about `count` values, 15 by default, with the ends of the domain. `p0` and `p1` are `[0, 0]` and `[1, 0]` when they are not given. A scale that is not continuous, such as an ordinal scale, is an issue |
| `copy(name)` | It returns a copy of a scale or a projection, or undefined for a name that is neither. The functions of projections take the copy of a projection in place of its name. The functions of scales find no scale in the copy of a scale, as in vega.js, so `scale` gives undefined and `domain` gives `[]` |
| `scale(name, value)` | The name can be a projection, or a copy of one. It maps `[longitude, latitude]` to `[x, y]` |
| `geoArea`, `geoBounds` and `geoCentroid` | With a projection, they measure in its plane. With null as the projection, they measure on the sphere, as d3-geo does: the area in steradians, the box of longitudes and latitudes, and the centroid in degrees |
| `geoShape(projection, geojson)` | It returns the path of the GeoJSON object in the projection, which a `shape` mark draws as its `shape`. With no projection, the path is empty |
| `treePath(name, source, target)` and `treeAncestors(name, node)` | They read the tree that a `stratify` transform of the dataset built, or of the dataset that it derives from. A node is the row with that key, after every transform of the dataset. A dataset with no tree, or a key that the tree does not have, gives undefined. zuzu has no `nest` transform |
| `vlSelectionTest(store, datum, op)` and `vlSelectionIdTest(store, datum, op)` | They report whether the datum matches an entry of the store, or, with `intersect`, an entry of each unit. A store holds only the entries that the spec gives it, such as the `value` of a param of Vega-Lite. An empty store gives 0, which is false. The `empty` of Vega-Lite is in the expression that calls the function, such as `!length(data('brush_store')) \|\| vlSelectionTest('brush_store', datum)` |
| `vlSelectionResolve(store, op, multi, vl5)` | It returns the values of each field of the store, joined by `op`. A store of ids gives its ids as a list, where vega.js gives a `Set` |
| `screen()` and `windowSize()` | A static chart has no window, so a call is an issue |

## Extension from Go

A Go program can add a function to the library, such as a function that
looks up a value in its own database. The function is a Go value that the
caller gives as an option, and the expression calls it by name. It is part of
the configuration that the caller owns, and no package level registry holds
it.

## Formats

An expression and an axis both format numbers and times. Vega uses
d3-format, such as `,.2f`, and d3-time-format, such as `%Y-%m-%d`. The
package `format` implements both, and the expression functions `format` and
`timeFormat` call it. The rules of d3 for locale, rounding and the SI prefix
are exact, so the tests compare the output with d3 on the same inputs.

## Errors

A parse error names the expression, the position and what the parser
expected. An error during evaluation names the expression. An
expression that calls a function that zuzu does not have is an error at
parse time, and never a value of `undefined` at run time.
