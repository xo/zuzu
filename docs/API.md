# API

This document holds the public API of zuzu (D24). The maintainers accepted it
on 2026-11-10, before any code existed, and it can change as the code shows
what it needs. It shows the signatures and the types, and not the code behind
them. D25 to D29 hold the decisions that it builds on.

GPT-6 Astra, Gemini 3.1 Pro and DeepSeek V4 each sketched an API on
2026-11-10. "Where the models agreed and differed" at the end of this file
says what each proposed and what the API takes.

## Conventions

1. An error that a caller can test is a sentinel error: a constant of the
   type `Error`, a defined string type, as dbmeta does. A constant cannot be
   reassigned by an importer, as a variable from `errors.New` can. zuzu wraps
   it with `%w` and the context, and a caller tests it with `errors.Is`.
2. A call takes a functional option, named `With...`, only for a setting
   that it can do without. An input that the call needs is a parameter. A
   call with no optional setting takes no options.
3. The caller owns every value that it gives in an option. zuzu keeps no
   configuration in a package level variable.

## The shape

A caller turns JSON into a `Chart`, and a `Chart` into a scene graph, SVG or
an image:

```go
src, err := os.ReadFile("chart.vl.json")
if err != nil {
	return err
}
chart, report, err := zuzu.Parse(src)
if err != nil {
	return err
}
report, err = chart.SVG(ctx, w, zuzu.WithMode(theme.Dark))
```

1. `zuzu.Parse` reads the JSON. The `$schema` field says whether it is Vega
   or Vega-Lite, and a Vega-Lite spec is compiled to Vega. The result is a
   `Chart`.
2. A `Chart` holds the compiled Vega spec. It does not change after `Parse`,
   so one `Chart` can render on many goroutines at once.
3. A render runs the spec: it loads the data, runs the transforms, builds the
   scales and lays out the chart. The result is a scene graph (D8), which a
   renderer writes.
4. A `View` holds the state of one render, so that it can take changes to
   the data and render again. That is for live data (D19).

## The root package

```go
package zuzu

// Parse reads a Vega or a Vega-Lite spec, or a diagram of DOT or Mermaid
// (D31). The $schema field decides between Vega and Vega-Lite. A diagram
// is found by its first word, as D31 says. Another source without $schema
// needs WithDialect.
func Parse(src []byte, opts ...Option) (*Chart, Report, error)

// Sniff returns the dialect of a source from its first bytes, as a
// detector of a type of file does: DOT or Mermaid from the first word, and
// Vega or Vega-Lite from a $schema in the first bytes. It needs no more
// than the first 512 bytes, and returns false when they do not decide.
func Sniff(prefix []byte) (Dialect, bool)

// FromVega and FromVegaLite take a spec that a program built as Go values.
// Each copies the spec, so a later change by the caller does not reach the
// Chart.
func FromVega(spec *vega.Spec, opts ...Option) (*Chart, Report, error)
func FromVegaLite(spec *vegalite.Spec, opts ...Option) (*Chart, Report, error)

// Chart is a parsed and compiled spec. It is safe for concurrent use.
type Chart struct{ /* unexported */ }

// Spec returns a copy of the compiled Vega spec, to read or to debug.
func (c *Chart) Spec() *vega.Spec

// Render runs the spec and returns the scene graph.
func (c *Chart) Render(ctx context.Context, opts ...RenderOption) (*scene.Scene, Report, error)

// SVG renders the chart and writes it as SVG.
func (c *Chart) SVG(ctx context.Context, w io.Writer, opts ...RenderOption) (Report, error)

// Image renders the chart and rasterizes it.
func (c *Chart) Image(ctx context.Context, opts ...RenderOption) (*image.RGBA, Report, error)

// NewView runs the spec once and keeps its state for updates. A View is not
// safe for concurrent use.
func (c *Chart) NewView(ctx context.Context, opts ...RenderOption) (*View, Report, error)

type View struct{ /* unexported */ }

func (v *View) Scene() *scene.Scene
func (v *View) Update(ctx context.Context, changes ...Change) (Report, error)

// Data, Signal and Scale read the datasets, the signals and the scales of
// the top level, as the view of vega.js does. The conformance check of D17
// reads them.
func (v *View) Data(name string) ([]map[string]any, error)
func (v *View) Signal(name string) (any, error)
func (v *View) Scale(name string) (scale.Scale, error)
```

The code has all of this document but these:

1. `WithFonts`. Since phase 6, `Image` draws with the bundled IBM Plex
   Sans and no font of the system (BACKLOG).
2. `WithEvaluator`, which comes with the package `script` (D9).
3. `Update` and `Change`, which come with live data in phase 8.
4. The whole of Vega-Lite. Since phase 4, `Parse` and
   `FromVegaLite` compile a single view, a layer, a facet, a repeat and a
   concatenation, with the composite marks. A part that the compiler does
   not handle, such as a selection, is an issue that wraps
   `vega.ErrUnsupported`, or is left out where the backlog says so.
5. The types of the columns of a result set, in item 3 of "Result sets".
   zuzu does not read `ColumnTypes`, and a field with no type does not
   get the type of its column.
6. The check of an unknown property, in item 3 of "Fields with more than
   one form". An unknown property goes in `Extra`, with no issue and no
   error, and no code returns `ErrUnknownProperty`.

