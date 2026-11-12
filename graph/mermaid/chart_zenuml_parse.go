package mermaid

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The reader of ZenUML follows the grammar of @zenuml/core, the plugin
// that draws a ZenUML diagram in mmdc. zuzu reads the same tokens and the
// same statements, and does not port its parser.

// zKind is the kind of a token of ZenUML.
type zKind int

// The kinds of the tokens of ZenUML.
const (
	zEOF zKind = iota
	zTitle
	zTitleText
	zCol
	zPayload
	zSOpen
	zSClose
	zReturnArrow
	zArrow
	zColor
	zLBracket
	zRBracket
	zOp
	zMinus
	zNot
	zSemi
	zComma
	zAssign
	zOPar
	zCPar
	zOBrace
	zCBrace
	zLiteral
	zIf
	zElse
	zWhile
	zReturn
	zNew
	zPar
	zGroup
	zOpt
	zCritical
	zSectionKw
	zRef
	zAs
	zTry
	zCatch
	zFinally
	zIn
	zStarter
	zAnnotationRet
	zAnnotation
	zDot
	zID
	zNumber
	zDigitName
	zString
	zDivider
	zOther
	zLT
	zGT
)

// zKeywords are the keywords of ZenUML. They match in their own case
// only.
var zKeywords = map[string]zKind{
	"true": zLiteral, "false": zLiteral, "nil": zLiteral, "null": zLiteral,
	"if": zIf, "else": zElse, "while": zWhile, "for": zWhile, "foreach": zWhile, "forEach": zWhile,
	"loop": zWhile, "return": zReturn, "new": zNew, "par": zPar, "group": zGroup, "opt": zOpt,
	"critical": zCritical, "section": zSectionKw, "frame": zSectionKw, "ref": zRef, "as": zAs,
	"try": zTry, "catch": zCatch, "finally": zFinally, "in": zIn,
}

// zModifiers are the words that the lexer of ZenUML puts aside, as it
// does a space.
var zModifiers = map[string]bool{"const": true, "readonly": true, "static": true, "await": true}

// zOps are the tokens of punctuation, the longest first.
var zOps = []struct {
	text string
	kind zKind
}{
	{"-->", zReturnArrow}, {"<<", zSOpen}, {">>", zSClose}, {"->", zArrow},
	{"||", zOp}, {"&&", zOp}, {"==", zOp}, {"!=", zOp}, {">=", zOp}, {"<=", zOp},
	{">", zGT}, {"<", zLT}, {"+", zOp}, {"-", zMinus}, {"*", zOp}, {"/", zOp}, {"%", zOp},
	{"^", zOp}, {"!", zNot}, {";", zSemi}, {",", zComma}, {"=", zAssign}, {"(", zOPar},
	{")", zCPar}, {"{", zOBrace}, {"}", zCBrace}, {"[", zLBracket}, {"]", zRBracket},
}

// zTok is a token of ZenUML: its kind, its text, its place in the source
// in bytes and the number of its line.
type zTok struct {
	kind       zKind
	text       string
	start, end int
	line       int
}

// zLexer splits the source of a ZenUML diagram into tokens. comments
// holds the texts of the comments, //, before each token, which ZenUML
// draws over the statement that starts at the token.
type zLexer struct {
	src  string
	at   int
	toks []zTok
	// comments holds the comments before each token, by its index.
	comments map[int][]string
	pending  []string
	// lines holds the number of the line in the source of Mermaid of each
	// line of src.
	lines []int
	// starts holds the offset of each line of src.
	starts []int
}

// zLex splits the lines of a body into tokens.
func zLex(lines []Line) *zLexer {
	l := &zLexer{comments: map[int][]string{}}
	var b strings.Builder
	for _, ln := range lines {
		l.starts = append(l.starts, b.Len())
		l.lines = append(l.lines, ln.N)
		b.WriteString(ln.Text)
		b.WriteByte('\n')
	}
	l.src = b.String()
	l.run()
	return l
}

// lineOf returns the number of the line in the source of Mermaid of an
// offset of src.
func (l *zLexer) lineOf(at int) int {
	n := 0
	for i, s := range l.starts {
		if s > at {
			break
		}
		n = l.lines[i]
	}
	return n
}

