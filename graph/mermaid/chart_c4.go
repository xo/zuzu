package mermaid

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/scene"
	"github.com/xo/zuzu/text"
)

// c4Attr is an argument of a statement of a C4 diagram: a value, or a
// named argument such as $tags="v1".
type c4Attr struct {
	key, value string
	named      bool
}

// c4Elem is an element of a C4 diagram: a person, a system, a container or
// a component, with its database, queue and external forms.
type c4Elem struct {
	alias string
	// kind is the kind of the element as Mermaid names it, such as person
	// or external_system_db.
	kind  string
	label string
	// fields are the other texts of the element by the names of Mermaid:
	// descr, techn, sprite, tags, link, shape, bgColor, fontColor and
	// borderColor.
	fields map[string]string
	// parent is the alias of the boundary that holds the element.
	parent string
	line   int

	// The layout: the shape, the texts in the shape, and the box of the
	// shape, with its top left at x, y.
	shape      string
	texts      []*c4Text
	tw, th     float64
	x, y, w, h float64
}

// c4Boundary is a boundary of a C4 diagram, or a deployment node, which
// holds elements and boundaries.
type c4Boundary struct {
	alias string
	label string
	// fields are the other texts of the boundary: type, descr, sprite,
	// tags, link, bgColor, fontColor and borderColor.
	fields map[string]string
	// node says whether the boundary is a deployment node, which draws
	// with a solid line.
	node   bool
	parent string
	line   int

	// The layout: the label, the type and the description, each at its
	// offset from the top of the box, and the box.
	texts      []*c4Text
	x, y, w, h float64
}

// c4Rel is a relationship of a C4 diagram.
type c4Rel struct {
	// kind is rel, birel or rel_b. A direction, such as Rel_U, draws as a
	// rel, as in Mermaid.
	kind, from, to, label string
	// fields are the other texts of the relationship: techn, descr,
	// sprite, tags, link, textColor, lineColor, offsetX and offsetY.
	fields map[string]string
	line   int
}

// c4Diagram is the state of the compile of a C4 diagram.
type c4Diagram struct {
	c *chart
	// src is the body of the diagram, and lines holds the offset in src
	// and the number in the source of each of its lines.
	src   string
	lines [][2]int
	pos   int
	ahead *c4Token

	elems   []*c4Elem
	bounds  []*c4Boundary
	rels    []*c4Rel
	byAlias map[string]*c4Elem
	boundBy map[string]*c4Boundary
	// current is the alias of the boundary that the parse is in, and
	// stack the boundaries around it.
	current string
	stack   []string
	title   string
	titled  bool

	// The settings of the layout, in the pixels of Mermaid.
	shapeInRow, boundaryInRow                   int
	margin, pad, width, marginX, marginY, lineX float64
	// wrap is the setting wrap of the top of the config, which wraps the
	// texts of a boundary with c4.wrap, and c4wrap is c4.wrap, which wraps
	// the texts of an element.
	wrap, c4wrap bool
	m            text.Measurer
}

// The kinds of the tokens of a C4 diagram.
const (
	c4EOF = iota
	c4Newline
	c4LBrace
	c4RBrace
	c4Kind
	c4Title
	c4AccTitle
	c4AccDescr
	c4Call
)

// c4Token is a token of a C4 diagram: a statement that runs to its end,
// with its keyword and its arguments, or a part of the structure.
type c4Token struct {
	kind  int
	pos   int
	word  string
	text  string
	attrs []c4Attr
}

// c4Global is the alias of the boundary that holds the whole diagram.
const c4Global = "global"

// The keywords of the kinds of C4 diagrams.
const (
	c4Context    = "C4Context"
	c4Container  = "C4Container"
	c4Component  = "C4Component"
	c4Dynamic    = "C4Dynamic"
	c4Deployment = "C4Deployment"
)

// c4Keywords are the keywords of the kinds of C4 diagrams.
var c4Keywords = []string{c4Context, c4Container, c4Component, c4Dynamic, c4Deployment}

// The kinds of relationships that draw another way, and the statement
// that updates a relationship.
const (
	c4BiRel        = "birel"
	c4RelBack      = "rel_b"
	updateRelStyle = "UpdateRelStyle"
)

