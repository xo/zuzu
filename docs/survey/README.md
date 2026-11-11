# Survey

This folder holds the survey of chart and diagram libraries that came before
the plan of zuzu (D23). zuzu does not use any file here. The files are for a
person who reviews what other libraries do.

| File | What it is |
| --- | --- |
| `libraries.html` | the survey of 20 chart libraries and four diagram tools, with the stars, the downloads, the chart types and the views of three other models, as measured on 2026-11-09. It is a record and is not updated |
| `compare.html` | the same real data drawn by 13 libraries, side by side, in a light and a dark theme |
| `compare.js` | the adapters of `compare.html`, one block for each kind of chart |
| `data.js` | the data of `compare.html`. `gen-data.mjs` generates it |
| `gen-data.mjs` | writes `data.js` from the files of vega-datasets |
| `static/` | the Matplotlib images of `compare.html`. `gen-static.py` generates them |
| `gen-static.py` | writes the Matplotlib images |
| `LICENSE-data.txt` | the license and the source of each file of data |
| `mermaid.md` | the kinds of diagram of Mermaid 12 and 13 alpha, the programs that draw them without mermaid.js, which kinds compile to a Vega spec, and the packs of icons, as surveyed on 2026-11-11 for D31 |

## Open the comparison

A browser does not load `data.js` and the images from a `file:` address on
every system, so serve the folder:

```bash
python3 -m http.server 8766 --bind 127.0.0.1 --directory docs/survey
```

Then open `http://127.0.0.1:8766/compare.html`. The page loads each library
from jsDelivr, so it needs a network connection.

Add `?still` to the address to turn off every animation. Use it for a
screenshot, or when the page is not in view, because a browser pauses the
animation of a page that is not in view and leaves some charts half drawn.

The Light and Dark buttons draw every chart again in that theme. The page
keeps the choice in the storage of the browser.

## What the page shows

Each card names the library, its version, and one of five labels:

| Label | Meaning |
| --- | --- |
| built in | the library draws this chart from the rows with one setting |
| plugin | the library needs a plugin or an extra module |
| composed | the caller builds the chart from other marks |
| caller prepares | the library has the chart, but the caller must aggregate or reshape the rows first |
| not available | the library cannot draw it |

The table at the top of the page shows the labels for every chart and
library at once.

Each library keeps its own palette, fonts, spacing and animation. Only the
dark theme is set by hand where a library has no dark theme of its own, and
the footer of each card says how the theme is set.

## Generate the data again

The data is real. It comes from vega-datasets 3.3.1, under the BSD-3-Clause
license, and each file has a source and a license of its own, which
`LICENSE-data.txt` lists. Fetch the twelve files into a folder, then run the
script in this folder:

```bash
for f in stocks.csv cars.json seattle-weather.csv ohlc.json flare.json \
  us-10m.json population_engineers_hurricanes.csv barley.json penguins.json \
  gapminder.json miserables.json flare-dependencies.json; do
  curl -sfO https://cdn.jsdelivr.net/npm/vega-datasets@3.3.1/data/$f
done
node gen-data.mjs DIR > data.js
```

`DIR` is the folder that holds the files.

## Generate the Matplotlib images again

Run this in this folder. It needs `uv`, which fetches Matplotlib 3.11.2 into
its cache:

```bash
uv run --no-project --with matplotlib==3.11.2 python -I gen-static.py
```

## Versions

Each library on the page is pinned to an exact version, and each version was
at least two weeks old on the day it was pinned, 2026-11-10. To move a
version, change its `script` element in `compare.html` and the `ver` field of
its row in `LIBS` in `compare.js`.

Highcharts is commercial software, free for a non-commercial use. The page
loads it from the CDN for review only.
