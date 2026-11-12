package mermaid

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// emFrame is a frame of an event model: a box on the time axis.
type emFrame struct {
	// name is the id of the frame, one to three digits, as the source
	// writes it, so that 1 and 01 are two frames.
	name string
	// kind is ui, processor, readmodel, command or event.
	kind string
	// ident is the name of the entity, which can hold a namespace, as in
	// Cart.ItemAdded.
	ident string
	// reset is true for a resetframe, which takes no arrow from the frame
	// before it.
	reset bool
	// sources are the ids of the frames after ->>, in the order of the
	// source.
	sources []string
	// ref is the name of a data block after [[, or "" for none.
	ref string
	// inline is the data in the line of the frame, with its braces or its
	// quotes, and hasInline reports whether the frame has it.
	inline    string
	hasInline bool
	line      int
}

// eventModel is an event model as the compiler reads it.
type eventModel struct {
	c      *chart
	title  string
	frames []*emFrame
	// data holds the text of each data block by its name, braces and
	// all. The first block of a name wins, as in Mermaid.
	data map[string]string
}

// The kinds of a token of an event model.
const (
	emKeyword = iota
	emID
	emFID
	emInline
	emBlock
	emAccDescr
	emAccTitle
	emTitle
	emEOF
)

// emToken is a token of an event model.
type emToken struct {
	kind int
	text string
	// value is the text of an accTitle, an accDescr or a title.
	value string
	line  int
}

// emKeywords are the keywords of the grammar of an event model in Mermaid
// 12.3. A word that is a keyword is a keyword in every place, so that an
// entity cannot be named data or ui.
var emKeywords = []string{
	"eventmodeling", "tf", "timeframe", "rf", "resetframe", "rmo", "readmodel", "ui", "cmd", "command",
	"evt", "event", "pcr", "processor", "json", "jsobj", "figma", "salt", "uri", "md", "html", "text",
	"entity", "data", "note", "gwt", "given", "when", "then",
}

// The tokens of an event model that are patterns, as the lexer of Mermaid
// 12.3 reads them. An accDescr, an accTitle and a title start with a space
// or a tab. At the start of a line, the word is a name, which is an error.
var (
	emIDRE       = regexp.MustCompile(`^[_a-zA-Z][_a-zA-Z0-9]*`)
	emFIDRE      = regexp.MustCompile(`^[0-9]{1,3}`)
	emInlineRE   = regexp.MustCompile(`^(?:\{.*\}|".*"|'.*')`)
	emBlockRE    = regexp.MustCompile(`^\{[\t ]*\n`)
	emAccDescrRE = regexp.MustCompile(`^[\t ]*accDescr(?:[\t ]*:([^\n]*)|\s*\{([^}]*)\})`)
	emAccTitleRE = regexp.MustCompile(`^[\t ]*accTitle[\t ]*:([^\n]*)`)
	emTitleRE    = regexp.MustCompile(`^[\t ]*title(?:[\t ]([^\n]*))?`)
)

// The kinds of a frame by the word of the source.
var emKinds = map[string]string{
	"ui": "ui", "pcr": "processor", "processor": "processor", "rmo": "readmodel", "readmodel": "readmodel",
	"cmd": "command", "command": "command", "evt": "event", "event": "event",
}

// eventModelChart compiles an event model: its frames along a time axis
// from left to right, each in the swimlane of its kind, with an arrow from
// each frame to the next, into a Vega spec (D31). The compiler lays the
// model out in Go, with the rules of Mermaid 12.3, and the spec holds rows
// of rects, paths and text.
func eventModelChart(c *chart) (map[string]any, bool, error) {
	// Mermaid reads no setting of eventmodeling but padding, and its
	// config holds rowHeight, which nothing reads (D32).
	if err := c.unreadSettings("eventmodeling.rowHeight"); err != nil {
		return nil, false, err
	}
	m := &eventModel{c: c, data: map[string]string{}}
	toks, err := m.lex()
	if err != nil {
		return nil, false, err
	}
	if err := m.parse(toks); err != nil {
		return nil, false, err
	}
	title := c.d.Title
	if m.title != "" {
		title = m.title
	}
	if len(m.frames) == 0 {
		return emptyChart(title, ""), false, nil
	}
	spec := newEMLayout(m).spec()
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	return spec, false, nil
}

