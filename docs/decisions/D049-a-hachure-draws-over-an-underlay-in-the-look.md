# D49. A hachure draws over an underlay in the look

Status: Decided. Amends D40 and D47. Amended by D50.

On 2026-11-22 the maintainers saw that in the look like Excalidraw a mark
under a hachure showed through its gaps, such as the blue bar under the
red part of a bar of `layer_bar_annotations.vl.json` of Vega-Lite, so the
chart read wrongly.

1. In a look drawn by hand, each item whose fill draws as a hachure first
   draws an underlay: a solid fill in the color of the background of the
   canvas, in the shape of the item, drawn by rough.js with the seed of
   the item, so it wobbles as the item does. Then come its hachure and
   its outline. So a mark hides what is under it, as in the standard look.
   Over the background, the underlay does not show.
2. The underlay takes the opacity of the fill: the alpha of its color, or
   the largest alpha of the stops of its gradient, times its fill
   opacity, and the item keeps its opacity. So a fill that lets a mark
   show through in the standard look lets it show through as much.
3. A fill that draws solid takes no underlay. A canvas with no opaque
   background draws none, because no color hides what is under the item.
4. The underlay is a path of the scene, so the SVG writer and raster draw
   the same. It makes the SVG of the look about 4% larger.
5. A shape shorter than 10 fills solid, with no hachure, because a
   hachure has room for only one to four short lines in it, which draw a
   dot as a dash. This amends D47 part 3.
6. The underlay of a path of several rings, such as a band of a contour
   with islands, fills the path as it is, with no wobble, as the standard
   look fills it. The hachure joins the rings through bridges that add no
   area, but rough.js wobbles the two lines of each bridge apart, so a
   solid fill of rough.js draws a sliver along each bridge, as in
   `vega/annual-precipitation.vg.json`. The tint of D50 does the same.
