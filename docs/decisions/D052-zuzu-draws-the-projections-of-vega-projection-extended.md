# D52. zuzu draws the projections of vega-projection-extended

Status: Decided. Amends D7.

On 2026-11-22 the maintainers asked zuzu to draw every projection of
`vega/projections.vg.json`. Vega itself has the projections of d3-geo,
which zuzu draws. The 18 others come from vega-projection-extended, an
official package of Vega that registers the projections of
d3-geo-projection with Vega under their names: airy, armadillo, baker,
berghaus, bottomley, collignon, eckert1, guyou, hammer, littrow, wagner6,
wiechel, winkel3, interruptedSinusoidal, interruptedMollweide,
interruptedMollweideHemispheres, polyhedralButterfly and peirceQuincuncial.

1. zuzu reads these 18 names as the type of a projection, with the
   defaults of scale, center, rotate and clip that vega-projection-extended
   gives each. They keep the names of Vega, not a name with `zuzu.`,
   because a spec of Vega that loads the package names them so.
2. zuzu draws each from its published formulas, such as those of Snyder,
   "Map Projections: A Working Manual" (1987), and of the paper that
   describes the projection. No file holds a translation of a source file
   of d3-geo-projection (hard rule 3).
3. The oracle of the test module registers the same projections of
   d3-geo-projection with vega.js, in `test/oracle/projections.mjs`, so
   that conformance and the review page compare zuzu with them.
4. The work comes in three tiers. The first holds the projections with a
   closed formula. The second holds guyou and peirceQuincuncial, which need
   the elliptic functions of a complex argument, armadillo, which clips
   the back of the globe in its own way, and berghaus, a star of lobes. The
   third holds a clip of a polygon on the sphere, which `geo` does not have,
   and the interrupted and polyhedral projections that need it.
