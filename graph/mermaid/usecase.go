package mermaid

import (
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/xo/zuzu/expr"
	"github.com/xo/zuzu/graph"
	"github.com/xo/zuzu/text"
	"github.com/xo/zuzu/vega"
)

// ucSettings are the settings of the section usecase of the config that a
// use case diagram reads.
var ucSettings = []string{
	"wrappingWidth", "minNodeWidth", "nodeSpacing", "rankSpacing", "diagramPadding", "colorScheme",
	"actorFontSize", "actorFontFamily", "actorFontWeight", "usecaseFontSize", "usecaseFontFamily", "usecaseFontWeight",
}

// ucConfig is the config of a use case diagram: the widths in the pixels
// of Mermaid at which a label wraps and under which a use case does not
// shrink, and the styles of the fonts of the actors and of the use cases.
type ucConfig struct {
	wrap, minWidth         float64
	actorFont, usecaseFont map[string]string
}

// usecaseGraph reads a use case diagram, usecase-beta, into a graph. It
// reads the source by the grammar of Mermaid 12.3.0, and a source that
// Mermaid reads as an error is an error.
func usecaseGraph(d *Diagram, strict bool) (*graph.Graph, vega.Report, error) {
	is := &graph.Issues{Strict: strict}
	if err := sectionConfig(d, is, "usecase", ucSettings...); err != nil {
		return nil, is.Report, err
	}
	g := &graph.Graph{Directed: true, Direction: graph.LR, Title: d.Title}
	cfg, err := ucReadConfig(d, is, g)
	if err != nil {
		return nil, is.Report, err
	}
	var b strings.Builder
	starts := make([]int, 0, len(d.Lines))
	for i, l := range d.Lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		starts = append(starts, b.Len())
		b.WriteString(l.Text)
	}
	at := func(off int) int {
		i := sort.SearchInts(starts, off+1) - 1
		if i < 0 {
			return d.N
		}
		return d.Lines[i].N
	}
	body := b.String()
	lx := &ucLexer{s: body, at: at}
	if err := lx.lex(); err != nil {
		return nil, is.Report, err
	}
	p := &ucParser{src: body, toks: lx.toks, at: at}
	if err := p.parse(); err != nil {
		return nil, is.Report, err
	}
	m := &ucModel{p: p, symbols: map[string]*ucSymbol{}, states: map[string]*ucState{}, boundaries: map[string]*ucBoundary{},
		jsons: map[string]*ucJSONDraft{}, edgeIDs: map[string]*ucEdge{}, first: map[string]int{}, classDefs: map[string][]string{},
		elemClasses: map[string][]string{}, elemStyles: map[string][]string{}}
	if err := m.finish(); err != nil {
		return nil, is.Report, err
	}
	if err := m.build(d, g, cfg, is); err != nil {
		return nil, is.Report, err
	}
	return g, is.Report, nil
}

// ucReadConfig reads the settings of the section usecase of the config: the
// spaces of the layout on the graph, and the rest into a ucConfig. A value
// that does not read is an issue, and the setting keeps its default.
func ucReadConfig(d *Diagram, is *graph.Issues, g *graph.Graph) (ucConfig, error) {
	cfg := ucConfig{wrap: 120, minWidth: 120, actorFont: map[string]string{}, usecaseFont: map[string]string{}}
	m, _ := d.Config["usecase"].(map[string]any)
	if m == nil {
		return cfg, nil
	}
	bad := func(k string) error { return is.Add("config", "the setting usecase.%s: %v", k, m[k]) }
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		n, isNum := configNumber(m, k)
		switch k {
		case "wrappingWidth", "minNodeWidth", "nodeSpacing", "rankSpacing", "diagramPadding", "actorFontSize", "usecaseFontSize":
			if !isNum || n < 0 || n == 0 && (k == "wrappingWidth" || strings.HasSuffix(k, "FontSize")) {
				if err := bad(k); err != nil {
					return cfg, err
				}
				continue
			}
		}
		switch k {
		case "wrappingWidth":
			cfg.wrap = n
		case "minNodeWidth":
			cfg.minWidth = n
		case "nodeSpacing":
			g.NodeSep = n * zuzuNodeSep / mermaidNodeSpacing
		case "rankSpacing":
			g.RankSep = n * zuzuRankSep / mermaidRankSpacing
		case "diagramPadding":
			pad := n
			g.Padding = &pad
		case "actorFontSize":
			cfg.actorFont["fontSize"] = strconv.FormatFloat(n, 'f', -1, 64)
		case "usecaseFontSize":
			cfg.usecaseFont["fontSize"] = strconv.FormatFloat(n, 'f', -1, 64)
		case "actorFontFamily", "usecaseFontFamily":
			// The name of a family gives a weight and a style, such as
			// Helvetica-Bold, and zuzu draws the rest of the name in IBM
			// Plex Sans (D32).
			s, ok := v.(string)
			if !ok {
				if err := bad(k); err != nil {
					return cfg, err
				}
				continue
			}
			font := cfg.actorFont
			if k == "usecaseFontFamily" {
				font = cfg.usecaseFont
			}
			weight, style := graph.FontFace(s)
			if _, ok := font["fontWeight"]; !ok && weight != "" {
				font["fontWeight"] = weight
			}
			if style != "" {
				font["fontStyle"] = style
			}
		case "actorFontWeight", "usecaseFontWeight":
			font := cfg.actorFont
			if k == "usecaseFontWeight" {
				font = cfg.usecaseFont
			}
			switch w := v.(type) {
			case string:
				font["fontWeight"] = strconv.Itoa(text.Weight(w))
			case float64:
				font["fontWeight"] = strconv.FormatFloat(w, 'f', -1, 64)
			default:
				if err := bad(k); err != nil {
					return cfg, err
				}
			}
		case "colorScheme":
			// The default, role, colors the actors and the use cases by
			// the colors of the theme of Mermaid, which zuzu draws in its
			// own look. rotate gives each a color of a palette of the
			// theme.
			if v != "role" {
				if err := bad(k); err != nil {
					return cfg, err
				}
			}
		}
	}
	return cfg, nil
}

