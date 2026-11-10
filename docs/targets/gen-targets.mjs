// gen-targets.mjs writes the target SVGs of zuzu (D13) and the gallery that
// shows them.
//
// A target is a sample of the look that zuzu aims for (D12), drawn from real
// data. d3 gives the math: the scales, the paths of the shapes and the
// layouts. This script writes each SVG element by hand, from the tokens of
// docs/STYLE.md, so that the output is the design of zuzu and not the output
// of a library.
//
// The script is a tool for the design. zuzu does not use it. Run it in this
// folder after npm ci:
//
//	node gen-targets.mjs
//
// It reads ../survey/data.js and writes svg/<name>-<light|dark>.svg and
// index.html.
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import * as d3 from "d3";
import { sankey as d3sankey, sankeyLinkHorizontal } from "d3-sankey";
import { hexbin as d3hexbin } from "d3-hexbin";
import * as topojson from "topojson-client";
import dagre from "@dagrejs/dagre";

const raw = readFileSync(new URL("../survey/data.js", import.meta.url), "utf8");
const D = JSON.parse(raw.slice(raw.indexOf("{"), raw.lastIndexOf("}") + 1));

// ------------------------------------------------------------- tokens

// The tokens of docs/STYLE.md. Change them there first.
//
// A theme is a mode and a color scheme (D14). The mode sets the neutral
// colors: the background, the text, the grid and the axes. The scheme sets
// the colors of the data. Any scheme works in either mode.
const FONT = "'IBM Plex Sans', Inter, 'Helvetica Neue', Arial, sans-serif";
const MODES = {
  light: { name: "light", bg: "#ffffff", panel: "#f5f6f8", text: "#1b2230", muted: "#5f6b7d", faint: "#98a2b3", grid: "#eceff3", axis: "#cfd5de", fillOpacity: 0.2 },
  dark: { name: "dark", bg: "#10141b", panel: "#171c25", text: "#e7ebf2", muted: "#9ba5b7", faint: "#6a7487", grid: "#1e242e", axis: "#333c4a", fillOpacity: 0.24 },
};
// Each scheme gives ten categorical colors for the light mode, the index of
// the color for a rise and a fall, and, if it wants, its own colors for the
// dark mode. Without them, the dark mode lifts each light color.
const SCHEMES = {
  zuzu: {
    cat: ["#5b6ee1", "#1fa187", "#f0a830", "#e2577a", "#3aa2d6", "#9a6fd1", "#7fb04a", "#e2733b", "#c2569f", "#5d8f9b"],
    dark: ["#7d8cf0", "#3cc4a6", "#f6be55", "#f07a97", "#5cbdea", "#b38ef0", "#9ccb66", "#f08f5c", "#df7cc0", "#7fb2bd"],
    up: 1, down: 3,
  },
  pastel: {
    cat: ["#e8a87c", "#7fc8b8", "#e6cf6a", "#e7838f", "#8fb3e0", "#b7a1d8", "#a9cf8c", "#d9a3c4", "#8ccbd6", "#c9b49a"],
    up: 1, down: 3,
  },
  harbor: {
    cat: ["#1f6f9f", "#3fa7b5", "#7cc3a4", "#e3b04b", "#d0684d", "#5b4e8c", "#8aa8c8", "#2f9c7a", "#b98b5a", "#6c7a89"],
    up: 7, down: 4,
  },
  ember: {
    cat: ["#d9480f", "#f08c00", "#e8590c", "#c2255c", "#f59f00", "#862e9c", "#a61e4d", "#e67700", "#5f3dc4", "#94d82d"],
    up: 9, down: 3,
  },
  // A scheme from an upstream theme brings its own neutrals for each mode,
  // from the palette of that theme. The colors come from the upstream file
  // at the commit named in source, and licenses/ holds its license. The
  // categorical colors are the accents of the theme, ordered so that
  // neighbors differ in hue (D15).
  catppuccin: {
    source: "github.com/catppuccin/palette 07d02aa110ef9eb7e7427afca5c73ba9cf7f8ebd palette.json, MIT",
    up: 1, down: 3,
    modes: {
      light: {
        bg: "#eff1f5", panel: "#e6e9ef", text: "#4c4f69", muted: "#6c6f85", faint: "#9ca0b0", grid: "#dce0e8", axis: "#bcc0cc",
        cat: ["#1e66f5", "#40a02b", "#df8e1d", "#d20f39", "#04a5e5", "#8839ef", "#fe640b", "#179299", "#ea76cb", "#7287fd"],
      },
      dark: {
        bg: "#1e1e2e", panel: "#181825", text: "#cdd6f4", muted: "#a6adc8", faint: "#6c7086", grid: "#313244", axis: "#45475a",
        cat: ["#89b4fa", "#a6e3a1", "#f9e2af", "#f38ba8", "#89dceb", "#cba6f7", "#fab387", "#94e2d5", "#f5c2e7", "#b4befe"],
      },
    },
  },
  solarized: {
    source: "github.com/altercation/solarized 62f656a02f93c5190a8753159e34b385588d5ff3 README.md, MIT",
    up: 1, down: 3,
    modes: {
      light: {
        bg: "#fdf6e3", panel: "#eee8d5", text: "#586e75", muted: "#657b83", faint: "#93a1a1", grid: "#eee8d5", axis: "#93a1a1",
        cat: ["#268bd2", "#859900", "#b58900", "#dc322f", "#2aa198", "#6c71c4", "#cb4b16", "#d33682"],
      },
      dark: {
        bg: "#002b36", panel: "#073642", text: "#93a1a1", muted: "#839496", faint: "#586e75", grid: "#073642", axis: "#586e75",
        cat: ["#268bd2", "#859900", "#b58900", "#dc322f", "#2aa198", "#6c71c4", "#cb4b16", "#d33682"],
      },
    },
  },
};
function theme(mode, scheme = "zuzu") {
  const s = SCHEMES[scheme], own = s.modes?.[mode];
  const m = { ...MODES[mode], ...(own || {}), name: mode };
  let cat = own ? own.cat : mode === "dark" ? (s.dark || s.cat.map((c) => d3.hsl(c).brighter(0.45).formatHex())) : s.cat;
  // A palette with fewer than ten colors gets the rest as each color mixed
  // toward the text color, so that a chart with ten series still differs.
  for (let i = cat.length; i < 10; i++) cat = [...cat, d3.interpolateRgb(cat[i % cat.length], m.text)(0.4)];
  const base = cat[0];
  // The sequential ramp runs from the background to the first color and
  // past it, so that it reads in either mode.
  const seq = mode === "dark"
    ? [d3.interpolateRgb(m.bg, base)(0.12), d3.interpolateRgb(m.bg, base)(0.45), base, d3.interpolateRgb(base, "#ffffff")(0.45), d3.interpolateRgb(base, "#ffffff")(0.85)]
    : [d3.interpolateRgb(m.bg, base)(0.1), d3.interpolateRgb(m.bg, base)(0.4), base, d3.interpolateRgb(base, "#000000")(0.35), d3.interpolateRgb(base, "#000000")(0.6)];
  return { ...m, scheme, cat, seq, up: cat[s.up], down: cat[s.down] };
}
const THEMES = { light: theme("light"), dark: theme("dark") };
const PAD = 24;
const SIZE = { title: 16, subtitle: 12.5, label: 11, small: 10, source: 10.5 };

// est estimates the width of a string in the font of the targets. zuzu
// measures text (D11), and this script only needs a close guess.
const est = (s, size) => String(s).length * size * 0.56;

// ---------------------------------------------------------------- svg

const r2 = (v) => Math.round(v * 100) / 100;
const esc = (s) => String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
const roundPath = (d) => String(d).replace(/-?\d*\.\d{3,}/g, (m) => String(r2(+m)));
const attrs = (o) => Object.entries(o)
  .filter(([, v]) => v != null && v !== false)
  .map(([k, v]) => ` ${k}="${k === "d" || k === "points" ? roundPath(v) : typeof v === "number" ? r2(v) : esc(v)}"`)
  .join("");
const el = (name, o = {}, ...kids) => {
  const body = kids.flat(Infinity).filter((k) => k != null && k !== false).join("");
  return body ? `<${name}${attrs(o)}>${body}</${name}>` : `<${name}${attrs(o)}/>`;
};
const txt = (s, o) => el("text", o, esc(s));
const g = (o, ...kids) => el("g", o, ...kids);

// A rectangle whose top corners are round, for a bar that grows up.
function barUp(x, y, w, h, r = 3) {
  if (h <= 0 || w <= 0) return "";
  r = Math.min(r, w / 2, h);
  return `M${x},${y + h}V${y + r}Q${x},${y} ${x + r},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${y + h}Z`;
}
// A rectangle whose right corners are round, for a bar that grows right.
function barRight(x, y, w, h, r = 3) {
  if (h <= 0 || w <= 0) return "";
  r = Math.min(r, h / 2, w);
  return `M${x},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${y + h - r}Q${x + w},${y + h} ${x + w - r},${y + h}H${x}Z`;
}

// ------------------------------------------------------------- layout

// legendRows lays out the items of a legend in rows no wider than width.
function legendRows(items, width) {
  const rows = [[]];
  let x = 0;
  for (const it of items) {
    const w = 14 + est(it.label, SIZE.label) + 16;
    if (x + w > width && rows[rows.length - 1].length) { rows.push([]); x = 0; }
    rows[rows.length - 1].push({ ...it, x });
    x += w;
  }
  return rows;
}

// frame lays out the title, the subtitle, the legend and the source of a
// chart, and returns the plot area that is left.
function frame(w, h, o) {
  const rows = o.legend ? legendRows(o.legend, w - 2 * PAD) : [];
  const top = (o.subtitle ? 66 : 50) + rows.length * 20 + (rows.length ? 10 : 0);
  return {
    w, h, rows,
    x0: PAD + (o.left ?? 40), x1: w - PAD - (o.right ?? 0),
    y0: top + (o.top ?? 0), y1: h - (o.source ? 30 : 16) - (o.bottom ?? 24),
    ...o,
  };
}

// svg writes a whole chart: the background, the header, the body and the
// source line.
function svg(t, f, body, defs = "") {
  const head = [
    txt(f.title, { x: PAD, y: 34, "font-size": SIZE.title, "font-weight": 600, fill: t.text }),
    f.subtitle && txt(f.subtitle, { x: PAD, y: 54, "font-size": SIZE.subtitle, fill: t.muted }),
  ];
  const ly = (f.subtitle ? 78 : 62);
  const legend = f.rows.map((row, ri) => row.map((it) => g({ transform: `translate(${PAD + it.x},${ly + ri * 20})` },
    it.line ? el("rect", { x: 0, y: -1.5, width: 12, height: 3, rx: 1.5, fill: it.color })
      : el("rect", { x: 0, y: -5, width: 10, height: 10, rx: 3, fill: it.color }),
    txt(it.label, { x: 15, y: 4, "font-size": SIZE.label, fill: t.muted }))));
  const src = f.source && txt(f.source, { x: PAD, y: f.h - 14, "font-size": SIZE.source, fill: t.faint });
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${f.w}" height="${f.h}" viewBox="0 0 ${f.w} ${f.h}" font-family="${esc(FONT)}" role="img" aria-label="${esc(f.title)}">` +
    el("title", {}, esc(f.title)) +
    (defs ? el("defs", {}, defs) : "") +
    el("rect", { width: f.w, height: f.h, fill: t.bg }) +
    head.join("") + legend.flat().join("") + body + (src || "") + "</svg>\n";
}

