package mermaid

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The keywords of the four railroad diagrams of Mermaid 12.3. Each reads
// another notation of a grammar into the same model.
const (
	rrIR   = "railroad-beta"
	rrEBNF = "railroad-ebnf-beta"
	rrABNF = "railroad-abnf-beta"
	rrPEG  = "railroad-peg-beta"
)

// rrKind is the kind of a node of a railroad diagram.
type rrKind int

const (
	rrTerminal rrKind = iota
	rrNonterminal
	rrSpecial
	rrSequence
	rrChoice
	rrOptional
	rrRepeat
)

// rrNode is a node of the definition of a rule: a box with a text, or a
// part that holds other nodes.
type rrNode struct {
	kind rrKind
	// text is the text of a terminal, a nonterminal or a special.
	text string
	// items are the nodes of a sequence or a choice, or the one node of an
	// optional or a repeat.
	items []*rrNode
	// min is the least count of a repeat. A repeat with a min of 0 draws
	// a bypass over its loop.
	min int

	// w and h are the size of the node, and up is the height of its track
	// over its top. The layout measures them.
	w, h, up float64
}

// rrRule is a rule of a grammar: its name and its definition.
type rrRule struct {
	name string
	def  *rrNode
}

// railroad is a railroad diagram as the compiler reads it.
type railroad struct {
	c *chart
	// title is the text of the statement title, which wins over the
	// title of the front matter.
	title    string
	hasTitle bool
	rules    []rrRule
	// accTitle and accDescr are the last of each in the source, with
	// the place of the token, so that they join in the order of the
	// source.
	accTitle, accDescr     string
	accTitleAt, accDescrAt int

	toks []rrToken
	pos  int
	// sizes are the settings of the section railroad, in the pixels of
	// Mermaid.
	sizes rrSizes
}

// rrSizes are the sizes of the layout of Mermaid 12.3 that the section
// railroad of the config sets, in the pixels of Mermaid.
type rrSizes struct {
	padding, vsep, hsep, arc, marker float64
}

// rrSettings are the settings of the section railroad that zuzu reads:
// the sizes, by the key of the setting.
var rrSettings = []string{"padding", "verticalSeparation", "horizontalSeparation", "arcRadius", "markerRadius"}

// railroadChart compiles a railroad diagram of any of the four notations:
// each rule a track from a start marker to an end marker, with its
// terminals and nonterminals as boxes, and branches and loops for its
// choices, optional parts and repeats. The compiler lays the diagram out
// in Go and writes a Vega spec (D31).
func railroadChart(c *chart) (map[string]any, bool, error) {
	r := &railroad{c: c, sizes: rrSizes{padding: 10, vsep: 8, hsep: 10, arc: 10, marker: 5}}
	if err := r.settings(); err != nil {
		return nil, false, err
	}
	if err := r.lex(); err != nil {
		return nil, false, err
	}
	if err := r.parse(); err != nil {
		return nil, false, err
	}
	switch {
	case r.accTitle != "" && r.accDescr != "" && r.accDescrAt < r.accTitleAt:
		c.acc = append(c.acc, r.accDescr, r.accTitle)
	default:
		c.acc = append(c.acc, r.accTitle, r.accDescr)
	}
	title := c.d.Title
	if r.hasTitle && r.title != "" {
		title = r.title
	}
	if len(r.rules) == 0 {
		// Mermaid draws an empty image for a grammar with no rule.
		return emptyChart(title, ""), false, nil
	}
	spec := newRailroadLayout(r).spec()
	if title != "" {
		spec["title"] = map[string]any{"text": title, "anchor": "start"}
	}
	return spec, false, nil
}