`SVG` and `Image` are short forms. A program that wants the scene graph calls
`Render` and gives the result to a renderer, because every renderer reads the
scene graph and only the scene graph (D8):

```go
s, report, err := chart.Render(ctx)
err = svg.Write(w, s)
pix, err := raster.Draw(s, raster.WithRenderer(fonts)) // phase 6 (D11)
```

`svg.Write` takes options. `svg.WithFont` embeds a font in a rule
`@font-face` when a text names its family, as `SVG` does for the font of a
look drawn by hand (D43).

`raster.Draw` exists since phase 6. `raster.WithRenderer` names the
renderer of okage whose fonts draw the text, with the layout of text of
okage D37. A program makes one renderer and shares it. It gives the
renderer `WithLoadSystemFonts(false)`, because a renderer of okage loads
the fonts of the system by default (okage D36), and the image then depends
on the machine. Without `raster.WithRenderer`, `raster.Draw` makes a
renderer with the bundled fonts alone.

### Links

An item of the scene graph carries a link, `Href`, and a tooltip,
`Tooltip`, which the encodings `href` and `tooltip` of Vega fill on every
mark (D32). `svg.Write` writes the link as an `<a href>` around the item
and the tooltip as a `<title>` inside it. `raster.Draw` draws neither.

A tooltip that is not a string shows as Vega shows it without a plugin. An
object shows one line `key: value` for each field, in the order of the
keys, and an array shows as `[a, b]`.

`scene.LinkAllowed` says which links a renderer writes. A link with no
scheme, such as `page.html`, `/a/b` or `#part`, is allowed. A link with
the scheme `http`, `https`, `mailto`, `ftp` or `tel` is allowed. A link
with any other scheme, such as `javascript:`, `data:` or `file:`, is not
written, because it can run a script or open a file. The runtime reports
such a link as an issue, and the item has no link.

### Options

An option is a functional option named `With...`, and the caller owns every
value it gives (okage D20). There are two kinds, so that an option given in
the wrong place does not compile. `WithEmoji`, `WithLook`,
`WithLookSeed`, `WithLookFont`, `WithLayoutEngine` and `WithLayout` are of
both kinds (D38, D40, D44, D45, D58). Each returns a `SharedOption`, which is an `Option` and a
`RenderOption`.

`Option` applies when a spec is parsed and compiled:

| Option | Default | What it sets |
| --- | --- | --- |
| `WithDialect(Dialect)` | from `$schema`, or the first word of a diagram | `Vega`, `VegaLite`, `DOT` or `Mermaid` |
| `WithStrict(bool)` | loose | the feature mode (D26) |
| `WithFunction(name string, fn expr.Func)` | none | a Go function that an expression can call |
| `WithMath(*text.MathFont)` | none, so the math of KaTeX in a label of Mermaid draws its source | the font of math, from the package `text/math`, which draws a formula of a label as math (D36) |
| `WithEmoji(*text.Fallback)` | none, so an emoji measures as nothing | the font of the emoji, which a kind of Mermaid that lays out when it compiles, such as a sequence, a pie or a mindmap, measures an emoji with. The chart keeps it, and a render that gives no font of its own draws with it (D38) |
| `WithLook(theme.Look)` | `zuzu.look` of the spec, then `theme.LookStandard` | the look, such as `theme.LookExcalidraw`, in which a kind of Mermaid that lays out when it compiles measures its text. The chart keeps it, and a render that gives no look of its own draws in it (D40) |
| `WithLookSeed(int64)` | `zuzu.lookSeed` of the spec, then `theme.SketchSeed` | the seed of a look drawn by hand. The chart keeps it, and a render that gives no seed of its own draws with it (D58) |
| `WithLookFont(theme.Look, *text.Fallback)` | none | the font of a look drawn by hand, such as Excalifont of the package `text/excalifont`, which a kind of Mermaid that lays out when it compiles measures its text in when the parse draws in that look. The chart keeps it (D40) |
| `WithLayoutEngine(graph.Engine)` | the built in engines, `layered` and `dagre` | a layout engine that a graph can lay out with, under its name, such as the engine `elk` of `elk.Layout()`. It forces nothing. The chart keeps it (D45) |
| `WithLayout(name string)` | the layout of the source | the layout that each graph of the chart lays out with, such as `dagre` or `elk.stress`, over the layout of the source. The chart keeps it, and a render that forces no layout of its own lays out with it (D45) |
| `WithNow(time.Time)` | none, so a gantt draws no marker of today and puts a date with no day on 1970-01-01, and `now()` gives 0 | the time of the render. A gantt of Mermaid draws the marker of today at it, and takes the day of a date with no day from it, in the time zone of the time. `now()` gives it. The chart keeps it (D38) |
| `WithEvaluator(expr.Evaluator)` | the native evaluator | the evaluator of expressions, such as the goja one (D9). Not built yet |
| `WithLogger(*slog.Logger)` | none | where zuzu logs |

`RenderOption` applies when a chart renders:

