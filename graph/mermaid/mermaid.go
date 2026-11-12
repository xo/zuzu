// Package mermaid reads diagrams of Mermaid 12.3.0 (D31): the flowchart, the
// swimlane diagram, the state diagram, the class diagram, the entity
// relationship diagram, the requirement diagram, the agentflow diagram and
// the use case diagram into a graph, and the
// charts into specs of Vega-Lite and Vega. It reads the subset that
// docs/DIAGRAMS.md lists, and reports the rest.
package mermaid

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
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
	// N is the number of the first line in the source, after the front
	// matter, the directives and the comments.
	N int
	// Title is the title of the front matter.
	Title string
	// Description is the description of the front matter. Mermaid ignores
	// it, and zuzu puts it in the description of the spec (D32).
	Description string
	// Config is the config of the front matter and of the directives.
	Config map[string]any
	// Lines are the lines of the body, without comments.
	Lines []Line

	// unread holds the parts of the front matter that zuzu does not read,
	// which Graph and Chart report.
	unread []unread
}

// unread is a part of the front matter that zuzu does not read: the number
// of its line in the source, and what it is.
type unread struct {
	n    int
	what string
}

// Line is a line of a body, with its number in the source and its indent:
// the number of spaces before its text, with a tab as 4.
type Line struct {
	Text   string
	N      int
	Indent int
}

// directive matches a directive, %%{init: {...}}%%, and wrapDirective the
// directive %%{wrap}%%, which sets wrap.
var (
	directive     = regexp.MustCompile(`(?s)%%\{\s*(init|initialize)\s*:\s*(.*?)\}%%`)
	wrapDirective = regexp.MustCompile(`%%\{\s*wrap\s*\}%%`)
)

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
		fm, lineOf, skipped := yamlMap(lines[i+1:j], i+2)
		d.unread = skipped
		// Mermaid reads title, displayMode and config, and ignores every
		// other key of the front matter. zuzu reads description too (D32).
		for _, k := range slices.Sorted(maps.Keys(fm)) {
			v := fm[k]
			switch k {
			case "title", "description":
				s, ok := yamlText(v)
				switch {
				case !ok:
					d.unread = append(d.unread, unread{lineOf[k], fmt.Sprintf("the %s of the front matter, which is not a text", k)})
				case k == "title":
					d.Title = s
				default:
					d.Description = s
				}
			case "config":
				c, ok := v.(map[string]any)
				if !ok {
					d.unread = append(d.unread, unread{lineOf[k], "the config of the front matter, which is not a map of YAML"})
					continue
				}
				merge(d.Config, c)
			case "displayMode":
				d.Config[k] = v
			default:
				if hint(k, v) {
					// Mermaid ignores look and layout here too, and a
					// hint is no issue (D32).
					continue
				}
				d.unread = append(d.unread, unread{lineOf[k], fmt.Sprintf("the key %s of the front matter, which Mermaid ignores", k)})
			}
		}
		slices.SortStableFunc(d.unread, func(a, b unread) int { return a.n - b.n })
		i = j + 1
	}
	body := strings.Join(lines[i:], "\n")
	inits := map[string]any{}
	for _, m := range directive.FindAllStringSubmatch(body, -1) {
		var v map[string]any
		if json.Unmarshal([]byte(relaxedJSON(m[2])), &v) == nil {
			merge(inits, v)
		}
	}
	body = directive.ReplaceAllStringFunc(body, func(s string) string {
		return strings.Repeat("\n", strings.Count(s, "\n"))
	})
	// %%{wrap}%% sets wrap, over the wrap of an init directive, as in
	// Mermaid.
	if wrapDirective.MatchString(body) {
		inits["wrap"] = true
		body = wrapDirective.ReplaceAllString(body, "")
	}
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
			// A git graph can start with gitGraph and a colon.
			if d.Kind == "gitGraph:" {
				d.Kind, d.Header = "gitGraph", strings.TrimSpace(":"+d.Header)
			}
			d.N = n0 + k + 1
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
	// Mermaid moves the key config of the directives to the section of
	// the kind, so %%{init: {'config': {'wrap': true}}}%% in a sequence
	// diagram sets sequence.wrap.
	if c, ok := inits["config"]; ok {
		delete(inits, "config")
		inits[configSection(d.Kind)] = c
	}
	merge(d.Config, inits)
	return d, nil
}

