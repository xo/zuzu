# D50. A gradient draws as a hachure over a tint in the look

Status: Decided. Amends D47 and D49.

On 2026-11-22 the maintainers saw that in the look like Excalidraw a node
of DOT with a gradient fill, such as the nodes of `dot/44-gradients.dot`,
read as a solid fill with white scratches. The lines of its hachure stood
1.5 apart, and the ends of the lines showed past the outline. The
maintainers chose a hachure over a tint.

1. A shape with a gradient fill that is at least 20 across draws, over its
   underlay, a tint: a solid fill of the same gradient at 0.25 of the
   opacity of the fill, in the shape of the item, with the same wobble.
2. The lines of its hachure take the gradient, as D47 part 1 says, and
   stand 5 apart, the gap of Excalidraw, so that they read as strokes of a
   pencil.
3. The hachure fills the shape inset by 2, so that no end of a line shows
   past the outline. The outline draws around the whole shape.
4. A shape with a gradient that is less than 20 across, such as the bar of
   a legend of a continuous color scale, keeps the hachure of D47, so that
   its colors still read.
5. The lines of the hachure take the width of every hachure, even in the
   box of a cluster, whose lines otherwise touch, so that the tint shows
   between them.
6. A label of a graph that sits on such a shape, with the color of text
   that a graph chooses to read on a fill, takes the color of text that
   reads on the tint over the background, and the halo of a label on a
   busy mark. A label on a shape with a plain fill inside it keeps its
   color, and a color of text that the spec gives stays.
7. A fill of one color is not affected.

`docs/STYLE.md` gives the tokens. This amends D47 part 1 and D49 part 1.
