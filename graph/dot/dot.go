// Package dot reads the DOT language of Graphviz into a graph (D31). It
// reads the subset that docs/DIAGRAMS.md lists, and reports the rest.
package dot

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// Error is an error of the package dot.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// ErrSyntax is the error of a source that is not DOT.
const ErrSyntax Error = "invalid DOT"

// token kinds.
const (
	tEOF = iota
	tID
	tPunct
	tEdge
)

type token struct {
	kind int
	text string
	// quoted says that the id came in quotes or as HTML, so it is never a
	// keyword.
	quoted, html bool
	line         int
}

// lex splits a source into tokens.
func lex(src string) ([]token, error) {
	var out []token
	line := 1
	i := 0
	atLineStart := true
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			atLineStart = true
			continue
		case c == ' ' || c == '\t' || c == '\r':
			i++
			continue
		case c == '#' && atLineStart:
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: a comment with no end: %w", line, ErrSyntax)
			}
			line += strings.Count(src[i:i+2+end], "\n")
			i += end + 4
			continue
		}
		atLineStart = false
		switch {
		case strings.HasPrefix(src[i:], "->") || strings.HasPrefix(src[i:], "--"):
			out = append(out, token{kind: tEdge, text: src[i : i+2], line: line})
			i += 2
		case strings.ContainsRune("{}[]=;,:", rune(c)):
			out = append(out, token{kind: tPunct, text: string(c), line: line})
			i++
		case c == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(src) && src[j] != '"'; j++ {
				if src[j] == '\\' && j+1 < len(src) && src[j+1] == '"' {
					b.WriteByte('"')
					j++
					continue
				}
				if src[j] == '\\' && j+1 < len(src) && src[j+1] == '\n' {
					j++
					line++
					continue
				}
				if src[j] == '\n' {
					line++
				}
				b.WriteByte(src[j])
			}
			if j >= len(src) {
				return nil, fmt.Errorf("line %d: a string with no end: %w", line, ErrSyntax)
			}
			s := b.String()
			i = j + 1
			// Strings joined with + are one string.
			if n := len(out); n >= 2 && out[n-1].kind == tPunct && out[n-1].text == "+" && out[n-2].quoted {
				out[n-2].text += s
				out = out[:n-1]
				continue
			}
			out = append(out, token{kind: tID, text: s, quoted: true, line: line})
		case c == '+':
			out = append(out, token{kind: tPunct, text: "+", line: line})
			i++
		case c == '<':
			depth, j := 0, i
			for ; j < len(src); j++ {
				if src[j] == '<' {
					depth++
				} else if src[j] == '>' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if j >= len(src) {
				return nil, fmt.Errorf("line %d: an HTML string with no end: %w", line, ErrSyntax)
			}
			out = append(out, token{kind: tID, text: src[i+1 : j], quoted: true, html: true, line: line})
			line += strings.Count(src[i:j], "\n")
			i = j + 1
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if !isIDRune(r) && r != '-' && r != '.' && !unicode.IsDigit(r) {
				return nil, fmt.Errorf("line %d: the character %q: %w", line, r, ErrSyntax)
			}
			j := i
			if r == '-' || r == '.' || unicode.IsDigit(r) {
				// A numeral.
				j += size
				for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '.') {
					j++
				}
			} else {
				for j < len(src) {
					r, size := utf8.DecodeRuneInString(src[j:])
					if !isIDRune(r) && !unicode.IsDigit(r) {
						break
					}
					j += size
				}
			}
			out = append(out, token{kind: tID, text: src[i:j], line: line})
			i = j
		}
	}
	return append(out, token{kind: tEOF, line: line}), nil
}

func isIDRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || r >= 0x80
}

// scope holds the defaults of a graph or a subgraph.
type scope struct {
	node, edge map[string]string
	cluster    string
	nodes      []string // the nodes that the subgraph names, in order
	// rank is the rank that a subgraph that starts here takes by default.
	rank string
	// scheme is the colorscheme of the graph or the subgraph, which a
	// subgraph takes from the graph around it.
	scheme string
	parent *scope
}

