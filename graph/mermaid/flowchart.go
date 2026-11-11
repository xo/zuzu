package mermaid

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/xo/zuzu/graph"
)

// flowchart reads the body of a flowchart.
type flowchart struct {
	d       *Diagram
	g       *graph.Graph
	issues  *graph.Issues
	nodes   map[string]*graph.Node
	classes map[string]map[string]string
	// nodeClasses and nodeStyles are the classes and the styles of each
	// node, in order.
	nodeClasses map[string][]string
	nodeStyles  map[string]map[string]string
	edgeIDs     map[string]*graph.Edge
	linkStyles  map[int]map[string]string
	linkDefault map[string]string
	linkCurves  map[int]string
	clusters    []*graph.Cluster
	curve       string
	line        int
	anon        int
	// wrap is how the labels wrap, and nodeWrap the width at which the
	// label of a node wraps. markdown holds the nodes whose label is a
	// string of markdown.
	wrap     wrapping
	nodeWrap float64
	markdown map[string]bool
	// edgeClasses are the classes of each edge by its id, in order.
	edgeClasses map[string][]string
	// collapsed holds the line of each @{ view: collapsed } by the id that
	// it names.
	collapsed map[string]int
	// head holds the statements after the direction on the first line,
	// with a semicolon between them.
	head []string
	// cont is the last group of nodes of a chain that ends a line, which
	// a line that starts with a link goes on from. pending is a link at
	// the end of a line, which goes to the first group of nodes of the
	// next line.
	cont    []string
	pending *pendingLink
	// acc holds the texts of accTitle and accDescr, which join into the
	// description of the graph.
	acc []string
}

// pendingLink is a link at the end of a line and the group of nodes that
// it goes from.
type pendingLink struct {
	from []string
	l    *link
}

func newFlowchart(d *Diagram, strict bool) *flowchart {
	return &flowchart{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		nodes: map[string]*graph.Node{}, classes: map[string]map[string]string{}, nodeClasses: map[string][]string{},
		nodeStyles: map[string]map[string]string{}, edgeIDs: map[string]*graph.Edge{},
		linkStyles: map[int]map[string]string{}, linkCurves: map[int]string{},
		nodeWrap: nodeWrap, markdown: map[string]bool{}, edgeClasses: map[string][]string{},
		collapsed: map[string]int{},
	}
}

// curves are the curves of Mermaid that Vega draws, by their names in
// Vega.
var curves = map[string]string{
	"basis": "basis", "linear": "linear", "cardinal": "cardinal", "catmullRom": "catmull-rom",
	"monotoneX": "monotone", "natural": "natural", "step": "step", "stepAfter": "step-after", "stepBefore": "step-before",
}

func (f *flowchart) add(format string, args ...any) error {
	return f.issues.Add(fmt.Sprintf("line %d", f.line), format, args...)
}

// config reads the config of the front matter and the directives.
func (f *flowchart) config() error {
	cfg := f.d.Config
	for _, k := range slices.Sorted(maps.Keys(cfg)) {
		v := cfg[k]
		if ok, err := lookSetting(f.issues, f.d.Kind, k, v); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		var read []string
		if k == "flowchart" {
			read = flowSettings
		}
		if err := graphSection(f.issues, f.d.Kind, k, v, read...); err != nil {
			return err
		}
	}
	if err := f.spacing(); err != nil {
		return err
	}
	f.wrap = labelWrap(cfg, f.g)
	var err error
	if f.nodeWrap, err = wrapSetting(f.issues, cfg, nodeWrap, "flowchart", "wrappingWidth"); err != nil {
		return err
	}
	if c := str(cfg, "flowchart", "curve"); c != "" {
		vc, ok := curves[c]
		if !ok {
			if err := f.issues.Add("config", "the curve %s", c); err != nil {
				return err
			}
		}
		f.curve = vc
	}
	head := statements(f.d.Header)
	f.head = head[1:]
	if h := strings.Fields(head[0]); len(h) > 0 {
		dir, err := direction(h[0])
		if err != nil {
			return fmt.Errorf("line %d: %w", f.d.N, err)
		}
		f.g.Direction = dir
	}
	return nil
}

// direction reads a direction: TB, TD, BT, LR or RL.
func direction(s string) (string, error) {
	dir := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(s), ";"))
	switch dir {
	case "TD":
		return graph.TB, nil
	case graph.TB, graph.BT, graph.LR, graph.RL:
		return dir, nil
	}
	return "", fmt.Errorf("the direction %s: %w", s, ErrSyntax)
}

// entityEnd matches the start of an entity, such as #quot or #35, at the
// end of a text, so that the semicolon after it does not end a statement.
var entityEnd = regexp.MustCompile(`#(?:[A-Za-z]+|\d+)$`)