// source returns the text of the body of an event model, from the keyword
// on, with each line at its place, so that a token knows its line. A line
// keeps its indent, which the end of a data block needs.
func (m *eventModel) source() string {
	d := m.c.d
	var b strings.Builder
	b.WriteString(d.Kind)
	if d.Header != "" {
		b.WriteString(" " + d.Header)
	}
	n := d.N
	for _, l := range d.Lines {
		for ; n < l.N; n++ {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(" ", l.Indent) + l.Text)
	}
	return b.String()
}

// lex splits the body of an event model into tokens, as the lexer of
// Mermaid 12.3 does. A keyword or a name is tried first, then a number,
// data, accDescr, accTitle and title, and then a space or a comment.
func (m *eventModel) lex() ([]emToken, error) {
	c := m.c
	src := m.source()
	line := c.d.N
	var toks []emToken
	add := func(kind int, text, value string) {
		toks = append(toks, emToken{kind: kind, text: text, value: value, line: line})
	}
	for p := 0; p < len(src); {
		rest := src[p:]
		var n int
		c.line = line
		switch {
		case strings.HasPrefix(rest, "->>"):
			add(emKeyword, "->>", "")
			n = 3
		case strings.HasPrefix(rest, "[["), strings.HasPrefix(rest, "]]"):
			add(emKeyword, rest[:2], "")
			n = 2
		case rest[0] == '`' || rest[0] == '.':
			add(emKeyword, rest[:1], "")
			n = 1
		case emIDRE.MatchString(rest):
			w := emIDRE.FindString(rest)
			n = len(w)
			kind := emID
			// The keyword eventmodeling needs a space, the end or %% after
			// it. Each other keyword is a keyword when no longer name
			// holds it.
			if slices.Contains(emKeywords, w) && (w != "eventmodeling" || emSpaceAfter(rest[n:])) {
				kind = emKeyword
			}
			add(kind, w, "")
		case emFIDRE.MatchString(rest):
			w := emFIDRE.FindString(rest)
			n = len(w)
			add(emFID, w, "")
		case emInlineRE.MatchString(rest):
			w := emInlineRE.FindString(rest)
			n = len(w)
			add(emInline, w, "")
		case emBlockRE.MatchString(rest):
			n = emBlockEnd(rest)
			if n < 0 {
				return nil, c.errorf("a data block with no } at the start of a line")
			}
			add(emBlock, rest[:n], "")
		case emAccDescrRE.MatchString(rest):
			sm := emAccDescrRE.FindStringSubmatch(rest)
			n = len(sm[0])
			add(emAccDescr, sm[0], strings.TrimSpace(sm[1]+sm[2]))
		case emAccTitleRE.MatchString(rest):
			sm := emAccTitleRE.FindStringSubmatch(rest)
			n = len(sm[0])
			add(emAccTitle, sm[0], strings.TrimSpace(sm[1]))
		case emTitleRE.MatchString(rest):
			sm := emTitleRE.FindStringSubmatch(rest)
			n = len(sm[0])
			add(emTitle, sm[0], strings.TrimSpace(sm[1]))
		case unicode.IsSpace(rune(rest[0])):
			n = len(rest) - len(strings.TrimLeftFunc(rest, unicode.IsSpace))
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return nil, c.errorf("a comment /* with no */")
			}
			n = end + 4
		case strings.HasPrefix(rest, "//"):
			n = strings.IndexByte(rest, '\n')
			if n < 0 {
				n = len(rest)
			}
		default:
			r := []rune(rest)[0]
			return nil, c.errorf("the character %q", r)
		}
		line += strings.Count(rest[:n], "\n")
		p += n
	}
	c.line = line
	toks = append(toks, emToken{kind: emEOF, line: line})
	return toks, nil
}

