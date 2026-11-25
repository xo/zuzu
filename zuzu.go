package zuzu

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	okagesvg "github.com/xo/okage/svg"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/icon"
	"github.com/xo/zuzu/raster"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/svg"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
	"github.com/xo/zuzu/vegalite"
)

// Error is an error of the package zuzu.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package. The errors of a spec are in the package vega.
const (
	// ErrUnknownDialect is the error of a spec whose $schema names neither
	// Vega nor Vega-Lite, and of a spec with no $schema and no WithDialect.
	ErrUnknownDialect Error = "unknown spec dialect"
	// ErrNoLoader is the error of a spec that names data by url, when the
	// render has no loader (D25).
	ErrNoLoader Error = "no loader for url data"
	// ErrCycle is the error of a signal, a dataset or a scale that depends
	// on itself.
	ErrCycle Error = "cycle in the spec"
	// ErrUnknownName is the error of a reference to a signal, a dataset or a
	// scale that the spec does not have.
	ErrUnknownName Error = "unknown name"
	// ErrNoGlyph is the error of a character of a text that no font of the
	// render has, in the strict mode. In the loose mode, the character
	// draws nothing and the report names it once (D32).
	ErrNoGlyph Error = "no font has the character"
	// ErrNoNow is the error of an expression that calls now() when the
	// parse has no WithNow, in the strict mode. In the loose mode, now()
	// gives 0 and the report names the expression once (D38).
	ErrNoNow Error = "no time of the render"
	// ErrNoLookFont is the warning of a look drawn by hand when the render
	// has no font of the look, which WithLookFont gives. The look draws its
	// text in IBM Plex Sans, so nothing is dropped, and the report holds
	// the warning in either mode, never an error (D40).
	ErrNoLookFont Error = "no font of the look"
)

// Report lists what zuzu did not draw in the loose mode, and what it warns
// of in either mode (D26). A warning, such as an issue that wraps
// ErrNoLookFont, draws in full in another way, so the strict mode reports
// it and does not turn it into an error.
type Report = vega.Report

// Dialect is the language of a spec.
type Dialect int

// The dialects.
const (
	// Vega is a Vega spec.
	Vega Dialect = iota + 1
	// VegaLite is a Vega-Lite spec.
	VegaLite
	// DOT is a graph in the DOT language of Graphviz (D31).
	DOT
	// Mermaid is a diagram of Mermaid (D31).
	Mermaid
)

// Option is a setting of Parse.
type Option interface {
	applyParse(o *options)
}

// parseFunc is an Option that is a function.
type parseFunc func(*options)

// applyParse satisfies Option.
func (f parseFunc) applyParse(o *options) {
	f(o)
}

// SharedOption is a setting of both Parse and a render, such as WithEmoji.
type SharedOption interface {
	Option
	RenderOption
}

// options are the settings of Parse.
type options struct {
	dialect Dialect
	strict  bool
	funcs   map[string]expr.Func
	logger  *slog.Logger
	math    *text.MathFont
	// emoji holds the fonts of WithEmoji, which a kind of Mermaid that
	// lays out when it compiles measures an emoji with, and which a render
	// with no font of its own draws with (D38).
	emoji []*text.Fallback
	// look is the look of WithLook, and hasLook says that the option set
	// it. lookFonts are the fonts of WithLookFont, by look (D40).
	look      theme.Look
	hasLook   bool
	lookFonts map[theme.Look]*text.Fallback
	// engines are the engines of WithLayoutEngine, and layout the layout of
	// WithLayout, which hasLayout says the option set (D45).
	engines   []graph.Engine
	layout    string
	hasLayout bool
	// now is the time of WithNow, or the zero time.
	now time.Time
}

// WithDialect sets the dialect of a spec that has no $schema.
func WithDialect(d Dialect) Option {
	return parseFunc(func(o *options) { o.dialect = d })
}

// WithStrict sets the feature mode (D26). In the strict mode, a feature
// that zuzu does not support is an error. In the loose mode, the default,
// zuzu skips it and names it in the report.
func WithStrict(strict bool) Option {
	return parseFunc(func(o *options) { o.strict = strict })
}

