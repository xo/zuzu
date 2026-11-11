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

// stateDiagram reads the body of a state diagram, stateDiagram or
// stateDiagram-v2, into a graph. A composite state is a cluster, and each
// region of a concurrent composite state is a cluster inside it.
type stateDiagram struct {
	d      *Diagram
	g      *graph.Graph
	issues *graph.Issues
	// states holds each simple state by its id, and composites each
	// composite state.
	states     map[string]*state
	composites map[string]*graph.Cluster
	// descs holds the descriptions of each state, simple or composite.
	descs map[string][]string
	notes []*stateNote
	// classes, nodeClasses and nodeStyles are the classes and the styles,
	// as in a flowchart.
	classes     map[string]map[string]string
	nodeClasses map[string][]string
	nodeStyles  map[string]map[string]string
	scopes      []*scope
	regions     int
	line        int
	acc         []string
	// wrap is the width at which the label of a state wraps.
	wrap float64
}

// state is a simple state: its node and its kind, which is empty, start,
// end, choice, fork or join.
type state struct {
	node *graph.Node
	kind string
}

// stateNote is a note of a state.
type stateNote struct {
	of, text string
	left     bool
}

// scope is the root, a composite state or a region: each has its own
// start and end.
type scope struct {
	// name names the start, name_start, and the end, name_end.
	name string
	// cluster is the cluster of the states of the scope, or empty for the
	// root.
	cluster string
	// region is true for a region of a concurrent composite state.
	region bool
}

// The patterns of a line that starts with state, after the word state:
// "Text" as id, id <<kind>> or id [[kind]], and id {. noteRE matches a
// note of a state, and accRE the text of accTitle or accDescr.
var (
	stateDescAs = regexp.MustCompile(`^"([^"]*)"\s+as\s+(\S+?)(\s*\{)?$`)
	stateKind   = regexp.MustCompile(`^(\S+?)\s*(?:<<(\w+)>>|\[\[(\w+)\]\])$`)
	stateOpen   = regexp.MustCompile(`^(\S+?)\s*\{$`)
	noteRE      = regexp.MustCompile(`^note\s+(left|right)\s+of\s+([^\s:]+)\s*(?::\s*(.*))?$`)
	accRE       = regexp.MustCompile(`^acc(?:Title\s*:|Descr\s*[:{])`)
)

// pseudo is the start or the end of a scope in a transition.
const pseudo = "[*]"

// stateGraph reads a state diagram into a graph.
func stateGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	s := &stateDiagram{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		states: map[string]*state{}, composites: map[string]*graph.Cluster{}, descs: map[string][]string{},
		classes: map[string]map[string]string{}, nodeClasses: map[string][]string{}, nodeStyles: map[string]map[string]string{},
		scopes: []*scope{{name: "root"}},
	}
	if err := s.config(); err != nil {
		return nil, s.issues.Report, err
	}
	if err := s.parse(); err != nil {
		return nil, s.issues.Report, err
	}
	// Mermaid wraps the label of a state, of a note and of a transition. A
	// state with more descriptions is a record, and does not wrap.
	wrapGraph(d.Config, s.g, s.wrap)
	return s.g, s.issues.Report, nil
}

func (s *stateDiagram) add(format string, args ...any) error {
	return s.issues.Add(fmt.Sprintf("line %d", s.line), format, args...)
}

func (s *stateDiagram) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", s.line, fmt.Sprintf(format, args...), ErrSyntax)
}

// config reads the config of the front matter and the directives. The
// settings of the section state set the spacing and the look of Mermaid,
// so each one is an issue, except wrappingWidth.
func (s *stateDiagram) config() error {
	if err := sectionConfig(s.d, s.issues, "state", "wrappingWidth"); err != nil {
		return err
	}
	var err error
	s.wrap, err = wrapSetting(s.issues, s.d.Config, nodeWrap, "state", "wrappingWidth")
	return err
}

// top returns the scope that holds the current line.
func (s *stateDiagram) top() *scope {
	return s.scopes[len(s.scopes)-1]
}

// dir returns the direction of the current scope: that of the innermost
// composite state with a direction, or else that of the diagram.
func (s *stateDiagram) dir() string {
	for i := len(s.scopes) - 1; i > 0; i-- {
		if c := s.composites[s.scopes[i].name]; c != nil && c.Direction != "" {
			return c.Direction
		}
	}
	return s.g.Direction
}