| Option | Default | What it sets |
| --- | --- | --- |
| `WithLoader(Loader)` | none | how the data of a `url` is read (D25) |
| `WithValues(name string, rows []map[string]any)` | none | the rows of a named dataset, given by the program |
| `WithResultSet(name string, rs ResultSet)` | none | the rows of a named dataset, read from the result of a query |
| `WithMode(theme.Mode)` | `zuzu.mode` of the spec, then `theme.Light` | the light or the dark mode (D14). A `zuzu.background` of the spec wins over it |
| `WithScheme(theme.Scheme)` | `zuzu.scheme` of the spec, then `theme.Zuzu` | the color scheme (D14, D15) |
| `WithLocation(*time.Location)` | `time.Local` | the time zone of time scales and formats (D27) |
| `WithLocale(format.Locale)` | `format.EnUS` | the locale of numbers and dates (D27) |
| `WithRandom(func() rand.Source)` | a source seeded from the system | the random numbers of a render (D28) |
| `WithFonts(fs.FS)` | IBM Plex Sans, as `text.Fonts()` gives it (D16) | the fonts that text is measured and drawn with, in a file system such as the one that `text.Fonts()` returns. Not built yet |
| `WithSize(width, height float64)` | from the spec | the size of the chart |
| `WithBackground(color.Color)` | `zuzu.background` of the spec, then the spec, then the theme | the background over the theme and the spec. An alpha of 0 draws none, and nil keeps the default |
| `WithScale(float64)` | 1 | the ratio of device pixels to the pixels of the chart in the image of `Image`, such as 2 for a screen of high density. The layout does not change, and the SVG does not change |
| `WithRenderLogger(*slog.Logger)` | none | where a render logs |
| `WithIcons(name string, *icon.Pack)` | the five built-in icons | a pack of icons, under the name that a diagram uses for it (D31) |
| `WithIconLoader(IconLoader)` | none | how a render loads an icon that no pack holds (D31) |
| `WithEmoji(*text.Fallback)` | the font of `WithEmoji` of `Parse`, then none, so an emoji draws nothing | the font of the emoji, from the package `text/emoji`, which draws an emoji in color (D32). `Parse` accepts it too, and the font of the render wins over the font of `Parse` (D38) |
| `WithLook(theme.Look)` | the look of `WithLook` of `Parse`, then `zuzu.look` of the spec, then `theme.LookStandard` | the look in which the chart draws, as "Looks" says (D40) |
| `WithLookSeed(int64)` | the seed of `WithLookSeed` of `Parse`, then `zuzu.lookSeed` of the spec, then `theme.SketchSeed` | the seed of a look drawn by hand, as "Looks" says (D58) |
| `WithLookFont(theme.Look, *text.Fallback)` | the font of `WithLookFont` of `Parse` for the look, then none, so the look draws its text in IBM Plex Sans | the font of a look drawn by hand, as "Looks" says (D40) |
| `WithLayoutEngine(graph.Engine)` | the engines of `WithLayoutEngine` of `Parse` | a layout engine that a graph can lay out with, as "Layouts" says. An engine of the render wins over an engine of `Parse` with the same name (D45) |
| `WithLayout(name string)` | the layout of `WithLayout` of `Parse`, then the layout of the source | the layout that each graph lays out with, as "Layouts" says. An empty name forces no layout (D45) |

A spec can ask for a look in its `config`, with two keys that Vega does not
have (D6, D32). `zuzu.mode` is `light` or `dark`, and `zuzu.scheme` is the
name of a scheme that `theme.Lookup` finds, such as `mermaid-forest`. The
compiler of Vega-Lite passes both on to the Vega config. The `theme` of a
Mermaid diagram fills them. A mode that the caller gives with `WithMode`
wins over `zuzu.mode`, and a scheme that the caller gives with `WithScheme`
wins over `zuzu.scheme`, so a program that sets its own look keeps it. A
value that zuzu does not have is an issue, even when the caller sets the
look. It is an error in the strict mode. In the loose mode, it is a line of
the report, and the render keeps the look of the caller or the default.
`theme.ConfigMode` and `theme.ConfigScheme` name the two keys.

A third key, `zuzu.background`, is a color of CSS that fills the page
(D37). The `bgcolor` of a graph of DOT fills it. The color is the
background of the theme, as a color of `WithBackground` is, so the boxes
under the labels of a diagram and the open arrowheads take it. The mode
follows its lightness, so that the text stays readable on it. A color with
a lightness in HSL below 0.5 draws in the dark mode, and another color in
the light mode. This mode wins over `WithMode` and over `zuzu.mode`. A
color with an alpha of 0 draws no background and keeps the mode. A
background that the caller gives with `WithBackground` wins over
`zuzu.background`, and the mode is then that of the caller or of
`zuzu.mode`. The compiler of Vega-Lite passes the key on. A value that is
not a color is an issue, as above. `theme.ConfigBackground` names the key.

### Looks

A look changes the shapes and the font of a chart, and keeps the colors
of its mode and its scheme (D39, D40). `theme.LookStandard` is the look
of `docs/STYLE.md`, the default. `theme.LookExcalidraw` draws each mark
as if by hand, as "The look like Excalidraw" in `docs/STYLE.md` says, and
draws its text in Excalifont. zuzu draws no other look by hand (D57).

1. `WithLook` of a render wins over `WithLook` of `Parse`, which wins over
   the key `zuzu.look` of the config of the spec, whose values are the
   names of the looks, `standard` and `excalidraw`. The compiler of
   Vega-Lite passes the key on. `look: handDrawn` of a Mermaid diagram
   fills it, and `classic` and `neo` draw in the standard look, as
   `docs/DIAGRAMS.md` says. `theme.ConfigLook` names the key.