// c4Words are the keywords of the statements of a C4 diagram, by the kind
// of the element, the boundary, the relationship or the update that each
// adds, as Mermaid names them.
var c4Words = map[string]string{
	"Person": "person", "Person_Ext": "external_person",
	"System": "system", "System_Ext": "external_system", "SystemDb": "system_db", "SystemDb_Ext": "external_system_db",
	"SystemQueue": "system_queue", "SystemQueue_Ext": "external_system_queue",
	"Container": "container", "Container_Ext": "external_container", "ContainerDb": "container_db",
	"ContainerDb_Ext": "external_container_db", "ContainerQueue": "container_queue", "ContainerQueue_Ext": "external_container_queue",
	"Component": "component", "Component_Ext": "external_component", "ComponentDb": "component_db",
	"ComponentDb_Ext": "external_component_db", "ComponentQueue": "component_queue", "ComponentQueue_Ext": "external_component_queue",
	"Boundary": "boundary", "Enterprise_Boundary": "boundary", "System_Boundary": "boundary", "Container_Boundary": "boundary",
	"Deployment_Node": "node", "Node": "node", "Node_L": "node", "Node_R": "node",
	"Rel": "rel", "BiRel": "rel", "Rel_Up": "rel", "Rel_U": "rel", "Rel_Down": "rel", "Rel_D": "rel",
	"Rel_Left": "rel", "Rel_L": "rel", "Rel_Right": "rel", "Rel_R": "rel", "Rel_Back": "rel", "RelIndex": "rel",
	"UpdateElementStyle": "update", "UpdateRelStyle": "update", "UpdateLayoutConfig": "update",
}

// The tokens of a C4 diagram, in the order in which Mermaid 12.3 tries
// them.
var (
	c4DirectionRE = regexp.MustCompile(`^[^\n]*direction\s+(?:TB|BT|RL|LR)`)
	c4TitleRE     = regexp.MustCompile(`^title\s[^#\n;]+`)
	c4AccDescRE   = regexp.MustCompile(`^accDescription\s[^#\n;]+`)
	c4AccTitleRE  = regexp.MustCompile(`^accTitle\s*:\s*`)
	c4AccDescrRE  = regexp.MustCompile(`^accDescr\s*:\s*`)
	c4AccBlockRE  = regexp.MustCompile(`^accDescr\s*\{\s*`)
	c4NewlineRE   = regexp.MustCompile(`^\s*\n`)
	c4SpaceRE     = regexp.MustCompile(`^\s+`)
	c4WordRE      = regexp.MustCompile(`^\w+`)
	c4LeadRE      = regexp.MustCompile(`^ *,`)
	c4EmptyRE     = regexp.MustCompile(`^ *""`)
	c4QuoteRE     = regexp.MustCompile(`^ *"`)
	c4NamedRE     = regexp.MustCompile(`^ *\$`)
	c4ValueRE     = regexp.MustCompile(`^= *"`)
)

// c4Chart compiles a C4 diagram of Mermaid: C4Context, C4Container,
// C4Component, C4Dynamic or C4Deployment. Its elements lie on the grid of
// Mermaid, in rows inside their boundaries, and compile to a Vega spec
// laid out in Go.
func c4Chart(c *chart) (map[string]any, bool, error) {
	p := &c4Diagram{
		c: c, byAlias: map[string]*c4Elem{}, boundBy: map[string]*c4Boundary{},
		current: c4Global, shapeInRow: 4, boundaryInRow: 2,
		margin: 50, pad: 20, width: 216, marginX: 50, marginY: 10,
		c4wrap: true, m: text.NewEstimate(),
	}
	if err := p.settings(); err != nil {
		return nil, false, err
	}
	global := &c4Boundary{alias: c4Global, label: c4Global, fields: map[string]string{"type": c4Global}}
	p.bounds = append(p.bounds, global)
	p.boundBy[c4Global] = global
	// The body is one stream of tokens, as in Mermaid. The keyword must
	// end its line.
	var sb strings.Builder
	add := func(t string, n int) {
		p.lines = append(p.lines, [2]int{sb.Len(), n})
		sb.WriteString(t)
		sb.WriteByte('\n')
	}
	add(strings.TrimSpace(c.d.Kind+" "+c.d.Header), c.d.N)
	for _, l := range c.d.Lines {
		add(l.Text, l.N)
	}
	p.src = strings.TrimSuffix(sb.String(), "\n")
	if err := p.document(); err != nil {
		return nil, false, err
	}
	if err := p.check(); err != nil {
		return nil, false, err
	}
	c.line = 0
	p.layout()
	spec := p.spec()
	if p.titled {
		if t := strings.TrimSpace(p.title); t != "" {
			spec["title"] = map[string]any{"text": t, "anchor": "start"}
		}
	}
	return spec, false, nil
}