// settings reads the section railroad of the config. Mermaid reads it in
// each of the four notations. zuzu reads the sizes. compactMode and
// showMarkers change nothing in Mermaid 12.3, so zuzu accepts them with no
// issue (D32). Each other setting, such as a color, is an issue.
func (r *railroad) settings() error {
	read := []string{"railroad.compactMode", "railroad.showMarkers"}
	for _, k := range rrSettings {
		read = append(read, "railroad."+k)
	}
	if err := r.c.unreadSettings(read...); err != nil {
		return err
	}
	sec, _ := r.c.d.Config["railroad"].(map[string]any)
	dst := map[string]*float64{
		"padding": &r.sizes.padding, "verticalSeparation": &r.sizes.vsep, "horizontalSeparation": &r.sizes.hsep,
		"arcRadius": &r.sizes.arc, "markerRadius": &r.sizes.marker,
	}
	for _, k := range rrSettings {
		v, ok := sec[k]
		if !ok {
			continue
		}
		n, ok := configNumber(sec, k)
		if !ok || n < 0 {
			if err := r.c.add("the setting railroad.%s %v, which is not a number of 0 or more", k, v); err != nil {
				return err
			}
			continue
		}
		*dst[k] = n
	}
	return nil
}

// rrToken is a token of the source of a railroad diagram.
type rrToken struct {
	// kind is the name of the token, or the text of a keyword.
	kind string
	text string
	// at is the number of the token in the source, and line its line.
	at, line int
}

// rrEOF is the kind of the token at the end of the source.
const rrEOF = "end of the source"

// rrMatch matches a token at the start of a text, and returns its length,
// or 0 when it does not match.
type rrMatch func(s string) int

// rrLexeme is a kind of token of a notation.
type rrLexeme struct {
	kind  string
	match rrMatch
	// hidden is true for a token that the parse skips, such as a comment.
	hidden bool
	// longer are the kinds of the tokens that win over a keyword when they
	// match more of the text, so that terminals is a name and not the
	// keyword terminal.
	longer []string
}

// The patterns of the tokens of the railroad diagrams of Mermaid 12.3. Each
// matches at the start of a text.
var (
	rrIdentRE   = regexp.MustCompile(`^[A-Z_a-z][\w-]*`)
	rrStringRE  = regexp.MustCompile(`^(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')`)
	rrSpaceRE   = regexp.MustCompile(`^[\t \r\n]+`)
	rrBlockRE   = regexp.MustCompile(`^/\*[\s\S]*?\*/`)
	rrISORE     = regexp.MustCompile(`^\(\*[\s\S]*?\*\)`)
	abnfNameRE  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*`)
	abnfStrRE   = regexp.MustCompile(`^"[^"]*"`)
	abnfNumRE   = regexp.MustCompile(`^%[xXdDbB][0-9A-Fa-f]+(?:-[0-9A-Fa-f]+|\.[0-9A-Fa-f]+)*`)
	abnfRepRE   = regexp.MustCompile(`^[0-9]*\*[0-9]*`)
	abnfExactRE = regexp.MustCompile(`^[0-9]+`)
	abnfNoteRE  = regexp.MustCompile(`^;[^\n\r]*`)
	pegNoteRE   = regexp.MustCompile(`^#[^\n\r]*`)
)

// rrRE returns a matcher of a pattern.
func rrRE(re *regexp.Regexp) rrMatch {
	return func(s string) int { return len(re.FindString(s)) }
}

// rrLine returns the length of the rest of the line of s, up to %% when the
// line holds it, as the pattern [^\n\r]*?(?=%%)|[^\n\r]* of Mermaid
// matches.
func rrLine(s string) int {
	end := strings.IndexAny(s, "\n\r")
	if end < 0 {
		end = len(s)
	}
	if c := strings.Index(s[:end], "%%"); c >= 0 {
		return c
	}
	return end
}

// rrTitle matches a title: the word title, then the rest of the line after
// a space or a tab, or nothing.
func rrTitle(s string) int {
	if !strings.HasPrefix(s, "title") {
		return 0
	}
	if rest := s[5:]; rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
		return 5 + rrLine(rest)
	}
	return 5
}

// rrAccTitle matches accTitle, a colon and the rest of the line.
func rrAccTitle(s string) int {
	if !strings.HasPrefix(s, "accTitle") {
		return 0
	}
	n := 8 + len(s[8:]) - len(strings.TrimLeft(s[8:], " \t"))
	if n >= len(s) || s[n] != ':' {
		return 0
	}
	return n + 1 + rrLine(s[n+1:])
}