// statements splits a line at each semicolon outside quotes, brackets and
// the text of a link between pipes. The semicolon at the end of an
// entity, such as #quot, does not split.
func statements(s string) []string {
	var out []string
	depth, quote, pipe, start := 0, false, false, 0
	for i := range len(s) {
		switch c := s[i]; {
		case c == '"':
			quote = !quote
		case quote:
		case c == '|':
			pipe = !pipe
		case pipe:
		case c == '[' || c == '(' || c == '{':
			depth++
		case c == ']' || c == ')' || c == '}':
			depth--
		case c == ';' && depth <= 0 && !entityEnd.MatchString(s[start:i]):
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// parse reads the statements of the body. A text in double quotes, such
// as a string of markdown, can run over more lines, and each new line in it
// starts a new line of the label.
func (f *flowchart) parse() error {
	lines, err := f.accessibility(f.d.Lines)
	if err != nil {
		return err
	}
	if len(f.head) > 0 {
		lines = append([]Line{{Text: strings.Join(f.head, ";"), N: f.d.N}}, lines...)
	}
	for _, l := range joinOpen(lines, `"`) {
		f.line = l.N
		var sts []string
		for _, st := range statements(l.Text) {
			if st = strings.TrimSpace(st); st != "" {
				sts = append(sts, st)
			}
		}
		for i, st := range sts {
			if err := f.stmt(st, i == 0, i == len(sts)-1); err != nil {
				return err
			}
		}
	}
	if f.pending != nil {
		return fmt.Errorf("line %d: a link with no node after it: %w", f.line, ErrSyntax)
	}
	if len(f.clusters) > 0 {
		return fmt.Errorf("a subgraph with no end: %w", ErrSyntax)
	}
	return f.finish()
}

// accessibility takes the lines of accTitle and accDescr out of the
// lines, and keeps their texts for the description of the graph. An
// accDescr in braces runs over more lines.
func (f *flowchart) accessibility(lines []Line) ([]Line, error) {
	var out []Line
	for i := 0; i < len(lines); i++ {
		if !accRE.MatchString(lines[i].Text) {
			out = append(out, lines[i])
			continue
		}
		text, n, ok := ganttAccText(lines, i)
		if !ok {
			return nil, fmt.Errorf("line %d: an accDescr with no end brace: %w", lines[i].N, ErrSyntax)
		}
		f.acc = append(f.acc, text)
		i = n
	}
	return out, nil
}

func (f *flowchart) cluster() string {
	if len(f.clusters) == 0 {
		return ""
	}
	return f.clusters[len(f.clusters)-1].ID
}

// keywords are the first words of the statements that are not chains.
var keywords = []string{"subgraph", "end", "direction", "classDef", "class", "style", "linkStyle", "click", "callback", "href"}

// stmt reads one statement. first and last say whether it is the first
// and the last statement of its line. A chain goes on from the line
// before when the line before ends with a link, or when the statement is
// the first of its line and starts with a link, as Mermaid reads a link
// over a line break.
func (f *flowchart) stmt(st string, first, last bool) error {
	word, rest, _ := strings.Cut(st, " ")
	rest = strings.TrimSpace(rest)
	cont := f.cont
	f.cont = nil
	if p := f.pending; p != nil {
		f.pending = nil
		if slices.Contains(keywords, word) {
			return fmt.Errorf("line %d: a link with no node after it: %w", f.line, ErrSyntax)
		}
		sc := &scanner{s: st}
		next, err := f.group(sc)
		if err != nil {
			return err
		}
		f.connect(p.from, next, p.l)
		return f.links(sc, next, last)
	}
	if first && cont != nil && startsLink(st) {
		return f.links(&scanner{s: st}, cont, last)
	}
	switch word {
	case "subgraph":
		return f.subgraph(rest)
	case "end":
		if rest == "" {
			if len(f.clusters) == 0 {
				return fmt.Errorf("line %d: an end with no subgraph: %w", f.line, ErrSyntax)
			}
			f.clusters = f.clusters[:len(f.clusters)-1]
			return nil
		}
	case "direction":
		dir, err := direction(rest)
		if err != nil {
			return fmt.Errorf("line %d: %w", f.line, err)
		}
		if len(f.clusters) > 0 {
			f.clusters[len(f.clusters)-1].Direction = dir
		}
		return nil
	case "classDef":
		names, css, _ := strings.Cut(rest, " ")
		for n := range strings.SplitSeq(names, ",") {
			st, err := f.css(css)
			if err != nil {
				return err
			}
			f.classes[strings.TrimSpace(n)] = st
		}
		return nil
	case "class":
		ids, name, _ := strings.Cut(rest, " ")
		for id := range strings.SplitSeq(ids, ",") {
			id = strings.TrimSpace(id)
			// The id of an edge, as in A e1@--> B, gives the class to the
			// edge.
			if _, ok := f.link(id); ok {
				f.edgeClasses[id] = append(f.edgeClasses[id], strings.TrimSpace(name))
				continue
			}
			f.node(id)
			f.nodeClasses[id] = append(f.nodeClasses[id], strings.TrimSpace(name))
		}
		return nil
	case "style":
		id, css, _ := strings.Cut(rest, " ")
		st, err := f.css(css)
		if err != nil {
			return err
		}
		f.node(id)
		if f.nodeStyles[id] == nil {
			f.nodeStyles[id] = map[string]string{}
		}
		maps.Copy(f.nodeStyles[id], st)
		return nil
	case "linkStyle":
		return f.linkStyle(rest)
	case "click", "callback", "href":
		return f.add("the interaction %s", word)
	}
	if m := linkPropsRE.FindStringSubmatch(st); m != nil {
		if e, ok := f.link(m[1]); ok {
			return f.linkProps(e, m[2])
		}
	}
	sc := &scanner{s: st}
	group, err := f.group(sc)
	if err != nil {
		return err
	}
	return f.links(sc, group, last)
}

// linkPropsRE matches the properties of an edge by its id, in a statement
// of their own: e1@{ curve: linear }.
var linkPropsRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)@\{(.*)\}$`)

// link returns the edge that an id names, when no node has the id. Every
// edge has an id: its own, as in A e1@--> B, or the id that Mermaid gives
// it, as L_A_B_0.
func (f *flowchart) link(id string) (*graph.Edge, bool) {
	e, ok := f.edgeIDs[id]
	if _, node := f.nodes[id]; node {
		return nil, false
	}
	return e, ok
}

// linkProps reads the properties of an edge: its curve, which wins over
// the curve of the links, or an animation, which is an issue.
func (f *flowchart) linkProps(e *graph.Edge, body string) error {
	m := inlineMap(body)
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		switch k {
		case "curve":
			c, ok := curves[v]
			if !ok {
				if err := f.add("the curve %s", v); err != nil {
					return err
				}
				continue
			}
			f.linkCurves[slices.Index(f.g.Edges, e)] = c
		default:
			if err := f.add("the property %s of a link", k); err != nil {
				return err
			}
		}
	}
	return nil
}

// startsLink reports whether a statement starts with a link, with no id
// of an edge before it.
func startsLink(st string) bool {
	if edgeIDRE.MatchString(st) {
		return false
	}
	return (&scanner{s: st}).link() != nil
}

// subgraph opens a subgraph: subgraph id, subgraph id[title], subgraph
// "title", or subgraph and a title with spaces.
func (f *flowchart) subgraph(rest string) error {
	id, title := rest, rest
	switch {
	case strings.HasPrefix(rest, `"`):
		title = label(rest)
		f.anon++
		id = fmt.Sprintf("subgraph%d", f.anon)
	case strings.Contains(rest, "["):
		i := strings.Index(rest, "[")
		id = strings.TrimSpace(rest[:i])
		title = label(strings.TrimSuffix(strings.TrimSpace(rest[i+1:]), "]"))
	}
	if id == "" {
		f.anon++
		id = fmt.Sprintf("subgraph%d", f.anon)
	}
	c := &graph.Cluster{ID: id, Label: title, Parent: f.cluster(), Style: map[string]string{}}
	f.g.Clusters = append(f.g.Clusters, c)
	f.clusters = append(f.clusters, c)
	return nil
}

