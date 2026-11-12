package mermaid

import (
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// classDiagram reads the body of a class diagram, classDiagram or
// classDiagram-v2, into a graph. A class is a record of three fields: its
// name, its attributes and its methods. A namespace is a cluster.
type classDiagram struct {
	d      *Diagram
	g      *graph.Graph
	issues *graph.Issues
	// classes holds each class by its id.
	classes map[string]*class
	// namespaces holds each namespace by its name, and scopes the open
	// namespaces, the innermost last.
	namespaces map[string]*graph.Cluster
	scopes     []string
	// body is the class whose body the current line is in, or nil.
	body *class
	// clicks are the statements click, link and callback, which finish
	// applies.
	clicks []click
	// styles, nodeClasses and nodeStyles are the classes and the styles,
	// as in a flowchart.
	styles      map[string]map[string]string
	nodeClasses map[string][]string
	nodeStyles  map[string]map[string]string
	notes       int
	interfaces  int
	line        int
	acc         []string
	// hideEmpty draws a class with no members as its name alone, for the
	// setting class.hideEmptyMembersBox.
	hideEmpty bool
	// flat draws a namespace with dots in its name as one cluster with the
	// whole name, for the setting class.hierarchicalNamespaces: false.
	flat bool
}

// class is a class: its node, its name as it draws, its annotations, its
// attributes and its methods.
type class struct {
	node        *graph.Node
	name        string
	annotations []string
	attrs       []member
	methods     []member
}

// member is an attribute or a method of a class, as it draws, with its
// font: a line under a static member and italic for an abstract one.
type member struct {
	text string
	font graph.Font
}

// classArrows are the ends of a relation of a class diagram, by the
// arrowheads that draw them.
var classArrows = map[string]string{
	"<|": "empty", "|>": "empty", "*": "diamond", "o": "odiamond", "<": "vee", ">": "vee", "()": "odot",
}

// relationArrowSize scales the ends of a relation of a class diagram and
// of an entity relationship diagram, so that the marks of UML and of
// crow's foot notation read at the size of a label.
const relationArrowSize = 1.4

// classGraph reads a class diagram into a graph.
func classGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	c := &classDiagram{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		classes: map[string]*class{}, namespaces: map[string]*graph.Cluster{},
		styles: map[string]map[string]string{}, nodeClasses: map[string][]string{}, nodeStyles: map[string]map[string]string{},
	}
	if err := sectionConfig(d, c.issues, "class", "hideEmptyMembersBox", "hierarchicalNamespaces"); err != nil {
		return nil, c.issues.Report, err
	}
	if m, ok := d.Config["class"].(map[string]any); ok {
		c.hideEmpty, _ = m["hideEmptyMembersBox"].(bool)
		if v, ok := m["hierarchicalNamespaces"]; ok {
			b, ok := v.(bool)
			if !ok {
				if err := c.issues.Add("config", "the setting class.hierarchicalNamespaces: %v", v); err != nil {
					return nil, c.issues.Report, err
				}
			}
			c.flat = ok && !b
		}
	}
	if err := c.parse(); err != nil {
		return nil, c.issues.Report, err
	}
	// Mermaid wraps the label of a relation, and not a class or a note.
	wrapGraph(d.Config, c.g, 0)
	return c.g, c.issues.Report, nil
}

func (c *classDiagram) add(format string, args ...any) error {
	return c.issues.Add(fmt.Sprintf("line %d", c.line), format, args...)
}

func (c *classDiagram) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", c.line, fmt.Sprintf(format, args...), ErrSyntax)
}

// parse reads the lines of the body.
func (c *classDiagram) parse() error {
	inBody := func() bool { return c.body != nil }
	if err := readStatements(c.d, &c.line, &c.acc, inBody, c.bodyLine, c.stmt); err != nil {
		return err
	}
	switch {
	case c.body != nil:
		return fmt.Errorf("the class %s has no end brace: %w", c.body.node.ID, ErrSyntax)
	case len(c.scopes) > 0:
		return fmt.Errorf("the namespace %s has no end brace: %w", c.scopes[len(c.scopes)-1], ErrSyntax)
	}
	return c.finish()
}

