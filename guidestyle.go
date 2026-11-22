package zuzu

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"strings"

	"github.com/xo/zuzu/scale"
	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/vega"
)

// guidePart is a part of a guide that the config of Vega styles: the prefix
// of its properties, such as label in labelColor, the role of its items, and
// whether it is text or a line. The symbols of a legend are neither, and
// take symbolProps.
type guidePart struct {
	prefix string
	role   string
	text   bool
}

// The parts of each kind of guide, the longest prefix first.
var (
	axisParts = []guidePart{
		{"domain", "axis-domain", false}, {"domain", "axis-zero", false},
		{"label", "axis-label", true}, {"title", "axis-title", true}, {"grid", "axis-grid", false},
		{"tick", "axis-tick", false},
	}
	legendStyleParts = []guidePart{
		{"label", "legend-label", true}, {"title", "legend-title", true}, {"symbol", "legend-symbol", false},
	}
	titleParts = []guidePart{
		{"subtitle", "title-subtitle", true}, {"", "title-text", true},
	}
)

// opacityEnd is the end of a property of the opacity of a part, such as
// symbolOpacity.
const opacityEnd = "Opacity"

// textProps and lineProps map the end of a property of the config, such as
// Color in labelColor, to the property of an item.
var (
	textProps = map[string]string{
		"Color": "fill", "Font": "font", "FontSize": "fontSize", "FontWeight": "fontWeight",
		"FontStyle": "fontStyle", "Opacity": "opacity",
	}
	lineProps = map[string]string{
		"Color": "stroke", "Width": "strokeWidth", "Dash": "strokeDash", "Opacity": "strokeOpacity",
		"Cap": "strokeCap",
	}
	// symbolProps maps the end of a property of the symbols of a legend,
	// such as FillColor in symbolFillColor.
	symbolProps = map[string]string{
		"FillColor": "fill", "StrokeColor": "stroke", "StrokeWidth": "strokeWidth", opacityEnd: "opacity",
		"Dash": "strokeDash",
	}
)

// configObject returns an object of the config of the render, such as axis
// or legend, or nil.
func (rt *runtime) configObject(name string) map[string]any {
	m, _ := rt.config[name].(map[string]any)
	return m
}

// axisConfig returns the config of an axis: axis, then axisX or axisY, then
// the axis of its orient, such as axisLeft, then axisBand for a band scale,
// each over the one before, as Vega reads them.
func (rt *runtime) axisConfig(orient string, sc scale.Scale) map[string]any {
	out := map[string]any{}
	names := []string{"axis", "axisX"}
	if orient == "left" || orient == "right" {
		names[1] = "axisY"
	}
	if orient != "" {
		names = append(names, "axis"+strings.ToUpper(orient[:1])+orient[1:])
	}
	if sc != nil && sc.Type() == "band" {
		names = append(names, "axisBand")
	}
	for _, n := range names {
		maps.Copy(out, rt.configObject(n))
	}
	return out
}

// datumSignal is a property of a guide whose signal reads the datum of an
// item, such as a labelOpacity of "datum.value ? 1 : 0". styleGuide
// evaluates it for each item of its part, with the datum of the item, as
// Vega does.
type datumSignal string

// guideProps returns the properties of a guide that style its parts: the
// config, with the properties that the spec of the guide gives over it. A
// signal that reads the datum stays a datumSignal.
func (s *scope) guideProps(config map[string]any, extra vega.Extra) map[string]any {
	out := map[string]any{}
	maps.Copy(out, config)
	for k, raw := range extra {
		var v any
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		if m, ok := v.(map[string]any); ok && len(m) == 1 {
			// A property of a guide can be a signal, or a value as an
			// encoding gives it.
			if sig, ok := m["signal"].(string); ok {
				if strings.Contains(sig, "datum") {
					out[k] = datumSignal(sig)
					continue
				}
				x, err := s.eval(sig, nil)
				if err != nil {
					continue
				}
				v = x
			} else if x, ok := m["value"]; ok {
				v = s.rt.specConfig(k, x)
			}
		} else {
			// A color that the guide names flips in the dark mode, as a
			// color of an encoding does (D48 part 1).
			v = s.rt.specConfig(k, v)
		}
		out[k] = v
	}
	return out
}

// styleGuide applies the properties of the config of a guide to its parts:
// labelColor to the fill of each label, gridDash to the dash of each grid
// line, and so on. A datumSignal takes the datum of each item.
func (s *scope) styleGuide(m *scene.Mark, props map[string]any, parts []guidePart) {
	if len(props) == 0 {
		return
	}
	// The properties of each role.
	byRole := map[string]map[string]any{}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		for _, p := range parts {
			rest, ok := strings.CutPrefix(k, p.prefix)
			if !ok || (p.prefix == "" && strings.ToUpper(k[:1]) == k[:1]) {
				continue
			}
			if p.prefix == "" {
				// The title itself names its properties without a prefix.
				rest = strings.ToUpper(k[:1]) + k[1:]
			}
			table := lineProps
			switch {
			case p.role == "legend-symbol":
				table = symbolProps
			case p.text:
				table = textProps
			}
			prop, ok := table[rest]
			if !ok {
				continue
			}
			if byRole[p.role] == nil {
				byRole[p.role] = map[string]any{}
			}
			byRole[p.role][prop] = props[k]
		}
	}
	var walk func(m *scene.Mark)
	walk = func(m *scene.Mark) {
		for i := range m.Items {
			it := &m.Items[i]
			if ps := byRole[m.Role]; ps != nil {
				st := &itemState{it: it, set: map[string]bool{}}
				for _, k := range slices.Sorted(maps.Keys(ps)) {
					v := ps[k]
					if sig, ok := v.(datumSignal); ok {
						// A signal that does not evaluate leaves the
						// property of the look, as guideProps does.
						x, err := s.eval(string(sig), it.Datum)
						if err != nil {
							continue
						}
						v = x
					}
					setProp(st, k, v)
				}
			}
			for j := range it.Items {
				walk(&it.Items[j])
			}
		}
	}
	walk(m)
}

// namedRange returns the range of the config of the render for a name,
// such as category, and whether the config has it. The range is a list of
// values, or a scheme, such as {"scheme": "tableau10"}.
func (s *scope) namedRange(name string) ([]any, *vega.SchemeRef, bool) {
	v, ok := s.rt.configObject("range")[name]
	if !ok {
		return nil, nil, false
	}
	switch x := v.(type) {
	case []any:
		return x, nil, true
	case map[string]any:
		b, err := json.Marshal(x)
		if err != nil {
			return nil, nil, false
		}
		var ref vega.SchemeRef
		if json.Unmarshal(b, &ref) != nil {
			return nil, nil, false
		}
		return nil, &ref, true
	}
	return nil, nil, false
}