// node returns a node, made in the current subgraph when it is new.
func (f *flowchart) node(id string) *graph.Node {
	if n, ok := f.nodes[id]; ok {
		if n.Cluster == "" && f.cluster() != "" {
			n.Cluster = f.cluster()
		}
		return n
	}
	// The id of a subgraph that an edge names is a node of its own here,
	// until subgraphEdges takes it out.
	n := &graph.Node{ID: id, Label: id, Shape: "rect", Cluster: f.cluster(), Style: map[string]string{}}
	f.nodes[id] = n
	f.g.Nodes = append(f.g.Nodes, n)
	return n
}

// shapes are the delimiters of the shapes of a node, the longest first.
var delims = []struct{ open, close, shape string }{
	{"(((", ")))", "doublecircle"}, {"((", "))", "circle"}, {"([", "])", "stadium"}, {"[[", "]]", "subroutine"},
	{"[(", ")]", "cylinder"}, {"[/", "/]", "lean-r"}, {"[/", `\]`, "trap-b"}, {`[\`, `\]`, "lean-l"}, {`[\`, "/]", "trap-t"},
	{"{{", "}}", "hexagon"}, {"{", "}", "diamond"}, {">", "]", "odd"}, {"(", ")", "rounded"}, {"[", "]", "rect"},
}

// scanner reads a statement.
type scanner struct {
	s string
	i int
}

func (sc *scanner) space() {
	for sc.i < len(sc.s) && (sc.s[sc.i] == ' ' || sc.s[sc.i] == '\t') {
		sc.i++
	}
}

func (sc *scanner) done() bool { sc.space(); return sc.i >= len(sc.s) }

// id reads the id of a node.
func (sc *scanner) id() string {
	start := sc.i
	for sc.i < len(sc.s) {
		r := rune(sc.s[sc.i])
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || r >= 0x80 {
			sc.i++
			continue
		}
		// A hyphen or a dot joins two parts of an id, but starts a link
		// before another hyphen, a dot, an equal sign or an arrow.
		if (r == '-' || r == '.') && sc.i > start && sc.i+1 < len(sc.s) {
			n := rune(sc.s[sc.i+1])
			if unicode.IsLetter(n) || unicode.IsDigit(n) || n == '_' {
				sc.i++
				continue
			}
		}
		break
	}
	return sc.s[start:sc.i]
}

// linkRE matches a link with no text, or with its text between pipes after
// it: a normal, a thick, a dotted or an invisible link. A dotted link is
// a dash, if any, then dots, then a dash, as in .-> or -..->.
var (
	linkRE   = regexp.MustCompile(`^([<ox])?(-{2,}|={2,}|-?\.+-|~{3,})(>|[ox](?:[^A-Za-z0-9_]|$))?`)
	edgeIDRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)@([-=~<ox.])`)
)

