# D47. A gradient keeps its colors and a small shape wobbles less in the look

Status: Decided. Amends D40 and D42. Amended by D49 and D50.

On 2026-11-22 the maintainers saw that the look like Excalidraw lost the
colors of a gradient, such as the legend of a continuous color scale, and
drew a small point or tick as a scribble.

1. In a look drawn by hand, a fill or a stroke with a gradient keeps the
   gradient. The lines of the hachure of a fill take the gradient of the
   fill, and the outline takes the gradient of the stroke. Each renderer
   spans the gradient over the bounds of the path that it paints, as SVG
   does by default. The lines of a hachure reach from edge to edge of the
   shape, so each color falls where the standard look draws it. The scene
   holds the gradient on each path, so the SVG writer and raster draw the
   same. The report names no gradient, because nothing is dropped. This
   replaces D40 part 8.
2. The gradient that the standard look gives an area still draws as one
   tint, the color of the area at the opacity of its top, because
   Excalidraw fills a shape with one tint. D42 part 5 stays, with this
   reason.
3. A small shape draws with less roughness, as Excalidraw does for a small
   element. A shape keeps its roughness when it is at least 20 by 50, a
   round shape when it is at least 15 by 15, and a line, or a path with
   no fill, when it is at least 150 long. Each other shape takes half of
   the roughness, or a third when it is shorter than 10. An edge of a
   graph keeps the roughness of D42 part 8. This amends D40 part 5.
