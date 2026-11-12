package mermaid

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/vega"
)

// requirementDiagram reads the body of a requirement diagram,
// requirementDiagram, into a graph. A requirement and an element are
// records: the type in guillemets and the name, then a line for each part
// of the body, such as the id and the text of a requirement.
type requirementDiagram struct {
	d      *Diagram
	g      *graph.Graph
	issues *graph.Issues
	// boxes holds each requirement and each element by its name, and order
	// holds them in the order of their first definition.
	boxes map[string]*reqBox
	order []*reqBox
	// body is the requirement or the element whose body the current line
	// is in, or nil.
	body      *reqBox
	relations []reqRelation
	// styles, nodeClasses and nodeStyles are the classes and the styles,
	// as in a flowchart.
	styles      map[string]map[string]string
	nodeClasses map[string][]string
	nodeStyles  map[string]map[string]string
	line        int
	acc         []string
}

// reqBox is a requirement or an element. typ is the type that draws in
// guillemets, such as Functional Requirement, or Element. The other
// fields are the parts of its body, empty when the body does not set them.
type reqBox struct {
	name, typ string
	element   bool
	// id, text, risk and verify are the parts of a requirement.
	id, text, risk, verify string
	// kind and docRef are the type and the reference of an element.
	kind, docRef string
}

// reqRelation is a relation from one requirement or element to another,
// such as satisfies, and the line of the source that names it.
type reqRelation struct {
	kind, src, dst string
	line           int
}

// reqTypes are the keywords of a requirement, by the type that Mermaid
// draws, and the keyword of an element.
var reqTypes = map[string]string{
	"requirement": "Requirement", "functionalrequirement": "Functional Requirement",
	"interfacerequirement": "Interface Requirement", "performancerequirement": "Performance Requirement",
	"physicalrequirement": "Physical Requirement", "designconstraint": "Design Constraint", "element": "Element",
}

// The words of a risk and of a method of verification, as Mermaid draws
// them.
var (
	reqRisks   = map[string]string{"low": "Low", "medium": "Medium", "high": "High"}
	reqMethods = map[string]string{"analysis": "Analysis", "demonstration": "Demonstration", "inspection": "Inspection", "test": "Test"}
)

var (
	// reqKeywordRE matches a keyword of Mermaid at the start of a text. A
	// name or a value without quotes cannot start with one, because
	// Mermaid reads the keyword first.
	reqKeywordRE = regexp.MustCompile(`(?i)^(?:title\s|acc(?:Title\s*:|Descr\s*[:{])|(?:requirementDiagram|id|text|risk|verifyMethod|requirement|` +
		`functionalRequirement|interfaceRequirement|performanceRequirement|physicalRequirement|designConstraint|low|medium|high|` +
		`analysis|demonstration|inspection|test|element|contains|copies|derives|satisfies|verifies|refines|traces|type|docref|` +
		`style|classDef|class)\b)`)
	// reqDirectionRE matches a direction. Mermaid reads it anywhere in a
	// text without quotes, so such a text is not a name.
	reqDirectionRE = regexp.MustCompile(`(?i)direction\s+(tb|bt|rl|lr)`)
	// reqDirStmtRE matches the statement of a direction.
	reqDirStmtRE = regexp.MustCompile(`(?i)^direction\s+(tb|bt|rl|lr)`)
	// reqStmtRE matches the keyword of a statement that is not a relation.
	reqStmtRE = regexp.MustCompile(`(?i)^(requirement|functionalRequirement|interfaceRequirement|performanceRequirement|` +
		`physicalRequirement|designConstraint|element|classDef|class|style)\b`)
	// reqFieldRE matches the key of a part of a body, and its colon.
	reqFieldRE = regexp.MustCompile(`(?i)^(id|text|risk|verifyMethod|type|docref)\b\s*:`)
	// reqRelationRE matches the type of a relation.
	reqRelationRE = regexp.MustCompile(`(?i)^(contains|copies|derives|satisfies|verifies|refines|traces)\b`)
	// reqWordRE matches a word at the start of a text.
	reqWordRE = regexp.MustCompile(`^\w+\b`)
)

// requirementGraph reads a requirement diagram into a graph.
func requirementGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	r := &requirementDiagram{
		d: d, g: &graph.Graph{Directed: true, Direction: graph.TB, Title: d.Title}, issues: &graph.Issues{Strict: strict},
		boxes:  map[string]*reqBox{},
		styles: map[string]map[string]string{}, nodeClasses: map[string][]string{}, nodeStyles: map[string]map[string]string{},
	}
	if err := sectionConfig(d, r.issues, "requirement"); err != nil {
		return nil, r.issues.Report, err
	}
	if err := r.parse(); err != nil {
		return nil, r.issues.Report, err
	}
	// Mermaid wraps the label of a relation, and not a requirement or an
	// element.
	wrapGraph(d.Config, r.g, 0)
	return r.g, r.issues.Report, nil
}