// yamlText returns a scalar of YAML as text: a string, or a number or a
// boolean as YAML writes it. A map is not text.
func yamlText(v any) (string, bool) {
	switch v := v.(type) {
	case string:
		return v, true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	}
	return "", false
}

// describe returns the description of the spec of a diagram: the
// description of the front matter first, then the text of each accTitle
// and accDescr in the order of the source, joined by spaces. An empty text
// adds nothing.
func describe(d *Diagram, acc ...string) string {
	parts := make([]string, 0, len(acc)+1)
	for _, s := range append([]string{d.Description}, acc...) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// configSection returns the key that Mermaid gives to the key config of a
// directive in a kind of diagram: the id of the kind in Mermaid, such as
// sequence for sequenceDiagram. It is the section of the config of most
// kinds, but not of all: the section of classDiagram is class, and so the
// key config of a class diagram sets nothing in Mermaid.
func configSection(kind string) string {
	switch kind {
	case "graph":
		return "flowchart"
	case "sequenceDiagram":
		return "sequence"
	case "erDiagram":
		return "er"
	case "requirementDiagram":
		return "requirement"
	case "classDiagram-v2":
		return "classDiagram"
	case "stateDiagram-v2":
		return "stateDiagram"
	case c4Context, c4Container, c4Component, c4Dynamic, c4Deployment:
		return "c4"
	}
	return strings.TrimSuffix(kind, "-beta")
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
// scalar values. A scalar can be a block, after | or >, or a plain or
// quoted text that goes on in the lines indented under its key. A map in
// braces, as {a: 1}, reads as JSON does. first is the number of the first
// line in the source. yamlMap returns the map, the number of the line of
// each key of the top level, and each part of YAML that it does not read: a
// list, a list in brackets, a map in braces that does not read, and a line
// that is not a key.
func yamlMap(lines []string, first int) (map[string]any, map[string]int, []unread) {
	type level struct {
		indent int
		path   string
		m      map[string]any
	}
	root := map[string]any{}
	lineOf := map[string]int{}
	var skipped []unread
	stack := []level{{-1, "", root}}
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		indent := yamlIndent(lines[i])
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		k, v, ok := yamlKey(t)
		if !ok {
			// A line that is not a key: an item of a list with no key,
			// or a text. The lines under it go with it.
			what := fmt.Sprintf("the text %q of the front matter, which is not a key of YAML", t)
			if yamlItem(t) {
				what = "a list of YAML in the front matter"
			}
			skipped = append(skipped, unread{first + i, what})
			for i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || yamlIndent(lines[i+1]) > indent) {
				i++
			}
			continue
		}
		v = yamlComment(v)
		top := stack[len(stack)-1]
		cur, path := top.m, top.path+k
		if top.path == "" {
			lineOf[k] = first + i
		}
		// The lines under the key, more indented than it, with the blank
		// lines between them. An item of a list can have the indent of its
		// key.
		end := i + 1
		for end < len(lines) && (strings.TrimSpace(lines[end]) == "" || yamlIndent(lines[end]) > indent ||
			v == "" && yamlIndent(lines[end]) == indent && yamlItem(strings.TrimSpace(lines[end]))) {
			end++
		}
		under, at := lines[i+1:end], first+i
		switch {
		case yamlBlockHeader(v):
			cur[k] = yamlBlock(v, under)
			i = end - 1
		case v == "" && yamlList(under):
			skipped = append(skipped, unread{at, "the list of YAML under " + path})
			i = end - 1
		case v == "" && !yamlNested(under):
			if s := yamlComment(yamlPlain(under)); s != "" {
				cur[k] = s
				i = end - 1
				continue
			}
			m := map[string]any{}
			cur[k] = m
			stack = append(stack, level{indent, path + ".", m})
		case v == "":
			m := map[string]any{}
			cur[k] = m
			stack = append(stack, level{indent, path + ".", m})
		default:
			if open := yamlOpenQuote(v); open {
				// A text in quotes goes on up to its closing quote, at
				// any indent.
				for end = i + 1; end < len(lines) && open; end++ {
					open = yamlOpenQuote(v + " " + yamlPlain(lines[i+1:end+1]))
				}
				under = lines[i+1 : end]
			}
			if more := yamlPlain(under); more != "" {
				v += " " + more
				i = end - 1
			}
			v = yamlComment(v)
			switch {
			case strings.HasPrefix(v, "["):
				skipped = append(skipped, unread{at, "the list of YAML under " + path})
			case strings.HasPrefix(v, "{"):
				var m map[string]any
				if json.Unmarshal([]byte(relaxedJSON(v)), &m) != nil {
					skipped = append(skipped, unread{at, "the map of YAML in braces under " + path})
					continue
				}
				cur[k] = m
			default:
				cur[k] = scalar(v)
			}
		}
	}
	return root, lineOf, skipped
}

