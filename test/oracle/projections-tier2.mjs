// projections-tier2.mjs writes what d3-geo-projection draws for the
// projections guyou, peirceQuincuncial, armadillo and berghaus (D52), for
// the test of these projections in the package geo. Run it from the test
// directory:
//
//	node oracle/projections-tier2.mjs testdata/corpus/data/world-110m.json ../geo/testdata
//
// It writes projections-tier2.json, a list of cases. Each case names a
// projection and the parameters that it sets, and holds:
//
//   - points: a list of [lon, lat, x, y] for a fixed set of points, and
//     for a point where the map is not smooth, such as a cut or a corner of
//     a square, the [x, y] of 8 points around it at a distance of 1e-5°;
//   - sphere: the area and the bounds of the path of the sphere;
//   - countries: the area and the bounds of the path of each country of
//     world-110m, in the order of the file.
//
// A point that the projection cannot place gives null for x and y.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as d3gp from 'd3-geo-projection';
import { geoPath } from 'd3-geo';
import { feature } from 'topojson-client';

const [world, out] = process.argv.slice(2);
if (!world || !out) {
  console.error('usage: node oracle/projections-tier2.mjs <world-110m.json> <dir>');
  process.exit(2);
}
const topo = JSON.parse(readFileSync(world, 'utf8'));
const countries = feature(topo, topo.objects.countries).features;

// The points: a grid of 30°, the poles at several longitudes, and points
// near the antimeridian and near the edges of the hemispheres.
const points = [];
for (let lat = -90; lat <= 90; lat += 30) {
  for (let lon = -180; lon <= 180; lon += 30) points.push([lon, lat]);
}
for (let lat = -75; lat <= 75; lat += 30) {
  for (let lon = -165; lon <= 165; lon += 30) points.push([lon, lat]);
}
for (const lon of [-179.999, -179.9, 179.9, 179.999]) {
  for (const lat of [-80, -45, -10, 0, 10, 45, 80]) points.push([lon, lat]);
}
for (const lon of [-90.001, -89.999, -45, 0.001, -0.001, 45, 89.999, 90.001]) {
  for (const lat of [-89.9, -60, -44.9, -45.1, 20, 44.9, 45.1, 89.9]) points.push([lon, lat]);
}
points.push([12.5, 41.9], [-74, 40.7], [151.2, -33.9], [-58.4, -34.6], [37.6, 55.8]);

// near returns 8 points at a distance of 1e-5° around a point, on the
// sphere, so that a point on a pole has neighbors in every direction.
const near = (lon, lat) => {
  const r = Math.PI / 180;
  const d = 1e-5 * r;
  const l = lon * r;
  const f = lat * r;
  const out = [];
  for (let k = 0; k < 8; k++) {
    const az = ((k + 0.5) * Math.PI) / 4;
    const f2 = Math.asin(Math.sin(f) * Math.cos(d) + Math.cos(f) * Math.sin(d) * Math.cos(az));
    const l2 = l + Math.atan2(Math.sin(az) * Math.sin(d) * Math.cos(f), Math.cos(d) - Math.sin(f) * Math.sin(f2));
    out.push([l2 / r, f2 / r]);
  }
  return out;
};

const cases = [];
const add = (type, make, params) => {
  const p = make();
  const path = geoPath(p);
  const box = (o) => ({ area: path.area(o), bounds: path.bounds(o) });
  cases.push({
    type,
    params,
    points: points.map(([lon, lat]) => {
      const q = p([lon, lat]);
      if (!q || !Number.isFinite(q[0]) || !Number.isFinite(q[1])) return [lon, lat, null, null];
      const around = near(lon, lat).map((r) => p(r));
      const spread = Math.max(...around.map((r) => Math.hypot(r[0] - q[0], r[1] - q[1])));
      return spread > 1e-3 ? [lon, lat, q[0], q[1], around] : [lon, lat, q[0], q[1]];
    }),
    sphere: box({ type: 'Sphere' }),
    countries: countries.map(box),
  });
};

const rotate = [20, -30, 10];
const types = {
  guyou: d3gp.geoGuyou,
  peirceQuincuncial: d3gp.geoPeirceQuincuncial,
  armadillo: d3gp.geoArmadillo,
  berghaus: d3gp.geoBerghaus,
};
for (const [type, fn] of Object.entries(types)) {
  add(type, () => fn(), {});
  add(type, () => fn().rotate(rotate), { rotate });
}
add('armadillo', () => d3gp.geoArmadillo().parallel(35), { parallel: 35 });
add('armadillo', () => d3gp.geoArmadillo().parallel(-20).rotate(rotate), { parallel: -20, rotate });
add('berghaus', () => d3gp.geoBerghaus().lobes(4), { lobes: 4 });
add('berghaus', () => d3gp.geoBerghaus().lobes(7).rotate(rotate), { lobes: 7, rotate });

writeFileSync(join(out, 'projections-tier2.json'), JSON.stringify(cases) + '\n');