2. The key `zuzu.lookSeed` of the config is the seed of a look drawn by
   hand, a whole number, 1 by default. `handDrawnSeed` of Mermaid fills
   it, and its default, 0, keeps the seed 1. The same source, seed and
   fonts give the same bytes on the same architecture (D41).
   `theme.ConfigLookSeed` names the key.
3. `WithLookSeed` of a render wins over `WithLookSeed` of `Parse`, which
   wins over `zuzu.lookSeed`, as the looks of rule 1 do (D58). A seed of 0
   is the seed 1, `theme.SketchSeed`. The seed changes nothing in the
   standard look.
4. A look that zuzu does not have, and a seed that is not a whole number,
   are issues, as a mode that zuzu does not have is. The render then draws
   in the standard look, or with the seed 1.
5. `WithLookFont` gives the font of a look, by look, as `WithEmoji` gives
   the font of the emoji. The look like Excalidraw measures its text in
   the font, with IBM Plex Sans for each character that the font does not
   have, so each label fits its box. The SVG names the family of the font
   first and embeds the font in a rule `@font-face` (D43), and the PNG of
   `Image` draws with the font. Without the font, the look draws its text
   in IBM Plex Sans, and the report warns of it with `ErrNoLookFont`, in
   the strict mode too. A font for the standard look changes nothing.
6. `Parse` takes `WithLook` and `WithLookFont`, because a kind of Mermaid
   that lays out when it compiles measures its text then. The look of
   `Parse` is the look of `WithLook`, or else the look of the source. A
   render that gives another look draws in it, with the layout of the
   parse. `Parse` takes `WithLookSeed` too, and the chart keeps the seed
   for each render, as it keeps the look.

```go
font := zuzu.WithLookFont(theme.LookExcalidraw, excalifont.Font())
c, report, err := zuzu.Parse(src, zuzu.WithLook(theme.LookExcalidraw), font)
img, report, err := c.Image(ctx, font)
```

A look drawn by hand runs after the layout and before every renderer, in
`Render`, so the scene that `Render` returns, and its JSON form, hold the
paths of the look (D40 part 7). The SVG writer and `raster` read the same
scene and draw the same look.

### Layouts

A layout engine lays out the graph of a diagram (D39 part 10, D44). The
interface `graph.Engine` holds it:

```go
package graph

type Engine interface {
	// Name returns the name of the engine, such as layered or dagre. The
	// transform of the engine is zuzu. and the name, such as zuzu.dagre.
	Name() string
	// Lay lays out a graph, with the places in the order of the input.
	Lay(in LayoutInput) (*Layout, error)
}

func Layered() Engine // the engine of zuzu.layered (D31)
func Dagre() Engine   // the engine of zuzu.dagre (D39 part 12, D45)

// The names of the engines. The module github.com/xo/zuzu/elk gives the
// engine of EngineELK.
const (
	EngineLayered = "layered"
	EngineDagre   = "dagre"
	EngineELK     = "elk"
)

// Finish puts the ends on the outlines of the shapes, and sets the ends,
// the labels at the ends, the xlabels, the size and the paths of a layout
// that an engine made from its places and points.
func Finish(in *LayoutInput, out *Layout)
```

```go
package elk // the module github.com/xo/zuzu/elk, under the EPL-2.0

// Layout returns the engine of ELK, named elk.
func Layout() graph.Engine
```

`LayoutInput` holds the nodes with their sizes, the clusters nested by their
parents, the edges with the boxes of their labels and of the labels at
their ends, the ports, the direction and the spaces. `Layout` holds the
places of the nodes and the clusters, the points of each edge and the
places of its labels. An engine names each part of the input that it does
not lay out as the input asks in `Layout.Unsupported`, and the transform
reports each one as an issue that wraps `vega.ErrUnsupported`, which is an
error in the strict mode (hard rule 4).

An engine is available when the caller gives it with `WithLayoutEngine`,
or when it is built in: `layered` and `dagre` (D45). zuzu registers no
engine at the level of a package (hard rule 9). The module
`github.com/xo/zuzu/elk` gives the engine of ELK, named `elk`
(`graph.EngineELK`), which lays out the layout `elk` and each layout whose
name starts with `elk.`, such as `elk.stress`. The runtime picks the layout
of each graph of a render in this order:

1. The layout that `WithLayout` of the render forces, else the layout that
   `WithLayout` of `Parse` forces. The option of the caller wins over the
   source, as `WithLook` does. A forced layout with no available engine is
   an issue that wraps `vega.ErrUnsupported`, an error in the strict mode,
   and the next rules pick the layout. The first graph that lays out
   reports the issue, so a chart with no graph reports nothing.
2. The key `zuzu.layout` of the config of the spec, which
   `graph.ConfigLayout` names. A layout with an available engine lays out
   with it. A layout of ELK with no engine of ELK lays out with the
   transform of the spec, `zuzu.layered`, with no issue (D32). Another
   layout with no engine is an issue, and the transform of the spec draws.
3. The engine of each transform of a graph in the spec, `zuzu.layered` or
   `zuzu.dagre`.

A diagram of Mermaid fills `zuzu.layout` for its graphs. A graph that sets
no layout, in a kind whose default layout in Mermaid 12.3 is ELK, gives
`elk`: the flowchart, the state diagram, the class diagram, the ER
diagram, the requirement diagram, the agentflow diagram and the use case
diagram. `layout: dagre` gives `dagre`, and an ELK layout gives its name.
The swimlane diagram, which lays out in lanes, and DOT give nothing, so
they draw with `zuzu.layered`. So without an engine of ELK, the output is
the output of `zuzu.layered`, except for `layout: dagre`, which draws with
`zuzu.dagre` (D45 part 2).

