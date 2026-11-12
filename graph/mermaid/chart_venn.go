package mermaid

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// vennArea is a set or a union of a venn diagram: the ids of its sets in
// order, its size, and its label.
type vennArea struct {
	sets     []string
	size     float64
	label    string
	hasLabel bool
}

// vennText is a text of a venn diagram: the ids of the sets of its area
// in order, its id, and its label, which draws in place of the id.
type vennText struct {
	sets     []string
	id       string
	label    string
	hasLabel bool
}

// venn is a venn diagram as the compiler reads it.
type venn struct {
	c        *chart
	title    string
	hasTitle bool
	areas    []*vennArea
	texts    []*vennText
	// styles are the styles of the style statements by their targets, the
	// ids in order joined by |. A later statement adds to an earlier one.
	styles map[string]map[string]string
	// styleLines are the numbers of the lines of the last style statement
	// of each target, for the report.
	styleLines map[string]int
	// known are the ids of the sets of the set statements so far, which
	// a union can name.
	known map[string]bool
	// current are the sets of the last set or union, which an indented
	// text belongs to.
	current []string
	// indent is the indent mode of the lexer of Mermaid: true after a set
	// or a union, and false after a line that starts with no space.
	indent bool
	// width and height are venn.width and venn.height, and padding is
	// venn.padding, in the pixels of Mermaid.
	width, height, padding float64
}

// vennChart compiles a venn diagram: a circle for each set, sized by the
// size of the set, with the overlaps of the unions, and their labels and
// texts, into a Vega spec (D31). The compiler lays the circles out in Go
// with the rules of Mermaid 12.3, and the spec holds rows of paths and
// text.
func vennChart(c *chart) (map[string]any, bool, error) {
	v := &venn{c: c, styles: map[string]map[string]string{}, styleLines: map[string]int{}, known: map[string]bool{}, width: 800, height: 450, padding: 8}
	if err := v.settings(); err != nil {
		return nil, false, err
	}
	if err := v.parse(); err != nil {
		return nil, false, err
	}
	// The title of the front matter draws when the source has none, as
	// in Mermaid, and Chart adds it to the spec.
	title := c.d.Title
	if v.hasTitle {
		title = v.title
	}
	l, err := newVennLayout(v, title != "")
	if err != nil {
		return nil, false, err
	}
	var spec map[string]any
	if l == nil {
		// Mermaid draws nothing for a set of size 0, and so nothing but
		// the title when every set has the size 0.
		spec = emptyChart(title, "")
	} else {
		spec = l.spec()
	}
	if v.hasTitle {
		spec["title"] = map[string]any{"text": v.title, "anchor": "start"}
	}
	return spec, false, nil
}

// settings reads the config of a venn diagram. zuzu reads venn.width,
// venn.height and venn.padding. venn.useDebugLayout draws the boxes of the
// layout of the texts, which zuzu does not draw, so true is an issue.
// venn.look is a hint, as look is (D32). Each other setting is an issue.
func (v *venn) settings() error {
	c := v.c
	if err := c.unreadSettings("venn.width", "venn.height", "venn.padding", "venn.useDebugLayout", "venn.look"); err != nil {
		return err
	}
	sec, _ := c.d.Config["venn"].(map[string]any)
	for _, k := range []string{"width", "height", "padding"} {
		if _, ok := sec[k]; !ok {
			continue
		}
		n, ok := configNumber(c.d.Config, "venn", k)
		if !ok || n < 0 || n == 0 && k != "padding" {
			if err := c.add("the setting venn.%s %v, which is not a positive number", k, sec[k]); err != nil {
				return err
			}
			continue
		}
		switch k {
		case "width":
			v.width = n
		case "height":
			v.height = n
		default:
			v.padding = n
		}
	}
	if d, ok := sec["useDebugLayout"]; ok && d != false {
		return c.add("the setting venn.useDebugLayout %v, which draws the layout of the texts", d)
	}
	return nil
}

// vennTok is the kind of a token of a venn diagram, as the lexer of
// Mermaid 12.3 reads it.
type vennTok int

const (
	vennEOF vennTok = iota
	vennNewline
	vennTitle
	vennKeyword
	vennSet
	vennUnion
	vennTextTok
	vennIndentText
	vennStyle
	vennBracket
	vennNumber
	vennHex
	vennRGBA
	vennRGB
	vennIdent
	vennString
	vennComma
	vennColon
)

