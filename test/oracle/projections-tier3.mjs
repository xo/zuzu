// projections-tier3.mjs writes what d3-geo-projection gives for the
// interrupted and polyhedral projections of D52, for the tests of the
// package geo. Run it from the test directory:
//
//	node oracle/projections-tier3.mjs ../geo/testdata
//
// It writes projections-tier3.json. For each projection, at its defaults
// and with a rotate of [20, -30, 10], it holds the x and y of a fixed set of
// points, and the area and the bounds that geoPath gives for the sphere,
// for the graticule and for each country of world-110m.
//
// d3 does not cut a line where it crosses from one lobe to the next. It
// draws a straight line across the gap between the lobes, which Vega hides
// when a mark clips to the sphere. A path that strays more than 0.05 pixels
// outside the outline of the sphere is marked gap, and the tests compare
// only that the path of zuzu lies inside its bounds and has no larger area.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';
import * as d3 from 'd3-geo';
import * as d3gp from 'd3-geo-projection';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/projections-tier3.mjs <dir>');
  process.exit(2);
}

const projections = {
  interruptedSinusoidal: d3gp.geoInterruptedSinusoidal,
  interruptedMollweide: d3gp.geoInterruptedMollweide,
  interruptedMollweideHemispheres: d3gp.geoInterruptedMollweideHemispheres,
  polyhedralButterfly: d3gp.geoPolyhedralButterfly,
};

// The points: a grid, the poles, and points on and near the meridians
// where the lobes and the faces meet.
const points = [];
for (let lat = -90; lat <= 90; lat += 15) {
  for (let lon = -180; lon <= 180; lon += 20) points.push([lon, lat]);
}
const cuts = [-180, -160, -110, -100, -90, -60, -40, -20, 0, 20, 30, 40, 80, 90, 110, 140, 180];
for (const lon of cuts) {
  for (const lat of [-89, -10, -0.5, 0.5, 10, 89]) {
    for (const d of [0, -1e-3, 1e-3]) {
      const l = lon + d;
      if (l >= -180 && l <= 180) points.push([l, lat]);
    }
  }
}

const topo = JSON.parse(readFileSync('testdata/corpus/data/world-110m.json', 'utf8'));
const countries = vega.read(topo, { type: 'topojson', feature: 'countries' });
const graticule = d3.geoGraticule()();
const sphere = { type: 'Sphere' };

// The numbers are rounded to 9 decimals, well below the tolerance of the
// tests, to keep the file small.
const round = (v) => Math.round(v * 1e9) / 1e9;
const finite = (v) => (Number.isFinite(v) ? round(v) : null);

// segments returns the segments of the path of an object in the plane.
const segments = (p, o) => {
  const out = [];
  let x0, y0, xs, ys;
  d3.geoPath(p, {
    moveTo(x, y) { x0 = xs = x; y0 = ys = y; },
    lineTo(x, y) { out.push([x0, y0, x, y]); x0 = x; y0 = y; },
    closePath() { out.push([x0, y0, xs, ys]); x0 = xs; y0 = ys; },
    arc() {},
  })(o);
  return out;
};

// inside reports whether the rings of segments hold a point, by the rule
// of even and odd.
const inside = (segs, x, y) => {
  let c = false;
  for (const [x0, y0, x1, y1] of segs) {
    if ((y0 > y) !== (y1 > y) && x < x0 + ((y - y0) * (x1 - x0)) / (y1 - y0)) c = !c;
  }
  return c;
};

// distance returns the distance from a point to the nearest segment.
const distance = (segs, x, y) => {
  let d = Infinity;
  for (const [x0, y0, x1, y1] of segs) {
    const dx = x1 - x0, dy = y1 - y0, l = dx * dx + dy * dy;
    const t = l ? Math.max(0, Math.min(1, ((x - x0) * dx + (y - y0) * dy) / l)) : 0;
    d = Math.min(d, Math.hypot(x - x0 - t * dx, y - y0 - t * dy));
  }
  return d;
};

// crosses reports whether a segment crosses one of segs.
const crosses = (segs, [ax, ay, bx, by]) => {
  const side = (x0, y0, x1, y1, x, y) => (x1 - x0) * (y - y0) - (y1 - y0) * (x - x0);
  for (const [cx, cy, dx, dy] of segs) {
    if ((side(ax, ay, bx, by, cx, cy) > 0) !== (side(ax, ay, bx, by, dx, dy) > 0) &&
        (side(cx, cy, dx, dy, ax, ay) > 0) !== (side(cx, cy, dx, dy, bx, by) > 0)) return true;
  }
  return false;
};

// strays reports whether a segment has a point more than 0.05 pixels
// outside the outline: its middle, or if it crosses the outline, a point
// of it in steps of 0.1 pixels.
const strays = (outline, seg) => {
  const [x0, y0, x1, y1] = seg;
  const out = (x, y) => !inside(outline, x, y) && distance(outline, x, y) > 0.05;
  if (out((x0 + x1) / 2, (y0 + y1) / 2)) return true;
  if (!crosses(outline, seg)) return false;
  const n = Math.min(2000, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / 0.1));
  for (let i = 1; i < n; i++) {
    const x = x0 + ((x1 - x0) * i) / n, y = y0 + ((y1 - y0) * i) / n;
    if (out(x, y)) return true;
  }
  return false;
};

const measure = (p, outline, o) => {
  const path = d3.geoPath(p);
  const b = path.bounds(o);
  const m = { area: finite(path.area(o)), bounds: b.map((q) => q.map(finite)) };
  if (outline) {
    m.gap = segments(p, o).some((seg) => strays(outline, seg)) || undefined;
  }
  return m;
};

const cases = [];
for (const [name, make] of Object.entries(projections)) {
  for (const rotate of [null, [20, -30, 10]]) {
    const p = make();
    if (rotate) p.rotate(rotate);
    const outline = segments(p, sphere);
    cases.push({
      type: name,
      rotate,
      xy: points.map((q) => {
        const r = p(q);
        return r && Number.isFinite(r[0]) && Number.isFinite(r[1]) ? r.map(round) : null;
      }),
      sphere: measure(p, null, sphere),
      graticule: measure(p, outline, graticule),
      countries: countries.map((f) => measure(p, outline, f)),
    });
  }
}

writeFileSync(join(out, 'projections-tier3.json'), JSON.stringify({ points, cases }) + '\n');
