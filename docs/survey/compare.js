// compare.js draws compare.html. Each library has one adapter for each kind
// of chart, and every adapter draws the same rows from data.js.
//
// The adapters keep each library's defaults: its palette, fonts, spacing and
// animation. Only the dark mode is set by hand, and only for a library that
// has no dark theme of its own. The card says which.
(() => {
  "use strict";

  const D = window.SURVEY_DATA;
  const P = prep(D);
  const h = window.React ? React.createElement : null;

  // ---------------------------------------------------------------- data

  function prep(D) {
    const P = {};
    const MON = { Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5, Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11 };
    const iso = (t) => new Date(t).toISOString().slice(0, 10);
    const utc = (s) => Date.parse(s + "T00:00:00Z");
    const mean = (a) => a.reduce((s, v) => s + v, 0) / a.length;
    const round = (v, n = 2) => Math.round(v * 10 ** n) / 10 ** n;

    // stocks: monthly prices of five symbols
    P.symbols = ["AAPL", "AMZN", "GOOG", "IBM", "MSFT"];
    P.stocks = D.stocks.map((r) => {
      const [m, d, y] = r.date.split(" ");
      const t = Date.UTC(+y, MON[m], +d);
      return { symbol: r.symbol, t, date: new Date(t), iso: iso(t), price: r.price };
    });
    P.stockBy = Object.fromEntries(P.symbols.map((s) => [s, P.stocks.filter((r) => r.symbol === s)]));
    const ts = [...new Set(P.stocks.map((r) => r.t))].sort((a, b) => a - b);
    P.stockWide = ts.map((t) => {
      const row = { t };
      for (const s of P.symbols) {
        const hit = P.stockBy[s].find((r) => r.t === t);
        row[s] = hit ? hit.price : null;
      }
      return row;
    });
    P.stockYears = [];
    for (let y = 2000; y <= 2009; y++) P.stockYears.push(y);
    P.stockYearly = Object.fromEntries(["AAPL", "AMZN", "IBM", "MSFT"].map((s) => [s, P.stockYears.map((y) =>
      round(mean(P.stockBy[s].filter((r) => r.date.getUTCFullYear() === y).map((r) => r.price)), 1))]));

    // weather: Seattle, 2012 to 2015
    P.wTypes = ["sun", "fog", "drizzle", "rain", "snow"];
    const months = [...new Set(D.weather.map((r) => r.date.slice(0, 7)))].sort();
    P.wMonths = months;
    P.wMonthT = months.map((m) => utc(m + "-01"));
    P.wMonthly = Object.fromEntries(P.wTypes.map((w) => [w, months.map((m) =>
      D.weather.filter((r) => r.weather === w && r.date.startsWith(m)).length)]));
    P.wLong = [];
    months.forEach((m, i) => P.wTypes.forEach((w) => P.wLong.push({ month: new Date(P.wMonthT[i]), weather: w, count: P.wMonthly[w][i] })));
    P.wWide = months.map((m, i) => Object.assign({ m }, Object.fromEntries(P.wTypes.map((w) => [w, P.wMonthly[w][i]]))));
    P.wCounts = P.wTypes.map((w) => ({ weather: w, count: D.weather.filter((r) => r.weather === w).length }));

    // heat map: mean daily high by month and year
    P.monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    P.heatYears = [2012, 2013, 2014, 2015];
    P.heat = P.heatYears.map((y) => P.monthNames.map((_, mi) => {
      const k = `${y}-${String(mi + 1).padStart(2, "0")}`;
      return round(mean(D.weather.filter((r) => r.date.startsWith(k)).map((r) => r.temp_max)), 1);
    }));
    P.heatCells = [];
    P.heatYears.forEach((y, yi) => P.monthNames.forEach((m, mi) => P.heatCells.push({ year: String(y), month: m, mi, yi, value: P.heat[yi][mi] })));

    // barley: yield by site, year and variety
    const total = (pred) => round(D.barley.filter(pred).reduce((s, r) => s + r.yield, 0), 1);
    P.sites = [...new Set(D.barley.map((r) => r.site))].sort((a, b) => total((r) => r.site === b) - total((r) => r.site === a));
    P.years = [1931, 1932];
    P.varieties = [...new Set(D.barley.map((r) => r.variety))].sort();
    P.barleyYear = Object.fromEntries(P.years.map((y) => [y, P.sites.map((s) => total((r) => r.site === s && r.year === y))]));
    P.barleyVar = Object.fromEntries(P.varieties.map((v) => [v, P.sites.map((s) => total((r) => r.site === s && r.variety === v))]));
    P.barleyLong = [];
    P.sites.forEach((s, i) => P.years.forEach((y) => P.barleyLong.push({ site: s, year: String(y), yield: P.barleyYear[y][i] })));
    P.barleyYearWide = P.sites.map((s, i) => ({ site: s, y1931: P.barleyYear[1931][i], y1932: P.barleyYear[1932][i] }));
    P.barleyVarWide = P.sites.map((s, i) => Object.assign({ site: s }, Object.fromEntries(P.varieties.map((v) => [v, P.barleyVar[v][i]]))));

    // cars
    P.origins = ["Europe", "Japan", "USA"];
    const cars = D.cars.filter((r) => r.Horsepower != null && r.Miles_per_Gallon != null);
    P.scatter = Object.fromEntries(P.origins.map((o) => [o, cars.filter((r) => r.Origin === o).map((r) => [r.Horsepower, r.Miles_per_Gallon])]));
    P.mpg = D.cars.map((r) => r.Miles_per_Gallon).filter((v) => v != null);
    P.bins = [];
    for (let x = 8; x < 48; x += 2) P.bins.push({ x0: x, x1: x + 2, label: `${x}–${x + 2}`, count: P.mpg.filter((v) => v >= x && v < x + 2).length });

    // penguins
    P.species = ["Adelie", "Chinstrap", "Gentoo"];
    P.penguins = D.penguins.filter((r) => r["Body Mass (g)"] != null);
    P.mass = Object.fromEntries(P.species.map((s) => [s, P.penguins.filter((r) => r.Species === s).map((r) => r["Body Mass (g)"]).sort((a, b) => a - b)]));
    const q = (a, p) => { const i = (a.length - 1) * p, lo = Math.floor(i), hi = Math.ceil(i); return a[lo] + (a[hi] - a[lo]) * (i - lo); };
    P.box = Object.fromEntries(P.species.map((s) => {
      const a = P.mass[s], q1 = q(a, 0.25), q3 = q(a, 0.75), iqr = q3 - q1;
      const lo = a.find((v) => v >= q1 - 1.5 * iqr), hi = [...a].reverse().find((v) => v <= q3 + 1.5 * iqr);
      return [s, { min: lo, q1, median: q(a, 0.5), q3, max: hi, out: a.filter((v) => v < lo || v > hi) }];
    }));
    P.measures = ["Beak Length (mm)", "Beak Depth (mm)", "Flipper Length (mm)", "Body Mass (g)"];
    P.measureNames = ["Beak length", "Beak depth", "Flipper length", "Body mass"];
    const means = Object.fromEntries(P.species.map((s) => [s, P.measures.map((m) =>
      mean(D.penguins.filter((r) => r.Species === s && r[m] != null).map((r) => r[m])))]));
    const maxes = P.measures.map((_, i) => Math.max(...P.species.map((s) => means[s][i])));
    P.radar = Object.fromEntries(P.species.map((s) => [s, means[s].map((v, i) => round(v / maxes[i], 3))]));

    // ohlc
    P.ohlc = D.ohlc.map((r) => Object.assign({}, r, { t: utc(r.date), when: new Date(utc(r.date)) }));

    // flare: the class hierarchy of the Flare toolkit
    const byId = new Map(D.flare.map((r) => [r.id, Object.assign({ children: [] }, r)]));
    let root = null;
    for (const n of byId.values()) {
      if (n.parent == null) root = n; else byId.get(n.parent).children.push(n);
    }
    const sum = (n) => (n.value = n.children.length ? n.children.reduce((s, c) => s + sum(c), 0) : (n.size || 0));
    sum(root);
    const strip = (n) => n.children.length ? { name: n.name, value: n.value, children: n.children.map(strip) } : { name: n.name, value: n.value };
    P.flareTree = strip(root);
    P.flareNodes = [...byId.values()].map((n) => ({ id: String(n.id), parent: n.parent == null ? "" : String(n.parent), name: n.name, value: n.value, size: n.children.length ? 0 : n.value }));
    P.flareLeaves = [];
    const walk = (n, path) => n.children.length ? n.children.forEach((c) => walk(c, path.concat(n.name))) : P.flareLeaves.push({ l1: path[1] || n.name, l2: path[2] || n.name, name: n.name, size: n.value });
    walk(root, []);
    P.flare2 = root.children.map((c) => ({ name: c.name, value: c.value, children: c.children.map((g) => ({ name: g.name, value: g.value })) }));

    // sankey: cars, from origin to number of cylinders
    const flows = new Map();
    for (const r of D.cars) {
      const k = `${r.Origin}|${r.Cylinders} cylinders`;
      flows.set(k, (flows.get(k) || 0) + 1);
    }
    P.links = [...flows].map(([k, v]) => { const [source, target] = k.split("|"); return { source, target, value: v }; })
      .sort((a, b) => a.source.localeCompare(b.source) || a.target.localeCompare(b.target));
    P.nodes = [...new Set(P.links.flatMap((l) => [l.source, l.target]))];

    // map: engineers per 1,000 people by state
    P.engineers = new Map(D.states.map((r) => [r.id, { state: r.state, value: round(r.engineers * 1000, 2) }]));
    P.stateGeo = topojson.feature(D.us, D.us.objects.states);
    for (const f of P.stateGeo.features) {
      const hit = P.engineers.get(+f.id);
      f.properties = { id: +f.id, name: hit ? hit.state : String(f.id), value: hit ? hit.value : null };
    }
    P.stateGeo.features = P.stateGeo.features.filter((f) => f.properties.value != null);
    return P;
  }

  // ---------------------------------------------------------- libraries

  const LIBS = [
    { id: "vegalite", name: "Vega-Lite", ver: "vega-lite 6.5.0, vega 6.4.0", tech: "SVG", theme: "theme \"dark\" from vega-themes" },
    { id: "vega", name: "Vega", ver: "vega 6.4.0", tech: "SVG", theme: "theme \"dark\" from vega-themes" },
    { id: "echarts", name: "ECharts", ver: "6.1.0", tech: "SVG renderer", theme: "built-in \"dark\" theme" },
    { id: "chartjs", name: "Chart.js", ver: "4.6.1", tech: "Canvas", theme: "dark by hand, through Chart.defaults" },
    { id: "plotly", name: "Plotly", ver: "plotly.js 4.1.2", tech: "SVG, WebGL", theme: "dark by hand. plotly.js ships no named template" },
    { id: "highcharts", name: "Highcharts", ver: "13.1.1", tech: "SVG", theme: "adaptive theme, a CSS class on the container" },
    { id: "apex", name: "ApexCharts", ver: "8.5.0", tech: "SVG", theme: "theme.mode \"dark\"" },
    { id: "plot", name: "Observable Plot", ver: "0.6.19", tech: "SVG", theme: "inherits the CSS color" },
    { id: "recharts", name: "Recharts", ver: "2.15.4, React 18", tech: "SVG", theme: "dark by hand, per axis" },
    { id: "uplot", name: "uPlot", ver: "1.6.32", tech: "Canvas", theme: "dark by hand, per axis" },
    { id: "lw", name: "Lightweight Charts", ver: "5.2.1", tech: "Canvas", theme: "dark by hand, through layout" },
    { id: "mermaid", name: "Mermaid", ver: "12.2.0", tech: "SVG", theme: "theme \"dark\"" },
    { id: "graphviz", name: "Graphviz", ver: "viz-js 3.31.0", tech: "SVG", theme: "dark by hand, as graph attributes" },
    { id: "mpl", name: "Matplotlib", ver: "3.11.2, static SVG", tech: "SVG file", theme: "style \"dark_background\"" },
  ];
  const LIB = Object.fromEntries(LIBS.map((l) => [l.id, l]));

  const ROWS = [
    { id: "line", title: "Multi-series line", data: "Monthly price of five stocks, 2000 to 2010. stocks.csv" },
    { id: "area", title: "Stacked area", data: "Days of each kind of weather in Seattle, by month, 2012 to 2015. seattle-weather.csv" },
    { id: "group", title: "Grouped bar", data: "Total barley yield by site, 1931 against 1932. barley.json" },
    { id: "stack", title: "Stacked horizontal bar", data: "Barley yield by site, stacked by variety. barley.json" },
    { id: "scatter", title: "Scatter", data: "Horsepower against miles per gallon, by origin. cars.json" },
    { id: "hist", title: "Histogram", data: "Miles per gallon of 398 cars, in bins of 2. cars.json" },
    { id: "pie", title: "Pie", data: "Days of each kind of weather in Seattle, 2012 to 2015. seattle-weather.csv" },
    { id: "heat", title: "Heat map", data: "Mean daily high in °C, by month and year. seattle-weather.csv" },
    { id: "box", title: "Box plot", data: "Body mass in grams, by species. penguins.json" },
    { id: "candle", title: "Candlestick", data: "Daily open, high, low and close of the CBOE VIX, June to August 2009. ohlc.json" },
    { id: "radar", title: "Radar", data: "Mean of four measures by species, each divided by its largest mean. penguins.json" },
    { id: "treemap", title: "Treemap", data: "Size of each class of the Flare toolkit. flare.json" },
    { id: "sunburst", title: "Sunburst", data: "The same Flare hierarchy. flare.json" },
    { id: "sankey", title: "Sankey", data: "Cars from each origin to each number of cylinders. cars.json" },
    { id: "map", title: "Choropleth map", data: "Engineers per 1,000 people, by state. population_engineers_hurricanes.csv, us-10m.json" },
    { id: "dag", title: "Flowchart (graph layout)", data: "The pipeline planned for zuzu, as a graph." },
  ];

  // R[row][lib] is { kind, note, draw }. kind is native, plugin, composed,
  // prep or none. draw(el, theme) draws into el and returns a function that
  // removes the chart, or a promise of one.
  const R = {};
  const def = (row, lib, kind, note, draw) => ((R[row] ||= {})[lib] = { kind, note, draw });
  const none = (row, lib, note) => def(row, lib, "none", note, null);

  // ------------------------------------------------------------ adapters

  // compare.html?still turns off every animation, so that a screenshot or a
  // page that is not in view shows each chart complete.
  const STILL = new URLSearchParams(location.search).has("still");

  // Vega-Lite and Vega, through vega-embed.
  const embed = async (el, spec, t) => {
    const r = await vegaEmbed(el, spec, { actions: false, renderer: "svg", theme: t.dark ? "dark" : undefined });
    return () => r.finalize();
  };
  const vl = (spec) => (el, t) => embed(el, Object.assign({
    $schema: "https://vega.github.io/schema/vega-lite/v6.json",
    width: "container", height: 240, background: "transparent",
  }, typeof spec === "function" ? spec(t) : spec), t);
  const vg = (spec) => (el, t) => embed(el, Object.assign({
    $schema: "https://vega.github.io/schema/vega/v6.json",
    width: Math.max(300, el.clientWidth - 20), height: 290, background: "transparent", padding: 4,
  }, spec), t);

  const ec = (fn) => (el, t) => {
    const c = echarts.init(el, t.dark ? "dark" : null, { renderer: "svg" });
    c.setOption(Object.assign({ backgroundColor: "transparent" }, STILL ? { animation: false } : {}, fn(t)));
    return () => c.dispose();
  };

  const cj = (fn) => (el, t) => {
    Chart.defaults.color = t.dark ? t.fg : "#666";
    Chart.defaults.borderColor = t.dark ? t.grid : "rgba(0,0,0,0.1)";
    const cv = document.createElement("canvas");
    el.append(cv);
    const cfg = fn(t);
    cfg.options = Object.assign({ responsive: true, maintainAspectRatio: false }, cfg.options);
    // The colors plugin turns itself off when the defaults change, as they
    // do in the dark. forceOverride keeps the library's own palette.
    // The plugin types set their own colors, so they are left alone.
    if (["line", "bar", "scatter", "radar", "pie"].includes(cfg.type)) {
      cfg.options.plugins = Object.assign({ colors: { forceOverride: true } }, cfg.options.plugins);
    }
    if (STILL) cfg.options.animation = false;
    const c = new Chart(cv, cfg);
    return () => c.destroy();
  };

  const pl = (fn) => (el, t) => {
    const { data, layout = {} } = fn(t);
    const L = Object.assign({ height: 300, margin: { l: 50, r: 16, t: 16, b: 40 } }, layout);
    if (t.dark) {
      Object.assign(L, { paper_bgcolor: "rgba(0,0,0,0)", plot_bgcolor: "rgba(0,0,0,0)", font: { color: t.fg } });
      for (const k of ["xaxis", "yaxis"]) L[k] = Object.assign({ gridcolor: t.grid, zerolinecolor: t.grid, linecolor: t.grid }, L[k]);
      if (L.geo) Object.assign(L.geo, { bgcolor: "rgba(0,0,0,0)", lakecolor: "rgba(0,0,0,0)", landcolor: t.card });
      if (L.polar) L.polar = Object.assign({ bgcolor: "rgba(0,0,0,0)", angularaxis: { gridcolor: t.grid }, radialaxis: { gridcolor: t.grid } }, L.polar);
    }
    Plotly.newPlot(el, data, L, { displayModeBar: false, responsive: true });
    return () => Plotly.purge(el);
  };

  const hc = (fn, ctor = "chart") => (el, t) => {
    el.classList.remove("highcharts-light", "highcharts-dark");
    el.classList.add(t.dark ? "highcharts-dark" : "highcharts-light");
    const o = fn(t);
    o.chart = Object.assign({ height: 300 }, o.chart);
    o.title = o.title || { text: "" };
    if (STILL) o.plotOptions = Object.assign({ series: { animation: false } }, o.plotOptions);
    const c = Highcharts[ctor](el, o);
    return () => c.destroy();
  };

  const ap = (fn) => (el, t) => {
    const o = fn(t);
    o.chart = Object.assign({ height: 300, background: "transparent", toolbar: { show: false } }, o.chart);
    o.theme = { mode: t.dark ? "dark" : "light" };
    if (STILL) o.chart.animations = { enabled: false };
    const c = new ApexCharts(el, o);
    c.render();
    return () => c.destroy();
  };

  const pt = (fn) => (el, t) => {
    const o = fn(t);
    const node = Plot.plot(Object.assign({ width: el.clientWidth || 400, height: 290, style: { background: "transparent" } }, o));
    el.append(node);
    return () => el.replaceChildren();
  };

  const rc = (fn) => (el, t) => {
    const root = ReactDOM.createRoot(el);
    root.render(h(Recharts.ResponsiveContainer, { width: "100%", height: 300 }, STILL ? still(fn(t)) : fn(t)));
    return () => root.unmount();
  };
  // still turns off the animation of every Recharts element in a tree.
  const still = (node) => {
    if (!React.isValidElement(node)) return node;
    const kids = React.Children.map(node.props.children, still);
    const props = typeof node.type === "string" ? {} : { isAnimationActive: false };
    return React.cloneElement(node, props, ...(kids || []));
  };
  // ax gives a Recharts axis readable colors in the dark.
  const ax = (t, extra) => Object.assign(t.dark ? { tick: { fill: t.muted }, stroke: t.grid } : {}, extra);

  const up = (fn) => (el, t) => {
    const { opts, data } = fn(t);
    const axis = t.dark ? { stroke: t.fg, grid: { stroke: t.grid }, ticks: { stroke: t.grid } } : {};
    opts.axes = (opts.axes || [{}, {}]).map((a) => Object.assign({}, axis, a));
    const u = new uPlot(Object.assign({ width: (el.clientWidth || 400) - 12, height: 270 }, opts), data, el);
    return () => u.destroy();
  };

  const lw = (fn) => (el, t) => {
    const o = { width: (el.clientWidth || 400) - 12, height: 300 };
    if (t.dark) {
      o.layout = { background: { type: "solid", color: "transparent" }, textColor: t.fg };
      o.grid = { vertLines: { color: t.grid }, horzLines: { color: t.grid } };
    }
    const c = LightweightCharts.createChart(el, o);
    fn(c, t);
    c.timeScale().fitContent();
    return () => c.remove();
  };

  // Mermaid renders one diagram at a time, because initialize is global.
  let mmQueue = Promise.resolve();
  let mmN = 0;
  const mm = (text) => (el, t) => {
    const job = mmQueue.then(async () => {
      mermaid.initialize({ startOnLoad: false, theme: t.dark ? "dark" : "default", securityLevel: "strict" });
      const { svg } = await mermaid.render("mm" + ++mmN, typeof text === "function" ? text() : text);
      el.innerHTML = svg;
      const s = el.querySelector("svg");
      if (s) { s.style.maxWidth = "100%"; s.style.maxHeight = "300px"; }
      return () => el.replaceChildren();
    });
    mmQueue = job.catch(() => {});
    return job;
  };

  let vizP = null;
  const gv = (dot) => async (el, t) => {
    vizP ||= Viz.instance();
    const viz = await vizP;
    const svg = viz.renderSVGElement(dot, t.dark ? {
      graphAttributes: { bgcolor: "transparent", fontcolor: t.fg },
      nodeAttributes: { color: t.fg, fontcolor: t.fg },
      edgeAttributes: { color: t.muted, fontcolor: t.fg },
    } : { graphAttributes: { bgcolor: "transparent" } });
    svg.style.maxWidth = "100%";
    svg.style.height = "auto";
    el.append(svg);
    return () => el.replaceChildren();
  };

  // st shows a static SVG made by gen-static.py.
  const st = (name) => (el, t) => {
    const img = new Image();
    img.alt = `Matplotlib, ${name}`;
    img.src = `static/mpl-${name}-${t.dark ? "dark" : "light"}.svg`;
    img.onerror = () => { el.innerHTML = '<p class="msg">Not generated. Run <code>gen-static.py</code>, as docs/survey/README.md says.</p>'; };
    el.append(img);
    return () => el.replaceChildren();
  };

  // A palette for the libraries that have none. uPlot, Lightweight Charts
  // and a Recharts pie draw every series in one color unless told otherwise.
  const TEN = ["#4c78a8", "#f58518", "#e45756", "#72b7b2", "#54a24b", "#eeca3b", "#b279a2", "#ff9da6", "#9d755d", "#bab0ac"];

  // ---------------------------------------------------------------- line

  def("line", "vegalite", "native", "", vl({
    data: { values: P.stocks.map((r) => ({ symbol: r.symbol, date: r.iso, price: r.price })) },
    mark: "line",
    encoding: { x: { field: "date", type: "temporal" }, y: { field: "price", type: "quantitative" }, color: { field: "symbol", type: "nominal" } },
  }));
  def("line", "echarts", "native", "", ec(() => ({
    tooltip: { trigger: "axis" }, legend: {}, grid: { left: 50, right: 20, bottom: 30 },
    xAxis: { type: "time" }, yAxis: { type: "value" },
    series: P.symbols.map((s) => ({ name: s, type: "line", showSymbol: false, data: P.stockBy[s].map((r) => [r.t, r.price]) })),
  })));
  def("line", "chartjs", "plugin", "needs a date adapter for a time axis", cj(() => ({
    type: "line",
    data: { datasets: P.symbols.map((s) => ({ label: s, data: P.stockBy[s].map((r) => ({ x: r.t, y: r.price })), pointRadius: 0, borderWidth: 1.5 })) },
    options: { scales: { x: { type: "time" } } },
  })));
  def("line", "plotly", "native", "", pl(() => ({
    data: P.symbols.map((s) => ({ type: "scatter", mode: "lines", name: s, x: P.stockBy[s].map((r) => r.iso), y: P.stockBy[s].map((r) => r.price) })),
  })));
  def("line", "highcharts", "native", "", hc(() => ({
    xAxis: { type: "datetime" }, yAxis: { title: { text: "price" } },
    series: P.symbols.map((s) => ({ name: s, data: P.stockBy[s].map((r) => [r.t, r.price]) })),
  })));
  def("line", "apex", "native", "", ap(() => ({
    chart: { type: "line" }, stroke: { width: 2 }, xaxis: { type: "datetime" },
    series: P.symbols.map((s) => ({ name: s, data: P.stockBy[s].map((r) => [r.t, r.price]) })),
  })));
  def("line", "plot", "native", "", pt(() => ({
    color: { legend: true }, y: { grid: true },
    marks: [Plot.lineY(P.stocks, { x: "date", y: "price", stroke: "symbol" })],
  })));
  def("line", "recharts", "prep", "the caller pivots the rows into one row per date", rc((t) =>
    h(Recharts.LineChart, { data: P.stockWide, margin: { top: 10, right: 16, bottom: 0, left: 0 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { dataKey: "t", type: "number", scale: "time", domain: ["dataMin", "dataMax"], tickFormatter: (v) => new Date(v).getUTCFullYear() })),
      h(Recharts.YAxis, ax(t)),
      h(Recharts.Tooltip, { labelFormatter: (v) => new Date(v).toISOString().slice(0, 10) }),
      h(Recharts.Legend),
      ...P.symbols.map((s, i) => h(Recharts.Line, { key: s, dataKey: s, dot: false, stroke: TEN[i], connectNulls: false, isAnimationActive: true })))));
  def("line", "uplot", "prep", "columnar arrays, seconds, and a color for each series", up((t) => ({
    opts: { series: [{}].concat(P.symbols.map((s, i) => ({ label: s, stroke: TEN[i], width: 1.5 }))), axes: [{}, {}] },
    data: [P.stockWide.map((r) => r.t / 1000)].concat(P.symbols.map((s) => P.stockWide.map((r) => r[s]))),
  })));
  def("line", "lw", "native", "one color for every series unless told", lw((c) => {
    P.symbols.forEach((s, i) => c.addSeries(LightweightCharts.LineSeries, { color: TEN[i], lineWidth: 2, title: s })
      .setData(P.stockBy[s].map((r) => ({ time: r.iso, value: r.price }))));
  }));
  def("line", "mermaid", "prep", "xychart: categories on x, so yearly means", mm(() =>
    `xychart-beta\n  x-axis [${P.stockYears.join(", ")}]\n  y-axis "price"\n` +
    Object.values(P.stockYearly).map((v) => `  line [${v.join(", ")}]`).join("\n")));
  def("line", "mpl", "native", "", st("line"));

  // ---------------------------------------------------------------- area

  def("area", "vegalite", "native", "aggregates the raw rows itself", vl({
    data: { values: D.weather },
    mark: "area",
    encoding: {
      x: { timeUnit: "yearmonth", field: "date", type: "temporal" },
      y: { aggregate: "count", type: "quantitative" },
      color: { field: "weather", type: "nominal", sort: P.wTypes },
    },
  }));
  def("area", "echarts", "prep", "", ec(() => ({
    tooltip: { trigger: "axis" }, legend: {}, grid: { left: 40, right: 20, bottom: 30 },
    xAxis: { type: "category", boundaryGap: false, data: P.wMonths }, yAxis: { type: "value" },
    series: P.wTypes.map((w) => ({ name: w, type: "line", stack: "all", areaStyle: {}, showSymbol: false, data: P.wMonthly[w] })),
  })));
  def("area", "chartjs", "prep", "", cj(() => ({
    type: "line",
    data: { labels: P.wMonths, datasets: P.wTypes.map((w) => ({ label: w, data: P.wMonthly[w], fill: true, pointRadius: 0 })) },
    options: { scales: { y: { stacked: true } } },
  })));
  def("area", "plotly", "prep", "", pl(() => ({
    data: P.wTypes.map((w) => ({ type: "scatter", mode: "lines", stackgroup: "one", name: w, x: P.wMonths, y: P.wMonthly[w] })),
  })));
  def("area", "highcharts", "prep", "", hc(() => ({
    chart: { type: "area" }, plotOptions: { area: { stacking: "normal", marker: { enabled: false } } },
    xAxis: { categories: P.wMonths }, yAxis: { title: { text: "days" } },
    series: P.wTypes.map((w) => ({ name: w, data: P.wMonthly[w] })),
  })));
  def("area", "apex", "prep", "", ap(() => ({
    chart: { type: "area", stacked: true }, dataLabels: { enabled: false }, xaxis: { categories: P.wMonths, tickAmount: 8 },
    series: P.wTypes.map((w) => ({ name: w, data: P.wMonthly[w] })),
  })));
  def("area", "plot", "prep", "stacks by itself, the caller counts", pt(() => ({
    color: { legend: true, domain: P.wTypes }, y: { grid: true },
    marks: [Plot.areaY(P.wLong, { x: "month", y: "count", fill: "weather" })],
  })));
  def("area", "recharts", "prep", "", rc((t) =>
    h(Recharts.AreaChart, { data: P.wWide, margin: { top: 10, right: 16, bottom: 0, left: 0 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { dataKey: "m" })), h(Recharts.YAxis, ax(t)), h(Recharts.Tooltip), h(Recharts.Legend),
      ...P.wTypes.map((w, i) => h(Recharts.Area, { key: w, dataKey: w, stackId: "1", stroke: TEN[i], fill: TEN[i] })))));
  none("area", "uplot", "no stacking. A demo builds it by hand");
  none("area", "lw", "one series per pane, no stacking");
  def("area", "mpl", "prep", "stackplot", st("area"));

  // --------------------------------------------------------------- group

  def("group", "vegalite", "native", "xOffset groups the bars", vl({
    data: { values: D.barley },
    mark: "bar",
    encoding: {
      x: { field: "site", type: "nominal", sort: P.sites },
      xOffset: { field: "year", type: "nominal" },
      y: { aggregate: "sum", field: "yield", type: "quantitative" },
      color: { field: "year", type: "nominal" },
    },
  }));
  def("group", "echarts", "prep", "", ec(() => ({
    tooltip: {}, legend: {}, grid: { left: 40, right: 10, bottom: 50 },
    xAxis: { type: "category", data: P.sites, axisLabel: { rotate: 30 } }, yAxis: { type: "value" },
    series: P.years.map((y) => ({ name: String(y), type: "bar", data: P.barleyYear[y] })),
  })));
  def("group", "chartjs", "prep", "", cj(() => ({
    type: "bar", data: { labels: P.sites, datasets: P.years.map((y) => ({ label: String(y), data: P.barleyYear[y] })) },
  })));
  def("group", "plotly", "prep", "", pl(() => ({
    data: P.years.map((y) => ({ type: "bar", name: String(y), x: P.sites, y: P.barleyYear[y] })), layout: { barmode: "group" },
  })));
  def("group", "highcharts", "prep", "", hc(() => ({
    chart: { type: "column" }, xAxis: { categories: P.sites }, yAxis: { title: { text: "yield" } },
    series: P.years.map((y) => ({ name: String(y), data: P.barleyYear[y] })),
  })));
  def("group", "apex", "prep", "", ap(() => ({
    chart: { type: "bar" }, dataLabels: { enabled: false }, xaxis: { categories: P.sites },
    series: P.years.map((y) => ({ name: String(y), data: P.barleyYear[y] })),
  })));
  def("group", "plot", "composed", "a facet for each site", pt(() => ({
    color: { legend: true }, fx: { label: null, domain: P.sites, tickRotate: 20 }, x: { axis: null }, y: { grid: true }, marginBottom: 50,
    marks: [Plot.barY(P.barleyLong, { fx: "site", x: "year", y: "yield", fill: "year" }), Plot.ruleY([0])],
  })));
  def("group", "recharts", "prep", "", rc((t) =>
    h(Recharts.BarChart, { data: P.barleyYearWide, margin: { top: 10, right: 10, bottom: 0, left: 0 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { dataKey: "site", interval: 0, fontSize: 11 })), h(Recharts.YAxis, ax(t)), h(Recharts.Tooltip), h(Recharts.Legend),
      h(Recharts.Bar, { dataKey: "y1931", name: "1931", fill: TEN[0] }), h(Recharts.Bar, { dataKey: "y1932", name: "1932", fill: TEN[1] }))));
  none("group", "mermaid", "xychart draws a second bar series over the first, not beside it");
  def("group", "mpl", "composed", "the caller offsets each bar", st("group"));

  // --------------------------------------------------------------- stack

  def("stack", "vegalite", "native", "", vl({
    data: { values: D.barley },
    mark: "bar",
    encoding: {
      y: { field: "site", type: "nominal", sort: P.sites },
      x: { aggregate: "sum", field: "yield", type: "quantitative" },
      color: { field: "variety", type: "nominal" },
    },
  }));
  def("stack", "echarts", "prep", "", ec(() => ({
    tooltip: {}, legend: { type: "scroll" }, grid: { left: 110, right: 20, bottom: 30, top: 40 },
    yAxis: { type: "category", data: P.sites }, xAxis: { type: "value" },
    series: P.varieties.map((v) => ({ name: v, type: "bar", stack: "all", data: P.barleyVar[v] })),
  })));
  def("stack", "chartjs", "prep", "", cj(() => ({
    type: "bar", data: { labels: P.sites, datasets: P.varieties.map((v) => ({ label: v, data: P.barleyVar[v] })) },
    options: { indexAxis: "y", scales: { x: { stacked: true }, y: { stacked: true } }, plugins: { legend: { labels: { boxWidth: 10, font: { size: 10 } } } } },
  })));
  def("stack", "plotly", "prep", "", pl(() => ({
    data: P.varieties.map((v) => ({ type: "bar", orientation: "h", name: v, y: P.sites, x: P.barleyVar[v] })),
    layout: { barmode: "stack", margin: { l: 110, r: 10, t: 10, b: 30 } },
  })));
  def("stack", "highcharts", "prep", "", hc(() => ({
    chart: { type: "bar" }, plotOptions: { series: { stacking: "normal" } }, xAxis: { categories: P.sites }, yAxis: { title: { text: "yield" } },
    series: P.varieties.map((v) => ({ name: v, data: P.barleyVar[v] })),
  })));
  def("stack", "apex", "prep", "", ap(() => ({
    chart: { type: "bar", stacked: true }, plotOptions: { bar: { horizontal: true } }, dataLabels: { enabled: false },
    xaxis: { categories: P.sites }, legend: { fontSize: "10px" },
    series: P.varieties.map((v) => ({ name: v, data: P.barleyVar[v] })),
  })));
  def("stack", "plot", "native", "groupY sums the raw rows", pt(() => ({
    color: { legend: true }, marginLeft: 110, x: { grid: true },
    marks: [Plot.barX(D.barley, Plot.groupY({ x: "sum" }, { y: "site", x: "yield", fill: "variety", sort: { y: "-x" } })), Plot.ruleX([0])],
  })));
  def("stack", "recharts", "prep", "", rc((t) =>
    h(Recharts.BarChart, { data: P.barleyVarWide, layout: "vertical", margin: { top: 10, right: 10, bottom: 0, left: 40 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { type: "number" })), h(Recharts.YAxis, ax(t, { type: "category", dataKey: "site", fontSize: 11 })),
      h(Recharts.Tooltip), h(Recharts.Legend, { wrapperStyle: { fontSize: 10 } }),
      ...P.varieties.map((v, i) => h(Recharts.Bar, { key: v, dataKey: v, stackId: "a", fill: TEN[i] })))));
  def("stack", "mpl", "composed", "the caller offsets each segment", st("stack"));

  // ------------------------------------------------------------- scatter

  def("scatter", "vegalite", "native", "", vl({
    data: { values: D.cars },
    mark: "point",
    encoding: { x: { field: "Horsepower", type: "quantitative" }, y: { field: "Miles_per_Gallon", type: "quantitative" }, color: { field: "Origin", type: "nominal" } },
  }));
  def("scatter", "echarts", "prep", "", ec(() => ({
    tooltip: {}, legend: {}, grid: { left: 40, right: 20, bottom: 30 },
    xAxis: { type: "value", name: "hp" }, yAxis: { type: "value", name: "mpg" },
    series: P.origins.map((o) => ({ name: o, type: "scatter", symbolSize: 6, data: P.scatter[o] })),
  })));
  def("scatter", "chartjs", "prep", "", cj(() => ({
    type: "scatter", data: { datasets: P.origins.map((o) => ({ label: o, data: P.scatter[o].map(([x, y]) => ({ x, y })), pointRadius: 3 })) },
  })));
  def("scatter", "plotly", "prep", "", pl(() => ({
    data: P.origins.map((o) => ({ type: "scatter", mode: "markers", name: o, x: P.scatter[o].map((p) => p[0]), y: P.scatter[o].map((p) => p[1]) })),
  })));
  def("scatter", "highcharts", "prep", "", hc(() => ({
    chart: { type: "scatter" }, xAxis: { title: { text: "hp" } }, yAxis: { title: { text: "mpg" } },
    series: P.origins.map((o) => ({ name: o, data: P.scatter[o], marker: { radius: 3 } })),
  })));
  def("scatter", "apex", "prep", "", ap(() => ({
    chart: { type: "scatter", zoom: { enabled: false } }, xaxis: { type: "numeric", tickAmount: 8 },
    series: P.origins.map((o) => ({ name: o, data: P.scatter[o] })),
  })));
  def("scatter", "plot", "native", "", pt(() => ({
    color: { legend: true }, grid: true,
    marks: [Plot.dot(D.cars, { x: "Horsepower", y: "Miles_per_Gallon", stroke: "Origin" })],
  })));
  def("scatter", "recharts", "prep", "", rc((t) =>
    h(Recharts.ScatterChart, { margin: { top: 10, right: 16, bottom: 0, left: 0 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { type: "number", dataKey: "x", name: "hp" })), h(Recharts.YAxis, ax(t, { type: "number", dataKey: "y", name: "mpg" })),
      h(Recharts.Tooltip), h(Recharts.Legend),
      ...P.origins.map((o, i) => h(Recharts.Scatter, { key: o, name: o, data: P.scatter[o].map(([x, y]) => ({ x, y })), fill: TEN[i] })))));
  def("scatter", "mpl", "prep", "", st("scatter"));

  // ---------------------------------------------------------------- hist

  def("hist", "vegalite", "native", "bins the raw rows", vl({
    data: { values: D.cars },
    mark: "bar",
    encoding: { x: { field: "Miles_per_Gallon", type: "quantitative", bin: { step: 2, extent: [8, 48] } }, y: { aggregate: "count", type: "quantitative" } },
  }));
  def("hist", "echarts", "prep", "the caller bins", ec(() => ({
    tooltip: {}, grid: { left: 40, right: 20, bottom: 30 },
    xAxis: { type: "category", data: P.bins.map((b) => b.x0) }, yAxis: { type: "value" },
    series: [{ type: "bar", barCategoryGap: "2%", data: P.bins.map((b) => b.count) }],
  })));
  def("hist", "chartjs", "prep", "the caller bins", cj(() => ({
    type: "bar", data: { labels: P.bins.map((b) => b.x0), datasets: [{ label: "cars", data: P.bins.map((b) => b.count), barPercentage: 1, categoryPercentage: 0.98 }] },
  })));
  def("hist", "plotly", "native", "", pl(() => ({ data: [{ type: "histogram", x: P.mpg, xbins: { start: 8, end: 48, size: 2 } }] })));
  def("hist", "highcharts", "plugin", "histogram module", hc(() => ({
    xAxis: [{ visible: false }, { title: { text: "mpg" } }], yAxis: [{ visible: false }, { title: { text: "cars" } }],
    series: [{ type: "histogram", xAxis: 1, yAxis: 1, baseSeries: "s", binWidth: 2, name: "cars" }, { id: "s", type: "scatter", data: P.mpg, visible: false, showInLegend: false }],
  })));
  def("hist", "apex", "prep", "the caller bins", ap(() => ({
    chart: { type: "bar" }, dataLabels: { enabled: false }, plotOptions: { bar: { columnWidth: "98%" } },
    xaxis: { categories: P.bins.map((b) => b.x0) }, series: [{ name: "cars", data: P.bins.map((b) => b.count) }],
  })));
  def("hist", "plot", "native", "", pt(() => ({
    y: { grid: true },
    marks: [Plot.rectY(D.cars, Plot.binX({ y: "count" }, { x: "Miles_per_Gallon", thresholds: d3.range(8, 50, 2) })), Plot.ruleY([0])],
  })));
  def("hist", "recharts", "prep", "the caller bins", rc((t) =>
    h(Recharts.BarChart, { data: P.bins, barCategoryGap: 1, margin: { top: 10, right: 10, bottom: 0, left: 0 } },
      h(Recharts.CartesianGrid, { strokeDasharray: "3 3", stroke: t.dark ? t.grid : undefined }),
      h(Recharts.XAxis, ax(t, { dataKey: "x0" })), h(Recharts.YAxis, ax(t)), h(Recharts.Tooltip),
      h(Recharts.Bar, { dataKey: "count", fill: TEN[0] }))));
  def("hist", "mpl", "native", "", st("hist"));

  // ----------------------------------------------------------------- pie

  def("pie", "vegalite", "native", "arc mark with theta", vl({
    data: { values: P.wCounts },
    mark: "arc",
    encoding: { theta: { field: "count", type: "quantitative" }, color: { field: "weather", type: "nominal", sort: P.wTypes } },
    view: { stroke: null },
  }));
  def("pie", "echarts", "native", "", ec(() => ({
    tooltip: {}, legend: { bottom: 0 },
    series: [{ type: "pie", radius: "65%", data: P.wCounts.map((r) => ({ name: r.weather, value: r.count })) }],
  })));
  def("pie", "chartjs", "native", "", cj(() => ({
    type: "pie", data: { labels: P.wTypes, datasets: [{ data: P.wCounts.map((r) => r.count) }] },
  })));
  def("pie", "plotly", "native", "", pl(() => ({ data: [{ type: "pie", labels: P.wTypes, values: P.wCounts.map((r) => r.count), sort: false }] })));
  def("pie", "highcharts", "native", "", hc(() => ({
    chart: { type: "pie" }, series: [{ name: "days", data: P.wCounts.map((r) => [r.weather, r.count]) }],
  })));
  def("pie", "apex", "native", "", ap(() => ({ chart: { type: "pie" }, labels: P.wTypes, series: P.wCounts.map((r) => r.count) })));
  none("pie", "plot", "no arc mark. The documentation points to D3");
  def("pie", "recharts", "native", "one color for every slice unless told", rc(() =>
    h(Recharts.PieChart, null,
      h(Recharts.Pie, { data: P.wCounts, dataKey: "count", nameKey: "weather", outerRadius: "75%", label: true },
        ...P.wCounts.map((_, i) => h(Recharts.Cell, { key: i, fill: TEN[i] }))),
      h(Recharts.Tooltip), h(Recharts.Legend))));
  def("pie", "mermaid", "native", "", mm(() => `pie\n` + P.wCounts.map((r) => `  "${r.weather}" : ${r.count}`).join("\n")));
  def("pie", "mpl", "native", "", st("pie"));

  // ---------------------------------------------------------------- heat

  def("heat", "vegalite", "native", "rect mark, aggregates the raw rows", vl({
    data: { values: D.weather },
    mark: "rect",
    encoding: {
      x: { timeUnit: "month", field: "date", type: "ordinal" },
      y: { timeUnit: "year", field: "date", type: "ordinal" },
      color: { aggregate: "mean", field: "temp_max", type: "quantitative" },
    },
  }));
  def("heat", "echarts", "prep", "needs a visualMap for its colors", ec(() => ({
    tooltip: {}, grid: { left: 50, right: 20, top: 10, bottom: 70 },
    xAxis: { type: "category", data: P.monthNames }, yAxis: { type: "category", data: P.heatYears.map(String) },
    visualMap: { min: 5, max: 30, calculable: true, orient: "horizontal", left: "center", bottom: 0 },
    series: [{ type: "heatmap", data: P.heatCells.map((c) => [c.mi, c.yi, c.value]), label: { show: true, fontSize: 9 } }],
  })));
  def("heat", "chartjs", "plugin", "matrix plugin, and the caller maps each value to a color", cj((t) => {
    const color = d3.scaleSequential(d3.interpolateYlOrRd).domain([5, 30]);
    return {
      type: "matrix",
      data: { datasets: [{ label: "°C", data: P.heatCells.map((c) => ({ x: c.month, y: c.year, v: c.value })),
        backgroundColor: (ctx) => color(ctx.raw ? ctx.raw.v : 0),
        width: ({ chart }) => (chart.chartArea || {}).width / 12 - 1, height: ({ chart }) => (chart.chartArea || {}).height / 4 - 1 }] },
      options: { plugins: { legend: { display: false } }, scales: { x: { type: "category", labels: P.monthNames, grid: { display: false } }, y: { type: "category", labels: P.heatYears.map(String), offset: true, grid: { display: false } } } },
    };
  }));
  def("heat", "plotly", "native", "", pl(() => ({ data: [{ type: "heatmap", x: P.monthNames, y: P.heatYears.map(String), z: P.heat }] })));
  def("heat", "highcharts", "plugin", "heatmap module", hc(() => ({
    chart: { type: "heatmap" }, xAxis: { categories: P.monthNames }, yAxis: { categories: P.heatYears.map(String), title: null },
    colorAxis: { min: 5, max: 30 }, series: [{ name: "°C", data: P.heatCells.map((c) => [c.mi, c.yi, c.value]), dataLabels: { enabled: true, style: { fontSize: "9px" } } }],
  })));
  def("heat", "apex", "prep", "shades one color", ap(() => ({
    chart: { type: "heatmap" }, dataLabels: { enabled: false },
    series: P.heatYears.map((y, yi) => ({ name: String(y), data: P.monthNames.map((m, mi) => ({ x: m, y: P.heat[yi][mi] })) })),
  })));
  def("heat", "plot", "native", "cell mark", pt(() => ({
    color: { legend: true, scheme: "ylorrd" }, padding: 0.05, marginLeft: 44,
    x: { domain: P.monthNames }, y: { label: null },
    marks: [Plot.cell(P.heatCells, { x: "month", y: "year", fill: "value", inset: 0.5 }), Plot.text(P.heatCells, { x: "month", y: "year", text: "value", fontSize: 9 })],
  })));
  none("heat", "recharts", "no heat map. A grid of Rectangle shapes by hand");
  def("heat", "mpl", "native", "imshow", st("heat"));

  // ----------------------------------------------------------------- box

  def("box", "vegalite", "native", "computes the quartiles", vl({
    data: { values: P.penguins },
    mark: "boxplot",
    encoding: { x: { field: "Species", type: "nominal" }, y: { field: "Body Mass (g)", type: "quantitative", scale: { zero: false } }, color: { field: "Species", type: "nominal", legend: null } },
  }));
  def("box", "echarts", "prep", "the caller computes the quartiles", ec(() => ({
    tooltip: {}, grid: { left: 50, right: 20, bottom: 30 },
    xAxis: { type: "category", data: P.species }, yAxis: { type: "value", scale: true },
    series: [
      { type: "boxplot", data: P.species.map((s) => { const b = P.box[s]; return [b.min, b.q1, b.median, b.q3, b.max]; }) },
      { type: "scatter", data: P.species.flatMap((s, i) => P.box[s].out.map((v) => [i, v])) },
    ],
  })));
  def("box", "chartjs", "plugin", "boxplot plugin, computes the quartiles. It draws in the border color, so it needs colors", cj(() => ({
    type: "boxplot", data: { labels: P.species, datasets: [{ label: "body mass (g)", data: P.species.map((s) => P.mass[s]), backgroundColor: TEN[0] + "66", borderColor: TEN[0], outlierBackgroundColor: TEN[0] }] },
  })));
  def("box", "plotly", "native", "", pl(() => ({ data: P.species.map((s) => ({ type: "box", name: s, y: P.mass[s] })) })));
  def("box", "highcharts", "prep", "highcharts-more, and the caller computes the quartiles", hc(() => ({
    chart: { type: "boxplot" }, legend: { enabled: false }, xAxis: { categories: P.species }, yAxis: { title: { text: "g" } },
    series: [
      { name: "body mass", data: P.species.map((s) => { const b = P.box[s]; return [b.min, b.q1, b.median, b.q3, b.max]; }) },
      { name: "outliers", type: "scatter", data: P.species.flatMap((s, i) => P.box[s].out.map((v) => [i, v])) },
    ],
  })));
  def("box", "apex", "prep", "the caller computes the quartiles", ap(() => ({
    chart: { type: "boxPlot" },
    series: [{ type: "boxPlot", data: P.species.map((s) => { const b = P.box[s]; return { x: s, y: [b.min, b.q1, b.median, b.q3, b.max] }; }) }],
  })));
  def("box", "plot", "native", "", pt(() => ({
    y: { grid: true }, marks: [Plot.boxY(P.penguins, { x: "Species", y: "Body Mass (g)" })],
  })));
  none("box", "recharts", "no box plot. Stacked bars and error bars by hand");
  def("box", "mpl", "native", "", st("box"));

  // -------------------------------------------------------------- candle

  def("candle", "vegalite", "composed", "a rule and a bar, layered", vl((t) => ({
    data: { values: D.ohlc },
    encoding: {
      x: { field: "date", type: "temporal", title: null },
      color: { condition: { test: "datum.open < datum.close", value: "#06982d" }, value: "#ae1325" },
    },
    layer: [
      { mark: "rule", encoding: { y: { field: "low", type: "quantitative", scale: { zero: false } }, y2: { field: "high" } } },
      { mark: { type: "bar", width: 5 }, encoding: { y: { field: "open", type: "quantitative" }, y2: { field: "close" } } },
    ],
  })));
  def("candle", "echarts", "native", "", ec(() => ({
    tooltip: { trigger: "axis" }, grid: { left: 40, right: 20, bottom: 30 },
    xAxis: { type: "category", data: P.ohlc.map((r) => r.date) }, yAxis: { type: "value", scale: true },
    series: [{ type: "candlestick", data: P.ohlc.map((r) => [r.open, r.close, r.low, r.high]) }],
  })));
  def("candle", "chartjs", "plugin", "financial plugin", cj(() => ({
    type: "candlestick", data: { datasets: [{ label: "VIX", data: P.ohlc.map((r) => ({ x: r.t, o: r.open, h: r.high, l: r.low, c: r.close })) }] },
  })));
  def("candle", "plotly", "native", "", pl(() => ({
    data: [{ type: "candlestick", x: P.ohlc.map((r) => r.date), open: P.ohlc.map((r) => r.open), high: P.ohlc.map((r) => r.high), low: P.ohlc.map((r) => r.low), close: P.ohlc.map((r) => r.close) }],
    layout: { xaxis: { rangeslider: { visible: false } } },
  })));
  def("candle", "highcharts", "plugin", "Highcharts Stock module", hc(() => ({
    rangeSelector: { enabled: false }, navigator: { enabled: false }, scrollbar: { enabled: false },
    series: [{ type: "candlestick", name: "VIX", data: P.ohlc.map((r) => [r.t, r.open, r.high, r.low, r.close]) }],
  }), "stockChart"));
  def("candle", "apex", "native", "", ap(() => ({
    chart: { type: "candlestick" }, xaxis: { type: "datetime" },
    series: [{ name: "VIX", data: P.ohlc.map((r) => ({ x: r.when, y: [r.open, r.high, r.low, r.close] })) }],
  })));
  def("candle", "plot", "composed", "two rule marks", pt(() => ({
    y: { grid: true }, color: { domain: [-1, 0, 1], range: ["#e41a1c", "currentColor", "#4daf4a"] },
    marks: [
      Plot.ruleX(P.ohlc, { x: "when", y1: "low", y2: "high" }),
      Plot.ruleX(P.ohlc, { x: "when", y1: "open", y2: "close", stroke: (d) => Math.sign(d.close - d.open), strokeWidth: 4, strokeLinecap: "round" }),
    ],
  })));
  none("candle", "recharts", "no candlestick. Custom bar shapes by hand");
  none("candle", "uplot", "a demo plugin draws one, nothing ships");
  def("candle", "lw", "native", "", lw((c) => {
    c.addSeries(LightweightCharts.CandlestickSeries).setData(P.ohlc.map((r) => ({ time: r.date, open: r.open, high: r.high, low: r.low, close: r.close })));
  }));
  def("candle", "mpl", "composed", "vlines and bars, without mplfinance", st("candle"));

  // --------------------------------------------------------------- radar

  none("radar", "vegalite", "no polar coordinates for lines. Arcs only");
  def("radar", "echarts", "native", "", ec(() => ({
    tooltip: {}, legend: { bottom: 0 },
    radar: { indicator: P.measureNames.map((n) => ({ name: n, max: 1 })), radius: "60%" },
    series: [{ type: "radar", data: P.species.map((s) => ({ name: s, value: P.radar[s] })) }],
  })));
  def("radar", "chartjs", "native", "", cj(() => ({
    type: "radar", data: { labels: P.measureNames, datasets: P.species.map((s) => ({ label: s, data: P.radar[s] })) },
    options: { scales: { r: { min: 0, max: 1 } } },
  })));
  def("radar", "plotly", "native", "scatterpolar", pl(() => ({
    data: P.species.map((s) => ({ type: "scatterpolar", fill: "toself", name: s, r: P.radar[s].concat(P.radar[s][0]), theta: P.measureNames.concat(P.measureNames[0]) })),
    layout: { polar: { radialaxis: { range: [0, 1] } }, margin: { l: 80, r: 80, t: 30, b: 30 } },
  })));
  def("radar", "highcharts", "plugin", "highcharts-more, polar", hc(() => ({
    chart: { polar: true, type: "line" }, pane: { size: "75%" },
    xAxis: { categories: P.measureNames, tickmarkPlacement: "on", lineWidth: 0 }, yAxis: { gridLineInterpolation: "polygon", min: 0, max: 1 },
    series: P.species.map((s) => ({ name: s, data: P.radar[s], pointPlacement: "on" })),
  })));
  def("radar", "apex", "native", "", ap(() => ({ chart: { type: "radar" }, xaxis: { categories: P.measureNames }, series: P.species.map((s) => ({ name: s, data: P.radar[s] })) })));
  none("radar", "plot", "no polar coordinates. A documented example builds one from lines");
  def("radar", "recharts", "prep", "the caller pivots the rows", rc((t) =>
    h(Recharts.RadarChart, { data: P.measureNames.map((n, i) => Object.assign({ m: n }, Object.fromEntries(P.species.map((s) => [s, P.radar[s][i]])))), outerRadius: "70%" },
      h(Recharts.PolarGrid, { stroke: t.dark ? t.grid : undefined }), h(Recharts.PolarAngleAxis, { dataKey: "m", tick: t.dark ? { fill: t.muted } : undefined }),
      h(Recharts.PolarRadiusAxis, { domain: [0, 1] }), h(Recharts.Legend),
      ...P.species.map((s, i) => h(Recharts.Radar, { key: s, dataKey: s, stroke: TEN[i], fill: TEN[i], fillOpacity: 0.3 })))));
  def("radar", "mermaid", "native", "radar-beta", mm(() =>
    "radar-beta\n  axis " + P.measureNames.map((n, i) => `m${i}["${n}"]`).join(", ") + "\n" +
    P.species.map((s) => `  curve ${s.toLowerCase()}["${s}"]{${P.radar[s].join(", ")}}`).join("\n") + "\n  max 1\n  min 0"));
  def("radar", "mpl", "composed", "polar axes, by hand", st("radar"));

  // ------------------------------------------------------------- treemap

  none("treemap", "vegalite", "no hierarchy. Vega has it");
  def("treemap", "vega", "native", "stratify and treemap transforms", vg({
    data: [{ name: "tree", values: D.flare, transform: [
      { type: "stratify", key: "id", parentKey: "parent" },
      { type: "treemap", field: "size", sort: { field: "value" }, round: true, method: "squarify", size: [{ signal: "width" }, { signal: "height" }] },
    ] }, { name: "nodes", source: "tree", transform: [{ type: "filter", expr: "datum.children" }] },
    { name: "leaves", source: "tree", transform: [{ type: "filter", expr: "!datum.children" }] }],
    scales: [{ name: "color", type: "ordinal", domain: { data: "nodes", field: "name" }, range: { scheme: "tableau20" } }],
    marks: [
      { type: "rect", from: { data: "nodes" }, interactive: false, encode: { enter: { fill: { scale: "color", field: "name" } }, update: { x: { field: "x0" }, y: { field: "y0" }, x2: { field: "x1" }, y2: { field: "y1" } } } },
      { type: "rect", from: { data: "leaves" }, encode: { enter: { stroke: { value: "#fff" } }, update: { x: { field: "x0" }, y: { field: "y0" }, x2: { field: "x1" }, y2: { field: "y1" }, fill: { value: "transparent" } } } },
      { type: "text", from: { data: "nodes" }, interactive: false, encode: { enter: { font: { value: "Helvetica Neue, Arial" }, align: { value: "center" }, baseline: { value: "middle" }, fill: { value: "#000" }, text: { field: "name" }, fontSize: { value: 11 }, fillOpacity: { signal: "datum.depth === 1 ? 1 : 0" } }, update: { x: { signal: "0.5 * (datum.x0 + datum.x1)" }, y: { signal: "0.5 * (datum.y0 + datum.y1)" } } } },
    ],
  }));
  def("treemap", "echarts", "native", "", ec(() => ({
    tooltip: {}, series: [{ type: "treemap", data: P.flareTree.children, leafDepth: 2, roam: false, breadcrumb: { show: false }, top: 5, bottom: 5, left: 5, right: 5 }],
  })));
  def("treemap", "chartjs", "plugin", "treemap plugin", cj(() => ({
    type: "treemap",
    data: { datasets: [{ tree: P.flareLeaves, key: "size", groups: ["l1"], spacing: 1, borderWidth: 1,
      backgroundColor: (ctx) => ctx.type === "data" ? TEN[(ctx.dataIndex || 0) % 10] + "cc" : "transparent",
      labels: { display: true, formatter: (ctx) => ctx.raw && ctx.raw.g } }] },
    options: { plugins: { legend: { display: false } } },
  })));
  def("treemap", "plotly", "native", "", pl(() => ({
    data: [{ type: "treemap", ids: P.flareNodes.map((n) => n.id), parents: P.flareNodes.map((n) => n.parent), labels: P.flareNodes.map((n) => n.name), values: P.flareNodes.map((n) => n.value), branchvalues: "total", maxdepth: 3 }],
    layout: { margin: { l: 4, r: 4, t: 4, b: 4 } },
  })));
  def("treemap", "highcharts", "plugin", "treemap module", hc(() => ({
    series: [{ type: "treemap", layoutAlgorithm: "squarified", allowTraversingTree: true, levels: [{ level: 1, dataLabels: { enabled: true }, borderWidth: 3, colorByPoint: true }, { level: 2, dataLabels: { enabled: false } }],
      data: P.flareNodes.filter((n) => n.parent !== "").map((n) => ({ id: n.id, parent: n.parent === "1" ? undefined : n.parent, name: n.name, value: n.size || undefined })) }],
  })));
  def("treemap", "apex", "prep", "two levels at most", ap(() => ({
    chart: { type: "treemap" }, legend: { show: false },
    series: P.flare2.map((c) => ({ name: c.name, data: c.children.map((g) => ({ x: g.name, y: g.value })) })),
  })));
  none("treemap", "plot", "no hierarchy layout. The documentation points to D3");
  def("treemap", "recharts", "native", "", rc(() =>
    h(Recharts.Treemap, { data: P.flareTree.children, dataKey: "value", nameKey: "name", stroke: "#fff", fill: TEN[0], isAnimationActive: false })));
  def("treemap", "mermaid", "native", "treemap-beta, two levels here", mm(() =>
    "treemap-beta\n" + P.flare2.map((c) => `"${c.name}"\n` + c.children.map((g) => `    "${g.name}": ${g.value}`).join("\n")).join("\n")));
  none("treemap", "mpl", "needs the squarify package");

  // ------------------------------------------------------------ sunburst

  none("sunburst", "vegalite", "no hierarchy. Vega has it");
  def("sunburst", "vega", "native", "stratify and partition transforms", vg({
    autosize: "none",
    data: [{ name: "tree", values: D.flare, transform: [
      { type: "stratify", key: "id", parentKey: "parent" },
      { type: "partition", field: "size", sort: { field: "value" }, size: [{ signal: "2 * PI" }, { signal: "min(width, height) / 2" }], as: ["a0", "r0", "a1", "r1", "depth", "children"] },
    ] }],
    scales: [{ name: "color", type: "ordinal", domain: { data: "tree", field: "depth" }, range: { scheme: "tableau20" } }],
    marks: [{ type: "arc", from: { data: "tree" }, encode: {
      enter: { x: { signal: "width / 2" }, y: { signal: "height / 2" }, fill: { scale: "color", field: "depth" }, tooltip: { signal: "datum.name" } },
      update: { startAngle: { field: "a0" }, endAngle: { field: "a1" }, innerRadius: { field: "r0" }, outerRadius: { field: "r1" }, stroke: { value: "white" }, strokeWidth: { value: 0.5 } },
    } }],
  }));
  def("sunburst", "echarts", "native", "", ec(() => ({
    tooltip: {}, series: [{ type: "sunburst", data: P.flareTree.children, radius: [0, "92%"], label: { show: false } }],
  })));
  none("sunburst", "chartjs", "no sunburst. Nested doughnuts by hand");
  def("sunburst", "plotly", "native", "", pl(() => ({
    data: [{ type: "sunburst", ids: P.flareNodes.map((n) => n.id), parents: P.flareNodes.map((n) => n.parent), labels: P.flareNodes.map((n) => n.name), values: P.flareNodes.map((n) => n.value), branchvalues: "total", maxdepth: 3 }],
    layout: { margin: { l: 4, r: 4, t: 4, b: 4 } },
  })));
  def("sunburst", "highcharts", "plugin", "sunburst module", hc(() => ({
    series: [{ type: "sunburst", allowTraversingTree: true, levels: [{ level: 2, colorByPoint: true }, { level: 3, colorVariation: { key: "brightness", to: -0.4 } }],
      data: P.flareNodes.map((n) => ({ id: n.id, parent: n.parent || undefined, name: n.name, value: n.size || undefined })), dataLabels: { enabled: false } }],
  })));
  none("sunburst", "apex", "no sunburst");
  none("sunburst", "plot", "no hierarchy layout");
  none("sunburst", "recharts", "SunburstChart arrived in Recharts 3");
  none("sunburst", "mpl", "nested pies by hand");

  // -------------------------------------------------------------- sankey

  none("sankey", "vegalite", "no sankey layout");
  def("sankey", "echarts", "native", "", ec(() => ({
    tooltip: {}, series: [{ type: "sankey", left: 10, right: 90, data: P.nodes.map((n) => ({ name: n })), links: P.links, emphasis: { focus: "adjacency" } }],
  })));
  def("sankey", "chartjs", "plugin", "sankey plugin", cj(() => ({
    type: "sankey", data: { datasets: [{ label: "cars", data: P.links.map((l) => ({ from: l.source, to: l.target, flow: l.value })), colorMode: "gradient" }] },
  })));
  def("sankey", "plotly", "native", "links by node index", pl(() => ({
    data: [{ type: "sankey", node: { label: P.nodes, pad: 12 }, link: { source: P.links.map((l) => P.nodes.indexOf(l.source)), target: P.links.map((l) => P.nodes.indexOf(l.target)), value: P.links.map((l) => l.value) } }],
    layout: { margin: { l: 8, r: 8, t: 8, b: 8 } },
  })));
  def("sankey", "highcharts", "plugin", "sankey module", hc(() => ({
    series: [{ type: "sankey", name: "cars", keys: ["from", "to", "weight"], data: P.links.map((l) => [l.source, l.target, l.value]) }],
  })));
  none("sankey", "apex", "no sankey");
  none("sankey", "plot", "no sankey layout. d3-sankey by hand");
  def("sankey", "recharts", "prep", "links by node index", rc(() =>
    h(Recharts.Sankey, { data: { nodes: P.nodes.map((name) => ({ name })), links: P.links.map((l) => ({ source: P.nodes.indexOf(l.source), target: P.nodes.indexOf(l.target), value: l.value })) },
      nodePadding: 20, margin: { top: 10, right: 90, bottom: 10, left: 10 }, link: { stroke: TEN[0] } }, h(Recharts.Tooltip))));
  def("sankey", "mermaid", "native", "sankey-beta, rows of CSV", mm(() => "sankey-beta\n" + P.links.map((l) => `${l.source},${l.target},${l.value}`).join("\n")));
  none("sankey", "mpl", "matplotlib.sankey draws a different kind of diagram");

  // ----------------------------------------------------------------- map

  def("map", "vegalite", "native", "geoshape with a projection", vl({
    data: { values: D.us, format: { type: "topojson", feature: "states" } },
    transform: [{ lookup: "id", from: { data: { values: D.states }, key: "id", fields: ["engineers", "state"] } }, { calculate: "datum.engineers * 1000", as: "per 1,000" }],
    projection: { type: "albersUsa" },
    mark: "geoshape",
    encoding: { color: { field: "per 1,000", type: "quantitative" }, tooltip: [{ field: "state" }, { field: "per 1,000", format: ".2f" }] },
  }));
  def("map", "echarts", "composed", "registerMap, and a D3 projection for Alaska and Hawaii", ec(() => {
    echarts.registerMap("states", P.stateGeo);
    const proj = d3.geoAlbersUsa();
    const vals = P.stateGeo.features.map((f) => f.properties.value);
    return {
      tooltip: {}, visualMap: { min: Math.min(...vals), max: Math.max(...vals), calculable: true, left: 0, bottom: 0 },
      series: [{ type: "map", map: "states", projection: { project: (p) => proj(p) || [0, 0], unproject: (p) => proj.invert(p) },
        data: P.stateGeo.features.map((f) => ({ name: f.properties.name, value: f.properties.value })) }],
    };
  }));
  none("map", "chartjs", "the geo plugin is not on the page");
  def("map", "plotly", "native", "", pl(() => ({
    data: [{ type: "choropleth", geojson: P.stateGeo, featureidkey: "properties.id", locations: P.stateGeo.features.map((f) => f.properties.id),
      z: P.stateGeo.features.map((f) => f.properties.value), text: P.stateGeo.features.map((f) => f.properties.name) }],
    layout: { geo: { scope: "usa" }, margin: { l: 0, r: 0, t: 0, b: 0 } },
  })));
  def("map", "highcharts", "plugin", "Highcharts Maps module. Alaska crosses the antimeridian, so the projection is rotated", hc(() => ({
    mapView: { projection: { name: "LambertConformalConic", rotation: [96], parallels: [33, 45] } }, colorAxis: {}, legend: { layout: "horizontal" },
    series: [{ name: "per 1,000", mapData: P.stateGeo, joinBy: ["id", "id"], data: P.stateGeo.features.map((f) => ({ id: f.properties.id, value: f.properties.value })) }],
  }), "mapChart"));
  none("map", "apex", "no maps");
  def("map", "plot", "native", "geo mark with a projection", pt(() => ({
    projection: "albers-usa", color: { legend: true, scheme: "blues", label: "per 1,000" },
    marks: [Plot.geo(P.stateGeo, { fill: (d) => d.properties.value, stroke: "currentColor", strokeWidth: 0.3, title: (d) => d.properties.name })],
  })));
  none("map", "recharts", "no maps");
  none("map", "mpl", "needs cartopy or geopandas");

  // ----------------------------------------------------------------- dag

  const DAG_MERMAID = `flowchart LR
  vl[Vega-Lite spec] --> c[compile] --> v[Vega spec]
  v --> df[dataflow and transforms]
  sql[(SQL result)] -.-> df
  df --> sc[scales, axes, legends]
  sc --> sg[scene graph]
  th[theme: light, dark] --> sg
  dot[DOT or Mermaid] --> gl[graph layout] --> sg
  sg --> svg[SVG writer] --> ok[okage svg raster]
  sg --> dr[direct okage draw]
  sg -.-> js[browser canvas, live]`;
  const DAG_DOT = `digraph zuzu {
  rankdir=LR; node [shape=box, fontname="Helvetica", fontsize=11]; edge [fontsize=10];
  vl [label="Vega-Lite spec"]; c [label="compile"]; v [label="Vega spec"];
  df [label="dataflow and transforms"]; sql [label="SQL result", shape=cylinder];
  sc [label="scales, axes, legends"]; sg [label="scene graph"]; th [label="theme: light, dark"];
  dot [label="DOT or Mermaid"]; gl [label="graph layout"]; svg [label="SVG writer"];
  ok [label="okage svg raster"]; dr [label="direct okage draw"]; js [label="browser canvas, live"];
  vl -> c -> v -> df -> sc -> sg; sql -> df [style=dashed]; th -> sg; dot -> gl -> sg;
  sg -> svg -> ok; sg -> dr; sg -> js [style=dashed];
}`;
  def("dag", "mermaid", "native", "flowchart, dagre layout", mm(DAG_MERMAID));
  def("dag", "graphviz", "native", "dot layout", gv(DAG_DOT));
  def("dag", "echarts", "composed", "graph series, force layout. No layered layout", ec(() => {
    const ids = ["vl", "c", "v", "df", "sql", "sc", "sg", "th", "dot", "gl", "svg", "ok", "dr", "js"];
    const edges = [["vl", "c"], ["c", "v"], ["v", "df"], ["sql", "df"], ["df", "sc"], ["sc", "sg"], ["th", "sg"], ["dot", "gl"], ["gl", "sg"], ["sg", "svg"], ["svg", "ok"], ["sg", "dr"], ["sg", "js"]];
    return { series: [{ type: "graph", layout: "force", force: { repulsion: 160, edgeLength: 50 }, roam: true, label: { show: true, fontSize: 10 }, edgeSymbol: ["none", "arrow"],
      data: ids.map((name) => ({ name })), links: edges.map(([source, target]) => ({ source, target })) }] };
  }));
  none("dag", "vegalite", "no graph layout. Vega has a force transform only");

  // ---------------------------------------------------------------- page

  const theme = () => {
    const dark = document.documentElement.dataset.theme === "dark";
    const css = getComputedStyle(document.documentElement);
    const v = (n) => css.getPropertyValue(n).trim();
    return { dark, fg: v("--fg"), muted: v("--muted"), grid: v("--rule"), card: v("--card") };
  };

  const KIND = { native: ["●", "built in"], plugin: ["◐", "plugin"], composed: ["◑", "composed"], prep: ["◒", "caller prepares"], none: ["–", "not available"] };
  const shown = new Set(LIBS.map((l) => l.id));
  const cards = [];

  function build() {
    document.getElementById("source").textContent = "Data: " + D.source + ".";

    // the coverage matrix
    const m = document.getElementById("matrix");
    const head = "<thead><tr><th>Chart</th>" + LIBS.map((l) => `<th>${l.name}</th>`).join("") + "</tr></thead>";
    const body = ROWS.map((r) => "<tr><td>" + r.title + "</td>" + LIBS.map((l) => {
      const e = (R[r.id] || {})[l.id];
      if (!e) return "<td></td>";
      const [sym, word] = KIND[e.kind];
      const tip = word + (e.note ? ": " + e.note : "");
      return `<td title="${tip.replace(/"/g, "&quot;")}"><a class="k-${e.kind}" href="#${r.id}-${l.id}">${sym}</a></td>`;
    }).join("") + "</tr>").join("");
    m.innerHTML = head + "<tbody>" + body + "</tbody>";

    // the jump list
    const jump = document.getElementById("jump");
    jump.innerHTML = '<option value="">Chart…</option>' + ROWS.map((r) => `<option value="${r.id}">${r.title}</option>`).join("");
    jump.addEventListener("change", () => { if (jump.value) document.getElementById(jump.value).scrollIntoView(); jump.value = ""; });

    // the library filter
    const libs = document.getElementById("libs");
    libs.innerHTML = LIBS.map((l) => `<label for="lib-${l.id}"><input type="checkbox" id="lib-${l.id}" data-lib="${l.id}" checked> ${l.name}</label>`).join("");
    libs.addEventListener("change", (e) => {
      const id = e.target.dataset.lib;
      if (e.target.checked) shown.add(id); else shown.delete(id);
      for (const c of cards) c.el.hidden = !shown.has(c.lib);
    });

    // the rows
    const rows = document.getElementById("rows");
    const io = new IntersectionObserver((entries) => {
      for (const en of entries) if (en.isIntersecting) { const c = cards.find((x) => x.el === en.target); if (c && !c.drawn) draw(c); }
    }, { rootMargin: "300px" });
    for (const r of ROWS) {
      const sec = document.createElement("section");
      sec.className = "row";
      sec.id = r.id;
      sec.innerHTML = `<h2>${r.title}</h2><p class="muted">${r.data}</p><div class="grid"></div>`;
      const grid = sec.querySelector(".grid");
      for (const l of LIBS) {
        const e = (R[r.id] || {})[l.id];
        if (!e) continue;
        const el = document.createElement("article");
        el.className = "card" + (e.kind === "none" ? " is-none" : "");
        el.id = `${r.id}-${l.id}`;
        const [, word] = KIND[e.kind];
        el.innerHTML = `<header><b>${l.name}</b><span class="ver">${l.ver}</span><span class="pill ${e.kind}">${word}</span></header>` +
          `<div class="chart${r.id === "dag" ? " diagram" : ""}"></div>` +
          `<footer>${e.note ? `<span>${e.note}</span>` : ""}${e.draw ? `<span class="loc">${l.tech}</span>` : ""}</footer>`;
        if (e.kind === "none") el.querySelector(".chart").innerHTML = `<p class="msg">Not available.</p>`;
        grid.append(el);
        const c = { row: r.id, lib: l.id, el, entry: e, drawn: false, undo: null };
        cards.push(c);
        if (e.draw) io.observe(el);
      }
      rows.append(sec);
    }
    drain();
  }

  // drain draws every card in order, one at a time, so that the page is
  // complete without scrolling. The observer above draws the cards in view
  // first.
  async function drain() {
    for (const c of cards) {
      if (c.drawn || !c.entry.draw) continue;
      await draw(c);
      await new Promise((r) => setTimeout(r, 0));
    }
  }

  async function draw(c) {
    c.drawn = true;
    const box = c.el.querySelector(".chart");
    box.replaceChildren();
    try {
      c.undo = await c.entry.draw(box, theme());
    } catch (err) {
      console.error(c.row, c.lib, err);
      box.innerHTML = `<p class="err">${String(err && err.stack || err).split("\n").slice(0, 4).join("\n")}</p>`;
    }
  }

  async function redraw() {
    for (const c of cards) {
      if (!c.drawn) continue;
      try { if (c.undo) c.undo(); } catch (err) { console.error(err); }
      c.undo = null;
      await draw(c);
    }
  }

  function setTheme(dark) {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    document.getElementById("t-light").setAttribute("aria-pressed", String(!dark));
    document.getElementById("t-dark").setAttribute("aria-pressed", String(dark));
    try { localStorage.setItem("survey-theme", dark ? "dark" : "light"); } catch (_) { /* no storage */ }
  }

  let saved = null;
  try { saved = localStorage.getItem("survey-theme"); } catch (_) { /* no storage */ }
  setTheme(saved ? saved === "dark" : matchMedia("(prefers-color-scheme: dark)").matches);
  document.getElementById("t-light").addEventListener("click", () => { setTheme(false); redraw(); });
  document.getElementById("t-dark").addEventListener("click", () => { setTheme(true); redraw(); });
  build();
})();
