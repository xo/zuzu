// funcs-cases.mjs writes the cases that funcs.mjs evaluates: the functions
// of expressions that read the view, with the GeoJSON objects, the scales,
// the trees and the stores of selections that they read. Run it from the
// test directory:
//
//	node oracle/funcs-cases.mjs ../testdata/funcs/cases.json
//
// The random shapes come from a generator with a fixed seed, so that the
// cases are the same each time.
import { writeFileSync } from 'node:fs';
const poly = (rings) => ({ type: 'Polygon', coordinates: rings });
const feat = (g) => ({ type: 'Feature', properties: {}, geometry: g });
const geo = {
  sq: feat(poly([[[0, 0], [0, 10], [10, 10], [10, 0], [0, 0]]])),
  ccw: feat(poly([[[0, 0], [10, 0], [10, 10], [0, 10], [0, 0]]])),
  anti: feat(poly([[[170, -10], [170, 10], [-170, 10], [-170, -10], [170, -10]]])),
  north: feat(poly([[[-180, 70], [-90, 70], [0, 70], [90, 70], [180, 70], [-180, 70]]])),
  south: feat(poly([[[180, 70], [90, 70], [0, 70], [-90, 70], [-180, 70], [180, 70]]])),
  hole: feat(poly([[[0, 0], [0, 20], [20, 20], [20, 0], [0, 0]], [[5, 5], [15, 5], [15, 15], [5, 15], [5, 5]]])),
  big: feat(poly([[[-60, -30], [-60, 50], [80, 50], [80, -30], [-60, -30]]])),
  line: feat({ type: 'LineString', coordinates: [[-10, 40], [30, 60], [100, 50]] }),
  lines: { type: 'MultiLineString', coordinates: [[[170, 0], [-170, 10]], [[0, -40], [0, -50]]] },
  dot: { type: 'LineString', coordinates: [[5, 5], [5, 5]] },
  multipoint: { type: 'MultiPoint', coordinates: [[10, 10], [20, -20], [170, 0], [-170, 5]] },
  point: { type: 'Point', coordinates: [30, 40] },
  multi: feat({ type: 'MultiPolygon', coordinates: [[[[0, 0], [0, 10], [10, 10], [10, 0], [0, 0]]], [[[40, 40], [40, 45], [45, 45], [45, 40], [40, 40]]]] }),
  fc: { type: 'FeatureCollection', features: [feat(poly([[[0, 0], [0, 10], [10, 10], [10, 0], [0, 0]]])), feat({ type: 'Point', coordinates: [100, 0] })] },
  gc: { type: 'GeometryCollection', geometries: [{ type: 'Point', coordinates: [1, 2] }, { type: 'LineString', coordinates: [[0, 0], [20, 20]] }] },
  sphere: { type: 'Sphere' },
  empty: { type: 'FeatureCollection', features: [] },
};
const names = Object.keys(geo);
const sphereExprs = [];
for (const n of names) {
  for (const f of ['geoArea', 'geoBounds', 'geoCentroid']) sphereExprs.push(`${f}(null, datum.${n})`);
}
const projected = [];
for (const n of ['sq', 'line', 'point', 'multipoint', 'hole', 'fc']) {
  projected.push(`geoArea('p', datum.${n})`, `geoBounds('p', datum.${n})`, `geoCentroid('p', datum.${n})`, `geoShape('p', datum.${n})`);
}
const proj = [{ name: 'p', type: 'mercator', scale: 100, translate: [300, 200] }, { name: 'q', type: 'orthographic', scale: 120, translate: [150, 150], pointRadius: 2 }];

const scales = [
  { name: 'seq', type: 'linear', domain: [0, 100], range: { scheme: 'blues' } },
  { name: 'lin2', type: 'linear', domain: [0, 100], range: ['red', 'blue'] },
  { name: 'logc', type: 'log', domain: [1, 1000], range: ['white', 'green'] },
  { name: 'div', type: 'linear', domain: [-1, 0, 1], range: { scheme: 'redblue' } },
  { name: 'flat', type: 'linear', domain: [5, 5], range: ['red', 'blue'] },
  { name: 'flatseq', type: 'linear', domain: [5, 5], range: { scheme: 'viridis' } },
  { name: 'tm', type: 'time', domain: [Date.UTC(2020, 0, 1), Date.UTC(2020, 0, 8)], range: ['red', 'blue'] },
  { name: 'root', type: 'sqrt', domain: [0, 100], range: ['black', 'white'] },
  { name: 'rev', type: 'linear', domain: [100, 0], range: ['red', 'blue'] },
  { name: 'x', type: 'linear', domain: [0, 10], range: [0, 100] },
  { name: 'b', type: 'band', domain: ['a', 'b', 'c'], range: [0, 90] },
];

