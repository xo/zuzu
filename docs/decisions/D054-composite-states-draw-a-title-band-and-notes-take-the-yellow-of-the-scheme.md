# D54. Composite states draw a title band, and notes take the yellow of the scheme

Status: Decided. Amends D53. Amended by D55.

On 2026-11-22 the maintainers saw that zuzu drew a composite state of a
state diagram, such as those of `diagrams/37-state-composite.mmd`, as a
plain panel, where Mermaid 12.3 draws a box with a band that holds the
title and an outline in a color of its own, and that zuzu drew a note,
such as those of `diagrams/40-state-notes.mmd`, in the color of a node,
where Mermaid draws it yellow.

1. A composite state draws as a panel with a band across its top that
   holds its title. The band takes the paint of the panel again, so it
   reads as a stronger tint, and its lower edge draws a line under the
   title.
2. In a color theme of Mermaid, `redux-color` or `redux-dark-color`, each
   composite state takes the categorical colors of the scheme in turn, in
   the order in which the composite states open, as D53 colors the flows
   of an agentflow diagram.
3. A note of a state diagram, of a class diagram and of a use case diagram
   takes the color of a note of the theme: the categorical color of the
   scheme whose hue is nearest the yellow of a note on paper, 52 degrees.
   So each scheme draws its notes in its own yellow, or the color nearest
   it. A fill or a stroke that the source gives a note wins. A note of DOT
   keeps the color of a node, as Graphviz draws it plain.
