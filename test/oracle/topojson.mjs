// topojson.mjs reads the mesh of TopoJSON topologies with the reader of
// vega.js and writes the rows, for the test of the mesh in the test
// module. Run it from the test directory:
//
//	node oracle/topojson.mjs testdata
//
// It writes testdata/topojson/mesh.json, a list of cases, each with the
// file, the object, the filter and the rows that vega.js reads.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import * as vega from 'vega';

const root = process.argv[2];
if (!root) {
  console.error('usage: node oracle/topojson.mjs <testdata>');
  process.exit(2);
}
const cases = [
  ['topojson/small.json', 'shapes', ''],
  ['topojson/small.json', 'shapes', 'interior'],
  ['topojson/small.json', 'shapes', 'exterior'],
  ['topojson/small.json', 'lines', ''],
  ['corpus/data/world-110m.json', 'countries', ''],
  ['corpus/data/world-110m.json', 'countries', 'interior'],
  ['corpus/data/world-110m.json', 'countries', 'exterior'],
  ['corpus/data/world-110m.json', 'land', ''],
  ['corpus/data/us-10m.json', 'states', 'interior'],
  ['corpus/data/us-10m.json', 'states', 'exterior'],
];
const out = cases.map(([file, mesh, filter]) => {
  const text = readFileSync(join(root, file), 'utf8');
  const format = { type: 'topojson', mesh };
  if (filter) format.filter = filter;
  return { file, mesh, filter, rows: vega.read(text, format) };
});
writeFileSync(join(root, 'topojson', 'mesh.json'), JSON.stringify(out) + '\n');
