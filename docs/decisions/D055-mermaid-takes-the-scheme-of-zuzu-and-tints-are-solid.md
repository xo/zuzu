# D55. Mermaid takes the scheme of zuzu, and tints are solid

Status: Decided. Amends D31, D37, D53 and D54. Amended by D56 and D59.

On 2026-11-22 the maintainers saw that many colors of the diagrams of
Mermaid were off: their default color was the pink of
`mermaid-redux-color`, where a chart of Vega and a graph of DOT take the
blue of the scheme of zuzu, and the tints of the nodes were not solid, so
a node in a flow of an agentflow diagram, such as
`diagrams/100-agentflow-flows.mmd`, mixed its tint with the tint of the
flow behind it.

1. A diagram of Mermaid whose source names no theme takes the scheme of
   zuzu, as a chart and a graph of DOT do, not the scheme
   `mermaid-redux-color`. This replaces D37 part 2. A theme that the
   source names still selects its scheme or its mode (D32 part 4).
2. A diagram whose source names no theme still counts as one in the theme
   `redux-color` for D53 and D54, so its nodes take their colors by kind
   and its flows and composite states take theirs in turn, from the
   categorical colors of the scheme of zuzu.
3. A tint is solid: the color of a node, of a note and of a panel with a
   color, such as a flow or a composite state, mixed into the background at
   `NodeTint`, with no opacity. So a node over a panel keeps its tint, and
   a panel in a panel keeps its own. The outline keeps the full color.
4. This holds for every graph, of DOT and of Mermaid, and for the nodes
   that the charts of Mermaid draw as a node of a graph.
5. A box of a chart of Mermaid that takes a color by its kind fills with
   the solid tint of its color and keeps the outline in the full color:
   an element of a C4 diagram, a box of an event model, a node of a
   mindmap, a box of a railroad, a participant of a sequence, the label of
   a branch of a gitgraph, an item of a Cynefin framework and a column of
   a kanban. A band that draws at a low opacity over an opaque back, such
   as a section of a timeline or an activation of a sequence, keeps its
   opacity, because it reads as a solid tint already.
6. A node that marks a point of the flow is solid: it fills with the color
   of its outline, not with a tint. These are the start and the end of a
   state diagram, a fork and a join bar, and a junction point of a
   flowchart. A fill or a stroke that the source gives wins. A start of a
   state diagram that points to one state takes the color of that state:
   the stroke of its style, or else its fill, or else its color, which is
   the color of a composite state. An end that one state ends in takes the
   color of that state in the same way. A start that points to more than
   one state, and an end that more than one state ends in, keep the color
   of a node. The look like
   Excalidraw fills such a node solid, not with a hachure, which draws a
   dot of 14 pixels as a pale scribble. The look keeps each ring of a path
   where its fill changes under the rule nonzero, so the dot of the end of
   a state diagram, inside the hole of its ring, stays filled.
7. In `redux-color` or `redux-dark-color`, and so by default, each
   subgraph of a flowchart and each lane of a swimlane takes the
   categorical colors in turn, in the order in which they open, as Mermaid
   colors them. The subgraphs, the lanes and the composite states of D54
   start at the second color and take the second to the tenth in turn, so
   a panel never takes the first color, the color of the nodes in it. This
   amends D54 part 2. The flows of an agentflow diagram keep D53 part 2.