// textLinks are the links with their text in their middle, by kind. Each
// pattern matches the mark before the start, the start, the blanks after
// it, the text, and the end with its head. A head can touch the node
// after it, as in A-- text -->B. A normal or a thick end with no head can
// also end at a blank, which Mermaid does not read.
// plain holds the marks that, right after the start, make a link with no
// text instead, such as the - of --- or the > of -->.
var textLinks = []struct {
	re    *regexp.Regexp
	kind  string
	plain string
}{
	{regexp.MustCompile(`(?s)^([<ox])?(--)(\s*)(.+?)\s*(-{2,})([->ox]|\s|$)`), "normal", "->ox"},
	{regexp.MustCompile(`(?s)^([<ox])?(==)(\s*)(.+?)\s*(={2,})([=>ox]|\s|$)`), "thick", "=>ox"},
	{regexp.MustCompile(`(?s)^([<ox])?(-\.)(\s*)(.+?)\s*(-?\.+-)([>ox]?)`), "dotted", ".-"},
}

// link is a link between two groups of nodes.
type link struct {
	id, text   string
	head, tail string
	kind       string // normal, thick, dotted or invisible
	minlen     int
	// markdown says whether the text is a string of markdown.
	markdown bool
}

// link reads a link, or returns nil when none comes next.
func (sc *scanner) link() *link {
	sc.space()
	r := sc.s[sc.i:]
	l := &link{}
	if m := edgeIDRE.FindStringSubmatch(r); m != nil {
		l.id = m[1]
		sc.i += len(m[1]) + 1
		r = sc.s[sc.i:]
	}
	body := func(b string, hasHead bool) {
		switch {
		case strings.HasPrefix(b, "="):
			l.kind = "thick"
			l.minlen = max(1, len(b)-1)
			if !hasHead {
				l.minlen = max(1, len(b)-2)
			}
		case strings.HasPrefix(b, "~"):
			l.kind = "invisible"
			l.minlen = max(1, len(b)-2)
		case strings.Contains(b, "."):
			l.kind = "dotted"
			l.minlen = max(1, strings.Count(b, "."))
		default:
			l.kind = "normal"
			l.minlen = max(1, len(b)-1)
			if !hasHead {
				l.minlen = max(1, len(b)-2)
			}
		}
	}
	for _, t := range textLinks {
		m := t.re.FindStringSubmatch(r)
		if m == nil || strings.HasPrefix(m[4], "|") || m[3] == "" && strings.ContainsAny(m[4][:1], t.plain) {
			continue
		}
		end, head := m[5], strings.TrimSpace(m[6])
		if head == "-" || head == "=" {
			end, head = end+head, ""
		}
		l.text, l.markdown = label(m[4]), isMarkdown(m[4])
		body(end, head != "")
		l.kind = t.kind
		// Mermaid reads the mark before the start only when it is the
		// head of the end, as in <-- text -->. Another mark, as in
		// <-- text ---, makes a link that Mermaid draws as -->.
		l.head, l.tail = arrow(head), "none"
		switch {
		case m[1] == "":
		case arrow(m[1]) == l.head:
			l.tail = l.head
		default:
			l.kind, l.head, l.minlen = "normal", "normal", 1
		}
		sc.i += len(m[0])
		return l
	}
	m := linkRE.FindStringSubmatch(r)
	if m == nil {
		return nil
	}
	head := strings.TrimRight(m[3], " \t")
	if len(head) > 1 {
		head = head[:1]
	}
	l.head, l.tail = arrow(head), "none"
	body(m[2], head != "")
	// Mermaid reads the mark before a link with no text only when it is
	// the head, as in <--> or o--o. Another mark counts in the length of
	// a normal or a thick link.
	switch {
	case m[1] == "":
	case arrow(m[1]) == l.head:
		l.tail = l.head
	case l.kind == "normal" || l.kind == "thick":
		l.minlen++
	}
	sc.i += len(m[1]) + len(m[2]) + len(head)
	sc.space()
	if sc.i < len(sc.s) && sc.s[sc.i] == '|' {
		end := strings.Index(sc.s[sc.i+1:], "|")
		if end >= 0 {
			raw := sc.s[sc.i+1 : sc.i+1+end]
			l.text, l.markdown = label(raw), isMarkdown(raw)
			sc.i += end + 2
		}
	}
	return l
}