const nodes = [
  { id: 'a' }, { id: 'b', parent: 'a' }, { id: 'c', parent: 'a' },
  { id: 'd', parent: 'b' }, { id: 'e', parent: 'b' }, { id: 'f', parent: 'c' },
];
const strat = { type: 'stratify', key: 'id', parentKey: 'parent' };

const E = (field, type, value, unit = 'u') => ({ unit, fields: [{ field, channel: 'x', type }], values: [value] });
const typed = {
  E: E('a', 'E', 2), Earray: E('a', 'E', [1, 2, 3]), Emiss: E('a', 'E', '2'),
  R: E('a', 'R', [0, 2]), Rrev: E('a', 'R', [5, 2]), RRE: E('a', 'R-RE', [0, 2]),
  RE: E('a', 'R-E', [2, 5]), RLE: E('a', 'R-LE', [1, 2]),
  LT: E('a', 'E-LT', 2), LTE: E('a', 'E-LTE', 2), GT: E('a', 'E-GT', 1), GTE: E('a', 'E-GTE', 2),
  VALID: E('a', 'E-VALID', null), INVALID: E('b', 'E-VALID', null), ONE: E('a', 'E-ONE', [4, 2]),
  nested: E('o.x', 'E', 1), str: E('b', 'R', ['w', 'y']),
};
const selData = Object.entries(typed).map(([k, v]) => ({ name: `t_${k}`, values: [v] }));
selData.push(
  { name: 's_e', values: [
    { unit: 'u1', fields: [{ field: 'a', channel: 'x', type: 'E' }, { field: 'b', channel: 'y', type: 'E' }], values: [2, 'x'] },
    { unit: 'u2', fields: [{ field: 'a', channel: 'x', type: 'E' }], values: [5] },
  ] },
  { name: 's_miss', values: [
    { unit: 'u1', fields: [{ field: 'a', type: 'E' }], values: [7] },
    { unit: 'u1', fields: [{ field: 'a', type: 'E' }], values: [2] },
    { unit: 'u2', fields: [{ field: 'b', type: 'E' }], values: ['x'] },
  ] },
  { name: 's_empty', values: [] },
  { name: 's_id', values: [{ unit: 'u1', _vgsid_: 1 }, { unit: 'u1', _vgsid_: 3 }, { unit: 'u2', _vgsid_: 3 }, { unit: 'u2', _vgsid_: 4 }] },
  { name: 's_id1', values: [{ unit: 'u1', _vgsid_: 3 }] },
  { name: 's_multi', values: [
    { unit: 'u1', fields: [{ field: 'a', type: 'E' }], values: [1] },
    { unit: 'u1', fields: [{ field: 'a', type: 'E' }], values: [2] },
    { unit: 'u2', fields: [{ field: 'a', type: 'E' }], values: [2] },
    { unit: 'u2', fields: [{ field: 'a', type: 'E' }], values: [3] },
  ] },
  { name: 's_int', values: [
    { unit: 'u1', fields: [{ field: 'a', type: 'R' }, { field: 'c', type: 'R' }], values: [[0, 5], [10, 1]] },
    { unit: 'u2', fields: [{ field: 'a', type: 'R' }], values: [[3, 8]] },
    { unit: 'u3', fields: [{ field: 'a', type: 'R-RE' }], values: [[4, 9]] },
  ] },
  { name: 's_apart', values: [
    { unit: 'u1', fields: [{ field: 'a', type: 'R' }], values: [[0, 1]] },
    { unit: 'u2', fields: [{ field: 'a', type: 'R' }], values: [[3, 8]] },
  ] },
);
const selExprs = [
  ...Object.keys(typed).map((k) => `vlSelectionTest('t_${k}', datum)`),
  "vlSelectionTest('s_e', datum)", "vlSelectionTest('s_e', datum, 'intersect')",
  "vlSelectionTest('s_e', {a: 5})", "vlSelectionTest('s_e', {a: 5}, 'intersect')",
  "vlSelectionTest('s_miss', datum)", "vlSelectionTest('s_miss', datum, 'intersect')",
  "vlSelectionTest('s_miss', {a: 2}, 'intersect')", "vlSelectionTest('s_miss', {a: 9})",
  "vlSelectionTest('s_empty', datum)", "vlSelectionTest('s_empty', datum, 'intersect')",
  "!length(data('s_empty')) || vlSelectionTest('s_empty', datum)",
  "length(data('s_empty')) && vlSelectionTest('s_empty', datum)",
  "vlSelectionIdTest('s_id', datum)", "vlSelectionIdTest('s_id', datum, 'intersect')",
  "vlSelectionIdTest('s_id', {_vgsid_: 1}, 'intersect')", "vlSelectionIdTest('s_id', {_vgsid_: 1})",
  "vlSelectionIdTest('s_id', {_vgsid_: 2})", "vlSelectionIdTest('s_id1', datum, 'intersect')",
  "vlSelectionIdTest('s_empty', datum)",
  "vlSelectionResolve('s_multi')", "vlSelectionResolve('s_multi', 'intersect')",
  "vlSelectionResolve('s_multi', 'union', true)", "vlSelectionResolve('s_multi', 'intersect', true, true)",
  "vlSelectionResolve('s_e', 'union', true, true)", "vlSelectionResolve('s_e', 'intersect')",
  "vlSelectionResolve('s_int')", "vlSelectionResolve('s_int', 'intersect')",
  "vlSelectionResolve('s_apart', 'intersect')", "vlSelectionResolve('t_Rrev')",
  "vlSelectionResolve('s_id')", "vlSelectionResolve('s_id', 'intersect', true, true)",
  "vlSelectionResolve('s_id1', 'union', true)",
  "vlSelectionResolve('s_empty')", "vlSelectionResolve('s_empty', 'union', true, true)",
];

