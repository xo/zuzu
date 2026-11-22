// schemes.mjs writes the colors of the named schemes of vega.js, and what
// some scales with a scheme map values to, for the tests of the package
// zuzu (D46). Run it from the test directory:
//
//	node oracle/schemes.mjs ../testdata
//
// It writes schemes.json. "schemes" holds each scheme by its name: the
// list of colors of a discrete scheme, or the color of a continuous scheme
// at some points from 0 to 1. "scales" holds each scale of the cases
// below, with the color that each input maps to.
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';
import { continuous, discrete } from './node_modules/vega-scale/src/palettes.js';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/schemes.mjs <dir>');
  process.exit(2);
}

const points = [0, 0.05, 0.1, 0.25, 1 / 3, 0.5, 0.6, 0.75, 0.9, 0.99, 1];
const schemes = {};
for (const name of [...Object.keys(discrete), ...Object.keys(continuous)].map((n) => n.toLowerCase()).sort()) {
  const s = vega.scheme(name);
  schemes[name] = typeof s === 'function'
    ? { at: points.map((t) => [t, s(t)]) }
    : { colors: s };
}

// Each case is a scale named color, and the values to map with it.
const cases = [
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'viridis' } }, inputs: [0, 10, 33.3, 50, 77, 100, 120, -5] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'Viridis' } }, inputs: [25] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'viridis', extent: [0.2, 0.8] } }, inputs: [0, 30, 50, 100] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'blues' }, reverse: true }, inputs: [0, 40, 100] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'blues', extent: [0.25, 1] }, reverse: true }, inputs: [0, 40, 100] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'blues', extent: [1, 0] } }, inputs: [0, 40, 100] },
  { scale: { type: 'linear', domain: [0, 100], range: { scheme: 'greens' }, clamp: true }, inputs: [-10, 50, 150] },
  { scale: { type: 'sqrt', domain: [0, 100], range: { scheme: 'oranges' } }, inputs: [1, 25, 64] },
  { scale: { type: 'log', domain: [1, 1000], range: { scheme: 'magma' } }, inputs: [1, 10, 300, 1000] },
  { scale: { type: 'linear', domain: [-1, 0, 2], range: { scheme: 'redblue' } }, inputs: [-1, -0.5, 0, 1, 2] },
  { scale: { type: 'linear', domain: [2, 0, -1], range: { scheme: 'redblue' } }, inputs: [-1, -0.5, 0, 1, 2] },
  { scale: { type: 'linear', domain: [-10, 30], domainMid: 0, range: { scheme: 'spectral' } }, inputs: [-10, -5, 0, 15, 30] },
  { scale: { type: 'linear', domain: [0, 360], range: { scheme: 'rainbow' } }, inputs: [0, 90, 200, 360] },
  { scale: { type: 'linear', domain: [0, 360], range: { scheme: 'sinebow' } }, inputs: [0, 45, 300] },
  { scale: { type: 'linear', domain: [0, 10, 20, 30], range: { scheme: 'viridis' } }, inputs: [0, 5, 10, 25, 30] },
  { scale: { type: 'linear', domain: [0, 1], range: { scheme: 'category10' } }, inputs: [0, 0.3, 0.5, 1] },
  { scale: { type: 'linear', domain: [0, 1], range: { scheme: ['#ff0000', '#0000ff', '#00ff00'] } }, inputs: [0, 0.25, 0.75, 1] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c'], range: { scheme: 'tableau10' } }, inputs: ['a', 'b', 'c'] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l'], range: { scheme: 'tableau10' } }, inputs: ['a', 'j', 'k', 'l'] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c'], range: { scheme: 'tableau20' }, reverse: true }, inputs: ['a', 'b', 'c'] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c', 'd'], range: { scheme: 'blues' } }, inputs: ['a', 'b', 'c', 'd'] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c'], range: { scheme: 'blues', count: 7 } }, inputs: ['a', 'b', 'c'] },
  { scale: { type: 'ordinal', domain: ['a', 'b', 'c'], range: { scheme: 'redblue' } }, inputs: ['a', 'b', 'c'] },
  { scale: { type: 'quantize', domain: [0, 100], range: { scheme: 'viridis' } }, inputs: [0, 30, 50, 70, 100] },
  { scale: { type: 'quantize', domain: [0, 100], range: { scheme: 'viridis', count: 3 } }, inputs: [0, 50, 100] },
  { scale: { type: 'quantize', domain: [0, 100], range: { scheme: 'set1', count: 4 } }, inputs: [0, 30, 60, 100] },
  { scale: { type: 'quantile', domain: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10], range: { scheme: 'purples', count: 4 } }, inputs: [1, 4, 6, 10] },
  { scale: { type: 'threshold', domain: [10, 20, 30], range: { scheme: 'reds' } }, inputs: [5, 15, 25, 35] },
  { scale: { type: 'threshold', domain: [10, 20], range: { scheme: 'dark2' } }, inputs: [5, 15, 25] },
  { scale: { type: 'bin-ordinal', domain: [0, 10, 20, 30, 40], range: { scheme: 'greys' } }, inputs: [5, 15, 25, 35] },
];

const scales = [];
for (const c of cases) {
  const spec = { scales: [{ name: 'color', ...c.scale }] };
  const view = new vega.View(vega.parse(spec), { renderer: 'none' });
  await view.runAsync();
  const s = view.scale('color');
  scales.push({ scale: c.scale, inputs: c.inputs, outputs: c.inputs.map((v) => s(v)) });
}
writeFileSync(join(out, 'schemes.json'), JSON.stringify({ schemes, scales }, null, 1) + '\n');
