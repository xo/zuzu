package mermaid

import (
	"fmt"
	"strings"
)

// ucKind is a kind of token of a use case diagram. The tokens are those of
// the lexer of Mermaid 12.3.0, so that zuzu reads a source as an error
// where Mermaid does.
type ucKind int

// The kinds of token. ucIdent and the keywords are words. A keyword
// matches only where an identifier of the same length does: actors is an
// identifier, and actor is the keyword.
const (
	ucEOF ucKind = iota
	ucNewline
	ucComment
	ucMarkdown         // "`...`", which can run over lines
	ucUnclosedMarkdown // "` with no end
	ucAccTitle         // accTitle: ... at the start of a line
	ucAccDescr         // accDescr: ... at the start of a line
	ucAccDescrBlock    // accDescr { ... }
	ucJSONStart        // json name@ before an open brace
	ucJSONObject       // the object after it
	ucUnclosedJSON     // an object with no end brace
	ucUsecase          // the keywords, from usecase-beta to false
	ucActor
	ucBoundaryKw
	ucEndKw
	ucDirection
	ucTD
	ucTB
	ucBT
	ucLR
	ucRL
	ucNote
	ucFor
	ucJSON
	ucClassDef
	ucClass
	ucStyle
	ucInclude
	ucExtend
	ucTrue
	ucFalse
	ucGeneralization // --|>
	ucDependency     // ..>
	ucStereoStart    // <<
	ucStereoText     // the text between << and >>
	ucStereoEnd      // >>
	ucUnclosedStereo // a text after << with no >>
	ucClassSep       // :::
	ucFwdSolid       // -->, --->
	ucBackSolid      // <--, <---
	ucFwdCircle      // --o
	ucBackCircle     // o--
	ucFwdCross       // --x
	ucBackCross      // x--
	ucMarkerless     // --, ---
	ucMetaStart      // @{
	ucAt             // @
	ucLBrace
	ucRBrace
	ucLBracket
	ucRBracket
	ucLParen
	ucRParen
	ucComma
	ucColon
	ucHashColor // #f9f
	ucString    // "..." or '...' on one line
	ucCSSIdent  // stroke-width
	ucIdent     // \w+
	ucNumber    // 1.5px, .5
	ucEscComma  // \,
	ucDash
	ucDot
	ucPercent
	ucCSSPunct   // ! # $ & * + / = ? ^ _ | ~
	ucLabelPunct // a semicolon, <, >, a backslash or a backtick
	ucSymbolTok  // a run of characters that are not printable ASCII
)

// ucKeywords are the keywords in the order of the lexer of Mermaid.
// include and extend match in any case, and the others in their own case.
var ucKeywords = []struct {
	word string
	kind ucKind
}{
	{"usecase-beta", ucUsecase}, {"actor", ucActor}, {"systemBoundary", ucBoundaryKw}, {"end", ucEndKw},
	{"direction", ucDirection}, {"TD", ucTD}, {"TB", ucTB}, {"BT", ucBT}, {"LR", ucLR}, {"RL", ucRL},
	{"note", ucNote}, {"for", ucFor}, {"json", ucJSON}, {"classDef", ucClassDef}, {"class", ucClass},
	{"style", ucStyle}, {"include", ucInclude}, {"extend", ucExtend}, {"true", ucTrue}, {"false", ucFalse},
}

// ucFixed are the tokens of fixed text that come after the keywords, in
// the order of the lexer of Mermaid. The arrows of solid lines of any
// length come between them.
var ucFixed = []struct {
	text string
	kind ucKind
}{
	{"--|>", ucGeneralization}, {"..>", ucDependency}, {"<<", ucStereoStart}, {":::", ucClassSep},
}

// ucToken is a token: its kind, its text and its offset in the body.
type ucToken struct {
	kind ucKind
	text string
	off  int
}

// isWord reports whether a token is an identifier or a keyword.
func (t ucToken) isWord() bool {
	return t.kind == ucIdent || t.kind >= ucUsecase && t.kind <= ucFalse
}

// isLabel reports whether a token can be a part of a label without
// quotes: a word, a number, @, a comma, a colon, a color, a name of CSS
// or a mark of punctuation.
func (t ucToken) isLabel() bool {
	switch t.kind {
	case ucNumber, ucAt, ucComma, ucColon, ucHashColor, ucCSSIdent, ucEscComma, ucDash, ucDot, ucPercent,
		ucCSSPunct, ucLabelPunct, ucSymbolTok:
		return true
	}
	return t.isWord()
}

// wordLen returns the length of the run of \w at the start of s.
func wordLen(s string) int {
	n := 0
	for n < len(s) && isWordByte(s[n]) {
		n++
	}
	return n
}

// ucLexer splits the body of a use case diagram into tokens.
type ucLexer struct {
	s    string
	i    int
	toks []ucToken
	// at returns the number of the line of the source at an offset.
	at func(off int) int
}

