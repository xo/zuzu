package mermaid

import (
	"fmt"
	"maps"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// erDiagram reads the body of an entity relationship diagram, erDiagram,
// into a graph. An entity with attributes is a record: its name, then a
// row for each attribute, with its type, its name, its keys and its
// comment in columns.
type erDiagram struct {
	d      *Diagram
	g      *graph.Graph
	issues *graph.Issues
	// entities holds each entity by its id.
	entities map[string]*entity
	// clusters holds each subgraph by its id, and scopes the open
	// subgraphs, the innermost last.
	clusters map[string]*graph.Cluster
	scopes   []string
	anon     int
	// body is the entity whose attributes the current line is in, or nil.
	body *entity
	// styles, nodeClasses and nodeStyles are the classes and the styles,
	// as in a flowchart.
	styles      map[string]map[string]string
	nodeClasses map[string][]string
	nodeStyles  map[string]map[string]string
	line        int
	acc         []string
}

// entity is an entity: its node, its name as it draws, and its attributes.
type entity struct {
	node  *graph.Node
	name  string
	attrs []attribute
}

// attribute is an attribute of an entity: its type, its name, its keys,
// such as PK, FK and UK, and its comment.
type attribute struct {
	typ, name, comment string
	keys               []string
}

// The cardinalities of an end of a relationship, by the arrowheads of crow's
// foot notation that draw them. The first shape is at the entity.
const (
	zeroOrOne  = "teeodot"
	onlyOne    = "teetee"
	zeroOrMore = "crowodot"
	oneOrMore  = "crowtee"
)

// erWords are the cardinalities in words, the longest first, and
// erSymbols the cardinalities as symbols. Mermaid reads each symbol on
// either side of the line.
var (
	erWords = []struct{ word, arrow string }{
		{"one or zero", zeroOrOne}, {"zero or one", zeroOrOne}, {"one or more", oneOrMore}, {"one or many", oneOrMore},
		{"zero or more", zeroOrMore}, {"zero or many", zeroOrMore}, {"only one", onlyOne}, {"many(0)", zeroOrMore},
		{"many(1)", oneOrMore}, {"many", zeroOrMore}, {"one", onlyOne}, {"1+", oneOrMore}, {"0+", zeroOrMore}, {"1", onlyOne},
	}
	erSymbols = map[string]string{
		"|o": zeroOrOne, "o|": zeroOrOne, "||": onlyOne, "}o": zeroOrMore, "o{": zeroOrMore, "}|": oneOrMore, "|{": oneOrMore,
	}
)

// erGraph reads an entity relationship diagram into a graph.
func erGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	e := &erDiagram{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		entities: map[string]*entity{}, clusters: map[string]*graph.Cluster{},
		styles: map[string]map[string]string{}, nodeClasses: map[string][]string{}, nodeStyles: map[string]map[string]string{},
	}
	if err := sectionConfig(d, e.issues, "er"); err != nil {
		return nil, e.issues.Report, err
	}
	if err := e.parse(); err != nil {
		return nil, e.issues.Report, err
	}
	// Mermaid wraps the label of a relationship, and not an entity.
	wrapGraph(d, e.g, 0)
	return e.g, e.issues.Report, nil
}

func (e *erDiagram) add(format string, args ...any) error {
	return e.issues.Add(fmt.Sprintf("line %d", e.line), format, args...)
}

func (e *erDiagram) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", e.line, fmt.Sprintf(format, args...), ErrSyntax)
}

// parse reads the lines of the body.
func (e *erDiagram) parse() error {
	inBody := func() bool { return e.body != nil }
	if err := readStatements(e.d, &e.line, &e.acc, inBody, e.bodyLine, e.stmt); err != nil {
		return err
	}
	switch {
	case e.body != nil:
		return fmt.Errorf("the entity %s has no end brace: %w", e.body.node.ID, ErrSyntax)
	case len(e.scopes) > 0:
		return fmt.Errorf("the subgraph %s has no end: %w", e.scopes[len(e.scopes)-1], ErrSyntax)
	}
	return e.finish()
}