// yamlOpenQuote reports whether a value of YAML starts with a quote and
// does not close it.
func yamlOpenQuote(v string) bool {
	if v == "" || v[0] != '"' && v[0] != '\'' {
		return false
	}
	q := v[0]
	for i := 1; i < len(v); i++ {
		switch {
		case q == '"' && v[i] == '\\':
			i++
		case q == '\'' && v[i] == '\'' && i+1 < len(v) && v[i+1] == '\'':
			i++
		case v[i] == q:
			return false
		}
	}
	return true
}

// yamlItem reports whether a line of YAML is an item of a list: a hyphen
// alone, or a hyphen and a space.
func yamlItem(t string) bool {
	return t == "-" || strings.HasPrefix(t, "- ")
}

// yamlList reports whether the first line under a key with no value is an
// item of a list, so that the key holds a list.
func yamlList(under []string) bool {
	for _, l := range under {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return yamlItem(t)
	}
	return false
}

// yamlComment cuts a comment off a value of YAML. A comment starts with a
// # after a space, outside quotes, and runs to the end of the value. A
// value in quotes starts with the quote, and its comment can only come
// after the closing quote.
func yamlComment(v string) string {
	from := 0
	switch q := v[:min(1, len(v))]; q {
	case `"`, "'":
		from = -1
		for i := 1; i < len(v); i++ {
			switch {
			case q == `"` && v[i] == '\\':
				i++
			case q == "'" && v[i] == '\'' && i+1 < len(v) && v[i+1] == '\'':
				i++
			case v[i] == q[0]:
				from = i + 1
			}
			if from >= 0 {
				break
			}
		}
		if from < 0 {
			return v
		}
	}
	for i := from; i < len(v); i++ {
		if v[i] == '#' && (i == 0 || v[i-1] == ' ' || v[i-1] == '\t') {
			return strings.TrimSpace(v[:i])
		}
	}
	return v
}

// yamlIndent returns the number of spaces before the text of a line.
func yamlIndent(l string) int {
	return len(l) - len(strings.TrimLeft(l, " "))
}

// yamlKey splits a line of a map into its key and its value. The colon of a
// key ends the line or has a space after it, so the colon of a text such as
// fa:fa-cogs is not a key.
func yamlKey(t string) (string, string, bool) {
	for i := range len(t) {
		if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ' || t[i+1] == '\t') {
			return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+1:]), true
		}
	}
	return "", "", false
}

// yamlNested reports whether the first line under a key with no value is a
// key of a map, so that the key holds a map.
func yamlNested(under []string) bool {
	for _, l := range under {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		_, _, ok := yamlKey(t)
		return ok
	}
	return false
}

// yamlPlain joins the lines of a plain text that goes on under its key: the
// lines join with a space, and a blank line is a new line.
func yamlPlain(under []string) string {
	var b strings.Builder
	blank := false
	for _, l := range under {
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			blank = b.Len() > 0
			if blank {
				b.WriteByte('\n')
			}
			continue
		case b.Len() > 0 && !blank:
			b.WriteByte(' ')
		}
		blank = false
		b.WriteString(t)
	}
	return strings.TrimRight(b.String(), "\n")
}

// yamlBlockRe matches the header of a block scalar: | or >, then a digit of
// indent and a sign of chomping, in either order.
var yamlBlockRe = regexp.MustCompile(`^([|>])([1-9]?)([+-]?)([1-9]?)(?:\s+#.*)?$`)

// yamlBlockHeader reports whether a value starts a block scalar.
func yamlBlockHeader(v string) bool {
	return yamlBlockRe.MatchString(v)
}

