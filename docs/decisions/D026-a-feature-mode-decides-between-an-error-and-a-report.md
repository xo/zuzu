# D26. A feature mode decides between an error and a report

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu has a feature mode, strict or
loose, for a feature of a spec that zuzu does not support.

1. In the strict mode, the first unsupported feature is an error, and zuzu
   draws nothing.
2. In the loose mode, zuzu skips each unsupported feature, draws the rest, and
   collects each skipped feature in a report that it returns with the
   result.

A malformed spec is an error in both modes. This makes D6 exact: zuzu never
drops a feature without a word, because the strict mode stops and the loose
mode reports. Each entry of the report is an error that wraps a sentinel, so
a caller tests it with `errors.Is` (D24).