// Random shapes, from a seeded generator, so that the cases stay the same.
let seed = 20261125;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const r3 = (x) => Math.round(x * 1000) / 1000;
const wrapLon = (x) => r3(((x + 540) % 360) - 180);
const ring = (clon, clat, r, n, cw) => {
  const pts = [];
  for (let i = 0; i < n; i++) {
    const a = (cw ? -1 : 1) * (2 * Math.PI * i) / n + rnd() * 0.3;
    const rr = r * (0.6 + 0.4 * rnd());
    pts.push([wrapLon(clon + rr * Math.cos(a)), r3(Math.max(-89, Math.min(89, clat + rr * Math.sin(a))))]);
  }
  pts.push(pts[0]);
  return pts;
};
const randomGeo = {};
let k = 0;
for (let i = 0; i < 40; i++) {
  const cw = i % 4 !== 0;
  randomGeo[`r${k++}`] = poly([ring(rnd() * 360 - 180, rnd() * 160 - 80, 1 + rnd() * 40, 3 + Math.floor(rnd() * 6), cw)]);
}
for (let i = 0; i < 10; i++) {
  randomGeo[`r${k++}`] = poly([ring(175 + rnd() * 10, rnd() * 120 - 60, 2 + rnd() * 20, 4 + Math.floor(rnd() * 4), true)]);
}
for (let i = 0; i < 20; i++) {
  const n = 2 + Math.floor(rnd() * 4);
  const pts = [];
  for (let j = 0; j < n; j++) pts.push([r3(rnd() * 360 - 180), r3(rnd() * 170 - 85)]);
  randomGeo[`r${k++}`] = { type: 'LineString', coordinates: pts };
}
for (let i = 0; i < 15; i++) {
  const n = 1 + Math.floor(rnd() * 6);
  const pts = [];
  for (let j = 0; j < n; j++) pts.push([r3(rnd() * 360 - 180), r3(rnd() * 170 - 85)]);
  randomGeo[`r${k++}`] = { type: 'MultiPoint', coordinates: pts };
}
for (let i = 0; i < 6; i++) {
  const lat = (i % 2 ? -1 : 1) * (50 + rnd() * 35);
  const pts = [];
  for (let j = 0; j <= 8; j++) pts.push([wrapLon(-180 + j * 45 * (i < 3 ? 1 : -1) + (j % 8 ? rnd() * 10 : 0)), r3(lat + rnd() * 3)]);
  pts[8] = pts[0];
  randomGeo[`r${k++}`] = poly([pts]);
}
const randomExprs = [];
for (const n of Object.keys(randomGeo)) {
  for (const f of ['geoArea', 'geoBounds', 'geoCentroid']) randomExprs.push(`${f}(null, datum.${n})`);
}

