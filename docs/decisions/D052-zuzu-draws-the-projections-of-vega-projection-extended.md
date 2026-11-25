# D52. zuzu draws the projections of vega-projection-extended

Status: Decided. Amends D7. Amended by D58.

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
5. zuzu clips an interrupted or a polyhedral projection to the outline of
   its lobes or faces, so that no line crosses the gap between two of them.
   d3-geo-projection does not clip there. Three details follow
   d3-geo-projection (D58, part 13):
   - A point on a meridian between two lobes or faces goes to the lobe or
     the face that d3-geo-projection gives it. A point on the meridian at
     -180° goes to the west edge of the first lobe or face, and a point on
     180° goes to the east edge of the last. The clip keeps 1e-6 radians
     inside each meridian of a lobe. zuzu moves a point that lies within
     2e-6 radians of a meridian of its lobe to 2e-6 radians inside it. So a
     line or a polygon on such a meridian draws along the edge of its lobe.
   - The parameter `lobes` reads the longitude and the latitude of each of
     the three points of a lobe. The latitude of the west point is where
     the lobe parts from its neighbors. Nearer to the equator, the lobes
     join as one map. The latitude of the central point is where the lobe
     ends. The outline of the sphere follows the three latitudes.
   - `armadillo` and `berghaus` keep the torus and the star as the sphere
     with any `clipAngle`. The clip angle cuts the torus or the star.