// stmt reads one statement outside the body of a class.
func (c *classDiagram) stmt(t string) error {
	word, rest, _ := strings.Cut(t, " ")
	rest = strings.TrimSpace(rest)
	switch {
	case t == "}":
		if len(c.scopes) == 0 {
			return c.errorf("} with no namespace")
		}
		c.scopes = c.scopes[:len(c.scopes)-1]
		return nil
	case word == "direction":
		dir, err := direction(rest)
		if err != nil {
			return fmt.Errorf("line %d: %w", c.line, err)
		}
		c.g.Direction = dir
		return nil
	case word == "namespace":
		return c.namespace(rest)
	case word == "class":
		return c.declare(rest)
	case word == "classDef":
		names, def, _ := strings.Cut(rest, " ")
		st, err := css(def, c.add)
		if err != nil {
			return err
		}
		for n := range strings.SplitSeq(names, ",") {
			c.styles[strings.TrimSpace(n)] = st
		}
		return nil
	case word == "cssClass":
		ids, name, ok := strings.Cut(strings.TrimSpace(rest), " ")
		if !ok {
			return c.errorf("a cssClass with no class")
		}
		for id := range strings.SplitSeq(unquote(ids), ",") {
			if id = strings.TrimSpace(id); id != "" {
				c.ref(id)
				c.nodeClasses[id] = append(c.nodeClasses[id], strings.TrimSpace(name))
			}
		}
		return nil
	case word == "style":
		id, def, _ := strings.Cut(rest, " ")
		st, err := css(def, c.add)
		if err != nil {
			return err
		}
		c.ref(id)
		if c.nodeStyles[id] == nil {
			c.nodeStyles[id] = map[string]string{}
		}
		maps.Copy(c.nodeStyles[id], st)
		return nil
	case word == "note":
		return c.note(rest)
	case word == "click", word == "link", word == "callback":
		cl, err := parseClick(word, rest, c.line)
		if err != nil {
			return fmt.Errorf("line %d: %w", c.line, err)
		}
		c.clicks = append(c.clicks, cl)
		return nil
	case strings.HasPrefix(t, "<<"):
		end := strings.Index(t, ">>")
		if end < 0 {
			return c.errorf("the annotation %q", t)
		}
		id, _ := classID(strings.TrimSpace(t[end+2:]))
		if id == "" {
			return c.errorf("an annotation with no class")
		}
		cl := c.ref(id)
		cl.annotations = append(cl.annotations, strings.TrimSpace(t[2:end]))
		return nil
	}
	if ok, err := c.relation(t); ok || err != nil {
		return err
	}
	if head, text, ok := cutColon(t); ok {
		id, _ := classID(head)
		if id == "" {
			return c.errorf("%q is not a statement", t)
		}
		c.member(c.ref(id), text)
		return nil
	}
	id, cls := classID(t)
	if id == "" || strings.ContainsAny(id, " \t") {
		return c.errorf("%q is not a statement", t)
	}
	c.ref(id)
	c.class(id, cls)
	return nil
}

// cutColon cuts a line at its first colon outside quotes that is not a
// part of :::, the colon of a member or of the label of a relation.
func cutColon(t string) (string, string, bool) {
	for i := 0; i < len(t); i++ {
		switch {
		case t[i] == '"':
			if j := strings.IndexByte(t[i+1:], '"'); j >= 0 {
				i += j + 1
			}
		case strings.HasPrefix(t[i:], ":::"):
			i += 2
		case t[i] == ':':
			return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+1:]), true
		}
	}
	return t, "", false
}

// classID reads the id of a class with its generic type and its class
// of style: Name, Name~T~, `Name with spaces` or Name:::style. It returns
// the id without the generic type.
func classID(s string) (string, string) {
	s = strings.TrimSpace(s)
	s, cls, _ := strings.Cut(s, ":::")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") && len(s) >= 2 {
		return s[1 : len(s)-1], strings.TrimSpace(cls)
	}
	id, _, _ := strings.Cut(s, "~")
	return strings.TrimSpace(id), strings.TrimSpace(cls)
}

// generic writes the generic types of a text, ~T~, as <T>. A tilde after
// a letter or a digit and before one opens a type, and any other tilde
// closes the open type.
func generic(s string) string {
	if !strings.Contains(s, "~") {
		return s
	}
	word := func(b byte) bool {
		return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	}
	var b strings.Builder
	depth := 0
	for i := range len(s) {
		if s[i] != '~' {
			b.WriteByte(s[i])
			continue
		}
		switch {
		case i > 0 && i+1 < len(s) && word(s[i-1]) && word(s[i+1]):
			b.WriteByte('<')
			depth++
		case depth > 0:
			b.WriteByte('>')
			depth--
		default:
			b.WriteByte('~')
		}
	}
	return b.String()
}