// WithFunction adds a Go function that an expression can call by name. It
// takes the place of a built in function of the same name.
func WithFunction(name string, fn expr.Func) Option {
	return parseFunc(func(o *options) {
		if o.funcs == nil {
			o.funcs = map[string]expr.Func{}
		}
		o.funcs[name] = fn
	})
}

// WithMath gives a parse a font of math, so that the math of KaTeX in a
// label of Mermaid, written as $$...$$, draws as math (D36). The package
// text/math gives the font, STIX Two Math:
//
//	c, report, err := zuzu.Parse(src, zuzu.WithMath(math.Font()))
//
// It is an option of Parse, because some kinds of Mermaid lay out their
// labels when they compile. The chart keeps the font, and each render of
// it uses the font. Without the font, a label with math draws its source,
// and the report names it. A program that does not import text/math does
// not hold the font.
func WithMath(f *text.MathFont) Option {
	return parseFunc(func(o *options) { o.math = f })
}

// WithNow gives a parse the time of the render (D38). A gantt of Mermaid
// with the time draws the marker of today, and a date of its dateFormat
// that has no day, such as HH:mm, takes the day of the time, as in Mermaid.
// The gantt takes the day and the hour that the time has in its own time
// zone. The function now() of an expression gives the time, in
// milliseconds since 1970.
//
// It is an option of Parse, because a gantt lays out when it compiles. The
// chart keeps the time, and each render of it uses the time. Without the
// time, a gantt draws no marker and puts a date with no day on 1970-01-01,
// and now() gives 0, so that the same spec gives the same bytes. The report
// names each expression that calls now() with no time, and in the strict
// mode such a call is an error that wraps ErrNoNow. The zero time is no
// time.
func WithNow(t time.Time) Option {
	return parseFunc(func(o *options) { o.now = t })
}

// WithLogger sets where zuzu logs.
func WithLogger(l *slog.Logger) Option {
	return parseFunc(func(o *options) { o.logger = l })
}

// Chart is a parsed spec. It does not change after Parse, so it is safe for
// concurrent use.
type Chart struct {
	spec *vega.Spec
	opts options
	// report holds the issues of the compile of a Vega-Lite spec, which
	// each render reports again.
	report Report
	// diagram says that the spec comes from a diagram of DOT or Mermaid,
	// whose colors the dark mode does not flip (D48 part 1).
	diagram bool
}

// Parse reads a Vega or a Vega-Lite spec, or a diagram of DOT or Mermaid
// (D31). The $schema field decides between Vega and Vega-Lite, and the
// first word of a source finds a diagram. Another source needs
// WithDialect.
func Parse(src []byte, opts ...Option) (*Chart, Report, error) {
	var o options
	for _, opt := range opts {
		opt.applyParse(&o)
	}
	d := o.dialect
	if d == DOT || d == Mermaid {
		return parseDiagram(src, d, o)
	}
	if d == 0 {
		if g := sniffDiagram(src); g != 0 {
			return parseDiagram(src, g, o)
		}
	}
	var head struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(src, &head); err != nil {
		return nil, nil, fmt.Errorf("reading the $schema of a spec: %w: %w", vega.ErrInvalidSpec, err)
	}
	switch {
	case strings.Contains(head.Schema, "/vega-lite/"):
		d = VegaLite
	case strings.Contains(head.Schema, "/vega/"):
		d = Vega
	case head.Schema != "":
		return nil, nil, fmt.Errorf("the $schema %q: %w", head.Schema, ErrUnknownDialect)
	}
	switch d {
	case Vega:
		spec, err := vega.Parse(src)
		if err != nil {
			return nil, nil, err
		}
		return FromVega(spec, opts...)
	case VegaLite:
		spec, err := vegalite.Parse(src)
		if err != nil {
			return nil, nil, err
		}
		return FromVegaLite(spec, opts...)
	}
	return nil, nil, fmt.Errorf("a spec with no $schema: %w", ErrUnknownDialect)
}

// FromVega takes a Vega spec that a program built as Go values. It copies
// the spec, so a later change by the caller does not reach the Chart.
func FromVega(spec *vega.Spec, opts ...Option) (*Chart, Report, error) {
	var o options
	for _, opt := range opts {
		opt.applyParse(&o)
	}
	b, err := vega.Write(spec)
	if err != nil {
		return nil, nil, err
	}
	cp, err := vega.Parse(b)
	if err != nil {
		return nil, nil, err
	}
	return &Chart{spec: cp, opts: o}, nil, nil
}