const cases = [
  { name: 'sphere', spec: {}, datum: geo, exprs: sphereExprs },
  { name: 'sphere-random', spec: {}, datum: randomGeo, exprs: randomExprs },
  { name: 'projected', spec: { projections: proj }, datum: geo, exprs: [
    ...projected,
    "geoShape('q', datum.point)", "geoShape('q', datum.sq)", "geoShape(null, datum.sq)",
    "copy('p')", "geoArea(copy('p'), datum.sq)", "geoCentroid(copy('q'), datum.sq)", "geoShape(copy('p'), datum.sq)", "scale(copy('p'), [10, 10])", "scale('p', [10, 10])", "scale('q', [10, 10])", "geoArea('p', datum.sphere)",
  ] },
  { name: 'gradient', spec: { scales }, datum: {}, exprs: [
    "gradient('seq', [0, 0], [1, 0])", "gradient('seq')", "gradient('lin2', [0, 1], [0, 0], 5)",
    "gradient('logc', null, null)", "gradient('div', [0, 0], [1, 0], 4)", "gradient('flat', [0, 0], [1, 0])",
    "gradient('flatseq', [0, 0], [1, 0], 3)", "gradient('tm', [0, 0], [1, 0], 4)",
    "gradient('root', [0, 0], [1, 0], 5)", "gradient('rev', [0, 0], [1, 0], 4)",
    "gradient('lin2', [0.5], [1, 1], 2)", "gradient(copy('lin2'), [0, 0], [1, 0], 2)",
  ] },
  { name: 'copy', spec: { scales }, datum: {}, exprs: [
    "copy('lin2')", "copy('x')", "scale(copy('x'), 5)", "invert(copy('x'), 50)", "domain(copy('seq'))",
    "range(copy('lin2'))", "bandwidth(copy('b'))", "scale(copy('b'), 'b')", "copy(copy('x'))", "copy('nope')", "copy(null)",
  ] },
  { name: 'tree', spec: { data: [
    { name: 'tree', values: nodes, transform: [strat] },
    { name: 'nums', values: [{ k: 1 }, { k: 2, p: 1 }, { k: 3, p: 1 }, { k: 4, p: 2 }], transform: [{ type: 'stratify', key: 'k', parentKey: 'p' }] },
    { name: 'plain', values: nodes },
    { name: 'derived', source: 'tree' },
    { name: 'laid', values: nodes, transform: [strat, { type: 'tree', size: [100, 100], as: ['tx', 'ty', 'tdepth', 'tchildren'] }] },
  ] }, datum: {}, exprs: [
    "treePath('tree', 'd', 'f')", "treePath('tree', 'd', 'e')", "treePath('tree', 'd', 'd')",
    "treePath('tree', 'a', 'f')", "treePath('tree', 'f', 'a')", "treePath('tree', 'd', 'zz')",
    "treeAncestors('tree', 'e')", "treeAncestors('tree', 'a')", "treeAncestors('tree', 'zz')",
    "treePath('nums', 4, '3')", "treeAncestors('nums', '4')", "treeAncestors('plain', 'a')",
    "treePath('derived', 'd', 'f')", "treeAncestors('laid', 'f')",
  ] },
  { name: 'selection', spec: { data: selData }, datum: { a: 2, b: 'x', o: { x: 1 }, _vgsid_: 3 }, exprs: selExprs },
];
writeFileSync(process.argv[2], JSON.stringify(cases, null, 1) + '\n');
