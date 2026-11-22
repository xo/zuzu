// mermaid-svg.mjs draws diagrams of Mermaid with mermaid.js, and writes the
// SVG of each. It is the oracle of the layouts of D44. It draws many
// diagrams with one Chrome, and each diagram in the layout that its job
// asks for, such as ELK or dagre.
//
// It draws as mmdc of mermaid-cli 12.2.0 draws: in headless Chrome through
// puppeteer, on the same page, with the same style sheets of fonts, the
// same version of mermaid.js and the same config. On the fixtures, its SVG
// has the same view box, the same places of the nodes and the same paths of
// the edges as the SVG of mmdc.
//
// mermaid.js 12.3.0 bundles the layouts of ELK, and its default layout is
// elk. The config of the script also sets layout: elk, so that a source
// that sets no layout draws in ELK if a later mermaid.js changes its
// default. A source that sets its own layout, in its front matter or in a
// directive, keeps it.
//
// It reads one job per line on its standard input, as JSON:
//
//	{"in": "testdata/diagrams/mermaid/01-basic.mmd", "out": "/abs/01-basic.svg"}
//
// A path is relative to the current folder. A job can also give "config",
// an object that the script merges over its config, as the flag -c of mmdc
// does. For example, {"layout": "dagre"} draws in dagre, {"layout":
// "elk.mrtree"} draws in the tree layout of ELK, and {"look": "handDrawn",
// "handDrawnSeed": 1} draws by hand. For each job, it writes the SVG to
// out. If mermaid.js cannot draw the source, it writes the message to out
// with the extension .err instead. It removes the other of the two files,
// so that only one is there.
//
// It uses puppeteer-core, which downloads no browser. It starts the Chrome
// that puppeteer names "chrome", the stable Chrome of the system. To start
// another Chrome, give a config of puppeteer in PUPPETEER_CONFIG, the same
// file that mmdc reads with -p, such as
// {"executablePath": "/usr/bin/google-chrome-stable", "args": ["--no-sandbox"]}.
//
// Run it from the test module after npm install in this folder:
//
//	node oracle/mermaid-svg.mjs < jobs
//
// Add -j n to draw n sources at the same time. The default is half of the
// processors, as for mmdc.
import { mkdir, readFile, realpath, rename, rm, writeFile } from "node:fs/promises";
import { availableParallelism } from "node:os";
import { dirname, extname, join, relative, resolve, sep } from "node:path";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer-core";

const modules = join(dirname(fileURLToPath(import.meta.url)), "node_modules");

// origin is the origin of the page. Chrome does not import a module from a
// file URL, so the script answers each request to this origin with a file
// from node_modules, as mmdc does with its own origin.
const origin = "https://zuzu-oracle.invalid";

// page is the page that mmdc draws on, dist/index.html of mermaid-cli.
const page = `<!doctype html>
<html>
  <body>
    <div id="container"></div>
  </body>
</html>
`;

// styles are the style sheets that mmdc loads before it draws, in its
// order. They give the fonts of the icons of Font Awesome, of the math of
// KaTeX, and the fonts Recursive and Open Sans.
const styles = [
  "@fortawesome/fontawesome-free/css/brands.css",
  "@fortawesome/fontawesome-free/css/regular.css",
  "@fortawesome/fontawesome-free/css/solid.css",
  "@fortawesome/fontawesome-free/css/fontawesome.css",
  "@fontsource-variable/recursive/index.css",
  "@fontsource/open-sans/400.css",
  "@fontsource/open-sans/400-italic.css",
  "katex/dist/katex.css",
];

// mermaidURL is the bundle of mermaid.js for a browser, which mmdc loads.
const mermaidURL = `${origin}/mermaid/dist/mermaid.esm.mjs`;

// config is the config of mermaid.js. mmdc passes an empty theme when no
// flag sets one.
const config = { theme: undefined, layout: "elk" };

