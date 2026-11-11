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
}

func newFlowchart(d *Diagram, strict bool) *flowchart {
	return &flowchart{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		nodes: map[string]*graph.Node{}, classes: map[string]map[string]string{}, nodeClasses: map[string][]string{},
		nodeStyles: map[string]map[string]string{}, edgeIDs: map[string]*graph.Edge{},
		linkStyles: map[int]map[string]string{}, linkCurves: map[int]string{},
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
	for k, v := range cfg {
		if ok, err := lookSetting(f.issues, k, v); ok || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		switch k {
		case "flowchart":
			m, _ := v.(map[string]any)
			for fk := range m {
				if fk != "curve" {
					if err := f.issues.Add("config", "the setting flowchart.%s", fk); err != nil {
						return err
					}
				}
			}
		default:
			if err := f.issues.Add("config", "the setting %s", k); err != nil {
				return err
			}
		}
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
	if h := strings.Fields(f.d.Header); len(h) > 0 {
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

// statements splits a line at each semicolon outside quotes and brackets.
func statements(s string) []string {
	var out []string
	depth, quote, start := 0, false, 0
	for i := range len(s) {
		switch c := s[i]; {
		case c == '"':
			quote = !quote
		case quote:
		case c == '[' || c == '(' || c == '{':
			depth++
		case c == ']' || c == ')' || c == '}':
			depth--
		case c == ';' && depth <= 0:
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
	for _, l := range joinOpen(f.d.Lines, `"`) {
		f.line = l.N
		for _, st := range statements(l.Text) {
			st = strings.TrimSpace(st)
			if st == "" {
				continue
			}
			if err := f.stmt(st); err != nil {
				return err
			}
		}
	}
	if len(f.clusters) > 0 {
		return fmt.Errorf("a subgraph with no end: %w", ErrSyntax)
	}
	return f.finish()
}

func (f *flowchart) cluster() string {
	if len(f.clusters) == 0 {
		return ""
	}
	return f.clusters[len(f.clusters)-1].ID
}

// stmt reads one statement.
func (f *flowchart) stmt(st string) error {
	word, rest, _ := strings.Cut(st, " ")
	rest = strings.TrimSpace(rest)
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
	case "accTitle:", "accDescr:", "accTitle", "accDescr":
		return f.add("the accessibility text %s", strings.TrimSuffix(word, ":"))
	}
	return f.chain(st)
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
// it. textLinkRE matches a link with its text in its middle.
var (
	linkRE     = regexp.MustCompile(`^([<ox])?(-{2,}|={2,}|-\.+-|~{3,})(>|[ox](?:[^A-Za-z0-9_]|$))?`)
	textLinkRE = regexp.MustCompile(`(?s)^([<ox])?(--|==|-\.)\s*([^-=.>].*?)\s*(-{2,}|={2,}|\.+-)([>ox])?(?:\s|$)`)
	edgeIDRE   = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)@([-=~<ox.])`)
)

// link is a link between two groups of nodes.
type link struct {
	id, text   string
	head, tail string
	kind       string // normal, thick, dotted or invisible
	minlen     int
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
	arrow := func(c string) string {
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
	if m := textLinkRE.FindStringSubmatch(r); m != nil && !strings.HasPrefix(strings.TrimSpace(m[3]), "|") {
		l.tail, l.head = arrow(m[1]), arrow(m[5])
		l.text = label(m[3])
		body(m[4], m[5] != "")
		if strings.HasPrefix(m[2], "-.") {
			l.kind = "dotted"
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
	l.tail, l.head = arrow(m[1]), arrow(head)
	body(m[2], head != "")
	sc.i += len(m[1]) + len(m[2]) + len(head)
	sc.space()
	if sc.i < len(sc.s) && sc.s[sc.i] == '|' {
		end := strings.Index(sc.s[sc.i+1:], "|")
		if end >= 0 {
			l.text = label(sc.s[sc.i+1 : sc.i+1+end])
			sc.i += end + 2
		}
	}
	return l
}

// chain reads a chain of groups of nodes joined by links: A & B --> C.
func (f *flowchart) chain(st string) error {
	sc := &scanner{s: st}
	group, err := f.group(sc)
	if err != nil {
		return err
	}
	for !sc.done() {
		l := sc.link()
		if l == nil {
			return fmt.Errorf("line %d: %q where a link goes: %w", f.line, sc.s[sc.i:], ErrSyntax)
		}
		next, err := f.group(sc)
		if err != nil {
			return err
		}
		for _, a := range group {
			for _, b := range next {
				f.edge(a, b, l)
			}
		}
		group = next
	}
	return nil
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
		default:
			if err := f.add("the property %s of a node", k); err != nil {
				return err
			}
		}
	}
	if media.Icon != "" || media.Image != "" {
		n.Media = media
	}
	return nil
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
func (f *flowchart) edge(a, b string, l *link) {
	e := &graph.Edge{Source: a, Target: b, Label: l.text, MinLen: l.minlen, Head: l.head, Tail: l.tail, Style: map[string]string{}, Curve: f.curve}
	switch l.kind {
	case "thick":
		e.Style["strokeWidth"] = "3"
	case "dotted":
		e.Style["strokeDash"] = "3,3"
	case "invisible":
		e.Style["stroke"] = "transparent"
		e.Head, e.Tail = "none", "none"
	}
	if l.id != "" {
		f.edgeIDs[l.id] = e
		e.ID = l.id
	}
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
	if err := f.subgraphEdges(); err != nil {
		return err
	}
	for _, n := range f.g.Nodes {
		n.Style = classStyle(f.classes, f.nodeClasses[n.ID], f.nodeStyles[n.ID], n.Style)
	}
	for i, e := range f.g.Edges {
		maps.Copy(e.Style, f.linkDefault)
		maps.Copy(e.Style, f.linkStyles[i])
		if c, ok := f.linkCurves[i]; ok {
			e.Curve = c
		}
	}
	for i := range f.linkStyles {
		if i >= len(f.g.Edges) {
			return fmt.Errorf("the linkStyle of the link %d, of %d links: %w", i, len(f.g.Edges), ErrSyntax)
		}
	}
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
