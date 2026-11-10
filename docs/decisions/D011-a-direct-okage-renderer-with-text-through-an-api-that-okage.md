# D11. A direct okage renderer, with text through an API that okage exposes

Status: Decided.

The maintainers asked on 2026-11-10 for a renderer that draws with the
primitives of okage directly, if that is faster than writing SVG and parsing
it again.

The okage peer measured the round trip on 2026-11-10. For a chart of 300
rectangles and 10 runs of text, the parse took 0.39 ms to 0.89 ms of a total
of 5.9 ms to 8.6 ms. A direct renderer saves about 7 to 10 percent, plus the
time of the SVG writer. `docs/PLAN.md` holds the table.

So the direct renderer is phase 6. It is built when a measurement on the
corpus shows the gain, and its images must match the images through SVG. It
draws with `Pixmap.FillPath`, `Pixmap.StrokePath` and `Pixmap.FillRect` of
the root package of okage.

Text is the hard part. The layout needs the width of each label, and the
direct renderer needs the outlines of the glyphs. The layout and the shaping
of text in okage are private. A first proposal was to parse a small SVG
fragment that holds one `text` element and read the result. The maintainers
rejected it the same day, because a parse of SVG to measure text defeats the
purpose of a direct renderer.

Instead, the okage peer writes the API that zuzu needs, when zuzu needs it: a
way to measure a string in a font and a size, and a way to get the outlines
of its glyphs, with the same fonts, fallback and shaping as the package `svg`
of okage. zuzu does not edit okage. The SVG output comes first, which
answered open question 10. Until the API exists, the measurement sits behind
an interface in the package `text`, and its first implementation estimates a
width from the characters of the string. A cache keyed by the text, the font
and the size holds each result.
