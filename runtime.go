package zuzu

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand/v2"
	"time"

	"github.com/xo/zuzu/dataflow"
	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/format"
	"github.com/xo/zuzu/geo"
	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/theme"
	"github.com/xo/zuzu/vega"
)

// runtime is the state of one render. It works out each signal, dataset and
// scale when something first asks for it, so that the order of the spec
// does not matter, as in the dataflow of Vega.
type runtime struct {
	// ctx is the context of the render, which the loader takes. The runtime
	// lives only as long as the call that gave it.
	ctx   context.Context //nolint:containedctx // The runtime is one call, which owns the context.
	chart *Chart
	opts  renderOptions
	// hierarchy is the tree that a stratify transform built, which the
	// hierarchy transforms of every dataset read.
	hierarchy dataflow.Hierarchy
	loc       *time.Location
	locale    *format.Locale
	rand      *rand.Rand
	theme     theme.Theme
	// endLabels holds, for each scale of color, the names of the series
	// that the ends of lines name, so that a legend of that scale can be
	// left out.
	endLabels map[string]map[string]bool
	// fitW and fitH are the size of the plot area that the autosize fit
	// asks for, or 0, which NewView renders again with.
	fitW, fitH float64
	// wantW and wantH are the size of the canvas that the autosize fit
	// asks for, or 0.
	wantW, wantH float64
	// config is the config of the theme with the config of the spec over
	// it (D14).
	config map[string]any
	report Report
	progs  map[string]*expr.Program
	root   *scope
	// measure measures the width of text, for the guides and the bounds.
	measure *text.Estimate
	// err is the first error of a signal that an expression read, which
	// the interface of expr.Signals cannot return.
	err error
}

// newRuntime makes the runtime of a render.
func newRuntime(ctx context.Context, c *Chart, opts []RenderOption) *runtime {
	o := renderOptions{scheme: theme.Zuzu}
	for _, opt := range opts {
		opt(&o)
	}
	rt := &runtime{ctx: ctx, chart: c, opts: o, progs: map[string]*expr.Program{}, measure: text.NewEstimate()}
	rt.report = append(rt.report, c.report...)
	rt.loc = o.loc
	if rt.loc == nil {
		rt.loc = time.Local //nolint:gosmopolitan // The zone of the system is the default (D27).
	}
	rt.locale = o.locale
	if rt.locale == nil {
		l := format.EnUS
		rt.locale = &l
	}
	if o.random != nil {
		rt.rand = rand.New(o.random()) //nolint:gosec // A chart asks for random data, not a secret (D28).
	} else {
		rt.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // A chart asks for random data, not a secret (D28).
	}
	rt.theme = theme.New(o.mode, o.scheme)
	rt.config = rt.theme.Config()
	rt.endLabels = map[string]map[string]bool{}
	for k, raw := range c.spec.Config {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			rt.config[k] = mergeConfig(rt.config[k], v)
		}
	}
	rt.root = rt.newScope(nil, &c.spec.Scope, "", nil)
	rt.root.builtins(c.spec)
	return rt
}

// issue handles a feature that zuzu does not support: an error in the
// strict mode, and an issue of the report in the loose mode.
func (rt *runtime) issue(err error, path, format string, args ...any) error {
	is := vega.Issue{Err: err, Path: path, Message: fmt.Sprintf(format, args...)}
	if existing, ok := errors.AsType[vega.Issue](err); ok {
		is = existing
		if is.Path == "" {
			is.Path = path
		}
	}
	if rt.chart.opts.strict {
		return is
	}
	rt.report = append(rt.report, is)
	return nil
}

// compile compiles an expression once.
func (rt *runtime) compile(src string) (*expr.Program, error) {
	if p, ok := rt.progs[src]; ok {
		return p, nil
	}
	p, err := expr.Compile(src)
	if err != nil {
		if errors.Is(err, expr.ErrUnknownFunction) {
			return nil, vega.Issue{Err: vega.ErrUnsupportedFunction, Message: err.Error()}
		}
		return nil, err
	}
	rt.progs[src] = p
	return p, nil
}

// state is how far the work on a signal, a dataset or a scale has gone.
type state uint8

const (
	todo state = iota
	busy
	done
)

