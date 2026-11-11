package mermaid

import (
	"maps"
	"strings"

	"github.com/xo/zuzu/graph"
)

// lookSetting reports a setting of the config that a graph draws in its
// own way: the layout, the look and the theme. It returns false for any
// other setting.
func lookSetting(is *graph.Issues, k string, v any) (bool, error) {
	switch k {
	case "layout":
		switch v {
		case "dagre":
			return true, nil
		case "elk":
			// ELK routes edges in its own way, so the layered layout
			// stands in for it (D31).
			return true, is.Add("config", "the layout elk, drawn with zuzu.layered")
		}
		return true, is.Add("config", "the layout %v", v)
	case "look":
		if v == "handDrawn" {
			return true, is.Add("config", "the look handDrawn")
		}
		return true, nil
	case "theme", "themeVariables", "themeCSS":
		return true, is.Add("config", "the %s of Mermaid, drawn in the look of zuzu", k)
	}
	return false, nil
}

// css reads a style of CSS of a class or a node, as fill:#f9f,stroke:#333,
// into the names of Vega. add records a property that zuzu does not draw.
func css(s string, add func(format string, args ...any) error) (map[string]string, error) {
	st := map[string]string{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		v = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(v, "!important")), "px")
		switch k {
		case "fill":
			st["fill"], st["fillOpacity"] = v, "1"
		case "stroke":
			st["stroke"] = v
		case "stroke-width":
			st["strokeWidth"] = v
		case "color":
			st["color"] = v
		case "stroke-dasharray":
			st["strokeDash"] = strings.Join(strings.Fields(strings.ReplaceAll(v, ",", " ")), ",")
		case "font-weight":
			st["fontWeight"] = v
		case "font-size":
			st["fontSize"] = v
		default:
			if err := add("the style %s", k); err != nil {
				return nil, err
			}
		}
	}
	return st, nil
}

// classStyle returns the style of a node or a cluster: the class default,
// then each of its classes in order, then each style of own in order. A
// later style wins over an earlier one.
func classStyle(classes map[string]map[string]string, names []string, own ...map[string]string) map[string]string {
	st := maps.Clone(classes["default"])
	if st == nil {
		st = map[string]string{}
	}
	for _, c := range names {
		maps.Copy(st, classes[c])
	}
	for _, o := range own {
		maps.Copy(st, o)
	}
	return st
}