// parse reads the lines of the body.
func (s *stateDiagram) parse() error {
	lines := s.d.Lines
	for i := 0; i < len(lines); i++ {
		s.line = lines[i].N
		t := lines[i].Text
		switch {
		case accRE.MatchString(t):
			text, n, ok := ganttAccText(lines, i)
			if !ok {
				return s.errorf("an accDescr with no end brace")
			}
			i = n
			s.acc = append(s.acc, text)
			continue
		case strings.HasPrefix(t, "note "):
			n, err := s.note(lines, i)
			if err != nil {
				return err
			}
			i = n
			continue
		}
		if err := s.stmt(t, lines, i); err != nil {
			return err
		}
	}
	if len(s.scopes) > 1 {
		return fmt.Errorf("the composite state %s has no end brace: %w", s.top().name, ErrSyntax)
	}
	return s.finish()
}

// stmt reads one statement.
func (s *stateDiagram) stmt(t string, lines []Line, i int) error {
	word, rest, _ := strings.Cut(t, " ")
	rest = strings.TrimSpace(rest)
	switch {
	case t == "}":
		return s.close()
	case t == "--":
		return s.region()
	case word == "state":
		return s.declare(rest, lines, i)
	case word == "direction":
		dir, err := direction(rest)
		if err != nil {
			return fmt.Errorf("line %d: %w", s.line, err)
		}
		if len(s.scopes) == 1 {
			s.g.Direction = dir
			return nil
		}
		for j := len(s.scopes) - 1; j > 0; j-- {
			if c := s.composites[s.scopes[j].name]; c != nil {
				c.Direction = dir
				break
			}
		}
		return nil
	case word == "classDef":
		names, def, _ := strings.Cut(rest, " ")
		st, err := css(def, s.add)
		if err != nil {
			return err
		}
		for n := range strings.SplitSeq(names, ",") {
			s.classes[strings.TrimSpace(n)] = st
		}
		return nil
	case word == "class":
		f := strings.Fields(rest)
		if len(f) < 2 {
			return s.errorf("a class with no name")
		}
		for id := range strings.SplitSeq(strings.Join(f[:len(f)-1], ""), ",") {
			if id = strings.TrimSpace(id); id != "" {
				s.ref(id)
				s.nodeClasses[id] = append(s.nodeClasses[id], f[len(f)-1])
			}
		}
		return nil
	case word == "style":
		ids, def, _ := strings.Cut(rest, " ")
		st, err := css(def, s.add)
		if err != nil {
			return err
		}
		for id := range strings.SplitSeq(ids, ",") {
			id = strings.TrimSpace(id)
			s.ref(id)
			if s.nodeStyles[id] == nil {
				s.nodeStyles[id] = map[string]string{}
			}
			maps.Copy(s.nodeStyles[id], st)
		}
		return nil
	case t == "hide empty description":
		// Mermaid reads this line of the first version of the state
		// diagram and draws nothing for it.
		return nil
	case word == "scale":
		return s.add("the scale of a state diagram")
	case word == "click":
		return s.add("the interaction click")
	}
	if a, b, ok := strings.Cut(t, "-->"); ok {
		return s.transition(a, b)
	}
	id, cls, desc := s.split(t)
	if id == "" || id == pseudo {
		return s.errorf("%q is not a statement", t)
	}
	s.ref(id)
	s.class(id, cls)
	if desc != "" {
		s.descs[id] = append(s.descs[id], label(desc))
	}
	return nil
}

// split reads an id with an optional class and description: id,
// id:::class or id : description.
func (s *stateDiagram) split(t string) (id, cls, desc string) {
	head, desc, _ := strings.Cut(strings.Replace(t, ":::", "\x00", 1), ":")
	head, cls, _ = strings.Cut(head, "\x00")
	id = strings.TrimSpace(head)
	if strings.ContainsAny(id, " \t\"{}<>[]") && id != pseudo {
		return "", "", ""
	}
	return id, strings.TrimSpace(cls), strings.TrimSpace(desc)
}

// class adds a class to a state, for id:::class.
func (s *stateDiagram) class(id, cls string) {
	if cls != "" {
		s.nodeClasses[id] = append(s.nodeClasses[id], cls)
	}
}

// ref returns the state of an id, made in the current scope when it is
// new. A composite state has no state, and ref returns nil for it.
func (s *stateDiagram) ref(id string) *state {
	if _, ok := s.composites[id]; ok {
		return nil
	}
	if st, ok := s.states[id]; ok {
		return st
	}
	n := &graph.Node{ID: id, Label: id, Shape: "rounded", Cluster: s.top().cluster, Style: map[string]string{}}
	st := &state{node: n}
	s.states[id] = st
	s.g.Nodes = append(s.g.Nodes, n)
	return st
}

// end returns the id of the start or the end of the current scope, made
// when it is new: a filled circle and a bullseye of 14 pixels.
func (s *stateDiagram) end(kind string) string {
	sc := s.top()
	id := sc.name + "_" + kind
	if _, ok := s.states[id]; ok {
		return id
	}
	shape := "point"
	if kind == "end" {
		shape = "bullseye"
	}
	n := &graph.Node{ID: id, Shape: shape, Cluster: sc.cluster, Width: 14, Height: 14,
		Style: map[string]string{"fillOpacity": "1", "strokeWidth": "0"}}
	s.states[id] = &state{node: n, kind: kind}
	s.g.Nodes = append(s.g.Nodes, n)
	return id
}