// rrAccDescr matches accDescr with a colon and the rest of the line, or
// with a block in braces.
func rrAccDescr(s string) int {
	if !strings.HasPrefix(s, "accDescr") {
		return 0
	}
	n := 8 + len(s[8:]) - len(strings.TrimLeft(s[8:], " \t"))
	if n < len(s) && s[n] == ':' {
		return n + 1 + rrLine(s[n+1:])
	}
	n = 8 + len(s[8:]) - len(strings.TrimLeftFunc(s[8:], unicode.IsSpace))
	if n < len(s) && s[n] == '{' {
		if end := strings.IndexByte(s[n:], '}'); end >= 0 {
			return n + end + 1
		}
	}
	return 0
}

// rrSpecialSeq matches a special sequence of EBNF: a text between two
// question marks, with no semicolon, that holds more than spaces.
func rrSpecialSeq(s string) int {
	if !strings.HasPrefix(s, "?") {
		return 0
	}
	end := strings.IndexAny(s[1:], "?;")
	if end < 0 || s[1+end] != '?' || strings.TrimSpace(s[1:1+end]) == "" {
		return 0
	}
	return end + 2
}

// rrKeyword matches a keyword. The keyword of the kind must have a space,
// the end of the source or %% after it, as Mermaid asks.
func rrKeyword(word string, head bool) rrMatch {
	return func(s string) int {
		if !strings.HasPrefix(s, word) {
			return 0
		}
		rest := s[len(word):]
		if r, _ := utf8.DecodeRuneInString(rest); head && rest != "" && !strings.HasPrefix(rest, "%%") && !unicode.IsSpace(r) {
			return 0
		}
		return len(word)
	}
}

// rrLexemes returns the tokens of a notation in the order in which the
// lexer of Mermaid 12.3 tries them: spaces, then the keywords from the
// longest, then the other tokens in the order of the grammar. A keyword
// that a longer token matches gives way to it, so that oneOrMoreX is a
// name.
func rrLexemes(kind string) []rrLexeme {
	ws := rrLexeme{kind: "space", match: rrRE(rrSpaceRE), hidden: true}
	head := rrLexeme{kind: kind, match: rrKeyword(kind, true)}
	acc := []rrLexeme{{kind: "title", match: rrTitle}, {kind: "accTitle", match: rrAccTitle}, {kind: "accDescr", match: rrAccDescr}}
	keywords := func(longer map[string][]string, words ...string) []rrLexeme {
		out := []rrLexeme{head}
		out[0].longer = longer[kind]
		for _, w := range words {
			out = append(out, rrLexeme{kind: w, match: rrKeyword(w, false), longer: longer[w]})
		}
		return out
	}
	var out []rrLexeme
	switch kind {
	case rrIR:
		id := []string{"id"}
		out = keywords(map[string][]string{kind: id, "nonterminal": id, "zeroOrMore": id, "oneOrMore": id, "sequence": id,
			"optional": id, "terminal": id, "special": id, "choice": id},
			"nonterminal", "zeroOrMore", "oneOrMore", "sequence", "optional", "terminal", "special", "choice", "=", ";", "(", ",", ")")
		out = append(append(out, acc...),
			rrLexeme{kind: "id", match: rrRE(rrIdentRE)}, rrLexeme{kind: "string", match: rrRE(rrStringRE)},
			rrLexeme{kind: "block", match: rrRE(rrBlockRE), hidden: true})
	case rrEBNF:
		out = keywords(map[string][]string{kind: {"id"}, "?": {"special"}, "(": {"iso"}},
			"::=", "=", ";", "|", ",", "(", ")", "[", "]", "{", "}", "?", "*", "+", "-")
		out = append(append(out, acc...),
			rrLexeme{kind: "id", match: rrRE(rrIdentRE)}, rrLexeme{kind: "string", match: rrRE(rrStringRE)},
			rrLexeme{kind: "special", match: rrSpecialSeq},
			rrLexeme{kind: "block", match: rrRE(rrBlockRE), hidden: true}, rrLexeme{kind: "iso", match: rrRE(rrISORE), hidden: true})
	case rrABNF:
		out = keywords(map[string][]string{kind: {"id"}, ";": {"comment"}}, "=", ";", "/", "(", ")", "[", "]")
		out = append(append(out, acc...),
			rrLexeme{kind: "id", match: rrRE(abnfNameRE)}, rrLexeme{kind: "string", match: rrRE(abnfStrRE)},
			rrLexeme{kind: "numval", match: rrRE(abnfNumRE)}, rrLexeme{kind: "repeat", match: rrRE(abnfRepRE)},
			rrLexeme{kind: "exact", match: rrRE(abnfExactRE)}, rrLexeme{kind: "comment", match: rrRE(abnfNoteRE), hidden: true})
	case rrPEG:
		out = keywords(map[string][]string{kind: {"id"}}, "<-", ";", "/", "&", "!", "?", "*", "+", "(", ")", ".")
		out = append(append(out, acc...),
			rrLexeme{kind: "id", match: rrRE(rrIdentRE)}, rrLexeme{kind: "string", match: rrRE(rrStringRE)},
			rrLexeme{kind: "comment", match: rrRE(pegNoteRE), hidden: true})
	}
	return append([]rrLexeme{ws}, out...)
}

