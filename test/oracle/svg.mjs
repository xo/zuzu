// svg.mjs draws specs of the corpus with vega.js, and vega-lite.js for a
// Vega-Lite spec, and writes the SVG of each. The tool test/cmd/review runs
// it, to show what the original renderer draws next to what zuzu draws.
//
// It reads one job per line on its standard input, as JSON:
//
//	{"spec": "vega/bar.vg.json", "dialect": "vega", "out": "/abs/bar.svg"}
//
// The spec is a path in the corpus folder, and the dialect is vega or
// vegalite. For each job, it writes the SVG to out. If vega.js cannot draw
// the spec, it writes the message to out with the extension .err instead.
// It removes the other of the two files, so that only one is there.
//
// Run it from the test module after npm install in this folder:
//
//	TZ=UTC node oracle/svg.mjs testdata/corpus < jobs
//
// The time zone is UTC, so that the SVG is the same on every machine.
import { readFileSync, writeFileSync, rmSync, mkdirSync, renameSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { createInterface } from "node:readline";
import * as vega from "vega";
import * as vegaLite from "vega-lite";

const corpus = resolve(process.argv[2] || "testdata/corpus");

if (process.env.TZ !== "UTC") {
  console.error("svg: run with TZ=UTC, so that the SVG is the same on every machine");
  process.exit(2);
}

const loader = vega.loader({ baseURL: corpus + "/", mode: "file" });

// seeded returns a generator of random numbers from a seed, by the method
// mulberry32, so that a spec that reads random numbers draws the same SVG
// on each run.
function seeded(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// write writes a file through a temporary file, so that a reader never
// sees half of it, and removes the file of the other outcome.
function write(file, other, text) {
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file + ".tmp", text);
  renameSync(file + ".tmp", file);
  rmSync(other, { force: true });
}

// recorder returns a logger of vega.js that keeps each error. vega.js logs
// an error of the dataflow and goes on, and the SVG of such a run is not
// what the spec draws.
function recorder(errors) {
  const log = vega.logger(vega.Error);
  log.error = (...args) => {
    errors.push(args.map((a) => String((a && a.message) || a)).join(" "));
    return log;
  };
  return log;
}

// draw draws one spec and returns its SVG.
async function draw(job) {
  let spec = JSON.parse(readFileSync(join(corpus, job.spec), "utf8"));
  if (job.dialect === "vegalite") {
    // The compile logs only its errors, so that its warnings do not fill
    // the output of the run.
    spec = vegaLite.compile(spec, { logger: vega.logger(vega.Error) }).spec;
  }
  const rand = seeded(1);
  Math.random = rand;
  vega.setRandom(rand);
  const errors = [];
  const view = new vega.View(vega.parse(spec), { loader, renderer: "none", logger: recorder(errors) });
  try {
    await view.runAsync();
    if (errors.length > 0) {
      let msg = errors.join("\n");
      // The transforms heatmap, label and wordcloud draw on a canvas, and
      // vega.js has none in Node.js without the package canvas.
      if (/getContext|of null \(setting 'height'\)/.test(msg)) {
        msg += "\nvega.js needs the npm package canvas for this spec, which oracle/package.json does not install";
      }
      throw new Error(msg);
    }
    return await view.toSVG();
  } finally {
    view.finalize();
  }
}

// fail rejects the job that runs now. vega.js can throw an error in a
// timer, outside of any promise of the job, and the job must still end.
let fail = null;
process.on("uncaughtException", (err) => {
  if (!fail) throw err;
  fail(err);
});

let ok = 0;
let failed = 0;
for await (const line of createInterface({ input: process.stdin })) {
  if (line.trim() === "") continue;
  const job = JSON.parse(line);
  const thrown = new Promise((_, reject) => {
    fail = reject;
  });
  try {
    write(job.out, job.out + ".err", await Promise.race([draw(job), thrown]));
    ok++;
  } catch (err) {
    write(job.out + ".err", job.out, String((err && err.message) || err) + "\n");
    failed++;
  } finally {
    fail = null;
  }
}
console.log(`svg: ${ok} drawn, ${failed} failed`);