// ucSymbol is a name that the diagram declares: an actor, a use case, a
// system boundary, a block of JSON or an edge with an id.
type ucSymbol struct {
	kind      string
	off       int
	generated bool
	label     string
}

// ucState is an actor or a use case, from all of its declarations.
type ucState struct {
	ucEntity

	kind            string
	parent          string
	actorType, icon string
	business        *bool
}

// ucBoundary is a system boundary: its draft, its type and its members.
type ucBoundary struct {
	ucBoundaryDraft

	typ string
}

// ucEdge is a relation with its id, its classes, its styles and its
// animation.
type ucEdge struct {
	ucRelation

	id              string
	classes, styles []string
	animate         bool
}

// ucModel checks the drafts of the statements as Mermaid does, and joins
// the declarations of each name.
type ucModel struct {
	p          *ucParser
	symbols    map[string]*ucSymbol
	states     map[string]*ucState
	stateOrder []*ucState
	boundaries map[string]*ucBoundary
	jsons      map[string]*ucJSONDraft
	edges      []*ucEdge
	edgeIDs    map[string]*ucEdge
	// first holds the offset of the first mention of each name, which
	// orders the actors, the use cases, the boundaries and the blocks.
	first map[string]int
	// classDefs holds the styles of each class. elemClasses and
	// elemStyles hold the classes and the styles of each actor, use case,
	// boundary and block of JSON.
	classDefs               map[string][]string
	elemClasses, elemStyles map[string][]string
	notes                   []ucNoteDraft
}

// errorf returns an error of the source at an offset.
func (m *ucModel) errorf(off int, format string, args ...any) error {
	return fmt.Errorf("line %d: %s: %w", m.p.at(off), fmt.Sprintf(format, args...), ErrSyntax)
}

// what names a kind of name in an error.
func ucWhat(kind string) string {
	switch kind {
	case ucActorKind:
		return "an actor"
	case ucUseCaseKind:
		return "a use case"
	case ucBoundaryKind:
		return "a system boundary"
	case ucJSONKind:
		return "a block of JSON"
	}
	return "an edge"
}

func (m *ucModel) recordFirst(id string, off int) {
	if v, ok := m.first[id]; !ok || off < v {
		m.first[id] = off
	}
}

// byFirst returns the ids in the order of their first mention.
func (m *ucModel) byFirst(ids []string) []string {
	out := slices.Clone(ids)
	slices.SortStableFunc(out, func(a, b string) int { return m.first[a] - m.first[b] })
	return out
}