// atLineStart reports whether an offset is at the start of a line.
func (l *zLexer) atLineStart(at int) bool {
	return at == 0 || l.src[at-1] == '\n'
}

// emit adds a token from an offset to the current offset.
func (l *zLexer) emit(kind zKind, start int) {
	if len(l.pending) > 0 {
		l.comments[len(l.toks)] = l.pending
		l.pending = nil
	}
	l.toks = append(l.toks, zTok{kind: kind, text: l.src[start:l.at], start: start, end: l.at, line: l.lineOf(start)})
}

// restOfLine moves to the end of the line and returns the offset where
// the text started.
func (l *zLexer) restOfLine() int {
	start := l.at
	if i := strings.IndexByte(l.src[l.at:], '\n'); i >= 0 {
		l.at += i
	} else {
		l.at = len(l.src)
	}
	return start
}

// run reads the tokens.
func (l *zLexer) run() {
	for l.at < len(l.src) {
		r, size := utf8.DecodeRuneInString(l.src[l.at:])
		start := l.at
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			l.at += size
		case strings.HasPrefix(l.src[l.at:], "//"):
			l.restOfLine()
			l.pending = append(l.pending, l.src[start+2:l.at])
		case l.atLineStart(l.at) && strings.HasPrefix(l.src[l.at:], "=="):
			// A divider takes a line that starts with ==.
			l.restOfLine()
			l.emit(zDivider, start)
		case r == ':':
			// The text after a colon runs to the end of the line.
			l.at++
			l.emit(zCol, start)
			if s := l.restOfLine(); l.at > s {
				l.emit(zPayload, s)
			}
		case r == '"':
			l.at++
			for l.at < len(l.src) && l.src[l.at] != '"' && l.src[l.at] != '\n' {
				l.at++
			}
			if l.at < len(l.src) && l.src[l.at] == '"' {
				l.at++
			}
			l.emit(zString, start)
		case r == '@':
			l.at++
			l.word()
			switch strings.ToLower(l.src[start+1 : l.at]) {
			case "starter":
				l.emit(zStarter, start)
			case "return", "reply":
				l.emit(zAnnotationRet, start)
			default:
				l.emit(zAnnotation, start)
			}
		case r == '#':
			l.at++
			for l.at < len(l.src) && isHex(l.src[l.at]) {
				l.at++
			}
			if l.at == start+1 {
				l.emit(zOther, start)
			} else {
				l.emit(zColor, start)
			}
		case r == '$' && l.at+1 < len(l.src) && isDigit(l.src[l.at+1]):
			l.at++
			l.number()
			l.emit(zNumber, start)
		case r >= '0' && r <= '9' || r == '.' && l.at+1 < len(l.src) && isDigit(l.src[l.at+1]):
			l.number()
			if l.at < len(l.src) {
				if n, _ := utf8.DecodeRuneInString(l.src[l.at:]); isWordStart(n) {
					l.word()
					l.emit(zDigitName, start)
					continue
				}
			}
			l.emit(zNumber, start)
		case r == '.':
			l.at++
			l.emit(zDot, start)
		case isWordStart(r):
			l.word()
			w := l.src[start:l.at]
			switch {
			case zModifiers[w]:
			case w == "title" && l.isTitle(start):
				l.emit(zTitle, start)
				if s := l.restOfLine(); l.at > s {
					l.emit(zTitleText, s)
				}
			default:
				kind, ok := zKeywords[w]
				if !ok {
					kind = zID
				}
				l.emit(kind, start)
			}
		default:
			l.op(start, size)
		}
	}
	l.emit(zEOF, l.at)
}

// op reads a token of punctuation, or one character that ZenUML does not
// read.
func (l *zLexer) op(start, size int) {
	for _, o := range zOps {
		if strings.HasPrefix(l.src[l.at:], o.text) {
			l.at += len(o.text)
			l.emit(o.kind, start)
			return
		}
	}
	l.at += size
	l.emit(zOther, start)
}

// isTitle reports whether the word title at an offset is the title of the
// diagram: only spaces and comments come before it, and no ., ( or =
// follows it.
func (l *zLexer) isTitle(start int) bool {
	for _, t := range l.toks {
		if t.start < start {
			return false
		}
	}
	rest := strings.TrimLeft(l.src[l.at:], " \t")
	return rest == "" || !strings.ContainsAny(rest[:1], ".(=")
}