// FromVegaLite takes a Vega-Lite spec that a program built as Go values,
// and compiles it to Vega. The report names each part that zuzu did not
// compile, in the loose mode.
func FromVegaLite(spec *vegalite.Spec, opts ...Option) (*Chart, Report, error) {
	var o options
	for _, opt := range opts {
		opt.applyParse(&o)
	}
	v, report, err := vegalite.Compile(spec, vegalite.WithStrict(o.strict))
	if err != nil {
		return nil, report, err
	}
	c, _, err := FromVega(v, opts...)
	if err != nil {
		return nil, report, err
	}
	c.report = report
	return c, report, nil
}

// Spec returns a copy of the Vega spec, to read or to debug.
func (c *Chart) Spec() *vega.Spec {
	b, err := vega.Write(c.spec)
	if err != nil {
		return nil
	}
	cp, err := vega.Parse(b)
	if err != nil {
		return nil
	}
	return cp
}

// RenderOption is a setting of a render.
type RenderOption interface {
	applyRender(o *renderOptions)
}

// renderFunc is a RenderOption that is a function.
type renderFunc func(*renderOptions)

// applyRender satisfies RenderOption.
func (f renderFunc) applyRender(o *renderOptions) {
	f(o)
}

// renderOptions are the settings of a render.
type renderOptions struct {
	loader     Loader
	values     map[string][]dataflow.Row
	resultSets map[string]ResultSet
	// mode and scheme are the look of WithMode and WithScheme, and
	// hasMode and hasScheme say that the caller set them, so that they
	// win over the look that the config of the spec asks for (D32).
	mode       theme.Mode
	hasMode    bool
	scheme     theme.Scheme
	hasScheme  bool
	loc        *time.Location
	locale     *format.Locale
	random     func() rand.Source
	width      float64
	height     float64
	logger     *slog.Logger
	icons      map[string]*icon.Pack
	iconLoader IconLoader
	// fallbacks are the fonts of WithEmoji, for the characters that IBM
	// Plex Sans does not have.
	fallbacks []*text.Fallback
	// background is the color of WithBackground as CSS, and hasBackground
	// says that the option set it. Where the caller sets none, the runtime
	// sets them from zuzu.background in the config of the spec (D37).
	background    string
	hasBackground bool
	// scale is the scale of WithScale, and hasScale says that the option
	// set it.
	scale    float64
	hasScale bool
	// look is the look of WithLook, and hasLook says that the option set
	// it. lookFonts are the fonts of WithLookFont, by look (D40).
	look      theme.Look
	hasLook   bool
	lookFonts map[theme.Look]*text.Fallback
	// engines are the engines of WithLayoutEngine, and layout the layout of
	// WithLayout, which hasLayout says the option set (D45).
	engines   []graph.Engine
	layout    string
	hasLayout bool
}

// WithScale draws the image of Image at a scale, as a ratio of device
// pixels to the pixels of the chart, such as 2 for a screen of high
// density. The layout, the fonts and the size of the scene do not change,
// and the image has the size of the scene times the scale. The SVG of SVG
// has a viewBox, so a viewer scales it, and the scale does not change it.
// A scale that is not more than zero, or not finite, is an error of Image.
func WithScale(scale float64) RenderOption {
	return renderFunc(func(o *renderOptions) { o.scale, o.hasScale = scale, true })
}

// WithLoader sets how the data of a url is read (D25).
func WithLoader(l Loader) RenderOption {
	return renderFunc(func(o *renderOptions) { o.loader = l })
}

// WithValues gives the rows of a named dataset. They take the place of the
// values or the url of the dataset in the spec, and its transforms still
// run.
func WithValues(name string, rows []map[string]any) RenderOption {
	return renderFunc(func(o *renderOptions) {
		if o.values == nil {
			o.values = map[string][]dataflow.Row{}
		}
		o.values[name] = rows
	})
}

// WithResultSet gives the rows of a named dataset, read from the result of
// a query. zuzu reads it once, and the caller closes it.
func WithResultSet(name string, rs ResultSet) RenderOption {
	return renderFunc(func(o *renderOptions) {
		if o.resultSets == nil {
			o.resultSets = map[string]ResultSet{}
		}
		o.resultSets[name] = rs
	})
}

