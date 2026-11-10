# D13. Target SVGs set the look before any code

Status: Decided.

The maintainers asked on 2026-11-10 for sample SVGs of the charts that zuzu
draws in the end, before any code, to set the goal of the look (D12). The
targets can change later.

The script in `docs/targets/` makes them: one SVG in the light mode and one in
the dark mode for each chart of tiers 0 to 3 in `docs/PLAN.md`, some charts in
each color scheme (D14, D15), all drawn from real data, and a gallery page
that shows them. The maintainers chose on 2026-11-10 not to commit the output
for now, so a person runs the script to see it. A script generates them with
the math of d3, such as the scales, the shapes and the layouts, and writes
each SVG element by hand. The script is a tool for the design. zuzu does not
use it.

Each target uses only the parts of SVG that okage renders, and each one is
rendered with okage once, so that the targets are a goal that the renderer
can reach.