// finish joins the declarations, resolves the relations, the metadata,
// the notes, the classes and the styles, and checks each as Mermaid does.
func (m *ucModel) finish() error {
	p := m.p
	for _, r := range p.relations {
		m.recordFirst(r.src.id, r.src.off)
		m.recordFirst(r.dst.id, r.dst.off)
	}
	for _, d := range p.elements {
		m.recordFirst(d.id, d.off)
	}
	for _, b := range p.boundaries {
		m.recordFirst(b.id, b.off)
	}
	for _, j := range p.jsons {
		m.recordFirst(j.id, j.off)
	}
	// The declarations, in the order of the source.
	type decl struct {
		off int
		run func() error
	}
	var decls []decl
	for i := range p.elements {
		d := &p.elements[i]
		decls = append(decls, decl{d.off, func() error { return m.element(d) }})
	}
	for i := range p.boundaries {
		b := &p.boundaries[i]
		decls = append(decls, decl{b.off, func() error { return m.boundary(b) }})
	}
	for i := range p.jsons {
		j := &p.jsons[i]
		decls = append(decls, decl{j.off, func() error {
			m.jsons[j.id] = j
			return m.unique(j.id, ucJSONKind, j.off)
		}})
	}
	for i := range p.relations {
		r := &p.relations[i]
		if r.explicitID != "" {
			decls = append(decls, decl{r.idOff, func() error { return m.unique(r.explicitID, ucEdgeKind, r.idOff) }})
		}
	}
	slices.SortStableFunc(decls, func(a, b decl) int { return a.off - b.off })
	for _, d := range decls {
		if err := d.run(); err != nil {
			return err
		}
	}
	if err := m.relations(); err != nil {
		return err
	}
	if err := m.metadata(); err != nil {
		return err
	}
	if err := m.check(); err != nil {
		return err
	}
	if err := m.resolveNotes(); err != nil {
		return err
	}
	return m.classesAndStyles()
}

// element joins a declaration of an actor or a use case.
func (m *ucModel) element(d *ucDraft) error {
	origin := m.symbols[d.id]
	if origin != nil && origin.kind != d.kind {
		return m.errorf(d.off, "%s is %s and %s", d.id, ucWhat(origin.kind), ucWhat(d.kind))
	}
	if origin != nil && (origin.generated || d.generated) {
		return m.errorf(d.off, "the id %s, which Mermaid makes from a name in quotes, is declared twice", d.id)
	}
	s := m.states[d.id]
	if s == nil {
		s = &ucState{ucEntity: d.ucEntity, kind: d.kind, parent: d.parent}
		s.classes = slices.Clone(d.classes)
		if err := m.declMeta(s, d.meta); err != nil {
			return err
		}
		m.states[d.id] = s
		m.stateOrder = append(m.stateOrder, s)
		m.symbols[d.id] = &ucSymbol{kind: d.kind, off: d.off, generated: d.generated, label: d.label.text}
		return nil
	}
	switch {
	case s.label != d.label:
		return m.errorf(d.off, "%s has two labels", d.id)
	case d.shape != "" && s.shape != "" && d.shape != s.shape:
		return m.errorf(d.off, "the use case %s has two shapes", d.id)
	case d.parent != "" && s.parent != "" && d.parent != s.parent:
		return m.errorf(d.off, "%s is in two system boundaries", d.id)
	case d.stereotype != "" && s.stereotype != "" && d.stereotype != s.stereotype:
		return m.errorf(d.off, "%s has two stereotypes", d.id)
	}
	if s.shape == "" {
		s.shape = d.shape
	}
	if s.parent == "" {
		s.parent = d.parent
	}
	if s.stereotype == "" {
		s.stereotype = d.stereotype
	}
	s.classes = pushUnique(s.classes, d.classes)
	return m.declMeta(s, d.meta)
}

// pushUnique appends the values that a list does not hold yet.
func pushUnique(list, values []string) []string {
	for _, v := range values {
		if !slices.Contains(list, v) {
			list = append(list, v)
		}
	}
	return list
}

// boundary joins a declaration of a system boundary.
func (m *ucModel) boundary(d *ucBoundaryDraft) error {
	origin := m.symbols[d.id]
	if origin != nil && origin.kind != ucBoundaryKind {
		return m.errorf(d.off, "%s is %s and a system boundary", d.id, ucWhat(origin.kind))
	}
	if origin != nil && (origin.generated || d.generated) {
		return m.errorf(d.off, "the id %s, which Mermaid makes from a name in quotes, is declared twice", d.id)
	}
	if b := m.boundaries[d.id]; b != nil {
		if b.label != d.label || b.markdown != d.markdown {
			return m.errorf(d.off, "the system boundary %s has two titles", d.id)
		}
		b.classes = pushUnique(b.classes, d.classes)
		return nil
	}
	m.boundaries[d.id] = &ucBoundary{ucBoundaryDraft: *d}
	m.boundaries[d.id].classes = slices.Clone(d.classes)
	m.symbols[d.id] = &ucSymbol{kind: ucBoundaryKind, off: d.off, generated: d.generated, label: d.label}
	return nil
}