// scope is the spec, or one item of a group mark, with the signals, the
// datasets and the scales that it defines.
type scope struct {
	rt      *runtime
	parent  *scope
	spec    *vega.Scope
	path    string
	datum   any
	signals map[string]*signalState
	data    map[string]*dataState
	scales  map[string]*scaleState
	// producers names the dataset whose transform sets each signal, such
	// as the signal of an extent.
	producers map[string]string
	funcs     map[string]expr.Func
	// item is the group item of the scope, which a field of the group
	// reads.
	item *scene.Item
	// markItems holds the items of each named mark, for a mark whose data
	// is another mark.
	markItems map[string][]scene.Item
	// markTypes holds the kind of each named mark.
	markTypes map[string]string
	// projections holds the projections of the scope, which are made when
	// first used.
	projections map[string]*projState
}

type signalState struct {
	def   *vega.Signal
	value any
	state state
	err   error
}

type dataState struct {
	def   *vega.Data
	path  string
	rows  []dataflow.Row
	state state
	err   error
	// facet holds the rows of a facet, which a group mark gives.
	facet []dataflow.Row
}

type scaleState struct {
	def   *vega.Scale
	path  string
	s     scale.Scale
	state state
	err   error
}

// newScope makes the scope of the spec or of a group item.
func (rt *runtime) newScope(parent *scope, spec *vega.Scope, path string, datum any) *scope {
	s := &scope{
		rt: rt, parent: parent, spec: spec, path: path, datum: datum,
		signals:   map[string]*signalState{},
		data:      map[string]*dataState{},
		scales:    map[string]*scaleState{},
		producers: map[string]string{},
		markItems: map[string][]scene.Item{},
		markTypes: map[string]string{},

		projections: map[string]*projState{},
	}
	for i := range spec.Projections {
		d := &spec.Projections[i]
		s.projections[d.Name] = &projState{def: d, path: fmt.Sprintf("%s/projections/%d", path, i)}
	}
	for i := range spec.Signals {
		d := &spec.Signals[i]
		if d.Push == "outer" {
			continue
		}
		s.signals[d.Name] = &signalState{def: d}
	}
	for i := range spec.Data {
		d := &spec.Data[i]
		s.data[d.Name] = &dataState{def: d, path: fmt.Sprintf("%s/data/%d", path, i)}
		for _, t := range d.Transform {
			if v, ok := t.Param("signal"); ok && v.Signal == "" {
				var name string
				if v.Decode(&name) == nil && name != "" {
					s.producers[name] = d.Name
				}
			}
		}
	}
	for i := range spec.Scales {
		d := &spec.Scales[i]
		s.scales[d.Name] = &scaleState{def: d, path: fmt.Sprintf("%s/scales/%d", path, i)}
	}
	s.funcs = s.viewFuncs()
	return s
}

// Signal gives the value of a signal to an expression. It satisfies
// expr.Signals. An error goes to the runtime, since the interface cannot
// return it.
func (s *scope) Signal(name string) (any, bool) {
	v, err := s.signal(name)
	if errors.Is(err, ErrUnknownName) {
		return nil, false
	}
	if err != nil {
		if s.rt.err == nil {
			s.rt.err = err
		}
		return expr.Undefined, true
	}
	return v, true
}

// builtins adds the signals that every spec has: width, height, padding,
// autosize and background, unless the spec defines them.
func (s *scope) builtins(spec *vega.Spec) {
	num := func(name string, v vega.NumberValue, def float64) {
		if _, ok := s.signals[name]; ok {
			return
		}
		d := &vega.Signal{Name: name}
		switch v.Kind {
		case vega.SignalRef:
			d.Update = vega.Expr(v.Signal)
		case vega.Literal:
			d.Value, _ = json.Marshal(v.Number)
		default:
			d.Value, _ = json.Marshal(def)
		}
		s.signals[name] = &signalState{def: d}
	}
	// A size that the caller gives replaces the size of the spec, even one
	// that a signal of the spec works out from a step.
	if o := s.rt.opts; o.width > 0 {
		v, _ := json.Marshal(o.width)
		s.signals["width"] = &signalState{def: &vega.Signal{Name: "width", Value: v}}
	}
	if o := s.rt.opts; o.height > 0 {
		v, _ := json.Marshal(o.height)
		s.signals["height"] = &signalState{def: &vega.Signal{Name: "height", Value: v}}
	}
	num("width", spec.Width, 0)
	num("height", spec.Height, 0)
	val := func(name string, v vega.Value) {
		if _, ok := s.signals[name]; ok {
			return
		}
		d := &vega.Signal{Name: name}
		if v.Signal != "" {
			d.Update = vega.Expr(v.Signal)
		} else {
			d.Value = v.JSON
		}
		s.signals[name] = &signalState{def: d}
	}
	val("padding", spec.Padding)
	val("autosize", spec.Autosize)
	val("background", spec.Background)
}