// settings reads the settings of the config that a C4 diagram reads, in
// the section c4, and wrap at the top. Each other setting is an issue.
func (p *c4Diagram) settings() error {
	c := p.c
	numbers := []struct {
		key string
		v   *float64
	}{
		{"c4ShapeMargin", &p.margin}, {"c4ShapePadding", &p.pad}, {"width", &p.width},
		{"diagramMarginX", &p.marginX}, {"diagramMarginY", &p.marginY}, {"nextLinePaddingX", &p.lineX},
	}
	read := []string{"wrap", "c4.wrap"}
	for _, n := range numbers {
		read = append(read, "c4."+n.key)
	}
	if err := c.unreadSettings(read...); err != nil {
		return err
	}
	c4, _ := c.d.Config["c4"].(map[string]any)
	for _, n := range numbers {
		v, ok := c4[n.key]
		if !ok {
			continue
		}
		if f, ok := configNumber(c.d.Config, "c4", n.key); ok && f >= 0 {
			*n.v = f
		} else if err := c.add("the setting c4.%s: %v", n.key, v); err != nil {
			return err
		}
	}
	for _, w := range []struct {
		v    any
		path string
		dst  *bool
	}{{c.d.Config["wrap"], "wrap", &p.wrap}, {c4["wrap"], "c4.wrap", &p.c4wrap}} {
		if w.v == nil {
			continue
		}
		if b, ok := w.v.(bool); ok {
			*w.dst = b
		} else if err := c.add("the setting %s: %v", w.path, w.v); err != nil {
			return err
		}
	}
	return nil
}

// lineOf returns the number in the source of the line of an offset of the
// body.
func (p *c4Diagram) lineOf(pos int) int {
	i, _ := slices.BinarySearchFunc(p.lines, pos, func(l [2]int, at int) int { return l[0] - at })
	if i == len(p.lines) || p.lines[i][0] > pos {
		i--
	}
	if i < 0 {
		return p.c.d.N
	}
	return p.lines[i][1]
}

// errorAt returns an error of the syntax at an offset of the body.
func (p *c4Diagram) errorAt(pos int, format string, args ...any) error {
	p.c.line = p.lineOf(pos)
	return p.c.errorf(format, args...)
}

// peek returns the next token without reading it.
func (p *c4Diagram) peek() (c4Token, error) {
	if p.ahead == nil {
		t, err := p.lex()
		if err != nil {
			return t, err
		}
		p.ahead = &t
	}
	return *p.ahead, nil
}

// next reads the next token.
func (p *c4Diagram) next() (c4Token, error) {
	t, err := p.peek()
	p.ahead = nil
	return t, err
}

// lex reads a token at the offset of the body, with the rules of Mermaid
// 12.3 in their order. A statement with arguments is one token, from its
// keyword to its closing parenthesis.
func (p *c4Diagram) lex() (c4Token, error) {
	for {
		t := c4Token{pos: p.pos}
		rest := p.src[p.pos:]
		if rest == "" {
			return t, nil
		}
		toEOL := func() string {
			r := p.src[p.pos:]
			if i := strings.IndexByte(r, '\n'); i >= 0 {
				r = r[:i]
			}
			p.pos += len(r)
			return r
		}
		switch {
		case c4DirectionRE.MatchString(rest):
			return t, p.errorAt(t.pos, "a direction, which a C4 diagram does not read, as in Mermaid")
		case c4TitleRE.MatchString(rest):
			m := c4TitleRE.FindString(rest)
			p.pos += len(m)
			t.kind, t.text = c4Title, m[len("title "):]
			return t, nil
		case c4AccDescRE.MatchString(rest):
			m := c4AccDescRE.FindString(rest)
			p.pos += len(m)
			t.kind, t.text = c4AccDescr, m[len("accDescription "):]
			return t, nil
		case c4AccTitleRE.MatchString(rest):
			p.pos += len(c4AccTitleRE.FindString(rest))
			t.kind, t.text = c4AccTitle, toEOL()
			return t, nil
		case c4AccDescrRE.MatchString(rest):
			p.pos += len(c4AccDescrRE.FindString(rest))
			t.kind, t.text = c4AccDescr, toEOL()
			return t, nil
		case c4AccBlockRE.MatchString(rest):
			p.pos += len(c4AccBlockRE.FindString(rest))
			end := strings.IndexByte(p.src[p.pos:], '}')
			if end < 0 {
				return t, p.errorAt(t.pos, "accDescr { with no }")
			}
			t.kind, t.text = c4AccDescr, p.src[p.pos:p.pos+end]
			p.pos += end + 1
			return t, nil
		case c4NewlineRE.MatchString(rest):
			p.pos += len(c4NewlineRE.FindString(rest))
			t.kind = c4Newline
			return t, nil
		case c4SpaceRE.MatchString(rest):
			p.pos += len(c4SpaceRE.FindString(rest))
			continue
		case rest[0] == '{':
			p.pos++
			t.kind = c4LBrace
			return t, nil
		case rest[0] == '}':
			p.pos++
			t.kind = c4RBrace
			return t, nil
		}
		w := c4WordRE.FindString(rest)
		if slices.Contains(c4Keywords, w) {
			p.pos += len(w)
			t.kind, t.word = c4Kind, w
			return t, nil
		}
		if _, ok := c4Words[w]; ok {
			p.pos += len(w)
			t.kind, t.word = c4Call, w
			attrs, err := p.arguments(t)
			t.attrs = attrs
			return t, err
		}
		return t, p.errorAt(t.pos, "the text %q", firstWord(rest))
	}
}