// lineStart reports whether only spaces and tabs come before an offset on
// its line.
func (l *ucLexer) lineStart(off int) bool {
	for j := off - 1; j >= 0; j-- {
		switch l.s[j] {
		case '\n', '\r':
			return true
		case ' ', '\t':
		default:
			return false
		}
	}
	return true
}

func (l *ucLexer) emit(kind ucKind, n int) {
	l.toks = append(l.toks, ucToken{kind: kind, text: l.s[l.i : l.i+n], off: l.i})
	l.i += n
}

// lex splits the body into tokens. A character that no token matches, such
// as a quote with no end, is an error, as in Mermaid.
func (l *ucLexer) lex() error {
	for l.i < len(l.s) {
		rest := l.s[l.i:]
		c := rest[0]
		if c == ' ' || c == '\t' {
			l.i++
			continue
		}
		kind, n := l.match(rest)
		if n == 0 {
			return fmt.Errorf("line %d: the character %q does not read: %w", l.at(l.i), rest[:1], ErrSyntax)
		}
		l.emit(kind, n)
		switch kind {
		case ucJSONStart:
			// The object follows at once, with its braces in balance.
			rest = l.s[l.i:]
			if n := jsonObjectLen(rest); n > 0 {
				l.emit(ucJSONObject, n)
			} else {
				l.emit(ucUnclosedJSON, len(rest))
			}
		case ucStereoStart:
			if err := l.stereotype(); err != nil {
				return err
			}
		}
	}
	l.toks = append(l.toks, ucToken{kind: ucEOF, off: len(l.s)})
	return nil
}

// stereotype reads the text of a stereotype after <<, and its >>.
func (l *ucLexer) stereotype() error {
	rest := l.s[l.i:]
	if strings.HasPrefix(rest, ">>") {
		l.emit(ucStereoEnd, 2)
		return nil
	}
	end := strings.Index(rest, ">>")
	if end > 0 && !strings.ContainsAny(rest[:end], "\n\r") && strings.TrimSpace(rest[:end]) != "" {
		l.emit(ucStereoText, end)
		l.emit(ucStereoEnd, 2)
		return nil
	}
	n := strings.IndexAny(rest, "\n\r")
	if n < 0 {
		n = len(rest)
	}
	if n == 0 {
		return fmt.Errorf("line %d: a stereotype with no text: %w", l.at(l.i), ErrSyntax)
	}
	l.emit(ucUnclosedStereo, n)
	return nil
}

// match returns the kind and the length of the token at the start of
// rest, in the order of the lexer of Mermaid, or a length of 0.
func (l *ucLexer) match(rest string) (ucKind, int) {
	c := rest[0]
	switch {
	case strings.HasPrefix(rest, "\"`"):
		if end := strings.Index(rest[2:], "`\""); end >= 0 {
			return ucMarkdown, end + 4
		}
		return ucUnclosedMarkdown, len(rest)
	case strings.HasPrefix(rest, "%%") && l.lineStart(l.i):
		return ucComment, lineLen(rest)
	case c == '\r' && strings.HasPrefix(rest, "\r\n"):
		return ucNewline, 2
	case c == '\n' || c == '\r':
		return ucNewline, 1
	}
	if c == 'a' && l.lineStart(l.i) {
		if kind, n := accLen(rest); n > 0 {
			return kind, n
		}
	}
	if n := jsonStartLen(rest); n > 0 {
		return ucJSONStart, n
	}
	word := wordLen(rest)
	for _, k := range ucKeywords {
		n := len(k.word)
		if len(rest) < n {
			continue
		}
		ok := rest[:n] == k.word
		if k.kind == ucInclude || k.kind == ucExtend {
			ok = strings.EqualFold(rest[:n], k.word)
		}
		if ok {
			if word > n {
				return ucIdent, word
			}
			return k.kind, n
		}
	}
	for _, f := range ucFixed {
		if strings.HasPrefix(rest, f.text) {
			return f.kind, len(f.text)
		}
	}
	dashes := func(s string) int {
		n := 0
		for n < len(s) && s[n] == '-' {
			n++
		}
		return n
	}
	if d := dashes(rest); d >= 2 && d < len(rest) && rest[d] == '>' {
		return ucFwdSolid, d + 1
	}
	if c == '<' {
		if d := dashes(rest[1:]); d >= 2 {
			return ucBackSolid, d + 1
		}
	}
	for _, f := range []struct {
		text string
		kind ucKind
	}{{"--o", ucFwdCircle}, {"o--", ucBackCircle}, {"--x", ucFwdCross}, {"x--", ucBackCross}} {
		if strings.HasPrefix(rest, f.text) {
			return f.kind, 3
		}
	}
	if d := dashes(rest); d >= 2 {
		return ucMarkerless, d
	}
	if strings.HasPrefix(rest, "@{") {
		return ucMetaStart, 2
	}
	switch c {
	case '@':
		return ucAt, 1
	case '{':
		return ucLBrace, 1
	case '}':
		return ucRBrace, 1
	case '[':
		return ucLBracket, 1
	case ']':
		return ucRBracket, 1
	case '(':
		return ucLParen, 1
	case ')':
		return ucRParen, 1
	case ',':
		return ucComma, 1
	case ':':
		return ucColon, 1
	case '#':
		n := 1
		for n < len(rest) && isHex(rest[n]) {
			n++
		}
		if n > 1 {
			return ucHashColor, n
		}
	case '"', '\'':
		if end := strings.IndexAny(rest[1:], string(c)+"\n\r"); end >= 0 && rest[1+end] == c {
			return ucString, end + 2
		}
	}
	if n := cssIdentLen(rest); n > 0 {
		return ucCSSIdent, n
	}
	if word > 0 {
		if n := numberLen(rest); n > word {
			return ucNumber, n
		}
		return ucIdent, word
	}
	if n := numberLen(rest); n > 0 {
		return ucNumber, n
	}
	switch {
	case strings.HasPrefix(rest, `\,`):
		return ucEscComma, 2
	case c == '-':
		return ucDash, 1
	case c == '.':
		return ucDot, 1
	case c == '%':
		return ucPercent, 1
	case strings.IndexByte("!#$&*+/=?^_|~", c) >= 0:
		return ucCSSPunct, 1
	case strings.IndexByte(";<>\\`", c) >= 0:
		return ucLabelPunct, 1
	}
	// A run of characters that are not printable ASCII, a space, a tab or
	// a new line.
	n := 0
	for n < len(rest) {
		b := rest[n]
		if b == '\t' || b == '\n' || b == '\r' || b >= ' ' && b <= '~' {
			break
		}
		n++
	}
	return ucSymbolTok, n
}