The engine gets the name of the layout as `LayoutInput.Algorithm`, so that
an engine with more than one algorithm, such as the one of ELK, can choose
one.

```go
c, report, err := zuzu.Parse(src, zuzu.WithLayoutEngine(elk.Layout()))
report, err = c.SVG(ctx, w, zuzu.WithLayout("dagre"))
```

The logger of a render is `WithRenderLogger`, and `WithLogger` is the
logger of `Parse`, so that each logger has its own place.

Without `WithNow`, a call of `now()` in an expression gives 0, so that the
same spec gives the same bytes. The report names each expression that
calls it, and in the strict mode the call is an error that wraps
`ErrNoNow`. A function `now` of `WithFunction` takes the place of the built
in one and needs no time.

`WithRandom` takes a function that makes a source, and not a source, so that
two renders on two goroutines never share one. The `rand` package is
`math/rand/v2`.

### The loader

zuzu reads no file and makes no request of its own (D25). A spec that names
a `url` needs a loader:

```go
// Loader reads the data that a spec names by url.
type Loader interface {
	Load(ctx context.Context, req LoadRequest) (io.ReadCloser, error)
}

// LoadRequest says what a spec asks for.
type LoadRequest struct {
	URL    string // the url of the spec, as written
	Format string // csv, tsv, json or topojson, from the spec, or image for the image of a node
	Name   string // the name of the dataset
}

// LoaderFunc makes a Loader of a function.
type LoaderFunc func(context.Context, LoadRequest) (io.ReadCloser, error)

// FSLoader reads each url as a path in fsys. The caller chooses fsys, so
// zuzu still reads only what the caller allows.
func FSLoader(fsys fs.FS) Loader
```

Without a loader, a spec with a `url` fails with an error that wraps
`ErrNoLoader`, in either feature mode.

### Icons

A diagram names an icon as `name:icon`, such as `fa:user`. A render looks
for it in the packs of `WithIcons`, then asks the icon loader. A name with
no prefix names a built-in icon: `cloud`, `database`, `disk`, `internet` or
`server`. A missing icon draws as a box with a question mark. It is an
error in the strict mode and a line of the report in the loose mode
(D31).

```go
// IconLoader gives the icons that no pack holds. zuzu asks it once for each
// icon of a render.
type IconLoader interface {
	LoadIcon(ctx context.Context, req IconRequest) (io.ReadCloser, error)
}

// IconRequest names an icon: aws and lambda, for aws:lambda.
type IconRequest struct {
	Prefix, Name string
}

// IconLoaderFunc makes an IconLoader of a function.
type IconLoaderFunc func(context.Context, IconRequest) (io.ReadCloser, error)

// FSIconLoader reads <prefix>/<name>.svg, or else the pack of Iconify
// <prefix>.json, from fsys.
func FSIconLoader(fsys fs.FS) IconLoader
```

The bytes are an SVG document, an icon of Iconify, or a pack of Iconify.
The package `icon` reads packs and icons, and zuzu bundles four packs, each
in a package of its own: `icons/carbon`, `icons/fa`, `icons/mdi` and
`icons/k8s`. Each has `Pack() *icon.Pack`.

```go
c, _, err := zuzu.Parse(src)
// ...
_, err = c.SVG(ctx, w, zuzu.WithIcons("fa", fa.Pack()), zuzu.WithIcons("fas", fa.Pack()))
```

An image of a node, `img: "url"` in Mermaid, comes through the loader of
data, `WithLoader`, with the format `image`.

### Result sets

A program that charts the result of a query gives the result as it is, and
zuzu reads it into a dataset:

```go
// ResultSet is the result of a query. *sql.Rows satisfies it, and so does
// the ResultSet of github.com/xo/tblfmt, which usql uses.
type ResultSet interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
}
```

```go
rows, err := db.QueryContext(ctx, "SELECT day, total FROM sales")
if err != nil {
	return err
}
defer rows.Close()
report, err := chart.SVG(ctx, w, zuzu.WithResultSet("sales", rows))
```

1. zuzu reads each row once, during the render, until `Next` returns false,
   and then checks `Err`. The caller closes the result set. zuzu does not.
2. Each row becomes a `map[string]any`, keyed by the names that `Columns`
   gives. A column scans into an `any`, so a NULL is `nil`, a `[]byte`
   becomes a `string` and an integer becomes a `float64`.
3. If the result set also has `ColumnTypes`, as `*sql.Rows` has, zuzu reads
   the type of each column from it. Otherwise, it reads the type from the Go
   values: a `time.Time` is temporal, a number is quantitative and a string
   is nominal. A Vega-Lite spec that names no type for a field then gets the
   type of its column. This item is not built yet.
4. A result set can be read once, so `WithResultSet` gives the data of one
   render. A `View` takes new rows through `Update`.

`ResultSet` holds only the four methods that zuzu calls, so that any type with
them works without an adapter. `usql` charts a query this way, much later
(D20).

### The report and the errors

```go
// Report lists what zuzu did not draw in the loose mode, and what it
// warns of in either mode.
type Report = vega.Report

// in package vega:
type Report []Issue

type Issue struct {
	Err     error  // a sentinel error, such as ErrUnsupportedTransform
	Path    string // a JSON pointer into a spec, such as "/transform/2", or a place in a diagram, such as "line 12"
	Message string // the detail, such as the name of the transform
}

func (i Issue) Error() string
func (i Issue) Unwrap() []error
```