// links reads the links of a chain and the groups of nodes after them, A &
// B --> C, from the group of nodes before them to the end of the
// statement. last says whether the statement is the last of its line: then
// a link at its end goes to the next line, and its last group is the one
// that a line that starts with a link goes on from.
func (f *flowchart) links(sc *scanner, group []string, last bool) error {
	for !sc.done() {
		l := sc.link()
		if l == nil {
			return fmt.Errorf("line %d: %q where a link goes: %w", f.line, sc.s[sc.i:], ErrSyntax)
		}
		if sc.done() {
			if !last {
				return fmt.Errorf("line %d: a link with no node after it: %w", f.line, ErrSyntax)
			}
			f.pending = &pendingLink{from: group, l: l}
			return nil
		}
		next, err := f.group(sc)
		if err != nil {
			return err
		}
		f.connect(group, next, l)
		group = next
	}
	if last {
		f.cont = group
	}
	return nil
}

// connect adds an edge of a link from each node of a group to each node of
// the next group. The id of the link names the edge from the last node of
// the group to the first node of the next group, as in Mermaid.
func (f *flowchart) connect(group, next []string, l *link) {
	for i, a := range group {
		for j, b := range next {
			id := ""
			if i == len(group)-1 && j == 0 {
				id = l.id
			}
			f.edge(a, b, l, id)
		}
	}
}

// arrow returns the arrow of a mark at an end of a link.
func arrow(c string) string {
	switch c {
	case ">", "<":
		return "normal"
	case "o":
		return "circle"
	case "x":
		return "cross"
	}
	return "none"
}

// group reads nodes joined by &.
func (f *flowchart) group(sc *scanner) ([]string, error) {
	var ids []string
	for {
		sc.space()
		id, err := f.nodeRef(sc)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		sc.space()
		if sc.i < len(sc.s) && sc.s[sc.i] == '&' {
			sc.i++
			continue
		}
		return ids, nil
	}
}

