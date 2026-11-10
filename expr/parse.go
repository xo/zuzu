// Package expr parses and evaluates the expression language of Vega (D9).
//
// An expression is a subset of JavaScript expressions, such as datum.price * 2
// or datum.open < datum.close ? 'up' : 'down'. Compile parses an expression
// into a Program, and a Program evaluates against a Scope: the datum, the
// signals and the functions that the runtime gives it. The rules for +, ==
// and truth follow JavaScript, as Vega does. docs/EXPRESSIONS.md holds the
// design.
package expr

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Error is an error of the package expr.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The errors of the package expr.
const (
	// ErrSyntax is the error of an expression that does not parse.
	ErrSyntax Error = "syntax error"
	// ErrUnknownFunction is the error of a call to a function that the
	// scope does not hold.
	ErrUnknownFunction Error = "unknown function"
	// ErrNotAllowed is the error of a construct that Vega does not allow in
	// an expression, such as an assignment.
	ErrNotAllowed Error = "not allowed in an expression"
)

// node is a node of the tree of an expression.
type node any

// The kinds of node.
type (
	literal struct{ v any }
	ident   struct{ name string }
	member  struct{ obj, prop node }
	call    struct {
		name string
		args []node
	}
	unary struct {
		op string
		x  node
	}
	binary struct {
		op   string
		x, y node
	}
	cond   struct{ test, yes, no node }
	array  struct{ elems []node }
	object struct {
		keys []string
		vals []node
	}
)

// token is a token of an expression.
// kind is the kind of a token.
type kind uint8

// The kinds of token.
const (
	kindEOF kind = iota
	kindNum
	kindStr
	kindIdent
	kindPunct
	// kindRegExp is a literal of a regular expression, /pattern/flags,
	// with the pattern in text and the flags in flags.
	kindRegExp
)

type token struct {
	kind  kind
	text  string
	flags string
	num   float64
	pos   int
}

// unicodeEscape reads the four hex digits after the u at l.pos, and the
// low half of a surrogate pair after them, if one follows. It returns the
// rune and the number of bytes it read after the u, or 0 if the digits are
// not there.
func (l *lexer) unicodeEscape() (rune, int) {
	hex := func(at int) (rune, bool) {
		if at+4 > len(l.src) {
			return 0, false
		}
		var r rune
		for _, c := range []byte(l.src[at : at+4]) {
			d := digitOf(c)
			if d < 0 || d > 15 {
				return 0, false
			}
			r = r<<4 | rune(d)
		}
		return r, true
	}
	r, ok := hex(l.pos + 1)
	if !ok {
		return 0, 0
	}
	if utf16.IsSurrogate(r) && strings.HasPrefix(l.src[l.pos+5:], "\\u") {
		if lo, ok := hex(l.pos + 7); ok {
			if pair := utf16.DecodeRune(r, lo); pair != utf8.RuneError {
				return pair, 10
			}
		}
	}
	return r, 4
}

// lexer splits an expression into tokens.
type lexer struct {
	src  string
	pos  int
	toks []token
}

// puncts are the operators and punctuation, longest first.
var puncts = []string{
	">>>", "===", "!==", "==", "!=", "<=", ">=", "&&", "||", "<<", ">>",
	"+", "-", "*", "/", "%", "<", ">", "!", "~", "&", "|", "^", "?", ":", "(", ")",
	"[", "]", "{", "}", ",", ".", "=",
}

