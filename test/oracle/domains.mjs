// domains.mjs writes the domains that vega.js sorts from values of mixed
// types, for the tests of the package zuzu. Run it from the test directory:
//
//	node oracle/domains.mjs ../testdata
//
// It writes domains.json. Each case holds the values of the field v of a
// dataset, the sort of the domain of a band scale over that field, and the
// domain that vega.js makes. JavaScript compares a string and a number as
// numbers, so the sort of mixed values is not a total order, and the
// result depends on the order of the values and on the sort of V8.
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/domains.mjs <dir>');
  process.exit(2);
}

// random returns a source of numbers from 0 to 1 that depends only on the
// seed, so that the cases are the same on each run.
function random(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const words = ['12 Angry Men', 'Alpha', 'alpha', 'Beta', 'Zeta', '2001: A Space Odyssey', '42nd Street', 'Oliver!', 'Éclair', 'Über', 'Ａwide', '𝔘math', 'zz', ''];
const numeric = ['12', '007', ' 5 ', '1e3', '0x10', '-3', '3.5', 'Infinity', '1776', 'null', 'true', '0b11', '0o7'];

// value returns one value of a mix: numbers, strings that read as numbers,
// strings that do not, null and booleans.
function value(rnd, mix) {
  const r = rnd();
  if (r < mix.number) return Math.floor(rnd() * 3000) - 50 + (rnd() < 0.1 ? 0.5 : 0);
  if (r < mix.number + mix.numeric) return numeric[Math.floor(rnd() * numeric.length)];
  if (r < mix.number + mix.numeric + mix.other) {
    if (rnd() < 0.5) return null;
    return rnd() < 0.5;
  }
  if (rnd() < 0.3) return words[Math.floor(rnd() * words.length)];
  let s = '';
  const n = 1 + Math.floor(rnd() * 6);
  for (let i = 0; i < n; i++) s += 'AbZ09 .:'[Math.floor(rnd() * 8)];
  return s;
}

const mixes = [
  { number: 0.1, numeric: 0, other: 0.01 },
  { number: 0.3, numeric: 0.1, other: 0.05 },
  { number: 0.6, numeric: 0.2, other: 0.05 },
];
const sorts = [true, { order: 'descending' }, { op: 'count' }, { op: 'count', order: 'descending' }];
const sizes = [3, 10, 30, 63, 64, 65, 130, 400, 1200, 3500];

const cases = [];
let seed = 1;
for (const size of sizes) {
  for (const mix of mixes) {
    const rnd = random(seed++);
    const values = [];
    for (let i = 0; i < size; i++) values.push(value(rnd, mix));
    if (rnd() < 0.5) {
      // A run of sorted numbers makes the sort gallop.
      const run = values.filter((v) => typeof v === 'number').sort((a, b) => a - b);
      values.push(...run.slice(0, Math.floor(run.length / 2)));
    }
    const sort = size > 400 ? true : sorts[(seed + size) % sorts.length];
    cases.push({ values, sort });
  }
}

for (const c of cases) {
  const spec = {
    data: [{ name: 'table', values: c.values.map((v) => ({ v })) }],
    scales: [{ name: 'x', type: 'band', domain: { data: 'table', field: 'v', sort: c.sort }, range: [0, 100] }],
  };
  const view = new vega.View(vega.parse(spec), { renderer: 'none' });
  await view.runAsync();
  c.domain = view.scale('x').domain();
}
writeFileSync(join(out, 'domains.json'), JSON.stringify({ cases }) + '\n');