// lex splits the source into tokens, as the lexer of Mermaid 12.3 does: at
// each place, the first kind of token that matches wins. A text that no
// token matches is an error, as in Mermaid.
func (r *railroad) lex() error {
	d := r.c.d
	// The source of the body, with each line at its own number, so that
	// an error names the line of the source.
	var b strings.Builder
	b.WriteString(d.Kind)
	if d.Header != "" {
		b.WriteString(" " + d.Header)
	}
	line := d.N
	for _, l := range d.Lines {
		b.WriteString(strings.Repeat("\n", max(l.N-line, 1)))
		line = max(l.N, line+1)
		b.WriteString(l.Text)
	}
	lexemes := rrLexemes(d.Kind)
	byKind := map[string]rrLexeme{}
	for _, x := range lexemes {
		byKind[x.kind] = x
	}
	s, line := b.String(), d.N
	for s != "" {
		var tok rrLexeme
		n := 0
		for _, x := range lexemes {
			if n = x.match(s); n > 0 {
				tok = x
				break
			}
		}
		for _, k := range tok.longer {
			if m := byKind[k].match(s); m > n {
				tok, n = byKind[k], m
				break
			}
		}
		if n == 0 {
			r.c.line = line
			return r.c.errorf("the text %q", rrClip(s))
		}
		if !tok.hidden {
			r.toks = append(r.toks, rrToken{kind: tok.kind, text: s[:n], at: len(r.toks), line: line})
		}
		line += strings.Count(s[:n], "\n")
		s = s[n:]
	}
	r.toks = append(r.toks, rrToken{kind: rrEOF, at: len(r.toks), line: line})
	return nil
}

// rrClip returns the start of a text for an error: up to the end of its
// line.
func rrClip(s string) string {
	if end := strings.IndexAny(s, "\n\r"); end >= 0 {
		s = s[:end]
	}
	if len(s) > 20 {
		s = s[:20] + "..."
	}
	return s
}

// peek returns the next token.
func (r *railroad) peek() rrToken {
	return r.toks[r.pos]
}

// next returns the next token and moves past it.
func (r *railroad) next() rrToken {
	t := r.toks[r.pos]
	if t.kind != rrEOF {
		r.pos++
	}
	return t
}

// back moves back before a token that next returned.
func (r *railroad) back(t rrToken) {
	r.pos = t.at
}

// is reports whether the next token is of one of the kinds.
func (r *railroad) is(kinds ...string) bool {
	return slices.Contains(kinds, r.peek().kind)
}

// expect reads a token of a kind, or returns an error of the syntax.
func (r *railroad) expect(kind string) (rrToken, error) {
	if !r.is(kind) {
		return rrToken{}, r.unexpected(kind)
	}
	return r.next(), nil
}

// unexpected returns an error for the next token, where the parse expects
// another.
func (r *railroad) unexpected(want string) error {
	t := r.peek()
	r.c.line = t.line
	if t.kind == rrEOF {
		return r.c.errorf("the end of the source, where %s is expected", want)
	}
	return r.c.errorf("the text %q, where %s is expected", rrClip(t.text), want)
}

