package mermaid

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
)

// fontFace is the weight and the style that the name of a font family
// gives, such as 600 and italic for Helvetica-BoldOblique. zuzu draws all
// text in IBM Plex Sans, so the rest of the name changes nothing (D32). An
// empty weight or style is one that the name does not give.
type fontFace struct {
	weight string
	style  string
}

// faceOf returns the weight and the style of the name of a font family.
func faceOf(family string) fontFace {
	weight, style := graph.FontFace(family)
	return fontFace{weight: weight, style: style}
}

// face returns the weight and the style of fontFamily at the top of the
// config, which every kind gives to its text (D58). A sequence diagram
// reads fontFamily with the families of its section, in newSeqConfig, so
// its face here is empty.
func (d *Diagram) face() fontFace {
	if d.Kind == "sequenceDiagram" {
		return fontFace{}
	}
	return faceOf(str(d.Config, "fontFamily"))
}

// empty reports whether the face gives neither a weight nor a style.
func (f fontFace) empty() bool {
	return f.weight == "" && f.style == ""
}

// font returns a font of a chart in the face. Text at the normal weight
// takes the weight of the face, and text that a kind draws at a weight of
// its own, such as a bold heading, keeps it. An italic face makes all text
// italic.
func (f fontFace) font(t text.Font) text.Font {
	if f.weight != "" && (t.Weight == 0 || t.Weight == 400) {
		t.Weight = text.Weight(f.weight)
	}
	if f.style != "" {
		t.Italic = true
	}
	return t
}

// faceMeasurer measures text in the face of a diagram, as font says, so
// that a kind that lays out when it compiles measures each label as the
// spec draws it.
type faceMeasurer struct {
	*text.Estimate

	face fontFace
}

// Width satisfies the interface text.Measurer.
func (m faceMeasurer) Width(f text.Font, s string) float64 {
	return m.Estimate.Width(m.face.font(f), s)
}

// apply gives the face to all the text of the spec of a chart, of Vega or
// of Vega-Lite, by the rule of font. The config of the spec gives the face
// to each text mark that sets no weight or no style, and to the labels and
// the titles of the axes, the legends and the title. Then each text mark
// that sets a weight or a style in its encoding or in its mark takes the
// face there, as props says. A setting that the kind puts in the config of
// its spec wins over the face.
func (f fontFace) apply(spec map[string]any) {
	if f.empty() {
		return
	}
	cfg, _ := spec["config"].(map[string]any)
	if cfg == nil {
		cfg = map[string]any{}
		spec["config"] = cfg
	}
	set := func(part string, props map[string]any) {
		if len(props) == 0 {
			return
		}
		m, _ := cfg[part].(map[string]any)
		if m == nil {
			m = map[string]any{}
			cfg[part] = m
		}
		for k, v := range props {
			if _, ok := m[k]; !ok {
				m[k] = v
			}
		}
	}
	// The title of a chart is bold in the look of zuzu, so it takes only
	// the style.
	marks, guide, title := map[string]any{}, map[string]any{}, map[string]any{}
	if f.weight != "" {
		w := faceWeight(f.weight)
		marks["fontWeight"] = w
		guide["labelFontWeight"], guide["titleFontWeight"] = w, w
		title["subtitleFontWeight"] = w
	}
	if f.style != "" {
		marks["fontStyle"] = f.style
		guide["labelFontStyle"], guide["titleFontStyle"] = f.style, f.style
		title["fontStyle"], title["subtitleFontStyle"] = f.style, f.style
	}
	set("text", marks)
	set("axis", guide)
	set("legend", guide)
	set("title", title)
	for k, v := range spec {
		if k != "config" {
			f.walk(v)
		}
	}
}

// faceWeight returns a weight of a face as a number, as a spec writes it.
func faceWeight(w string) any {
	if n, err := strconv.Atoi(w); err == nil {
		return n
	}
	return w
}

// walk gives the face to each text mark under a part of a spec: a mark of
// Vega, with an encoding, or a mark of Vega-Lite, with its properties in
// the mark itself. It does not look in the data, the transforms, the
// scales or the signals, whose objects are not marks.
func (f fontFace) walk(v any) {
	switch v := v.(type) {
	case []any:
		for _, x := range v {
			f.walk(x)
		}
	case map[string]any:
		if v["type"] == "text" {
			if enc, ok := v["encode"].(map[string]any); ok {
				for _, props := range enc {
					if props, ok := props.(map[string]any); ok {
						f.props(props)
					}
				}
			} else {
				f.props(v)
			}
		}
		for k, x := range v {
			switch k {
			case "data", "datasets", "values", "transform", "scales", "signals", "projections":
			default:
				f.walk(x)
			}
		}
	}
}

// props gives the face to the properties of a text mark: the properties of
// a set of its encoding, such as update, or of a mark of Vega-Lite. A kind
// writes the normal weight as the number 400, and a weight or a style that
// a style of the source gives, such as font-weight:normal in a block
// diagram, as a string. So the number 400 and a field with no value in a
// datum take the face, and a string stays, as a style of a node wins over
// fontFamily in a graph.
func (f fontFace) props(props map[string]any) {
	if v, ok := props["fontWeight"]; ok && f.weight != "" {
		props["fontWeight"] = faceRef(v, faceWeight(f.weight), true)
	}
	if v, ok := props["fontStyle"]; ok && f.style != "" {
		props["fontStyle"] = faceRef(v, f.style, false)
	}
}

// faceRef returns a weight, with weight true, or a style of a text mark in
// the face, to: a value itself, as a mark of Vega-Lite gives it, a value or
// a field of an encoding of Vega, or a list of rules of an encoding. A
// weight that is the number 400 takes the face. A field takes it when its
// datum has no value, or a weight of the number 400. A field of a nested
// object and a signal stay as they are.
func faceRef(v, to any, weight bool) any {
	switch r := v.(type) {
	case []any:
		for i, x := range r {
			r[i] = faceRef(x, to, weight)
		}
		return r
	case map[string]any:
		if x, ok := r["value"]; ok {
			if weight && isNormal(x) {
				r["value"] = to
			}
			return r
		}
		name, ok := r["field"].(string)
		if !ok || len(r) != 1 || strings.ContainsAny(name, `.[]\`) {
			return r
		}
		d := "datum[" + strconv.Quote(name) + "]"
		test := d + " == null"
		if weight {
			test += " || " + d + " === 400"
		}
		lit := fmt.Sprint(to)
		if s, ok := to.(string); ok {
			lit = strconv.Quote(s)
		}
		return map[string]any{"signal": test + " ? " + lit + " : " + d}
	}
	if weight && isNormal(v) {
		return to
	}
	return v
}

// isNormal reports whether a weight of a spec is the number 400, the
// normal weight as a kind writes it.
func isNormal(v any) bool {
	switch v := v.(type) {
	case float64:
		return v == 400
	case int:
		return v == 400
	}
	return false
}
