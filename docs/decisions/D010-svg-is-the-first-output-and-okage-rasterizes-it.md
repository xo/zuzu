# D10. SVG is the first output, and okage rasterizes it

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu writes SVG first, and that
the package `svg` of `github.com/xo/okage` turns the SVG into an image. okage
is the pure Go port of resvg and tiny-skia.

The SVG writer is deterministic: the same scene graph and the same fonts give
the same bytes. zuzu makes one `svg.Renderer` or one `svg.FontDatabase` and
shares it, because okage loads the fonts of the system on the first call
(okage D36).