// firstWord returns the text up to the first white space.
func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// arguments reads the arguments of a statement in parentheses, right
// after its keyword, as Mermaid 12.3 reads them. A value in double quotes
// or a named value runs to its end quote, and any other value runs to the
// next comma, even past the closing parenthesis and the end of its line.
func (p *c4Diagram) arguments(t c4Token) ([]c4Attr, error) {
	rest := p.src[p.pos:]
	if !strings.HasPrefix(rest, "(") {
		return nil, p.errorAt(t.pos, "%s with no ( right after it", t.word)
	}
	p.pos++
	var attrs []c4Attr
	empty := c4Attr{}
	if m := c4LeadRE.FindString(p.src[p.pos:]); m != "" {
		p.pos += len(m)
		attrs = append(attrs, empty)
	}
	for {
		rest := p.src[p.pos:]
		switch {
		case rest == "":
			return nil, p.errorAt(t.pos, "%s with no closing parenthesis", t.word)
		case rest[0] == ')':
			p.pos++
			if len(attrs) == 0 {
				return nil, p.errorAt(t.pos, "%s with no arguments", t.word)
			}
			return attrs, nil
		case strings.HasPrefix(rest, ",,"):
			p.pos += 2
			attrs = append(attrs, empty)
		case rest[0] == ',':
			p.pos++
		case c4EmptyRE.MatchString(rest):
			p.pos += len(c4EmptyRE.FindString(rest))
			attrs = append(attrs, empty)
		case c4QuoteRE.MatchString(rest):
			p.pos += len(c4QuoteRE.FindString(rest))
			end := strings.IndexByte(p.src[p.pos:], '"')
			if end < 0 {
				return nil, p.errorAt(t.pos, "a text of %s with no end quote", t.word)
			}
			attrs = append(attrs, c4Attr{value: strings.TrimSpace(p.src[p.pos : p.pos+end])})
			p.pos += end + 1
		case c4NamedRE.MatchString(rest):
			p.pos += len(c4NamedRE.FindString(rest))
			r := p.src[p.pos:]
			eq := strings.IndexByte(r, '=')
			if eq < 0 {
				return nil, p.errorAt(t.pos, "a named argument of %s with no =", t.word)
			}
			key := strings.TrimSpace(r[:eq])
			p.pos += eq
			m := c4ValueRE.FindString(p.src[p.pos:])
			if m == "" {
				return nil, p.errorAt(t.pos, "the named argument $%s of %s with no value in double quotes", key, t.word)
			}
			p.pos += len(m)
			end := strings.IndexByte(p.src[p.pos:], '"')
			if end <= 0 {
				return nil, p.errorAt(t.pos, "the named argument $%s of %s with an empty value or no end quote", key, t.word)
			}
			attrs = append(attrs, c4Attr{key: key, value: strings.TrimSpace(p.src[p.pos : p.pos+end]), named: true})
			p.pos += end + 1
		default:
			end := strings.IndexByte(rest, ',')
			if end < 0 {
				return nil, p.errorAt(t.pos, "%s with no closing parenthesis: a text not in quotes runs to the next comma", t.word)
			}
			attrs = append(attrs, c4Attr{value: strings.TrimSpace(rest[:end])})
			p.pos += end
		}
	}
}

// document reads the keyword of the diagram, a new line, and the
// statements up to the end of the source.
func (p *c4Diagram) document() error {
	if _, err := p.next(); err != nil {
		return err
	}
	t, err := p.next()
	if err != nil {
		return err
	}
	switch t.kind {
	case c4EOF:
		return p.errorAt(t.pos, "a C4 diagram with no statement")
	case c4Newline:
	default:
		return p.errorAt(t.pos, "a statement on the line of %s", p.c.d.Kind)
	}
	return p.statements(false)
}

// statements reads statements up to the end of the source, or up to the }
// of a boundary when nested. A new line ends each statement, and a
// boundary starts with an element, a boundary or a relationship, as in
// Mermaid 12.3.
func (p *c4Diagram) statements(nested bool) error {
	first := true
	for {
		t, err := p.peek()
		if err != nil {
			return err
		}
		switch t.kind {
		case c4EOF:
			if nested {
				return p.errorAt(t.pos, "the boundary %s has no }", p.current)
			}
			if first {
				return p.errorAt(t.pos, "a C4 diagram with no statement")
			}
			return nil
		case c4RBrace:
			if !nested {
				return p.errorAt(t.pos, "a } with no boundary")
			}
			if first {
				return p.errorAt(t.pos, "the boundary %s is empty", p.current)
			}
			return nil
		}
		other := t.kind == c4Title || t.kind == c4AccTitle || t.kind == c4AccDescr
		if nested && first && other {
			return p.errorAt(t.pos, "a title or an accessible text at the start of the boundary %s", p.current)
		}
		if err := p.statement(); err != nil {
			return err
		}
		first = false
		if t, err = p.peek(); err != nil {
			return err
		}
		switch {
		case t.kind == c4Newline:
			_, _ = p.next()
		case t.kind == c4EOF, t.kind == c4RBrace:
		case other && t.kind == c4Call:
			// accDescr { ... } can end on the line of the next
			// statement.
		default:
			return p.errorAt(t.pos, "a second statement on a line")
		}
	}
}