func (r *requirementDiagram) add(format string, args ...any) error {
	return r.issues.Add(fmt.Sprintf("line %d", r.line), format, args...)
}

func (r *requirementDiagram) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", r.line, fmt.Sprintf(format, args...), ErrSyntax)
}

// parse reads the lines of the body.
func (r *requirementDiagram) parse() error {
	inBody := func() bool { return r.body != nil }
	if err := readStatements(r.d, &r.line, &r.acc, inBody, r.bodyLine, r.stmt); err != nil {
		return err
	}
	if r.body != nil {
		return fmt.Errorf("the %s %s has no end brace: %w", r.what(r.body), r.body.name, ErrSyntax)
	}
	return r.finish()
}

// what names the kind of a box in an error.
func (r *requirementDiagram) what(b *reqBox) string {
	if b.element {
		return "element"
	}
	return "requirement"
}

// value reads a name or a value at the start of a text: a text in double
// quotes, or a text that starts with a letter, a digit or an underscore
// and runs up to a colon, a comma, a brace, an angle bracket, a dash or
// an equals sign. It returns the value and the rest of the text.
func (r *requirementDiagram) value(s string) (string, string, error) {
	s = strings.TrimLeft(s, " \t")
	if q, ok := strings.CutPrefix(s, `"`); ok {
		v, rest, ok := strings.Cut(q, `"`)
		if !ok {
			return "", "", r.errorf("the text %s has no end quote", s)
		}
		return v, rest, nil
	}
	if !reqWordRE.MatchString(s) {
		return "", "", r.errorf("%q is not a name or a value", s)
	}
	if k := reqKeywordRE.FindString(s); k != "" {
		return "", "", r.errorf("%q starts with the keyword %s, so it must be in double quotes", s, strings.TrimSpace(strings.TrimRight(k, ":{")))
	}
	if reqDirectionRE.MatchString(s) {
		return "", "", r.errorf("%q holds a direction, so it must be in double quotes", s)
	}
	end := strings.IndexAny(s, ":,{<>-=")
	if end < 0 {
		end = len(s)
	}
	return strings.TrimSpace(s[:end]), s[end:], nil
}

// list reads a list of names with a comma between each two, as the
// classes after :::. It returns the names and the rest of the text.
func (r *requirementDiagram) list(s string) ([]string, string, error) {
	var out []string
	for {
		v, rest, err := r.value(s)
		if err != nil {
			return nil, "", err
		}
		out = append(out, v)
		rest = strings.TrimLeft(rest, " \t")
		if after, ok := strings.CutPrefix(rest, ","); ok {
			s = after
			continue
		}
		return out, rest, nil
	}
}

// styleList reads a list of names in a statement classDef, class or
// style: words or texts in double quotes, with a comma between each two.
// It returns the names and the rest of the text.
func styleList(s string) ([]string, string) {
	var out []string
	for {
		s = strings.TrimLeft(s, " \t")
		var v string
		if q, ok := strings.CutPrefix(s, `"`); ok {
			name, rest, ok := strings.Cut(q, `"`)
			if !ok {
				return out, s
			}
			v, s = name, rest
		} else {
			v = reqWordRE.FindString(s)
			if v == "" {
				return out, s
			}
			s = s[len(v):]
		}
		out = append(out, v)
		rest := strings.TrimLeft(s, " \t")
		after, ok := strings.CutPrefix(rest, ",")
		if !ok {
			return out, rest
		}
		s = after
	}
}