// vennToken is a token of a venn diagram, with the number of its line.
type vennToken struct {
	kind vennTok
	text string
	line int
}

// String names a token in an error.
func (t vennToken) String() string {
	switch t.kind {
	case vennEOF:
		return "the end of the source"
	case vennNewline:
		return "the end of the line"
	case vennString:
		return "the string " + t.text
	}
	return strconv.Quote(t.text)
}

// The rules of the lexer of Mermaid 12.3 for a venn diagram, in its order,
// in any case. Each one matches at the start of the rest of the source.
var vennRules = []struct {
	re   *regexp.Regexp
	kind vennTok
}{
	{regexp.MustCompile(`(?i)^title\s[^#\n;]+`), vennTitle},
	{regexp.MustCompile(`(?i)^venn-beta\b`), vennKeyword},
	{regexp.MustCompile(`(?i)^set\b`), vennSet},
	{regexp.MustCompile(`(?i)^union\b`), vennUnion},
	{regexp.MustCompile(`(?i)^text\b`), vennTextTok},
	{regexp.MustCompile(`(?i)^style\b`), vennStyle},
	{regexp.MustCompile(`^\["[^"]*"\]`), vennBracket},
	{regexp.MustCompile(`^\[[^\]"]+\]`), vennBracket},
	{regexp.MustCompile(`^[+-]?(\d+(\.\d+)?|\.\d+)`), vennNumber},
	{regexp.MustCompile(`^#[0-9a-fA-F]{3,8}`), vennHex},
	{regexp.MustCompile(`(?i)^rgba\(\s*[0-9.]+\s*,\s*[0-9.]+\s*,\s*[0-9.]+\s*,\s*[0-9.]+\s*\)`), vennRGBA},
	{regexp.MustCompile(`(?i)^rgb\(\s*[0-9.]+\s*,\s*[0-9.]+\s*,\s*[0-9.]+\s*\)`), vennRGB},
	{regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9\-_]*`), vennIdent},
	{regexp.MustCompile(`^"[^"]*"`), vennString},
	{regexp.MustCompile(`^,`), vennComma},
	{regexp.MustCompile(`^:`), vennColon},
}

// vennIndentRE matches the spaces before the keyword text at the start of
// a line.
var vennIndentRE = regexp.MustCompile(`(?i)^[ \t]+text\b`)

// vennLexer reads the tokens of the body of a venn diagram as the lexer of
// Mermaid 12.3 does. At the start of a line, the spaces before the
// keyword text make an indented text when the last statement was a set or
// a union, and a line that starts with no space ends that mode.
type vennLexer struct {
	v   *venn
	src string
	pos int
	// starts and nums are the offset of each line in src and its number
	// in the source.
	starts, nums []int
	// bol is true at the start of a line.
	bol bool
	// skipText is true after an indented text, so that its keyword text
	// is no token.
	skipText bool
}

// line returns the number of the line at an offset of the source.
func (lx *vennLexer) line(pos int) int {
	i := sort.Search(len(lx.starts), func(i int) bool { return lx.starts[i] > pos }) - 1
	return lx.nums[max(i, 0)]
}

// next returns the next token.
func (lx *vennLexer) next() (vennToken, error) {
	for {
		rest := lx.src[lx.pos:]
		tok := vennToken{line: lx.line(lx.pos)}
		if lx.bol {
			n := len(rest) - len(strings.TrimLeft(rest, " \t"))
			switch {
			case n > 0 && n < len(rest) && rest[n] == '\n':
				lx.pos += n
				continue
			case n > 0 && vennIndentRE.MatchString(rest):
				lx.pos += n
				if lx.v.indent {
					lx.skipText, lx.bol = true, false
					tok.kind, tok.text = vennIndentText, rest[:n]
					return tok, nil
				}
				continue
			case n > 0:
				lx.pos += n
				continue
			case rest != "" && rest[0] != '\n':
				// The first character of a line with no space before it
				// ends the indent mode.
				lx.v.indent, lx.bol = false, false
				continue
			}
		}
		switch {
		case rest == "":
			tok.kind = vennEOF
			return tok, nil
		case rest[0] == '\n':
			lx.pos += len(rest) - len(strings.TrimLeft(rest, "\n"))
			lx.bol = true
			tok.kind = vennNewline
			return tok, nil
		case rest[0] == ' ' || rest[0] == '\t':
			lx.pos += len(rest) - len(strings.TrimLeft(rest, " \t"))
			continue
		}
		matched := false
		for i, r := range vennRules {
			m := r.re.FindString(rest)
			if m == "" {
				continue
			}
			lx.pos += len(m)
			matched = true
			tok.kind, tok.text = r.kind, m
			switch {
			case r.kind == vennTextTok && lx.skipText:
				lx.skipText = false
				tok.kind = -1
			case r.kind == vennBracket && i == 6:
				tok.text = m[2 : len(m)-2]
			case r.kind == vennBracket:
				tok.text = strings.TrimSpace(m[1 : len(m)-1])
			}
			break
		}
		if !matched {
			lx.v.c.line = tok.line
			return tok, lx.v.c.errorf("the text %q", firstWord(rest))
		}
		if tok.kind >= 0 {
			return tok, nil
		}
	}
}

