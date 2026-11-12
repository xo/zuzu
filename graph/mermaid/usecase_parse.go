package mermaid

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
)

// The types of a relation of a use case diagram.
const (
	ucAssociation  = "association"
	ucIncludes     = "include"
	ucExtends      = "extend"
	ucGeneralizes  = "generalization"
	ucActorKind    = "actor"
	ucUseCaseKind  = "usecase"
	ucBoundaryKind = "boundary"
	ucJSONKind     = "json"
	ucEdgeKind     = "edge"
	ucEllipse      = "ellipse"
	ucRect         = "rect"
)

// The arrows of an association, as Mermaid numbers them: an arrowhead at
// the end, at the start, none, a circle or a cross at the end, and a
// circle or a cross at the start.
const (
	ucSolidArrow = iota
	ucBackArrow
	ucLineSolid
	ucCircleArrow
	ucCrossArrow
	ucCircleBack
	ucCrossBack
)

// ucLabel is a label: its text, and whether it is a string of markdown,
// in "` and `".
type ucLabel struct {
	text     string
	markdown bool
}

// ucProp is a property of metadata, such as type: hollow. Its value is a
// string or a boolean.
type ucProp struct {
	key   string
	value any
	off   int
}

// ucEntity is the name of an actor or of a use case as a statement writes
// it, with its label, its shape, its metadata, its stereotype and its
// classes.
type ucEntity struct {
	id         string
	label      ucLabel
	off        int
	generated  bool
	shape      string
	meta       []ucProp
	stereotype string
	classes    []string
	// declared says that the name declares an element: it has a label in
	// brackets, metadata or a stereotype, or it stands alone.
	declared bool
}

// ucDraft is a declaration of an actor or of a use case, in the boundary
// parent, or in none.
type ucDraft struct {
	ucEntity

	kind   string
	parent string
}

// ucBoundaryDraft is a declaration of a system boundary.
type ucBoundaryDraft struct {
	id, label string
	markdown  bool
	off       int
	generated bool
	classes   []string
}

// ucJSONDraft is a block of JSON: its rows of keys and values.
type ucJSONDraft struct {
	id      string
	off     int
	rows    [][2]string
	classes []string
}

// ucEnd is an end of a relation. declaration says that the statement
// declares it, and classes that it has classes after :::.
type ucEnd struct {
	ucEntity

	declaration, classes bool
}

// ucRelation is a relation from one name to another.
type ucRelation struct {
	src, dst   ucEnd
	off        int
	explicitID string
	idOff      int
	typ        string
	arrow      int
	label      *ucLabel
	minlen     int
}

// ucNoteDraft is a note for an actor or a use case.
type ucNoteDraft struct {
	target string
	off    int
	label  ucLabel
}

// ucMetaDraft is a statement that gives metadata to a name.
type ucMetaDraft struct {
	target string
	off    int
	props  []ucProp
}

// ucTarget is a name that a statement class or style names.
type ucTarget struct {
	id  string
	off int
}

// ucClassDraft is a statement class: its targets and its classes.
type ucClassDraft struct {
	targets []ucTarget
	classes []string
}

// ucStyleDraft is a statement style.
type ucStyleDraft struct {
	target ucTarget
	styles []string
}

// ucClassDefDraft is a statement classDef.
type ucClassDefDraft struct {
	ids    []string
	styles []string
}

// ucParser reads the tokens of a use case diagram by the grammar of Mermaid
// 12.3.0, and keeps the drafts of the statements for ucModel.
type ucParser struct {
	src  string
	toks []ucToken
	pos  int
	at   func(off int) int
	// parent is the system boundary whose body the parser reads, or nil.
	parent *ucBoundaryDraft
	// jsonErr is the first block of JSON that does not read. Mermaid
	// reads the JSON after the grammar, so a syntax error wins over it.
	jsonErr error

	elements   []ucDraft
	boundaries []ucBoundaryDraft
	jsons      []ucJSONDraft
	relations  []ucRelation
	notes      []ucNoteDraft
	metas      []ucMetaDraft
	classes    []ucClassDraft
	styles     []ucStyleDraft
	classDefs  []ucClassDefDraft
	directions []string
	acc        []string
}