// ref returns the class of an id, made in the current namespace when it is
// new.
func (c *classDiagram) ref(id string) *class {
	if cl, ok := c.classes[id]; ok {
		return cl
	}
	n := &graph.Node{ID: id, Label: id, Shape: "rect", Cluster: c.namespace0(), Style: map[string]string{}}
	cl := &class{node: n, name: id}
	c.classes[id] = cl
	c.g.Nodes = append(c.g.Nodes, n)
	return cl
}

// namespace0 returns the innermost open namespace, or empty.
func (c *classDiagram) namespace0() string {
	if len(c.scopes) == 0 {
		return ""
	}
	return c.scopes[len(c.scopes)-1]
}

// class adds a class of style to a class, for Name:::style.
func (c *classDiagram) class(id, cls string) {
	if cls != "" {
		c.nodeClasses[id] = append(c.nodeClasses[id], cls)
	}
}

// namespace opens a namespace, namespace Name {. A name with dots, such as
// A.B.C, is a namespace C in a namespace B in a namespace A, each a
// cluster with its own part of the name, as in Mermaid. With the setting
// class.hierarchicalNamespaces: false, it is one cluster with the whole
// name.
func (c *classDiagram) namespace(rest string) error {
	name, ok := strings.CutSuffix(rest, "{")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return c.errorf("the namespace %q", rest)
	}
	parts := strings.Split(name, ".")
	if c.flat || slices.Contains(parts, "") {
		parts = []string{name}
	}
	parent := c.namespace0()
	for i, part := range parts {
		id := strings.Join(parts[:i+1], ".")
		if _, ok := c.namespaces[id]; !ok {
			cl := &graph.Cluster{ID: id, Label: part, Parent: parent, Style: map[string]string{}}
			c.namespaces[id] = cl
			c.g.Clusters = append(c.g.Clusters, cl)
		}
		parent = id
	}
	c.scopes = append(c.scopes, name)
	return nil
}

// declare reads a line that starts with class: class Name, class
// Name~T~, class Name["Label"], class Name:::style, each with an optional
// body in braces. A class that a line before named moves into the
// current namespace.
func (c *classDiagram) declare(rest string) error {
	head, body, open := rest, "", false
	if i := braceAt(rest); i >= 0 {
		head, body, open = rest[:i], rest[i+1:], true
	}
	head = strings.TrimSpace(head)
	label := ""
	if i := strings.Index(head, "["); i >= 0 {
		if end := strings.LastIndex(head, "]"); end > i {
			label = unquote(head[i+1 : end])
			head = head[:i] + head[end+1:]
		}
	}
	id, cls := classID(head)
	if id == "" {
		return c.errorf("a class with no name")
	}
	cl := c.ref(id)
	if ns := c.namespace0(); ns != "" {
		cl.node.Cluster = ns
	}
	if name, _, _ := strings.Cut(head, ":::"); strings.Contains(name, "~") {
		cl.name = generic(strings.TrimSpace(name))
	}
	if label != "" {
		cl.name = label
	}
	c.class(id, cls)
	if !open {
		return nil
	}
	c.body = cl
	// A body can start, or start and end, on the line of the class.
	body = strings.TrimSpace(body)
	if inner, ok := strings.CutSuffix(body, "}"); ok {
		c.body = nil
		body = strings.TrimSpace(inner)
	}
	if body != "" {
		c.member(cl, body)
	}
	return nil
}

// braceAt returns the place of the brace that opens the body of a class,
// outside quotes and brackets, or -1.
func braceAt(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				return -1
			}
			i += j + 1
		case '[':
			depth++
		case ']':
			depth--
		case '{':
			if depth <= 0 {
				return i
			}
		}
	}
	return -1
}

// bodyLine reads a line in the body of a class: a member, an annotation,
// or the end brace.
func (c *classDiagram) bodyLine(t string) error {
	cl := c.body
	if inner, ok := strings.CutSuffix(t, "}"); ok {
		c.body = nil
		t = strings.TrimSpace(inner)
	}
	if t != "" {
		c.member(cl, t)
	}
	return nil
}