// signal returns the value of a signal of this scope or of a scope above.
func (s *scope) signal(name string) (any, error) {
	for sc := s; sc != nil; sc = sc.parent {
		if st, ok := sc.signals[name]; ok {
			return sc.signalValue(name, st)
		}
		if ds, ok := sc.producers[name]; ok {
			if err := sc.produce(ds, name); err != nil {
				return nil, err
			}
			if st, ok := sc.signals[name]; ok {
				return st.value, nil
			}
		}
	}
	return nil, fmt.Errorf("the signal %q: %w", name, ErrUnknownName)
}

// signalValue works out the value of a signal: its update, or else its
// init, or else its value.
func (s *scope) signalValue(name string, st *signalState) (any, error) {
	switch st.state {
	case done:
		return st.value, st.err
	case busy:
		return nil, fmt.Errorf("the signal %q: %w", name, ErrCycle)
	}
	st.state = busy
	defer func() { st.state = done }()
	d := st.def
	src := string(d.Update)
	if src == "" {
		src = string(d.Init)
	}
	if src == "" {
		var v any
		if len(d.Value) > 0 {
			if err := json.Unmarshal(d.Value, &v); err != nil {
				st.err = err
				return nil, err
			}
		}
		st.value = v
		return v, nil
	}
	v, err := s.eval(src, nil)
	if err != nil {
		st.err = fmt.Errorf("the signal %q: %w", name, err)
		return nil, st.err
	}
	st.value = v
	return v, nil
}

// setSignal sets a signal that a transform writes.
func (s *scope) setSignal(name string, v any) {
	st, ok := s.signals[name]
	if !ok {
		st = &signalState{def: &vega.Signal{Name: name}}
		s.signals[name] = st
	}
	st.value, st.state, st.err = v, done, nil
}

// exprScope returns the scope that an expression runs in, with a datum.
func (s *scope) exprScope(datum any) *expr.Scope {
	var parent any
	if s.datum != nil {
		parent = s.datum
	}
	return &expr.Scope{
		Datum: datum, Parent: parent, Signals: s, Functions: s.funcs,
		Location: s.rt.loc, Rand: s.rt.rand, Locale: s.rt.locale,
	}
}

// eval evaluates an expression with a datum.
func (s *scope) eval(src string, datum any) (any, error) {
	p, err := s.rt.compile(src)
	if err != nil {
		return nil, err
	}
	s.rt.err = nil
	v, err := p.Eval(s.exprScope(datum))
	if err == nil && s.rt.err != nil {
		err = s.rt.err
	}
	s.rt.err = nil
	return v, err
}

