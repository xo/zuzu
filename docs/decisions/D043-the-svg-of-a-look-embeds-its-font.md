# D43. The SVG of a look embeds its font

Status: Decided. Amends D10 and D40.

On 2026-11-22 the maintainers asked that the SVG of a look drawn by hand
embed its font, so that a viewer that lacks the font, such as a browser
that shows the SVG in an image of the review page, draws the text in it.
Until then, the SVG named Excalifont and the viewer drew its own font.

1. In a look drawn by hand with a font of `WithLookFont`, `Chart.SVG`
   embeds the font. The SVG holds a style element in its defs with a rule
   `@font-face` that names the family and gives the bytes of the font file
   in a data URL, in base64.
2. The font keeps the format that `text.NewFallback` took, such as WOFF2
   for Excalifont, about 70 KB in base64. `text.Fallback` keeps those bytes,
   and its method `Source` returns them.
3. The SVG writer embeds a font only when a text of the scene names its
   family. The option `svg.WithFont` gives it a font. The writer reads the
   scene and its options, and no spec (hard rule 5).
4. The standard look embeds no font. Its text names IBM Plex Sans, as
   before (D10, D16).
5. The golden files of the look hold the SHA-256 of the font in place of
   its bytes, so that each file stays small.