// parse reads the tokens: the keyword, then the title, accTitle and
// accDescr, then the rules, each in its notation.
func (r *railroad) parse() error {
	kind := r.c.d.Kind
	if _, err := r.expect(kind); err != nil {
		return err
	}
	for r.is("title", "accTitle", "accDescr") {
		r.header(r.next())
	}
	define := map[string][]string{rrIR: {"="}, rrEBNF: {"=", "::="}, rrABNF: {"="}, rrPEG: {"<-"}}[kind]
	for !r.is(rrEOF) {
		name, err := r.expect("id")
		if err != nil {
			return err
		}
		if !r.is(define...) {
			return r.unexpected(strings.Join(define, " or "))
		}
		r.next()
		var def *rrNode
		switch kind {
		case rrIR:
			def, err = r.irExpr()
		case rrEBNF:
			def, err = r.ebnfChoice()
		case rrABNF:
			def, err = r.abnfAlternation()
		default:
			def, err = r.pegChoice()
		}
		if err != nil {
			return err
		}
		if _, err := r.expect(";"); err != nil {
			return err
		}
		r.rules = append(r.rules, rrRule{name: name.text, def: def})
	}
	return nil
}

// rrSpaces matches two or more spaces or tabs, which Mermaid reads as one
// space in a title, an accTitle and an accDescr.
var rrSpaces = regexp.MustCompile(`[\t ]{2,}`)

// header reads a title, an accTitle or an accDescr, as Mermaid 12.3 reads
// them. A later one wins over an earlier one of the same kind.
func (r *railroad) header(t rrToken) {
	one := func(s string) string { return rrSpaces.ReplaceAllString(strings.TrimSpace(s), " ") }
	switch t.kind {
	case "title":
		v := one(t.text[5:])
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			if r.c.d.Kind == rrABNF {
				v = v[1 : len(v)-1]
			} else {
				v = rrUnescape(v)
			}
		}
		r.title, r.hasTitle = decodeEntities(v), true
	case "accTitle":
		_, v, _ := strings.Cut(t.text, ":")
		r.accTitle, r.accTitleAt = decodeEntities(one(v)), t.at
	case "accDescr":
		v := strings.TrimLeft(t.text[8:], " \t")
		if rest, ok := strings.CutPrefix(v, ":"); ok {
			v = one(rest)
		} else {
			v = strings.TrimSpace(v)
			v = strings.TrimSuffix(strings.TrimPrefix(v, "{"), "}")
			var lines []string
			for l := range strings.SplitSeq(v, "\n") {
				if l = rrSpaces.ReplaceAllString(strings.TrimSpace(l), " "); l != "" {
					lines = append(lines, l)
				}
			}
			v = strings.Join(lines, "\n")
		}
		r.accDescr, r.accDescrAt = decodeEntities(v), t.at
	}
}

