// funcs.mjs evaluates the expression functions that read the view, such as
// gradient, copy, geoShape, treePath and vlSelectionTest, with vega.js, for
// the tests of the package zuzu. Run it from the test directory:
//
//	TZ=UTC node oracle/funcs.mjs ../testdata/funcs
//
// It reads cases.json, a list of cases. Each case holds a part of a Vega
// spec, a datum and the expressions. Each expression runs in a formula
// transform over one row, the datum, in a dataset after the datasets of
// the spec. It writes oracle.json, with what each expression returns. The
// output encodes the values that JSON does not hold as exprs.mjs does, and
// also a scale as {"scale": ...}, a projection as {"projection": ...}, the
// function that geoShape returns as {"shape": path}, and a set as
// {"set": [...]}.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';

const dir = process.argv[2];
if (!dir) {
  console.error('usage: node oracle/funcs.mjs <dir>');
  process.exit(2);
}
const cases = JSON.parse(readFileSync(join(dir, 'cases.json'), 'utf8'));

function encode(v) {
  if (v === undefined) return { undefined: true };
  if (v instanceof Date) return { date: v.getTime() };
  if (v instanceof Set) return { set: [...v].map(encode) };
  if (typeof v === 'number' && !Number.isFinite(v)) return { number: String(v) };
  if (typeof v === 'function') {
    if (typeof v.domain === 'function' && typeof v.range === 'function') {
      return { scale: { domain: encode(v.domain()), range: encode(v.range()) } };
    }
    if (typeof v.translate === 'function' && typeof v.scale === 'function') {
      return { projection: { scale: v.scale(), translate: v.translate() } };
    }
    return { shape: v(null) };
  }
  if (Array.isArray(v)) return v.map(encode);
  if (v && typeof v === 'object') {
    const o = {};
    for (const k of Object.keys(v).sort()) {
      if (typeof v[k] === 'function' && k === 'stop') continue;
      o[k] = encode(v[k]);
    }
    return o;
  }
  return v;
}

const out = [];
for (const c of cases) {
  const results = [];
  for (const expr of c.exprs) {
    const spec = structuredClone(c.spec);
    spec.data = [...(spec.data || []), {
      name: 'zuzu_out', values: [c.datum || {}],
      transform: [{ type: 'formula', expr, as: 'r' }],
    }];
    try {
      const view = new vega.View(vega.parse(spec), { renderer: 'none' });
      await view.runAsync();
      results.push([expr, encode(view.data('zuzu_out')[0].r)]);
    } catch (err) {
      results.push([expr, { error: String(err.message).split('\n')[0] }]);
    }
  }
  out.push({ name: c.name, results });
}
writeFileSync(join(dir, 'oracle.json'), JSON.stringify(out, null, 1) + '\n');