// stmt reads one statement outside the attributes of an entity.
func (e *erDiagram) stmt(t string) error {
	word, rest, _ := strings.Cut(t, " ")
	rest = strings.TrimSpace(rest)
	switch word {
	case "direction":
		dir, err := direction(rest)
		if err != nil {
			return fmt.Errorf("line %d: %w", e.line, err)
		}
		if c := e.cluster(); c != "" {
			e.clusters[c].Direction = dir
			return nil
		}
		e.g.Direction = dir
		return nil
	case "subgraph":
		return e.subgraph(rest)
	case "end":
		if rest != "" || len(e.scopes) == 0 {
			break
		}
		e.scopes = e.scopes[:len(e.scopes)-1]
		return nil
	case "classDef":
		names, def, _ := strings.Cut(rest, " ")
		st, err := css(def, e.add)
		if err != nil {
			return err
		}
		for n := range strings.SplitSeq(names, ",") {
			e.styles[strings.TrimSpace(n)] = st
		}
		return nil
	case "class":
		f := strings.Fields(rest)
		if len(f) < 2 {
			return e.errorf("a class with no name")
		}
		for id := range strings.SplitSeq(strings.Join(f[:len(f)-1], ""), ",") {
			if id = strings.TrimSpace(id); id != "" {
				e.ref(id, "")
				e.nodeClasses[id] = append(e.nodeClasses[id], f[len(f)-1])
			}
		}
		return nil
	case "style":
		ids, def, _ := strings.Cut(rest, " ")
		st, err := css(def, e.add)
		if err != nil {
			return err
		}
		for id := range strings.SplitSeq(ids, ",") {
			id = strings.TrimSpace(id)
			e.ref(id, "")
			if e.nodeStyles[id] == nil {
				e.nodeStyles[id] = map[string]string{}
			}
			maps.Copy(e.nodeStyles[id], st)
		}
		return nil
	case "click":
		return e.add("the interaction click")
	}
	id, rest, err := e.name(t)
	if err != nil {
		return err
	}
	if c := e.cluster(); c != "" && (rest == "" || strings.HasPrefix(rest, "{")) {
		// An entity that a subgraph names moves into it.
		if ent, ok := e.entities[id]; ok {
			ent.node.Cluster = c
		}
	}
	switch {
	case rest == "":
		return nil
	case strings.HasPrefix(rest, "{"):
		ent := e.entities[id]
		if ent == nil {
			return e.errorf("the attributes of the subgraph %s", id)
		}
		e.body = ent
		body := strings.TrimSpace(rest[1:])
		if inner, ok := strings.CutSuffix(body, "}"); ok {
			e.body = nil
			body = strings.TrimSpace(inner)
		}
		if body != "" {
			return e.attribute(ent, body)
		}
		return nil
	}
	return e.relationship(id, rest, t)
}

// name reads the name of an entity at the start of a text: a word, or a
// text in double quotes, with an optional alias in brackets, as p[Person]
// or p["A person"], and an optional class, as p:::hot. It makes the entity
// when it is new, and returns its id and the rest of the text.
func (e *erDiagram) name(t string) (string, string, error) {
	t = strings.TrimSpace(t)
	var id string
	if strings.HasPrefix(t, `"`) {
		end := strings.IndexByte(t[1:], '"')
		if end < 0 {
			return "", "", e.errorf("the name %q", t)
		}
		id, t = t[1:end+1], t[end+2:]
	} else {
		end := strings.IndexAny(t, " \t{[|}:")
		if end < 0 {
			end = len(t)
		}
		id, t = t[:end], t[end:]
	}
	if id == "" {
		return "", "", e.errorf("%q is not a statement", t)
	}
	alias := ""
	if strings.HasPrefix(t, "[") {
		end := strings.IndexByte(t, ']')
		if end < 0 {
			return "", "", e.errorf("the alias %q", t)
		}
		alias, t = unquote(t[1:end]), t[end+1:]
	}
	if _, ok := e.clusters[id]; ok && e.entities[id] == nil {
		// The end of a relationship can be a subgraph.
		return id, strings.TrimSpace(t), nil
	}
	cls := ""
	if rest, ok := strings.CutPrefix(t, ":::"); ok {
		end := strings.IndexAny(rest, " \t{")
		if end < 0 {
			end = len(rest)
		}
		cls, t = rest[:end], rest[end:]
	}
	e.ref(id, alias)
	if cls != "" {
		e.nodeClasses[id] = append(e.nodeClasses[id], cls)
	}
	return id, strings.TrimSpace(t), nil
}

// ref makes the entity of an id when it is new. An alias names the entity
// as it draws.
func (e *erDiagram) ref(id, alias string) {
	ent, ok := e.entities[id]
	if !ok {
		n := &graph.Node{ID: id, Label: id, Shape: "rect", Cluster: e.cluster(), Style: map[string]string{}}
		ent = &entity{node: n, name: id}
		e.entities[id] = ent
		e.g.Nodes = append(e.g.Nodes, n)
	}
	if alias != "" {
		ent.name = alias
	}
}