// types are the media types of the files that the page can ask for.
const types = {
  ".css": "text/css;charset=UTF-8",
  ".js": "application/javascript",
  ".mjs": "application/javascript",
  ".json": "application/json",
  ".woff2": "font/woff2",
  ".woff": "font/woff",
  ".ttf": "font/ttf",
};

// serve answers a request of the page. It answers the page itself, and a
// file of node_modules for a path under the origin. It refuses a path out
// of node_modules.
async function serve(req) {
  const url = new URL(req.url());
  if (url.origin !== origin) {
    return req.continue();
  }
  const headers = { "Access-Control-Allow-Origin": "*" };
  if (url.pathname === "/index.html") {
    return req.respond({ status: 200, headers, contentType: "text/html;charset=UTF-8", body: page });
  }
  try {
    const file = await realpath(join(modules, decodeURIComponent(url.pathname)));
    const rel = relative(await realpath(modules), file);
    const type = types[extname(file).toLowerCase()];
    if (rel.startsWith(".." + sep) || type === undefined) {
      throw new Error(`the page cannot read ${url.pathname}`);
    }
    return req.respond({ status: 200, headers, contentType: type, body: await readFile(file) });
  } catch (err) {
    return req.respond({ status: 404, headers, contentType: "text/plain;charset=UTF-8", body: String(err.message || err) });
  }
}

