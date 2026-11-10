# D24. The public API

Status: Decided.

The maintainers asked on 2026-11-10 for a draft of the public API, and for
the advice of the flagship models on its shape and on the relation of `vega`
and `vegalite`. `docs/API.md` holds it, with what Astra, Gemini and
DeepSeek proposed.

The maintainers set three conventions the same day, and those are decided:

1. An error that a caller can test is a sentinel error, a constant of a
   defined string type, wrapped with `%w` and tested with `errors.Is`.
2. A call takes functional options, named `With...`, only where it has a
   setting that it can do without.
3. The caller owns the value of each option. No configuration is a package
   level variable.

The maintainers reviewed the rest of `docs/API.md` and accepted it the same
day: the `Chart` and the `View`, the two kinds of option, the loader, the
report, and the types of `vega` and `vegalite`. They also answered its three
questions. Rows that a program gives are `[]map[string]any`, and zuzu also
takes the result of a SQL query as a `ResultSet` that `*sql.Rows` and the
`ResultSet` of tblfmt both satisfy. The form of
`Change` stands for now and can change before phase 8. zuzu reads and writes
JSON with `encoding/json/v2` of the standard library, which TinyGo supports,
and writes no decoder of its own. The API can change as the
code shows what it needs, and a change is a decision.