// cluster returns the innermost open subgraph, or empty.
func (e *erDiagram) cluster() string {
	if len(e.scopes) == 0 {
		return ""
	}
	return e.scopes[len(e.scopes)-1]
}

// subgraph opens a subgraph: subgraph id, subgraph "title", subgraph
// id[title] or subgraph id["title"]. Its end is the line end.
func (e *erDiagram) subgraph(rest string) error {
	id, title := rest, rest
	switch {
	case strings.HasPrefix(rest, `"`):
		title = unquote(rest)
		id = title
	case strings.Contains(rest, "["):
		i := strings.Index(rest, "[")
		id = strings.TrimSpace(rest[:i])
		title = label(strings.TrimSuffix(strings.TrimSpace(rest[i+1:]), "]"))
	}
	if id == "" {
		e.anon++
		id = fmt.Sprintf("subgraph%d", e.anon)
	}
	if _, ok := e.clusters[id]; !ok {
		c := &graph.Cluster{ID: id, Label: title, Parent: e.cluster(), Style: map[string]string{}}
		e.clusters[id] = c
		e.g.Clusters = append(e.g.Clusters, c)
	}
	e.scopes = append(e.scopes, id)
	return nil
}

// bodyLine reads a line of the attributes of an entity, or the end brace.
func (e *erDiagram) bodyLine(t string) error {
	ent := e.body
	if inner, ok := strings.CutSuffix(t, "}"); ok {
		e.body = nil
		t = strings.TrimSpace(inner)
	}
	if t == "" {
		return nil
	}
	return e.attribute(ent, t)
}

// attribute reads an attribute: its type, its name, its keys, such as PK
// or PK, FK, and its comment in double quotes.
func (e *erDiagram) attribute(ent *entity, t string) error {
	var a attribute
	if strings.HasSuffix(t, `"`) && len(t) >= 2 {
		if i := strings.LastIndexByte(t[:len(t)-1], '"'); i >= 0 {
			a.comment, t = label(t[i+1:len(t)-1]), strings.TrimSpace(t[:i])
		}
	}
	f := strings.Fields(t)
	if len(f) < 2 {
		return e.errorf("the attribute %q", t)
	}
	a.typ, a.name = f[0], f[1]
	a.keys = strings.FieldsFunc(strings.Join(f[2:], " "), func(r rune) bool { return r == ',' || r == ' ' })
	ent.attrs = append(ent.attrs, a)
	return nil
}

// relationship reads the rest of a relationship after its first entity:
// the cardinality of each end, as symbols or as words, the line between
// them, the second entity, and an optional label after a colon.
func (e *erDiagram) relationship(from, rest, t string) error {
	tail, rest, ok := erCard(rest)
	if !ok {
		return e.errorf("%q is not a statement", t)
	}
	dash := false
	switch {
	case strings.HasPrefix(rest, "--"):
		rest = rest[2:]
	case strings.HasPrefix(rest, ".."), strings.HasPrefix(rest, ".-"), strings.HasPrefix(rest, "-."):
		rest, dash = rest[2:], true
	case strings.HasPrefix(rest, "to "):
		rest = rest[3:]
	case strings.HasPrefix(rest, "optionally to "):
		rest, dash = rest[14:], true
	default:
		return e.errorf("the relationship %q", t)
	}
	head, rest, ok := erCard(strings.TrimSpace(rest))
	if !ok {
		return e.errorf("the relationship %q", t)
	}
	to, rest, err := e.name(rest)
	if err != nil {
		return err
	}
	text := ""
	if rest != "" {
		r, ok := strings.CutPrefix(rest, ":")
		if !ok {
			return e.errorf("the relationship %q", t)
		}
		text = label(r)
	}
	text, font := markdownFont(text)
	// The cardinality u, the parent of Mermaid, draws no mark in Mermaid
	// 12, so it draws none here either.
	ed := &graph.Edge{Source: from, Target: to, Label: text, Head: "none", Tail: "none", ArrowSize: relationArrowSize, Style: fontStyle(font)}
	if head != "u" {
		ed.Head = head
	}
	if tail != "u" {
		ed.Tail = tail
	}
	if dash {
		ed.Style["strokeDash"] = dashed
	}
	e.g.Edges = append(e.g.Edges, ed)
	return nil
}