// drawPage runs in the page. It draws the source as mmdc does, and returns
// the SVG.
async function drawPage(container, { source, config, mermaidURL }) {
  const { default: mermaid } = await import(mermaidURL);
  await Promise.all(Array.from(document.fonts, (font) => font.load()));
  mermaid.initialize({ startOnLoad: false, ...config });
  const { svg: text } = await mermaid.render("my-svg", source, container);
  container.innerHTML = text;
  const svg = container.getElementsByTagName("svg")[0];
  svg.style.backgroundColor = "white";
  await embedFonts(svg);
  return new XMLSerializer().serializeToString(svg);

  // embedFonts adds a style element to the SVG for each style sheet of
  // fonts that it uses, with each font that it uses as a data URL, so that
  // the SVG shows its icons and its math in an img element. mmdc does the
  // same, and also adds the license of the fonts.
  async function embedFonts(svg) {
    const families = new Set();
    const first = (family) => family.split(",")[0].trim().replace(/^["']|["']$/g, "");
    for (const el of svg.querySelectorAll("*")) {
      const icon = ["fa", "fab", "fak", "fal", "far", "fas"].some((c) => el.classList.contains(c));
      if (icon || [...el.childNodes].some((n) => n.nodeType === Node.TEXT_NODE)) {
        families.add(first(getComputedStyle(el).fontFamily));
      }
    }
    const applies = (rule) => {
      const sel = rule.selectorText.replaceAll(/::[\w-]+|:(before|after|first-letter|first-line)\b/g, "");
      return svg.matches(sel) || svg.querySelector(sel) !== null;
    };
    const asData = async (url) => {
      const blob = await (await fetch(url)).blob();
      return await new Promise((ok, fail) => {
        const r = new FileReader();
        r.onload = () => ok(r.result);
        r.onerror = () => fail(new Error(`cannot read the font ${url}`));
        r.readAsDataURL(blob);
      });
    };
    const after = svg.querySelector("style");
    for (const link of document.querySelectorAll("link.mermaid-cli-css")) {
      const rules = [];
      let used = false;
      for (const rule of link.sheet.cssRules) {
        if (rule instanceof CSSFontFaceRule) {
          if (!families.has(first(rule.style.fontFamily))) {
            continue;
          }
          const src = rule.style.getPropertyValue("src");
          const urls = [...src.matchAll(/url\("?([^")]+)"?\)/g)].map((m) => m[1]);
          const url = urls.find((u) => u.includes("woff2")) || urls[0];
          const data = await asData(new URL(url, link.href));
          rules.push(rule.cssText.replace(src, `url("${data}") format("woff2")`));
          used = true;
          continue;
        }
        if (rule instanceof CSSStyleRule && !used) {
          used = applies(rule);
        }
        rules.push(rule.cssText);
      }
      if (used && after) {
        const style = document.createElementNS("http://www.w3.org/2000/svg", "style");
        style.textContent = rules.join("\n");
        after.after(style);
      }
    }
  }
}

// draw draws one job in a new page, as mmdc draws each diagram in a new
// page, and returns the SVG.
async function draw(browser, job) {
  const source = await readFile(job.in, "utf8");
  const tab = await browser.newPage();
  try {
    // mmdc sets a view port of zero when no flag sets its size.
    await tab.setViewport({ width: 0, height: 0, deviceScaleFactor: 1 });
    await tab.setRequestInterception(true);
    tab.on("request", serve);
    await tab.goto(`${origin}/index.html`);
    await tab.$eval("body", (body) => {
      body.style.background = "white";
    });
    await tab.evaluate(async (hrefs) => {
      await Promise.all(
        hrefs.map((href) => {
          const link = document.createElement("link");
          link.setAttribute("crossorigin", "anonymous");
          link.setAttribute("rel", "stylesheet");
          link.setAttribute("href", href);
          link.setAttribute("class", "mermaid-cli-css");
          const loaded = new Promise((ok, fail) => {
            link.onload = () => ok();
            link.onerror = () => fail(new Error(`cannot load the style sheet ${href}`));
          });
          document.head.appendChild(link);
          return loaded;
        }),
      );
    }, styles.map((s) => `${origin}/${s}`));
    const cfg = { ...config, ...(job.config || {}) };
    return await tab.$eval("#container", drawPage, { source, config: cfg, mermaidURL });
  } finally {
    await tab.close();
  }
}

// write writes a file through a temporary file, so that a reader never
// sees half of it, and removes the file of the other outcome.
async function write(file, other, text) {
  await mkdir(dirname(file), { recursive: true });
  await writeFile(file + ".tmp", text);
  await rename(file + ".tmp", file);
  await rm(other, { force: true });
}

// launchConfig returns the options that start Chrome: those of mmdc, then
// the file in PUPPETEER_CONFIG, or the stable Chrome of the system.
async function launchConfig() {
  const opts = { headless: "shell" };
  if (process.env.PUPPETEER_CONFIG) {
    return { ...opts, ...JSON.parse(await readFile(process.env.PUPPETEER_CONFIG, "utf8")) };
  }
  return { ...opts, channel: "chrome" };
}

// jobsFlag returns the value of the flag -j, or half of the processors.
function jobsFlag() {
  const i = process.argv.indexOf("-j");
  if (i < 0) {
    return Math.max(1, Math.floor(availableParallelism() / 2));
  }
  const n = Number.parseInt(process.argv[i + 1], 10);
  if (!(n >= 1)) {
    console.error("mermaid-svg: -j needs a count of one or more");
    process.exit(2);
  }
  return n;
}

const jobs = [];
for await (const line of createInterface({ input: process.stdin })) {
  if (line.trim() !== "") {
    const job = JSON.parse(line);
    jobs.push({ ...job, in: resolve(job.in), out: resolve(job.out) });
  }
}

const browser = await puppeteer.launch(await launchConfig());
let ok = 0;
let failed = 0;
try {
  let next = 0;
  const worker = async () => {
    while (next < jobs.length) {
      const job = jobs[next++];
      try {
        await write(job.out, job.out + ".err", await draw(browser, job));
        ok++;
      } catch (err) {
        await write(job.out + ".err", job.out, String((err && err.message) || err) + "\n");
        failed++;
      }
    }
  };
  await Promise.all(Array.from({ length: Math.min(jobsFlag(), jobs.length) }, worker));
} finally {
  await browser.close();
}
console.log(`mermaid-svg: ${ok} drawn, ${failed} failed`);