An issue is an error, so a caller tests it the same way in both modes:

```go
for _, issue := range report {
	if errors.Is(issue, vega.ErrUnsupportedTransform) {
		// ...
	}
}
```

In the strict mode, the first unsupported feature stops the work, and the
error is that issue. In the loose mode, zuzu
skips the feature, draws the rest, and adds an `Issue` to the report (D26).
A malformed spec is an error in both modes.

The sentinel errors are constants of a string type. The errors that the spec
causes are in `vega`, so that `vegalite` and the root package share them, and
the root package names its own:

```go
package vega

type Error string

func (err Error) Error() string { return string(err) }

const (
	ErrInvalidSpec          Error = "invalid spec"
	ErrUnsupported          Error = "unsupported feature"
	ErrUnsupportedMark      Error = "unsupported mark"
	ErrUnsupportedTransform Error = "unsupported transform"
	ErrUnsupportedFunction  Error = "unsupported expression function"
	ErrUnknownProperty      Error = "unknown property"
)
```

```go
package zuzu

const (
	ErrUnknownDialect Error = "unknown spec dialect"
	ErrNoLoader       Error = "no loader for url data"
	ErrCycle          Error = "cycle in the spec"
	ErrUnknownName    Error = "unknown name"
	ErrNoGlyph        Error = "no font has the character"
	ErrNoNow          Error = "no time of the render"
	ErrNoLookFont     Error = "no font of the look"
)
```

`ErrCycle` is the error of a signal, a dataset or a scale that depends on
itself. `ErrUnknownName` is the error of a reference to a signal, a dataset
or a scale that the spec does not have.

`ErrNoGlyph` is the error of a character of a text that no font of the
render has, such as a character of Chinese, or an emoji without
`WithEmoji`. The character draws nothing. In the loose mode, the report
names each such character once for each render (D32).

`ErrNoNow` is the error of an expression that calls `now()` when `Parse`
has no `WithNow`, in the strict mode. In the loose mode, `now()` gives 0,
and the report names the expression once for each render (D38).

`ErrNoLookFont` is a warning, and never an error. A look drawn by hand
with no font of the look, which `WithLookFont` gives, draws its text in
IBM Plex Sans, so it drops nothing. The report holds the warning once for
each render, in the loose and the strict mode alike (D40). It is the one
warning of the report. A caller tells it apart with
`errors.Is(issue, zuzu.ErrNoLookFont)`. A gradient in a look drawn by hand
is no issue, because the look keeps its colors.

A constant cannot wrap another error, so the issue does it. An issue for an
unsupported mark, transform or function unwraps to its own sentinel and to
`ErrUnsupported`, with `Unwrap() []error`, so that
`errors.Is(err, vega.ErrUnsupported)` matches any of them.

### Changes

A `View` takes changes to its data, in the manner of the changesets of Vega.
This is for live data (D19), and its form can change before phase 8.
`Change` and `View.Update` are not built yet.

```go
// Change is one change to one dataset.
type Change struct {
	Data    string           // the name of the dataset
	Insert  []map[string]any // rows to add
	Remove  func(row map[string]any) bool
	Replace bool             // remove every row first
}
```

## vega and vegalite

The two packages have separate trees of types (D21). Vega-Lite is a grammar of
its own, and not a short form of the types of Vega, so no type is shared.

```go
package vega

// Parse reads a Vega spec. It checks the structure, and not whether zuzu
// supports each feature, which the root package checks.
func Parse(src []byte) (*Spec, error)

type Spec struct {
	Scope

	Schema      string      `json:"$schema,omitzero"`
	Description string      `json:"description,omitzero"`
	Width       NumberValue `json:"width,omitzero"`
	Height      NumberValue `json:"height,omitzero"`
	Padding     Value       `json:"padding,omitzero"`
	Autosize    Value       `json:"autosize,omitzero"`
	Background  Value       `json:"background,omitzero"`
	Style       Strings     `json:"style,omitzero"`
	Config      Config      `json:"config,omitzero"`

	Extra Extra `json:",embed"`
}

// Scope holds what a spec and a group mark both hold.
type Scope struct {
	Signals     []Signal     `json:"signals,omitzero"`
	Data        []Data       `json:"data,omitzero"`
	Scales      []Scale      `json:"scales,omitzero"`
	Projections []Projection `json:"projections,omitzero"`
	Axes        []Axis       `json:"axes,omitzero"`
	Legends     []Legend     `json:"legends,omitzero"`
	Title       *Title       `json:"title,omitzero"`
	Marks       []Mark       `json:"marks,omitzero"`
	Layout      Value        `json:"layout,omitzero"`
	Encode      Encode       `json:"encode,omitzero"`
}

// Extra holds the properties that zuzu does not know, so that the report
// can name them and a spec can be written back whole.
type Extra = map[string]jsontext.Value
```

```go
package vegalite

func Parse(src []byte) (*Spec, error)

// Compile turns a Vega-Lite spec into a Vega spec. The report names each
// part that zuzu did not compile, in the loose mode.
func Compile(spec *Spec, opts ...CompileOption) (*vega.Spec, vega.Report, error)
```