// statement reads one statement.
func (p *c4Diagram) statement() error {
	t, err := p.next()
	if err != nil {
		return err
	}
	p.c.line = p.lineOf(t.pos)
	switch t.kind {
	case c4Title, c4AccTitle:
		// Mermaid 12.3 sets the title of a C4 diagram with accTitle too,
		// and the last of the two wins.
		p.title, p.titled = t.text, true
		return nil
	case c4AccDescr:
		p.c.acc = append(p.c.acc, strings.TrimSpace(t.text))
		return nil
	case c4Call:
	case c4Kind:
		return p.errorAt(t.pos, "a second keyword %s", t.word)
	default:
		return p.errorAt(t.pos, "a { or a new line where a statement starts")
	}
	switch c4Words[t.word] {
	case "rel":
		return p.rel(t)
	case "update":
		return p.update(t)
	case "boundary", "node":
		if err := p.boundary(t); err != nil {
			return err
		}
		if err := p.open(t); err != nil {
			return err
		}
		if err := p.statements(true); err != nil {
			return err
		}
		_, _ = p.next()
		p.current, p.stack = p.stack[len(p.stack)-1], p.stack[:len(p.stack)-1]
		return nil
	}
	return p.element(t)
}

// open reads the { that opens a boundary: { and a new line, or a new line
// and {, with a new line after it or not.
func (p *c4Diagram) open(start c4Token) error {
	t, err := p.next()
	if err != nil {
		return err
	}
	switch t.kind {
	case c4LBrace:
		if t, err = p.next(); err != nil {
			return err
		}
		if t.kind != c4Newline {
			return p.errorAt(t.pos, "a statement on the line of the { of %s", start.word)
		}
		return nil
	case c4Newline:
		if t, err = p.next(); err != nil {
			return err
		}
		if t.kind != c4LBrace {
			return p.errorAt(start.pos, "%s with no {", start.word)
		}
		if t, err = p.peek(); err == nil && t.kind == c4Newline {
			_, _ = p.next()
		}
		return err
	}
	return p.errorAt(start.pos, "%s with no {", start.word)
}

// slot returns the argument at a place, and whether the statement has it.
func slot(attrs []c4Attr, i int) (c4Attr, bool) {
	if i < len(attrs) {
		return attrs[i], true
	}
	return c4Attr{}, false
}

// c4ElemKeys, c4BoundKeys and c4RelKeys are the named arguments that zuzu
// draws on an element, a boundary and a relationship.
var (
	c4ElemKeys  = []string{"label", "descr", "techn", "sprite", "tags", "link", "shape", "bgColor", "fontColor", "borderColor"}
	c4BoundKeys = []string{"label", "type", "descr", "sprite", "tags", "link", "bgColor", "fontColor", "borderColor"}
	c4RelKeys   = []string{"label", "techn", "descr", "sprite", "tags", "link", "textColor", "lineColor", "offsetX", "offsetY"}
)

// assign sets the fields of the places of a statement after its alias and
// its label, as Mermaid 12.3 does: a value at a place sets the field of
// that place, and a named argument at any place sets the field of its
// name. A named argument that zuzu does not draw is an issue.
func (p *c4Diagram) assign(fields map[string]string, attrs []c4Attr, from int, names, known []string, word string) error {
	for i, name := range names {
		a, ok := slot(attrs, from+i)
		if !ok {
			return nil
		}
		if !a.named {
			fields[name] = a.value
			continue
		}
		if a.key == "label" {
			fields["label"] = a.value
			continue
		}
		if !slices.Contains(known, a.key) {
			if err := p.c.add("the argument $%s of %s", a.key, word); err != nil {
				return err
			}
			continue
		}
		fields[a.key] = a.value
	}
	return p.extra(len(attrs)-from-len(names), word)
}

// name reads the alias of a statement, which must be a value and not a
// named argument.
func (p *c4Diagram) name(t c4Token, i int) (string, bool, error) {
	a, ok := slot(t.attrs, i)
	if !ok {
		return "", false, nil
	}
	if a.named {
		return "", false, p.errorAt(t.pos, "the named argument $%s of %s in the place of an alias", a.key, t.word)
	}
	return a.value, true, nil
}