// rrUnescape returns the text of a string in quotes, with its escapes read:
// \n, \r and \t as their characters, and a backslash before any other
// character as that character.
func rrUnescape(s string) string {
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// rrLeaf returns a box of a kind with a text.
func rrLeaf(kind rrKind, text string) *rrNode {
	return &rrNode{kind: kind, text: text}
}

// rrGroup returns a sequence or a choice of nodes, or the node itself when
// there is one, as Mermaid does.
func rrGroup(kind rrKind, items []*rrNode) *rrNode {
	if len(items) == 1 {
		return items[0]
	}
	return &rrNode{kind: kind, items: items}
}

// rrWrap returns an optional or a repeat of a node.
func rrWrap(kind rrKind, n *rrNode, least int) *rrNode {
	return &rrNode{kind: kind, items: []*rrNode{n}, min: least}
}

// irExpr reads an expression of the notation of railroad-beta, such as
// sequence(terminal("a"), nonterminal("b")).
func (r *railroad) irExpr() (*rrNode, error) {
	t := r.next()
	switch t.kind {
	case "terminal", "nonterminal", "special":
		if _, err := r.expect("("); err != nil {
			return nil, err
		}
		s, err := r.expect("string")
		if err != nil {
			return nil, err
		}
		if _, err := r.expect(")"); err != nil {
			return nil, err
		}
		kind := map[string]rrKind{"terminal": rrTerminal, "nonterminal": rrNonterminal, "special": rrSpecial}[t.kind]
		return rrLeaf(kind, rrUnescape(s.text)), nil
	case "sequence", "choice", "optional", "oneOrMore", "zeroOrMore":
		if _, err := r.expect("("); err != nil {
			return nil, err
		}
		var items []*rrNode
		for {
			n, err := r.irExpr()
			if err != nil {
				return nil, err
			}
			items = append(items, n)
			if (t.kind == "sequence" || t.kind == "choice") && r.is(",") {
				r.next()
				continue
			}
			break
		}
		if _, err := r.expect(")"); err != nil {
			return nil, err
		}
		switch t.kind {
		case "sequence":
			return rrGroup(rrSequence, items), nil
		case "choice":
			return rrGroup(rrChoice, items), nil
		case "optional":
			return rrWrap(rrOptional, items[0], 0), nil
		case "oneOrMore":
			return rrWrap(rrRepeat, items[0], 1), nil
		}
		return rrWrap(rrRepeat, items[0], 0), nil
	}
	r.back(t)
	return nil, r.unexpected("an expression")
}

// ebnfChoice reads alternatives of EBNF, split by |.
func (r *railroad) ebnfChoice() (*rrNode, error) {
	var alts []*rrNode
	for {
		n, err := r.ebnfSequence()
		if err != nil {
			return nil, err
		}
		alts = append(alts, n)
		if !r.is("|") {
			return rrGroup(rrChoice, alts), nil
		}
		r.next()
	}
}

// ebnfStarts are the tokens that start a term of EBNF.
var ebnfStarts = []string{"string", "special", "id", "(", "[", "{"}

// ebnfSequence reads the terms of a sequence of EBNF, with or without
// commas between them.
func (r *railroad) ebnfSequence() (*rrNode, error) {
	var items []*rrNode
	for {
		n, err := r.ebnfTerm()
		if err != nil {
			return nil, err
		}
		items = append(items, n)
		if r.is(",") {
			r.next()
			continue
		}
		if !r.is(ebnfStarts...) {
			return rrGroup(rrSequence, items), nil
		}
	}
}

// ebnfTerm reads a primary of EBNF and its postfixes: ?, *, + and an
// exception after -.
func (r *railroad) ebnfTerm() (*rrNode, error) {
	n, err := r.ebnfPrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch r.peek().kind {
		case "?":
			n = rrWrap(rrOptional, n, 0)
		case "*":
			n = rrWrap(rrRepeat, n, 0)
		case "+":
			n = rrWrap(rrRepeat, n, 1)
		case "-":
			r.next()
			ex, err := r.ebnfPrimary()
			if err != nil {
				return nil, err
			}
			n = &rrNode{kind: rrSequence, items: []*rrNode{n, rrLeaf(rrTerminal, "-"), ex}}
			continue
		default:
			return n, nil
		}
		r.next()
	}
}

// ebnfPrimary reads a string, a special sequence, a name, or a group in
// parentheses, brackets or braces.
func (r *railroad) ebnfPrimary() (*rrNode, error) {
	t := r.next()
	switch t.kind {
	case "string":
		return rrLeaf(rrTerminal, rrUnescape(t.text)), nil
	case "special":
		return rrLeaf(rrSpecial, strings.TrimSpace(t.text[1:len(t.text)-1])), nil
	case "id":
		return rrLeaf(rrNonterminal, t.text), nil
	case "(", "[", "{":
		n, err := r.ebnfChoice()
		if err != nil {
			return nil, err
		}
		if _, err := r.expect(map[string]string{"(": ")", "[": "]", "{": "}"}[t.kind]); err != nil {
			return nil, err
		}
		switch t.kind {
		case "[":
			return rrWrap(rrOptional, n, 0), nil
		case "{":
			return rrWrap(rrRepeat, n, 0), nil
		}
		return n, nil
	}
	r.back(t)
	return nil, r.unexpected("a term")
}

// abnfAlternation reads alternatives of ABNF, split by /.
func (r *railroad) abnfAlternation() (*rrNode, error) {
	var alts []*rrNode
	for {
		var items []*rrNode
		for {
			n, err := r.abnfElement()
			if err != nil {
				return nil, err
			}
			items = append(items, n)
			if !r.is("repeat", "exact", "string", "numval", "id", "(", "[") {
				break
			}
		}
		alts = append(alts, rrGroup(rrSequence, items))
		if !r.is("/") {
			return rrGroup(rrChoice, alts), nil
		}
		r.next()
	}
}