func (s *scope) child() *scope {
	c := &scope{node: map[string]string{}, edge: map[string]string{}, cluster: s.cluster, rank: s.rank, scheme: s.scheme, parent: s}
	maps.Copy(c.node, s.node)
	maps.Copy(c.edge, s.edge)
	return c
}

// parser reads tokens into a graph.
type parser struct {
	toks    []token
	pos     int
	g       *graph.Graph
	issues  *graph.Issues
	nodes   map[string]*graph.Node
	attrs   map[string]map[string]string
	strict  bool
	ranks   int
	edgeOps string
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func (p *parser) is(text string) bool {
	t := p.peek()
	return (t.kind == tPunct || t.kind == tEdge) && t.text == text
}

func (p *parser) keyword(word string) bool {
	t := p.peek()
	return t.kind == tID && !t.quoted && strings.EqualFold(t.text, word)
}

func (p *parser) expect(text string) error {
	t := p.next()
	if (t.kind != tPunct && t.kind != tEdge) || t.text != text {
		return fmt.Errorf("line %d: %q where %q goes: %w", t.line, t.text, text, ErrSyntax)
	}
	return nil
}

// Parse reads a source of DOT into a graph. In the loose mode the report
// names each part that zuzu does not draw, and in the strict mode such a
// part is an error.
func Parse(src string, strict bool) (*graph.Graph, vega.Report, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, nil, err
	}
	p := &parser{
		toks: toks, g: &graph.Graph{Direction: graph.TB}, issues: &graph.Issues{Strict: strict},
		nodes: map[string]*graph.Node{}, attrs: map[string]map[string]string{},
	}
	if p.keyword("strict") {
		p.next()
		p.strict = true
	}
	switch {
	case p.keyword("digraph"):
		p.g.Directed, p.edgeOps = true, "->"
	case p.keyword("graph"):
		p.edgeOps = "--"
	default:
		return nil, nil, fmt.Errorf("line %d: no graph or digraph: %w", p.peek().line, ErrSyntax)
	}
	p.next()
	if p.peek().kind == tID {
		p.next()
	}
	if err := p.expect("{"); err != nil {
		return nil, nil, err
	}
	root := &scope{node: map[string]string{}, edge: map[string]string{}}
	if err := p.stmts(root, nil); err != nil {
		return nil, nil, err
	}
	if t := p.peek(); t.kind != tEOF {
		return nil, nil, fmt.Errorf("line %d: %q after the graph: %w", t.line, t.text, ErrSyntax)
	}
	if err := p.finish(); err != nil {
		return nil, nil, err
	}
	return p.g, p.issues.Report, nil
}

// stmts reads statements to the closing brace of a scope.
func (p *parser) stmts(s *scope, graphAttrs map[string]string) error {
	if graphAttrs == nil {
		graphAttrs = map[string]string{}
		defer func() { _ = p.graphAttrs(graphAttrs) }()
	}
	for {
		if p.is("}") {
			p.next()
			return nil
		}
		if p.peek().kind == tEOF {
			return fmt.Errorf("line %d: a graph with no closing brace: %w", p.peek().line, ErrSyntax)
		}
		if p.is(";") {
			p.next()
			continue
		}
		if err := p.stmt(s, graphAttrs); err != nil {
			return err
		}
	}
}