// label reads the label of a statement. A named argument there draws no
// label in Mermaid, and is an issue.
func (p *c4Diagram) label(t c4Token, i int) (string, error) {
	a, ok := slot(t.attrs, i)
	if !ok {
		return "", nil
	}
	if a.named {
		return "", p.c.add("the named argument $%s of %s in the place of the label, which Mermaid draws as no label", a.key, t.word)
	}
	return a.value, nil
}

// element reads a person, a system, a container or a component. A second
// statement with the same alias updates the element, which keeps its place.
func (p *c4Diagram) element(t c4Token) error {
	alias, _, err := p.name(t, 0)
	if err != nil {
		return err
	}
	e, ok := p.byAlias[alias]
	if !ok {
		e = &c4Elem{alias: alias, fields: map[string]string{}, line: p.c.line}
		p.byAlias[alias] = e
		p.elems = append(p.elems, e)
	}
	e.kind = c4Words[t.word]
	if e.label, err = p.label(t, 1); err != nil {
		return err
	}
	names := []string{"descr", "sprite", "tags", "link"}
	if strings.Contains(e.kind, "container") || strings.Contains(e.kind, "component") {
		names = []string{"techn", "descr", "sprite", "tags", "link"}
	}
	if err := p.assign(e.fields, t.attrs, 2, names, c4ElemKeys, t.word); err != nil {
		return err
	}
	if l, ok := e.fields["label"]; ok {
		e.label = l
		delete(e.fields, "label")
	}
	e.parent = p.current
	return nil
}

// boundary reads a boundary or a deployment node, and makes it the
// boundary that the parse is in.
func (p *c4Diagram) boundary(t c4Token) error {
	attrs := t.attrs
	// Mermaid puts the type of the boundary in the third place of its
	// arguments, or in the second when it has one.
	if typ := map[string]string{"Enterprise_Boundary": "ENTERPRISE", "System_Boundary": "SYSTEM", "Container_Boundary": "CONTAINER"}[t.word]; typ != "" {
		at := min(2, len(attrs))
		attrs = slices.Insert(slices.Clone(attrs), at, c4Attr{value: typ})
	}
	alias, _, err := p.name(t, 0)
	if err != nil {
		return err
	}
	if alias == c4Global {
		return p.errorAt(t.pos, "a boundary with the alias global, which Mermaid gives to the whole diagram")
	}
	if alias == p.current || slices.Contains(p.stack, alias) {
		return p.errorAt(t.pos, "the boundary %s inside itself", alias)
	}
	b, ok := p.boundBy[alias]
	if !ok {
		b = &c4Boundary{alias: alias, fields: map[string]string{}, line: p.c.line}
		p.boundBy[alias] = b
		p.bounds = append(p.bounds, b)
	}
	tt := t
	tt.attrs = attrs
	if b.label, err = p.label(tt, 1); err != nil {
		return err
	}
	names := []string{"type", "tags", "link"}
	def := "system"
	if c4Words[t.word] == "node" {
		names = []string{"type", "descr", "sprite", "tags", "link"}
		def = "node"
		b.node = true
		if t.word == "Node_L" || t.word == "Node_R" {
			if err := p.c.add("%s, which draws as Node, as in Mermaid", t.word); err != nil {
				return err
			}
		}
	}
	if _, ok := slot(attrs, 2); !ok {
		b.fields["type"] = def
	}
	if err := p.assign(b.fields, attrs, 2, names, c4BoundKeys, t.word); err != nil {
		return err
	}
	if l, ok := b.fields["label"]; ok {
		b.label = l
		delete(b.fields, "label")
	}
	b.parent = p.current
	p.stack = append(p.stack, p.current)
	p.current = alias
	return nil
}

// rel reads a relationship. A second relationship between the same two
// ends replaces the first, which keeps its place, as in Mermaid.
func (p *c4Diagram) rel(t c4Token) error {
	attrs := t.attrs
	kind := "rel"
	switch t.word {
	case "RelIndex":
		// Mermaid numbers the relationships of a C4Dynamic in their order,
		// and drops the index.
		if len(attrs) > 0 {
			attrs = attrs[1:]
		}
	case "BiRel":
		kind = c4BiRel
	case "Rel_Back":
		kind = c4RelBack
	case "Rel_Up", "Rel_U", "Rel_Down", "Rel_D", "Rel_Left", "Rel_L", "Rel_Right", "Rel_R":
		if err := p.c.add("the direction of %s, which zuzu does not use in the layout, as Mermaid does not", t.word); err != nil {
			return err
		}
	}
	tt := t
	tt.attrs = attrs
	from, _, err := p.name(tt, 0)
	if err != nil {
		return err
	}
	to, ok, err := p.name(tt, 1)
	if err != nil {
		return err
	}
	if _, has := slot(attrs, 2); !ok || !has {
		return p.c.add("%s from %s with no label, which Mermaid does not draw", t.word, from)
	}
	label, err := p.label(tt, 2)
	if err != nil {
		return err
	}
	var r *c4Rel
	for _, old := range p.rels {
		if old.from == from && old.to == to {
			r = old
		}
	}
	if r == nil {
		r = &c4Rel{from: from, to: to, fields: map[string]string{}, line: p.c.line}
		p.rels = append(p.rels, r)
	}
	r.kind, r.label = kind, label
	if err := p.assign(r.fields, attrs, 3, []string{"techn", "descr", "sprite", "tags", "link"}, c4RelKeys, t.word); err != nil {
		return err
	}
	if l, ok := r.fields["label"]; ok {
		r.label = l
		delete(r.fields, "label")
	}
	return nil
}

