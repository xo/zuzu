// oracle.mjs runs vega.js and vega-lite.js over the corpus and writes the
// references that the tests compare zuzu with (D17).
//
// For each spec that the inventory puts in the first target, it writes one
// JSON file in testdata/references, with the same path as the spec:
//
//	compiled  the Vega spec that vega-lite.js compiles, for a Vega-Lite spec
//	datasets  the rows of each dataset after its transforms
//	scales    the type and the domain of each scale of the top level
//	marks     each mark of the scene graph, with the data of each item
//
// zuzu does not draw as Vega draws (D12), so the references hold no position,
// no size and no color. A spec that reads random numbers is marked, because
// its rows differ on each run (D28). The oracle runs each spec with two seeds
// to find them.
//
// Run it from the test module after npm install in this folder:
//
//	TZ=UTC node oracle/oracle.mjs
//
// The time zone is UTC, so that the references are the same on every machine.
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import * as vega from "vega";
import * as vegaLite from "vega-lite";

const root = resolve(process.argv[2] || "testdata");
const corpus = join(root, "corpus");
const out = join(root, "references");
const only = process.argv[3];

if (process.env.TZ !== "UTC") {
  console.error("oracle: run with TZ=UTC, so that the references are the same on every machine");
  process.exit(2);
}

// The rows of the inventory that are in the first target.
const rows = readFileSync(join(corpus, "inventory.tsv"), "utf8").trim().split("\n").slice(1)
  .map((line) => line.split("\t"))
  .map(([spec, dialect, , , functions, , , first]) => ({ spec, dialect, functions: functions.split(","), first }))
  .filter((r) => r.first === "yes" && (!only || r.spec.includes(only)));

// value turns a value of vega.js into plain JSON: a date becomes a string in
// ISO form, and the internal fields of Vega are dropped. A row of a graph
// links to other rows, so an object that the same row already holds becomes
// the string "[ref]", and an array below the second level becomes its length,
// which keeps the coordinates of a map from filling the file.
function value(v, depth = 0, seen = new WeakSet()) {
  // A typed array, such as the grid of a density, is an array.
  if (ArrayBuffer.isView(v) && !(v instanceof DataView)) v = Array.from(v);
  if (v instanceof Date) return isNaN(v) ? null : v.toISOString();
  if (typeof v === "number") return Number.isFinite(v) ? v : String(v);
  if (v === undefined) return null;
  if (v === null || typeof v !== "object") return v;
  if (seen.has(v)) return "[ref]";
  if (Array.isArray(v)) {
    if (depth > 2) return { length: v.length };
    seen.add(v);
    return v.map((e) => value(e, depth + 1, depth === 0 ? new WeakSet() : seen));
  }
  if (depth > 4) return "[deep]";
  seen.add(v);
  const o = {};
  for (const k of Object.keys(v).sort()) {
    if (k === "_id" || k === "__proto__") continue;
    o[k] = value(v[k], depth + 1, seen);
  }
  return o;
}

// marks walks the scene graph and returns each mark with the data of its
// items. A group holds its marks in its items.
function marks(node, path, list) {
  if (!node) return list;
  if (node.marktype) {
    const name = node.name || "";
    const items = node.items || [];
    const entry = { path, marktype: node.marktype, role: node.role || "", name, count: items.length };
    if (node.role !== "axis" && node.role !== "legend" && !String(node.role).startsWith("axis") && !String(node.role).startsWith("legend")) {
      entry.data = items.map((it) => value(it.datum, 1));
    }
    list.push(entry);
    items.forEach((it, i) => (it.items || []).forEach((child, j) => marks(child, `${path}/${i}/${j}`, list)));
  }
  return list;
}

const loader = vega.loader({ baseURL: corpus + "/", mode: "file" });

// seeded returns a generator of random numbers from a seed, by the method
// mulberry32, so that two runs with two seeds can show whether a spec reads
// random numbers.
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

// run runs a Vega spec with the random numbers of a seed, and returns what
// a reference holds.
async function run(spec, seed) {
  const rand = seeded(seed);
  Math.random = rand;
  vega.setRandom(rand);
  // vega.js changes the inline values of a spec in place, so each run
  // parses a copy.
  const view = new vega.View(vega.parse(structuredClone(spec)), { loader, renderer: "none" });
  await view.runAsync();
  const res = { datasets: {}, scales: {} };
  for (const d of spec.data || []) {
    try {
      res.datasets[d.name] = value(view.data(d.name));
    } catch (err) {
      res.datasets[d.name] = { error: String(err.message || err) };
    }
  }
  for (const s of spec.scales || []) {
    const sc = view.scale(s.name);
    if (sc) res.scales[s.name] = { type: sc.type, domain: value(sc.domain()) };
  }
  res.marks = marks(view.scenegraph().root, "", []);
  view.finalize();
  return res;
}

// signature returns what a run gives, for the test of random numbers. The
// items of a mark whose data is another mark hold the ids of the operators
// of vega.js, which grow on each run, so only their count is kept.
function signature(res) {
  const marks = res.marks.map((m) => {
    const d = m.data || [];
    const items = d.length > 0 && d[0] && typeof d[0] === "object" && "bounds" in d[0] && "mark" in d[0];
    return items ? { ...m, data: d.length } : m;
  });
  return JSON.stringify({ ...res, marks });
}

let ok = 0;
let failed = 0;
for (const r of rows) {
  const src = JSON.parse(readFileSync(join(corpus, r.spec), "utf8"));
  const ref = { spec: r.spec, vega: vega.version, vegalite: vegaLite.version, random: r.functions.includes("random") };
  try {
    let spec = src;
    if (r.dialect === "vegalite") {
      spec = vegaLite.compile(src).spec;
      ref.compiled = spec;
    }
    const a = await run(spec, 1);
    // A spec whose results change with the seed reads random numbers,
    // through random() or through a transform such as a bootstrap or a
    // word cloud (D28).
    const b = await run(spec, 2);
    if (signature(a) !== signature(b)) ref.random = true;
    Object.assign(ref, a);
    ok++;
  } catch (err) {
    ref.error = String(err.message || err);
    failed++;
  }
  const file = join(out, r.spec.replace(/\.json$/, ".ref.json"));
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, JSON.stringify(ref, null, 1) + "\n");
}
console.log(`oracle: ${ok} references written, ${failed} specs failed in vega.js, to ${out}`);
