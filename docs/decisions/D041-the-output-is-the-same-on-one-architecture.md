# D41. The output is the same on one architecture

Status: Decided. Amends D39 and D40.

On 2026-11-22 the copy of rough-go showed that the functions `Sin`, `Cos`,
`Tan`, `Atan`, `Asin` and `Atan2` of the package `math` of Go give other
bits on arm64 and on amd64 v3 than on amd64 v1. The Go compiler fuses a
multiply and an add into one instruction inside the package `math`, and
s390x runs assembly. So a shape of rough-go, and any part of zuzu that calls
those functions, can give other bytes on another machine. A copy of the six
functions of Go, with each fusion stopped, gave the same bytes on seven
architectures and on TinyGo. The maintainers answered three questions on
it.

## The answers

1. zuzu does not copy the functions of the package `math` of Go. The copy
   of rough-go calls `math`, as rough-go does.
2. Hard rule 6 holds on one architecture. The same spec, data, theme,
   fonts and time zone give the same bytes on the same architecture. zuzu
   does not promise the same bytes on another architecture, or on amd64
   with another level of `GOAMD64`. The copy of rough-go keeps no change
   that stops a fused multiply-add, so that it stays close to its upstream.
   This amends part 13 and part 15 of D39.
3. A golden file of a copy holds the output of amd64 v1, and its test runs
   only there. The golden file of `internal/rough` keeps each path in full,
   680 KB, so that each change shows in its diff.
4. The job of CI on arm64 stays. It runs the tests of the root module on
   arm64, with the golden tests of the copies skipped. This amends part 10
   of D40.