// vennParser reads the statements of a venn diagram from the tokens of
// its lexer, as the grammar of Mermaid 12.3 does.
type vennParser struct {
	v      *venn
	lx     *vennLexer
	peeked *vennToken
}

// next returns the next token.
func (p *vennParser) next() (vennToken, error) {
	if p.peeked != nil {
		t := *p.peeked
		p.peeked = nil
		return t, nil
	}
	return p.lx.next()
}

// peek returns the next token and keeps it.
func (p *vennParser) peek() (vennToken, error) {
	if p.peeked == nil {
		t, err := p.lx.next()
		if err != nil {
			return t, err
		}
		p.peeked = &t
	}
	return *p.peeked, nil
}

// unexpected returns the error of a token where the grammar expects
// another.
func (p *vennParser) unexpected(t vennToken, want string) error {
	p.v.c.line = t.line
	return p.v.c.errorf("%s, where %s is expected", t, want)
}

// parse reads the statements of a venn diagram. The text after the keyword
// on the first line holds statements too, as in Mermaid.
func (v *venn) parse() error {
	lx := &vennLexer{v: v}
	var b strings.Builder
	add := func(text string, n, indent int) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		lx.starts, lx.nums = append(lx.starts, b.Len()), append(lx.nums, n)
		b.WriteString(strings.Repeat(" ", indent))
		b.WriteString(text)
	}
	add(v.c.d.Header, v.c.d.N, 0)
	for _, l := range v.c.d.Lines {
		add(l.Text, l.N, l.Indent)
	}
	lx.src = b.String()
	p := &vennParser{v: v, lx: lx}
	for {
		t, err := p.next()
		if err != nil {
			return err
		}
		v.c.line = t.line
		switch t.kind {
		case vennEOF:
			if len(v.areas) == 0 {
				// Mermaid fails to lay out a diagram with no set.
				return v.c.errorf("a venn diagram with no set")
			}
			return nil
		case vennNewline:
		case vennTitle:
			v.title, v.hasTitle = strings.TrimSpace(t.text[6:]), true
		case vennSet, vennUnion:
			err = p.area(t)
		case vennTextTok:
			var sets []string
			if sets, err = p.identifiers(); err == nil {
				err = p.text(sets)
			}
		case vennIndentText:
			err = p.text(v.current)
		case vennStyle:
			err = p.style()
		default:
			err = p.unexpected(t, "a statement")
		}
		if err != nil {
			return err
		}
	}
}

// area reads a set or a union: its ids, then a label in brackets and a
// size after a colon, each optional.
func (p *vennParser) area(kw vennToken) error {
	v := p.v
	var ids []string
	if kw.kind == vennSet {
		t, err := p.next()
		if err != nil {
			return err
		}
		if t.kind != vennIdent && t.kind != vennString {
			return p.unexpected(t, "the id of a set")
		}
		ids = []string{t.text}
	} else {
		var err error
		if ids, err = p.identifiers(); err != nil {
			return err
		}
	}
	a := &vennArea{}
	t, err := p.peek()
	if err != nil {
		return err
	}
	if t.kind == vennBracket {
		_, _ = p.next()
		a.label, a.hasLabel = vennNormalize(t.text), true
		if t, err = p.peek(); err != nil {
			return err
		}
	}
	size := 10 / float64(len(ids)*len(ids))
	if t.kind == vennColon {
		_, _ = p.next()
		n, err := p.next()
		if err != nil {
			return err
		}
		if n.kind != vennNumber {
			return p.unexpected(n, "a size")
		}
		size, _ = strconv.ParseFloat(n.text, 64)
	}
	v.c.line = kw.line
	if size < 0 || math.IsInf(size, 0) {
		// Mermaid draws no circle of such a size.
		return v.c.errorf("the size %v, which is not a number of 0 or more", size)
	}
	if kw.kind == vennUnion {
		if len(ids) < 2 {
			return v.c.errorf("a union of one set")
		}
		var unknown []string
		for _, id := range ids {
			if id = vennNormalize(id); !v.known[id] {
				unknown = append(unknown, id)
			}
		}
		if len(unknown) > 0 {
			return v.c.errorf("a union of the set %s, which no set statement names", strings.Join(unknown, ", "))
		}
	}
	a.sets, a.size = vennSorted(ids), size
	v.areas = append(v.areas, a)
	v.current = a.sets
	if len(a.sets) == 1 {
		v.known[a.sets[0]] = true
	}
	v.indent = true
	return nil
}