// peek returns the token k ahead, from 1.
func (p *ucParser) peek(k int) ucToken {
	if i := p.pos + k - 1; i < len(p.toks) {
		return p.toks[i]
	}
	return p.toks[len(p.toks)-1]
}

func (p *ucParser) next() ucToken {
	t := p.peek(1)
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

// fail returns the error of a token that the grammar does not expect.
func (p *ucParser) fail(t ucToken, want string) error {
	found := fmt.Sprintf("%q", t.text)
	switch t.kind {
	case ucEOF:
		found = "the end of the diagram"
	case ucNewline:
		found = "the end of the line"
	}
	return fmt.Errorf("line %d: expected %s, found %s: %w", p.at(t.off), want, found, ErrSyntax)
}

// expect reads a token of one of the kinds.
func (p *ucParser) expect(want string, kinds ...ucKind) (ucToken, error) {
	t := p.peek(1)
	if slices.Contains(kinds, t.kind) {
		return p.next(), nil
	}
	return t, p.fail(t, want)
}

// lineEnd reads the end of a statement: a new line or the end of the
// diagram.
func (p *ucParser) lineEnd() error {
	if t := p.peek(1); t.kind == ucEOF {
		return nil
	}
	_, err := p.expect("the end of the line", ucNewline)
	return err
}

// parse reads the statements of the body.
func (p *ucParser) parse() error {
	for {
		switch p.peek(1).kind {
		case ucEOF:
			return p.jsonErr
		case ucNewline:
			p.next()
		case ucComment:
			p.next()
			if err := p.lineEnd(); err != nil {
				return err
			}
		default:
			if err := p.statement(); err != nil {
				return err
			}
		}
	}
}

// ucForbidden are the statements of PlantUML that Mermaid reads as errors.
var ucForbidden = map[string]bool{"allowmixing": true, "newpage": true, "package": true, "rectangle": true, "skinparam": true}

// statement reads a statement.
func (p *ucParser) statement() error {
	t := p.peek(1)
	switch t.kind {
	case ucAccTitle, ucAccDescr:
		p.next()
		_, text, _ := strings.Cut(t.text, ":")
		p.acc = append(p.acc, strings.TrimSpace(text))
		return p.lineEnd()
	case ucAccDescrBlock:
		p.next()
		text := t.text[strings.IndexByte(t.text, '{')+1 : strings.LastIndexByte(t.text, '}')]
		p.acc = append(p.acc, strings.TrimSpace(text))
		return p.lineEnd()
	case ucDirection:
		p.next()
		d, err := p.expect("a direction", ucTD, ucTB, ucBT, ucLR, ucRL)
		if err != nil {
			return err
		}
		dir := d.text
		if dir == "TD" {
			dir = "TB"
		}
		p.directions = append(p.directions, dir)
		return p.lineEnd()
	case ucActor:
		return p.actorStatement()
	case ucBoundaryKw:
		return p.boundary()
	case ucNote:
		return p.note()
	case ucJSONStart:
		return p.json()
	case ucClassDef:
		return p.classDef()
	case ucClass:
		return p.class()
	case ucStyle:
		return p.style()
	case ucIdent, ucString, ucMarkdown:
		if p.peek(2).kind == ucMetaStart {
			return p.metaStatement()
		}
		if t.kind == ucIdent && ucForbidden[strings.ToLower(t.text)] {
			return fmt.Errorf("line %d: %s is a statement of PlantUML, which Mermaid does not read: %w", p.at(t.off), t.text, ErrSyntax)
		}
		return p.entityStatement()
	}
	return p.fail(t, "a statement")
}

// tokenLabel returns the label of a token: the text in a string, or the
// text of a word.
func tokenLabel(t ucToken) ucLabel {
	switch t.kind {
	case ucMarkdown:
		return ucLabel{text: t.text[2 : len(t.text)-2], markdown: true}
	case ucString:
		return ucLabel{text: t.text[1 : len(t.text)-1]}
	}
	return ucLabel{text: t.text}
}

// textOff returns the offset of the text of a token, after its quotes.
func textOff(t ucToken) int {
	switch t.kind {
	case ucMarkdown:
		return t.off + 2
	case ucString:
		return t.off + 1
	}
	return t.off
}

// generatedID returns the id that Mermaid gives to a name in quotes: each
// character that is not \w of JavaScript becomes an underscore, one for
// each unit of UTF-16.
func generatedID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && isWordByte(byte(r&0x7f)) {
			b.WriteRune(r)
			continue
		}
		b.WriteString(strings.Repeat("_", utf16.RuneLen(r)))
	}
	return b.String()
}