// yGrid draws the horizontal grid lines and their labels on the left.
function yGrid(t, f, y, ticks, fmt, { right = false, color } = {}) {
  return g({}, ticks.map((v) => [
    el("line", { x1: f.x0, x2: f.x1, y1: y(v), y2: y(v), stroke: t.grid, "stroke-width": 1 }),
    txt(fmt(v), right
      ? { x: f.x1 + 8, y: y(v) + 3.5, "font-size": SIZE.label, fill: color || t.muted }
      : { x: f.x0 - 8, y: y(v) + 3.5, "font-size": SIZE.label, fill: color || t.muted, "text-anchor": "end" }),
  ]));
}
// yLabels draws the labels of an axis without grid lines.
const yLabels = (t, f, y, ticks, fmt, x, anchor, color) => g({}, ticks.map((v) =>
  txt(fmt(v), { x, y: y(v) + 3.5, "font-size": SIZE.label, fill: color || t.muted, "text-anchor": anchor })));

// xAxis draws the base line and the labels below the plot area.
function xAxis(t, f, x, ticks, fmt, { line = true, at } = {}) {
  const y = at ?? f.y1;
  return g({}, line && el("line", { x1: f.x0, x2: f.x1, y1: y, y2: y, stroke: t.axis, "stroke-width": 1 }),
    ticks.map((v) => txt(fmt(v), { x: x(v), y: y + 18, "font-size": SIZE.label, fill: t.muted, "text-anchor": "middle" })));
}

// relax moves labels apart so that no two are closer than gap.
function relax(items, gap, lo, hi) {
  items.sort((a, b) => a.y - b.y);
  for (let k = 0; k < 40; k++) {
    let moved = false;
    for (let i = 1; i < items.length; i++) {
      const d = items[i].y - items[i - 1].y;
      if (d < gap) { const m = (gap - d) / 2; items[i].y += m; items[i - 1].y -= m; moved = true; }
    }
    for (const it of items) it.y = Math.max(lo, Math.min(hi, it.y));
    if (!moved) break;
  }
  return items;
}

// lcg is a fixed random sequence, so that a jitter is the same on every run.
function lcg(seed = 7) {
  let s = seed;
  return () => ((s = (s * 1664525 + 1013904223) % 4294967296) / 4294967296);
}

const seqScale = (t, domain) => d3.scaleSequential(d3.interpolateRgbBasis(t.seq)).domain(domain);
const fmtInt = d3.format(",");
const fmtUsd = (v) => "$" + d3.format(",")(v);
const MONTHS = { Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5, Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11 };
const stockDate = (s) => { const [m, d, y] = s.split(" "); return new Date(Date.UTC(+y, MONTHS[m], +d)); };
const SYMBOLS = ["AAPL", "AMZN", "GOOG", "IBM", "MSFT"];
const stocks = D.stocks.map((r) => ({ ...r, date: stockDate(r.date) }));
const bySym = d3.group(stocks, (r) => r.symbol);
const weather = D.weather.map((r) => ({ ...r, d: new Date(r.date + "T00:00:00Z") }));
const W_TYPES = ["sun", "fog", "drizzle", "rain", "snow"];

// ------------------------------------------------------------- charts

const CHARTS = [];
const chart = (id, tier, title, data, fn) => CHARTS.push({ id, tier, title, data, fn });

// ---- tier 0

