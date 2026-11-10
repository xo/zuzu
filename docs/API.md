# API

This document holds the public API of zuzu (D24). The maintainers accepted it
on 2026-11-10, before any code exists, and it can change as the code shows
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

// Parse reads a Vega or a Vega-Lite spec. The $schema field decides which.
// A spec without $schema needs WithDialect. Parse never guesses.
func Parse(src []byte, opts ...Option) (*Chart, Report, error)

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

On 2026-11-10 the code of phase 3 has all of the above but these:

1. `Image` and `WithFonts`, which come with the fonts of okage (BACKLOG).
2. `WithEvaluator`, which comes with the package `script` (D9).
3. `Update`, which comes with live data in phase 8.
4. The whole of Vega-Lite. Since phase 4 started, `Parse` and
   `FromVegaLite` compile a single view, a layer, a facet, a repeat and a
   concatenation, with the composite marks. A part that the compiler does
   not handle, such as the config of a spec, is an issue that wraps
   `vega.ErrUnsupported`, or is left out where the backlog says so.

`SVG` and `Image` are short forms. A program that wants the scene graph calls
`Render` and gives the result to a renderer, because every renderer reads the
scene graph and only the scene graph (D8):

```go
s, report, err := chart.Render(ctx)
err = svg.Write(w, s, svg.WithIndent("  "))
pix, err := raster.Draw(s, raster.WithText(tr)) // phase 6 (D11)
```

`raster.Draw` exists since phase 6 and draws every mark but text. A scene
with text needs a `raster.TextRenderer`, which waits for the API of okage
D37 that gives the outlines of glyphs, in okage v0.3.0 (D11).

### Options

An option is a functional option named `With...`, and the caller owns every
value it gives (okage D20). There are two kinds, so that an option given in
the wrong place does not compile.

`Option` applies when a spec is parsed and compiled:

| Option | Default | What it sets |
| --- | --- | --- |
| `WithDialect(Dialect)` | from `$schema` | `Vega` or `VegaLite`, for a spec without `$schema` |
| `WithStrict(bool)` | loose | the feature mode (D26) |
| `WithFunction(name string, fn expr.Func)` | none | a Go function that an expression can call |
| `WithEvaluator(expr.Evaluator)` | the native evaluator | the evaluator of expressions, such as the goja one (D9) |
| `WithLogger(*slog.Logger)` | none | where zuzu logs |

`RenderOption` applies when a chart renders:

| Option | Default | What it sets |
| --- | --- | --- |
| `WithLoader(Loader)` | none | how the data of a `url` is read (D25) |
| `WithValues(name string, rows []map[string]any)` | none | the rows of a named dataset, given by the program |
| `WithResultSet(name string, rs ResultSet)` | none | the rows of a named dataset, read from the result of a query |
| `WithMode(theme.Mode)` | `theme.Light` | the light or the dark mode (D14) |
| `WithScheme(theme.Scheme)` | `theme.Zuzu` | the color scheme (D14, D15) |
| `WithLocation(*time.Location)` | `time.Local` | the time zone of time scales and formats (D27) |
| `WithLocale(format.Locale)` | `format.EnUS` | the locale of numbers and dates (D27) |
| `WithRandom(func() rand.Source)` | a source seeded from the system | the random numbers of a render (D28) |
| `WithFonts(text.Fonts)` | IBM Plex Sans (D16) | the fonts that text is measured and drawn with |
| `WithSize(width, height float64)` | from the spec | the size of the chart |
| `WithRenderLogger(*slog.Logger)` | none | where a render logs |

A Go function cannot have two types, so the logger of a render is
`WithRenderLogger`, and `WithLogger` is the logger of `Parse`.

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
	Format string // csv, tsv, json or topojson, from the spec
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
   gives. A column scans into an `any`, so a NULL is `nil` and a `[]byte`
   becomes a `string`.
3. If the result set also has `ColumnTypes`, as `*sql.Rows` has, zuzu reads
   the type of each column from it. Otherwise, it reads the type from the Go
   values: a `time.Time` is temporal, a number is quantitative and a string
   is nominal. A Vega-Lite spec that names no type for a field then gets the
   type of its column.
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
	Path    string // a JSON pointer into the spec, such as "/transform/2"
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
)
```

A constant cannot wrap another error, so the issue does it. An issue for an
unsupported mark, transform or function unwraps to its own sentinel and to
`ErrUnsupported`, with `Unwrap() []error`, so that
`errors.Is(err, vega.ErrUnsupported)` matches any of them.

### Changes

A `View` takes changes to its data, in the manner of the changesets of Vega.
This is for live data (D19), and its form can change before phase 8.

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
	Schema      string      `json:"$schema,omitempty"`
	Description string      `json:"description,omitempty"`
	Width       NumberValue `json:"width,omitempty"`
	Height      NumberValue `json:"height,omitempty"`
	Padding     Padding     `json:"padding,omitempty"`
	Autosize    Autosize    `json:"autosize,omitempty"`
	Config      Config      `json:"config,omitempty"`
	Signals     []Signal    `json:"signals,omitempty"`
	Data        []Data      `json:"data,omitempty"`
	Scales      []Scale     `json:"scales,omitempty"`
	Projections []Projection `json:"projections,omitempty"`
	Axes        []Axis      `json:"axes,omitempty"`
	Legends     []Legend    `json:"legends,omitempty"`
	Title       *Title      `json:"title,omitempty"`
	Marks       []Mark      `json:"marks,omitempty"`

	// Extra holds the properties that zuzu does not know, so that the
	// report can name them and a spec can be written back whole.
	Extra map[string]jsontext.Value `json:",embed"`
}
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
	Kind   RefKind // Absent, Literal or Signal
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
   error.
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
	Data: vegalite.Values(rows),
	Mark: vegalite.Mark{Type: vegalite.Line},
	Encoding: vegalite.Encoding{
		X: vegalite.Field("date", vegalite.Temporal),
		Y: vegalite.Field("price", vegalite.Quantitative),
	},
}
chart, report, err := zuzu.FromVegaLite(spec)
```

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
// (D15).
type Scheme struct{ /* unexported */ }

func Lookup(name string) (Scheme, bool)
func Schemes() []string

// Config returns the Vega config that a mode and a scheme make (D14).
func Config(m Mode, s Scheme) vega.Config
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