// value evaluates a property that is a literal or a signal.
func (s *scope) value(v vega.Value) (any, error) {
	if v.Signal != "" {
		return s.eval(v.Signal, nil)
	}
	if v.JSON == nil {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(v.JSON, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// findData returns the scope and the state of a dataset that this scope can
// see.
func (s *scope) findData(name string) (*scope, *dataState) {
	for sc := s; sc != nil; sc = sc.parent {
		if ds, ok := sc.data[name]; ok {
			return sc, ds
		}
	}
	return nil, nil
}

// dataset returns the rows of a dataset, after its transforms.
func (s *scope) dataset(name string) ([]dataflow.Row, error) {
	sc, ds := s.findData(name)
	if ds == nil {
		return nil, fmt.Errorf("the dataset %q: %w", name, ErrUnknownName)
	}
	switch ds.state {
	case done:
		return ds.rows, ds.err
	case busy:
		return nil, fmt.Errorf("the dataset %q: %w", name, ErrCycle)
	}
	ds.state = busy
	ds.rows, ds.err = sc.load(ds)
	ds.state = done
	return ds.rows, ds.err
}

// produce works out a signal that a transform of a dataset sets. If the
// dataset is not loaded yet, it runs the transforms only up to that one, as
// the dataflow of Vega does: a later transform of the dataset can need the
// signal, through a projection that fits the data.
func (s *scope) produce(name, signal string) error {
	ds, ok := s.data[name]
	if !ok {
		return nil
	}
	if ds.state != todo {
		_, err := s.dataset(name)
		return err
	}
	end := -1
	for i, t := range ds.def.Transform {
		if v, ok := t.Param("signal"); ok && v.Signal == "" {
			var n string
			if v.Decode(&n) == nil && n == signal {
				end = i + 1
			}
		}
	}
	if end < 0 {
		_, err := s.dataset(name)
		return err
	}
	ds.state = busy
	defer func() { ds.state = todo }()
	_, err := s.loadTo(ds, end)
	return err
}

// load reads the rows of a dataset and runs its transforms.
func (s *scope) load(ds *dataState) ([]dataflow.Row, error) {
	return s.loadTo(ds, len(ds.def.Transform))
}

// loadTo reads the rows of a dataset and runs its first n transforms.
func (s *scope) loadTo(ds *dataState, n int) ([]dataflow.Row, error) {
	d := ds.def
	rows, err := s.source(ds)
	if err != nil {
		if _, ok := errors.AsType[vega.Issue](err); !ok || !errors.Is(err, vega.ErrUnsupported) {
			return nil, fmt.Errorf("%s: %w", ds.path, err)
		}
		// In the loose mode, a dataset that zuzu cannot read is empty.
		if err := s.rt.issue(err, ds.path, ""); err != nil {
			return nil, err
		}
	}
	env := &dataEnv{s: s}
	for i, t := range d.Transform[:n] {
		path := fmt.Sprintf("%s/transform/%d", ds.path, i)
		if !dataflow.Supported(t.Type) {
			if err := s.rt.issue(vega.ErrUnsupportedTransform, path, "%s", t.Type); err != nil {
				return nil, err
			}
			continue
		}
		out, err := dataflow.Run(env, rows, d.Transform[i:i+1], ds.path+"/transform")
		if err != nil {
			if _, ok := errors.AsType[vega.Issue](err); ok {
				if err := s.rt.issue(err, path, ""); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}
		rows = out
	}
	return rows, nil
}

// source returns the rows of a dataset before its transforms: the rows that
// the program gives, a facet, its values, its url or its sources.
func (s *scope) source(ds *dataState) ([]dataflow.Row, error) {
	d := ds.def
	o := s.rt.opts
	if rows, ok := o.values[d.Name]; ok && s.parent == nil {
		return copyRows(rows), nil
	}
	if rs, ok := o.resultSets[d.Name]; ok && s.parent == nil {
		return readResultSet(rs)
	}
	if ds.facet != nil {
		return ds.facet, nil
	}
	f, err := s.format(d.Format)
	if err != nil {
		return nil, err
	}
	switch {
	case !d.Values.IsZero():
		v, err := s.value(d.Values)
		if err != nil {
			return nil, err
		}
		if text, ok := v.(string); ok {
			return dataflow.Read([]byte(text), f, s.rt.loc)
		}
		if f.Type == "topojson" {
			return dataflow.ReadTopoJSON(v, f)
		}
		if f.Property != "" {
			if m, ok := v.(map[string]any); ok {
				v = dataflow.FieldAccessor(f.Property).Get(m)
			}
		}
		rows := dataflow.Ingest(v)
		if err := dataflow.ParseTypes(rows, f, s.rt.loc); err != nil {
			return nil, err
		}
		return rows, nil
	case !d.URL.IsZero():
		v, err := s.value(d.URL)
		if err != nil {
			return nil, err
		}
		url := expr.String(v)
		if o.loader == nil {
			return nil, fmt.Errorf("reading %q: %w", url, ErrNoLoader)
		}
		r, err := o.loader.Load(s.rt.ctx, LoadRequest{URL: url, Format: f.Type, Name: d.Name})
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", url, err)
		}
		if f.Type == "" {
			f.Type = "json"
		}
		return dataflow.Read(b, f, s.rt.loc)
	case !d.Source.IsZero():
		var rows []dataflow.Row
		for _, name := range d.Source.List {
			r, err := s.dataset(name)
			if err != nil {
				return nil, err
			}
			rows = append(rows, r...)
		}
		return rows, nil
	}
	return nil, nil
}

// format resolves the format of a dataset.
func (s *scope) format(f *vega.Format) (dataflow.Format, error) {
	if f == nil {
		return dataflow.Format{}, nil
	}
	out := dataflow.Format{Type: f.Type, Delimiter: f.Delimiter, Header: f.Header, Property: f.Property, Feature: f.Feature, Mesh: f.Mesh, Filter: f.Filter}
	if !f.Parse.IsZero() {
		v, err := s.value(f.Parse)
		if err != nil {
			return out, err
		}
		switch p := v.(type) {
		case string:
			out.Auto = p == "auto"
		case map[string]any:
			out.Parse = map[string]string{}
			for k, t := range p {
				if t != nil {
					out.Parse[k] = expr.String(t)
				}
			}
		}
	}
	return out, nil
}

// copyRows copies rows, so that a transform cannot change the rows of the
// caller.
func copyRows(rows []map[string]any) []dataflow.Row {
	out := make([]dataflow.Row, len(rows))
	for i, r := range rows {
		c := make(dataflow.Row, len(r))
		for k, v := range r {
			c[k] = normalize(v)
		}
		out[i] = c
	}
	return out
}

// normalize converts a Go value to a value of an expression.
func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case float32:
		return float64(x)
	}
	return v
}

// dataEnv is the view that the transforms of a dataset read.
type dataEnv struct {
	s *scope
}

func (e *dataEnv) Compile(src string) (*expr.Program, error) { return e.s.rt.compile(src) }
func (e *dataEnv) Scope(datum any) *expr.Scope               { return e.s.exprScope(datum) }
func (e *dataEnv) Data(name string) ([]dataflow.Row, error)  { return e.s.datasetOrItems(name) }
func (e *dataEnv) SetSignal(name string, v any)              { e.s.setSignal(name, v) }
func (e *dataEnv) Location() *time.Location                  { return e.s.rt.loc }
func (e *dataEnv) Random() float64                           { return e.s.rt.rand.Float64() }
func (e *dataEnv) Hierarchy() *dataflow.Hierarchy            { return &e.s.rt.hierarchy }
func (e *dataEnv) Projection(name string) (*geo.Projection, error) {
	return e.s.projection(name)
}

// TextSize returns the width and the height of a text as the writer draws
// it, for the transform label.
func (e *dataEnv) TextSize(txt any, font string, size float64, weight string) (float64, float64) {
	it := &scene.Item{Font: font, FontSize: size, FontWeight: scene.Weight(weight)}
	switch t := txt.(type) {
	case []any:
		for _, l := range t {
			it.Text = append(it.Text, expr.String(l))
		}
	default:
		it.Text = scene.Lines{expr.String(t)}
	}
	return e.s.rt.style().TextSize(it)
}

// MarkShapes returns the kind of a named mark and its items, each with its
// place and its bounds, for the transform label to avoid.
func (e *dataEnv) MarkShapes(name string) (string, []dataflow.Row, bool) {
	items, ok := e.s.findMarkItems(name)
	if !ok {
		return "", nil, false
	}
	typ := e.s.findMarkType(name)
	out := make([]dataflow.Row, len(items))
	for i := range items {
		out[i] = e.s.itemDatum(typ, &items[i])
	}
	return typ, out, true
}

// number reads a value as a number, with def for null or undefined.
func number(v any, def float64) float64 {
	if v == nil || expr.IsUndefined(v) {
		return def
	}
	f := expr.Number(v)
	if math.IsNaN(f) {
		return def
	}
	return f
}

// rawUnmarshal decodes JSON into a value.
func rawUnmarshal(v jsontext.Value, out any) error {
	return json.Unmarshal(v, out)
}

// rawMarshal encodes a value as JSON.
func rawMarshal(v any) jsontext.Value {
	b, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return jsontext.Value("null")
	}
	return b
}

// datasetOrItems returns the rows of a dataset or, where no dataset has the
// name, the items of a mark with the name, as Vega does for the data that a
// transform reads.
func (s *scope) datasetOrItems(name string) ([]dataflow.Row, error) {
	rows, err := s.dataset(name)
	if err == nil || !errors.Is(err, ErrUnknownName) {
		return rows, err
	}
	items, ok := s.findMarkItems(name)
	if !ok {
		return nil, err
	}
	out := make([]dataflow.Row, len(items))
	for i := range items {
		out[i] = s.itemDatum(s.findMarkType(name), &items[i])
	}
	return out, nil
}

// mergeConfig returns a value of a config with another over it: two objects
// merge key by key, and any other value replaces the first.
func mergeConfig(base, over any) any {
	b, ok1 := base.(map[string]any)
	o, ok2 := over.(map[string]any)
	if !ok1 || !ok2 {
		return over
	}
	out := make(map[string]any, len(b)+len(o))
	maps.Copy(out, b)
	for k, v := range o {
		out[k] = mergeConfig(b[k], v)
	}
	return out
}
