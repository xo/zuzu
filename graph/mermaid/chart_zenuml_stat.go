package mermaid

import "strings"

// block reads statements up to the end of the source, or up to the brace
// that ends a block when top is false.
func (p *zParser) block(top bool) ([]*zStat, error) {
	var out []*zStat
	for {
		switch k := p.peek(0); {
		case k == zEOF:
			return out, nil
		case k == zCBrace && !top:
			return out, nil
		}
		at := p.pos
		st, err := p.stat()
		if err != nil {
			return nil, err
		}
		if st == nil {
			if err := p.skip(); err != nil {
				return nil, err
			}
			continue
		}
		st.line = p.toks[at].line
		st.comment = p.lx.comments[at]
		out = append(out, st)
	}
}

// braceBlock reads a block in braces, and returns its statements.
func (p *zParser) braceBlock() ([]*zStat, error) {
	p.next()
	stats, err := p.block(false)
	if err != nil {
		return nil, err
	}
	if !p.accept(zCBrace) {
		if err := p.missing("}"); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

// stat reads a statement, or returns nil when the tokens start none.
func (p *zParser) stat() (*zStat, error) {
	switch p.peek(0) {
	case zIf:
		return p.alt()
	case zPar, zOpt, zCritical, zWhile:
		return p.single()
	case zSectionKw, zOBrace:
		return p.section()
	case zRef:
		return p.ref()
	case zTry:
		return p.tcf()
	case zReturn:
		p.next()
		start := p.pos
		p.expr()
		st := &zStat{kind: "return", label: p.text(start)}
		p.accept(zSemi)
		return st, nil
	case zAnnotationRet:
		p.next()
		st := p.async()
		if st == nil {
			return nil, nil
		}
		st.kind = "return"
		return st, nil
	case zDivider:
		t := strings.Trim(strings.TrimSpace(p.next().text), "=")
		return &zStat{kind: "divider", label: strings.TrimSpace(t)}, nil
	case zNew:
		return p.creation("")
	}
	return p.message()
}

// parExpr reads a condition in parentheses and returns its text.
func (p *zParser) parExpr() (string, bool) {
	if p.peek(0) != zOPar {
		return "", false
	}
	p.next()
	start := p.pos
	p.balanced()
	text := p.text(start)
	p.accept(zCPar)
	return text, true
}

// balanced moves up to the parenthesis that closes the one before the
// current token, or up to a brace or the end of the source.
func (p *zParser) balanced() {
	depth := 0
	for {
		switch p.peek(0) {
		case zEOF, zOBrace, zCBrace:
			return
		case zOPar:
			depth++
		case zCPar:
			if depth == 0 {
				return
			}
			depth--
		}
		p.next()
	}
}

// single reads a fragment of one block: par, opt, critical or a loop.
func (p *zParser) single() (*zStat, error) {
	kw := p.next()
	frag := map[zKind]string{zPar: "par", zOpt: "opt", zCritical: "critical", zWhile: "loop"}[kw.kind]
	label, cond := p.parExpr()
	st := &zStat{kind: "fragment", frag: frag}
	if p.peek(0) != zOBrace {
		if cond && kw.kind != zWhile {
			// A condition wants a block after it.
			return st, p.missing("block in braces")
		}
		st.sections = []zSection{{label: label}}
		return st, nil
	}
	stats, err := p.braceBlock()
	st.sections = []zSection{{label: label, stats: stats}}
	return st, err
}

// section reads a section, or a block in braces alone, which is a section
// too.
func (p *zParser) section() (*zStat, error) {
	st := &zStat{kind: "fragment", frag: "section", sections: []zSection{{}}}
	if p.accept(zSectionKw) {
		if p.accept(zOPar) {
			p.balanced()
			p.accept(zCPar)
		}
		if p.peek(0) != zOBrace {
			return st, nil
		}
	}
	stats, err := p.braceBlock()
	st.sections[0].stats = stats
	return st, err
}

// ref reads a ref: a name and the participants that it spans.
func (p *zParser) ref() (*zStat, error) {
	start := p.pos
	p.next()
	if !p.accept(zOPar) {
		return nil, nil
	}
	st := &zStat{kind: "fragment", frag: "ref"}
	if _, ok := p.name(); !ok {
		return st, p.missing("name")
	}
	for p.accept(zComma) {
		for isName(p.peek(0)) {
			n, _ := p.name()
			st.refs = append(st.refs, n)
		}
	}
	if !p.accept(zCPar) {
		return st, p.missing(")")
	}
	st.sections = []zSection{{label: p.text(start)}}
	p.accept(zSemi)
	return st, nil
}

// tcf reads try, its catches and its finally.
func (p *zParser) tcf() (*zStat, error) {
	p.next()
	st := &zStat{kind: "fragment", frag: "tcf"}
	if p.peek(0) != zOBrace {
		return st, p.missing("block in braces")
	}
	stats, err := p.braceBlock()
	if err != nil {
		return nil, err
	}
	st.sections = append(st.sections, zSection{stats: stats})
	for p.accept(zCatch) {
		label := "catch"
		if p.accept(zOPar) {
			start := p.pos
			p.balanced()
			if e := p.text(start); e != "" {
				label += " " + e
			}
			p.accept(zCPar)
		}
		if p.peek(0) != zOBrace {
			return st, p.missing("block in braces")
		}
		stats, err := p.braceBlock()
		if err != nil {
			return nil, err
		}
		st.sections = append(st.sections, zSection{label: label, stats: stats})
	}
	if p.accept(zFinally) {
		if p.peek(0) != zOBrace {
			return st, p.missing("block in braces")
		}
		stats, err := p.braceBlock()
		if err != nil {
			return nil, err
		}
		st.sections = append(st.sections, zSection{label: "finally", stats: stats})
	}
	return st, nil
}

// alt reads if, its else ifs and its else.
func (p *zParser) alt() (*zStat, error) {
	st := &zStat{kind: "fragment", frag: "alt"}
	first := true
	for first || p.peek(0) == zElse && p.peek(1) == zIf {
		if !first {
			p.next()
		}
		p.next()
		label, ok := p.parExpr()
		if !ok {
			return st, p.missing("condition in parentheses")
		}
		sec := zSection{label: label}
		if p.peek(0) == zOBrace {
			stats, err := p.braceBlock()
			if err != nil {
				return nil, err
			}
			sec.stats = stats
		}
		st.sections = append(st.sections, sec)
		first = false
	}
	if p.accept(zElse) {
		sec := zSection{label: "else"}
		if p.peek(0) == zOBrace {
			stats, err := p.braceBlock()
			if err != nil {
				return nil, err
			}
			sec.stats = stats
		}
		st.sections = append(st.sections, sec)
	}
	return st, nil
}

// isAtom reports whether a kind of token is an atom of an expression.
func isAtom(k zKind) bool {
	return isName(k) || k == zNumber || k == zLiteral
}

// assignment reads the start of an assignment, a type, names and =, and
// returns the text of the names.
func (p *zParser) assignment() string {
	k := 0
	if isName(p.peek(0)) && isAtom(p.peek(1)) {
		// A type before the name.
		k = 1
	}
	if !isAtom(p.peek(k)) && p.peek(k) != zNew {
		return ""
	}
	end := k + 1
	for p.peek(end) == zComma && (p.peek(end+1) == zID || p.peek(end+1) == zDigitName) {
		end += 2
	}
	if p.peek(end) != zAssign {
		return ""
	}
	start := p.pos + k
	p.pos += end
	text := p.text(start)
	p.next()
	return text
}

// creation reads new and a constructor, with an assignment before it.
func (p *zParser) creation(assignee string) (*zStat, error) {
	p.next()
	st := &zStat{kind: "creation", assignee: assignee, label: "«create»"}
	construct, ok := p.name()
	if !ok {
		construct = "Missing Constructor"
	}
	st.to = construct
	if ok && p.peek(0) == zOPar {
		p.next()
		start := p.pos
		p.balanced()
		if params := p.text(start); params != "" {
			st.label = "«" + params + "»"
		}
		p.accept(zCPar)
	}
	return st, p.tail(st)
}

// tail reads the end of a message or a creation: a semicolon or a block.
func (p *zParser) tail(st *zStat) error {
	if p.accept(zSemi) {
		return nil
	}
	if p.peek(0) != zOBrace {
		return nil
	}
	st.braces = true
	stats, err := p.braceBlock()
	st.stats = stats
	return err
}

// async reads an async message: a sender, an arrow, a receiver and a text
// after a colon. The sender and the receiver can each go.
func (p *zParser) async() *zStat {
	start := p.pos
	st := &zStat{kind: "async"}
	if isName(p.peek(0)) && (p.peek(1) == zArrow || p.peek(1) == zMinus) {
		st.from, _ = p.name()
		p.next()
		if isName(p.peek(0)) {
			st.to, _ = p.name()
		}
		if !p.accept(zCol) {
			return st
		}
	} else {
		var ok bool
		if st.to, ok = p.name(); !ok || !p.accept(zCol) {
			p.pos = start
			return nil
		}
	}
	if p.peek(0) == zPayload {
		st.label = strings.TrimSpace(p.next().text)
	}
	return st
}

// returnAsync reads a return with a dashed arrow: A-->B: text.
func (p *zParser) returnAsync() *zStat {
	st := &zStat{kind: "return"}
	st.from, _ = p.name()
	p.next()
	if isName(p.peek(0)) {
		st.to, _ = p.name()
	}
	if p.accept(zCol) && p.peek(0) == zPayload {
		st.label = zFormat(p.next().text)
	}
	return st
}

// message reads a statement that starts with a name: a sync message, an
// async message, a creation with an assignment or a return with a dashed
// arrow.
func (p *zParser) message() (*zStat, error) {
	assignee := p.assignment()
	if assignee != "" && p.peek(0) == zNew {
		return p.creation(assignee)
	}
	if assignee == "" {
		if isName(p.peek(0)) && p.peek(1) == zReturnArrow {
			return p.returnAsync(), nil
		}
		if isName(p.peek(0)) && (p.peek(1) == zCol || p.peek(1) == zMinus ||
			p.peek(1) == zArrow && (!isName(p.peek(2)) || p.peek(3) != zDot)) {
			return p.async(), nil
		}
	}
	st := &zStat{kind: "sync", assignee: assignee}
	if isName(p.peek(0)) && p.peek(1) == zArrow {
		st.from, _ = p.name()
		p.next()
	}
	if isName(p.peek(0)) && p.peek(1) == zDot {
		st.to, _ = p.name()
		p.next()
	}
	var sigs []string
	for {
		start := p.pos
		if p.peek(0) == zLBracket && isName(p.peek(1)) && p.peek(2) == zRBracket {
			p.pos += 3
		}
		if _, ok := p.name(); !ok {
			p.pos = start
			break
		}
		if p.peek(0) == zOPar {
			p.next()
			p.balanced()
			p.accept(zCPar)
		}
		sigs = append(sigs, p.text(start))
		if p.peek(0) != zDot || !isName(p.peek(1)) && p.peek(1) != zLBracket {
			break
		}
		p.next()
	}
	if len(sigs) == 0 {
		return nil, nil
	}
	st.label = strings.Join(sigs, ".")
	return st, p.tail(st)
}

// expr reads an expression, such as the value of a return, and reports
// whether it read one.
func (p *zParser) expr() bool {
	if !p.unary() {
		return false
	}
	for {
		switch p.peek(0) {
		case zOp, zMinus, zLT, zGT:
		default:
			return true
		}
		at := p.pos
		p.next()
		if !p.unary() {
			p.pos = at
			return true
		}
	}
}

// unary reads a term of an expression: a sign, then a value, a call, a
// creation or an expression in parentheses.
func (p *zParser) unary() bool {
	switch k := p.peek(0); {
	case k == zMinus || k == zNot:
		p.next()
		return p.unary()
	case k == zOPar:
		p.next()
		p.balanced()
		p.accept(zCPar)
		return true
	case k == zNew:
		p.next()
		if _, ok := p.name(); ok && p.peek(0) == zOPar {
			p.next()
			p.balanced()
			p.accept(zCPar)
		}
		return true
	case isAtom(k):
		p.next()
		for {
			if p.peek(0) == zOPar {
				p.next()
				p.balanced()
				p.accept(zCPar)
				continue
			}
			if p.peek(0) == zDot && isName(p.peek(1)) {
				p.pos += 2
				continue
			}
			return true
		}
	}
	return false
}
