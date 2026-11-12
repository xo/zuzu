# D9. Expressions run in a native evaluator, and goja is optional

Status: Decided. Amended by D33.

Vega has an expression language, a subset of JavaScript expressions. The
maintainers chose on 2026-11-10 choice C of `docs/EXPRESSIONS.md`, which also
answered open question 1:

1. A native evaluator runs every expression. zuzu parses the language with a
   parser written by hand, compiles the tree into Go closures, and has its own
   library of the functions of Vega. A JavaScript engine is not needed for it.
2. An interface stands between the runtime and the evaluator.
3. goja, a JavaScript engine in pure Go, stands behind the same interface
   twice. In the test module, it is an oracle that finds each difference
   between the evaluator and JavaScript. In an optional package, it runs
   scripts beyond the expression language. A program that does not import
   that package does not link goja.

Gemini, Astra and DeepSeek each proposed a native evaluator on 2026-11-10.
`docs/EXPRESSIONS.md` holds the three choices and the reasons.