// nodeLabel reads a label in parentheses or in brackets: a string, or
// words and marks up to the end mark.
func (p *ucParser) nodeLabel() (ucLabel, error) {
	t := p.peek(1)
	switch {
	case t.kind == ucString || t.kind == ucMarkdown:
		p.next()
		return tokenLabel(t), nil
	case !t.isLabel():
		return ucLabel{}, p.fail(t, "a label")
	}
	start, end := t.off, t.off
	for p.peek(1).isLabel() {
		u := p.next()
		end = u.off + len(u.text)
	}
	return ucLabel{text: p.src[start:end]}, nil
}

// name reads the name of an actor, of a use case or of a boundary: an id,
// with a label in parentheses, or in brackets when brackets is true, or a
// string. A label in brackets gives the shape rect, and one in
// parentheses the shape ellipse.
func (p *ucParser) name(brackets bool) (ucEntity, error) {
	t := p.peek(1)
	switch t.kind {
	case ucString, ucMarkdown:
		p.next()
		l := tokenLabel(t)
		return ucEntity{id: generatedID(l.text), label: l, off: textOff(t), generated: true}, nil
	case ucIdent:
	default:
		return ucEntity{}, p.fail(t, "a name")
	}
	p.next()
	e := ucEntity{id: t.text, label: ucLabel{text: t.text}, off: t.off}
	open := p.peek(1).kind
	if open != ucLParen && (!brackets || open != ucLBracket) {
		return e, nil
	}
	p.next()
	l, err := p.nodeLabel()
	if err != nil {
		return e, err
	}
	closing, shape := ucRParen, ucEllipse
	if open == ucLBracket {
		closing, shape = ucRBracket, ucRect
	}
	if _, err := p.expect("the end of the label", closing); err != nil {
		return e, err
	}
	e.label, e.shape = l, shape
	return e, nil
}

// suffix reads the metadata, the stereotype and the classes after a name.
func (p *ucParser) suffix(e *ucEntity) error {
	if p.peek(1).kind == ucMetaStart {
		props, err := p.metadata()
		if err != nil {
			return err
		}
		e.meta = props
	}
	if p.peek(1).kind == ucStereoStart {
		p.next()
		t, err := p.expect("the text of a stereotype", ucStereoText)
		if err != nil {
			return err
		}
		if _, err := p.expect(">>", ucStereoEnd); err != nil {
			return err
		}
		e.stereotype = strings.TrimSpace(t.text)
	}
	if p.peek(1).kind == ucClassSep {
		classes, err := p.classSuffix()
		if err != nil {
			return err
		}
		e.classes = classes
	}
	return nil
}

// classSuffix reads the classes after :::.
func (p *ucParser) classSuffix() ([]string, error) {
	p.next()
	var out []string
	for {
		t, err := p.expect("the name of a class", ucIdent)
		if err != nil {
			return nil, err
		}
		out = append(out, t.text)
		if p.peek(1).kind != ucComma {
			return out, nil
		}
		p.next()
	}
}

// actorItem reads an actor of a statement actor.
func (p *ucParser) actorItem() (ucEntity, error) {
	e, err := p.name(false)
	if err != nil {
		return e, err
	}
	err = p.suffix(&e)
	return e, err
}

