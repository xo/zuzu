"""gen-static.py writes the Matplotlib column of compare.html.

Matplotlib renders in Python and not in a browser, so its charts are static
SVG files in static/, two for each kind of chart: one with the default style
and one with the style dark_background. The data is the same data.js that the
page reads, so the numbers match the other libraries.

Run it with a Python that has Matplotlib, from this folder:

    uv run --with matplotlib==3.11.2 python gen-static.py

The script reads data.js and writes static/mpl-<chart>-<light|dark>.svg.
"""

import json
import pathlib
import statistics
from collections import Counter, defaultdict
from datetime import date

import matplotlib

matplotlib.use("svg")
import matplotlib.pyplot as plt  # noqa: E402

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE / "static"

# The size of a card on the page, in inches at 100 dots per inch.
SIZE = (4.6, 3.0)


def load():
    """Reads the JSON object that data.js assigns."""
    text = (HERE / "data.js").read_text(encoding="utf-8")
    start = text.index("{")
    end = text.rindex("}") + 1
    return json.loads(text[start:end])


MONTHS = {m: i for i, m in enumerate(
    ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"], start=1)}


def stock_date(s):
    """Reads a date such as "Jan 1 2000"."""
    m, d, y = s.split()
    return date(int(y), MONTHS[m], int(d))


def line(d, ax):
    by = defaultdict(list)
    for r in d["stocks"]:
        by[r["symbol"]].append((stock_date(r["date"]), r["price"]))
    for sym, pts in sorted(by.items()):
        ax.plot([p[0] for p in pts], [p[1] for p in pts], label=sym, linewidth=1.2)
    ax.legend(fontsize=7)


def area(d, ax):
    types = ["sun", "fog", "drizzle", "rain", "snow"]
    months = sorted({r["date"][:7] for r in d["weather"]})
    counts = Counter((r["date"][:7], r["weather"]) for r in d["weather"])
    xs = [date(int(m[:4]), int(m[5:]), 1) for m in months]
    ax.stackplot(xs, [[counts[(m, t)] for m in months] for t in types], labels=types)
    ax.legend(fontsize=7, loc="upper left")


def barley(d):
    total = defaultdict(float)
    for r in d["barley"]:
        total[r["site"]] += r["yield"]
    return sorted(total, key=lambda s: -total[s])


def group(d, ax):
    sites = barley(d)
    sums = defaultdict(float)
    for r in d["barley"]:
        sums[(r["site"], r["year"])] += r["yield"]
    w = 0.4
    for i, y in enumerate([1931, 1932]):
        ax.bar([k + (i - 0.5) * w for k in range(len(sites))], [sums[(s, y)] for s in sites], w, label=str(y))
    ax.set_xticks(range(len(sites)), sites, rotation=30, ha="right", fontsize=7)
    ax.legend(fontsize=7)


def stack(d, ax):
    sites = barley(d)
    varieties = sorted({r["variety"] for r in d["barley"]})
    sums = defaultdict(float)
    for r in d["barley"]:
        sums[(r["site"], r["variety"])] += r["yield"]
    left = [0.0] * len(sites)
    for v in varieties:
        vals = [sums[(s, v)] for s in sites]
        ax.barh(sites, vals, left=left, label=v)
        left = [a + b for a, b in zip(left, vals)]
    ax.invert_yaxis()
    ax.tick_params(axis="y", labelsize=7)
    ax.legend(fontsize=5, ncol=2)


def scatter(d, ax):
    for o in ["Europe", "Japan", "USA"]:
        pts = [(r["Horsepower"], r["Miles_per_Gallon"]) for r in d["cars"]
               if r["Origin"] == o and r["Horsepower"] is not None and r["Miles_per_Gallon"] is not None]
        ax.scatter([p[0] for p in pts], [p[1] for p in pts], s=8, label=o)
    ax.legend(fontsize=7)


def hist(d, ax):
    mpg = [r["Miles_per_Gallon"] for r in d["cars"] if r["Miles_per_Gallon"] is not None]
    ax.hist(mpg, bins=range(8, 50, 2))


def pie(d, ax):
    types = ["sun", "fog", "drizzle", "rain", "snow"]
    c = Counter(r["weather"] for r in d["weather"])
    ax.pie([c[t] for t in types], labels=types, textprops={"fontsize": 7})


def heat(d, ax):
    years = [2012, 2013, 2014, 2015]
    grid = []
    for y in years:
        row = []
        for m in range(1, 13):
            k = f"{y}-{m:02d}"
            row.append(statistics.mean(r["temp_max"] for r in d["weather"] if r["date"].startswith(k)))
        grid.append(row)
    im = ax.imshow(grid, aspect="auto")
    ax.set_yticks(range(4), [str(y) for y in years])
    ax.set_xticks(range(12), list(MONTHS), fontsize=6)
    plt.colorbar(im, ax=ax)


def box(d, ax):
    species = ["Adelie", "Chinstrap", "Gentoo"]
    data = [[r["Body Mass (g)"] for r in d["penguins"] if r["Species"] == s and r["Body Mass (g)"] is not None] for s in species]
    ax.boxplot(data, tick_labels=species)


def candle(d, ax):
    rows = d["ohlc"]
    xs = [date.fromisoformat(r["date"]) for r in rows]
    for x, r in zip(xs, rows):
        up = r["close"] >= r["open"]
        color = "#2ca02c" if up else "#d62728"
        ax.vlines(x, r["low"], r["high"], color=color, linewidth=0.8)
        ax.bar(x, abs(r["close"] - r["open"]), bottom=min(r["open"], r["close"]), width=0.7, color=color)
    ax.tick_params(axis="x", labelsize=6)


def radar(d, fig):
    import math
    measures = ["Beak Length (mm)", "Beak Depth (mm)", "Flipper Length (mm)", "Body Mass (g)"]
    names = ["Beak length", "Beak depth", "Flipper length", "Body mass"]
    species = ["Adelie", "Chinstrap", "Gentoo"]
    means = {s: [statistics.mean(r[m] for r in d["penguins"] if r["Species"] == s and r[m] is not None) for m in measures] for s in species}
    maxes = [max(means[s][i] for s in species) for i in range(4)]
    angles = [i * 2 * math.pi / 4 for i in range(4)] + [0]
    ax = fig.add_subplot(projection="polar")
    for s in species:
        vals = [means[s][i] / maxes[i] for i in range(4)]
        ax.plot(angles, vals + vals[:1], label=s)
        ax.fill(angles, vals + vals[:1], alpha=0.2)
    ax.set_xticks(angles[:-1], names, fontsize=7)
    ax.set_ylim(0, 1)
    ax.legend(fontsize=6, loc="upper right", bbox_to_anchor=(1.35, 1.1))


CHARTS = {
    "line": line, "area": area, "group": group, "stack": stack, "scatter": scatter,
    "hist": hist, "pie": pie, "heat": heat, "box": box, "candle": candle,
}


def main():
    d = load()
    OUT.mkdir(exist_ok=True)
    for style, suffix in (("default", "light"), ("dark_background", "dark")):
        with plt.style.context(style):
            for name, fn in list(CHARTS.items()) + [("radar", None)]:
                fig = plt.figure(figsize=SIZE, dpi=100)
                if fn is None:
                    radar(d, fig)
                else:
                    fn(d, fig.add_subplot())
                fig.tight_layout()
                # A transparent figure takes the color of the card behind it.
                fig.savefig(OUT / f"mpl-{name}-{suffix}.svg", transparent=suffix == "dark")
                plt.close(fig)


if __name__ == "__main__":
    main()