// transition reads a --> b, with an optional label after a colon.
func (s *stateDiagram) transition(a, b string) error {
	from, fromCls, _ := s.split(strings.TrimSpace(a))
	to, toCls, text := s.split(strings.TrimSpace(b))
	if from == "" || to == "" {
		return s.errorf("the transition %q", a+"-->"+b)
	}
	if from == pseudo {
		from = s.end("start")
	} else {
		s.ref(from)
		s.class(from, fromCls)
	}
	if to == pseudo {
		to = s.end("end")
	} else {
		s.ref(to)
		s.class(to, toCls)
	}
	s.g.Edges = append(s.g.Edges, &graph.Edge{Source: from, Target: to, Label: label(text), Style: map[string]string{}})
	return nil
}

// declare reads a line that starts with state: state "Text" as id, state
// id <<choice>>, state id [[fork]], or state id { to open a composite
// state.
func (s *stateDiagram) declare(rest string, lines []Line, i int) error {
	if m := stateDescAs.FindStringSubmatch(rest); m != nil {
		id, cls, _ := strings.Cut(m[2], ":::")
		s.descs[id] = append(s.descs[id], label(m[1]))
		if m[3] != "" {
			return s.open(id, lines, i)
		}
		s.ref(id)
		s.class(id, cls)
		return nil
	}
	if m := stateKind.FindStringSubmatch(rest); m != nil {
		kind := m[2] + m[3]
		st := s.ref(m[1])
		if st == nil {
			return s.errorf("the composite state %s as a %s", m[1], kind)
		}
		switch kind {
		case "choice", "fork", "join":
			// A choice, a fork and a join draw no text.
			st.node.Label = ""
		}
		switch kind {
		case "choice":
			st.kind = kind
			st.node.Shape, st.node.Width, st.node.Height = "diamond", 28, 28
		case "fork", "join":
			st.kind = kind
			st.node.Shape, st.node.Width, st.node.Height = "bar", 70, 10
			if d := s.dir(); d == graph.LR || d == graph.RL {
				st.node.Width, st.node.Height = 10, 70
			}
			st.node.Style["fillOpacity"] = "1"
		default:
			return s.add("the kind of state %s, drawn as a state", kind)
		}
		return nil
	}
	if m := stateOpen.FindStringSubmatch(rest); m != nil {
		return s.open(m[1], lines, i)
	}
	id, cls, desc := s.split(rest)
	if id == "" || id == pseudo {
		return s.errorf("the state %q", rest)
	}
	s.ref(id)
	s.class(id, cls)
	if desc != "" {
		s.descs[id] = append(s.descs[id], label(desc))
	}
	return nil
}

// open opens a composite state at the line i. A state of the same id that
// a line before named becomes the composite state. When a line of two
// hyphens splits its body, each part is a region, a cluster inside it.
func (s *stateDiagram) open(id string, lines []Line, i int) error {
	if _, ok := s.composites[id]; !ok {
		c := &graph.Cluster{ID: id, Label: id, Parent: s.top().cluster, Style: map[string]string{}}
		if st, ok := s.states[id]; ok {
			if st.kind != "" {
				return s.errorf("the %s %s as a composite state", st.kind, id)
			}
			s.g.Nodes = slices.DeleteFunc(s.g.Nodes, func(n *graph.Node) bool { return n == st.node })
			delete(s.states, id)
		}
		s.composites[id] = c
		s.g.Clusters = append(s.g.Clusters, c)
	}
	s.scopes = append(s.scopes, &scope{name: id, cluster: id})
	if regions(lines, i) {
		s.openRegion(id)
	}
	return nil
}

// regions reports whether a line of two hyphens splits the body of the
// composite state that opens at the line i.
func regions(lines []Line, i int) bool {
	depth := 0
	note := false
	for _, l := range lines[i+1:] {
		t := l.Text
		switch {
		case note:
			note = t != "end note"
		case noteRE.MatchString(t) && !strings.Contains(t, ":"):
			note = true
		case strings.HasPrefix(t, "state ") && strings.HasSuffix(t, "{"):
			depth++
		case t == "}":
			if depth == 0 {
				return false
			}
			depth--
		case t == "--" && depth == 0:
			return true
		}
	}
	return false
}

// openRegion opens a region of a composite state, a cluster with no title
// and a dashed border.
func (s *stateDiagram) openRegion(parent string) {
	s.regions++
	id := fmt.Sprintf("divider-id-%d", s.regions)
	c := &graph.Cluster{ID: id, Parent: parent, Style: map[string]string{"strokeDash": dashed}}
	s.g.Clusters = append(s.g.Clusters, c)
	s.scopes = append(s.scopes, &scope{name: id, cluster: id, region: true})
}