// stmt reads one statement.
func (p *parser) stmt(s *scope, graphAttrs map[string]string) error {
	if p.keyword("graph") || p.keyword("node") || p.keyword("edge") {
		kind := strings.ToLower(p.next().text)
		a, err := p.attrList()
		if err != nil {
			return err
		}
		switch kind {
		case "graph":
			maps.Copy(graphAttrs, a)
			if r, ok := a["rank"]; ok {
				s.rank = r
			}
			if v, ok := a["colorscheme"]; ok {
				s.scheme = v
			}
		case "node":
			maps.Copy(s.node, a)
		case "edge":
			maps.Copy(s.edge, a)
		}
		return nil
	}
	t := p.peek()
	if t.kind == tID && p.toks[p.pos+1].kind == tPunct && p.toks[p.pos+1].text == "=" {
		p.next()
		p.next()
		v := p.next()
		graphAttrs[t.text] = v.text
		if v.html {
			graphAttrs[t.text] = htmlMark + v.text
		}
		if t.text == "rank" {
			s.rank = v.text
		}
		if t.text == "colorscheme" {
			s.scheme = v.text
		}
		return nil
	}
	ends, err := p.operand(s)
	if err != nil {
		return err
	}
	if p.peek().kind != tEdge {
		if len(ends) == 1 && ends[0].sub == nil {
			a, err := p.attrList()
			if err != nil {
				return err
			}
			p.node(s, ends[0].id, a)
		}
		return nil
	}
	chain := [][]end{ends}
	for p.peek().kind == tEdge {
		op := p.next()
		if op.text != p.edgeOps {
			return fmt.Errorf("line %d: the edge %s in a graph of %s: %w", op.line, op.text, p.edgeOps, ErrSyntax)
		}
		e, err := p.operand(s)
		if err != nil {
			return err
		}
		chain = append(chain, e)
	}
	a, err := p.attrList()
	if err != nil {
		return err
	}
	attrs := maps.Clone(s.edge)
	maps.Copy(attrs, a)
	for i := 1; i < len(chain); i++ {
		for _, from := range chain[i-1] {
			for _, to := range chain[i] {
				if err := p.edge(from, to, attrs, t.line); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// end is an end of an edge: a node with its port.
type end struct {
	id, port string
	sub      *scope
}

// operand reads a node or a subgraph, and returns its nodes.
func (p *parser) operand(s *scope) ([]end, error) {
	if p.keyword("subgraph") || p.is("{") {
		sub, err := p.subgraph(s)
		if err != nil {
			return nil, err
		}
		var out []end
		for _, id := range sub.nodes {
			out = append(out, end{id: id, sub: sub})
		}
		return out, nil
	}
	t := p.next()
	if t.kind != tID {
		return nil, fmt.Errorf("line %d: %q where a node goes: %w", t.line, t.text, ErrSyntax)
	}
	e := end{id: t.text}
	if p.is(":") {
		p.next()
		e.port = p.next().text
		if p.is(":") {
			p.next()
			e.port += ":" + p.next().text
		}
	}
	p.node(s, e.id, nil)
	return []end{e}, nil
}

// subgraph reads a subgraph: a cluster when its name starts with cluster.
func (p *parser) subgraph(s *scope) (*scope, error) {
	name := ""
	if p.keyword("subgraph") {
		p.next()
		if p.peek().kind == tID {
			name = p.next().text
		}
	}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	sub := s.child()
	// A subgraph takes the rank of the graph that holds it at the time
	// that it starts, as in dot, unless it sets its own.
	inherited := s.rank
	var cl *graph.Cluster
	if strings.HasPrefix(name, "cluster") {
		cl = &graph.Cluster{ID: name, Parent: s.cluster, Style: map[string]string{}}
		p.g.Clusters = append(p.g.Clusters, cl)
		sub.cluster = name
	}
	attrs := map[string]string{}
	if err := p.stmts(sub, attrs); err != nil {
		return nil, err
	}
	if cl != nil {
		var err error
		// A cluster takes the colorscheme of the graph around it.
		if sub.scheme != "" {
			attrs["colorscheme"] = sub.scheme
		}
		if cl.Label, cl.HTML, err = p.label(attrs["label"], name, sub.scheme, "subgraph "+name); err != nil {
			return nil, err
		}
		if attrs["label"] == "" {
			cl.Label = ""
		}
		if err := p.paint(cl.Style, attrs, true, "line "+strconv.Itoa(p.peek().line)); err != nil {
			return nil, err
		}
	}
	if r := cmp.Or(attrs["rank"], inherited); r != "" {
		switch r {
		case "same":
			p.ranks++
			g := fmt.Sprintf("same%d", p.ranks)
			for _, id := range sub.nodes {
				p.nodes[id].Rank = g
			}
		default:
			if err := p.issues.Add("subgraph "+name, "the rank %s", r); err != nil {
				return nil, err
			}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		switch k {
		case "label", "rank", "style", "color", "fillcolor", "bgcolor", "fontcolor", "penwidth", "pencolor", "gradientangle", "colorscheme":
		default:
			if err := p.issues.Add("subgraph "+name, "the attribute %s of a subgraph", k); err != nil {
				return nil, err
			}
		}
	}
	for _, id := range sub.nodes {
		if !slices.Contains(s.nodes, id) {
			s.nodes = append(s.nodes, id)
		}
	}
	return sub, nil
}

// attrList reads lists of attributes, [a=b, c=d][e=f].
func (p *parser) attrList() (map[string]string, error) {
	a := map[string]string{}
	for p.is("[") {
		p.next()
		for !p.is("]") {
			k := p.next()
			if k.kind != tID {
				return nil, fmt.Errorf("line %d: %q where an attribute goes: %w", k.line, k.text, ErrSyntax)
			}
			v := "true"
			if p.is("=") {
				p.next()
				vt := p.next()
				v = vt.text
				if vt.html {
					v = htmlMark + v
				}
			}
			a[k.text] = v
			if p.is(",") || p.is(";") {
				p.next()
			}
		}
		p.next()
	}
	return a, nil
}

// within reports whether a cluster is inside another cluster, or the other
// is the graph, named by the empty string.
func (p *parser) within(cluster, outer string) bool {
	switch {
	case outer == "":
		return true
	case cluster == outer || cluster == "":
		return false
	}
	for _, c := range p.g.Clusters {
		if c.ID == cluster {
			return c.Parent == outer || p.within(c.Parent, outer)
		}
	}
	return false
}

// node makes a node, or adds attributes to it, with the defaults of its
// scope when it is new.
func (p *parser) node(s *scope, id string, a map[string]string) {
	attrs, ok := p.attrs[id]
	if !ok {
		attrs = maps.Clone(s.node)
		p.attrs[id] = attrs
		n := &graph.Node{ID: id, Cluster: s.cluster, Style: map[string]string{}}
		p.nodes[id] = n
		p.g.Nodes = append(p.g.Nodes, n)
	} else if n := p.nodes[id]; s.cluster != "" && p.within(s.cluster, n.Cluster) {
		// A cluster that names a node made outside it takes the node in, as
		// dot does.
		n.Cluster = s.cluster
	}
	maps.Copy(attrs, a)
	for sc := s; sc != nil; sc = sc.parent {
		if !slices.Contains(sc.nodes, id) {
			sc.nodes = append(sc.nodes, id)
		}
	}
}

// edge adds an edge.
func (p *parser) edge(from, to end, a map[string]string, line int) error {
	e := &graph.Edge{Source: from.id, Target: to.id, SourcePort: from.port, TargetPort: to.port, Style: map[string]string{}}
	path := "line " + strconv.Itoa(line)
	if p.strict {
		for _, x := range p.g.Edges {
			if x.Source == e.Source && x.Target == e.Target || !p.g.Directed && x.Source == e.Target && x.Target == e.Source {
				return nil
			}
		}
	}
	dir := "forward"
	if !p.g.Directed {
		dir = "none"
	}
	labels := map[string]*[2]string{
		"label": {}, "headlabel": {}, "taillabel": {}, "xlabel": {},
	}
	placed := false
	e.LabelDistance, e.LabelAngle = 1, -25
	for _, k := range slices.Sorted(maps.Keys(a)) {
		v := a[k]
		switch k {
		case "label", "headlabel", "taillabel", "xlabel":
			var err error
			l := labels[k]
			if l[0], l[1], err = p.label(v, "", a["colorscheme"], path); err != nil {
				return err
			}
		case "labeldistance", "labelangle", "arrowsize", "labelfontsize":
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				if err := p.issues.Add(path, "the %s %s", k, v); err != nil {
					return err
				}
				continue
			}
			switch k {
			case "labeldistance":
				// A distance of 0 puts the label on the end, as dot does.
				e.LabelDistance, placed = math.Max(f, 0.01), true
			case "labelangle":
				e.LabelAngle, placed = f, true
			case "arrowsize":
				e.ArrowSize = f
			case "labelfontsize":
				e.LabelStyle = labelStyle(e.LabelStyle, "fontSize", v)
			}
		case "labelfontcolor":
			c, ok := graph.ColorIn(v, a["colorscheme"])
			if !ok {
				if err := p.issues.Add(path, "the color %s", v); err != nil {
					return err
				}
				continue
			}
			e.LabelStyle = labelStyle(e.LabelStyle, "color", c)
		case "dir":
			dir = v
		case "arrowhead":
			e.Head = v
		case "arrowtail":
			e.Tail = v
		case "weight":
			e.Weight, _ = strconv.ParseFloat(v, 64)
		case "minlen":
			m, _ := strconv.ParseFloat(v, 64)
			e.MinLen = int(m)
		case "constraint":
			e.NoConstraint = v == "false" || v == "no" || v == "0"
		case "headport":
			e.TargetPort = v
		case "tailport":
			e.SourcePort = v
		case "color", "penwidth", "style", "fontcolor", "fontsize", "colorscheme":
		default:
			if err := p.issues.Add(path, "the attribute %s of an edge", k); err != nil {
				return err
			}
		}
	}
	if !placed {
		e.LabelDistance, e.LabelAngle = 0, 0
	}
	e.Label, e.HTML = labels["label"][0], labels["label"][1]
	e.HeadLabel, e.HeadHTML = labels["headlabel"][0], labels["headlabel"][1]
	e.TailLabel, e.TailHTML = labels["taillabel"][0], labels["taillabel"][1]
	e.XLabel, e.XHTML = labels["xlabel"][0], labels["xlabel"][1]
	if err := p.paint(e.Style, a, false, path); err != nil {
		return err
	}
	if a["style"] == "invis" {
		e.Style["stroke"] = "transparent"
		if e.LabelStyle != nil {
			e.LabelStyle["color"] = "transparent"
		}
		e.Style["color"] = "transparent"
	}
	head, tail := e.Head, e.Tail
	if head == "" {
		head = "normal"
	}
	if tail == "" {
		tail = "normal"
	}
	switch dir {
	case "forward":
		e.Head, e.Tail = head, "none"
	case "back":
		e.Head, e.Tail = "none", tail
	case "both":
		e.Head, e.Tail = head, tail
	default:
		e.Head, e.Tail = "none", "none"
	}
	for _, k := range []string{e.Head, e.Tail} {
		if !graph.ArrowOf(k) {
			if err := p.issues.Add(path, "the arrowhead %s", k); err != nil {
				return err
			}
		}
	}
	p.g.Edges = append(p.g.Edges, e)
	return nil
}

// graphAttrs reads the attributes of the graph.
func (p *parser) graphAttrs(a map[string]string) error {
	for _, k := range slices.Sorted(maps.Keys(a)) {
		v := a[k]
		switch k {
		case "label":
			var err error
			if p.g.Title, p.g.TitleHTML, err = p.label(v, "", a["colorscheme"], "graph"); err != nil {
				return err
			}
		case "colorscheme":
		case "rankdir":
			switch strings.ToUpper(v) {
			case "TB", "BT", "LR", "RL":
				p.g.Direction = strings.ToUpper(v)
			}
		case "ranksep":
			if f, err := strconv.ParseFloat(strings.Fields(v + " ")[0], 64); err == nil {
				p.g.RankSep = f * 72
			}
		case "nodesep":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				p.g.NodeSep = f * 72
			}
		case "splines":
			if v != "true" && v != "spline" && v != "" {
				if err := p.issues.Add("graph", "the splines %s", v); err != nil {
					return err
				}
			}
		case "rank":
			// The rank of the graph is the rank of each subgraph that
			// starts after it, and the subgraph reads it.
		case "newrank":
			// zuzu ranks the nodes of every cluster with those of the graph,
			// as dot does with newrank=true.
		case "layout":
			if v != "dot" {
				if err := p.issues.Add("graph", "the layout %s", v); err != nil {
					return err
				}
			}
		default:
			if err := p.issues.Add("graph", "the attribute %s of a graph", k); err != nil {
				return err
			}
		}
	}
	return nil
}

// finish sets the label, the shape and the paint of each node from its
// attributes.
func (p *parser) finish() error {
	for _, n := range p.g.Nodes {
		a := p.attrs[n.ID]
		path := "node " + n.ID
		n.Label = label(a["label"], n.ID)
		if a["label"] == "" {
			n.Label = n.ID
		}
		n.Shape = "ellipse"
		if s, ok := a["shape"]; ok {
			n.Shape = s
		}
		src, isHTMLLabel := isHTML(a["label"])
		if n.Shape == "record" || n.Shape == "Mrecord" {
			switch {
			case isHTMLLabel && len(splitRecord(src)) == 1:
				// A label of HTML with no brace and no bar, such as a
				// table, draws in the shape of the record, as one field.
			case isHTMLLabel:
				// The braces and the bars split a label of HTML into the
				// fields of the record, and the text of each field is
				// HTML.
				fields, err := p.htmlRecord(src, a["colorscheme"], path)
				if err != nil {
					return err
				}
				n.Fields, isHTMLLabel = fields, false
			default:
				n.Fields = record(a["label"], n.ID)
			}
			// The fields of one group in braces lie one under another
			// when the ranks run down. When the ranks run across, they
			// lie one under another too, and not side by side as in
			// Graphviz.
			n.Stack = wholeGroup(src) && (p.g.Direction == graph.TB || p.g.Direction == graph.BT)
			var parts []string
			for _, f := range n.Fields {
				parts = append(parts, f.Text)
			}
			n.Label = strings.Join(parts, " | ")
		}
		if n.Shape == "polygon" {
			s, err := p.polygon(a, path)
			if err != nil {
				return err
			}
			n.Shape = s
		}
		if _, ok := graph.ShapeOf(n.Shape); !ok {
			if err := p.issues.Add(path, "the shape %s", n.Shape); err != nil {
				return err
			}
		}
		if isHTMLLabel {
			l, err := p.html(src, a["colorscheme"], path)
			if err != nil {
				return err
			}
			n.HTML, n.Label, n.Fields = graph.HTMLInScheme(src, a["colorscheme"]), htmlText(l), nil
		}
		if v, ok := a["xlabel"]; ok {
			var err error
			if n.XLabel, n.XHTML, err = p.label(v, n.ID, a["colorscheme"], path); err != nil {
				return err
			}
		}
		if v, ok := a["margin"]; ok {
			m, err := margin(v)
			if err != nil {
				if err := p.issues.Add(path, "the margin %s", v); err != nil {
					return err
				}
			}
			n.Margin = m
		} else if a["shape"] == "plain" {
			n.Margin = []float64{0, 0}
		}
		for _, k := range slices.Sorted(maps.Keys(a)) {
			switch k {
			case "label", "shape", "style", "color", "fillcolor", "fontcolor", "fontsize", "penwidth", "width", "height", "fixedsize", "margin", "xlabel", "gradientangle", "colorscheme":
			case "sides", "skew", "distortion":
				// Only a polygon reads these. Graphviz draws every other
				// shape without them.
			case "orientation", "regular":
				// A polygon reads these. Graphviz turns another shape too,
				// or makes it regular, and zuzu does not.
				if a["shape"] == "polygon" || (k == "regular" && a[k] == "false") {
					break
				}
				if x, err := strconv.ParseFloat(a[k], 64); k == "orientation" && err == nil && x == 0 {
					break
				}
				if err := p.issues.Add(path, "the attribute %s of a node of the shape %s", k, n.Shape); err != nil {
					return err
				}
			default:
				if err := p.issues.Add(path, "the attribute %s of a node", k); err != nil {
					return err
				}
			}
		}
		if err := p.paint(n.Style, a, true, path); err != nil {
			return err
		}
		// width and height are inches. With fixedsize true or shape, they
		// are the size of the node. Without it, they are its least size,
		// and the node grows to fit its label.
		w, _ := strconv.ParseFloat(a["width"], 64)
		h, _ := strconv.ParseFloat(a["height"], 64)
		switch a["fixedsize"] {
		case "true", "shape":
			n.Width, n.Height = w*72, h*72
		default:
			n.MinWidth, n.MinHeight = w*72, h*72
		}
		if strings.Contains(a["style"], "invis") {
			n.Style["fill"], n.Style["stroke"], n.Style["color"] = "transparent", "transparent", "transparent"
		}
		if strings.Contains(a["style"], "rounded") && (n.Shape == "box" || n.Shape == "rect" || n.Shape == "rectangle") {
			n.Shape = "rounded"
		}
	}
	return nil
}

// polygon returns the name of the shape of a node of the shape polygon,
// from its attributes sides, skew, distortion, orientation and regular. A
// value that is not a number is an issue, and draws as the default.
func (p *parser) polygon(a map[string]string, path string) (string, error) {
	sides, skew, distortion, orientation := 4, 0.0, 0.0, 0.0
	if v, ok := a["sides"]; ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			if err := p.issues.Add(path, "the sides %s", v); err != nil {
				return "", err
			}
		} else {
			sides = n
		}
	}
	for _, f := range []struct {
		name string
		v    *float64
	}{{"skew", &skew}, {"distortion", &distortion}, {"orientation", &orientation}} {
		v, ok := a[f.name]
		if !ok {
			continue
		}
		x, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			if err := p.issues.Add(path, "the %s %s", f.name, v); err != nil {
				return "", err
			}
			continue
		}
		*f.v = x
	}
	return graph.Polygon(sides, skew, distortion, orientation, a["regular"] == "true"), nil
}