`zuzu.Parse` calls `vega.Parse`, or `vegalite.Parse` and `vegalite.Compile`.
A program can call them itself, to read the compiled Vega spec of a
Vega-Lite spec. `Chart.Spec` gives the same thing for a parsed chart.

The checks happen in three places:

1. `vega.Parse` and `vegalite.Parse` check the structure: the JSON is well
   formed, and each field has a type that the grammar allows.
2. `vegalite.Compile` checks the meaning of the Vega-Lite spec as it lowers
   it to Vega.
3. The root package checks that zuzu supports each feature of the Vega spec,
   and applies the feature mode.

### Fields with more than one form

Many fields of Vega take more than one form. A width is a number or a signal.
A fill is a value, a field, a signal or a scale. zuzu types each one as a
tagged union, and never as `map[string]any`:

```go
// NumberValue is a number or a reference to a signal.
type NumberValue struct {
	Kind   RefKind // Absent, Literal or SignalRef
	Number float64
	Signal string
}

func (v *NumberValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error
func (v NumberValue) MarshalJSONTo(enc *jsontext.Encoder) error
func (v NumberValue) IsZero() bool
```

zuzu reads and writes JSON with `encoding/json/v2` and `encoding/json/jsontext`
of the standard library, which TinyGo supports (D29). It writes no decoder of
its own, and no registry of types exists.

1. Each union implements `UnmarshalJSONFrom`. It looks at the kind of the
   next token with `Decoder.PeekKind`, and decodes the rest with
   `json.UnmarshalDecode`. `MarshalJSONTo` writes it back.
2. The `Kind` tells an absent field from a zero. `IsZero` reports an absent
   field, and the `omitzero` tag leaves it out when the spec is written back,
   so a spec written back is the spec that was read.
3. The `Extra` field of each object, with the tag `json:",embed"`, collects
   the properties that zuzu does not know. In the loose mode, each one is an
   issue that wraps `ErrUnknownProperty`. In the strict mode, zuzu decodes
   with `json.RejectUnknownMembers(true)`, and an unknown property is an
   error. The issue and the error are not built yet.
4. zuzu writes JSON with `json.Deterministic(true)`, so that a map writes its
   keys in the same order each time, and the same spec gives the same bytes.

A test on 2026-11-10 decoded and wrote back such a union, an `Extra` field
and an unknown property with Go 1.27.3 and with TinyGo 0.44.0, for Linux and
for WebAssembly, with the same result.

## Building a spec in Go

zuzu has no fluent builder. A program builds a spec as Go values, with a few
small constructors for the unions:

```go
spec := &vegalite.Spec{
	Data: &vegalite.Data{Values: vega.Lit(rows)},
	Mark: &vegalite.Mark{Type: "line"},
	Encoding: vegalite.Encoding{
		"x": {Defs: []vegalite.FieldDef{{Field: vega.Lit("date"), Type: "temporal"}}},
		"y": {Defs: []vegalite.FieldDef{{Field: vega.Lit("price"), Type: "quantitative"}}},
	},
}
chart, report, err := zuzu.FromVegaLite(spec)
```

The constructors are `vega.Lit` for a `Value`, `vega.Num` for a
`NumberValue` and `vega.List` for a list of strings.

A constructor is added when code repeats a pattern, not before. `usql` builds
its charts this way, much later (D20).

## Themes, fonts and expressions

```go
package theme

type Mode int

const (
	Light Mode = iota
	Dark
)

// Scheme is a color scheme. Zuzu, Pastel, Harbor and Ember are built in,
// and Schemes lists every scheme by name, with those from upstream themes
// (D15) and from the themes of Mermaid, such as mermaid-forest (D32). A
// scheme from an upstream theme can bring its own neutrals for a mode.
type Scheme struct{ /* unexported */ }

func Lookup(name string) (Scheme, bool)
func Schemes() []string

// ConfigMode, ConfigScheme, ConfigBackground, ConfigLook and
// ConfigLookSeed are the keys of the config of a spec that ask for a mode,
// a scheme, a background, a look and the seed of a look (D32, D37, D40).
const (
	ConfigMode       = "zuzu.mode"
	ConfigScheme     = "zuzu.scheme"
	ConfigBackground = "zuzu.background"
	ConfigLook       = "zuzu.look"
	ConfigLookSeed   = "zuzu.lookSeed"
)

// Look is a way to draw the marks of a chart (D40). Known reports whether
// zuzu draws a look, and HandDrawn whether it draws as if by hand. The
// tokens of the look like Excalidraw, such as SketchRoughness, are
// constants of the package, from docs/STYLE.md.
type Look string

const (
	LookStandard   Look = "standard"
	LookExcalidraw Look = "excalidraw"
)

func (l Look) Known() bool
func (l Look) HandDrawn() bool

// Theme is a mode and a scheme together: every color that a chart uses.
type Theme struct{ /* the neutrals and the colors */ }

func New(m Mode, s Scheme) Theme

// Config returns the Vega config that the mode and the scheme make (D14).
func (t Theme) Config() map[string]any
```

```go
package expr

// Func is a function that an expression can call. It takes the scope of
// the call, so that a function of the view, such as scale, reaches it.
type Func func(s *Scope, args []any) (any, error)

// Scope is what an expression reads when it runs: the datum, the datum of
// the group, the signals, the functions, the time zone, the random source
// and the locale.
type Scope struct{ /* fields */ }

// Compile parses an expression, and Eval runs it in a scope.
func Compile(src string) (*Program, error)
func (p *Program) Eval(s *Scope) (any, error)
```