// emSpaceAfter reports whether a keyword ends where a text starts: at its
// end, before a space, or before %%.
func emSpaceAfter(s string) bool {
	return s == "" || unicode.IsSpace(rune(s[0])) || strings.HasPrefix(s, "%%")
}

// emBlockEnd returns the length of the data block at the start of s, as
// Mermaid reads it, or -1. The block runs from a { at the end of its line
// to the first } at the start of a line with a space or the end after it,
// and takes the new line after the }.
func emBlockEnd(s string) int {
	start := len(emBlockRE.FindString(s))
	for j := start; j < len(s); j++ {
		if s[j] != '}' || s[j-1] != '\n' {
			continue
		}
		if j+1 == len(s) {
			return j + 1
		}
		if s[j+1] == '\n' {
			return j + 2
		}
		if unicode.IsSpace(rune(s[j+1])) {
			return j + 1
		}
	}
	return -1
}

// emParser reads the tokens of an event model.
type emParser struct {
	m    *eventModel
	toks []emToken
	i    int
}

// peek returns the next token.
func (p *emParser) peek() emToken {
	return p.toks[p.i]
}

// next returns the next token and moves past it.
func (p *emParser) next() emToken {
	t := p.toks[p.i]
	if t.kind != emEOF {
		p.i++
	}
	return t
}

// is reports whether the next token is a keyword of a set.
func (p *emParser) is(words ...string) bool {
	t := p.peek()
	return t.kind == emKeyword && slices.Contains(words, t.text)
}

// errorf returns an error of the syntax at the line of the next token.
func (p *emParser) errorf(format string, args ...any) error {
	p.m.c.line = p.peek().line
	return p.m.c.errorf(format, args...)
}

// unexpected returns the error of a token that the grammar does not
// expect.
func (p *emParser) unexpected(want string) error {
	t := p.peek()
	if t.kind == emEOF {
		return p.errorf("the end of the source where Mermaid expects %s", want)
	}
	return p.errorf("the text %q where Mermaid expects %s", strings.TrimSpace(t.text), want)
}

// expect reads a token of a kind, or a keyword.
func (p *emParser) expect(kind int, word, want string) (emToken, error) {
	t := p.peek()
	if t.kind != kind || word != "" && t.text != word {
		return t, p.unexpected(want)
	}
	return p.next(), nil
}