// identifiers reads a list of ids with commas between them.
func (p *vennParser) identifiers() ([]string, error) {
	var ids []string
	for {
		t, err := p.next()
		if err != nil {
			return nil, err
		}
		if t.kind != vennIdent && t.kind != vennString {
			return nil, p.unexpected(t, "the id of a set")
		}
		ids = append(ids, t.text)
		if t, err = p.peek(); err != nil {
			return nil, err
		}
		if t.kind != vennComma {
			return ids, nil
		}
		_, _ = p.next()
	}
}

// text reads the id of a text and its label in brackets, which is
// optional, and adds the text to the area of the sets.
func (p *vennParser) text(sets []string) error {
	t, err := p.next()
	if err != nil {
		return err
	}
	if t.kind != vennIdent && t.kind != vennString && t.kind != vennNumber {
		return p.unexpected(t, "the id of a text")
	}
	x := &vennText{sets: vennSorted(sets), id: vennNormalize(t.text)}
	if t, err = p.peek(); err != nil {
		return err
	}
	if t.kind == vennBracket {
		_, _ = p.next()
		x.label, x.hasLabel = vennNormalize(t.text), true
	}
	p.v.texts = append(p.v.texts, x)
	return nil
}

// style reads a style statement: the ids of its targets and a list of
// fields, each a name, a colon and a value, with commas between them.
func (p *vennParser) style() error {
	targets, err := p.identifiers()
	if err != nil {
		return err
	}
	fields := map[string]string{}
	for {
		name, err := p.next()
		if err != nil {
			return err
		}
		if name.kind != vennIdent {
			return p.unexpected(name, "the name of a style")
		}
		colon, err := p.next()
		if err != nil {
			return err
		}
		if colon.kind != vennColon {
			return p.unexpected(colon, "a colon")
		}
		value, err := p.styleValue()
		if err != nil {
			return err
		}
		fields[name.text] = value
		t, err := p.peek()
		if err != nil {
			return err
		}
		if t.kind != vennComma {
			break
		}
		_, _ = p.next()
	}
	key := strings.Join(vennSorted(targets), "|")
	if p.v.styles[key] == nil {
		p.v.styles[key] = map[string]string{}
	}
	maps.Copy(p.v.styles[key], fields)
	p.v.styleLines[key] = p.v.c.line
	return nil
}

// styleValue reads the value of a style: a string, or one or more numbers,
// names and colors, which Mermaid joins with spaces, so that 2px is "2 px".
func (p *vennParser) styleValue() (string, error) {
	t, err := p.next()
	if err != nil {
		return "", err
	}
	if t.kind == vennString {
		return vennNormalize(t.text), nil
	}
	if !vennValue(t.kind) {
		return "", p.unexpected(t, "the value of a style")
	}
	parts := []string{t.text}
	for {
		if t, err = p.peek(); err != nil {
			return "", err
		}
		if !vennValue(t.kind) {
			return strings.Join(parts, " "), nil
		}
		_, _ = p.next()
		parts = append(parts, t.text)
	}
}

// vennValue reports whether a token can be a part of the value of a
// style: a number, a name or a color.
func vennValue(k vennTok) bool {
	switch k {
	case vennNumber, vennIdent, vennHex, vennRGB, vennRGBA:
		return true
	}
	return false
}

// vennNormalize returns a text as Mermaid keeps it: trimmed, and without
// the double quotes around it.
func vennNormalize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// vennSorted returns ids without their quotes, sorted, as Mermaid keeps
// the sets of an area.
func vennSorted(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = vennNormalize(id)
	}
	slices.Sort(out)
	return out
}
