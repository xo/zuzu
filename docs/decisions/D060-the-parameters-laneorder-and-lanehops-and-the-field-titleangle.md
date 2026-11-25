# D60. The parameters laneOrder and laneHops, and the field titleAngle

Status: Decided. Amends D31.

On 2026-11-26 the maintainers answered open question 37. They accept the
parameters that a swimlane diagram of Mermaid 12.3 needs, as D35 accepted
`lanes` and `laneEdges` for open question 29.

1. With `lanes`, the parameter `laneOrder` of the transform `zuzu.layered`
   orders the lanes as the setting `swimlane.automaticLaneOrdering` of
   Mermaid 12.3 does. It is false by default.
2. The parameter `laneHops` of `zuzu.layered`, `arc` or `gap`, draws a hop
   where two routes cross, as the setting `swimlane.lineHops` of Mermaid
   does. It is empty by default, and an empty value draws no hop.
3. The field `titleAngle` of a row of a cluster turns the title of the
   cluster around its place, in degrees. A lane takes -90 when the ranks
   run from the left to the right.

"Swimlane diagram" in `docs/DIAGRAMS.md` says how a swimlane diagram uses
them.
