package zuzu

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// WithLayoutEngine makes a layout engine available to zuzu, under its name
// (D45 part 3). It does not force the engine on a graph. A graph lays out
// with the engine when WithLayout or the spec names it, or when the source
// asks for its layout. The module github.com/xo/zuzu/elk gives the engine
// of ELK, named elk:
//
//	c, report, err := zuzu.Parse(src, zuzu.WithLayoutEngine(elk.Layout()))
//
// A diagram of Mermaid that sets no layout, in a kind whose default layout
// in Mermaid 12.3 is ELK, then draws with ELK, and so does a diagram with
// layout: elk or a layout whose name starts with elk., such as elk.stress.
// Without the engine of ELK, such a diagram draws with zuzu.layered, with no
// issue (D32). The built in engines, graph.Layered and graph.Dagre, need no
// option.
//
// It is an option of Parse too, as WithLook is. The chart keeps the
// engines of Parse, and a render can make more available. When the render
// and the parse each give an engine with the same name, the engine of the
// render wins. A nil engine makes nothing available.
func WithLayoutEngine(e graph.Engine) SharedOption {
	return layoutEngineOption{e: e}
}

// layoutEngineOption is the option of WithLayoutEngine.
type layoutEngineOption struct {
	e graph.Engine
}

// applyParse satisfies Option.
func (l layoutEngineOption) applyParse(o *options) {
	if l.e != nil {
		o.engines = append(o.engines, l.e)
	}
}

// applyRender satisfies RenderOption.
func (l layoutEngineOption) applyRender(o *renderOptions) {
	if l.e != nil {
		o.engines = append(o.engines, l.e)
	}
}

// WithLayout forces a layout on each graph of the chart, by its name (D45
// part 4): layered for zuzu.layered, dagre for zuzu.dagre, elk for the
// layered layout of ELK, or the name of another algorithm of ELK that
// Mermaid 12.3 has, such as elk.stress, elk.force, elk.mrtree,
// elk.sporeOverlap, elk.box or elk.rectpacking. A name of ELK needs the
// engine named elk of WithLayoutEngine, and another name that is not built
// in needs the engine of WithLayoutEngine with that name. The engine gets
// the name as graph.LayoutInput.Algorithm.
//
// The option of the caller wins over the layout that the source asks for,
// as WithLook does. A forced layout with no engine is an issue, and an
// error in the strict mode, and the graph then lays out as though nothing
// forced it. The issue comes from the first graph that lays out, so a chart
// with no graph reports nothing.
//
// It is an option of Parse too. The chart keeps the layout of Parse, and a
// render that forces no layout of its own lays out with it. The layout of
// the render wins over the layout of Parse, and an empty name in a render
// forces no layout, not even the layout of Parse.
func WithLayout(name string) SharedOption {
	return layoutOption{name: name}
}

// layoutOption is the option of WithLayout.
type layoutOption struct {
	name string
}

// applyParse satisfies Option.
func (l layoutOption) applyParse(o *options) {
	o.layout, o.hasLayout = l.name, true
}

// applyRender satisfies RenderOption.
func (l layoutOption) applyRender(o *renderOptions) {
	o.layout, o.hasLayout = l.name, true
}

// engineName returns the name of the engine that lays out a layout: elk
// for elk and each name that starts with elk., and the name itself for any
// other layout.
func engineName(layout string) string {
	if layout == graph.EngineELK || strings.HasPrefix(layout, graph.EngineELK+".") {
		return graph.EngineELK
	}
	return layout
}

// findEngine returns the engine of a layout: the engine of WithLayoutEngine
// of the render with its name, else that of Parse, else the built in engine
// of the name, else nil.
func (rt *runtime) findEngine(layout string) graph.Engine {
	name := engineName(layout)
	for _, list := range [][]graph.Engine{rt.opts.engines, rt.chart.opts.engines} {
		for _, e := range slices.Backward(list) {
			if e.Name() == name {
				return e
			}
		}
	}
	return graph.BuiltinEngine(name)
}

// layoutEngine picks the layout engine of the graphs of the render and the
// name of their layout (D45):
//
//  1. The layout that WithLayout of the render forces, else that of Parse.
//     A forced layout with no engine is an issue, and the next rules pick.
//  2. The layout that the spec names with zuzu.layout in its config, which
//     a diagram of Mermaid fills. A layout of ELK with no engine of ELK
//     lays out with the engine of the transform of the spec with no issue,
//     as layout: elk of Mermaid does without the module of ELK (D32). Any
//     other layout with no engine is an issue.
//  3. The engine of the transform of the spec, zuzu.layered or zuzu.dagre.
//
// It keeps the issue for the first graph that lays out, so that a chart
// with no graph reports nothing.
func (rt *runtime) layoutEngine() {
	forced, hasForced := rt.chart.opts.layout, rt.chart.opts.hasLayout
	if rt.opts.hasLayout {
		forced, hasForced = rt.opts.layout, true
	}
	if hasForced && forced != "" {
		if e := rt.findEngine(forced); e != nil {
			rt.engine, rt.algorithm = e, forced
			return
		}
		rt.layoutIssue = &vega.Issue{
			Err:     vega.ErrUnsupported,
			Message: fmt.Sprintf("the layout %q of zuzu.WithLayout, which no engine of the render lays out, drawn as though no layout were forced", forced),
		}
	}
	raw, ok := rt.chart.spec.Config[graph.ConfigLayout]
	if !ok {
		return
	}
	issue := func(format string, args ...any) {
		if rt.layoutIssue == nil {
			rt.layoutIssue = &vega.Issue{Err: vega.ErrUnsupported, Path: "config." + graph.ConfigLayout, Message: fmt.Sprintf(format, args...)}
		}
	}
	var layout string
	if err := json.Unmarshal(raw, &layout); err != nil {
		issue("the layout %s", raw)
		return
	}
	rt.algorithm = layout
	if rt.engine = rt.findEngine(layout); rt.engine != nil || engineName(layout) == graph.EngineELK {
		return
	}
	issue("the layout %q, which zuzu has no engine for, drawn with the transform of the spec", layout)
}

// Layout returns the layout engine of the render, or nil for the engine of
// the transform, and the name of the layout, for the transforms
// zuzu.layered and zuzu.dagre. The first call reports the issue of a
// layout with no engine, which is an error in the strict mode.
func (e *dataEnv) Layout() (graph.Engine, string, error) {
	rt := e.s.rt
	if is := rt.layoutIssue; is != nil {
		rt.layoutIssue = nil
		if err := rt.issue(is.Err, is.Path, "%s", is.Message); err != nil {
			return nil, "", err
		}
	}
	return rt.engine, rt.algorithm, nil
}