// parse reads the statements of an event model, as the grammar of
// Mermaid 12.3 reads them. A statement can run over lines, and a line can
// hold more statements.
func (m *eventModel) parse(toks []emToken) error {
	p := &emParser{m: m, toks: toks}
	if _, err := p.expect(emKeyword, "eventmodeling", "eventmodeling"); err != nil {
		return err
	}
	seen := map[string]int{}
	for p.peek().kind != emEOF {
		t := p.peek()
		var err error
		switch {
		case t.kind == emAccDescr || t.kind == emAccTitle:
			p.next()
			m.c.acc = append(m.c.acc, t.value)
		case t.kind == emTitle:
			p.next()
			m.title = t.value
		case p.is("tf", "timeframe", "rf", "resetframe"):
			var f *emFrame
			if f, err = p.frame(); err == nil {
				if _, ok := seen[f.name]; ok {
					m.c.line = f.line
					return m.c.errorf("the frame id %q twice", f.name)
				}
				seen[f.name] = f.line
				m.frames = append(m.frames, f)
			}
		case p.is("entity"):
			// An entity draws nothing in Mermaid 12.3 (D32).
			p.next()
			_, err = p.qualified()
		case p.is("data"):
			p.next()
			var name emToken
			if name, err = p.expect(emID, "", "the name of a data block"); err == nil {
				var block string
				if block, err = p.dataBlock(); err == nil {
					if _, ok := m.data[name.text]; !ok {
						m.data[name.text] = block
					}
				}
			}
		case p.is("note"):
			// A note draws nothing in Mermaid 12.3 (D32).
			p.next()
			if _, err = p.expect(emFID, "", "the id of a frame"); err == nil {
				_, err = p.dataBlock()
			}
		case p.is("gwt"):
			err = p.gwt()
		default:
			err = p.unexpected("a statement")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// frame reads a time frame or a reset frame: its id, its kind, its entity,
// the frames after ->>, a data block after [[ and data in the line.
func (p *emParser) frame() (*emFrame, error) {
	kw := p.next()
	f := &emFrame{reset: kw.text == "rf" || kw.text == "resetframe", line: kw.line}
	id, err := p.expect(emFID, "", "the id of a frame, of one to three digits")
	if err != nil {
		return nil, err
	}
	f.name = id.text
	kind := p.peek()
	if kind.kind != emKeyword || emKinds[kind.text] == "" {
		return nil, p.unexpected("ui, cmd, evt, rmo or pcr")
	}
	p.next()
	f.kind = emKinds[kind.text]
	if f.ident, err = p.qualified(); err != nil {
		return nil, err
	}
	for p.is("->>") {
		p.next()
		src, err := p.expect(emFID, "", "the id of a frame")
		if err != nil {
			return nil, err
		}
		f.sources = append(f.sources, src.text)
	}
	if p.is("[[") {
		p.next()
		ref, err := p.expect(emID, "", "the name of a data block")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(emKeyword, "]]", "]]"); err != nil {
			return nil, err
		}
		f.ref = ref.text
	}
	if p.is("`") || p.peek().kind == emInline {
		if err := p.dataType(); err != nil {
			return nil, err
		}
		t, err := p.expect(emInline, "", "data in braces or quotes")
		if err != nil {
			return nil, err
		}
		f.inline, f.hasInline = t.text, true
	}
	return f, nil
}

// qualified reads a name with a namespace before it or not, as
// Cart.ItemAdded.
func (p *emParser) qualified() (string, error) {
	t, err := p.expect(emID, "", "a name")
	if err != nil {
		return "", err
	}
	parts := []string{t.text}
	for p.is(".") {
		p.next()
		t, err := p.expect(emID, "", "a name after the dot")
		if err != nil {
			return "", err
		}
		parts = append(parts, t.text)
	}
	return strings.Join(parts, "."), nil
}

// dataType reads the type of data in backticks, as `json`, when there is
// one. The type draws nothing in Mermaid 12.3 (D32).
func (p *emParser) dataType() error {
	if !p.is("`") {
		return nil
	}
	p.next()
	if !p.is("json", "jsobj", "figma", "salt", "uri", "md", "html", "text") {
		return p.unexpected("a type of data")
	}
	p.next()
	_, err := p.expect(emKeyword, "`", "`")
	return err
}

// dataBlock reads a block of data over lines, with its type or not.
func (p *emParser) dataBlock() (string, error) {
	if err := p.dataType(); err != nil {
		return "", err
	}
	t, err := p.expect(emBlock, "", "a data block, a { at the end of a line")
	return t.text, err
}

// gwt reads a statement given, when and then, which draws nothing in
// Mermaid 12.3 (D32).
func (p *emParser) gwt() error {
	p.next()
	if _, err := p.expect(emFID, "", "the id of a frame"); err != nil {
		return err
	}
	if _, err := p.expect(emKeyword, "given", "given"); err != nil {
		return err
	}
	if err := p.gwtStatements(); err != nil {
		return err
	}
	if p.is("when") {
		p.next()
		if err := p.gwtStatements(); err != nil {
			return err
		}
	}
	if _, err := p.expect(emKeyword, "then", "then"); err != nil {
		return err
	}
	return p.gwtStatements()
}

// gwtStatements reads one or more kinds of an entity, each with its name.
func (p *emParser) gwtStatements() error {
	for n := 0; ; n++ {
		t := p.peek()
		if t.kind != emKeyword || emKinds[t.text] == "" {
			if n == 0 {
				return p.unexpected("ui, cmd, evt, rmo or pcr")
			}
			return nil
		}
		p.next()
		if _, err := p.expect(emID, "", "a name"); err != nil {
			return err
		}
	}
}
