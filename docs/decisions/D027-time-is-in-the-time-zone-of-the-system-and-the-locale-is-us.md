# D27. Time is in the time zone of the system, and the locale is US English

Status: Decided.

The maintainers decided on 2026-11-10 that a time scale, a time format and
the time functions of an expression use the time zone of the system by
default, which is `time.Local` in Go, and that the caller can set another
`*time.Location`. Vega uses the time zone of the browser in the same way. The
default locale of the formats of numbers and dates is US English, and the
caller can set another.

A test that must give the same bytes on every machine sets the time zone.
Under WebAssembly, `time.Local` is often UTC, so a program there sets the
time zone that it wants (D29). The locales are tables of zuzu in the package
`format`, and not `golang.org/x/text`, which is large.