// update reads UpdateElementStyle, UpdateRelStyle or UpdateLayoutConfig.
// An update of an element or a relationship that does not exist yet does
// nothing in Mermaid, and is an issue.
func (p *c4Diagram) update(t c4Token) error {
	switch t.word {
	case "UpdateLayoutConfig":
		set := func(i int, dst *int) {
			a, ok := slot(t.attrs, i)
			if !ok {
				return
			}
			// Mermaid reads the value of a named argument, whatever its
			// name.
			if n, ok := jsParseInt(a.value); ok && n >= 1 {
				*dst = n
			}
		}
		set(0, &p.shapeInRow)
		set(1, &p.boundaryInRow)
		return p.extra(len(t.attrs)-2, t.word)
	case updateRelStyle:
		from, _, err := p.name(t, 0)
		if err != nil {
			return err
		}
		to, _, err := p.name(t, 1)
		if err != nil {
			return err
		}
		var r *c4Rel
		for _, old := range p.rels {
			if old.from == from && old.to == to {
				r = old
			}
		}
		if r == nil {
			return p.c.add("%s of %s and %s, which have no relationship before it", t.word, from, to)
		}
		return p.updateFields(r.fields, t, []string{"textColor", "lineColor", "offsetX", "offsetY"}, []string{"textColor", "lineColor", "offsetX", "offsetY"})
	}
	alias, _, err := p.name(t, 0)
	if err != nil {
		return err
	}
	names := []string{"bgColor", "fontColor", "borderColor", "shadowing", "shape", "sprite", "techn", "legendText", "legendSprite"}
	if e, ok := p.byAlias[alias]; ok {
		return p.updateFields(e.fields, t, names, []string{"bgColor", "fontColor", "borderColor", "shape", "sprite"})
	}
	if b, ok := p.boundBy[alias]; ok && alias != c4Global {
		return p.updateFields(b.fields, t, names, []string{"bgColor", "fontColor", "borderColor"})
	}
	return p.c.add("%s of %s, which is not an element or a boundary before it", t.word, alias)
}

// updateFields sets the fields of an update from its places after the
// first, as Mermaid does: a value sets the field of its place, and a named
// argument the field of its name. A field that zuzu does not draw is an
// issue.
func (p *c4Diagram) updateFields(fields map[string]string, t c4Token, names, drawn []string) error {
	from := 1
	if t.word == updateRelStyle {
		from = 2
	}
	for i, name := range names {
		a, ok := slot(t.attrs, from+i)
		if !ok {
			break
		}
		key := name
		if a.named {
			key = a.key
		}
		if !slices.Contains(drawn, key) {
			if err := p.c.add("the argument $%s of %s", key, t.word); err != nil {
				return err
			}
			continue
		}
		fields[key] = a.value
	}
	return p.extra(len(t.attrs)-from-len(names), t.word)
}

// extra reports the arguments of a statement after the last that Mermaid
// reads, which Mermaid ignores.
func (p *c4Diagram) extra(n int, word string) error {
	switch {
	case n == 1:
		return p.c.add("an argument of %s after the last that Mermaid reads", word)
	case n > 1:
		return p.c.add("%d arguments of %s after the last that Mermaid reads", n, word)
	}
	return nil
}