// yamlBlock reads a block scalar. A literal block, after |, keeps its new
// lines. A folded block, after >, joins its lines with a space, except a
// blank line and a line indented more than the block. The chomping sign
// sets the new lines at the end: - for none, + for all of them, and one
// by default.
func yamlBlock(header string, under []string) string {
	m := yamlBlockRe.FindStringSubmatch(header)
	literal, chomp := m[1] == "|", m[3]
	// The indent of the block is the indent of its first line with text.
	indent := -1
	for _, l := range under {
		if strings.TrimSpace(l) != "" {
			indent = yamlIndent(l)
			break
		}
	}
	if indent < 0 {
		return ""
	}
	var rows []string
	for _, l := range under {
		if strings.TrimSpace(l) == "" {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, l[min(indent, yamlIndent(l)):])
	}
	trailing := 0
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
		trailing++
	}
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			prev := rows[i-1]
			switch {
			case literal, r == "", strings.HasPrefix(r, " "), strings.HasPrefix(prev, " "):
				b.WriteByte('\n')
			case prev != "":
				b.WriteByte(' ')
			}
		}
		b.WriteString(r)
	}
	switch chomp {
	case "-":
	case "+":
		b.WriteString(strings.Repeat("\n", trailing+1))
	default:
		b.WriteByte('\n')
	}
	return b.String()
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

// Graph reads a flowchart, a swimlane diagram, a state diagram, a class
// diagram, an entity relationship diagram, a requirement diagram, an
// agentflow diagram or a use case diagram into a graph. A swimlane diagram,
// and a flowchart with the layout swimlane, lay out in lanes. The report
// names each part that zuzu does not draw, and in the strict mode such a
// part is an error.
func Graph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	is := &graph.Issues{Strict: strict}
	if err := d.reportUnread(is); err != nil {
		return nil, is.Report, err
	}
	g, report, err := readGraph(d, strict)
	report = append(is.Report, report...)
	if err != nil {
		return g, report, err
	}
	sp := &graph.Issues{Strict: strict}
	err = graphSpacing(d, g, sp)
	report = append(report, sp.Report...)
	if err != nil {
		return g, report, err
	}
	familyStyle(g, str(d.Config, "fontFamily"))
	return g, report, nil
}

// reportUnread reports each part of the front matter that zuzu does not
// read, at its line.
func (d *Diagram) reportUnread(is *graph.Issues) error {
	for _, u := range d.unread {
		if err := is.Add(fmt.Sprintf("line %d", u.n), "%s", u.what); err != nil {
			return err
		}
	}
	return nil
}

// familyStyle gives each node, edge and cluster of a graph the weight and
// the style that the setting fontFamily names, such as Helvetica-Bold,
// unless its own style sets them. zuzu draws in IBM Plex Sans only, so the
// rest of the name changes nothing (D32).
func familyStyle(g *graph.Graph, family string) {
	weight, style := graph.FontFace(family)
	if weight == "" && style == "" {
		return
	}
	set := func(st map[string]string) map[string]string {
		if st == nil {
			st = map[string]string{}
		}
		if _, ok := st["fontWeight"]; !ok && weight != "" {
			st["fontWeight"] = weight
		}
		if _, ok := st["fontStyle"]; !ok && style != "" {
			st["fontStyle"] = style
		}
		return st
	}
	for _, n := range g.Nodes {
		n.Style = set(n.Style)
	}
	for _, e := range g.Edges {
		e.Style = set(e.Style)
	}
	for _, c := range g.Clusters {
		c.Style = set(c.Style)
	}
}

// readGraph reads a graph of any kind.
func readGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	switch d.Kind {
	case "flowchart", "graph", "flowchart-elk", "swimlane-beta":
	case "stateDiagram", "stateDiagram-v2":
		return stateGraph(d, strict)
	case "classDiagram", "classDiagram-v2":
		return classGraph(d, strict)
	case "erDiagram":
		return erGraph(d, strict)
	case "requirementDiagram":
		return requirementGraph(d, strict)
	case "agentflow-beta":
		return agentflowGraph(d, strict)
	case "usecase-beta":
		return usecaseGraph(d, strict)
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
	f.inheritDir()
	if lanesOn(d) {
		f.lanes()
	}
	return f.g, f.issues.Report, nil
}
