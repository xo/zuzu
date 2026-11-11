// gantt.mjs reads the gantt charts of Mermaid in a folder with mermaid.js,
// and writes the start and the end of each task as JSON. It is the oracle
// of the gantt charts of D31: the tests of graph/mermaid ask zuzu for the
// same times.
//
// It loads the parser and the database of the gantt from the mermaid.js
// that mermaid-cli bundles, so it needs no DOM and no browser. The times
// are milliseconds since the epoch. Run it in UTC, so that the local time
// of mermaid.js is UTC, as in the tests:
//
//   cd test/oracle
//   TZ=UTC node gantt.mjs ../../graph/mermaid/testdata/gantt > ../../graph/mermaid/testdata/gantt/oracle.json
//
// MERMAID names the folder of the package mermaid. The default is the one
// of mermaid-cli in ~/.local/lib/node_modules.
//
// A date with no day, such as 10:00 in the format HH:mm, takes the day of
// today. The script fixes the clock at 2024-03-15T10:20:30.400Z, so that
// the output does not change from day to day, and the tests give zuzu the
// same time.

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const NOW = Date.UTC(2024, 2, 15, 10, 20, 30, 400);
const RealDate = Date;
class FixedDate extends RealDate {
  constructor(...args) {
    if (args.length === 0) {
      super(NOW);
    } else {
      super(...args);
    }
  }
  static now() {
    return NOW;
  }
}
globalThis.Date = FixedDate;

const root = process.env.MERMAID ||
  path.join(os.homedir(), '.local/lib/node_modules/@mermaid-js/mermaid-cli/node_modules/mermaid');
const chunks = path.join(root, 'dist/chunks/mermaid.core');
const file = fs.readdirSync(chunks).find((f) => /^ganttDiagram-.*\.mjs$/.test(f));
const { diagram } = await import(path.join(chunks, file));

const ms = (d) => (d === null || d === undefined || isNaN(+d) ? null : +d);

const dir = process.argv[2];
const out = {};
for (const name of fs.readdirSync(dir).filter((f) => f.endsWith('.mmd')).sort()) {
  // A title goes through DOMPurify, which needs a DOM. It does not move a
  // task, so the script drops it.
  const src = fs.readFileSync(path.join(dir, name), 'utf8').replace(/^\s*(title|accTitle|accDescr)\b.*$/gim, '');
  try {
    diagram.db.clear();
    diagram.parser.parser.yy = diagram.db;
    diagram.parser.parse(src);
    out[name] = diagram.db.getTasks().map((t) => ({
      id: t.id,
      name: t.task.trim(),
      section: t.section,
      start: ms(t.startTime),
      end: ms(t.endTime),
      renderEnd: ms(t.renderEndTime),
      milestone: !!t.milestone,
      vert: !!t.vert,
      tags: ['active', 'done', 'crit'].filter((k) => t[k]),
    }));
  } catch (e) {
    out[name] = { error: String(e.message).split('\n')[0] };
  }
}
process.stdout.write(JSON.stringify(out, null, 1) + '\n');