// paint sets the paint of a node, an edge or a cluster from the attributes
// of DOT: the color is the outline, the fill color fills a filled one, and
// the font color colors the label.
func (p *parser) paint(st, a map[string]string, fills bool, path string) error {
	// err keeps the first issue of a color, which is an error in the
	// strict mode.
	var err error
	col := func(v string) (string, bool) {
		c, ok := graph.ColorIn(v, a["colorscheme"])
		if !ok && err == nil {
			err = p.issues.Add(path, "the color %s", v)
		}
		return c, ok
	}
	if v := a["color"]; v != "" {
		if c, ok := col(v); ok {
			st["stroke"] = c
		}
	}
	if v := a["pencolor"]; v != "" {
		if c, ok := col(v); ok {
			st["stroke"] = c
		}
	}
	if fills {
		// style=radial fills too, with a radial gradient.
		radial := hasStyle(a["style"], "radial")
		filled := strings.Contains(a["style"], "filled") || radial
		v := a["fillcolor"]
		if v == "" {
			v = a["bgcolor"]
		}
		if v == "" && filled {
			v = a["color"]
		}
		switch {
		case v != "":
			if c, ok := col(v); ok {
				st["fill"], st["fillOpacity"] = c, "1"
				if gerr := p.gradient(st, v, a, radial, path); gerr != nil {
					return gerr
				}
			}
		case filled:
			st["fill"], st["fillOpacity"] = "lightgrey", "1"
		}
	}
	if v := a["fontcolor"]; v != "" {
		if c, ok := col(v); ok {
			st["color"] = c
		}
	}
	if v := a["fontsize"]; v != "" {
		st["fontSize"] = v
	}
	style := a["style"]
	// setlinewidth(n) in a style is the old form of penwidth. A penwidth
	// wins over it.
	if _, rest, ok := strings.Cut(style, "setlinewidth("); ok {
		if v, _, ok := strings.Cut(rest, ")"); ok {
			st["strokeWidth"] = strings.TrimSpace(v)
		}
	}
	if v := a["penwidth"]; v != "" {
		st["strokeWidth"] = v
	}
	switch {
	case strings.Contains(style, "dashed"):
		st["strokeDash"] = "5,4"
	case strings.Contains(style, "dotted"):
		st["strokeDash"] = "1,3"
	}
	if strings.Contains(style, "bold") && a["penwidth"] == "" {
		st["strokeWidth"] = "2"
	}
	return err
}