// abnfElement reads an element of ABNF: a repeat, such as 1* or 3, before
// a string, a value, a name, or a group in parentheses or brackets. A
// repeat of 0 to 1 is an optional, and each other repeat draws as a loop,
// with a bypass when its least count is 0.
func (r *railroad) abnfElement() (*rrNode, error) {
	repeat := ""
	if r.is("repeat", "exact") {
		repeat = r.next().text
	}
	t := r.next()
	var n *rrNode
	switch t.kind {
	case "string":
		n = rrLeaf(rrTerminal, t.text[1:len(t.text)-1])
	case "numval":
		n = rrLeaf(rrTerminal, t.text)
	case "id":
		n = rrLeaf(rrNonterminal, t.text)
	case "(", "[":
		inner, err := r.abnfAlternation()
		if err != nil {
			return nil, err
		}
		if _, err := r.expect(map[string]string{"(": ")", "[": "]"}[t.kind]); err != nil {
			return nil, err
		}
		n = inner
		if t.kind == "[" {
			n = rrWrap(rrOptional, inner, 0)
		}
	default:
		r.back(t)
		return nil, r.unexpected("an element")
	}
	if repeat == "" {
		return n, nil
	}
	least, most := abnfRepeat(repeat)
	if least == 0 && most == 1 {
		return rrWrap(rrOptional, n, 0), nil
	}
	return rrWrap(rrRepeat, n, least), nil
}

// abnfRepeat returns the least and the most counts of a repeat of ABNF,
// such as 1*3, *, or 2. A most of -1 has no limit.
func abnfRepeat(s string) (int, int) {
	lo, hi, star := strings.Cut(s, "*")
	least, _ := strconv.Atoi(lo)
	if !star {
		return least, least
	}
	most := -1
	if hi != "" {
		most, _ = strconv.Atoi(hi)
	}
	return least, most
}

// pegChoice reads the ordered alternatives of PEG, split by /.
func (r *railroad) pegChoice() (*rrNode, error) {
	var alts []*rrNode
	for {
		var items []*rrNode
		for {
			n, err := r.pegPrefix()
			if err != nil {
				return nil, err
			}
			items = append(items, n)
			if !r.is("&", "!", "string", "id", "(", ".") {
				break
			}
		}
		alts = append(alts, rrGroup(rrSequence, items))
		if !r.is("/") {
			return rrGroup(rrChoice, alts), nil
		}
		r.next()
	}
}

// pegPrefix reads a primary of PEG with its suffix, ?, * or +, and its
// prefix, & or !. A prefix is a lookahead, which Mermaid draws as a
// special box with the prefix and the name or the string, such as
// !Keyword, or (...) for any other part.
func (r *railroad) pegPrefix() (*rrNode, error) {
	op := ""
	if r.is("&", "!") {
		op = r.next().kind
	}
	t := r.next()
	var n *rrNode
	switch t.kind {
	case "string":
		n = rrLeaf(rrTerminal, rrUnescape(t.text))
	case "id":
		n = rrLeaf(rrNonterminal, t.text)
	case ".":
		n = rrLeaf(rrSpecial, ".")
	case "(":
		inner, err := r.pegChoice()
		if err != nil {
			return nil, err
		}
		if _, err := r.expect(")"); err != nil {
			return nil, err
		}
		n = inner
	default:
		r.back(t)
		return nil, r.unexpected("an expression")
	}
	switch r.peek().kind {
	case "?":
		r.next()
		n = rrWrap(rrOptional, n, 0)
	case "*":
		r.next()
		n = rrWrap(rrRepeat, n, 0)
	case "+":
		r.next()
		n = rrWrap(rrRepeat, n, 1)
	}
	if op == "" {
		return n, nil
	}
	label := "(...)"
	switch n.kind {
	case rrTerminal:
		label = `"` + n.text + `"`
	case rrNonterminal, rrSpecial:
		label = n.text
	}
	return rrLeaf(rrSpecial, op+label), nil
}