// member adds a member or an annotation to a class. A member with
// parentheses is a method. A $ marks a static member, which draws with a
// line under it, and a * marks an abstract one, which draws in italic.
func (c *classDiagram) member(cl *class, t string) {
	t = strings.TrimSpace(t)
	if strings.HasPrefix(t, "<<") && strings.HasSuffix(t, ">>") {
		cl.annotations = append(cl.annotations, strings.TrimSpace(t[2:len(t)-2]))
		return
	}
	var m member
	mark := func(s string) string {
		switch {
		case strings.HasSuffix(s, "$"):
			m.font.Underline = true
		case strings.HasSuffix(s, "*"):
			m.font.Italic = true
		default:
			return s
		}
		return strings.TrimSpace(s[:len(s)-1])
	}
	open, end := strings.Index(t, "("), strings.LastIndex(t, ")")
	if open < 0 || end < open {
		m.text = memberText(generic(mark(t)))
		cl.attrs = append(cl.attrs, m)
		return
	}
	// A method: the mark comes right after the parentheses or at the end,
	// and the return type after them.
	head, ret := t[:end+1], strings.TrimSpace(t[end+1:])
	if ret != "" && (ret[0] == '$' || ret[0] == '*') {
		mark(ret[:1])
		ret = strings.TrimSpace(ret[1:])
	}
	ret = mark(ret)
	m.text = generic(head)
	if ret != "" {
		m.text += " : " + generic(ret)
	}
	m.text = memberText(m.text)
	cl.methods = append(cl.methods, m)
}

// memberText reads the text of a member: as a label, with the entities
// of HTML, such as &lt;, as their characters, since Mermaid draws a member
// as HTML.
func memberText(s string) string {
	return html.UnescapeString(label(s))
}

// relation reads a relation, A "1" <|-- "many" B : label, and reports
// false for a line that is not a relation. The line of a relation is --
// or .., with an end on each side or none: <| or |>, *, o, < or >, and ()
// for a lollipop.
func (c *classDiagram) relation(t string) (bool, error) {
	at, dash := linkAt(t)
	if at < 0 {
		return false, nil
	}
	if _, _, ok := cutColon(t[:at]); ok {
		return false, nil
	}
	left, right := strings.TrimSpace(t[:at]), strings.TrimSpace(t[at+2:])
	tail, left := endBefore(left)
	head, right := endAfter(right)
	left, tailCard := cardinality(left, false)
	right, text, _ := cutColon(right)
	right, headCard := cardinality(right, true)
	from, fromCls := classID(left)
	to, toCls := classID(right)
	// Only a name in backticks holds a space.
	bad := func(raw, id string) bool {
		return id == "" || !strings.HasPrefix(raw, "`") && strings.ContainsAny(id, " \t\"")
	}
	if bad(left, from) || bad(right, to) {
		return true, c.errorf("the relation %q", t)
	}
	from, to = c.end(from, fromCls, tail), c.end(to, toCls, head)
	e := &graph.Edge{Source: from, Target: to, Label: label(generic(text)), Head: "none", Tail: "none",
		HeadLabel: headCard, TailLabel: tailCard, ArrowSize: relationArrowSize, Style: map[string]string{}}
	if a, ok := classArrows[head]; ok {
		e.Head = a
	}
	if a, ok := classArrows[tail]; ok {
		e.Tail = a
	}
	if dash {
		e.Style["strokeDash"] = dashed
	}
	c.g.Edges = append(c.g.Edges, e)
	return true, nil
}

// end returns the id of an end of a relation. At a lollipop, the end is an
// interface: a node of its own that draws its name with no box.
func (c *classDiagram) end(id, cls, mark string) string {
	if mark != "()" {
		c.ref(id)
		c.class(id, cls)
		return id
	}
	n := &graph.Node{ID: fmt.Sprintf("interface%d", c.interfaces), Label: id, Shape: "none", Cluster: c.namespace0(), Style: map[string]string{}}
	c.interfaces++
	c.g.Nodes = append(c.g.Nodes, n)
	return n.ID
}

// linkAt returns the place of the line of a relation, two hyphens or two
// dots outside quotes, or -1, and whether the line is dashed.
func linkAt(t string) (int, bool) {
	for i := 0; i+1 < len(t); i++ {
		switch {
		case t[i] == '"':
			j := strings.IndexByte(t[i+1:], '"')
			if j < 0 {
				return -1, false
			}
			i += j + 1
		case t[i] == '-' && t[i+1] == '-':
			return i, false
		case t[i] == '.' && t[i+1] == '.':
			return i, true
		}
	}
	return -1, false
}