// WithMode sets the light or the dark mode (D14). It wins over the mode
// that the spec asks for with zuzu.mode in its config, such as the dark
// theme of a Mermaid diagram (D32).
func WithMode(m theme.Mode) RenderOption {
	return renderFunc(func(o *renderOptions) { o.mode, o.hasMode = m, true })
}

// WithScheme sets the color scheme (D14). It wins over the scheme that the
// spec asks for with zuzu.scheme in its config, such as the forest theme
// of a Mermaid diagram (D32).
func WithScheme(s theme.Scheme) RenderOption {
	return renderFunc(func(o *renderOptions) { o.scheme, o.hasScheme = s, true })
}

// WithLocation sets the time zone of local time (D27). The default is the
// zone of the system.
func WithLocation(loc *time.Location) RenderOption {
	return renderFunc(func(o *renderOptions) { o.loc = loc })
}

// WithLocale sets the locale of numbers and dates (D27).
func WithLocale(l format.Locale) RenderOption {
	return renderFunc(func(o *renderOptions) { o.locale = &l })
}

// WithRandom sets the function that makes the source of random numbers of
// a render (D28). Each render calls it once.
func WithRandom(fn func() rand.Source) RenderOption {
	return renderFunc(func(o *renderOptions) { o.random = fn })
}

// WithBackground sets the background of the chart, over the background of
// the theme and of the spec, and over zuzu.background in the config of the
// spec. A color with an alpha of 0 draws no background, and nil keeps the
// background of the theme and of the spec.
// The boxes under the labels of a diagram and the open arrowheads take an
// opaque color too, so that they match the background.
func WithBackground(c color.Color) RenderOption {
	return renderFunc(func(o *renderOptions) {
		if c == nil {
			o.background, o.hasBackground = "", false
			return
		}
		o.background, o.hasBackground = cssColor(c), true
	})
}

// cssColor writes a color as #rrggbb when it is opaque, as transparent
// when its alpha is 0, and as rgba() between the two.
func cssColor(c color.Color) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA) //nolint:forcetypeassert // The model of NRGBA always returns an NRGBA.
	switch n.A {
	case 0:
		return "transparent"
	case 255:
		return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", n.R, n.G, n.B, strconv.FormatFloat(float64(n.A)/255, 'f', 3, 64))
}

// WithSize sets the size of the chart, in place of its width and height.
func WithSize(width, height float64) RenderOption {
	return renderFunc(func(o *renderOptions) { o.width, o.height = width, height })
}

// WithRenderLogger sets where a render logs.
func WithRenderLogger(l *slog.Logger) RenderOption {
	return renderFunc(func(o *renderOptions) { o.logger = l })
}

// View holds the state of one render. It is not safe for concurrent use.
type View struct {
	rt    *runtime
	scene *scene.Scene
}

// NewView runs the spec and keeps its state.
func (c *Chart) NewView(ctx context.Context, opts ...RenderOption) (*View, Report, error) {
	rt, err := newRuntime(ctx, c, opts)
	if err != nil {
		return nil, nil, err
	}
	s, err := rt.render()
	if err != nil {
		return nil, rt.report, err
	}
	if rt.fitW > 0 || rt.fitH > 0 {
		// The autosize fit renders again with the plot area that leaves
		// room for the guides inside the size of the spec, as Vega does.
		w, err := rt.root.signal("width")
		if err != nil {
			return nil, rt.report, err
		}
		h, err := rt.root.signal("height")
		if err != nil {
			return nil, rt.report, err
		}
		fw, fh := number(w, 0), number(h, 0)
		if rt.fitW > 0 {
			fw = rt.fitW
		}
		if rt.fitH > 0 {
			fh = rt.fitH
		}
		wantW, wantH := rt.wantW, rt.wantH
		// The guides of a smaller plot can take another size, so a render
		// that misses the size corrects the plot again, twice at most. A
		// correction that does not bring the canvas nearer to the size
		// stops, and the render that came nearest stays.
		var best *runtime
		var bestScene *scene.Scene
		bestMiss := math.Inf(1)
		for range 3 {
			again, err := newRuntime(ctx, c, append(slices.Clone(opts), WithSize(fw, fh)))
			if err != nil {
				return nil, nil, err
			}
			got, err := again.render()
			if err != nil {
				return nil, again.report, err
			}
			dw, dh := 0.0, 0.0
			if wantW > 0 {
				dw = wantW - again.fitCanvasW
			}
			if wantH > 0 {
				dh = wantH - again.fitCanvasH
			}
			miss := math.Abs(dw) + math.Abs(dh)
			if miss >= bestMiss {
				break
			}
			best, bestScene, bestMiss = again, got, miss
			if (math.Abs(dw) < 1 && math.Abs(dh) < 1) || (wantW > 0 && fw+dw <= 0) || (wantH > 0 && fh+dh <= 0) {
				break
			}
			fw, fh = fw+dw, fh+dh
		}
		rt, s = best, bestScene
	}
	rt.sketch(s)
	return &View{rt: rt, scene: s}, rt.report, nil
}