// unique declares a block of JSON or the id of an edge, which a name can
// declare once.
func (m *ucModel) unique(id, kind string, off int) error {
	switch prev := m.symbols[id]; {
	case prev != nil && prev.kind == kind:
		return m.errorf(off, "%s is declared twice", id)
	case prev != nil:
		return m.errorf(off, "%s is %s and %s", id, ucWhat(prev.kind), ucWhat(kind))
	}
	m.symbols[id] = &ucSymbol{kind: kind, off: off}
	return nil
}

// relations resolves the ends of each relation. A name that no statement
// declares is a use case.
func (m *ucModel) relations() error {
	n := 0
	for _, r := range m.p.relations {
		kinds := [2]string{}
		for i, end := range []ucEnd{r.src, r.dst} {
			if end.classes && !end.declaration {
				return m.errorf(end.off, "the end %s of a relation has classes after ::: and no declaration", end.id)
			}
			if s := m.symbols[end.id]; s != nil {
				kinds[i] = s.kind
				continue
			}
			s := &ucState{kind: ucUseCaseKind}
			s.id, s.label, s.off, s.generated, s.shape = end.id, end.label, end.off, end.generated, ucEllipse
			m.states[end.id] = s
			m.stateOrder = append(m.stateOrder, s)
			m.symbols[end.id] = &ucSymbol{kind: ucUseCaseKind, off: end.off, generated: end.generated, label: end.label.text}
			m.recordFirst(end.id, end.off)
			kinds[i] = ucUseCaseKind
		}
		if err := m.checkRelation(r, kinds); err != nil {
			return err
		}
		e := &ucEdge{ucRelation: r, id: r.explicitID}
		if e.id == "" {
			e.id = "edge-" + strconv.Itoa(n)
			n++
		} else {
			m.edgeIDs[e.id] = e
		}
		m.edges = append(m.edges, e)
	}
	return nil
}

// checkRelation checks the kinds of the ends of a relation.
func (m *ucModel) checkRelation(r ucRelation, kinds [2]string) error {
	for i, end := range []ucEnd{r.src, r.dst} {
		switch kinds[i] {
		case ucActorKind, ucUseCaseKind, ucJSONKind:
		default:
			return m.errorf(end.off, "a relation cannot join %s %s", ucWhat(kinds[i]), end.id)
		}
	}
	switch {
	case (r.typ == ucIncludes || r.typ == ucExtends) && (kinds[0] != ucUseCaseKind || kinds[1] != ucUseCaseKind):
		return m.errorf(r.off, "the relation %s must join two use cases", r.typ)
	case r.typ == ucGeneralizes && (kinds[0] != kinds[1] || kinds[0] == ucJSONKind):
		return m.errorf(r.off, "a generalization must join two actors or two use cases")
	case r.typ == ucAssociation && (kinds[0] == ucJSONKind || kinds[1] == ucJSONKind) && r.arrow > ucLineSolid:
		return m.errorf(r.off, "a relation with a block of JSON must be -->, <-- or --")
	}
	return nil
}

// declMeta applies the metadata of a declaration. A value that differs
// from the value of an earlier declaration is an error.
func (m *ucModel) declMeta(s *ucState, props []ucProp) error {
	for _, prop := range props {
		if err := m.prop(s, prop, false); err != nil {
			return err
		}
	}
	return nil
}

// prop applies a property of metadata to an actor or a use case. replace
// says that a statement of metadata gives it, which wins over a
// declaration.
func (m *ucModel) prop(s *ucState, prop ucProp, replace bool) error {
	str, isStr := prop.value.(string)
	b, isBool := prop.value.(bool)
	set := func(cur *string) error {
		if !replace && *cur != "" && *cur != str {
			return m.errorf(prop.off, "the actor %s has two values of %s", s.id, prop.key)
		}
		*cur = str
		return nil
	}
	switch {
	case prop.key == "business" && isBool:
		if !replace && s.business != nil && *s.business != b {
			return m.errorf(prop.off, "%s has two values of business", s.id)
		}
		s.business = &b
		return nil
	case s.kind != ucActorKind:
	case prop.key == "type" && isStr && (str == "normal" || str == "hollow" || str == "awesome"):
		return set(&s.actorType)
	case prop.key == "icon" && isStr:
		return set(&s.icon)
	}
	return m.errorf(prop.off, "the metadata %s is not a property of %s %s", prop.key, ucWhat(s.kind), s.id)
}

