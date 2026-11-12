package mermaid

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// An agentflow diagram, agentflow-beta, is a flowchart of the agents of a
// program: its nodes, its links and its flows, which are its subgraphs.
// The flowchart reads it, with the changes that agentState holds.

// agentSettings are the settings of the section agentflow of the config
// that an agentflow diagram reads, and agentFlowSettings the settings of
// the section flowchart that it reads, as Mermaid 12.3 does.
var (
	agentSettings     = []string{"nodeSpacing", "rankSpacing", "diagramPadding", "titleTopMargin", "wrappingWidth", "minNodeWidth"}
	agentFlowSettings = []string{"curve", "subGraphTitleMargin", "inheritDir"}
)

// agentMinWidth is the least width in the pixels of Mermaid of the label of
// a node of an agentflow diagram, the default of agentflow.minNodeWidth.
const agentMinWidth = 120

// agentState holds what a flowchart reads only in an agentflow diagram.
type agentState struct {
	// blocks are the flows and the blocks global that are open, the
	// innermost last.
	blocks []*agentBlock
	// ends holds each flow, by its id, and the ids that its statements
	// name, in the order of the end of each flow.
	ends []agentEnd
	// flows holds each flow that has ended, by its id, and order the ids
	// of the flows in the order of their first end.
	flows map[string]*graph.Cluster
	order []string
	// count is the number of flows that have ended, which numbers a flow
	// with no id.
	count int
	// global holds the ids that a block global names, which no flow holds.
	global map[string]bool
	// connectors holds the id of each connector.
	connectors map[string]bool
	// minWidth is the least width of a node in the pixels of zuzu.
	minWidth float64
}

// agentBlock is a flow or a block global that is open: the cluster of a
// flow, or nil for a block global, and the ids that its statements name.
type agentBlock struct {
	cluster *graph.Cluster
	// id is the id of a flow, and anon says that the flow has none. props
	// holds the properties of the flow in @{ and }, which apply at its end.
	id    string
	anon  bool
	props string
	ids   []string
}

// agentEnd is the end of a flow: its id and the ids that its statements
// name.
type agentEnd struct {
	flow string
	ids  []string
}

// word returns the keyword of a block.
func (b *agentBlock) word() string {
	if b.cluster == nil {
		return "global"
	}
	return "flow"
}

// ref records an id that a statement names, for the block that holds the
// statement. A flow holds the nodes and the flows that its statements
// name, as Mermaid reads it.
func (a *agentState) ref(id string) {
	if len(a.blocks) > 0 {
		b := a.blocks[len(a.blocks)-1]
		b.ids = append(b.ids, id)
	}
}

// agentflowGraph reads an agentflow diagram into a graph.
func agentflowGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	f := newFlowchart(d, strict)
	f.af = &agentState{
		flows: map[string]*graph.Cluster{}, global: map[string]bool{}, connectors: map[string]bool{},
		minWidth: agentWidth(agentMinWidth),
	}
	if err := f.config(); err != nil {
		return nil, f.issues.Report, err
	}
	if err := f.parse(); err != nil {
		return nil, f.issues.Report, err
	}
	f.inheritDir()
	return f.g, f.issues.Report, nil
}

// agentWidth returns the least width of a node in the pixels of zuzu for
// a least width of its label in the pixels of Mermaid: the width scaled as
// a wrap is, and the padding of the node on each side.
func agentWidth(w float64) float64 {
	return scaleWrap("agentflow-beta", "", w) + 2*8
}

// agentConfig reads agentflow.minNodeWidth.
func (f *flowchart) agentConfig() error {
	m, _ := f.d.Config["agentflow"].(map[string]any)
	n, ok, err := f.configNum("agentflow.minNodeWidth", m["minNodeWidth"])
	if err != nil || !ok {
		return err
	}
	f.af.minWidth = 0
	if n > 0 {
		f.af.minWidth = agentWidth(n)
	}
	return nil
}

// agentStmt reads a statement that only an agentflow diagram has: flow,
// global, connector, and the end of a flow or of a block global. It
// returns false for any other statement.
func (f *flowchart) agentStmt(word, rest string) (bool, error) {
	switch word {
	case "flow":
		return true, f.agentFlow(rest)
	case "global":
		if rest != "" {
			return true, fmt.Errorf("line %d: %q after global: %w", f.line, rest, ErrSyntax)
		}
		f.af.blocks = append(f.af.blocks, &agentBlock{})
		return true, nil
	case "connector":
		return true, f.agentConnector(rest)
	case "end":
		if rest == "" {
			return true, f.agentEnd()
		}
	}
	return false, nil
}

