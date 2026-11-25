// projections-details.mjs prints what d3-geo-projection draws for three
// details of the projections of D52, as the Go literals of the tests in
// geo/details_test.go. Run it from the test directory:
//
//	node oracle/projections-details.mjs
//
// It prints the area and the bounds that geoPath gives for these objects:
//
//   - A line and a polygon on the meridians at -180° and 180° of each
//     interrupted and polyhedral projection, with no rotation, and a line
//     on the meridian that the default rotation turns to -180°.
//   - The points and the sphere of an interrupted projection whose lobes
//     have latitudes other than 0 and ±90, the sphere with a precision of
//     0.01.
//   - The sphere of armadillo and berghaus with a clip angle.
import * as d3 from 'd3-geo';
import * as d3gp from 'd3-geo-projection';

const f = (v) => String(+v.toPrecision(15));
const detail = (p, o) => {
  const path = d3.geoPath(p);
  const b = path.bounds(o);
  return `detail{${f(path.area(o))}, [2][2]float64{{${f(b[0][0])}, ${f(b[0][1])}}, {${f(b[1][0])}, ${f(b[1][1])}}}}`;
};
const sphere = { type: 'Sphere' };
const line = (lon) => ({ type: 'LineString', coordinates: [[lon, -60], [lon, 0], [lon, 60]] });
const box = (lon0, lon1) => ({
  type: 'Polygon',
  coordinates: [[[lon0, -30], [lon0, 30], [lon1, 30], [lon1, -30], [lon0, -30]]],
});

console.log('// TestLobeMeridians');
const lobed = {
  interruptedSinusoidal: d3gp.geoInterruptedSinusoidal,
  interruptedMollweide: d3gp.geoInterruptedMollweide,
  interruptedMollweideHemispheres: d3gp.geoInterruptedMollweideHemispheres,
  polyhedralButterfly: d3gp.geoPolyhedralButterfly,
};
for (const [type, make] of Object.entries(lobed)) {
  const p = make().rotate([0, 0]);
  const r = make().rotate()[0];
  console.log(`{"${type}", ${f(r)}, [5]detail{`);
  for (const o of [line(-180), line(180), box(-180, -170), box(170, 180)]) console.log(`\t${detail(p, o)},`);
  console.log(`\t${detail(make(), line(-180 - r))},`);
  console.log('}},');
}

console.log('// TestLobeLatitudes');
const lobes = {
  interruptedSinusoidal: [d3gp.geoInterruptedSinusoidal, [
    [[[-180, 15], [-100, 80], [-40, 10]], [[-40, 10], [30, 90], [180, 15]]],
    [[[-180, -20], [-150, -90], [-90, -5]], [[-90, -5], [0, -70], [60, -30]], [[60, -30], [120, -90], [180, -20]]],
  ]],
  interruptedMollweide: [d3gp.geoInterruptedMollweide, [
    [[[-180, 30], [-90, 90], [0, 30]], [[0, 30], [90, 90], [180, 30]]],
    [[[-180, 0], [-90, -60], [0, 0]], [[0, 0], [90, -90], [180, 0]]],
  ]],
};
const points = [
  [-170, 5], [-170, 40], [-60, 12], [0, 8], [0, 50], [150, 14], [-170, -10],
  [-170, -60], [-50, -3], [30, -50], [100, -25], [100, -70], [0, 0],
];
for (const [type, [make, l]] of Object.entries(lobes)) {
  for (const rotate of [[0, 0, 0], [20, -30, 10]]) {
    const p = make().lobes(l).rotate(rotate);
    const s = detail(make().lobes(l).precision(0.01), sphere);
    console.log(`{"${type}", [3]float64{${rotate.join(', ')}}, ${s}, [][2]float64{`);
    console.log(`\t${points.map((q) => p(q)).map(([x, y]) => `{${f(x)}, ${f(y)}}`).join(', ')},`);
    console.log('}},');
  }
}

console.log('// TestOutlineClipAngle');
const outline = (type, param, angle, p) => {
  const a = angle === null ? 'nil' : `${angle}.0`;
  console.log(`{"${type}", ${param}, ${a}, ${detail(p.clipAngle(angle), sphere)}},`);
};
for (const a of [90, 150, null]) outline('armadillo', 20, a, d3gp.geoArmadillo());
outline('armadillo', 35, 90, d3gp.geoArmadillo().parallel(35));
for (const a of [90, 150]) outline('berghaus', 5, a, d3gp.geoBerghaus());
outline('berghaus', 4, 90, d3gp.geoBerghaus().lobes(4));
for (const n of [4, 6]) outline('berghaus', n, null, d3gp.geoBerghaus().lobes(n));