// jsParseInt reads a whole number at the start of a text, as parseInt of
// JavaScript does: after white space, a sign and digits, up to the first
// other character.
func jsParseInt(s string) (int, bool) {
	s = strings.TrimLeft(s, " \t\n\r\f\v")
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	digits := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == digits {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	return n, err == nil
}

// c4ShapeWords are the words of a shape that $shape, $sprite and $tags
// can name, and the shapes of zuzu that draw them, as in Mermaid 12.3.
var c4ShapeWords = map[string]string{
	"person": "person", "box": "rounded", "rounded": "rounded", "cylinder": "cylinder", "database": "cylinder",
	"db": "cylinder", "queue": "h-cyl", "pipe": "h-cyl", "component": "subroutine",
}

// check resolves the shape of each element, reports the parts that zuzu
// does not draw, and checks that each relationship joins two parts of
// the diagram.
func (p *c4Diagram) check() error {
	c := p.c
	for _, e := range p.elems {
		c.line = e.line
		e.shape = c4Shape(e)
		if s := e.fields["shape"]; s != "" && c4ShapeWords[strings.ToLower(s)] == "" {
			if err := c.add("the shape %q of %s, which Mermaid does not draw either", s, e.alias); err != nil {
				return err
			}
		}
		if s := e.fields["sprite"]; s != "" && c4ShapeWords[strings.ToLower(s)] == "" {
			if err := c.add("the sprite %q of %s, which Mermaid does not draw either", s, e.alias); err != nil {
				return err
			}
		}
		if err := p.checkTags(e.fields["tags"], e.alias, true); err != nil {
			return err
		}
		if err := p.checkLink(e.fields, e.alias); err != nil {
			return err
		}
	}
	for _, b := range p.bounds[1:] {
		c.line = b.line
		if s := b.fields["sprite"]; s != "" {
			if err := c.add("the sprite %q of %s, which Mermaid does not draw either", s, b.alias); err != nil {
				return err
			}
		}
		if err := p.checkTags(b.fields["tags"], b.alias, false); err != nil {
			return err
		}
		if err := p.checkLink(b.fields, b.alias); err != nil {
			return err
		}
	}
	for _, r := range p.rels {
		c.line = r.line
		for _, end := range []string{r.from, r.to} {
			if _, ok := p.byAlias[end]; ok {
				continue
			}
			if b, ok := p.boundBy[end]; ok && b.alias != c4Global {
				continue
			}
			return c.errorf("a relationship from %s to %s: %s is not an element or a boundary", r.from, r.to, end)
		}
		name := r.from + " to " + r.to
		for _, k := range []string{"descr", "sprite"} {
			if s := r.fields[k]; s != "" {
				if err := c.add("the %s %q of the relationship from %s, which Mermaid does not draw either", k, s, name); err != nil {
					return err
				}
			}
		}
		if err := p.checkTags(r.fields["tags"], "the relationship from "+name, false); err != nil {
			return err
		}
		if err := p.checkLink(r.fields, "the relationship from "+name); err != nil {
			return err
		}
		for _, k := range []string{"offsetX", "offsetY"} {
			if s, ok := r.fields[k]; ok {
				if _, ok := jsParseInt(s); !ok {
					if err := c.add("the %s %q of the relationship from %s", k, s, name); err != nil {
						return err
					}
					delete(r.fields, k)
				}
			}
		}
	}
	return nil
}

// checkTags reports each tag that names no shape. Mermaid draws a tag
// only as the shape of an element.
func (p *c4Diagram) checkTags(tags, of string, shapes bool) error {
	if tags == "" {
		return nil
	}
	for tag := range strings.SplitSeq(tags, ",") {
		tag = strings.TrimSpace(tag)
		if shapes && c4ShapeWords[strings.ToLower(tag)] != "" {
			continue
		}
		if err := p.c.add("the tag %q of %s, which Mermaid does not draw either", tag, of); err != nil {
			return err
		}
	}
	return nil
}

// checkLink drops a link whose scheme zuzu does not write, with an issue.
func (p *c4Diagram) checkLink(fields map[string]string, of string) error {
	if l := fields["link"]; l != "" && !scene.LinkAllowed(l) {
		delete(fields, "link")
		return p.c.add("the link %s of %s, whose scheme zuzu does not write", l, of)
	}
	return nil
}

// c4Shape returns the shape of an element: the shape that $shape, $sprite
// or a tag names, or else the shape of its kind, as in Mermaid 12.3.
func c4Shape(e *c4Elem) string {
	for _, k := range []string{"shape", "sprite"} {
		if s := c4ShapeWords[strings.ToLower(e.fields[k])]; s != "" {
			return s
		}
	}
	if tags := e.fields["tags"]; tags != "" {
		for tag := range strings.SplitSeq(tags, ",") {
			if s := c4ShapeWords[strings.ToLower(strings.TrimSpace(tag))]; s != "" {
				return s
			}
		}
	}
	switch {
	case strings.HasSuffix(e.kind, "person"):
		return "person"
	case strings.HasSuffix(e.kind, "_db"):
		return "cylinder"
	case strings.HasSuffix(e.kind, "_queue"):
		return "h-cyl"
	}
	return "rounded"
}

// c4Stereotype returns the text in brackets under the name of an element,
// such as [Container: Java].
func c4Stereotype(e *c4Elem) string {
	base := strings.TrimPrefix(e.kind, "external_")
	base = strings.TrimSuffix(strings.TrimSuffix(base, "_db"), "_queue")
	name := map[string]string{"person": "Person", "system": "Software System", "container": "Container", "component": "Component"}[base]
	if t := e.fields["techn"]; t != "" {
		return "[" + name + ": " + t + "]"
	}
	return "[" + name + "]"
}
