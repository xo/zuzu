// zonenames.mjs writes the names of the time zones that the Date of
// JavaScript writes at the end of toString(), such as "Eastern Standard
// Time", for the package expr. It reads the names of the zones of Go from
// the zoneinfo.zip of Go. Run it from the test directory:
//
//	node oracle/zonenames.mjs ../expr "$(go env GOROOT)/lib/time/zoneinfo.zip"
//
// It writes zonenames.json. "names" holds each name once. "zones" holds
// each time zone of Go and of Intl that Node.js knows, with the index of the
// name of each offset from UTC, in seconds, that the zone has on some day
// of 2024. Node.js takes the time zone from the variable TZ, which the
// script sets for each zone.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const out = process.argv[2];
const zip = process.argv[3];
if (!out || !zip) {
  console.error('usage: node oracle/zonenames.mjs <dir> <zoneinfo.zip>');
  process.exit(2);
}

// zipNames returns the names of the files in a zip, from its central
// directory.
function zipNames(b) {
  let end = b.length - 22;
  while (end >= 0 && b.readUInt32LE(end) !== 0x06054b50) end--;
  if (end < 0) throw new Error('zonenames: no central directory in the zip');
  const count = b.readUInt16LE(end + 10);
  let at = b.readUInt32LE(end + 16);
  const names = [];
  for (let i = 0; i < count; i++) {
    if (b.readUInt32LE(at) !== 0x02014b50) throw new Error('zonenames: a bad entry in the zip');
    const n = b.readUInt16LE(at + 28), x = b.readUInt16LE(at + 30), c = b.readUInt16LE(at + 32);
    names.push(b.toString('utf8', at + 46, at + 46 + n));
    at += 46 + n + x + c;
  }
  return names;
}

// known reports whether Intl knows a time zone. With a zone that it does
// not know, Node.js falls back to UTC.
function known(zone) {
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}

const zones = {};
const names = new Set();
const fromGo = zipNames(readFileSync(zip)).filter((n) => !n.endsWith('/'));
const all = [...new Set([...fromGo, ...Intl.supportedValuesOf('timeZone'), 'UTC'])].sort();
for (const zone of all.filter(known)) {
  process.env.TZ = zone;
  const byOffset = {};
  for (let t = Date.UTC(2024, 0, 1, 12); t < Date.UTC(2025, 0, 1); t += 86400000) {
    const date = new Date(t);
    const s = date.toString();
    const off = -date.getTimezoneOffset() * 60;
    if (!(off in byOffset)) byOffset[off] = s.slice(s.indexOf('(') + 1, -1);
  }
  zones[zone] = byOffset;
  Object.values(byOffset).forEach((n) => names.add(n));
}
const list = [...names].sort();
const index = new Map(list.map((n, i) => [n, i]));
for (const zone of Object.keys(zones)) {
  const o = {};
  for (const [off, name] of Object.entries(zones[zone])) o[off] = index.get(name);
  zones[zone] = o;
}
writeFileSync(join(out, 'zonenames.json'), JSON.stringify({ names: list, zones }) + '\n');