// region reads --, which ends a region of a composite state and opens the
// next.
func (s *stateDiagram) region() error {
	if !s.top().region {
		return s.errorf("-- outside a composite state")
	}
	s.scopes = s.scopes[:len(s.scopes)-1]
	s.openRegion(s.top().name)
	return nil
}

// close reads }, the end of a composite state.
func (s *stateDiagram) close() error {
	if s.top().region {
		s.scopes = s.scopes[:len(s.scopes)-1]
	}
	if len(s.scopes) == 1 {
		return s.errorf("} with no composite state")
	}
	s.scopes = s.scopes[:len(s.scopes)-1]
	return nil
}

// note reads a note at the line i: note right of id : text, or note left
// of id and the lines of its text up to end note. It returns the last line
// that it read.
func (s *stateDiagram) note(lines []Line, i int) (int, error) {
	t := lines[i].Text
	m := noteRE.FindStringSubmatch(t)
	if m == nil {
		// A note with no state, note "text" as id, floats, and Mermaid
		// does not draw it either.
		if strings.HasPrefix(t, `note "`) {
			return i, s.add("a note that floats, with no state")
		}
		return i, s.errorf("the note %q", t)
	}
	text := m[3]
	if !strings.Contains(t, ":") {
		var body []string
		for i++; ; i++ {
			if i >= len(lines) {
				return i, s.errorf("a note with no end note")
			}
			if lines[i].Text == "end note" {
				break
			}
			body = append(body, lines[i].Text)
		}
		text = strings.Join(body, "\n")
	}
	if m[2] != pseudo {
		s.ref(m[2])
	}
	// Mermaid keeps one note for each state, the last.
	s.notes = slices.DeleteFunc(s.notes, func(n *stateNote) bool { return n.of == m[2] })
	s.notes = append(s.notes, &stateNote{of: m[2], text: label(text), left: m[1] == "left"})
	return i, nil
}

// finish sets the labels from the descriptions, adds the notes, and applies
// the classes and the styles.
func (s *stateDiagram) finish() error {
	for _, n := range s.g.Nodes {
		d := s.descs[n.ID]
		switch {
		case len(d) == 1:
			n.Label = d[0]
		case len(d) > 1:
			// A state with more descriptions draws the first as a title,
			// over a line and the rest.
			n.Label = strings.Join(d, "\n")
			n.Shape = "rounded"
			n.Fields = []graph.Field{{Text: d[0]}, {Text: strings.Join(d[1:], "\n")}}
			n.Stack = true
		}
	}
	for _, c := range s.g.Clusters {
		if d := s.descs[c.ID]; len(d) > 0 {
			c.Label = strings.Join(d, "\n")
		}
	}
	if err := s.addNotes(); err != nil {
		return err
	}
	for _, n := range s.g.Nodes {
		n.Style = classStyle(s.classes, s.nodeClasses[n.ID], s.nodeStyles[n.ID], n.Style)
	}
	for _, c := range s.g.Clusters {
		c.Style = classStyle(s.classes, s.nodeClasses[c.ID], s.nodeStyles[c.ID], c.Style)
	}
	if len(s.acc) > 0 {
		s.g.Description = strings.Join(s.acc, " ")
	}
	return nil
}

// addNotes adds each note as a node of the shape note, with a dotted line
// to its state. As in Mermaid, the line goes from the state to a note on
// the right, and from a note on the left to the state, so a note on the
// right comes in the rank after its state, and a note on the left in the
// rank before it. The note keeps a neighbor in the layout, which holds it
// close to its state.
func (s *stateDiagram) addNotes() error {
	for _, nt := range s.notes {
		id := nt.of + "----note"
		var of *graph.Node
		var cluster string
		switch st, c := s.states[nt.of], s.composites[nt.of]; {
		case st != nil:
			of, cluster = st.node, st.node.Cluster
		case c != nil:
			cluster = c.Parent
		default:
			return fmt.Errorf("a note of %s, which is not a state: %w", nt.of, ErrSyntax)
		}
		n := &graph.Node{ID: id, Label: nt.text, Shape: "note", Cluster: cluster, Style: map[string]string{}}
		e := &graph.Edge{Source: nt.of, Target: id, Head: "none", Style: map[string]string{"strokeDash": "2,3"}}
		if nt.left {
			e.Source, e.Target = id, nt.of
		}
		at := len(s.g.Nodes)
		if of != nil {
			at = slices.Index(s.g.Nodes, of)
			if !nt.left {
				at++
			}
		}
		s.g.Nodes = slices.Insert(s.g.Nodes, at, n)
		s.g.Edges = append(s.g.Edges, e)
	}
	return nil
}