// lex splits src into tokens.
func lex(src string) ([]token, error) {
	l := &lexer{src: src}
	for {
		for l.pos < len(src) {
			r, n := utf8.DecodeRuneInString(src[l.pos:])
			if !unicode.IsSpace(r) {
				break
			}
			l.pos += n
		}
		if l.pos >= len(src) {
			l.toks = append(l.toks, token{kind: kindEOF, pos: l.pos})
			return l.toks, nil
		}
		start := l.pos
		c := src[l.pos]
		switch {
		case c >= '0' && c <= '9' || c == '.' && l.pos+1 < len(src) && src[l.pos+1] >= '0' && src[l.pos+1] <= '9':
			if err := l.number(); err != nil {
				return nil, err
			}
		case c == '\'' || c == '"':
			if err := l.str(c); err != nil {
				return nil, err
			}
		case c == '/' && l.regExpAllowed():
			if err := l.regExp(); err != nil {
				return nil, err
			}
		case c == '_' || c == '$' || unicode.IsLetter(rune(c)) || c >= utf8.RuneSelf:
			for l.pos < len(src) {
				r, n := utf8.DecodeRuneInString(src[l.pos:])
				if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				l.pos += n
			}
			l.toks = append(l.toks, token{kind: kindIdent, text: src[start:l.pos], pos: start})
		default:
			found := false
			for _, p := range puncts {
				if strings.HasPrefix(src[l.pos:], p) {
					l.toks = append(l.toks, token{kind: kindPunct, text: p, pos: start})
					l.pos += len(p)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("reading %q at %d: %w", string(c), start, ErrSyntax)
			}
		}
	}
}

// regExpAllowed reports whether a slash at this place starts a regular
// expression rather than a division: it does where an operand can start,
// which is not after a number, a string, a name or a closing bracket.
func (l *lexer) regExpAllowed() bool {
	if len(l.toks) == 0 {
		return true
	}
	t := l.toks[len(l.toks)-1]
	switch t.kind {
	case kindNum, kindStr, kindIdent, kindRegExp:
		return false
	case kindPunct:
		return t.text != ")" && t.text != "]" && t.text != "}"
	}
	return true
}

// regExp reads a literal of a regular expression: the pattern up to a
// slash that no backslash escapes and that lies outside a class in
// brackets, then the flags.
func (l *lexer) regExp() error {
	start := l.pos
	src := l.src
	i := l.pos + 1
	inClass := false
	for ; ; i++ {
		if i >= len(src) || src[i] == '\n' || src[i] == '\r' {
			return fmt.Errorf("reading the regular expression at %d: %w", start, ErrSyntax)
		}
		c := src[i]
		if c == '\\' {
			i++
			continue
		}
		switch {
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '/' && !inClass:
			l.regExpFlags(start, src[start+1:i], i+1)
			return nil
		}
	}
}

// regExpFlags reads the flags of a regular expression from i, and adds its
// token.
func (l *lexer) regExpFlags(start int, pattern string, i int) {
	src := l.src
	f := i
	for i < len(src) {
		r, n := utf8.DecodeRuneInString(src[i:])
		if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		i += n
	}
	l.toks = append(l.toks, token{kind: kindRegExp, text: pattern, flags: src[f:i], pos: start})
	l.pos = i
}

// number reads a number literal: decimal, with an exponent, or hexadecimal.
func (l *lexer) number() error {
	start := l.pos
	src := l.src
	if strings.HasPrefix(src[l.pos:], "0x") || strings.HasPrefix(src[l.pos:], "0X") {
		l.pos += 2
		for l.pos < len(src) && strings.ContainsRune("0123456789abcdefABCDEF", rune(src[l.pos])) {
			l.pos++
		}
		v, err := strconv.ParseInt(src[start+2:l.pos], 16, 64)
		if err != nil {
			return fmt.Errorf("reading the number %q: %w", src[start:l.pos], ErrSyntax)
		}
		l.toks = append(l.toks, token{kind: kindNum, num: float64(v), pos: start})
		return nil
	}
	for l.pos < len(src) && (src[l.pos] >= '0' && src[l.pos] <= '9' || src[l.pos] == '.') {
		l.pos++
	}
	if l.pos < len(src) && (src[l.pos] == 'e' || src[l.pos] == 'E') {
		l.pos++
		if l.pos < len(src) && (src[l.pos] == '+' || src[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(src) && src[l.pos] >= '0' && src[l.pos] <= '9' {
			l.pos++
		}
	}
	v, err := strconv.ParseFloat(src[start:l.pos], 64)
	if err != nil {
		return fmt.Errorf("reading the number %q: %w", src[start:l.pos], ErrSyntax)
	}
	l.toks = append(l.toks, token{kind: kindNum, num: v, pos: start})
	return nil
}

// str reads a string literal in quotes, with the escapes of JavaScript.
func (l *lexer) str(q byte) error {
	start := l.pos
	l.pos++
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == q:
			l.pos++
			l.toks = append(l.toks, token{kind: kindStr, text: b.String(), pos: start})
			return nil
		case c == '\\' && l.pos+1 < len(l.src):
			l.pos++
			switch e := l.src[l.pos]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'v':
				b.WriteByte('\v')
			case '0':
				b.WriteByte(0)
			case 'u':
				r, n := l.unicodeEscape()
				if n == 0 {
					b.WriteByte('u')
					break
				}
				b.WriteRune(r)
				l.pos += n
			default:
				b.WriteByte(e)
			}
			l.pos++
		default:
			b.WriteByte(c)
			l.pos++
		}
	}
	return fmt.Errorf("reading a string at %d: no closing quote: %w", start, ErrSyntax)
}

// parser builds the tree of an expression with precedence climbing.
type parser struct {
	toks []token
	i    int
	src  string
}

// parse parses an expression into its tree.
func parse(src string) (node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, src: src}
	n, err := p.expr()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != kindEOF {
		return nil, p.fail(t, "unexpected")
	}
	return n, nil
}

// peek returns the next token.
func (p *parser) peek() token {
	return p.toks[p.i]
}

// next returns the next token and moves past it.
func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != kindEOF {
		p.i++
	}
	return t
}

// is reports whether the next token is the punctuation s.
func (p *parser) is(s string) bool {
	t := p.peek()
	return t.kind == kindPunct && t.text == s
}

// expect moves past the punctuation s, or fails.
func (p *parser) expect(s string) error {
	if !p.is(s) {
		return p.fail(p.peek(), "expected "+s+", found")
	}
	p.next()
	return nil
}

// fail returns a syntax error at a token.
func (p *parser) fail(t token, what string) error {
	text := t.text
	if t.kind == kindEOF {
		text = "the end"
	}
	return fmt.Errorf("parsing %q: %s %q at %d: %w", p.src, what, text, t.pos, ErrSyntax)
}