// hasStyle reports whether a style of DOT, a list split by commas or
// spaces, holds a word.
func hasStyle(style, word string) bool {
	return slices.Contains(strings.FieldsFunc(style, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }), word)
}

// gradient sets the gradient of a fill of two colors, `a:b` or `a;0.3:b`, with
// the angle of gradientangle. Each color of the list must read, and the
// gradient holds them as CSS, with their weights. A color after the second
// and a weight that does not read are issues.
func (p *parser) gradient(st map[string]string, v string, a map[string]string, radial bool, path string) error {
	colors, weights, ok := graph.ColorList(v)
	if !ok {
		if err := p.issues.Add(path, "the weights of the colors %s", v); err != nil {
			return err
		}
	}
	if len(colors) < 2 {
		return nil
	}
	if len(colors) > 2 {
		if err := p.issues.Add(path, "the colors after the second of the gradient %s", v); err != nil {
			return err
		}
	}
	list := make([]string, 2)
	for i := range list {
		c, ok := graph.ColorIn(colors[i], a["colorscheme"])
		if !ok {
			return p.issues.Add(path, "the color %s", colors[i])
		}
		list[i] = c
		if weights[i] >= 0 {
			list[i] += ";" + strconv.FormatFloat(weights[i], 'g', -1, 64)
		}
	}
	st["gradient"] = strings.Join(list, ":")
	if radial {
		st["gradientType"] = "radial"
	}
	if g := a["gradientangle"]; g != "" {
		if _, err := strconv.ParseFloat(g, 64); err != nil {
			return p.issues.Add(path, "the gradientangle %s", g)
		}
		st["gradientAngle"] = g
	}
	return nil
}