// agentHeader reads the id of a flow or of a connector, its title in
// brackets, if any, and its properties in @{ and }, if any: id, id["title"],
// id["title"]@{ view: collapsed } or id@{ ... }.
func (f *flowchart) agentHeader(what, rest string) (string, string, string, error) {
	id, tail := rest, ""
	if i := strings.IndexAny(rest, "[@"); i >= 0 {
		id, tail = rest[:i], rest[i:]
	}
	id = unquote(id)
	if id == "" {
		return "", "", "", fmt.Errorf("line %d: a %s with no id: %w", f.line, what, ErrSyntax)
	}
	title := ""
	if strings.HasPrefix(tail, "[") {
		from := 1
		if strings.HasPrefix(tail[1:], `"`) {
			q := strings.Index(tail[2:], `"`)
			if q < 0 {
				return "", "", "", fmt.Errorf("line %d: the title of the %s %s has no end quote: %w", f.line, what, id, ErrSyntax)
			}
			from = q + 3
		}
		end := strings.Index(tail[from:], "]")
		if end < 0 {
			return "", "", "", fmt.Errorf("line %d: the title of the %s %s has no end bracket: %w", f.line, what, id, ErrSyntax)
		}
		title = label(tail[1 : from+end])
		tail = tail[from+end+1:]
	}
	props := ""
	if strings.HasPrefix(tail, "@{") {
		end := matching(tail, 1, '{', '}')
		if end < 0 {
			return "", "", "", fmt.Errorf("line %d: properties with no end: %w", f.line, ErrSyntax)
		}
		props, tail = tail[2:end], tail[end+1:]
	}
	if t := strings.TrimSpace(tail); t != "" {
		return "", "", "", fmt.Errorf("line %d: %q after the %s %s: %w", f.line, t, what, id, ErrSyntax)
	}
	return id, title, props, nil
}

// agentFlow opens a flow: flow, flow id, flow id["title"], or either with
// properties in @{ and }. A flow with no title draws none, and a flow with
// no id takes the id subGraph and a number, as in Mermaid.
func (f *flowchart) agentFlow(rest string) error {
	b := &agentBlock{anon: rest == ""}
	title := ""
	if !b.anon {
		var err error
		if b.id, title, b.props, err = f.agentHeader("flow", rest); err != nil {
			return err
		}
	}
	b.cluster = &graph.Cluster{ID: b.id, Label: title, Style: map[string]string{}}
	f.g.Clusters = append(f.g.Clusters, b.cluster)
	f.clusters = append(f.clusters, b.cluster)
	f.af.blocks = append(f.af.blocks, b)
	return nil
}

// agentEnd ends a flow or a block global. Mermaid reads a flow at its end:
// a flow with the id of a flow that ended before it adds to that flow, and
// the flow that holds it, if any, holds its id. The ids of a block global
// are in no flow.
func (f *flowchart) agentEnd() error {
	a := f.af
	if len(a.blocks) == 0 {
		return fmt.Errorf("line %d: an end with no flow or global: %w", f.line, ErrSyntax)
	}
	b := a.blocks[len(a.blocks)-1]
	a.blocks = a.blocks[:len(a.blocks)-1]
	if b.cluster == nil {
		for _, id := range b.ids {
			a.global[id] = true
		}
		return nil
	}
	f.clusters = f.clusters[:len(f.clusters)-1]
	id := b.id
	if b.anon {
		id = fmt.Sprintf("subGraph%d", a.count)
	}
	a.count++
	c := b.cluster
	if old := a.flows[id]; old != nil {
		if c.Label != "" {
			old.Label = c.Label
		}
		if c.Direction != "" {
			old.Direction = c.Direction
		}
		f.g.Clusters = slices.DeleteFunc(f.g.Clusters, func(o *graph.Cluster) bool { return o == c })
	} else {
		c.ID = id
		a.flows[id] = c
		a.order = append(a.order, id)
	}
	a.ends = append(a.ends, agentEnd{flow: id, ids: b.ids})
	f.flowProps(id, b.props)
	a.ref(id)
	return nil
}

// flowProps reads the properties of a flow. Only view draws: view:
// collapsed draws the flow as one node. Mermaid keeps the other
// properties, such as model and instruction, as data that it does not
// draw.
func (f *flowchart) flowProps(id, body string) {
	v, ok := agentMap(body)["view"]
	switch {
	case !ok:
	case v == "collapsed":
		f.collapsed[id] = f.line
	default:
		delete(f.collapsed, id)
	}
}

// agentConnector reads a connector: connector id, or connector id["title"],
// with properties in @{ and }, if any. A connector draws as a node with
// its title, or its id, and its properties are data that Mermaid does not
// draw.
func (f *flowchart) agentConnector(rest string) error {
	if rest == "" {
		return fmt.Errorf("line %d: a connector with no id: %w", f.line, ErrSyntax)
	}
	id, title, _, err := f.agentHeader("connector", rest)
	if err != nil {
		return err
	}
	n := f.node(id)
	switch {
	case title != "":
		n.Label = title
	case !f.af.connectors[id]:
		n.Label = id
	}
	if !f.af.connectors[id] {
		n.Shape, n.Media = "rect", nil
	}
	f.af.connectors[id] = true
	f.af.ref(id)
	return nil
}