// erCard reads a cardinality at the start of a text: a symbol, a word, or
// u, the parent of Mermaid. A word ends at a space or at the line of the
// relationship. It returns the arrowhead, or u, and the rest of the text.
func erCard(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if a, ok := erSymbols[s[:2]]; ok {
			return a, strings.TrimSpace(s[2:]), true
		}
	}
	for _, w := range append(erWords, struct{ word, arrow string }{"u", "u"}) {
		rest, ok := strings.CutPrefix(s, w.word)
		if ok && (rest == "" || strings.ContainsAny(rest[:1], " \t-.")) {
			return w.arrow, strings.TrimSpace(rest), true
		}
	}
	return "", s, false
}

// finish writes the record of each entity with attributes and applies the
// classes and the styles.
func (e *erDiagram) finish() error {
	if err := e.subgraphEnds(); err != nil {
		return err
	}
	// fonts holds the font of the name of each entity with no attributes,
	// from its markdown, which a class or a style can change.
	fonts := map[string]map[string]string{}
	for _, n := range e.g.Nodes {
		ent := e.entities[n.ID]
		name, font := markdownFont(ent.name)
		n.Label = name
		if len(ent.attrs) == 0 {
			fonts[n.ID] = fontStyle(font)
			continue
		}
		font.Bold = true
		n.Fields = []graph.Field{{Text: name, Fonts: []graph.Font{font}}}
		for _, a := range ent.attrs {
			typ, typFont := markdownFont(a.typ)
			attr, attrFont := markdownFont(a.name)
			comment, commentFont := markdownFont(a.comment)
			f := graph.Field{Cells: []string{typ, attr, strings.Join(a.keys, ", "), comment}, Port: a.name}
			if typFont != (graph.Font{}) || attrFont != (graph.Font{}) || commentFont != (graph.Font{}) {
				f.Fonts = []graph.Font{typFont, attrFont, {}, commentFont}
			}
			n.Fields = append(n.Fields, f)
		}
		n.Stack = true
	}
	for _, n := range e.g.Nodes {
		n.Style = classStyle(e.styles, e.nodeClasses[n.ID], e.nodeStyles[n.ID], n.Style)
		for k, v := range fonts[n.ID] {
			if _, ok := n.Style[k]; !ok {
				n.Style[k] = v
			}
		}
	}
	e.g.Description = describe(e.d, e.acc...)
	return nil
}

// subgraphEnds takes out each entity with the id of a subgraph that only
// a relationship before the subgraph made, since the layout ends such a
// relationship at the box of the subgraph. A relationship to a subgraph
// that holds no entity is an error.
func (e *erDiagram) subgraphEnds() error {
	holds := map[string]bool{}
	var keep []*graph.Node
	for _, n := range e.g.Nodes {
		if _, ok := e.clusters[n.ID]; ok && len(e.entities[n.ID].attrs) == 0 {
			delete(e.entities, n.ID)
			continue
		}
		keep = append(keep, n)
		for c := n.Cluster; c != "" && !holds[c]; c = e.clusters[c].Parent {
			holds[c] = true
		}
	}
	e.g.Nodes = keep
	for _, ed := range e.g.Edges {
		for _, end := range []string{ed.Source, ed.Target} {
			if _, ok := e.clusters[end]; ok && !holds[end] {
				return fmt.Errorf("a relationship to the empty subgraph %s: %w", end, ErrSyntax)
			}
		}
	}
	return nil
}

// markdownFont reads the marks of markdown in a name or a label of an
// entity relationship diagram, as Mermaid does. It returns the text
// without its marks, and the font that all of its text shares: bold,
// italic, both or neither. A line of a record or of a label draws in one
// font, so a part of the text that alone is bold or italic draws plain.
func markdownFont(s string) (string, graph.Font) {
	var b strings.Builder
	bold, italic, some := true, true, false
	for _, r := range markdownRuns(s) {
		b.WriteString(r.text)
		if strings.TrimSpace(r.text) == "" {
			continue
		}
		some = true
		bold, italic = bold && r.bold, italic && r.italic
	}
	if !some {
		return b.String(), graph.Font{}
	}
	return b.String(), graph.Font{Bold: bold, Italic: italic}
}

// fontStyle returns the style of a label in a font.
func fontStyle(f graph.Font) map[string]string {
	st := map[string]string{}
	if f.Bold {
		st["fontWeight"] = graph.BoldWeight
	}
	if f.Italic {
		st["fontStyle"] = "italic"
	}
	return st
}