// Scene returns the scene graph of the view.
func (v *View) Scene() *scene.Scene {
	return v.scene
}

// Data returns the rows of a dataset of the top level, after its
// transforms.
func (v *View) Data(name string) ([]map[string]any, error) {
	return v.rt.root.dataset(name)
}

// Signal returns the value of a signal of the top level.
func (v *View) Signal(name string) (any, error) {
	return v.rt.root.signal(name)
}

// Scale returns a scale of the top level.
func (v *View) Scale(name string) (scale.Scale, error) {
	return v.rt.root.scale(name)
}

// Render runs the spec and returns the scene graph.
func (c *Chart) Render(ctx context.Context, opts ...RenderOption) (*scene.Scene, Report, error) {
	v, report, err := c.NewView(ctx, opts...)
	if err != nil {
		return nil, report, err
	}
	return v.scene, report, nil
}

// SVG renders the chart and writes it as SVG. In a look drawn by hand, the
// SVG embeds the font of the look, such as Excalifont, so that a viewer
// that lacks the font draws the text in it (D43).
func (c *Chart) SVG(ctx context.Context, w io.Writer, opts ...RenderOption) (Report, error) {
	v, report, err := c.NewView(ctx, opts...)
	if err != nil {
		return report, err
	}
	var b bytes.Buffer
	if err := svg.Write(&b, v.scene, svg.WithFont(v.rt.lookFont)); err != nil {
		return report, err
	}
	_, err = w.Write(b.Bytes())
	return report, err
}

// Image renders the chart and draws it with the direct renderer of okage
// (D11). The text draws in the bundled IBM Plex Sans (D16), the fonts of
// WithEmoji and the font of a look drawn by hand of WithLookFont, and in
// no font of the system, so that a chart gives the same pixels on every
// machine. A character that no font has draws nothing (D32). The pixels
// are premultiplied, as image.RGBA expects.
func (c *Chart) Image(ctx context.Context, opts ...RenderOption) (*image.RGBA, Report, error) {
	v, report, err := c.NewView(ctx, opts...)
	if err != nil {
		return nil, report, err
	}
	s := v.scene
	var o renderOptions
	for _, opt := range opts {
		opt.applyRender(&o)
	}
	ropts := []raster.Option{}
	if o.hasScale {
		ropts = append(ropts, raster.WithScale(o.scale))
	}
	fopts := []okagesvg.Option{
		okagesvg.WithLoadSystemFonts(false), okagesvg.WithFontFS(text.Fonts()),
		okagesvg.WithSansSerifFamily(text.Family), okagesvg.WithFontFamily(text.Family),
	}
	// okage logs to the logger of the render, or else of the parse.
	logger := o.logger
	if logger == nil {
		logger = c.opts.logger
	}
	if logger != nil {
		fopts = append(fopts, okagesvg.WithLogger(logger))
	}
	fallbacks := c.fallbacks(&o)
	if v.rt.lookFont != nil {
		// The font of the look draws the text whose family names it.
		fallbacks = append(slices.Clone(fallbacks), v.rt.lookFont)
	}
	for _, f := range fallbacks {
		fopts = append(fopts, okagesvg.WithFontFile(f.Data()))
	}
	ropts = append(ropts, raster.WithRenderer(okagesvg.New(fopts...)), raster.WithFallbacks(fallbacks...))
	img, err := raster.Draw(s, ropts...)
	if err != nil {
		return nil, report, fmt.Errorf("drawing the image: %w", err)
	}
	return img, report, nil
}