// nodeRef reads a node with its shape, its properties and its class.
func (f *flowchart) nodeRef(sc *scanner) (string, error) {
	id := sc.id()
	if id == "" {
		return "", fmt.Errorf("line %d: %q where a node goes: %w", f.line, sc.s[sc.i:], ErrSyntax)
	}
	n := f.node(id)
	if strings.HasPrefix(sc.s[sc.i:], "@{") {
		end := matching(sc.s, sc.i+1, '{', '}')
		if end < 0 {
			return "", fmt.Errorf("line %d: properties with no end: %w", f.line, ErrSyntax)
		}
		if err := f.props(n, sc.s[sc.i+2:end]); err != nil {
			return "", err
		}
		sc.i = end + 1
	}
	for _, d := range delims {
		if !strings.HasPrefix(sc.s[sc.i:], d.open) {
			continue
		}
		start := sc.i + len(d.open)
		var text string
		var end int
		if start < len(sc.s) && sc.s[start] == '"' {
			q := strings.Index(sc.s[start+1:], `"`)
			if q < 0 {
				continue
			}
			text = sc.s[start : start+q+2]
			end = start + q + 2
			if !strings.HasPrefix(sc.s[end:], d.close) {
				continue
			}
		} else {
			e := strings.Index(sc.s[start:], d.close)
			if e < 0 {
				continue
			}
			// A shape that opens with [/ closes with /] or \], whichever
			// comes first.
			if d.open == "[/" || d.open == `[\` {
				other := "/]"
				if d.close == "/]" {
					other = `\]`
				}
				if o := strings.Index(sc.s[start:], other); o >= 0 && o < e {
					continue
				}
			}
			text = sc.s[start : start+e]
			end = start + e
		}
		n.Label = label(text)
		f.markdown[id] = isMarkdown(text)
		n.Shape = d.shape
		sc.i = end + len(d.close)
		if strings.Contains(text, "fa:fa-") || strings.Contains(text, "fab:fa-") || strings.Contains(text, "fas:fa-") || strings.Contains(text, "far:fa-") {
			labelIcon(n)
		}
		break
	}
	if strings.HasPrefix(sc.s[sc.i:], ":::") {
		sc.i += 3
		c := sc.id()
		f.nodeClasses[id] = append(f.nodeClasses[id], c)
	}
	return id, nil
}

// labelIcon takes an icon of Font Awesome out of the label of a node, as
// fa:fa-name, and draws it before the label.
func labelIcon(n *graph.Node) {
	words := strings.Fields(n.Label)
	var rest []string
	for _, w := range words {
		if p, name, ok := strings.Cut(w, ":fa-"); ok && n.Media == nil && (p == "fa" || p == "fab" || p == "fas" || p == "far") {
			n.Media = &graph.Media{Icon: p + ":" + name, Inline: true}
			continue
		}
		rest = append(rest, w)
	}
	n.Label = strings.Join(rest, " ")
}

// matching returns the index of the bracket that closes the one at i.
func matching(s string, i int, open, closing byte) int {
	depth := 0
	quote := false
	for j := i; j < len(s); j++ {
		switch {
		case s[j] == '"':
			quote = !quote
		case quote:
		case s[j] == open:
			depth++
		case s[j] == closing:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// props reads the properties of a node, A@{ shape: rect, label: "..." },
// with an icon, A@{ icon: "fa:user", form: square, pos: t, h: 48 }, or an
// image, A@{ img: "url", w: 60, h: 60, constraint: on }.
func (f *flowchart) props(n *graph.Node, body string) error {
	m := inlineMap(body)
	media := &graph.Media{}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		switch k {
		case "shape":
			if _, ok := graph.ShapeOf(v); !ok && v != "icon" && v != "image" {
				if err := f.add("the shape %s, drawn as a rounded rectangle", v); err != nil {
					return err
				}
				v = "rounded"
			}
			n.Shape = v
		case "label":
			n.Label = label(v)
			f.markdown[n.ID] = isMarkdown(v)
		case "icon":
			media.Icon = v
		case "img":
			media.Image = v
		case "form":
			media.Form = v
		case "pos":
			media.Pos = v
		case "w", "h":
			x, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
			if err != nil || x <= 0 {
				return fmt.Errorf("line %d: the size %s: %q: %w", f.line, k, v, ErrSyntax)
			}
			if k == "w" {
				media.Width = x
			} else {
				media.Height = x
			}
		case "constraint":
			media.Keep = v == "on"
		case "view":
			// finish collapses a subgraph. Any other view draws the
			// subgraph open, as in Mermaid.
			if v == "collapsed" {
				f.collapsed[n.ID] = f.line
			} else {
				delete(f.collapsed, n.ID)
			}
		default:
			if err := f.add("the property %s of a node", k); err != nil {
				return err
			}
		}
	}
	if media.Icon != "" || media.Image != "" {
		n.Media = media
	}
	f.bare(n)
	return nil
}

// bare gives a node of a shape that draws no text, as Mermaid does, its
// size and no label: a fork or a join, a bar across the ranks, an
// hourglass, a bolt, a crossed circle, a framed circle, a choice and an
// anchor.
func (f *flowchart) bare(n *graph.Node) {
	kind, _ := graph.ShapeOf(n.Shape)
	// A choice is a diamond and an anchor is a point by their shape, but
	// each has a size of its own.
	if n.Shape == "choice" || n.Shape == "anchor" {
		kind = n.Shape
	}
	var w, h float64
	switch kind {
	case "choice":
		// A small diamond, as the choice of a state diagram.
		w, h = 28, 28
	case "anchor":
		// A dot in the color of the node, where edges meet.
		w, h = 2, 2
		n.Style["fillOpacity"] = "1"
	case "bar":
		w, h = 70, 10
		if d := f.g.Direction; d == graph.LR || d == graph.RL {
			w, h = 10, 70
		}
		n.Style["fillOpacity"] = "1"
	case "hourglass":
		w, h = 30, 30
	case "bolt":
		w, h = 35, 70
	case "cross-circ":
		w, h = 40, 40
	case "fr-circ":
		w, h = 14, 14
	default:
		return
	}
	n.Label, n.Width, n.Height = "", w, h
}

// inlineMap reads a map of the properties of a node: key: value, by commas
// or new lines, with values in double quotes, in single quotes or plain. In
// single quotes, two single quotes are one, as in YAML.
func inlineMap(body string) map[string]string {
	out := map[string]string{}
	var parts []string
	double, single, start := false, false, 0
	// value reports whether only blanks come between the colon of the
	// current part and i, so that a single quote at i opens a value.
	value := func(i int) bool {
		return strings.HasSuffix(strings.TrimRight(body[start:i], " \t"), ":")
	}
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case single:
			if c == '\'' {
				if i+1 < len(body) && body[i+1] == '\'' {
					i++
					continue
				}
				single = false
			}
		case c == '"':
			double = !double
		case double:
		case c == '\'' && value(i):
			single = true
		case c == ',' || c == '\n':
			parts = append(parts, body[start:i])
			start = i + 1
		}
	}
	parts = append(parts, body[start:])
	for _, p := range parts {
		k, v, ok := strings.Cut(p, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch {
		case len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"':
			v = v[1 : len(v)-1]
		case len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'':
			v = strings.ReplaceAll(v[1:len(v)-1], "''", "'")
		}
		out[strings.TrimSpace(k)] = v
	}
	return out
}

// edge adds an edge from a link.
// edge adds an edge from a link, with an id or none.
func (f *flowchart) edge(a, b string, l *link, id string) {
	e := &graph.Edge{
		Source: a, Target: b, Label: l.text, MinLen: l.minlen, Head: l.head, Tail: l.tail, Style: map[string]string{}, Curve: f.curve,
		Wrap: f.wrap.width(edgeWrap, l.markdown),
	}
	switch l.kind {
	case "thick":
		e.Style["strokeWidth"] = "3"
	case "dotted":
		e.Style["strokeDash"] = "3,3"
	case "invisible":
		e.Style["stroke"] = "transparent"
		e.Head, e.Tail = "none", "none"
	}
	if _, ok := f.edgeIDs[id]; id == "" || ok {
		// An edge with no id, or with the id of an edge before it, takes
		// the id that Mermaid gives it: L_a_b_0 for the first edge from a
		// to b, and then L_a_b_2, L_a_b_3 and on.
		n := 0
		for _, o := range f.g.Edges {
			if o.Source == a && o.Target == b {
				n++
			}
		}
		if n > 0 {
			n++
		}
		id = fmt.Sprintf("L_%s_%s_%d", a, b, n)
	}
	e.ID = id
	f.edgeIDs[id] = e
	f.g.Edges = append(f.g.Edges, e)
}

// linkStyle reads linkStyle 0,1 stroke:..., or linkStyle default, with an
// optional interpolate.
func (f *flowchart) linkStyle(rest string) error {
	which, css, _ := strings.Cut(rest, " ")
	css = strings.TrimSpace(css)
	curve := ""
	if after, ok := strings.CutPrefix(css, "interpolate "); ok {
		c, more, _ := strings.Cut(after, " ")
		vc, ok := curves[c]
		if !ok {
			if err := f.add("the curve %s", c); err != nil {
				return err
			}
		}
		curve, css = vc, more
	}
	st, err := f.css(css)
	if err != nil {
		return err
	}
	if which == "default" {
		f.linkDefault = st
		if curve != "" {
			f.curve = curve
		}
		return nil
	}
	for s := range strings.SplitSeq(which, ",") {
		i, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("line %d: the link %q: %w", f.line, s, ErrSyntax)
		}
		f.linkStyles[i] = st
		if curve != "" {
			f.linkCurves[i] = curve
		}
	}
	return nil
}

// css reads the style of a class or a node: fill:#f9f,stroke:#333.
func (f *flowchart) css(s string) (map[string]string, error) {
	return css(s, f.add)
}

// subgraphEdges takes out the nodes that only an edge to a subgraph made,
// since the layout ends such an edge at the box of the subgraph.
func (f *flowchart) subgraphEdges() error {
	parent := map[string]string{}
	for _, c := range f.g.Clusters {
		parent[c.ID] = c.Parent
	}
	holds := map[string]bool{}
	var keep []*graph.Node
	for _, n := range f.g.Nodes {
		_, isCluster := parent[n.ID]
		if isCluster && n.Label == n.ID && n.Shape == "rect" && n.Media == nil {
			continue
		}
		keep = append(keep, n)
		for c := n.Cluster; c != "" && !holds[c]; c = parent[c] {
			holds[c] = true
		}
	}
	if len(keep) == len(f.g.Nodes) {
		return nil
	}
	f.g.Nodes = keep
	nodes := map[string]bool{}
	for _, n := range keep {
		nodes[n.ID] = true
	}
	for _, e := range f.g.Edges {
		for _, end := range []string{e.Source, e.Target} {
			if _, ok := parent[end]; ok && !nodes[end] && !holds[end] {
				return fmt.Errorf("an edge to the empty subgraph %s: %w", end, ErrSyntax)
			}
		}
	}
	return nil
}

// finish applies the classes and the styles, and the styles of the links.
func (f *flowchart) finish() error {
	for _, i := range slices.Sorted(maps.Keys(f.linkStyles)) {
		if i >= len(f.g.Edges) {
			return fmt.Errorf("the linkStyle of the link %d, of %d links: %w", i, len(f.g.Edges), ErrSyntax)
		}
	}
	for i, e := range f.g.Edges {
		maps.Copy(e.Style, f.linkDefault)
		for _, c := range f.edgeClasses[e.ID] {
			maps.Copy(e.Style, f.classes[c])
		}
		maps.Copy(e.Style, f.linkStyles[i])
		if c, ok := f.linkCurves[i]; ok {
			e.Curve = c
		}
	}
	if err := f.collapse(); err != nil {
		return err
	}
	if err := f.subgraphEdges(); err != nil {
		return err
	}
	for _, n := range f.g.Nodes {
		n.Style = classStyle(f.classes, f.nodeClasses[n.ID], f.nodeStyles[n.ID], n.Style)
		n.Wrap = f.wrap.width(f.nodeWrap, f.markdown[n.ID])
	}
	if len(f.acc) > 0 {
		f.g.Description = strings.Join(f.acc, " ")
	}
	return nil
}

// collapse draws each subgraph that @{ view: collapsed } names as one node,
// as Mermaid does. The node takes the title of the subgraph, and the nodes
// and the subgraphs inside it do not draw. An edge to a node inside it goes
// to the node, and an edge between two nodes inside it does not draw. A
// collapsed subgraph inside a collapsed subgraph goes into the outer one.
func (f *flowchart) collapse() error {
	if len(f.collapsed) == 0 {
		return nil
	}
	clusters := map[string]*graph.Cluster{}
	for _, c := range f.g.Clusters {
		clusters[c.ID] = c
	}
	for _, id := range slices.Sorted(maps.Keys(f.collapsed)) {
		if clusters[id] == nil {
			if err := f.issues.Add(fmt.Sprintf("line %d", f.collapsed[id]), "the view collapsed of %s, which is not a subgraph", id); err != nil {
				return err
			}
		}
	}
	// outer returns the outermost collapsed subgraph that holds a
	// subgraph, or is it, or empty for none.
	outer := func(id string) string {
		top := ""
		for c := clusters[id]; c != nil; c = clusters[c.Parent] {
			if _, ok := f.collapsed[c.ID]; ok {
				top = c.ID
			}
		}
		return top
	}
	// into holds the collapsed subgraph that takes the place of each node
	// that does not draw.
	into := map[string]string{}
	var nodes []*graph.Node
	for _, n := range f.g.Nodes {
		top := outer(n.Cluster)
		if c := clusters[n.ID]; c != nil {
			// The node of the id of a subgraph is the collapsed node, or
			// goes into the collapsed subgraph that holds it.
			top = outer(n.ID)
			if top == n.ID {
				n.Label, n.Shape, n.Cluster, n.Media = c.Label, "collapsedGroup", c.Parent, nil
				top = ""
			}
		}
		if top != "" {
			into[n.ID] = top
			continue
		}
		nodes = append(nodes, n)
	}
	f.g.Nodes = nodes
	var kept []*graph.Cluster
	for _, c := range f.g.Clusters {
		if outer(c.ID) == "" {
			kept = append(kept, c)
		}
	}
	f.g.Clusters = kept
	var edges []*graph.Edge
	for _, e := range f.g.Edges {
		a, aok := into[e.Source]
		b, bok := into[e.Target]
		if !aok {
			a = e.Source
		}
		if !bok {
			b = e.Target
		}
		if a == b && (aok || bok) {
			continue
		}
		e.Source, e.Target = a, b
		edges = append(edges, e)
	}
	f.g.Edges = edges
	return nil
}

// unquote takes the quotes off a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// entities are the entities of a label, such as #quot and #35, each with a
// semicolon after it.
var entities = regexp.MustCompile(`#([a-z]+|\d+);`)

// isMarkdown reports whether the text of a label is a string of markdown,
// in backquotes.
func isMarkdown(s string) bool {
	s = unquote(s)
	return len(s) >= 2 && strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")
}

// label reads the text of a label: quotes off, <br> as a new line,
// entities, and a string of markdown as plain text.
func label(s string) string {
	s = unquote(s)
	if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") && len(s) >= 2 {
		s = strings.NewReplacer("**", "", "__", "", "*", "", "_", "").Replace(s[1 : len(s)-1])
	}
	s = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(s, "\n")
	s = entities.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		switch name {
		case "quot":
			return `"`
		case "amp":
			return "&"
		case "lt":
			return "<"
		case "gt":
			return ">"
		}
		if n, err := strconv.ParseInt(name, 10, 32); err == nil && n >= 0 {
			return string(rune(n))
		}
		return m
	})
	return strings.TrimSpace(s)
}
