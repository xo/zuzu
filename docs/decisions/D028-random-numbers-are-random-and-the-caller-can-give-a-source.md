# D28. Random numbers are random, and the caller can give a source

Status: Decided.

Vega has a function `random()` in its expression language, and transforms
such as `sample` use random numbers. A spec cannot set a seed. vega.js sends
every random number through one function that calls `Math.random` of
JavaScript by default, and has a global `setRandom` that a test can replace.

The maintainers decided on 2026-11-10 that zuzu does not copy the random
numbers of JavaScript. A spec that asks for random numbers gets real random
numbers, from a source seeded from the system for each render. A caller can
give a function that makes a source, for a test that needs the same numbers
each time. The function makes one source for each render, so that two
renders never share one, and nothing is global as `setRandom` is.

This amends the rule that output is deterministic: the same spec, data,
theme, fonts and time zone give the same bytes, except where the spec asks
for random numbers and the caller gives no source.