// endBefore cuts the end of a relation off the text before its line. An
// end of o or * needs a space or a quote before it, so that it is not the
// last letter of a name.
func endBefore(s string) (string, string) {
	for _, e := range []string{"<|", "()", "<", "*", "o"} {
		rest, ok := strings.CutSuffix(s, e)
		if !ok {
			continue
		}
		if (e == "o" || e == "*") && rest != "" && !strings.HasSuffix(rest, " ") && !strings.HasSuffix(rest, `"`) {
			continue
		}
		return e, strings.TrimSpace(rest)
	}
	return "", s
}

// endAfter cuts the end of a relation off the text after its line.
func endAfter(s string) (string, string) {
	for _, e := range []string{"|>", "()", ">", "*", "o"} {
		rest, ok := strings.CutPrefix(s, e)
		if !ok {
			continue
		}
		if (e == "o" || e == "*") && rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, `"`) {
			continue
		}
		return e, strings.TrimSpace(rest)
	}
	return "", s
}

// cardinality cuts a quoted cardinality off the end of the text before
// the line of a relation, or off the start of the text after it.
func cardinality(s string, after bool) (string, string) {
	s = strings.TrimSpace(s)
	if after {
		if strings.HasPrefix(s, `"`) {
			if j := strings.IndexByte(s[1:], '"'); j >= 0 {
				return strings.TrimSpace(s[j+2:]), s[1 : j+1]
			}
		}
		return s, ""
	}
	if strings.HasSuffix(s, `"`) && len(s) >= 2 {
		if j := strings.LastIndexByte(s[:len(s)-1], '"'); j >= 0 {
			return strings.TrimSpace(s[:j]), s[j+1 : len(s)-1]
		}
	}
	return s, ""
}

// note reads a note: note "text", which floats, or note for Name "text",
// which has a dotted line to its class.
func (c *classDiagram) note(rest string) error {
	of := ""
	if r, ok := strings.CutPrefix(rest, "for "); ok {
		r = strings.TrimSpace(r)
		i := strings.IndexByte(r, '"')
		if i < 0 {
			return c.errorf("the note %q", rest)
		}
		of, _ = classID(r[:i])
		rest = r[i:]
	}
	text := strings.TrimSpace(rest)
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return c.errorf("the note %q", rest)
	}
	n := &graph.Node{ID: fmt.Sprintf("note%d", c.notes), Label: label(strings.ReplaceAll(text[1:len(text)-1], `\n`, "\n")),
		Shape: "note", Cluster: c.namespace0(), Style: map[string]string{}}
	c.notes++
	c.g.Nodes = append(c.g.Nodes, n)
	if of != "" {
		c.ref(of)
		c.g.Edges = append(c.g.Edges, &graph.Edge{Source: n.ID, Target: of, Head: "none", Style: map[string]string{"strokeDash": "2,3"}})
	}
	return nil
}

// finish writes the record of each class and applies the classes and the
// styles.
func (c *classDiagram) finish() error {
	err := applyClicks(c.issues, c.clicks, func(id string) *graph.Node {
		if cl, ok := c.classes[id]; ok {
			return cl.node
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, n := range c.g.Nodes {
		cl, ok := c.classes[n.ID]
		if !ok {
			continue
		}
		n.Label = cl.name
		var head []string
		var fonts []graph.Font
		// Each line of the name is bold, and the annotations above it are
		// not.
		for _, a := range cl.annotations {
			for l := range strings.SplitSeq("«"+a+"»", "\n") {
				head, fonts = append(head, l), append(fonts, graph.Font{})
			}
		}
		for l := range strings.SplitSeq(cl.name, "\n") {
			head, fonts = append(head, l), append(fonts, graph.Font{Bold: true})
		}
		n.Fields = []graph.Field{{Text: strings.Join(head, "\n"), Fonts: fonts}}
		if !c.hideEmpty || len(cl.attrs)+len(cl.methods) > 0 {
			n.Fields = append(n.Fields, compartment(cl.attrs), compartment(cl.methods))
		}
		n.Stack = true
	}
	for _, n := range c.g.Nodes {
		n.Style = classStyle(c.styles, c.nodeClasses[n.ID], c.nodeStyles[n.ID], n.Style)
	}
	c.g.Description = describe(c.d, c.acc...)
	return nil
}

// compartment returns the field of the attributes or of the methods of a
// class: a line for each, from the left.
func compartment(ms []member) graph.Field {
	f := graph.Field{Align: "l"}
	var texts []string
	for _, m := range ms {
		texts, f.Fonts = append(texts, m.text), append(f.Fonts, m.font)
	}
	f.Text = strings.Join(texts, "\n")
	return f
}