A value of an expression is a plain Go value, and not a type of its own:
`float64` for a number, `string`, `bool`, `nil` for null, `expr.Undefined`,
`time.Time` for a date, `[]any` for an array, `map[string]any` for an
object and `*expr.RegExp`. The rows of a dataset hold the same values, so a
function of the caller reads them with no conversion. The interface
`Evaluator`, which the goja evaluator implements (D9), comes with the
package `script`.

The package `script` holds the evaluator that runs scripts in goja (D9). A
program that does not import it does not link goja.

### Emoji

zuzu draws text in IBM Plex Sans (D16). The package `text/emoji` bundles
Noto Color Emoji, the font of 10.7 MB that draws an emoji in color (D32). A
program that does not import the package does not hold the font.
`NewFallback` accepts a font in TrueType, OpenType, WOFF, WOFF2 or EOT, and
`Data` gives it as TrueType or OpenType (D34). A render takes the font with
`WithEmoji`, as it takes a pack of icons. `Parse` takes it too, so that a
kind of Mermaid that lays out when it compiles measures an emoji with it
(D38):

```go
package text

// Fallback is a font for the characters that IBM Plex Sans does not have.
type Fallback struct{ /* unexported */ }

func NewFallback(family string, data []byte) *Fallback
func (f *Fallback) Family() string
func (f *Fallback) Data() []byte
```

```go
package emoji

// Font returns Noto Color Emoji as a fallback.
func Font() *text.Fallback
```

```go
c, report, err := zuzu.Parse(src, zuzu.WithEmoji(emoji.Font()))
img, report, err := c.Image(ctx, zuzu.WithEmoji(emoji.Font()))
```

With the font, an emoji measures at the width that the font gives it, so
that its label fits, and the PNG of `Image` draws it in color. The SVG
names the family `'Noto Color Emoji'` after the family of a text that
holds a character that IBM Plex Sans does not have, and a viewer draws
the emoji in the color font that it has. Without the font, an emoji is a
character that no font has: it draws nothing, and it is `ErrNoGlyph`.

### Excalifont

The package `text/excalifont` bundles Excalifont, the font of the look
like Excalidraw, under the SIL Open Font License 1.1 (D40). It holds the
file of Excalidraw in WOFF2, of 52 KB, unchanged. A program that does not
import the package does not hold the font. `Font` gives the font in the
same form as `emoji.Font`, so `Data` gives the TrueType bytes that a
renderer of okage loads:

```go
package excalifont

// Family is the family of the font.
const Family = "Excalifont"

// Font returns Excalifont.
func Font() *text.Fallback
```

`WithLookFont` takes the font, as "Looks" says, and without the font the
look draws its text in IBM Plex Sans (D40). Excalifont has no glyph for
seven characters of Latin-1: `¤`, `§`, `¬`, `²`, `³`, `µ` and `¹`. Such a
character draws in IBM Plex Sans.

`WithLookFont` is an option of its own, and not a second argument of
`WithLook`, because a source can ask for the look by itself, such as
`look: handDrawn` of Mermaid, and the font then still reaches the render.
It takes the look, so that each look drawn by hand gets its own font. This
amends D24.

An `Estimate` of the package `text` measures in the font with `SetMain`:

```go
package text

// SetMain gives the Estimate a main font, which measures each character
// that it has before IBM Plex Sans, in each family, weight and style.
func (e *Estimate) SetMain(f *Fallback)
```

## Where the models agreed and differed

All three said:

1. Keep separate trees of types for `vega` and `vegalite`, with
   `vegalite.Compile` from one to the other.
2. Let `$schema` choose between them in `zuzu.Parse`.
3. Make the parsed chart immutable and safe for concurrent use.
4. Do not write a fluent builder. Use Go values and small constructors.
5. Type each field with more than one form as a tagged union that decodes
   itself, and never as `map[string]any`, for TinyGo.

They differed on five points:

1. Astra put the writers outside the root package, as `svg.Write` and
   `raster.Draw`. Gemini put `RenderSVG` on the view. The API does both: the
   writers are in their packages, and `Chart.SVG` and `Chart.Image` are short
   forms.
2. Astra required `WithDialect` for a spec without `$schema`. Gemini and
   DeepSeek guessed. The API follows Astra, because a guess can be wrong.
3. Astra and DeepSeek split the options into those of the parse and those of
   the render. Gemini had one kind. The API splits them, so that a misplaced
   option does not compile.
4. Astra proposed a function that makes a random source, so that renders on
   two goroutines never share one. The others took a source. The API takes
   the function.
5. DeepSeek put `context.Context` on `Parse`. The others did not, because a
   parse does no I/O and takes little time. The API follows the others.

## Answers to the open questions

The maintainers answered the three questions of the draft on 2026-11-10:

1. Rows that a program gives are `[]map[string]any`, as `WithValues` and
   `Change` show. The maintainers added the same day that zuzu also takes
   the result of a SQL query, as a `ResultSet`, which "Result sets" above
   shows.
2. The form of `Change` stands for now, and it can change before phase 8.
3. `encoding/json` works with TinyGo, so zuzu writes no decoder of its own.
   The maintainers then chose `encoding/json/v2` of the standard library, and
   "Fields with more than one form" above shows how zuzu uses it.