// agentAliases are the names of the shapes of an agentflow diagram, by the
// shape of Mermaid that each names.
var agentAliases = map[string]string{
	"task": "roundedRect", "tool": "subroutine", "input": "lean-right", "decision": "diamond",
	"refdoc": "lin-doc", "action": "hexagon", "round": "rect",
}

// agentShapes are the shapes of Mermaid that an agentflow diagram draws,
// by the shape of zuzu that draws each.
var agentShapes = map[string]string{
	"roundedRect": "rounded", "subroutine": "subroutine", "subprocess": "subroutine", "subproc": "subroutine",
	"framed-rectangle": "subroutine", "lean-right": "lean-r", "diamond": "diamond", "lin-doc": "lin-doc",
	"lined-document": "lin-doc", "hexagon": "hexagon", "hex": "hexagon",
}

// agentShape returns the shape of zuzu that draws a shape of an agentflow
// diagram. A shape that Mermaid has and that an agentflow diagram does not
// draw, such as rect or cyl, draws as a rounded rectangle, as in Mermaid.
// A name with a capital or an underscore and a name that Mermaid does not
// have are errors, as in Mermaid.
func agentShape(v string) (string, error) {
	if v != strings.ToLower(v) || strings.Contains(v, "_") {
		return "", fmt.Errorf("the shape %s: a name of a shape is in lower case, with no underscore: %w", v, ErrSyntax)
	}
	s := v
	if a, ok := agentAliases[v]; ok {
		s = a
	}
	if z, ok := agentShapes[s]; ok {
		return z, nil
	}
	if _, ok := graph.ShapeOf(s); ok || s == "icon" {
		return "rounded", nil
	}
	return "", fmt.Errorf("the shape %s, which Mermaid does not have: %w", v, ErrSyntax)
}

// agentBracket returns the shape of a node of an agentflow diagram that
// its brackets give: a subroutine, a diamond, a hexagon or a
// parallelogram, as in a flowchart, and a rounded rectangle for each
// other shape, as in Mermaid.
func agentBracket(shape string) string {
	switch shape {
	case "subroutine", "diamond", "hexagon", "lean-r":
		return shape
	}
	return "rounded"
}