// expr parses a conditional expression. An assignment is refused, as Vega
// refuses it.
func (p *parser) expr() (node, error) {
	test, err := p.binary(0)
	if err != nil {
		return nil, err
	}
	if p.is("=") {
		return nil, fmt.Errorf("parsing %q: an assignment: %w", p.src, ErrNotAllowed)
	}
	if !p.is("?") {
		return test, nil
	}
	p.next()
	yes, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(":"); err != nil {
		return nil, err
	}
	no, err := p.expr()
	if err != nil {
		return nil, err
	}
	return cond{test, yes, no}, nil
}

// precedence is the binding power of each binary operator.
var precedence = map[string]int{
	"||": 2, "&&": 3, "|": 4, "^": 5, "&": 6,
	"==": 7, "!=": 7, "===": 7, "!==": 7,
	"<": 8, ">": 8, "<=": 8, ">=": 8, "in": 8,
	"<<": 9, ">>": 9, ">>>": 9,
	"+": 10, "-": 10, "*": 11, "/": 11, "%": 11,
}

// binary parses binary operators at or above a binding power.
func (p *parser) binary(least int) (node, error) {
	x, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		op := t.text
		prec, ok := precedence[op]
		isOp := t.kind == kindPunct || (t.kind == kindIdent && op == "in")
		if !ok || !isOp || prec < least {
			return x, nil
		}
		p.next()
		// Every binary operator binds to the left.
		y, err := p.binary(prec + 1)
		if err != nil {
			return nil, err
		}
		x = binary{op, x, y}
	}
}

// unary parses a prefix operator.
func (p *parser) unary() (node, error) {
	t := p.peek()
	if t.kind == kindPunct && (t.text == "!" || t.text == "-" || t.text == "+" || t.text == "~") {
		p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return unary{t.text, x}, nil
	}
	return p.postfix()
}

// postfix parses member access and calls after a primary expression.
func (p *parser) postfix() (node, error) {
	x, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.is("."):
			p.next()
			t := p.next()
			if t.kind != kindIdent {
				return nil, p.fail(t, "expected a name after ., found")
			}
			x = member{x, literal{t.text}}
		case p.is("["):
			p.next()
			prop, err := p.expr()
			if err != nil {
				return nil, err
			}
			if err := p.expect("]"); err != nil {
				return nil, err
			}
			x = member{x, prop}
		case p.is("("):
			id, ok := x.(ident)
			if !ok {
				return nil, fmt.Errorf("parsing %q: a call of something other than a function name: %w", p.src, ErrNotAllowed)
			}
			p.next()
			var args []node
			for !p.is(")") {
				a, err := p.expr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if !p.is(",") {
					break
				}
				p.next()
			}
			if err := p.expect(")"); err != nil {
				return nil, err
			}
			x = call{id.name, args}
		default:
			return x, nil
		}
	}
}

// primary parses a literal, a name, a group, an array or an object.
func (p *parser) primary() (node, error) {
	t := p.next()
	switch t.kind {
	case kindNum:
		return literal{t.num}, nil
	case kindStr:
		return literal{t.text}, nil
	case kindRegExp:
		re, err := newRegExp(t.text, t.flags)
		if err != nil {
			return nil, fmt.Errorf("the regular expression /%s/%s at %d: %w: %w", t.text, t.flags, t.pos, ErrSyntax, err)
		}
		return literal{re}, nil
	case kindIdent:
		switch t.text {
		case "true":
			return literal{true}, nil
		case "false":
			return literal{false}, nil
		case "null":
			return literal{nil}, nil
		case "NaN":
			return literal{nan()}, nil
		case "this", "new", "function", "var", "let", "const", "typeof", "void", "delete", "instanceof":
			return nil, fmt.Errorf("parsing %q: %s: %w", p.src, t.text, ErrNotAllowed)
		}
		return ident{t.text}, nil
	case kindPunct:
		switch t.text {
		case "(":
			x, err := p.expr()
			if err != nil {
				return nil, err
			}
			return x, p.expect(")")
		case "[":
			var elems []node
			for !p.is("]") {
				e, err := p.expr()
				if err != nil {
					return nil, err
				}
				elems = append(elems, e)
				if !p.is(",") {
					break
				}
				p.next()
			}
			return array{elems}, p.expect("]")
		case "{":
			var obj object
			for !p.is("}") {
				k := p.next()
				if k.kind != kindIdent && k.kind != kindStr && k.kind != kindNum {
					return nil, p.fail(k, "expected a key, found")
				}
				key := k.text
				if k.kind == kindNum {
					key = strconv.FormatFloat(k.num, 'f', -1, 64)
				}
				if err := p.expect(":"); err != nil {
					return nil, err
				}
				v, err := p.expr()
				if err != nil {
					return nil, err
				}
				obj.keys = append(obj.keys, key)
				obj.vals = append(obj.vals, v)
				if !p.is(",") {
					break
				}
				p.next()
			}
			return obj, p.expect("}")
		}
	}
	return nil, p.fail(t, "unexpected")
}