// metadata applies each statement of metadata, as A@{ type: hollow }.
func (m *ucModel) metadata() error {
	for _, md := range m.p.metas {
		origin := m.symbols[md.target]
		if origin == nil {
			return m.errorf(md.off, "the metadata of %s, which the diagram does not declare", md.target)
		}
		for _, prop := range md.props {
			var err error
			switch origin.kind {
			case ucActorKind, ucUseCaseKind:
				err = m.prop(m.states[md.target], prop, true)
			case ucBoundaryKind:
				if v, _ := prop.value.(string); prop.key != "type" || v != "rect" && v != "package" {
					err = m.errorf(prop.off, "the metadata %s is not a property of the system boundary %s", prop.key, md.target)
				} else {
					m.boundaries[md.target].typ = v
				}
			case ucEdgeKind:
				e := m.edgeIDs[md.target]
				switch v := prop.value.(type) {
				case bool:
					if prop.key != "animate" {
						err = m.errorf(prop.off, "the metadata %s is not a property of the edge %s", prop.key, md.target)
					}
					e.animate = v
				case string:
					if prop.key != "animation" || v != "fast" && v != "slow" {
						err = m.errorf(prop.off, "the metadata %s is not a property of the edge %s", prop.key, md.target)
					}
					e.animate = true
				}
			default:
				err = m.errorf(prop.off, "the metadata %s is not a property of the block of JSON %s", prop.key, md.target)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// check checks the actors and the use cases after their metadata.
func (m *ucModel) check() error {
	for _, s := range m.stateOrder {
		business := s.business != nil && *s.business
		switch {
		case s.kind == ucActorKind && s.icon != "" && s.actorType != "" && s.actorType != "normal":
			return m.errorf(s.off, "the actor %s has an icon and the type %s", s.id, s.actorType)
		case s.kind == ucActorKind && business && (s.icon != "" || s.actorType == "awesome"):
			return m.errorf(s.off, "the business actor %s must be normal or hollow", s.id)
		case s.kind == ucUseCaseKind && s.shape == ucRect && business:
			return m.errorf(s.off, "the use case %s in brackets cannot be a business use case", s.id)
		}
	}
	return nil
}

// resolveNotes checks the target of each note.
func (m *ucModel) resolveNotes() error {
	for _, n := range m.p.notes {
		origin := m.symbols[n.target]
		if origin == nil {
			return m.errorf(n.off, "the note for %s, which the diagram does not declare", n.target)
		}
		if origin.kind != ucActorKind && origin.kind != ucUseCaseKind {
			return m.errorf(n.off, "a note must be for an actor or a use case, and %s is %s", n.target, ucWhat(origin.kind))
		}
		m.notes = append(m.notes, n)
	}
	return nil
}

// classesAndStyles applies the statements classDef, class and style. A
// later classDef of a name replaces an earlier one.
func (m *ucModel) classesAndStyles() error {
	for _, d := range m.p.classDefs {
		for _, id := range d.ids {
			m.classDefs[id] = d.styles
		}
	}
	for _, s := range m.stateOrder {
		m.elemClasses[s.id] = s.classes
	}
	for id, b := range m.boundaries {
		m.elemClasses[id] = b.classes
	}
	for id, j := range m.jsons {
		m.elemClasses[id] = j.classes
	}
	for _, c := range m.p.classes {
		for _, t := range c.targets {
			e, err := m.styled(t)
			if err != nil {
				return err
			}
			if e != nil {
				e.classes = pushUnique(e.classes, c.classes)
				continue
			}
			m.elemClasses[t.id] = pushUnique(m.elemClasses[t.id], c.classes)
		}
	}
	for _, st := range m.p.styles {
		e, err := m.styled(st.target)
		if err != nil {
			return err
		}
		if e != nil {
			e.styles = append(e.styles, st.styles...)
			continue
		}
		m.elemStyles[st.target.id] = append(m.elemStyles[st.target.id], st.styles...)
	}
	return nil
}

// styled returns the edge that a statement class or style names, or nil
// for an actor, a use case, a system boundary or a block of JSON. Any other
// name is an error.
func (m *ucModel) styled(t ucTarget) (*ucEdge, error) {
	if e := m.edgeIDs[t.id]; e != nil {
		return e, nil
	}
	if m.symbols[t.id] != nil {
		return nil, nil
	}
	return nil, m.errorf(t.off, "the class or the style of %s, which is not an actor, a use case, a system boundary, a block of JSON or an edge with an id", t.id)
}

// ucText returns the text of a label and the font that all of it shares:
// a string of markdown without its marks, or a text as it is written. The
// entities, such as `#quot;`, read as their characters.
func ucText(l ucLabel) (string, graph.Font) {
	if l.markdown {
		return markdownFont(labelText(l.text))
	}
	return strings.TrimSpace(decodeEntities(l.text)), graph.Font{}
}

// The size of the box of the figure of an actor, and the scale of the
// stick figure in it.
const (
	ucFigureWidth  = 36.0
	ucFigureHeight = 45.0
	ucStickScale   = 1.5
)

// ucFigure returns the path of the figure of an actor around the center of
// its box: a stick figure, an outline for hollow, or a filled silhouette
// for awesome. A business actor has a line across its head.
func ucFigure(typ string, business bool) string {
	var b strings.Builder
	var hy, hr float64
	switch typ {
	case "hollow":
		hy, hr = -15, 6
		b.WriteString(ringPath(0, hy, hr))
		// The body is an outline: the arms, and the legs apart.
		b.WriteString("M-13,-6H13V-1H5L12,13L6,18L0,8L-6,18L-12,13L-5,-1H-13Z")
	case "awesome":
		hy, hr = -13, 8
		b.WriteString(ringPath(0, hy, hr))
		// The shoulders are a dome under the head.
		b.WriteString("M-15,18C-15,5 -8,-1 0,-1C8,-1 15,5 15,18Z")
	default:
		hy, hr = -8*ucStickScale, 5*ucStickScale
		b.WriteString(stickFigure(ucStickScale))
	}
	if business {
		// A line across the head, at 60 degrees, a little off its center.
		off, half := hr*0.6, hr*0.8
		dx, dy := math.Cos(math.Pi/3), -math.Sin(math.Pi/3)
		cx, cy := off*-dy, hy+off*dx
		b.WriteString("M" + pathPoint(cx-half*dx, cy-half*dy) + "L" + pathPoint(cx+half*dx, cy+half*dy))
	}
	return b.String()
}

// ucIconSize is the size of the icon of an actor with an icon.
const ucIconSize = 28.0

// build writes the actors, the use cases, the notes and the blocks of JSON
// as the nodes of the graph, the system boundaries as its clusters, and
// the relations and the links of the notes as its edges.
func (m *ucModel) build(d *Diagram, g *graph.Graph, cfg ucConfig, is *graph.Issues) error {
	if n := len(m.p.directions); n > 0 {
		g.Direction = m.p.directions[n-1]
	}
	classes := map[string]map[string]string{}
	addf := func(format string, args ...any) error { return is.Add("style", format, args...) }
	for _, id := range slices.Sorted(maps.Keys(m.classDefs)) {
		st, err := ucCSS(m.classDefs[id], addf)
		if err != nil {
			return err
		}
		classes[id] = st
	}
	style := func(cls, own []string) (map[string]string, error) {
		st, err := ucCSS(own, addf)
		if err != nil {
			return nil, err
		}
		return classStyle(classes, cls, st), nil
	}
	w := labelWrap(d.Config, g)
	// Mermaid draws an actor and a note at 14 pixels and a use case at
	// 12, so the label of each wraps at the ratio of its own size (D32).
	wrap, caseWrap := scaleWrap(d.Kind, "", cfg.wrap), scaleWrap(d.Kind, labelUsecase, cfg.wrap)
	under := func(st, more map[string]string) {
		for k, v := range more {
			if _, ok := st[k]; !ok {
				st[k] = v
			}
		}
	}
	var actors, usecases []*ucState
	for _, id := range m.byFirst(slices.Collect(maps.Keys(m.states))) {
		if s := m.states[id]; s.kind == ucActorKind {
			actors = append(actors, s)
		} else {
			usecases = append(usecases, s)
		}
	}
	labelOf := func(s *ucState) (string, graph.Font) {
		t, font := ucText(s.label)
		if s.stereotype != "" {
			t = "«" + decodeEntities(s.stereotype) + "»\n" + t
		}
		return t, font
	}
	for _, s := range actors {
		t, font := labelOf(s)
		st, err := style(m.elemClasses[s.id], m.elemStyles[s.id])
		if err != nil {
			return err
		}
		under(st, fontStyle(font))
		under(st, cfg.actorFont)
		n := &graph.Node{ID: s.id, Label: t, Shape: ucRect, Cluster: s.parent, Style: st, Wrap: w.width(wrap, s.label.markdown)}
		business := s.business != nil && *s.business
		if s.icon != "" {
			name := s.icon
			if !strings.Contains(name, ":") {
				name = "fa:" + name
			}
			n.Media = &graph.Media{Icon: name, Form: "rounded", Height: ucIconSize}
		} else {
			n.Media = &graph.Media{Path: ucFigure(s.actorType, business), Width: ucFigureWidth, Height: ucFigureHeight}
			if s.actorType == "hollow" {
				st["fillOpacity"] = "0"
			}
		}
		g.Nodes = append(g.Nodes, n)
	}
	for _, s := range usecases {
		t, font := labelOf(s)
		st, err := style(m.elemClasses[s.id], m.elemStyles[s.id])
		if err != nil {
			return err
		}
		under(st, fontStyle(font))
		under(st, cfg.usecaseFont)
		shape, pad := s.shape, 20.0
		switch {
		case shape == ucRect:
			pad = 10
		case s.business != nil && *s.business:
			shape = "businessEllipse"
		}
		g.Nodes = append(g.Nodes, &graph.Node{ID: s.id, Label: t, Shape: shape, Cluster: s.parent, Style: st,
			Wrap: w.width(caseWrap, s.label.markdown), MinWidth: scaleWrap(d.Kind, labelUsecase, cfg.minWidth+2*pad)})
	}
	for i, n := range m.notes {
		t, font := ucText(n.label)
		g.Nodes = append(g.Nodes, &graph.Node{ID: "note-" + strconv.Itoa(i), Label: t, Shape: "note", Style: fontStyle(font),
			Wrap: w.width(wrap, n.label.markdown), MinWidth: scaleWrap(d.Kind, "", cfg.minWidth+20)})
	}
	for _, id := range m.byFirst(slices.Collect(maps.Keys(m.jsons))) {
		j := m.jsons[id]
		st, err := style(m.elemClasses[id], m.elemStyles[id])
		if err != nil {
			return err
		}
		n := &graph.Node{ID: id, Label: id, Shape: ucRect, Stack: true, Style: st, Fields: []graph.Field{{Text: id}}}
		for _, r := range j.rows {
			n.Fields = append(n.Fields, graph.Field{Cells: []string{r[0], r[1]}})
		}
		g.Nodes = append(g.Nodes, n)
	}
	for _, id := range m.byFirst(slices.Collect(maps.Keys(m.boundaries))) {
		b := m.boundaries[id]
		t, font := ucText(ucLabel{text: b.label, markdown: b.markdown})
		st, err := style(m.elemClasses[id], m.elemStyles[id])
		if err != nil {
			return err
		}
		under(st, fontStyle(font))
		c := &graph.Cluster{ID: id, Label: t, Style: st}
		if b.typ == "package" {
			// A boundary of the type package draws with a tab at its top
			// left, as a package of UML (D32).
			c.Shape = graph.ClusterPackage
		}
		g.Clusters = append(g.Clusters, c)
	}
	for _, e := range m.edges {
		ge, err := m.edge(e, classes, addf)
		if err != nil {
			return err
		}
		g.Edges = append(g.Edges, ge)
	}
	for i, n := range m.notes {
		g.Edges = append(g.Edges, &graph.Edge{Source: "note-" + strconv.Itoa(i), Target: n.target, Head: "none", Tail: "none",
			Style: map[string]string{"strokeDash": dashed}})
	}
	g.Description = describe(d, m.p.acc...)
	return nil
}

// edge returns the edge of a relation.
func (m *ucModel) edge(e *ucEdge, classes map[string]map[string]string, addf func(string, ...any) error) (*graph.Edge, error) {
	own, err := ucCSS(e.styles, addf)
	if err != nil {
		return nil, err
	}
	st := classStyle(classes, e.classes, own)
	// An edge is a line, and a fill of its class changes nothing.
	delete(st, "fill")
	delete(st, "fillOpacity")
	ge := &graph.Edge{Source: e.src.id, Target: e.dst.id, MinLen: e.minlen, Head: "none", Tail: "none", Style: st}
	switch e.typ {
	case ucIncludes, ucExtends:
		ge.Head, ge.Label = "normal", "«"+e.typ+"»"
		if _, ok := st["strokeDash"]; !ok {
			st["strokeDash"] = dashed
		}
	case ucGeneralizes:
		ge.Head = "empty"
	default:
		switch e.arrow {
		case ucSolidArrow:
			ge.Head = "normal"
		case ucBackArrow:
			ge.Tail = "normal"
		case ucCircleArrow:
			ge.Head = "circle"
		case ucCrossArrow:
			ge.Head = "cross"
		case ucCircleBack:
			ge.Tail = "circle"
		case ucCrossBack:
			ge.Tail = "cross"
		}
		if e.label != nil {
			t, font := ucText(*e.label)
			ge.Label = t
			for k, v := range fontStyle(font) {
				if _, ok := st[k]; !ok {
					st[k] = v
				}
			}
		}
	}
	if ge.Label != "" {
		ge.Wrap = scaleWrap("usecase-beta", "", edgeWrap)
	}
	// Mermaid moves the dash of an animated edge, and zuzu draws one still
	// frame of it (D32).
	if e.animate {
		st["strokeDash"] = animatedDash
	}
	return ge, nil
}

// ucCSS reads the properties of the styles of a class or of a name, each
// as name:value, into the names of Vega. A later property wins.
func ucCSS(styles []string, add func(format string, args ...any) error) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range styles {
		// A comma of a value, as in rgb(1,2,3), stays in the value.
		st, err := readCSS(strings.ReplaceAll(s, ",", "\x00"), false, false, add)
		if err != nil {
			return nil, err
		}
		for k, v := range st {
			out[k] = strings.ReplaceAll(v, "\x00", ",")
		}
	}
	return out, nil
}

// jsonRows reads an object of JSON into the rows of a table: the path of
// each value and the value, in the order of the source. An array of
// values takes a row for each, with the path in the first. An empty
// array or object is a row of [] or {}.
func jsonRows(src string) ([][2]string, error) {
	dec := jsontext.NewDecoder(strings.NewReader(src), jsontext.AllowDuplicateNames(true))
	v, err := readJSON(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.ReadToken(); !errors.Is(err, io.EOF) {
		return nil, errors.New("more after the object")
	}
	if v.kind != '{' {
		return nil, errors.New("the value is not an object")
	}
	var rows [][2]string
	var visit func(v *jsonValue, path string)
	visit = func(v *jsonValue, path string) {
		switch v.kind {
		case '[':
			if len(v.items) == 0 {
				rows = append(rows, [2]string{path, "[]"})
				return
			}
			scalars := true
			for _, it := range v.items {
				scalars = scalars && it.kind != '[' && it.kind != '{'
			}
			for i, it := range v.items {
				switch {
				case scalars && i == 0:
					rows = append(rows, [2]string{path, it.text})
				case scalars:
					rows = append(rows, [2]string{"", it.text})
				default:
					visit(it, path+"["+strconv.Itoa(i)+"]")
				}
			}
		case '{':
			if len(v.keys) == 0 {
				rows = append(rows, [2]string{path, "{}"})
				return
			}
			for _, k := range v.keys {
				child := k
				if path != "" {
					child = path + "." + k
				}
				visit(v.fields[k], child)
			}
		default:
			rows = append(rows, [2]string{path, v.text})
		}
	}
	visit(v, "")
	return rows, nil
}

// jsonValue is a value of JSON that keeps the order of the keys of an
// object: its kind, as the first character of its token, its text as
// JavaScript writes a scalar, and its items or its fields.
type jsonValue struct {
	kind   byte
	text   string
	items  []*jsonValue
	keys   []string
	fields map[string]*jsonValue
}

// readJSON reads a value of JSON. A key that comes twice keeps its first
// place and its last value, as JSON.parse of JavaScript does.
func readJSON(dec *jsontext.Decoder) (*jsonValue, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, fmt.Errorf("reading JSON: %w", err)
	}
	v := &jsonValue{kind: byte(tok.Kind())}
	switch v.kind {
	case '[':
		for dec.PeekKind() != ']' {
			it, err := readJSON(dec)
			if err != nil {
				return nil, err
			}
			v.items = append(v.items, it)
		}
		_, err = dec.ReadToken()
	case '{':
		v.fields = map[string]*jsonValue{}
		for dec.PeekKind() != '}' {
			k, err := dec.ReadToken()
			if err != nil {
				return nil, fmt.Errorf("reading JSON: %w", err)
			}
			name := k.String()
			it, err := readJSON(dec)
			if err != nil {
				return nil, err
			}
			if _, ok := v.fields[name]; !ok {
				v.keys = append(v.keys, name)
			}
			v.fields[name] = it
		}
		_, err = dec.ReadToken()
	case '"':
		v.text = tok.String()
	case '0':
		f, _ := strconv.ParseFloat(tok.String(), 64)
		v.text = expr.String(f)
	case 'n':
		v.text = "null"
	case 't', 'f':
		v.text = strconv.FormatBool(tok.Bool())
	}
	if err != nil {
		return nil, fmt.Errorf("reading JSON: %w", err)
	}
	return v, nil
}