// word moves past the letters, the digits and the underscores of a word.
func (l *zLexer) word() {
	for l.at < len(l.src) {
		r, size := utf8.DecodeRuneInString(l.src[l.at:])
		if !isWordStart(r) && !unicode.IsDigit(r) {
			return
		}
		l.at += size
	}
}

// number moves past the digits of a number and its fraction.
func (l *zLexer) number() {
	for l.at < len(l.src) && isDigit(l.src[l.at]) {
		l.at++
	}
	if l.at < len(l.src) && l.src[l.at] == '.' {
		l.at++
		for l.at < len(l.src) && isDigit(l.src[l.at]) {
			l.at++
		}
	}
}

// isWordStart reports whether a rune starts a word: a letter or an
// underscore.
func isWordStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

// isDigit reports whether a byte is a digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// zFormat formats the text of a part of the source as ZenUML shows it:
// each run of spaces becomes one space, no space stays around a comma, a
// semicolon, a dot or a parenthesis, and the quotes of a string go.
func zFormat(s string) string {
	s = zSpaceRE.ReplaceAllString(s, " ")
	s = zPunctRE.ReplaceAllString(s, "$1")
	s = zParenRE.ReplaceAllString(s, "$1$2")
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

var (
	zSpaceRE = regexp.MustCompile(`\s+`)
	zPunctRE = regexp.MustCompile(`\s*([,;.])\s*`)
	zParenRE = regexp.MustCompile(`\s*(\()\s*|\s*(\))`)
)

// zPart is a participant that the head of a diagram declares.
type zPart struct {
	name, label, typ, stereotype, emoji, color, group string
	line                                              int
}

// zSection is a section of a fragment: its label and its statements.
type zSection struct {
	label string
	stats []*zStat
}

// zStat is a statement of ZenUML.
type zStat struct {
	// kind is sync, async, creation, return, divider or fragment.
	kind string
	line int
	// comment holds the lines of the comment before the statement.
	comment []string
	// from and to are the names that the statement gives, or empty.
	from, to string
	// assignee is the name before = of a sync message or a creation.
	assignee string
	label    string
	// braces reports whether a message or a creation has a block, and
	// stats are the statements of its block.
	braces bool
	stats  []*zStat
	// frag is the kind of a fragment: alt, loop, opt, par, critical,
	// section, tcf or ref. sections are its sections, the first one with
	// the label of the fragment.
	frag     string
	sections []zSection
	// refs are the participants of a ref.
	refs []string
}

// zProg is a ZenUML diagram as zuzu reads it.
type zProg struct {
	title    string
	hasTitle bool
	parts    []zPart
	starter  string
	stats    []*zStat
}

// zParser reads the tokens of ZenUML into statements. A token that does
// not fit the grammar is an issue, and the parser goes on after it, as
// the parser of ZenUML does.
type zParser struct {
	c    *chart
	lx   *zLexer
	toks []zTok
	pos  int
	// group is the name of the group that the parser reads.
	group string
}

// peek returns the kind of the token at an offset from the current one.
func (p *zParser) peek(k int) zKind {
	if p.pos+k >= len(p.toks) {
		return zEOF
	}
	return p.toks[p.pos+k].kind
}

// next returns the current token and moves past it.
func (p *zParser) next() zTok {
	t := p.toks[p.pos]
	if t.kind != zEOF {
		p.pos++
	}
	return t
}

// accept moves past the current token when it is of a kind.
func (p *zParser) accept(kind zKind) bool {
	if p.peek(0) != kind {
		return false
	}
	p.next()
	return true
}

// text returns the formatted text of the source from the token at start to
// the token before the current one.
func (p *zParser) text(start int) string {
	if start >= p.pos {
		return ""
	}
	return zFormat(p.lx.src[p.toks[start].start:p.toks[p.pos-1].end])
}

// skip reports the current token as text that zuzu does not read, and moves
// past it.
func (p *zParser) skip() error {
	t := p.next()
	p.c.line = t.line
	if t.kind == zEOF {
		return p.c.add("the end of the source where the grammar of ZenUML wants more")
	}
	return p.c.add("the text %q, which does not fit the grammar of ZenUML", t.text)
}

// missing reports a token that the grammar of ZenUML wants at the current
// one and that the source does not have. The parser goes on without it.
func (p *zParser) missing(what string) error {
	p.c.line = p.toks[p.pos].line
	return p.c.add("no %s where the grammar of ZenUML wants one", what)
}

// isName reports whether a kind of token can be a name.
func isName(k zKind) bool {
	return k == zID || k == zDigitName || k == zString
}

// name reads a name and returns its text.
func (p *zParser) name() (string, bool) {
	if !isName(p.peek(0)) {
		return "", false
	}
	return zFormat(p.next().text), true
}

// prog reads a diagram.
func (p *zParser) prog() (*zProg, error) {
	prog := &zProg{}
	if p.peek(0) == zTitle {
		p.next()
		if p.peek(0) == zTitleText {
			prog.title = strings.TrimSpace(p.next().text)
		}
		prog.hasTitle = true
	}
	if err := p.head(prog); err != nil {
		return nil, err
	}
	stats, err := p.block(true)
	if err != nil {
		return nil, err
	}
	prog.stats = stats
	return prog, nil
}

// head reads the participants, the groups and the starter at the start of
// a diagram.
func (p *zParser) head(prog *zProg) error {
	for {
		switch {
		case p.peek(0) == zGroup:
			if err := p.groupDecl(prog); err != nil {
				return err
			}
		case p.peek(0) == zStarter:
			p.next()
			if p.accept(zOPar) {
				if n, ok := p.name(); ok {
					prog.starter = n
				}
				if !p.accept(zCPar) {
					return p.missing(")")
				}
			}
			// The starter ends the head.
			return nil
		default:
			part, ok := p.participant()
			if !ok {
				return nil
			}
			prog.parts = append(prog.parts, part)
		}
	}
}

// groupDecl reads a group of participants.
func (p *zParser) groupDecl(prog *zProg) error {
	p.next()
	name, _ := p.name()
	if !p.accept(zOBrace) {
		return nil
	}
	p.group = name
	defer func() { p.group = "" }()
	for {
		if p.accept(zCBrace) || p.peek(0) == zEOF {
			return nil
		}
		part, ok := p.participant()
		if !ok {
			if err := p.skip(); err != nil {
				return err
			}
			continue
		}
		prog.parts = append(prog.parts, part)
	}
}

// zStatFollow are the tokens after a name that make it the start of a
// statement, and not a participant.
var zStatFollow = map[zKind]bool{
	zDot: true, zArrow: true, zReturnArrow: true, zCol: true, zAssign: true,
	zMinus: true, zOPar: true, zComma: true, zSemi: true,
}

// participant reads a participant of the head, if the tokens are one and
// do not start a statement.
func (p *zParser) participant() (zPart, bool) {
	start := p.pos
	part := zPart{group: p.group, line: p.toks[p.pos].line}
	if p.peek(0) == zAnnotation {
		part.typ = p.next().text[1:]
	}
	if p.peek(0) == zSOpen || p.peek(0) == zLT {
		p.next()
		part.stereotype, _ = p.name()
		if p.peek(0) == zSClose || p.peek(0) == zGT {
			p.next()
		}
	}
	prefix := p.pos
	if p.peek(0) == zLBracket && isName(p.peek(1)) && p.peek(2) == zRBracket {
		p.next()
		part.emoji, _ = p.name()
		p.next()
	}
	name, ok := p.name()
	if !ok {
		p.pos = prefix
		if prefix == start || part.emoji != "" {
			p.pos = start
			return zPart{}, false
		}
		// A type or a stereotype alone is a participant with no name.
		return part, true
	}
	part.name = name
	if p.peek(0) == zNumber && !strings.ContainsAny(p.toks[p.pos].text, ".$") {
		// ZenUML reads the width of a participant, and draws nothing for it.
		p.next()
	}
	if p.accept(zAs) {
		part.label, _ = p.name()
	}
	if p.peek(0) == zColor {
		part.color = p.next().text
	}
	if zStatFollow[p.peek(0)] {
		// The name starts a statement. A type or a stereotype before it is
		// a participant alone.
		if prefix > start && part.emoji == "" && part.label == "" && part.color == "" {
			p.pos = prefix
			return zPart{typ: part.typ, stereotype: part.stereotype, group: part.group, line: part.line}, true
		}
		p.pos = start
		return zPart{}, false
	}
	return part, true
}