// stmt reads one statement outside a body.
func (r *requirementDiagram) stmt(t string) error {
	if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "%") {
		// Mermaid reads # and % at the start of a statement as a comment.
		return nil
	}
	if m := reqDirStmtRE.FindStringSubmatch(t); m != nil {
		r.g.Direction = strings.ToUpper(m[1])
		return nil
	}
	if m := reqStmtRE.FindStringSubmatch(t); m != nil {
		word, rest := strings.ToLower(m[1]), t[len(m[0]):]
		switch word {
		case "classdef":
			return r.classDef(rest)
		case "class":
			ids, rest := styleList(rest)
			names, rest := styleList(rest)
			if len(ids) == 0 || len(names) == 0 || rest != "" {
				return r.errorf("the class %q", t)
			}
			return r.setClass(ids, names)
		case "style":
			ids, def := styleList(rest)
			if len(ids) == 0 {
				return r.errorf("the style %q", t)
			}
			st, err := css(def, r.add)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if err := r.known(id, "the style"); err != nil {
					return err
				}
				if r.boxes[id] == nil {
					continue
				}
				if r.nodeStyles[id] == nil {
					r.nodeStyles[id] = map[string]string{}
				}
				maps.Copy(r.nodeStyles[id], st)
			}
			return nil
		}
		return r.define(reqTypes[word], rest)
	}
	src, rest, err := r.value(t)
	if err != nil {
		return err
	}
	rest = strings.TrimSpace(rest)
	if cls, ok := strings.CutPrefix(rest, ":::"); ok {
		names, rest, err := r.list(cls)
		if err != nil {
			return err
		}
		if strings.TrimSpace(rest) != "" {
			return r.errorf("the class %q", t)
		}
		return r.setClass([]string{src}, names)
	}
	return r.relation(src, rest, t)
}

// classDef reads the rest of a statement classDef: the names of the
// classes and their style.
func (r *requirementDiagram) classDef(rest string) error {
	names, def := styleList(rest)
	if len(names) == 0 {
		return r.errorf("a classDef with no name")
	}
	st, err := css(def, r.add)
	if err != nil {
		return err
	}
	for _, n := range names {
		if r.styles[n] == nil {
			r.styles[n] = map[string]string{}
		}
		maps.Copy(r.styles[n], st)
	}
	return nil
}

// known reports a statement that names a requirement or an element
// before its definition. Mermaid ignores such a statement.
func (r *requirementDiagram) known(id, what string) error {
	if r.boxes[id] != nil {
		return nil
	}
	return r.add("%s of %s before its definition, which Mermaid ignores", what, id)
}

// setClass adds classes to requirements and elements.
func (r *requirementDiagram) setClass(ids, names []string) error {
	for _, id := range ids {
		if err := r.known(id, "the class"); err != nil {
			return err
		}
		if r.boxes[id] != nil {
			r.nodeClasses[id] = append(r.nodeClasses[id], names...)
		}
	}
	return nil
}

// define reads the rest of the definition of a requirement or of an
// element, after its keyword: its name, its classes after ::: and an open
// brace at the end of the line. The lines after it are its body.
func (r *requirementDiagram) define(typ, rest string) error {
	name, rest, err := r.value(rest)
	if err != nil {
		return err
	}
	rest = strings.TrimSpace(rest)
	var classes []string
	if cls, ok := strings.CutPrefix(rest, ":::"); ok {
		if classes, rest, err = r.list(cls); err != nil {
			return err
		}
	}
	if strings.TrimSpace(rest) != "{" {
		return r.errorf("the definition of %s has no open brace at the end of its line", name)
	}
	b := &reqBox{name: name, typ: typ, element: typ == "Element"}
	if old, ok := r.boxes[name]; ok {
		if old.element != b.element {
			return r.errorf("%s is a requirement and an element", name)
		}
		// Mermaid keeps the first definition of a name.
		if err := r.add("the second definition of %s, which Mermaid ignores", name); err != nil {
			return err
		}
	} else {
		r.boxes[name] = b
		r.order = append(r.order, b)
	}
	r.body = b
	if classes != nil {
		r.nodeClasses[name] = append(r.nodeClasses[name], classes...)
	}
	return nil
}

// bodyLine reads a line of the body of a requirement or of an element, or
// its end brace. A statement can follow the brace on its line.
func (r *requirementDiagram) bodyLine(t string) error {
	b := r.body
	if rest, ok := strings.CutPrefix(t, "}"); ok {
		r.body = nil
		if rest = strings.TrimSpace(rest); rest != "" {
			return r.stmt(rest)
		}
		return nil
	}
	if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "%") {
		return nil
	}
	m := reqFieldRE.FindStringSubmatch(t)
	if m == nil {
		return r.errorf("%q is not a part of the %s %s", t, r.what(b), b.name)
	}
	key, rest := strings.ToLower(m[1]), strings.TrimSpace(t[len(m[0]):])
	switch {
	case b.element && (key == "type" || key == "docref"):
	case !b.element && key != "type" && key != "docref":
	default:
		return r.errorf("%q is not a part of the %s %s", t, r.what(b), b.name)
	}
	var v string
	switch key {
	case "risk", "verifymethod":
		words := reqRisks
		if key == "verifymethod" {
			words = reqMethods
		}
		w := strings.ToLower(reqWordRE.FindString(rest))
		v = words[w]
		if v == "" || strings.TrimSpace(rest[len(w):]) != "" {
			return r.errorf("the %s %q", m[1], rest)
		}
	default:
		var err error
		if v, rest, err = r.value(rest); err != nil {
			return err
		}
		if strings.TrimSpace(rest) != "" {
			return r.errorf("%q after the %s of %s", rest, m[1], b.name)
		}
	}
	switch key {
	case "id":
		b.id = v
	case "text":
		b.text = v
	case "risk":
		b.risk = v
	case "verifymethod":
		b.verify = v
	case "type":
		b.kind = v
	case "docref":
		b.docRef = v
	}
	return nil
}

