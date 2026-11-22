// projections-tier1.mjs writes the numbers of the projections of tier 1 of
// D52 that the tests of the package geo compare with. The projections come
// from d3-geo-projection, which vega-projection-extended registers with
// vega.js. Run it from the test directory, after npm install in oracle:
//
//	node oracle/projections-tier1.mjs ../geo/testdata
//
// It writes projections-tier1.json. For each projection, at its defaults,
// with a rotation, and with other values of its parameter if it has one,
// the file holds the x and y of a fixed set of points, and the area and the
// bounds of the path of the sphere and of the countries of world-110m.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { geoPath } from 'd3-geo';
import * as d3gp from 'd3-geo-projection';
import { feature } from 'topojson-client';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/projections-tier1.mjs <dir>');
  process.exit(2);
}

// types maps the name of a projection in a spec to its function.
const types = {
  hammer: d3gp.geoHammer,
  wagner6: d3gp.geoWagner6,
  winkel3: d3gp.geoWinkel3,
  eckert1: d3gp.geoEckert1,
  collignon: d3gp.geoCollignon,
  baker: d3gp.geoBaker,
  littrow: d3gp.geoLittrow,
  bottomley: d3gp.geoBottomley,
  wiechel: d3gp.geoWiechel,
  airy: d3gp.geoAiry,
};

// The points: a grid, the poles, and points near the antimeridian.
const points = [];
for (let lat = -90; lat <= 90; lat += 30) {
  for (let lon = -180; lon <= 180; lon += 30) points.push([lon, lat]);
}
for (const lat of [-89.999, -60, -12.5, 0, 33.3, 75, 89.999]) {
  for (const lon of [-179.999, -179.5, 179.5, 179.999]) points.push([lon, lat]);
}
points.push([0, 90], [0, -90], [-73.98, 40.75], [139.69, 35.69], [-58.4, -34.6], [151.2, -33.9], [5, 1], [-1, -2]);

const world = JSON.parse(readFileSync('testdata/corpus/data/world-110m.json', 'utf8'));
const countries = feature(world, world.objects.countries);
const sphere = { type: 'Sphere' };

const finite = (v) => (Number.isFinite(v) ? v : null);
const measure = (p, o) => {
  const path = geoPath(p);
  return { area: path.area(o), bounds: path.bounds(o).map((c) => c.map(finite)) };
};

const cases = [];
for (const [type, fn] of Object.entries(types)) {
  for (const rotate of [null, [20, -30, 10]]) {
    const p = fn();
    if (rotate) p.rotate(rotate);
    const xy = points.map((pt) => {
      const r = p(pt);
      return r && Number.isFinite(r[0]) && Number.isFinite(r[1]) ? r : null;
    });
    cases.push({ type, rotate, xy, sphere: measure(p, sphere), world: measure(p, countries) });
  }
}
// The parameters of hammer, bottomley and airy, which a spec of Vega can
// set as a property of the projection.
const params = [
  ['hammer', 'coefficient', 3],
  ['hammer', 'coefficient', 1.5],
  ['bottomley', 'fraction', 0.3],
  ['bottomley', 'fraction', 1],
  ['airy', 'radius', 60],
  ['airy', 'radius', 120],
];
for (const [type, name, value] of params) {
  const p = types[type]()[name](value);
  const xy = points.map((pt) => {
    const r = p(pt);
    return r && Number.isFinite(r[0]) && Number.isFinite(r[1]) ? r : null;
  });
  cases.push({ type, param: { [name]: value }, xy, sphere: measure(p, sphere), world: measure(p, countries) });
}

writeFileSync(join(out, 'projections-tier1.json'), JSON.stringify({ points, cases }) + '\n');
