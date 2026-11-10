# D7. The first release supports static Vega and the Vega-Lite that compiles to it

Status: Decided.

All of Vega is large. Its signals react to the events of a mouse or a
keyboard, and its dataflow updates only what changed, which is a program in a
browser. zuzu draws a chart once.

The first release supports static Vega: data, transforms, scales,
projections, marks, axes, legends, titles and signals whose value is a
constant or an expression. Event streams are out of it. So is a selection or
a parameter of Vega-Lite that a person changes. Of the 87 Vega specs in the
corpus, 25 bind an event.

Gemini, Astra and DeepSeek each proposed this on 2026-11-10, and the
maintainers accepted it. The maintainers also decided the same day, answering
open question 5, that signals that bind events can come at a later date, and
not in the first release. The scene graph and the browser component (D19)
keep that door open.