// entity reads the name of a use case, or of an end of a relation.
func (p *ucParser) entity() (ucEntity, error) {
	e, err := p.name(true)
	if err != nil {
		return e, err
	}
	if err := p.suffix(&e); err != nil {
		return e, err
	}
	e.declared = e.shape != "" || e.meta != nil || e.stereotype != ""
	return e, nil
}

// draft adds the declaration of an element, in the current boundary.
func (p *ucParser) draft(e ucEntity, kind string) {
	d := ucDraft{ucEntity: e, kind: kind}
	if p.parent != nil {
		d.parent = p.parent.id
	}
	p.elements = append(p.elements, d)
}

// actorStatement reads actor A, actor A, B or actor A --> B.
func (p *ucParser) actorStatement() error {
	p.next()
	first, err := p.actorItem()
	if err != nil {
		return err
	}
	items := []ucEntity{first}
	var rel *ucRelation
	switch {
	case p.peek(1).kind == ucComma:
		for p.peek(1).kind == ucComma {
			p.next()
			e, err := p.actorItem()
			if err != nil {
				return err
			}
			items = append(items, e)
		}
	case p.startsRelation():
		r, err := p.relationTail()
		if err != nil {
			return err
		}
		rel = &r
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	for _, e := range items {
		p.draft(e, ucActorKind)
	}
	if rel != nil {
		if rel.dst.declared {
			p.draft(rel.dst.ucEntity, ucUseCaseKind)
		}
		rel.src = ucEnd{ucEntity: first, declaration: true, classes: len(first.classes) > 0}
		p.relations = append(p.relations, *rel)
	}
	return nil
}

// startsRelation reports whether the next tokens start the rest of a
// relation: an arrow, or the id of an edge and @.
func (p *ucParser) startsRelation() bool {
	switch p.peek(1).kind {
	case ucDependency, ucGeneralization, ucFwdSolid, ucBackSolid, ucMarkerless, ucFwdCircle, ucBackCircle, ucFwdCross, ucBackCross:
		return true
	case ucIdent:
		return true
	}
	return false
}

// entityStatement reads a use case, or a relation from a name.
func (p *ucParser) entityStatement() error {
	src, err := p.entity()
	if err != nil {
		return err
	}
	if !p.startsRelation() {
		if err := p.lineEnd(); err != nil {
			return err
		}
		src.declared = true
		if src.shape == "" {
			src.shape = ucEllipse
		}
		p.draft(src, ucUseCaseKind)
		return nil
	}
	rel, err := p.relationTail()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	if src.declared {
		p.draft(src, ucUseCaseKind)
	}
	if rel.dst.declared {
		p.draft(rel.dst.ucEntity, ucUseCaseKind)
	}
	rel.src = ucEnd{ucEntity: src, declaration: src.declared, classes: len(src.classes) > 0}
	p.relations = append(p.relations, rel)
	return nil
}

// relationTail reads the rest of a relation after its first name: the id
// of the edge and @, the arrow and the second name.
func (p *ucParser) relationTail() (ucRelation, error) {
	var r ucRelation
	if p.peek(1).kind == ucIdent && p.peek(2).kind == ucAt {
		t := p.next()
		p.next()
		r.explicitID, r.idOff = t.text, t.off
	}
	r.off = p.peek(1).off
	if r.explicitID != "" {
		r.off = r.idOff
	}
	if err := p.arrow(&r); err != nil {
		return r, err
	}
	dst, err := p.entity()
	if err != nil {
		return r, err
	}
	r.dst = ucEnd{ucEntity: dst, declaration: dst.declared, classes: len(dst.classes) > 0}
	return r, nil
}

// solidMinlen returns the least number of ranks of an arrow of a solid
// line: one less than its dashes, and at least 1.
func solidMinlen(t ucToken) int {
	return max(1, strings.Count(t.text, "-")-1)
}

// arrow reads the arrow of a relation, with its label.
func (p *ucParser) arrow(r *ucRelation) error {
	t := p.peek(1)
	r.typ, r.minlen = ucAssociation, 1
	labeled := func(after ...ucKind) (ucToken, bool, error) {
		if !p.labeledRight(after) {
			return ucToken{}, false, nil
		}
		l, err := p.edgeLabel()
		if err != nil {
			return ucToken{}, false, err
		}
		r.label = &l
		end, err := p.expect("an arrow", after...)
		return end, true, err
	}
	switch t.kind {
	case ucDependency:
		p.next()
		if _, err := p.expect("a colon", ucColon); err != nil {
			return err
		}
		k, err := p.expect("include or extend", ucInclude, ucExtend)
		if err != nil {
			return err
		}
		r.typ = ucIncludes
		if k.kind == ucExtend {
			r.typ = ucExtends
		}
		r.arrow = ucSolidArrow
	case ucGeneralization:
		p.next()
		r.typ, r.arrow = ucGeneralizes, ucSolidArrow
	case ucFwdSolid:
		p.next()
		r.arrow, r.minlen = ucSolidArrow, solidMinlen(t)
	case ucBackSolid:
		p.next()
		r.arrow, r.minlen = ucBackArrow, solidMinlen(t)
		if t.text == "<--" {
			end, ok, err := labeled(ucMarkerless)
			if err != nil {
				return err
			}
			if ok {
				r.minlen = solidMinlen(end)
			}
		}
	case ucMarkerless:
		p.next()
		r.arrow, r.minlen = ucLineSolid, solidMinlen(t)
		if t.text == "--" {
			end, ok, err := labeled(ucFwdSolid, ucMarkerless, ucFwdCircle, ucFwdCross)
			if err != nil {
				return err
			}
			if ok {
				switch end.kind {
				case ucFwdSolid:
					r.arrow, r.minlen = ucSolidArrow, solidMinlen(end)
				case ucFwdCircle:
					r.arrow, r.minlen = ucCircleArrow, 1
				case ucFwdCross:
					r.arrow, r.minlen = ucCrossArrow, 1
				default:
					r.minlen = solidMinlen(end)
				}
			}
		}
	case ucFwdCircle:
		p.next()
		r.arrow = ucCircleArrow
	case ucFwdCross:
		p.next()
		r.arrow = ucCrossArrow
	case ucBackCircle, ucBackCross:
		p.next()
		r.arrow = ucCircleBack
		if t.kind == ucBackCross {
			r.arrow = ucCrossBack
		}
		if _, _, err := labeled(ucMarkerless); err != nil {
			return err
		}
	default:
		return p.fail(t, "an arrow")
	}
	return nil
}

// labeledRight reports whether a label follows, and then one of the
// tokens after.
func (p *ucParser) labeledRight(after []ucKind) bool {
	label := func(t ucToken) bool { return t.isLabel() || t.kind == ucString || t.kind == ucMarkdown }
	if !label(p.peek(1)) {
		return false
	}
	for k := 2; ; k++ {
		t := p.peek(k)
		if slices.Contains(after, t.kind) {
			return true
		}
		if !label(t) || t.kind == ucEOF {
			return false
		}
	}
}

// edgeLabel reads the label of an edge: a string, or words and marks.
func (p *ucParser) edgeLabel() (ucLabel, error) {
	return p.nodeLabel()
}

// metadata reads @{ key: value, ... }. A comma or new lines part two
// properties.
func (p *ucParser) metadata() ([]ucProp, error) {
	p.next()
	newlines := func() {
		for p.peek(1).kind == ucNewline {
			p.next()
		}
	}
	isKey := func() bool { k := p.peek(1).kind; return k == ucIdent || k == ucString }
	newlines()
	var props []ucProp
	if isKey() {
		for {
			prop, err := p.prop()
			if err != nil {
				return nil, err
			}
			props = append(props, prop)
			save := p.pos
			switch p.peek(1).kind {
			case ucComma:
				p.next()
				newlines()
			case ucNewline:
				newlines()
				if p.peek(1).kind == ucComma {
					p.next()
					newlines()
				}
			}
			if p.pos != save && isKey() {
				continue
			}
			p.pos = save
			break
		}
		if p.peek(1).kind == ucComma {
			p.next()
		}
		newlines()
	}
	if _, err := p.expect("the end of the metadata", ucRBrace); err != nil {
		return nil, err
	}
	if props == nil {
		props = []ucProp{}
	}
	return props, nil
}

// prop reads a property of metadata.
func (p *ucParser) prop() (ucProp, error) {
	k := p.next()
	if _, err := p.expect("a colon", ucColon); err != nil {
		return ucProp{}, err
	}
	v, err := p.expect("a value", ucIdent, ucString, ucTrue, ucFalse)
	if err != nil {
		return ucProp{}, err
	}
	prop := ucProp{key: tokenLabel(k).text, off: k.off}
	switch v.kind {
	case ucTrue:
		prop.value = true
	case ucFalse:
		prop.value = false
	default:
		prop.value = tokenLabel(v).text
	}
	return prop, nil
}

// boundary reads a system boundary and its body, up to end.
func (p *ucParser) boundary() error {
	p.next()
	e, err := p.name(true)
	if err != nil {
		return err
	}
	b := ucBoundaryDraft{id: e.id, label: e.label.text, markdown: e.label.markdown, off: e.off, generated: e.generated}
	var meta []ucProp
	if p.peek(1).kind == ucMetaStart {
		if meta, err = p.metadata(); err != nil {
			return err
		}
	}
	if p.peek(1).kind == ucClassSep {
		if b.classes, err = p.classSuffix(); err != nil {
			return err
		}
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	p.boundaries = append(p.boundaries, b)
	p.parent = &b
	defer func() { p.parent = nil }()
	for {
		switch p.peek(1).kind {
		case ucNewline:
			p.next()
			continue
		case ucComment:
			p.next()
			if err := p.lineEnd(); err != nil {
				return err
			}
			continue
		case ucActor:
			p.next()
			for {
				e, err := p.actorItem()
				if err != nil {
					return err
				}
				p.draft(e, ucActorKind)
				if p.peek(1).kind != ucComma {
					break
				}
				p.next()
			}
			if err := p.lineEnd(); err != nil {
				return err
			}
			continue
		case ucIdent, ucString, ucMarkdown:
			e, err := p.entity()
			if err != nil {
				return err
			}
			if err := p.lineEnd(); err != nil {
				return err
			}
			e.declared = true
			if e.shape == "" {
				e.shape = ucEllipse
			}
			p.draft(e, ucUseCaseKind)
			continue
		}
		break
	}
	if _, err := p.expect("end, or an actor or a use case of the system boundary", ucEndKw); err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	if meta != nil {
		p.metas = append(p.metas, ucMetaDraft{target: b.id, off: b.off, props: meta})
	}
	return nil
}

// metaStatement reads a name and its metadata, as A@{ type: hollow }.
func (p *ucParser) metaStatement() error {
	t := p.next()
	id := t.text
	if t.kind != ucIdent {
		id = generatedID(tokenLabel(t).text)
	}
	props, err := p.metadata()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	p.metas = append(p.metas, ucMetaDraft{target: id, off: textOff(t), props: props})
	return nil
}

// note reads note for A "text".
func (p *ucParser) note() error {
	p.next()
	if _, err := p.expect("for", ucFor); err != nil {
		return err
	}
	t, err := p.expect("the name of an actor or a use case", ucIdent)
	if err != nil {
		return err
	}
	l, err := p.nodeLabel()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	p.notes = append(p.notes, ucNoteDraft{target: t.text, off: t.off, label: l})
	return nil
}

// json reads json name@{ ... }, with its classes.
func (p *ucParser) json() error {
	start := p.next()
	obj, err := p.expect("an object of JSON with its end brace", ucJSONObject)
	if err != nil {
		return err
	}
	rest := strings.TrimLeft(strings.TrimPrefix(start.text, "json"), " \t")
	id := rest[:wordLen(rest)]
	d := ucJSONDraft{id: id, off: start.off + len(start.text) - len(rest)}
	if p.peek(1).kind == ucClassSep {
		if d.classes, err = p.classSuffix(); err != nil {
			return err
		}
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	rows, err := jsonRows(obj.text)
	if err != nil && p.jsonErr == nil {
		p.jsonErr = fmt.Errorf("line %d: the JSON of %s does not read: %w: %w", p.at(obj.off), id, err, ErrSyntax)
	}
	d.rows = rows
	p.jsons = append(p.jsons, d)
	return nil
}

// ids reads names with a comma between each two.
func (p *ucParser) ids() ([]ucToken, error) {
	var out []ucToken
	for {
		t, err := p.expect("a name", ucIdent)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		if p.peek(1).kind != ucComma {
			return out, nil
		}
		p.next()
	}
}

// classDef reads classDef a,b fill:#f9f,stroke:#333.
func (p *ucParser) classDef() error {
	p.next()
	ids, err := p.ids()
	if err != nil {
		return err
	}
	styles, err := p.styleList()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	d := ucClassDefDraft{styles: styles}
	for _, t := range ids {
		d.ids = append(d.ids, t.text)
	}
	p.classDefs = append(p.classDefs, d)
	return nil
}

// class reads class a,b c,d: the classes c and d on a and b.
func (p *ucParser) class() error {
	p.next()
	targets, err := p.ids()
	if err != nil {
		return err
	}
	classes, err := p.ids()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	d := ucClassDraft{}
	for _, t := range targets {
		d.targets = append(d.targets, ucTarget{id: t.text, off: t.off})
	}
	for _, t := range classes {
		d.classes = append(d.classes, t.text)
	}
	p.classes = append(p.classes, d)
	return nil
}

// style reads style a fill:#f9f.
func (p *ucParser) style() error {
	p.next()
	t, err := p.expect("a name", ucIdent)
	if err != nil {
		return err
	}
	styles, err := p.styleList()
	if err != nil {
		return err
	}
	if err := p.lineEnd(); err != nil {
		return err
	}
	p.styles = append(p.styles, ucStyleDraft{target: ucTarget{id: t.text, off: t.off}, styles: styles})
	return nil
}

// styleList reads the properties of a style, with a comma between each
// two.
func (p *ucParser) styleList() ([]string, error) {
	var out []string
	for {
		v, err := p.styleValue()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		if p.peek(1).kind != ucComma {
			return out, nil
		}
		p.next()
	}
}

// styleValue reads a property of a style: a name, a colon and a value. \,
// is a comma in a value.
func (p *ucParser) styleValue() (string, error) {
	t := p.peek(1)
	switch {
	case t.isWord(), t.kind == ucCSSIdent:
		p.next()
	case t.kind == ucMarkerless:
		p.next()
		if u := p.peek(1); !u.isWord() {
			return "", p.fail(u, "the name of a property")
		}
		p.next()
	default:
		return "", p.fail(t, "the name of a property")
	}
	if _, err := p.expect("a colon", ucColon); err != nil {
		return "", err
	}
	end := 0
	for styleComponent(p.peek(1)) {
		u := p.next()
		end = u.off + len(u.text)
	}
	if end == 0 {
		return "", p.fail(p.peek(1), "a value")
	}
	return strings.ReplaceAll(p.src[t.off:end], `\,`, ","), nil
}

// styleComponent reports whether a token can be a part of the value of a
// property of a style.
func styleComponent(t ucToken) bool {
	switch t.kind {
	case ucString, ucNumber, ucHashColor, ucCSSIdent, ucEscComma, ucDash, ucDot, ucPercent, ucCSSPunct, ucColon,
		ucLParen, ucRParen, ucLBracket, ucRBracket, ucLBrace, ucRBrace, ucAt, ucMarkerless:
		return true
	}
	return t.isWord()
}