// isHex reports whether a byte is a hexadecimal digit.
func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// lineLen returns the length of the line at the start of s, without its
// new line.
func lineLen(s string) int {
	if n := strings.IndexAny(s, "\n\r"); n >= 0 {
		return n
	}
	return len(s)
}

// accLen returns the kind and the length of accTitle: or accDescr: and the
// rest of its line, or of accDescr { up to its end brace, at the start of
// s, or a length of 0.
func accLen(s string) (ucKind, int) {
	for _, key := range []string{"accTitle", "accDescr"} {
		rest, ok := strings.CutPrefix(s, key)
		if !ok {
			continue
		}
		t := strings.TrimLeft(rest, " \t")
		switch {
		case strings.HasPrefix(t, ":") && key == "accTitle":
			return ucAccTitle, len(s) - len(t) + lineLen(t)
		case strings.HasPrefix(t, ":"):
			return ucAccDescr, len(s) - len(t) + lineLen(t)
		case key == "accDescr" && strings.HasPrefix(t, "{"):
			if end := strings.IndexByte(t, '}'); end >= 0 {
				return ucAccDescrBlock, len(s) - len(t) + end + 1
			}
		}
	}
	return ucEOF, 0
}

// jsonStartLen returns the length of json name@ at the start of s, with
// the spaces between, when an open brace follows it, or 0.
func jsonStartLen(s string) int {
	rest, ok := strings.CutPrefix(s, "json")
	if !ok {
		return 0
	}
	t := strings.TrimLeft(rest, " \t")
	if len(t) == len(rest) {
		return 0
	}
	w := wordLen(t)
	if w == 0 {
		return 0
	}
	t = strings.TrimLeft(t[w:], " \t")
	if !strings.HasPrefix(t, "@") {
		return 0
	}
	t = strings.TrimLeft(t[1:], " \t")
	if !strings.HasPrefix(t, "{") {
		return 0
	}
	return len(s) - len(t)
}

// jsonObjectLen returns the length of the object of JSON at the start of
// s, up to the brace that closes it, outside the strings, or 0.
func jsonObjectLen(s string) int {
	depth, quoted, escaped := 0, false, false
	for i := range len(s) {
		c := s[i]
		if quoted {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				quoted = false
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}

// cssIdentLen returns the length of a name of CSS with a dash in it at the
// start of s, such as stroke-width, or 0.
func cssIdentLen(s string) int {
	if s == "" || !isWordByte(s[0]) || s[0] >= '0' && s[0] <= '9' {
		return 0
	}
	n := wordLen(s)
	parts := 0
	for n+1 < len(s) && s[n] == '-' && isWordByte(s[n+1]) {
		n += 1 + wordLen(s[n+1:])
		parts++
	}
	if parts == 0 {
		return 0
	}
	return n
}

// numberLen returns the length of a number at the start of s, with the
// letters of a unit after it, such as 1.5px or .5, or 0.
func numberLen(s string) int {
	digits := func(i int) int {
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		return j - i
	}
	n := digits(0)
	switch {
	case n > 0 && n+1 < len(s) && s[n] == '.' && digits(n+1) > 0:
		n += 1 + digits(n+1)
	case n == 0 && len(s) > 1 && s[0] == '.' && digits(1) > 0:
		n = 1 + digits(1)
	case n == 0:
		return 0
	}
	for n < len(s) && (s[n] >= 'a' && s[n] <= 'z' || s[n] >= 'A' && s[n] <= 'Z') {
		n++
	}
	return n
}
