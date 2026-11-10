// exprs.mjs evaluates expressions with vega.js and writes what each one
// returns, for the tests of the package expr. Run it from the test
// directory:
//
//	TZ=America/New_York node oracle/exprs.mjs ../expr/testdata
//
// Each expression runs in a formula transform over one row, the datum
// below. The output encodes the values that JSON does not hold: a date is
// {"date": ms}, undefined is {"undefined": true}, and NaN and the
// infinities are {"number": "NaN"} and so on.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/exprs.mjs <dir>');
  process.exit(2);
}
const exprs = readFileSync(join(out, 'exprs.txt'), 'utf8')
  .split('\n')
  .filter((l) => l.trim() !== '' && !l.startsWith('#'));

const datum = {
  a: 3, b: 'Hello, World', c: null, d: [1, 2, 3], n: -2.5, s: '2024-02-29',
  t: Date.UTC(2024, 1, 29, 17, 30, 5, 7), o: { x: 1, y: 'two' }, z: 0, e: '',
};

function encode(v) {
  if (v === undefined) return { undefined: true };
  if (v instanceof Date) return { date: v.getTime() };
  if (v instanceof RegExp) return { regexp: v.source };
  if (typeof v === 'number' && !Number.isFinite(v)) return { number: String(v) };
  if (typeof v === 'number' && Object.is(v, -0)) return { number: '-0' };
  if (Array.isArray(v)) return v.map(encode);
  if (v && typeof v === 'object') {
    const o = {};
    for (const k of Object.keys(v).sort()) o[k] = encode(v[k]);
    return o;
  }
  return v;
}

const results = [];
for (const expr of exprs) {
  const spec = {
    data: [{ name: 'd', values: [datum], transform: [{ type: 'formula', expr, as: 'r' }] }],
  };
  try {
    const view = new vega.View(vega.parse(spec), { renderer: 'none' });
    await view.runAsync();
    results.push([expr, encode(view.data('d')[0].r)]);
  } catch (err) {
    results.push([expr, { error: String(err.message).split('\n')[0] }]);
  }
}
writeFileSync(join(out, 'eval.json'), JSON.stringify(results, null, 1) + '\n');