chart("line", 0, "Multi-series line with a crosshair", "stocks.csv", (t) => {
  const f = frame(640, 400, {
    title: "Tech stock prices", subtitle: "Monthly closing price in US dollars, 2000 to 2010",
    source: "Source: vega-datasets, stocks.csv", right: 52, left: 36,
  });
  const x = d3.scaleUtc(d3.extent(stocks, (r) => r.date), [f.x0, f.x1]);
  const y = d3.scaleLinear([0, d3.max(stocks, (r) => r.price)], [f.y1, f.y0]).nice();
  const line = d3.line().x((r) => x(r.date)).y((r) => y(r.price)).curve(d3.curveMonotoneX);
  const at = new Date(Date.UTC(2007, 9, 1));
  const tipRows = SYMBOLS.map((s, i) => ({ s, i, v: bySym.get(s).find((r) => +r.date === +at)?.price })).sort((a, b) => b.v - a.v);
  const tx = x(at), tipW = 128, tipH = 26 + tipRows.length * 17, tipX = tx - tipW - 12, tipY = f.y0 + 6;
  const ends = relax(SYMBOLS.map((s, i) => { const last = bySym.get(s).at(-1); return { s, i, x: x(last.date), y: y(last.price), v: last.price }; }).map((e) => ({ ...e, py: e.y })), 13, f.y0, f.y1);
  const body = [
    yGrid(t, f, y, y.ticks(5), fmtUsd),
    xAxis(t, f, x, x.ticks(d3.utcYear.every(2)), d3.utcFormat("%Y")),
    el("line", { x1: tx, x2: tx, y1: f.y0, y2: f.y1, stroke: t.faint, "stroke-width": 1, "stroke-dasharray": "3 3" }),
    SYMBOLS.map((s, i) => el("path", { d: line(bySym.get(s)), fill: "none", stroke: t.cat[i], "stroke-width": 2, "stroke-linejoin": "round", "stroke-linecap": "round" })),
    tipRows.filter((r) => r.v != null).map((r) => el("circle", { cx: tx, cy: y(r.v), r: 4, fill: t.cat[r.i], stroke: t.bg, "stroke-width": 2 })),
    ends.map((e) => [
      el("circle", { cx: e.x, cy: e.py, r: 3, fill: t.cat[e.i] }),
      txt(e.s, { x: e.x + 8, y: e.y + 3.5, "font-size": SIZE.label, "font-weight": 600, fill: t.cat[e.i] }),
    ]),
    g({ transform: `translate(${tipX},${tipY})` },
      el("rect", { width: tipW, height: tipH, rx: 8, fill: t.panel, stroke: t.axis, "stroke-width": 1 }),
      txt("October 2007", { x: 12, y: 18, "font-size": SIZE.label, "font-weight": 600, fill: t.text }),
      tipRows.map((r, k) => [
        el("circle", { cx: 16, cy: 35 + k * 17, r: 3.5, fill: t.cat[r.i] }),
        txt(r.s, { x: 26, y: 39 + k * 17, "font-size": SIZE.label, fill: t.muted }),
        txt(fmtUsd(r.v.toFixed(0)), { x: tipW - 12, y: 39 + k * 17, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
      ])),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("area", 0, "Area with a gradient", "stocks.csv", (t) => {
  const f = frame(640, 400, { title: "Apple", subtitle: "Monthly closing price in US dollars, 2000 to 2010", source: "Source: vega-datasets, stocks.csv", left: 36, right: 16 });
  const rows = bySym.get("AAPL");
  const x = d3.scaleUtc(d3.extent(rows, (r) => r.date), [f.x0, f.x1]);
  const y = d3.scaleLinear([0, d3.max(rows, (r) => r.price)], [f.y1, f.y0]).nice();
  const area = d3.area().x((r) => x(r.date)).y0(y(0)).y1((r) => y(r.price)).curve(d3.curveMonotoneX);
  const line = area.lineY1();
  const max = d3.greatest(rows, (r) => r.price), last = rows.at(-1);
  const defs = el("linearGradient", { id: "fill", x1: 0, x2: 0, y1: 0, y2: 1 },
    el("stop", { offset: 0, "stop-color": t.cat[0], "stop-opacity": 0.32 }),
    el("stop", { offset: 1, "stop-color": t.cat[0], "stop-opacity": 0.02 }));
  const body = [
    yGrid(t, f, y, y.ticks(5), fmtUsd),
    xAxis(t, f, x, x.ticks(d3.utcYear.every(2)), d3.utcFormat("%Y")),
    el("path", { d: area(rows), fill: "url(#fill)" }),
    el("path", { d: line(rows), fill: "none", stroke: t.cat[0], "stroke-width": 2, "stroke-linejoin": "round" }),
    el("circle", { cx: x(max.date), cy: y(max.price), r: 4, fill: t.bg, stroke: t.cat[0], "stroke-width": 2 }),
    txt(`High ${fmtUsd(max.price.toFixed(0))}`, { x: x(max.date) - 10, y: y(max.price) - 4, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
    txt(d3.utcFormat("%b %Y")(max.date), { x: x(max.date) - 10, y: y(max.price) + 10, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" }),
    el("circle", { cx: x(last.date), cy: y(last.price), r: 4, fill: t.cat[0], stroke: t.bg, "stroke-width": 2 }),
  ];
  return svg(t, f, body.flat().join(""), defs);
});

const wMonths = d3.utcMonths(new Date(Date.UTC(2012, 0, 1)), new Date(Date.UTC(2016, 0, 1)));
const wByMonth = wMonths.map((m) => {
  const rows = weather.filter((r) => r.d.getUTCFullYear() === m.getUTCFullYear() && r.d.getUTCMonth() === m.getUTCMonth());
  const o = { m };
  for (const w of W_TYPES) o[w] = rows.filter((r) => r.weather === w).length;
  return o;
});

chart("stacked-area", 0, "Stacked area", "seattle-weather.csv", (t) => {
  const order = ["sun", "rain", "fog", "drizzle", "snow"];
  const f = frame(640, 400, {
    title: "Seattle weather", subtitle: "Days of each kind of weather, by month",
    legend: order.map((w) => ({ label: w, color: t.cat[W_TYPES.indexOf(w)] })), source: "Source: vega-datasets, seattle-weather.csv", left: 30,
  });
  const series = d3.stack().keys(order)(wByMonth);
  const x = d3.scaleUtc(d3.extent(wMonths), [f.x0, f.x1]);
  const y = d3.scaleLinear([0, 31], [f.y1, f.y0]);
  const area = d3.area().x((d) => x(d.data.m)).y0((d) => y(d[0])).y1((d) => y(d[1])).curve(d3.curveMonotoneX);
  const body = [
    yGrid(t, f, y, [0, 10, 20, 30], String),
    series.map((s) => el("path", { d: area(s), fill: t.cat[W_TYPES.indexOf(s.key)], "fill-opacity": 0.88, stroke: t.bg, "stroke-width": 1 })),
    xAxis(t, f, x, x.ticks(d3.utcYear.every(1)), d3.utcFormat("%Y")),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("streamgraph", 1, "Streamgraph with direct labels", "stocks.csv", (t) => {
  const f = frame(640, 400, { title: "Five stocks as a stream", subtitle: "Monthly price, stacked around a moving center", source: "Source: vega-datasets, stocks.csv", left: 0 });
  const dates = [...new Set(stocks.map((r) => +r.date))].sort((a, b) => a - b);
  const rows = dates.map((d) => Object.assign({ d: new Date(d) }, Object.fromEntries(SYMBOLS.map((s) => [s, bySym.get(s).find((r) => +r.date === d)?.price ?? 0]))));
  const series = d3.stack().keys(SYMBOLS).offset(d3.stackOffsetWiggle).order(d3.stackOrderInsideOut)(rows);
  const x = d3.scaleUtc(d3.extent(rows, (r) => r.d), [f.x0, f.x1]);
  const y = d3.scaleLinear([d3.min(series, (s) => d3.min(s, (d) => d[0])), d3.max(series, (s) => d3.max(s, (d) => d[1]))], [f.y1 - 6, f.y0]);
  const area = d3.area().x((d) => x(d.data.d)).y0((d) => y(d[0])).y1((d) => y(d[1])).curve(d3.curveBasis);
  const body = [
    x.ticks(d3.utcYear.every(2)).map((v) => el("line", { x1: x(v), x2: x(v), y1: f.y0, y2: f.y1, stroke: t.grid })),
    series.map((s) => el("path", { d: area(s), fill: t.cat[SYMBOLS.indexOf(s.key)], "fill-opacity": 0.9 })),
    series.map((s) => {
      const i = d3.maxIndex(s, (d) => d[1] - d[0]);
      const d = s[i];
      if (y(d[0]) - y(d[1]) < 16) return "";
      return txt(s.key, { x: x(d.data.d), y: (y(d[0]) + y(d[1])) / 2 + 4, "font-size": SIZE.label + 1, "font-weight": 600, fill: t.bg, "text-anchor": "middle" });
    }),
    xAxis(t, f, x, x.ticks(d3.utcYear.every(2)), d3.utcFormat("%Y"), { line: false }),
  ];
  return svg(t, f, body.flat().join(""));
});

const barleySites = d3.groupSort(D.barley, (v) => -d3.sum(v, (r) => r.yield), (r) => r.site);

chart("bar-grouped", 0, "Grouped bar", "barley.json", (t) => {
  const f = frame(640, 400, {
    title: "Barley yield by site", subtitle: "Total yield in bushels per acre, over ten varieties",
    legend: [{ label: "1931", color: t.cat[0] }, { label: "1932", color: t.cat[2] }], source: "Source: vega-datasets, barley.json", left: 30,
  });
  const sum = d3.rollup(D.barley, (v) => d3.sum(v, (r) => r.yield), (r) => r.site, (r) => r.year);
  const x = d3.scaleBand(barleySites, [f.x0, f.x1]).paddingInner(0.3).paddingOuter(0.15);
  const xi = d3.scaleBand([1931, 1932], [0, x.bandwidth()]).paddingInner(0.08);
  const y = d3.scaleLinear([0, d3.max([...sum.values()], (m) => d3.max(m.values()))], [f.y1, f.y0]).nice();
  const body = [
    yGrid(t, f, y, y.ticks(5), String),
    barleySites.map((s) => [1931, 1932].map((yr, k) => {
      const v = sum.get(s).get(yr);
      return el("path", { d: barUp(x(s) + xi(yr), y(v), xi.bandwidth(), y(0) - y(v), 3), fill: t.cat[k * 2] });
    })),
    xAxis(t, f, (v) => x(v) + x.bandwidth() / 2, barleySites, String),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("bar-stacked", 0, "Stacked horizontal bar", "barley.json", (t) => {
  const varieties = d3.groupSort(D.barley, (v) => -d3.sum(v, (r) => r.yield), (r) => r.variety);
  const f = frame(640, 420, {
    title: "What each site grew", subtitle: "Yield by variety, 1931 and 1932 together",
    legend: varieties.map((v, i) => ({ label: v, color: t.cat[i] })), source: "Source: vega-datasets, barley.json", left: 88, right: 34, bottom: 22,
  });
  const sum = d3.rollup(D.barley, (v) => d3.sum(v, (r) => r.yield), (r) => r.site, (r) => r.variety);
  const rows = barleySites.map((s) => Object.assign({ s }, Object.fromEntries(varieties.map((v) => [v, sum.get(s).get(v)]))));
  const series = d3.stack().keys(varieties)(rows);
  const y = d3.scaleBand(barleySites, [f.y0, f.y1]).padding(0.28);
  const x = d3.scaleLinear([0, d3.max(series.at(-1), (d) => d[1])], [f.x0, f.x1]).nice();
  const body = [
    x.ticks(5).map((v) => [
      el("line", { x1: x(v), x2: x(v), y1: f.y0, y2: f.y1, stroke: t.grid }),
      txt(String(v), { x: x(v), y: f.y1 + 16, "font-size": SIZE.label, fill: t.muted, "text-anchor": "middle" }),
    ]),
    series.map((s, i) => s.map((d) => {
      const last = i === series.length - 1;
      const w = x(d[1]) - x(d[0]) - (last ? 0 : 1.5);
      return last ? el("path", { d: barRight(x(d[0]), y(d.data.s), w, y.bandwidth(), 4), fill: t.cat[i] })
        : el("rect", { x: x(d[0]), y: y(d.data.s), width: Math.max(0, w), height: y.bandwidth(), fill: t.cat[i] });
    })),
    barleySites.map((s) => txt(s, { x: f.x0 - 10, y: y(s) + y.bandwidth() / 2 + 4, "font-size": SIZE.label, fill: t.text, "text-anchor": "end" })),
    rows.map((r) => txt(d3.format(".0f")(series.at(-1).find((d) => d.data.s === r.s)[1]), { x: x(series.at(-1).find((d) => d.data.s === r.s)[1]) + 6, y: y(r.s) + y.bandwidth() / 2 + 4, "font-size": SIZE.small, "font-weight": 600, fill: t.muted })),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("histogram", 0, "Histogram with a reference line", "cars.json", (t) => {
  const mpg = D.cars.map((r) => r.Miles_per_Gallon).filter((v) => v != null);
  const f = frame(640, 400, { title: "Fuel economy of 398 cars", subtitle: "Number of cars in each band of 2 miles per gallon, 1970 to 1982", source: "Source: vega-datasets, cars.json", left: 30 });
  const bins = d3.bin().domain([8, 48]).thresholds(d3.range(8, 48, 2))(mpg);
  const x = d3.scaleLinear([8, 48], [f.x0, f.x1]);
  const y = d3.scaleLinear([0, d3.max(bins, (b) => b.length)], [f.y1, f.y0]).nice();
  const mean = d3.mean(mpg);
  const body = [
    yGrid(t, f, y, y.ticks(5), String),
    bins.map((b) => el("path", { d: barUp(x(b.x0) + 1, y(b.length), x(b.x1) - x(b.x0) - 2, y(0) - y(b.length), 2.5), fill: t.cat[4] })),
    el("line", { x1: x(mean), x2: x(mean), y1: f.y0 - 4, y2: f.y1, stroke: t.text, "stroke-width": 1.5, "stroke-dasharray": "4 3" }),
    txt(`Mean ${mean.toFixed(1)} mpg`, { x: x(mean) + 6, y: f.y0 + 6, "font-size": SIZE.label, "font-weight": 600, fill: t.text }),
    xAxis(t, f, x, d3.range(10, 50, 5), String),
  ];
  return svg(t, f, body.flat().join(""));
});

const CLUSTERS = ["South Asia", "Europe and Central Asia", "Sub-Saharan Africa", "The Americas", "East Asia and Pacific", "Middle East and North Africa"];

chart("bubble", 0, "Bubble scatter with labels", "gapminder.json", (t) => {
  const rows = D.gapminder.filter((r) => r.year === 2005);
  const f = frame(640, 440, {
    title: "Fewer children, longer lives", subtitle: "Fertility and life expectancy of 63 countries in 2005. The area of a circle is the population",
    legend: CLUSTERS.map((c, i) => ({ label: c, color: t.cat[i] })), source: "Source: Gapminder Foundation, via vega-datasets gapminder.json", left: 30, bottom: 34, top: 16,
  });
  const x = d3.scaleLinear([1, 7.5], [f.x0, f.x1]);
  const y = d3.scaleLinear([40, 85], [f.y1, f.y0]);
  const r = d3.scaleSqrt([0, d3.max(rows, (d) => d.pop)], [0, 30]);
  const big = d3.sort(rows, (d) => -d.pop).slice(0, 4).map((d) => d.country);
  const body = [
    yGrid(t, f, y, [40, 50, 60, 70, 80], String),
    x.ticks(7).map((v) => el("line", { x1: x(v), x2: x(v), y1: f.y0, y2: f.y1, stroke: t.grid })),
    xAxis(t, f, x, x.ticks(7), String, { line: false }),
    txt("Children per woman", { x: f.x1, y: f.y1 + 34, "font-size": SIZE.small, fill: t.faint, "text-anchor": "end" }),
    txt("Life expectancy in years", { x: f.x0 - 30, y: f.y0 - 12, "font-size": SIZE.small, fill: t.faint }),
    d3.sort(rows, (d) => -d.pop).map((d) => el("circle", { cx: x(d.fertility), cy: y(d.life_expect), r: Math.max(2.5, r(d.pop)), fill: t.cat[d.cluster], "fill-opacity": 0.72, stroke: t.bg, "stroke-width": 1 })),
    rows.filter((d) => big.includes(d.country)).map((d) => txt(d.country, { x: x(d.fertility), y: y(d.life_expect) - r(d.pop) - 5, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "middle" })),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("donut", 0, "Donut with a value table", "seattle-weather.csv", (t) => {
  const counts = d3.sort(W_TYPES.map((w) => ({ w, n: weather.filter((r) => r.weather === w).length })), (d) => -d.n);
  const total = d3.sum(counts, (d) => d.n);
  const f = frame(640, 360, { title: "Weather in Seattle", subtitle: "Days of each kind, 2012 to 2015", source: "Source: vega-datasets, seattle-weather.csv", left: 0 });
  const cx = PAD + 130, cy = (f.y0 + f.y1) / 2, R = Math.min(115, (f.y1 - f.y0) / 2);
  const arcs = d3.pie().value((d) => d.n).sort(null).padAngle(0.014)(counts);
  const arc = d3.arc().innerRadius(R * 0.64).outerRadius(R).cornerRadius(4);
  const tx = 340;
  const body = [
    g({ transform: `translate(${cx},${cy})` },
      arcs.map((a) => el("path", { d: arc(a), fill: t.cat[W_TYPES.indexOf(a.data.w)] })),
      txt(fmtInt(total), { x: 0, y: 4, "font-size": 28, "font-weight": 600, fill: t.text, "text-anchor": "middle" }),
      txt("days", { x: 0, y: 24, "font-size": SIZE.label, fill: t.muted, "text-anchor": "middle" })),
    counts.map((d, i) => {
      const yy = cy - (counts.length * 30) / 2 + i * 30 + 18;
      return [
        el("rect", { x: tx, y: yy - 9, width: 10, height: 10, rx: 3, fill: t.cat[W_TYPES.indexOf(d.w)] }),
        txt(d.w, { x: tx + 18, y: yy, "font-size": 12.5, fill: t.text }),
        txt(fmtInt(d.n), { x: tx + 170, y: yy, "font-size": 12.5, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
        txt(d3.format(".0%")(d.n / total), { x: tx + 230, y: yy, "font-size": 12.5, fill: t.muted, "text-anchor": "end" }),
        i < counts.length - 1 && el("line", { x1: tx, x2: tx + 230, y1: yy + 11, y2: yy + 11, stroke: t.grid }),
      ];
    }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("calendar", 0, "Calendar heat map", "seattle-weather.csv", (t) => {
  const days = weather.filter((r) => r.d.getUTCFullYear() === 2015);
  const cell = 9.5, gap = 1.5, step = cell + gap;
  const f = frame(680, 222, { title: "Daily high in Seattle, 2015", subtitle: "Each square is a day. Darker is warmer", source: "Source: vega-datasets, seattle-weather.csv", left: 28 });
  const color = seqScale(t, d3.extent(days, (r) => r.temp_max));
  const year = new Date(Date.UTC(2015, 0, 1));
  const wk = (d) => d3.utcSunday.count(year, d);
  const ox = f.x0, oy = f.y0 + 16;
  const body = [
    days.map((r) => el("rect", { x: ox + wk(r.d) * step, y: oy + r.d.getUTCDay() * step, width: cell, height: cell, rx: 2, fill: color(r.temp_max) })),
    d3.utcMonths(year, new Date(Date.UTC(2016, 0, 1))).map((m) => txt(d3.utcFormat("%b")(m), { x: ox + wk(m) * step, y: oy - 6, "font-size": SIZE.small, fill: t.muted })),
    [[1, "Mon"], [3, "Wed"], [5, "Fri"]].map(([d, s]) => txt(s, { x: ox - 6, y: oy + d * step + 8, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" })),
    txt("Colder", { x: f.x1 - 120, y: oy + 7 * step + 22, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" }),
    d3.range(5).map((i) => el("rect", { x: f.x1 - 114 + i * 14, y: oy + 7 * step + 13, width: cell, height: cell, rx: 2, fill: color(color.domain()[0] + (i / 4) * (color.domain()[1] - color.domain()[0])) })),
    txt("Warmer", { x: f.x1 - 40, y: oy + 7 * step + 22, "font-size": SIZE.small, fill: t.muted }),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("dual-axis", 0, "Bars and a line with two axes", "seattle-weather.csv", (t) => {
  const rows = d3.range(12).map((m) => {
    const v = weather.filter((r) => r.d.getUTCFullYear() === 2015 && r.d.getUTCMonth() === m);
    return { m, rain: d3.sum(v, (r) => r.precipitation), high: d3.mean(v, (r) => r.temp_max) };
  });
  const f = frame(640, 400, {
    title: "Rain and warmth in Seattle, 2015", subtitle: "Total rain in millimeters, and the mean daily high in °C, by month",
    legend: [{ label: "Rain (mm)", color: t.cat[4] }, { label: "Mean high (°C)", color: t.cat[7], line: true }], source: "Source: vega-datasets, seattle-weather.csv", left: 34, right: 34,
  });
  const x = d3.scaleBand(d3.range(12), [f.x0, f.x1]).padding(0.32);
  const yl = d3.scaleLinear([0, d3.max(rows, (r) => r.rain)], [f.y1, f.y0]).nice();
  const yr = d3.scaleLinear([0, 30], [f.y1, f.y0]);
  const line = d3.line().x((r) => x(r.m) + x.bandwidth() / 2).y((r) => yr(r.high)).curve(d3.curveMonotoneX);
  const mn = ["J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"];
  const body = [
    yGrid(t, f, yl, yl.ticks(5), String, { color: t.cat[4] }),
    yLabels(t, f, yr, yr.ticks(5).filter((v) => v <= 30), (v) => v + "°", f.x1 + 8, "start", t.cat[7]),
    rows.map((r) => el("path", { d: barUp(x(r.m), yl(r.rain), x.bandwidth(), yl(0) - yl(r.rain), 3), fill: t.cat[4], "fill-opacity": 0.85 })),
    el("path", { d: line(rows), fill: "none", stroke: t.cat[7], "stroke-width": 2.5, "stroke-linejoin": "round" }),
    rows.map((r) => el("circle", { cx: x(r.m) + x.bandwidth() / 2, cy: yr(r.high), r: 3.5, fill: t.cat[7], stroke: t.bg, "stroke-width": 2 })),
    xAxis(t, f, (v) => x(v) + x.bandwidth() / 2, d3.range(12), (v) => mn[v]),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("small-multiples", 1, "Small multiples", "stocks.csv", (t) => {
  const f = frame(640, 420, { title: "Each stock on its own scale", subtitle: "Monthly closing price in US dollars, 2000 to 2010", source: "Source: vega-datasets, stocks.csv", left: 0 });
  const cols = 3, gw = 14, gh = 20;
  const pw = (f.x1 - f.x0 - (cols - 1) * gw) / cols, ph = (f.y1 - f.y0 - gh) / 2;
  const x = d3.scaleUtc(d3.extent(stocks, (r) => r.date), [0, pw]);
  const parts = SYMBOLS.map((s, i) => {
    const rows = bySym.get(s);
    const col = i < 3 ? i : i - 3 + 0.5, row = i < 3 ? 0 : 1;
    const ox = f.x0 + col * (pw + gw), oy = f.y0 + row * (ph + gh);
    const y = d3.scaleLinear([0, d3.max(rows, (r) => r.price)], [ph - 16, 34]).nice();
    const area = d3.area().x((r) => x(r.date)).y0(y(0)).y1((r) => y(r.price)).curve(d3.curveMonotoneX);
    const last = rows.at(-1);
    return g({ transform: `translate(${ox},${oy})` },
      el("rect", { width: pw, height: ph, rx: 10, fill: t.panel }),
      txt(s, { x: 12, y: 20, "font-size": 12.5, "font-weight": 600, fill: t.text }),
      txt(fmtUsd(last.price.toFixed(0)), { x: pw - 12, y: 20, "font-size": 12.5, "font-weight": 600, fill: t.cat[i], "text-anchor": "end" }),
      el("path", { d: area(rows), fill: `url(#sm${i})` }),
      el("path", { d: area.lineY1()(rows), fill: "none", stroke: t.cat[i], "stroke-width": 1.75 }),
      txt(fmtUsd(y.domain()[1]), { x: 12, y: y(y.domain()[1]) + 12, "font-size": SIZE.small, fill: t.faint }),
      txt("2000", { x: 4, y: ph - 4, "font-size": SIZE.small, fill: t.faint }),
      txt("2010", { x: pw - 4, y: ph - 4, "font-size": SIZE.small, fill: t.faint, "text-anchor": "end" }));
  });
  const defs = SYMBOLS.map((s, i) => el("linearGradient", { id: `sm${i}`, x1: 0, x2: 0, y1: 0, y2: 1 },
    el("stop", { offset: 0, "stop-color": t.cat[i], "stop-opacity": 0.3 }), el("stop", { offset: 1, "stop-color": t.cat[i], "stop-opacity": 0.02 }))).join("");
  return svg(t, f, parts.join(""), defs);
});

// ---- tier 1

const SPECIES = ["Adelie", "Chinstrap", "Gentoo"];
const penguins = D.penguins.filter((r) => r["Body Mass (g)"] != null && r["Flipper Length (mm)"] != null);

chart("box", 1, "Box plot with raw points", "penguins.json", (t) => {
  const f = frame(640, 360, { title: "Body mass of penguins", subtitle: "Grams, by species. Each dot is one penguin", source: "Source: vega-datasets, penguins.json", left: 72, bottom: 26 });
  const y = d3.scaleBand(SPECIES, [f.y0, f.y1]).padding(0.22);
  const x = d3.scaleLinear(d3.extent(penguins, (r) => r["Body Mass (g)"]), [f.x0, f.x1]).nice();
  const rnd = lcg(11);
  const body = [
    x.ticks(6).map((v) => [el("line", { x1: x(v), x2: x(v), y1: f.y0, y2: f.y1, stroke: t.grid }), txt(fmtInt(v), { x: x(v), y: f.y1 + 16, "font-size": SIZE.label, fill: t.muted, "text-anchor": "middle" })]),
    SPECIES.map((s, i) => {
      const v = penguins.filter((r) => r.Species === s).map((r) => r["Body Mass (g)"]).sort(d3.ascending);
      const q1 = d3.quantile(v, 0.25), q2 = d3.quantile(v, 0.5), q3 = d3.quantile(v, 0.75), iqr = q3 - q1;
      const lo = d3.min(v.filter((a) => a >= q1 - 1.5 * iqr)), hi = d3.max(v.filter((a) => a <= q3 + 1.5 * iqr));
      const by = y(s), bh = y.bandwidth() * 0.46, my = by + bh / 2;
      return [
        txt(s, { x: f.x0 - 12, y: my + 4, "font-size": 12, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
        el("line", { x1: x(lo), x2: x(hi), y1: my, y2: my, stroke: t.cat[i], "stroke-width": 1.5 }),
        el("line", { x1: x(lo), x2: x(lo), y1: my - 6, y2: my + 6, stroke: t.cat[i], "stroke-width": 1.5 }),
        el("line", { x1: x(hi), x2: x(hi), y1: my - 6, y2: my + 6, stroke: t.cat[i], "stroke-width": 1.5 }),
        el("rect", { x: x(q1), y: by, width: x(q3) - x(q1), height: bh, rx: 5, fill: t.cat[i], "fill-opacity": t.fillOpacity, stroke: t.cat[i], "stroke-width": 1.5 }),
        el("line", { x1: x(q2), x2: x(q2), y1: by, y2: by + bh, stroke: t.cat[i], "stroke-width": 2.5 }),
        v.map((a) => el("circle", { cx: x(a), cy: by + bh + 8 + rnd() * (y.bandwidth() - bh - 12), r: 2, fill: t.cat[i], "fill-opacity": 0.5 })),
      ];
    }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("violin", 1, "Violin with quartiles", "penguins.json", (t) => {
  const key = "Flipper Length (mm)";
  const f = frame(640, 400, { title: "Flipper length of penguins", subtitle: "Millimeters, by species. The width shows how many penguins share a length", source: "Source: vega-datasets, penguins.json", left: 30 });
  const x = d3.scaleBand(SPECIES, [f.x0, f.x1]).padding(0.18);
  const y = d3.scaleLinear([170, 235], [f.y1, f.y0]);
  const grid = d3.range(170, 235.5, 1);
  const kde = (v, bw) => grid.map((g0) => [g0, d3.mean(v, (a) => { const u = (g0 - a) / bw; return Math.abs(u) <= 1 ? 0.75 * (1 - u * u) / bw : 0; })]);
  const dens = SPECIES.map((s) => kde(penguins.filter((r) => r.Species === s).map((r) => r[key]), 5));
  const w = d3.scaleLinear([0, d3.max(dens, (d) => d3.max(d, (p) => p[1]))], [0, x.bandwidth() / 2]);
  const body = [
    yGrid(t, f, y, d3.range(170, 236, 10), String),
    SPECIES.map((s, i) => {
      const cx = x(s) + x.bandwidth() / 2;
      const v = penguins.filter((r) => r.Species === s).map((r) => r[key]).sort(d3.ascending);
      const area = d3.area().y((p) => y(p[0])).x0((p) => cx - w(p[1])).x1((p) => cx + w(p[1])).curve(d3.curveBasis);
      const q1 = d3.quantile(v, 0.25), q2 = d3.quantile(v, 0.5), q3 = d3.quantile(v, 0.75);
      return [
        el("path", { d: area(dens[i].filter((p) => p[1] > 0.0005)), fill: t.cat[i], "fill-opacity": 0.75, stroke: t.cat[i], "stroke-width": 1 }),
        el("rect", { x: cx - 4, y: y(q3), width: 8, height: y(q1) - y(q3), rx: 4, fill: t.bg, "fill-opacity": 0.85 }),
        el("circle", { cx, cy: y(q2), r: 3, fill: t.cat[i] }),
        txt(s, { x: cx, y: f.y1 + 18, "font-size": 12, "font-weight": 600, fill: t.text, "text-anchor": "middle" }),
      ];
    }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("candlestick", 1, "Candlestick on a trading-day scale", "ohlc.json", (t) => {
  const rows = D.ohlc.map((r) => ({ ...r, d: new Date(r.date + "T00:00:00Z") }));
  const f = frame(640, 400, {
    title: "CBOE Volatility Index", subtitle: "Daily open, high, low and close, June to August 2009. Weekends leave no gaps",
    legend: [{ label: "Rose", color: t.up }, { label: "Fell", color: t.down }, { label: "5-day mean", color: t.cat[0], line: true }], source: "Source: vega-datasets, ohlc.json", left: 26,
  });
  const x = d3.scaleBand(rows.map((r) => r.date), [f.x0, f.x1]).padding(0.3);
  const y = d3.scaleLinear([d3.min(rows, (r) => r.low), d3.max(rows, (r) => r.high)], [f.y1, f.y0]).nice();
  const ma = rows.map((r, i) => i < 4 ? null : { r, v: d3.mean(rows.slice(i - 4, i + 1), (a) => a.close) }).filter(Boolean);
  const line = d3.line().x((p) => x(p.r.date) + x.bandwidth() / 2).y((p) => y(p.v)).curve(d3.curveMonotoneX);
  const firsts = rows.filter((r, i) => i === 0 || r.d.getUTCDay() < rows[i - 1].d.getUTCDay());
  const body = [
    yGrid(t, f, y, y.ticks(5), String),
    rows.map((r) => {
      const c = r.close >= r.open ? t.up : t.down, cx = x(r.date) + x.bandwidth() / 2;
      const top = y(Math.max(r.open, r.close)), h = Math.max(1, Math.abs(y(r.open) - y(r.close)));
      return [el("line", { x1: cx, x2: cx, y1: y(r.high), y2: y(r.low), stroke: c, "stroke-width": 1.25 }),
        el("rect", { x: x(r.date), y: top, width: x.bandwidth(), height: h, rx: 1.5, fill: c })];
    }),
    el("path", { d: line(ma), fill: "none", stroke: t.cat[0], "stroke-width": 2, "stroke-opacity": 0.9 }),
    xAxis(t, f, (v) => x(v) + x.bandwidth() / 2, firsts.map((r) => r.date), (v) => d3.utcFormat("%b %-d")(new Date(v + "T00:00:00Z"))),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("radar", 1, "Radar", "penguins.json", (t) => {
  const keys = ["Beak Length (mm)", "Beak Depth (mm)", "Flipper Length (mm)", "Body Mass (g)"];
  const names = ["Beak length", "Beak depth", "Flipper length", "Body mass"];
  const means = SPECIES.map((s) => keys.map((k) => d3.mean(D.penguins.filter((r) => r.Species === s && r[k] != null), (r) => r[k])));
  const max = keys.map((_, i) => d3.max(means, (m) => m[i]));
  const f = frame(640, 440, { title: "Penguins compared", subtitle: "Mean of each measure, as a share of the largest mean", legend: SPECIES.map((s, i) => ({ label: s, color: t.cat[i] })), source: "Source: vega-datasets, penguins.json", left: 0 });
  const cx = (f.x0 + f.x1) / 2, cy = (f.y0 + f.y1) / 2 + 4, R = (f.y1 - f.y0) / 2 - 24;
  const ang = (i) => -Math.PI / 2 + (i * 2 * Math.PI) / keys.length;
  const pt = (i, v) => [cx + Math.cos(ang(i)) * R * v, cy + Math.sin(ang(i)) * R * v];
  const poly = (vals) => vals.map((v, i) => pt(i, v).join(",")).join(" ");
  const body = [
    [0.25, 0.5, 0.75, 1].map((v) => el("polygon", { points: poly(keys.map(() => v)), fill: "none", stroke: t.grid, "stroke-width": 1 })),
    keys.map((_, i) => el("line", { x1: cx, y1: cy, x2: pt(i, 1)[0], y2: pt(i, 1)[1], stroke: t.grid })),
    [0.5, 1].map((v) => txt(d3.format(".0%")(v), { x: cx + 4, y: cy - R * v - 3, "font-size": SIZE.small, fill: t.faint })),
    means.map((m, si) => {
      const v = m.map((a, i) => 0.25 + 0.75 * ((a / max[i]) - 0.6) / 0.4);
      return [el("polygon", { points: poly(v), fill: t.cat[si], "fill-opacity": 0.14, stroke: t.cat[si], "stroke-width": 2, "stroke-linejoin": "round" }),
        v.map((a, i) => el("circle", { cx: pt(i, a)[0], cy: pt(i, a)[1], r: 3.5, fill: t.cat[si], stroke: t.bg, "stroke-width": 1.5 }))];
    }),
    names.map((n, i) => {
      const [px, py] = pt(i, 1.12);
      return txt(n, { x: px, y: py + 4, "font-size": 12, "font-weight": 600, fill: t.text, "text-anchor": Math.abs(px - cx) < 2 ? "middle" : px > cx ? "start" : "end" });
    }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("choropleth", 1, "Choropleth map", "us-10m.json, population_engineers_hurricanes.csv", (t) => {
  const geo = topojson.feature(D.us, D.us.objects.states);
  const mesh = topojson.mesh(D.us, D.us.objects.states, (a, b) => a !== b);
  const val = new Map(D.states.map((r) => [r.id, r.engineers * 1000]));
  const f = frame(640, 440, { title: "Where the engineers live", subtitle: "Engineers per 1,000 people, by state", source: "Source: vega-datasets, us-10m.json and population_engineers_hurricanes.csv", left: 0, bottom: 34 });
  const proj = d3.geoAlbersUsa().fitExtent([[f.x0, f.y0], [f.x1, f.y1]], geo);
  const path = d3.geoPath(proj);
  const color = seqScale(t, d3.extent([...val.values()]));
  const lx = f.x1 - 180, ly = f.y1 + 20;
  const defs = el("linearGradient", { id: "ramp", x1: 0, x2: 1, y1: 0, y2: 0 }, d3.range(0, 1.01, 0.25).map((o) => el("stop", { offset: o, "stop-color": color(color.domain()[0] + o * (color.domain()[1] - color.domain()[0])) })));
  const body = [
    geo.features.map((ft) => el("path", { d: path(ft), fill: val.has(+ft.id) ? color(val.get(+ft.id)) : t.panel })),
    el("path", { d: path(mesh), fill: "none", stroke: t.bg, "stroke-width": 0.8, "stroke-linejoin": "round" }),
    el("rect", { x: lx, y: ly, width: 180, height: 8, rx: 4, fill: "url(#ramp)" }),
    txt(color.domain()[0].toFixed(1), { x: lx, y: ly + 22, "font-size": SIZE.small, fill: t.muted }),
    txt(color.domain()[1].toFixed(1), { x: lx + 180, y: ly + 22, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" }),
    txt("per 1,000 people", { x: lx - 8, y: ly + 8, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" }),
  ];
  return svg(t, f, body.flat().join(""), defs);
});

// The Flare toolkit as a tree, shared by the hierarchy charts.
function flareRoot(depth = Infinity) {
  const root = d3.stratify().id((d) => d.id).parentId((d) => d.parent)(D.flare);
  root.sum((d) => d.size || 0).sort((a, b) => b.value - a.value);
  root.each((n) => { n.top = n.depth === 0 ? null : n.ancestors().at(-2).data.name; });
  if (depth !== Infinity) root.each((n) => { if (n.depth >= depth) n.children = null; });
  return root;
}
const FLARE_TOPS = flareRoot(1).children.map((c) => c.data.name);

chart("treemap", 1, "Treemap with group headers", "flare.json", (t) => {
  const f = frame(640, 440, { title: "The Flare toolkit by size", subtitle: "Each box is a package of the toolkit, sized by the bytes of its source", source: "Source: vega-datasets, flare.json", left: 0 });
  const root = flareRoot(2);
  d3.treemap().size([f.x1 - f.x0, f.y1 - f.y0]).paddingOuter(2).paddingTop(18).paddingInner(2).round(true)(root);
  const body = g({ transform: `translate(${f.x0},${f.y0})` },
    root.children.map((c) => {
      const ci = FLARE_TOPS.indexOf(c.data.name);
      return [
        el("rect", { x: c.x0, y: c.y0, width: c.x1 - c.x0, height: c.y1 - c.y0, rx: 6, fill: t.cat[ci], "fill-opacity": 0.14 }),
        c.x1 - c.x0 > 40 && txt(c.data.name, { x: c.x0 + 6, y: c.y0 + 13, "font-size": SIZE.label, "font-weight": 600, fill: t.text }),
        (c.children || []).map((l) => [
          el("rect", { x: l.x0, y: l.y0, width: l.x1 - l.x0, height: l.y1 - l.y0, rx: 3, fill: t.cat[ci], "fill-opacity": 0.85 }),
          l.x1 - l.x0 > est(l.data.name, SIZE.small) + 10 && l.y1 - l.y0 > 18 && txt(l.data.name, { x: l.x0 + 5, y: l.y0 + 13, "font-size": SIZE.small, fill: t.bg, "font-weight": 600 }),
        ]),
      ];
    }));
  return svg(t, f, body);
});

chart("sunburst", 1, "Sunburst", "flare.json", (t) => {
  const f = frame(640, 460, { title: "The Flare toolkit as rings", subtitle: "Packages from the center out. The angle is the size of the source", legend: FLARE_TOPS.map((n, i) => ({ label: n, color: t.cat[i] })), source: "Source: vega-datasets, flare.json", left: 0 });
  const root = flareRoot(3);
  const R = Math.min(f.x1 - f.x0, f.y1 - f.y0) / 2;
  d3.partition().size([2 * Math.PI, R])(root);
  const arc = d3.arc().startAngle((d) => d.x0).endAngle((d) => d.x1).padAngle(0.004).padRadius(R / 2)
    .innerRadius((d) => d.y0 + 1).outerRadius((d) => d.y1 - 1).cornerRadius(2);
  const cx = (f.x0 + f.x1) / 2, cy = (f.y0 + f.y1) / 2;
  const body = g({ transform: `translate(${cx},${cy})` },
    root.descendants().filter((d) => d.depth > 0).map((d) => el("path", {
      d: arc(d), fill: t.cat[FLARE_TOPS.indexOf(d.top)], "fill-opacity": d.depth === 1 ? 1 : d.depth === 2 ? 0.7 : 0.42,
    })),
    txt("flare", { x: 0, y: 5, "font-size": 15, "font-weight": 600, fill: t.text, "text-anchor": "middle" }));
  return svg(t, f, body);
});

chart("sankey", 1, "Sankey", "cars.json", (t) => {
  const band = (v) => v == null ? null : v < 20 ? "Under 20 mpg" : v < 30 ? "20 to 30 mpg" : "30 mpg or more";
  const cars = D.cars.filter((r) => r.Miles_per_Gallon != null);
  const links = [];
  const add = (s, tg) => { const l = links.find((a) => a.source === s && a.target === tg); if (l) l.value++; else links.push({ source: s, target: tg, value: 1 }); };
  for (const r of cars) { add(r.Origin, `${r.Cylinders} cylinders`); add(`${r.Cylinders} cylinders`, band(r.Miles_per_Gallon)); }
  const names = [...new Set(links.flatMap((l) => [l.source, l.target]))];
  const f = frame(640, 420, { title: "From origin to fuel economy", subtitle: "398 cars, by where they were made, their cylinders and their miles per gallon", source: "Source: vega-datasets, cars.json", left: 0, right: 0 });
  const { nodes, links: ls } = d3sankey().nodeId((d) => d.name).nodeWidth(10).nodePadding(16).nodeSort(null)
    .extent([[f.x0 + 60, f.y0], [f.x1 - 110, f.y1]])({ nodes: names.map((name) => ({ name })), links: links.map((l) => ({ ...l })) });
  const origins = ["Europe", "Japan", "USA"];
  const colorOf = (n) => origins.includes(n.name) ? t.cat[origins.indexOf(n.name)] : t.faint;
  const linkColor = (l) => {
    let n = l.source;
    if (!origins.includes(n.name)) return t.faint;
    return colorOf(n);
  };
  const body = [
    ls.map((l) => el("path", { d: sankeyLinkHorizontal()(l), fill: "none", stroke: linkColor(l), "stroke-opacity": 0.32, "stroke-width": Math.max(1, l.width) })),
    nodes.map((n) => [
      el("rect", { x: n.x0, y: n.y0, width: n.x1 - n.x0, height: Math.max(1, n.y1 - n.y0), rx: 3, fill: colorOf(n) }),
      txt(n.name, { x: n.x0 < 200 ? n.x0 - 8 : n.x1 + 8, y: (n.y0 + n.y1) / 2 + 1, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": n.x0 < 200 ? "end" : "start" }),
      txt(String(n.value), { x: n.x0 < 200 ? n.x0 - 8 : n.x1 + 8, y: (n.y0 + n.y1) / 2 + 14, "font-size": SIZE.small, fill: t.muted, "text-anchor": n.x0 < 200 ? "end" : "start" }),
    ]),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("funnel", 1, "Funnel", "cars.json", (t) => {
  const c = D.cars;
  const steps = [
    ["All cars", c],
    ["Mileage known", c.filter((r) => r.Miles_per_Gallon != null)],
    ["25 mpg or more", c.filter((r) => r.Miles_per_Gallon >= 25)],
    ["Under 90 horsepower", c.filter((r) => r.Miles_per_Gallon >= 25 && r.Horsepower != null && r.Horsepower < 90)],
    ["Made in Japan", c.filter((r) => r.Miles_per_Gallon >= 25 && r.Horsepower != null && r.Horsepower < 90 && r.Origin === "Japan")],
  ].map(([label, rows]) => ({ label, n: rows.length }));
  const f = frame(640, 380, { title: "Narrowing down the cars", subtitle: "How many of 406 cars pass each filter in turn", source: "Source: vega-datasets, cars.json", left: 130, right: 90 });
  const y = d3.scaleBand(steps.map((s) => s.label), [f.y0, f.y1]).paddingInner(0.32);
  const cx = (f.x0 + f.x1) / 2;
  const w = d3.scaleLinear([0, steps[0].n], [0, f.x1 - f.x0]);
  const body = [
    steps.map((s, i) => {
      const half = w(s.n) / 2, top = y(s.label), h = y.bandwidth();
      const next = steps[i + 1];
      return [
        next && el("path", { d: `M${cx - half},${top + h}L${cx + half},${top + h}L${cx + w(next.n) / 2},${y(next.label)}L${cx - w(next.n) / 2},${y(next.label)}Z`, fill: t.cat[0], "fill-opacity": 0.12 }),
        el("rect", { x: cx - half, y: top, width: Math.max(2, 2 * half), height: h, rx: 6, fill: t.cat[0], "fill-opacity": 1 - i * 0.14 }),
        txt(s.label, { x: f.x0 - 130 + PAD - 24, y: top + h / 2 + 4, "font-size": 12, fill: t.text }),
        txt(fmtInt(s.n), { x: f.x1 + 86, y: top + h / 2 - 1, "font-size": 13, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
        txt(i === 0 ? "start" : d3.format(".0%")(s.n / steps[i - 1].n) + " kept", { x: f.x1 + 86, y: top + h / 2 + 13, "font-size": SIZE.small, fill: t.muted, "text-anchor": "end" }),
      ];
    }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("gauge", 1, "Gauges", "seattle-weather.csv", (t) => {
  const days = weather.filter((r) => r.d.getUTCFullYear() === 2015);
  const share = (w) => days.filter((r) => r.weather === w).length / days.length;
  const items = [["Sunny days", share("sun"), 0], ["Rainy days", share("rain"), 4], ["Foggy days", share("fog"), 1]];
  const f = frame(640, 250, { title: "Seattle in 2015", subtitle: "Share of the 365 days of each kind", source: "Source: vega-datasets, seattle-weather.csv", left: 0 });
  const cw = (f.x1 - f.x0) / 3, R = Math.min(cw / 2 - 22, f.y1 - f.y0 - 34);
  const arc = d3.arc().innerRadius(R * 0.78).outerRadius(R).cornerRadius(R);
  const body = items.map(([label, v, ci], i) => {
    const cx = f.x0 + cw * i + cw / 2, cy = f.y0 + R + 10;
    const a0 = -Math.PI / 2, a1 = Math.PI / 2;
    return g({ transform: `translate(${cx},${cy})` },
      el("path", { d: arc({ startAngle: a0, endAngle: a1 }), fill: t.grid }),
      el("path", { d: arc({ startAngle: a0, endAngle: a0 + v * Math.PI }), fill: t.cat[ci] }),
      txt(d3.format(".0%")(v), { x: 0, y: -6, "font-size": 30, "font-weight": 600, fill: t.text, "text-anchor": "middle" }),
      txt(label, { x: 0, y: 22, "font-size": 12, fill: t.muted, "text-anchor": "middle" }));
  });
  return svg(t, f, body.join(""));
});

chart("waterfall", 1, "Waterfall", "stocks.csv", (t) => {
  const rows = bySym.get("AAPL").filter((r) => r.date.getUTCFullYear() === 2008);
  const steps = [{ label: "Jan", kind: "total", v: rows[0].price }];
  for (let i = 1; i < rows.length; i++) steps.push({ label: d3.utcFormat("%b")(rows[i].date), kind: "delta", v: rows[i].price - rows[i - 1].price });
  steps.push({ label: "Dec", kind: "end", v: rows.at(-1).price });
  let run = 0;
  for (const s of steps) { if (s.kind === "delta") { s.a = run; run += s.v; s.b = run; } else { s.a = 0; s.b = s.v; run = s.v; } }
  const f = frame(640, 400, {
    title: "Apple in 2008, month by month", subtitle: "The price on the first trading day of January, each monthly change, and the price in December",
    legend: [{ label: "Rose", color: t.up }, { label: "Fell", color: t.down }, { label: "Price", color: t.cat[0] }], source: "Source: vega-datasets, stocks.csv", left: 30,
  });
  steps.at(-1).label = "Dec close";
  const x = d3.scaleBand(steps.map((s, i) => i), [f.x0, f.x1]).padding(0.28);
  const y = d3.scaleLinear([0, d3.max(steps, (s) => Math.max(s.a, s.b))], [f.y1, f.y0]).nice();
  const body = [
    yGrid(t, f, y, y.ticks(5), fmtUsd),
    steps.map((s, i) => {
      const c = s.kind !== "delta" ? t.cat[0] : s.v >= 0 ? t.up : t.down;
      const top = y(Math.max(s.a, s.b)), h = Math.max(1, Math.abs(y(s.a) - y(s.b)));
      const next = steps[i + 1];
      return [
        el("rect", { x: x(i), y: top, width: x.bandwidth(), height: h, rx: 3, fill: c }),
        next && el("line", { x1: x(i) + x.bandwidth(), x2: x(i + 1), y1: y(s.b), y2: y(s.b), stroke: t.faint, "stroke-dasharray": "2 2" }),
        txt(s.kind === "delta" ? (s.v >= 0 ? "+" : "−") + Math.abs(s.v).toFixed(0) : fmtUsd(s.v.toFixed(0)), { x: x(i) + x.bandwidth() / 2, y: top - 5, "font-size": SIZE.small, "font-weight": 600, fill: s.kind === "delta" ? c : t.text, "text-anchor": "middle" }),
      ];
    }),
    xAxis(t, f, (v) => x(v) + x.bandwidth() / 2, steps.map((s, i) => i), (i) => steps[i].label.replace(" close", "")),
  ];
  return svg(t, f, body.flat(2).join(""));
});

// ---- tier 2

chart("parallel", 2, "Parallel coordinates", "penguins.json", (t) => {
  const keys = ["Beak Length (mm)", "Beak Depth (mm)", "Flipper Length (mm)", "Body Mass (g)"];
  const rows = D.penguins.filter((r) => keys.every((k) => r[k] != null));
  const f = frame(640, 420, { title: "Four measures of each penguin", subtitle: "Each line is one penguin. Each axis has its own scale", legend: SPECIES.map((s, i) => ({ label: s, color: t.cat[i] })), source: "Source: vega-datasets, penguins.json", left: 20, right: 20, top: 16 });
  const x = d3.scalePoint(keys, [f.x0, f.x1]);
  const ys = Object.fromEntries(keys.map((k) => [k, d3.scaleLinear(d3.extent(rows, (r) => r[k]), [f.y1, f.y0]).nice()]));
  const line = d3.line();
  const body = [
    rows.map((r) => el("path", { d: line(keys.map((k) => [x(k), ys[k](r[k])])), fill: "none", stroke: t.cat[SPECIES.indexOf(r.Species)], "stroke-opacity": 0.32, "stroke-width": 1 })),
    keys.map((k) => [
      el("line", { x1: x(k), x2: x(k), y1: f.y0, y2: f.y1, stroke: t.axis }),
      txt(k.replace(/ \(.*\)/, ""), { x: x(k), y: f.y0 - 12, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "middle" }),
      ys[k].ticks(4).map((v) => txt(fmtInt(v), { x: x(k) + 5, y: ys[k](v) + 3.5, "font-size": SIZE.small, fill: t.muted, stroke: t.bg, "stroke-width": 3, "paint-order": "stroke" })),
    ]),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("chord", 2, "Chord", "flare-dependencies.json, flare.json", (t) => {
  const root = flareRoot();
  const topOf = new Map(root.descendants().map((n) => [+n.data.id, n.top]));
  const idx = new Map(FLARE_TOPS.map((n, i) => [n, i]));
  const m = FLARE_TOPS.map(() => FLARE_TOPS.map(() => 0));
  for (const d of D.flareDeps) {
    const a = idx.get(topOf.get(d.source)), b = idx.get(topOf.get(d.target));
    if (a != null && b != null) m[a][b]++;
  }
  const f = frame(640, 520, { title: "Which packages of Flare use which", subtitle: "Each ribbon is the imports between two packages of the toolkit", source: "Source: vega-datasets, flare-dependencies.json", left: 0 });
  const cx = (f.x0 + f.x1) / 2, cy = (f.y0 + f.y1) / 2, R = Math.min(f.x1 - f.x0, f.y1 - f.y0) / 2 - 34;
  const chords = d3.chord().padAngle(0.035).sortSubgroups(d3.descending)(m);
  const arc = d3.arc().innerRadius(R).outerRadius(R + 10).cornerRadius(2);
  const ribbon = d3.ribbon().radius(R - 2);
  const body = g({ transform: `translate(${cx},${cy})` },
    chords.map((c) => el("path", { d: ribbon(c), fill: t.cat[c.source.index], "fill-opacity": 0.55, stroke: t.bg, "stroke-width": 0.5 })),
    chords.groups.map((gr) => {
      const a = (gr.startAngle + gr.endAngle) / 2 - Math.PI / 2;
      const lx = Math.cos(a) * (R + 22), ly = Math.sin(a) * (R + 22);
      return [el("path", { d: arc(gr), fill: t.cat[gr.index] }),
        txt(FLARE_TOPS[gr.index], { x: lx, y: ly + 4, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": Math.cos(a) > 0.1 ? "start" : Math.cos(a) < -0.1 ? "end" : "middle" })];
    }));
  return svg(t, f, body);
});

chart("force", 2, "Force-directed network", "miserables.json", (t) => {
  const nodes = D.miserables.nodes.map((n, i) => ({ ...n, i }));
  const links = D.miserables.links.map((l) => ({ ...l }));
  const deg = new Map(); for (const l of links) { deg.set(l.source, (deg.get(l.source) || 0) + l.value); deg.set(l.target, (deg.get(l.target) || 0) + l.value); }
  const sim = d3.forceSimulation(nodes).force("link", d3.forceLink(links).id((d) => d.i).distance(28).strength(0.6))
    .force("charge", d3.forceManyBody().strength(-60)).force("x", d3.forceX(0).strength(0.07)).force("y", d3.forceY(0).strength(0.1)).force("collide", d3.forceCollide(7)).stop();
  for (let i = 0; i < 400; i++) sim.tick();
  const f = frame(640, 480, { title: "Who meets whom in Les Misérables", subtitle: "Characters who appear in the same chapter. The size is how often", source: "Source: vega-datasets, miserables.json", left: 0 });
  const x = d3.scaleLinear(d3.extent(nodes, (n) => n.x), [f.x0 + 10, f.x1 - 10]);
  const y = d3.scaleLinear(d3.extent(nodes, (n) => n.y), [f.y0 + 10, f.y1 - 10]);
  const s = Math.min((x.range()[1] - x.range()[0]) / (x.domain()[1] - x.domain()[0]), (y.range()[1] - y.range()[0]) / (y.domain()[1] - y.domain()[0]));
  const ox = (f.x0 + f.x1) / 2 - s * (x.domain()[0] + x.domain()[1]) / 2, oy = (f.y0 + f.y1) / 2 - s * (y.domain()[0] + y.domain()[1]) / 2;
  const P = (n) => [ox + s * n.x, oy + s * n.y];
  const r = d3.scaleSqrt([0, d3.max(deg.values())], [2.5, 11]);
  const top = d3.sort(nodes, (n) => -(deg.get(n.i) || 0)).slice(0, 6).map((n) => n.i);
  const body = [
    links.map((l) => el("line", { x1: P(l.source)[0], y1: P(l.source)[1], x2: P(l.target)[0], y2: P(l.target)[1], stroke: t.axis, "stroke-width": Math.min(3, 0.6 + l.value * 0.15) })),
    nodes.map((n) => el("circle", { cx: P(n)[0], cy: P(n)[1], r: r(deg.get(n.i) || 0), fill: t.cat[n.group % 10], stroke: t.bg, "stroke-width": 1.5 })),
    nodes.filter((n) => top.includes(n.i)).map((n) => txt(n.name, { x: P(n)[0], y: P(n)[1] - r(deg.get(n.i)) - 5, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "middle", stroke: t.bg, "stroke-width": 3, "paint-order": "stroke" })),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("tree", 2, "Radial tree", "flare.json", (t) => {
  const root = flareRoot(3);
  const f = frame(640, 560, { title: "The packages of Flare", subtitle: "Three levels of the toolkit, from the root at the center", legend: FLARE_TOPS.map((n, i) => ({ label: n, color: t.cat[i] })), source: "Source: vega-datasets, flare.json", left: 0 });
  const R = Math.min(f.x1 - f.x0, f.y1 - f.y0) / 2 - 10;
  d3.tree().size([2 * Math.PI, R]).separation((a, b) => (a.parent === b.parent ? 1 : 2) / a.depth)(root);
  const cx = (f.x0 + f.x1) / 2, cy = (f.y0 + f.y1) / 2;
  const link = d3.linkRadial().angle((d) => d.x).radius((d) => d.y);
  const pt = (d) => [Math.cos(d.x - Math.PI / 2) * d.y, Math.sin(d.x - Math.PI / 2) * d.y];
  const body = g({ transform: `translate(${cx},${cy})` },
    root.links().map((l) => el("path", { d: link(l), fill: "none", stroke: l.target.top ? t.cat[FLARE_TOPS.indexOf(l.target.top)] : t.axis, "stroke-opacity": 0.45, "stroke-width": 1.2 })),
    root.descendants().map((d) => el("circle", { cx: pt(d)[0], cy: pt(d)[1], r: d.depth === 0 ? 5 : d.depth === 1 ? 4 : 2.5, fill: d.depth === 0 ? t.text : t.cat[FLARE_TOPS.indexOf(d.top)], stroke: t.bg, "stroke-width": 1 })),
    root.children.map((d) => txt(d.data.name, { x: pt(d)[0], y: pt(d)[1] - 8, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "middle", stroke: t.bg, "stroke-width": 3, "paint-order": "stroke" })));
  return svg(t, f, body);
});

chart("pack", 2, "Circle packing", "flare.json", (t) => {
  const root = flareRoot(3);
  const f = frame(640, 520, { title: "Flare as nested circles", subtitle: "Packages inside packages, sized by source", source: "Source: vega-datasets, flare.json", left: 0 });
  const S = Math.min(f.x1 - f.x0, f.y1 - f.y0);
  d3.pack().size([S, S]).padding(3)(root);
  const ox = (f.x0 + f.x1) / 2 - S / 2, oy = f.y0;
  const body = g({ transform: `translate(${ox},${oy})` },
    root.descendants().map((d) => el("circle", {
      cx: d.x, cy: d.y, r: d.r,
      fill: d.depth === 0 ? t.panel : t.cat[FLARE_TOPS.indexOf(d.top)],
      "fill-opacity": d.depth === 0 ? 1 : d.depth === 1 ? 0.16 : d.depth === 2 ? 0.45 : 0.85,
      stroke: d.depth === 1 ? t.cat[FLARE_TOPS.indexOf(d.top)] : "none", "stroke-width": 1,
    })),
    root.children.filter((d) => d.r > 24).map((d) => txt(d.data.name, { x: d.x, y: d.y - d.r + 14, "font-size": SIZE.label, "font-weight": 600, fill: t.text, "text-anchor": "middle", stroke: t.bg, "stroke-width": 3, "paint-order": "stroke" })));
  return svg(t, f, body);
});

chart("beeswarm", 2, "Beeswarm", "cars.json", (t) => {
  const origins = ["USA", "Europe", "Japan"];
  const rows = D.cars.filter((r) => r.Miles_per_Gallon != null);
  const R = 2.6, GAP = 18;
  const x = d3.scaleLinear([8, 48], [PAD + 60, 640 - PAD]);
  // dodge places each dot at the smallest offset from the center line that
  // does not overlap a dot already placed. Each row then gets the height its
  // dots need, so a crowded row never spills into the next.
  const swarms = origins.map((o) => {
    const placed = [];
    for (const r of d3.sort(rows.filter((a) => a.Origin === o), (a) => a.Miles_per_Gallon)) {
      const px = x(r.Miles_per_Gallon);
      let k = 0, py = 0;
      for (;;) {
        py = (k % 2 ? 1 : -1) * Math.ceil(k / 2) * (R * 2 + 0.6);
        if (placed.every((p) => (p[0] - px) ** 2 + (p[1] - py) ** 2 >= (2 * R + 0.6) ** 2)) break;
        k++;
      }
      placed.push([px, py]);
    }
    const lo = d3.min(placed, (p) => p[1]) - R, hi = d3.max(placed, (p) => p[1]) + R;
    return { o, placed, lo, hi };
  });
  const need = d3.sum(swarms, (s) => s.hi - s.lo) + GAP * (origins.length - 1);
  const f0 = frame(640, 400, { title: "", subtitle: "x", source: "x", left: 60, bottom: 26 });
  const f = frame(640, Math.ceil(need + f0.y0 + (400 - f0.y1)), { title: "Fuel economy by origin", subtitle: "Each dot is one car. Dots pile up where many cars share a value", source: "Source: vega-datasets, cars.json", left: 60, bottom: 26 });
  let top = f.y0;
  const body = [
    x.ticks(8).map((v) => [el("line", { x1: x(v), x2: x(v), y1: f.y0, y2: f.y1, stroke: t.grid }), txt(String(v), { x: x(v), y: f.y1 + 16, "font-size": SIZE.label, fill: t.muted, "text-anchor": "middle" })]),
    swarms.map((s, i) => {
      const cy = top - s.lo;
      top += s.hi - s.lo + GAP;
      return [txt(s.o, { x: f.x0 - 14, y: cy + 4, "font-size": 12, "font-weight": 600, fill: t.text, "text-anchor": "end" }),
        s.placed.map(([px, py]) => el("circle", { cx: px, cy: cy + py, r: R, fill: t.cat[i], "fill-opacity": 0.9 }))];
    }),
    txt("Miles per gallon", { x: f.x1, y: f.y1 + 30, "font-size": SIZE.small, fill: t.faint, "text-anchor": "end" }),
  ];
  return svg(t, f, body.flat(2).join(""));
});

chart("contour", 2, "Density contours", "penguins.json", (t) => {
  const kx = "Beak Length (mm)", ky = "Beak Depth (mm)";
  const rows = D.penguins.filter((r) => r[kx] != null && r[ky] != null);
  const f = frame(640, 440, { title: "Where the penguins cluster", subtitle: "Beak length against beak depth in millimeters. The rings show the density of each species", legend: SPECIES.map((s, i) => ({ label: s, color: t.cat[i] })), source: "Source: vega-datasets, penguins.json", left: 30 });
  const x = d3.scaleLinear([30, 61], [f.x0, f.x1]);
  const y = d3.scaleLinear([13, 22], [f.y1, f.y0]);
  const path = d3.geoPath();
  const body = [
    yGrid(t, f, y, y.ticks(5), String),
    xAxis(t, f, x, x.ticks(7), String),
    SPECIES.map((s, i) => {
      const pts = rows.filter((r) => r.Species === s);
      const cs = d3.contourDensity().x((r) => x(r[kx]) - f.x0).y((r) => y(r[ky]) - f.y0).size([f.x1 - f.x0, f.y1 - f.y0]).bandwidth(16).thresholds(6)(pts);
      return g({ transform: `translate(${f.x0},${f.y0})` }, cs.map((c, k) => el("path", { d: path(c), fill: t.cat[i], "fill-opacity": 0.06 + k * 0.035, stroke: t.cat[i], "stroke-width": 1, "stroke-opacity": 0.8 })));
    }),
    rows.map((r) => el("circle", { cx: x(r[kx]), cy: y(r[ky]), r: 1.6, fill: t.cat[SPECIES.indexOf(r.Species)], "fill-opacity": 0.7 })),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("hexbin", 2, "Hexagonal bins", "gapminder.json", (t) => {
  const rows = D.gapminder.filter((r) => r.fertility != null && r.life_expect != null);
  const f = frame(640, 440, { title: "Fifty years of fertility and life span", subtitle: "Every country in every fifth year from 1955 to 2005. Darker cells hold more", source: "Source: Gapminder Foundation, via vega-datasets gapminder.json", left: 30, bottom: 34, top: 18 });
  const x = d3.scaleLinear([1, 8.5], [f.x0, f.x1]);
  const y = d3.scaleLinear([25, 90], [f.y1, f.y0]);
  const hb = d3hexbin().x((r) => x(r.fertility)).y((r) => y(r.life_expect)).radius(12).extent([[f.x0, f.y0], [f.x1, f.y1]]);
  const bins = hb(rows);
  const color = seqScale(t, [1, d3.max(bins, (b) => b.length)]);
  const body = [
    yGrid(t, f, y, y.ticks(6), String),
    xAxis(t, f, x, x.ticks(8), String, { line: false }),
    txt("Children per woman", { x: f.x1, y: f.y1 + 34, "font-size": SIZE.small, fill: t.faint, "text-anchor": "end" }),
    txt("Life expectancy in years", { x: f.x0 - 30, y: f.y0 - 12, "font-size": SIZE.small, fill: t.faint }),
    bins.map((b) => el("path", { d: `M${b.x},${b.y}` + hb.hexagon(11), fill: color(b.length), stroke: t.bg, "stroke-width": 0.5 })),
  ];
  return svg(t, f, body.flat().join(""));
});

chart("wordcloud", 2, "Word cloud", "flare.json", (t) => {
  const root = flareRoot();
  const words = d3.sort(root.leaves(), (d) => -d.value).slice(0, 70);
  const f = frame(640, 420, { title: "The largest classes of Flare", subtitle: "The 70 largest classes. The size of a name is the size of its source", source: "Source: vega-datasets, flare.json", left: 0 });
  const fs = d3.scaleSqrt(d3.extent(words, (d) => d.value), [10, 40]);
  const cx = (f.x0 + f.x1) / 2, cy = (f.y0 + f.y1) / 2;
  const boxes = [];
  const out = [];
  for (const wd of words) {
    const size = fs(wd.value), w = est(wd.data.name, size) * 0.98, h = size * 0.92;
    for (let k = 0; k < 4000; k++) {
      const a = 0.35 * k, rr = 1.6 * a;
      const x = cx + rr * Math.cos(a) * 1.5 - w / 2, y = cy + rr * Math.sin(a) - h / 2;
      if (x < f.x0 || x + w > f.x1 || y < f.y0 || y + h > f.y1) continue;
      if (boxes.some((b) => x < b.x + b.w + 3 && x + w + 3 > b.x && y < b.y + b.h + 1 && y + h + 1 > b.y)) continue;
      boxes.push({ x, y, w, h });
      out.push(txt(wd.data.name, { x: x + w / 2, y: y + h * 0.8, "font-size": size, "font-weight": size > 22 ? 600 : 500, fill: t.cat[FLARE_TOPS.indexOf(wd.top)], "text-anchor": "middle" }));
      break;
    }
  }
  return svg(t, f, out.join(""));
});

// ---- tier 3

chart("flowchart", 3, "Flowchart with a layered layout", "the zuzu pipeline", (t) => {
  const N = [
    ["vl", "Vega-Lite spec", "in"], ["dot", "DOT or Mermaid", "in"], ["sql", "SQL result", "in"],
    ["c", "compile", "step"], ["v", "Vega spec", "data"], ["gl", "graph layout", "step"],
    ["df", "dataflow", "step"], ["sc", "scales and guides", "step"], ["th", "theme", "in"],
    ["sg", "scene graph", "data"], ["svg", "SVG writer", "out"], ["ok", "okage raster", "out"], ["dr", "direct draw", "out"], ["js", "browser canvas", "out"],
  ];
  const E = [["vl", "c"], ["c", "v"], ["v", "df"], ["sql", "df", 1], ["df", "sc"], ["sc", "sg"], ["th", "sg"], ["dot", "gl"], ["gl", "sg"], ["sg", "svg"], ["svg", "ok"], ["sg", "dr"], ["sg", "js", 1]];
  const gr = new dagre.graphlib.Graph();
  gr.setGraph({ rankdir: "LR", nodesep: 12, ranksep: 22, marginx: 0, marginy: 0 });
  gr.setDefaultEdgeLabel(() => ({}));
  for (const [id, label] of N) gr.setNode(id, { label, width: est(label, 12) + 20, height: 30 });
  for (const [a, b] of E) gr.setEdge(a, b);
  dagre.layout(gr);
  const f = frame(640, 290, { title: "How zuzu draws a chart", subtitle: "The planned stages. A dashed edge is later work", source: "Layout: dagre, as Mermaid uses", left: 0 });
  const gw = gr.graph().width, gh = gr.graph().height;
  const s = Math.min(1, (f.x1 - f.x0) / gw, (f.y1 - f.y0) / gh);
  const ox = f.x0 + ((f.x1 - f.x0) - gw * s) / 2, oy = f.y0 + ((f.y1 - f.y0) - gh * s) / 2;
  const kind = Object.fromEntries(N.map(([id, , k]) => [id, k]));
  const fillOf = { in: t.cat[0], data: t.cat[2], step: t.text, out: t.cat[1] };
  const line = d3.line().curve(d3.curveBasis);
  const defs = el("marker", { id: "arrow", viewBox: "0 0 10 10", refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: "auto-start-reverse" },
    el("path", { d: "M0,1L9,5L0,9Z", fill: t.faint }));
  const body = g({ transform: `translate(${ox},${oy}) scale(${r2(s)})` },
    E.map(([a, b, later]) => el("path", { d: line(gr.edge(a, b).points.map((p) => [p.x, p.y])), fill: "none", stroke: t.faint, "stroke-width": 1.5, "stroke-dasharray": later ? "4 4" : null, "marker-end": "url(#arrow)" })),
    N.map(([id, label]) => {
      const n = gr.node(id), k = kind[id], c = fillOf[k];
      return g({ transform: `translate(${n.x - n.width / 2},${n.y - n.height / 2})` },
        el("rect", { width: n.width, height: n.height, rx: k === "data" ? 16 : 8, fill: k === "step" ? t.panel : c, "fill-opacity": k === "step" ? 1 : 0.16, stroke: k === "step" ? t.axis : c, "stroke-width": 1.25 }),
        txt(label, { x: n.width / 2, y: n.height / 2 + 4, "font-size": 12, "font-weight": k === "data" ? 600 : 500, fill: t.text, "text-anchor": "middle" }));
    }));
  return svg(t, f, body, defs);
});

// ---------------------------------------------------------------- main

const OUT = new URL("./svg/", import.meta.url);
mkdirSync(OUT, { recursive: true });
for (const c of CHARTS) {
  for (const th of Object.values(THEMES)) writeFileSync(new URL(`${c.id}-${th.name}.svg`, OUT), c.fn(th));
}

// The same charts in each color scheme and each mode, to show that a scheme
// swaps without a change to the chart.
const SWAP = ["stacked-area", "donut", "treemap", "bar-grouped"];
const OTHER = Object.keys(SCHEMES).filter((s) => s !== "zuzu");
for (const id of SWAP) {
  const c = CHARTS.find((x) => x.id === id);
  for (const s of OTHER) for (const m of Object.keys(MODES)) writeFileSync(new URL(`${id}-${s}-${m}.svg`, OUT), c.fn(theme(m, s)));
}
const schemeCards = SWAP.map((id) => `
    <figure>
      <figcaption><b>${esc(CHARTS.find((x) => x.id === id).title)}</b></figcaption>
      <div class="schemes">${["zuzu", ...OTHER].map((s) => `<div><span>${s}</span>${["light", "dark"].map((m) => `<img src="svg/${id}${s === "zuzu" ? "" : "-" + s}-${m}.svg" alt="${esc(id)}, ${s}, ${m}" loading="lazy">`).join("")}</div>`).join("")}</div>
    </figure>`).join("");

const TIER = ["Tier 0, the basics", "Tier 1, the common charts", "Tier 2, the specialist charts", "Tier 3, diagrams"];
const cards = (tier) => CHARTS.filter((c) => c.tier === tier).map((c) => `
    <figure id="${c.id}">
      <div class="pair"><img src="svg/${c.id}-light.svg" alt="${esc(c.title)}, light theme" loading="lazy"><img src="svg/${c.id}-dark.svg" alt="${esc(c.title)}, dark theme" loading="lazy"></div>
      <figcaption><b>${esc(c.title)}</b><span>${esc(c.data)}</span><a href="svg/${c.id}-light.svg">light</a><a href="svg/${c.id}-dark.svg">dark</a></figcaption>
    </figure>`).join("");
writeFileSync(new URL("./index.html", import.meta.url), `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>zuzu Targets</title>
<!-- Generated by gen-targets.mjs. Do not edit. README.md in this folder says what it is. -->
<style>
:root { --bg: #e9ebef; --fg: #1b2230; --muted: #5f6b7d; color-scheme: light; }
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--fg); font: 15px/1.5 ${FONT}; }
main { max-width: 1440px; margin: 0 auto; padding: 32px 16px 64px; display: grid; gap: 28px; }
h1 { margin: 0; font-size: 1.8rem; } h2 { margin: 12px 0 0; font-size: 1.2rem; }
p { margin: 0; max-width: 80ch; color: var(--muted); }
section { display: grid; gap: 18px; }
figure { margin: 0; display: grid; gap: 8px; }
.pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.pair img { width: 100%; height: auto; display: block; border-radius: 12px; box-shadow: 0 1px 2px rgb(0 0 0 / .08), 0 4px 16px rgb(0 0 0 / .06); }
figcaption { display: flex; flex-wrap: wrap; gap: 4px 14px; align-items: baseline; font-size: .85rem; color: var(--muted); }
figcaption b { color: var(--fg); font-size: .95rem; }
figcaption a { color: inherit; }
.schemes { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.schemes div { display: grid; gap: 8px; align-content: start; }
.schemes span { font-size: .85rem; font-weight: 600; }
.schemes img { width: 100%; height: auto; display: block; border-radius: 10px; box-shadow: 0 1px 2px rgb(0 0 0 / .08), 0 4px 16px rgb(0 0 0 / .06); }
@media (max-width: 900px) { .pair, .schemes { grid-template-columns: 1fr; } }
</style>
</head>
<body>
<main>
  <header style="display:grid;gap:8px">
    <h1>zuzu Targets</h1>
    <p>The look that zuzu aims for, at the quality of nivo and visx, in the light and the dark mode, drawn from real data. These are targets and not output of zuzu. docs/STYLE.md holds the rules they follow, and gen-targets.mjs draws them.</p>
  </header>
  ${TIER.map((name, i) => `<section><h2>${name}</h2>${cards(i)}</section>`).join("\n  ")}
  <section id="schemes"><h2>Color schemes</h2><p>A theme is a mode and a color scheme. The same charts in each scheme, in both modes. The chart does not change, only the scheme. catppuccin and solarized come from the palettes of those editor themes, and bring their own background and text colors.</p>${schemeCards}</section>
</main>
</body>
</html>
`);
console.log(`wrote ${CHARTS.length * 2 + SWAP.length * OTHER.length * 2} SVG files and index.html`);
