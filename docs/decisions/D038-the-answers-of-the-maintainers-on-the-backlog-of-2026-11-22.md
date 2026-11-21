# D38. The answers of the maintainers on the backlog of 2026-11-22

Status: Decided. Amends D24, D28, D31 and D32.

On 2026-11-22 the plan for the easy items of the backlog raised four
questions. The maintainers answered each one. This decision records the
answers.

## The small circles of Mermaid

1. The shapes `f-circ`, `junction`, `sm-circ` and `start` of a flowchart
   draw as small circles of 14 pixels with no text, as in Mermaid 12.3.
   `f-circ` and `junction` are filled, and `sm-circ` and `start` are open,
   as Mermaid draws them in its standard look. A label that the source
   gives to such a node does not draw, and it is no issue, because Mermaid
   does not draw it either. This amends D31.

## The colors of a status

2. A critical task of a gantt and the line of a priority of a kanban card
   take their colors from the scheme, not from fixed colors. A critical task
   and the priority `Very High` take the color of a fall. `High` takes a
   tint of the color of a fall. `Low` takes the first categorical color, and
   `Very Low` a tint of it. So the colors follow each scheme and each mode.
   `docs/STYLE.md` holds the rule.

## The time of the render

3. The new Parse option `zuzu.WithNow(time.Time)` gives the time of the
   render. A Parse option gives it, because a gantt lays out when it
   compiles. The chart keeps it.
4. A gantt with the time draws the marker of today, and a `dateFormat`
   with no date puts its times on the day of that time, as in Mermaid.
   Without the time, a gantt draws no marker and puts such times on
   1970-01-01, as before (D31).
5. The function `now()` of the expression language gives that time.
   Without it, `now()` gives 0, so that the same spec gives the same bytes
   (hard rule 6), and the report names the call. This differs from
   `random()`, which D28 leaves random when the caller gives no source.
   This amends D28.

## The font of the emoji

6. `WithEmoji` is also an option of Parse, as `WithMath` is, and the chart
   keeps the font. A kind of Mermaid that lays out when it compiles, such as
   a sequence, a pie or a mindmap, then measures an emoji with it. The
   option of the render stays, for the graphs and for Vega. When both give
   a font, the option of the render wins. This amends D24 and part 6 of
   D32.
7. So that one function can return an option that both Parse and a render
   accept, `Option` and `RenderOption` become interfaces with an
   unexported method, and the new interface `SharedOption` is both.
   `WithEmoji` returns a `SharedOption`. Each other option keeps its kind,
   so an option in the wrong place still does not compile. This amends
   D24.