// relation reads the rest of a relation after its first name: - type ->
// name, or <- type - name, which goes from the second name to the first.
func (r *requirementDiagram) relation(first, rest, t string) error {
	back := false
	switch {
	case strings.HasPrefix(rest, "<-"):
		back, rest = true, rest[2:]
	case strings.HasPrefix(rest, "-") && !strings.HasPrefix(rest, "->"):
		rest = rest[1:]
	default:
		return r.errorf("%q is not a statement", t)
	}
	rest = strings.TrimLeft(rest, " \t")
	m := reqRelationRE.FindString(rest)
	if m == "" {
		return r.errorf("the relation %q", t)
	}
	rest = strings.TrimLeft(rest[len(m):], " \t")
	switch {
	case !back && strings.HasPrefix(rest, "->"):
		rest = rest[2:]
	case back && strings.HasPrefix(rest, "-") && !strings.HasPrefix(rest, "->"):
		rest = rest[1:]
	default:
		return r.errorf("the relation %q", t)
	}
	second, rest, err := r.value(rest)
	if err != nil {
		return err
	}
	if strings.TrimSpace(rest) != "" {
		return r.errorf("the relation %q", t)
	}
	src, dst := first, second
	if back {
		src, dst = second, first
	}
	r.relations = append(r.relations, reqRelation{kind: strings.ToLower(m), src: src, dst: dst, line: r.line})
	return nil
}

// finish writes a node for each requirement and each element, as Mermaid
// orders them, the requirements first, and an edge for each relation.
func (r *requirementDiagram) finish() error {
	for _, element := range []bool{false, true} {
		for _, b := range r.order {
			if b.element == element {
				r.g.Nodes = append(r.g.Nodes, r.node(b))
			}
		}
	}
	for _, rel := range r.relations {
		for _, end := range []string{rel.src, rel.dst} {
			if r.boxes[end] == nil {
				return fmt.Errorf("line %d: the relation %s names %s, which is not a requirement or an element: %w", rel.line, rel.kind, end, ErrSyntax)
			}
		}
		// A relation contains has a circle with a cross at the container
		// and a solid line. Every other relation has an open arrowhead at
		// its end and a dashed line.
		e := &graph.Edge{Source: rel.src, Target: rel.dst, Label: "«" + rel.kind + "»", Head: "vee", Tail: "none",
			ArrowSize: relationArrowSize, Style: map[string]string{"strokeDash": dashed}}
		if rel.kind == "contains" {
			e.Head, e.Tail, e.Style = "none", "circleplus", map[string]string{}
		}
		r.g.Edges = append(r.g.Edges, e)
	}
	if len(r.acc) > 0 {
		r.g.Description = strings.Join(r.acc, " ")
	}
	return nil
}

// node returns the node of a requirement or of an element: a record of
// its type in guillemets over its name in bold, then the lines of its
// body from the left, when it has any.
func (r *requirementDiagram) node(b *reqBox) *graph.Node {
	name, font := markdownFont(label(b.name))
	font.Bold = true
	head, fonts := []string{"«" + b.typ + "»"}, []graph.Font{{}}
	for l := range strings.SplitSeq(name, "\n") {
		head, fonts = append(head, l), append(fonts, font)
	}
	n := &graph.Node{ID: b.name, Label: name, Shape: "rect", Stack: true,
		Fields: []graph.Field{{Text: strings.Join(head, "\n"), Fonts: fonts}}}
	var lines []string
	add := func(key, v string) {
		if v != "" {
			text, _ := markdownFont(label(v))
			lines = append(lines, key+": "+text)
		}
	}
	if b.element {
		add("Type", b.kind)
		add("Doc Ref", b.docRef)
	} else {
		add("ID", b.id)
		add("Text", b.text)
		add("Risk", b.risk)
		add("Verification", b.verify)
	}
	if len(lines) > 0 {
		n.Fields = append(n.Fields, graph.Field{Text: strings.Join(lines, "\n"), Align: "l"})
	}
	n.Style = classStyle(r.styles, r.nodeClasses[b.name], r.nodeStyles[b.name])
	return n
}
