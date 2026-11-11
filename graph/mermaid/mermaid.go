// Package mermaid reads diagrams of Mermaid 12.3.0 (D31): the flowchart, the
// state diagram and the class diagram into a graph, and the charts into
// specs of Vega-Lite and Vega. It reads the subset that docs/DIAGRAMS.md
// lists, and reports the rest.
package mermaid

import (
	"encoding/json/v2"
	"fmt"
	"regexp"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// Error is an error of the package mermaid.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package.
const (
	// ErrSyntax is the error of a source that does not read.
	ErrSyntax Error = "invalid Mermaid"
	// ErrKind is the error of a kind of diagram that zuzu does not draw.
	ErrKind Error = "unsupported kind of Mermaid diagram"
)

// Diagram is a source of Mermaid read up to its kind: its front matter and
// its directives as config, its kind, and its body.
type Diagram struct {
	// Kind is the first word, such as flowchart or pie.
	Kind string
	// Header is the rest of the first line, such as LR.
	Header string
	// Title is the title of the front matter.
	Title string
	// Config is the config of the front matter and of the directives.
	Config map[string]any
	// Lines are the lines of the body, without comments.
	Lines []Line
}

// Line is a line of a body, with its number in the source and its indent:
// the number of spaces before its text, with a tab as 4.
type Line struct {
	Text   string
	N      int
	Indent int
}

// directive matches a directive, %%{init: {...}}%%.
var directive = regexp.MustCompile(`(?s)%%\{\s*(init|initialize)\s*:\s*(.*?)\}%%`)

// Read reads a source up to its kind.
func Read(src string) (*Diagram, error) {
	d := &Diagram{Config: map[string]any{}}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.TrimSpace(lines[i]) == "---" {
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) != "---" {
			j++
		}
		if j >= len(lines) {
			return nil, fmt.Errorf("front matter with no end: %w", ErrSyntax)
		}
		fm := yamlMap(lines[i+1 : j])
		if t, ok := fm["title"].(string); ok {
			d.Title = t
		}
		if c, ok := fm["config"].(map[string]any); ok {
			merge(d.Config, c)
		}
		for k, v := range fm {
			if k != "title" && k != "config" {
				d.Config[k] = v
			}
		}
		i = j + 1
	}
	body := strings.Join(lines[i:], "\n")
	for _, m := range directive.FindAllStringSubmatch(body, -1) {
		var v map[string]any
		if json.Unmarshal([]byte(relaxedJSON(m[2])), &v) == nil {
			merge(d.Config, v)
		}
	}
	body = directive.ReplaceAllStringFunc(body, func(s string) string {
		return strings.Repeat("\n", strings.Count(s, "\n"))
	})
	n0 := i
	for k, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		if d.Kind == "" {
			f := strings.Fields(t)
			d.Kind = strings.TrimSuffix(f[0], ";")
			d.Header = strings.TrimSpace(strings.TrimPrefix(t, f[0]))
			continue
		}
		// A comment runs to the end of the line.
		if c := strings.Index(t, "%%"); c >= 0 && !inQuotes(t, c) {
			t = strings.TrimSpace(t[:c])
		}
		indent := 0
		for _, r := range l {
			if r != ' ' && r != '\t' {
				break
			}
			indent++
			if r == '\t' {
				indent += 3
			}
		}
		d.Lines = append(d.Lines, Line{Text: t, N: n0 + k + 1, Indent: indent})
	}
	if d.Kind == "" {
		return nil, fmt.Errorf("no kind of diagram: %w", ErrSyntax)
	}
	return d, nil
}

// inQuotes reports whether a place of a line is inside double quotes.
func inQuotes(s string, at int) bool {
	return strings.Count(s[:at], `"`)%2 == 1
}

// merge copies the keys of one map into another, the objects key by key.
func merge(dst, src map[string]any) {
	for k, v := range src {
		if m, ok := v.(map[string]any); ok {
			if d, ok := dst[k].(map[string]any); ok {
				merge(d, m)
				continue
			}
		}
		dst[k] = v
	}
}

// relaxedJSON turns the JavaScript of a directive into JSON: single quotes
// become double, and the keys get quotes.
func relaxedJSON(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr != 0:
			if c == inStr {
				b.WriteByte('"')
				inStr = 0
				continue
			}
			if c == '"' {
				b.WriteString(`\"`)
				continue
			}
			b.WriteByte(c)
		case c == '\'' || c == '"':
			inStr = c
			b.WriteByte('"')
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i
			for j < len(s) && (s[j] == '_' || s[j] == '-' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			w := s[i:j]
			k := j
			for k < len(s) && s[k] == ' ' {
				k++
			}
			if k < len(s) && s[k] == ':' {
				b.WriteString(`"` + w + `"`)
			} else {
				switch w {
				case "true", "false", "null":
					b.WriteString(w)
				default:
					b.WriteString(`"` + w + `"`)
				}
			}
			i = j - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// yamlMap reads the subset of YAML of a front matter: maps by indent, with
// scalar values.
func yamlMap(lines []string) map[string]any {
	type level struct {
		indent int
		m      map[string]any
	}
	root := map[string]any{}
	stack := []level{{-1, root}}
	for _, l := range lines {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		k, v, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			continue
		}
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		cur := stack[len(stack)-1].m
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if v == "" {
			m := map[string]any{}
			cur[k] = m
			stack = append(stack, level{indent, m})
			continue
		}
		cur[k] = scalar(v)
	}
	return root
}

// scalar reads a scalar of YAML: a quoted string, a number, a boolean, or
// a plain string.
func scalar(v string) any {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		return v[1 : len(v)-1]
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	}
	var f float64
	if _, err := fmt.Sscan(v, &f); err == nil && !strings.ContainsAny(v, " ") {
		return f
	}
	return v
}

// str returns a string of a config by a path of keys.
func str(cfg map[string]any, keys ...string) string {
	var v any = cfg
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[k]
	}
	s, _ := v.(string)
	return s
}

// Graph reads a flowchart, a state diagram or a class diagram into a
// graph. The report
// names each part that zuzu does not draw, and in the strict mode such a
// part is an error.
func Graph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	switch d.Kind {
	case "flowchart", "graph", "flowchart-elk":
	case "stateDiagram", "stateDiagram-v2":
		return stateGraph(d, strict)
	case "classDiagram", "classDiagram-v2":
		return classGraph(d, strict)
	default:
		return nil, nil, fmt.Errorf("%s: %w", d.Kind, ErrKind)
	}
	f := newFlowchart(d, strict)
	if err := f.config(); err != nil {
		return nil, f.issues.Report, err
	}
	if err := f.parse(); err != nil {
		return nil, f.issues.Report, err
	}
	return f.g, f.issues.Report, nil
}