// labelStyle sets a key of the style of the labels at the ends of an
// edge, and makes the style when it is nil.
func labelStyle(st map[string]string, k, v string) map[string]string {
	if st == nil {
		st = map[string]string{}
	}
	st[k] = v
	return st
}

// label reads a label of DOT: \n, \l and \r end a line, and \N names the
// node.
func label(v, id string) string {
	r := strings.NewReplacer(`\n`, "\n", `\l`, "\n", `\r`, "\n", `\N`, id, `\G`, "", `\"`, `"`, `\ `, " ")
	return strings.TrimRight(r.Replace(v), "\n")
}

// margin reads the margin of a node, x or x,y in inches, as points.
func margin(v string) ([]float64, error) {
	f := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
	if len(f) == 0 || len(f) > 2 {
		return nil, ErrSyntax
	}
	var out []float64
	for _, s := range f {
		x, err := strconv.ParseFloat(s, 64)
		if err != nil || x < 0 {
			return nil, ErrSyntax
		}
		out = append(out, x*72)
	}
	if len(out) == 1 {
		out = append(out, out[0])
	}
	return out, nil
}

// record reads the fields of the label of a record, {a|<p>b|c}, flat: a
// field of a nested group is a field of the record.
func record(v, id string) []graph.Field {
	if v == "" {
		v = id
	}
	// An escaped bracket or bar is text, so it hides in a private rune
	// while the fields split.
	hide := strings.NewReplacer(`\{`, "\ue001", `\}`, "\ue002", `\|`, "\ue003", `\<`, "\ue004", `\>`, "\ue005")
	show := strings.NewReplacer("\ue001", "{", "\ue002", "}", "\ue003", "|", "\ue004", "<", "\ue005", ">")
	v = strings.NewReplacer("{", "|", "}", "|").Replace(hide.Replace(v))
	var out []graph.Field
	for part := range strings.SplitSeq(v, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := graph.Field{}
		if strings.HasPrefix(part, "<") {
			if j := strings.Index(part, ">"); j > 0 {
				f.Port = part[1:j]
				part = strings.TrimSpace(part[j+1:])
			}
		}
		f.Text = show.Replace(label(part, id))
		out = append(out, f)
	}
	return out
}