// agentProps reads the properties of a node of an agentflow diagram in @{
// and }. shape and label draw, as in a flowchart. The properties of a flow
// that has ended are the properties of the flow, and the properties of a
// connector draw nothing. Mermaid keeps every other property, such as
// value, params, returns or connectorRef, as data that it does not draw,
// and so does zuzu.
func (f *flowchart) agentProps(n *graph.Node, body string) error {
	id := n.ID
	if f.af.flows[id] != nil {
		f.flowProps(id, body)
		return nil
	}
	if f.af.connectors[id] {
		return nil
	}
	m := agentMap(body)
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		switch k {
		case "shape":
			if v == "" {
				continue
			}
			s, err := agentShape(v)
			if err != nil {
				return fmt.Errorf("line %d: %w", f.line, err)
			}
			n.Shape = s
		case "label":
			n.Label = label(v)
			f.markdown[id] = isMarkdown(v)
		case "view":
			// Mermaid collapses only a flow that has ended.
			if v == "collapsed" {
				if err := f.add("the view collapsed of %s, which is not a flow that ends before it", id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// agentMap reads the properties of a node or a flow of an agentflow
// diagram. Properties on one line are a map of YAML in braces, as in a
// flowchart. Properties over more lines are a block of YAML, one key on
// each line, so a comma at the end of a plain value is a part of the
// value, as in Mermaid: shape: input, is the shape "input,". A comma after
// a value in quotes is not.
func agentMap(body string) map[string]string {
	if !strings.Contains(body, "\n") {
		return inlineMap(body)
	}
	out := map[string]string{}
	for l := range strings.SplitSeq(body, "\n") {
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if t := strings.TrimSuffix(v, ","); len(t) >= 2 && (t[0] == '"' && t[len(t)-1] == '"' || t[0] == '\'' && t[len(t)-1] == '\'') {
			v = inlineMap("v: " + t)["v"]
		}
		out[strings.TrimSpace(k)] = v
	}
	return out
}

// joinProps joins a line that opens properties with @{ and does not close
// them with the lines after it, up to the line that closes them, with a
// new line between them.
func joinProps(lines []Line) []Line {
	open := func(s string) bool {
		i := strings.LastIndex(s, "@{")
		return i >= 0 && matching(s, i+1, '{', '}') < 0
	}
	var out []Line
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		for open(l.Text) && i+1 < len(lines) {
			i++
			l.Text += "\n" + lines[i].Text
		}
		out = append(out, l)
	}
	return out
}

// The links of an agentflow diagram: a link with a head, --> or --x, with
// more dashes for a longer link, and a dotted link with no head, -.-, with
// more dots for a longer link.
var (
	agentArrowRE = regexp.MustCompile(`^--+[x>]`)
	agentDotRE   = regexp.MustCompile(`^-\.+-`)
)

// agentLink reads a link of an agentflow diagram, after its id, if any:
// -->, --x or -.-, with its text between pipes after it, or a link with
// its text in its middle, such as "-- yes -->". It returns nil when
// no link comes next. Mermaid reads no other link here, such as ---, ==>
// or -.->.
func (sc *scanner) agentLink(l *link) *link {
	r := sc.s[sc.i:]
	l.tail = "none"
	if m := agentArrowRE.FindString(r); m != "" {
		l.kind, l.head, l.minlen = "normal", arrow(m[len(m)-1:]), min(len(m)-2, 10)
		sc.i += len(m)
		sc.pipeText(l)
		return l
	}
	if m := agentDotRE.FindString(r); m != "" {
		l.kind, l.head, l.minlen = "dotted", "none", min(strings.Count(m, "."), 10)
		sc.i += len(m)
		sc.pipeText(l)
		return l
	}
	if !strings.HasPrefix(r, "--") {
		return nil
	}
	// The text runs to the next two dashes outside double quotes, which
	// must start the end of the link.
	quote := false
	i := 2
	for ; i < len(r); i++ {
		if r[i] == '"' {
			quote = !quote
		}
		if !quote && strings.HasPrefix(r[i:], "--") {
			break
		}
	}
	m := agentArrowRE.FindString(r[min(i, len(r)):])
	text := strings.TrimSpace(r[2:min(i, len(r))])
	if m == "" || text == "" {
		return nil
	}
	l.text, l.markdown = label(text), isMarkdown(text)
	l.kind, l.head, l.minlen = "normal", arrow(m[len(m)-1:]), min(len(m)-2, 10)
	sc.i += i + len(m)
	return l
}

// pipeText reads the text of a link between pipes, when it comes next.
func (sc *scanner) pipeText(l *link) {
	sc.space()
	if sc.i >= len(sc.s) || sc.s[sc.i] != '|' {
		return
	}
	end := strings.Index(sc.s[sc.i+1:], "|")
	if end < 0 {
		return
	}
	raw := sc.s[sc.i+1 : sc.i+1+end]
	l.text, l.markdown = label(raw), isMarkdown(raw)
	sc.i += end + 2
}

// agentMembers puts each node and each flow in the flow that holds it, as
// Mermaid does when it draws: at its end, a flow holds each id that its
// statements name and that no flow that ended before it holds, except
// itself and the ids of a block global. A flow does not hold a flow that
// holds it, as in Mermaid, and such a loop is an issue. It makes a node
// for each collapsed flow too.
func (f *flowchart) agentMembers() error {
	a := f.af
	held := map[string]bool{}
	members := map[string][]string{}
	for _, e := range a.ends {
		for _, id := range e.ids {
			if id == e.flow || a.global[id] || held[id] {
				continue
			}
			held[id] = true
			members[e.flow] = append(members[e.flow], id)
		}
	}
	parent := map[string]string{}
	// cycle reports whether a flow that holds an id closes a loop.
	cycle := func(id, flow string) bool {
		seen := map[string]bool{}
		for c := flow; c != "" && !seen[c]; c = parent[c] {
			if c == id {
				return true
			}
			seen[c] = true
		}
		return false
	}
	for _, fl := range slices.Backward(a.order) {
		for _, id := range members[fl] {
			if cycle(id, fl) {
				if err := f.issues.Add("flow "+fl, "the flow %s holds %s, which holds it", fl, id); err != nil {
					return err
				}
				continue
			}
			parent[id] = fl
		}
	}
	for _, c := range f.g.Clusters {
		c.Parent = parent[c.ID]
	}
	for _, n := range f.g.Nodes {
		n.Cluster = parent[n.ID]
	}
	for _, id := range slices.Sorted(maps.Keys(f.collapsed)) {
		if a.flows[id] != nil {
			f.node(id).Cluster = parent[id]
		}
	}
	return nil
}

// agentShapes draws each node with no shape of its own as a rounded
// rectangle, and gives each node the least width of agentflow.minNodeWidth,
// except a collapsed flow and a diamond, which grows to fit its label.
func (f *flowchart) agentShapes() {
	for _, n := range f.g.Nodes {
		if n.Shape == "rect" || n.Shape == "" {
			n.Shape = "rounded"
		}
		if n.Shape != "collapsedGroup" && n.Shape != "diamond" {
			n.MinWidth = f.af.minWidth
		}
	}
}
