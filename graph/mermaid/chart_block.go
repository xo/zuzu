package mermaid

import (
	"fmt"
	"html"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/text"
)

// The kinds of the blocks of a block diagram.
const (
	blockLeaf = iota
	blockComposite
	blockSpace
)

// blockArrow is the shape of a block arrow.
const blockArrow = "blockArrow"

// blockNode is a block of a block diagram: a block with a shape and a
// label, a composite block that holds a grid of its own, or a cell of
// space.
type blockNode struct {
	id    string
	label string
	shape string
	kind  int
	// span is the number of columns that the block takes.
	span int
	// columns is the number of columns of the grid of a composite block,
	// or -1 for auto: all the blocks in one row.
	columns int
	kids    []*blockNode
	// parent is the composite block that holds the block, or nil for the
	// diagram.
	parent *blockNode
	// dirs are the directions of a block arrow: right, left, up and down.
	dirs    map[string]bool
	classes []string
	// own is the style of the last style statement of the block.
	own  map[string]string
	line int

	// The layout. lines is the label in lines, tw and th its size, and size
	// the size of its font. w and h are the natural size of the block. The
	// grid of a composite block has cols columns and rows rows of cells of
	// cellW by cellH, under a band of titleH for its label. px and py are
	// the cell of the block in the grid of its parent, and x, y, bw and bh
	// its box. dw and dh are the size of the shape drawn in the middle of
	// the box.
	lines              []string
	tw, th, size       float64
	w, h               float64
	cellW, cellH       float64
	titleH             float64
	cols, rows, px, py int
	x, y, bw, bh       float64
	dw, dh             float64
	row                map[string]any
}

// blockEdge is an edge between two blocks.
type blockEdge struct {
	a, b       string
	label      string
	head, tail string
	thick      bool
	dotted     bool
	line       int
}

// blockStyle is a class or a style statement, which applies after the
// compile reads the whole diagram.
type blockStyle struct {
	ids  []string
	text string
	line int
}

// blockDiagram is the state of the compile of a block diagram.
type blockDiagram struct {
	c *chart
	// src is the body of the diagram, and lines holds the offset in src
	// and the number in the source of each of its lines.
	src   string
	lines [][2]int
	pos   int
	ahead *blockToken

	root      *blockNode
	byID      map[string]*blockNode
	edges     []*blockEdge
	classDefs map[string]map[string]string
	classes   []blockStyle
	styles    []blockStyle
	anon      int

	pad, font float64
	m         text.Measurer
}

// The kinds of the tokens of a block diagram.
const (
	btEOF = iota
	btKey
	btIDBlock
	btColumns
	btSpace
	btClassDef
	btClass
	btStyle
	btEnd
	btID
	btShape
	btLink
	btSize
)

// blockToken is a token of a block diagram.
type blockToken struct {
	kind int
	pos  int
	// text is an id, the name of a class, the text of a style or of a
	// link, n a number of columns, of cells of space or of a width, and
	// ids the ids of a class or a style statement.
	text string
	n    int
	ids  []string
	// A shape: its delimiters, its label and the directions of a block
	// arrow. label is also the label of a link.
	open, close string
	label       string
	dirs        []string
}

// The tokens of a block diagram, in the order in which Mermaid 12.3 tries
// them.
var (
	blockColumnsRE  = regexp.MustCompile(`^columns\s+(auto\b|\d+)`)
	blockSpaceRE    = regexp.MustCompile(`^space(?::(\d+)|\b)`)
	blockUnreadRE   = regexp.MustCompile(`^(default|linkStyle|interpolate)\b`)
	blockClassDefRE = regexp.MustCompile(`^classDef\s+`)
	blockClassIDRE  = regexp.MustCompile(`^(\w+)\s+`)
	blockClassRE    = regexp.MustCompile(`^(class|style)\s+(\w+(?:,\s*\w+)*)`)
	blockAccRE      = regexp.MustCompile(`^(accTitle\s*:|accDescr\s*[:{])`)
	blockEndRE      = regexp.MustCompile(`^end\b`)
	blockIDRE       = regexp.MustCompile(`^[^(\[\n\-){}\s<>:=]+`)
	blockLinkRE     = regexp.MustCompile(`^(?:[xo<]?--+[-xo>]|[xo<]?==+[=xo>]|[xo<]?-?\.+-[xo>]?)`)
	blockOpenLinkRE = regexp.MustCompile(`^(?:[xo<]?--|[xo<]?==|[xo<]?-\.)`)
	blockSizeRE     = regexp.MustCompile(`^:(\d+)`)
	blockArrowEndRE = regexp.MustCompile(`^\]>\s*\(`)
	blockDirRE      = regexp.MustCompile(`^,?\s*(right|left|x|y|up|down)\s*`)
	blockDirEndRE   = regexp.MustCompile(`^\)`)
)

// open3 and close3 are three parentheses, which open and close a double
// circle.
const (
	open3  = "((("
	close3 = ")))"
)

// blockOpens and blockCloses are the delimiters of the shapes of a block,
// in the order in which Mermaid 12.3 tries them.
var (
	blockOpens  = []string{"-)", "(-", "))", ")", open3, "((", "{{", "{", ">", "([", "(", "[[", "[|", "[(", close3, `[\`, "[/", "["}
	blockCloses = []string{open3, close3, "))", "}}", "}", "(-", "-)", "((", "]]", "(", "])", `\]`, "/]", ")]", ")", "]>", "]"}
)

// blockShapes are the shapes of zuzu that draw the pairs of delimiters of
// Mermaid. Another pair draws as a rectangle, as in Mermaid.
var blockShapes = map[string]string{
	"[]": "rect", "()": "rounded", "(())": "circle", ">]": "odd", "{}": "diamond", "{{}}": "hexagon",
	"([])": "stadium", "[[]]": "subroutine", "[()]": "cylinder", "((()))": "doublecircle",
	"[//]": "lean-r", `[\\]`: "lean-l", `[/\]`: "trap-b", `[\/]`: "trap-t",
}

// blockChart compiles a block diagram: blocks on a grid of columns, with
// composite blocks that hold grids of their own, block arrows and edges
// between blocks, into a Vega spec laid out in Go.
func blockChart(c *chart) (map[string]any, bool, error) {
	if err := c.unreadSettings("block.padding"); err != nil {
		return nil, false, err
	}
	b := &blockDiagram{
		c: c, byID: map[string]*blockNode{}, classDefs: map[string]map[string]string{},
		pad: 8, font: 11, m: text.NewEstimate(),
	}
	if m, _ := c.d.Config["block"].(map[string]any); m != nil && m["padding"] != nil {
		if v, ok := configNumber(c.d.Config, "block", "padding"); ok && v >= 0 {
			b.pad = v
		} else if err := c.add("the setting block.padding: %v", m["padding"]); err != nil {
			return nil, false, err
		}
	}
	// The body is one stream of tokens, as in Mermaid: a statement can go
	// on over a line, and a line can hold more than one statement.
	var sb strings.Builder
	add := func(t string, n int) {
		b.lines = append(b.lines, [2]int{sb.Len(), n})
		sb.WriteString(t)
		sb.WriteByte('\n')
	}
	if c.d.Header != "" {
		add(c.d.Header, c.d.N)
	}
	for _, l := range c.d.Lines {
		add(l.Text, l.N)
	}
	b.src = sb.String()
	b.root = &blockNode{id: "root", kind: blockComposite, columns: -1, span: 1}
	n, err := b.document(b.root, false)
	if err != nil {
		return nil, false, err
	}
	if n == 0 {
		c.line = c.d.N
		return nil, false, c.errorf("a block diagram with no statement")
	}
	if err := b.check(); err != nil {
		return nil, false, err
	}
	c.line = 0
	b.measure(b.root)
	b.root.x, b.root.y = 0, 0
	b.assign(b.root, b.root.w, b.root.h)
	return b.spec(), false, nil
}

// lineOf returns the number in the source of the line of an offset of the
// body.
func (b *blockDiagram) lineOf(pos int) int {
	i, _ := slices.BinarySearchFunc(b.lines, pos, func(l [2]int, p int) int { return l[0] - p })
	if i == len(b.lines) || b.lines[i][0] > pos {
		i--
	}
	if i < 0 {
		return b.c.d.N
	}
	return b.lines[i][1]
}

// errorAt returns an error of the syntax at an offset of the body.
func (b *blockDiagram) errorAt(pos int, format string, args ...any) error {
	b.c.line = b.lineOf(pos)
	return b.c.errorf(format, args...)
}

// peek returns the next token without reading it.
func (b *blockDiagram) peek() (blockToken, error) {
	if b.ahead == nil {
		t, err := b.lex()
		if err != nil {
			return t, err
		}
		b.ahead = &t
	}
	return *b.ahead, nil
}

// next reads the next token.
func (b *blockDiagram) next() (blockToken, error) {
	t, err := b.peek()
	b.ahead = nil
	return t, err
}

// lex reads a token at the offset of the body. White space and new lines
// only part the tokens, except in a classDef, a class and a style
// statement, which run to the end of their line.
func (b *blockDiagram) lex() (blockToken, error) {
	s := b.src
	for b.pos < len(s) && strings.IndexByte(" \t\n\r\f\v", s[b.pos]) >= 0 {
		b.pos++
	}
	t := blockToken{pos: b.pos}
	if b.pos >= len(s) {
		return t, nil
	}
	rest := s[b.pos:]
	take := func(n int) string {
		b.pos += n
		return rest[:n]
	}
	toEOL := func() string {
		r := s[b.pos:]
		if i := strings.IndexByte(r, '\n'); i >= 0 {
			r = r[:i]
		}
		b.pos += len(r)
		return strings.TrimSpace(r)
	}
	switch {
	case strings.HasPrefix(rest, "block-beta") && wordEnd(rest, 10):
		take(10)
		t.kind = btKey
	case strings.HasPrefix(rest, "block:"):
		take(6)
		t.kind = btIDBlock
	case strings.HasPrefix(rest, "block") && wordEnd(rest, 5):
		take(5)
		t.kind = btKey
	case blockColumnsRE.MatchString(rest):
		m := blockColumnsRE.FindStringSubmatch(rest)
		take(len(m[0]))
		t.kind, t.n = btColumns, -1
		if m[1] != "auto" {
			t.n, _ = strconv.Atoi(m[1])
		}
	case rest[0] == '"':
		return t, b.errorAt(t.pos, "a text in quotes that is not the label of a block or of a link")
	case blockSpaceRE.MatchString(rest):
		m := blockSpaceRE.FindStringSubmatch(rest)
		take(len(m[0]))
		t.kind, t.n = btSpace, 1
		if m[1] != "" {
			t.n, _ = strconv.Atoi(m[1])
		}
	case blockUnreadRE.MatchString(rest):
		w := blockUnreadRE.FindString(rest)
		return t, b.errorAt(t.pos, "%s, which Mermaid does not read in a block diagram", w)
	case blockClassDefRE.MatchString(rest):
		take(len(blockClassDefRE.FindString(rest)))
		m := blockClassIDRE.FindStringSubmatch(s[b.pos:])
		if m == nil || m[1] == "DEFAULT" {
			return t, b.errorAt(t.pos, "a classDef with no name")
		}
		b.pos += len(m[0])
		t.kind, t.text, t.label = btClassDef, m[1], toEOL()
	case blockClassRE.MatchString(rest):
		m := blockClassRE.FindStringSubmatch(rest)
		take(len(m[0]))
		t.kind = btClass
		if m[1] == "style" {
			t.kind = btStyle
		}
		for id := range strings.SplitSeq(m[2], ",") {
			t.ids = append(t.ids, strings.TrimSpace(id))
		}
		t.text = toEOL()
	case blockAccRE.MatchString(rest):
		w := strings.TrimRight(blockAccRE.FindString(rest), " \t:{")
		return t, b.errorAt(t.pos, "%s, which Mermaid does not read in a block diagram", w)
	case blockEndRE.MatchString(rest):
		take(3)
		t.kind = btEnd
	case strings.HasPrefix(rest, "<["):
		take(2)
		return b.arrowShape(t)
	default:
		for _, o := range blockOpens {
			if strings.HasPrefix(rest, o) {
				take(len(o))
				t.open = o
				return b.shape(t)
			}
		}
		switch {
		case blockIDRE.MatchString(rest):
			t.kind, t.text = btID, take(len(blockIDRE.FindString(rest)))
		case blockLinkRE.MatchString(rest):
			t.kind, t.text = btLink, take(len(blockLinkRE.FindString(rest)))
		case blockOpenLinkRE.MatchString(rest):
			take(len(blockOpenLinkRE.FindString(rest)))
			return b.labeledLink(t)
		case blockSizeRE.MatchString(rest):
			m := blockSizeRE.FindStringSubmatch(rest)
			take(len(m[0]))
			t.kind = btSize
			t.n, _ = strconv.Atoi(m[1])
		default:
			r := rest
			if i := strings.IndexAny(r, " \t\n"); i >= 0 {
				r = r[:i]
			}
			return t, b.errorAt(t.pos, "the text %q", r)
		}
	}
	return t, nil
}

// wordEnd reports whether a word that ends at an offset of a text ends
// there, as \b of a regular expression: the text ends there, or a
// character that is not a letter, a digit or _ comes next.
func wordEnd(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	c := s[i]
	return c != '_' && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z')
}

// quoted reads a label in double quotes at the offset of the body. Mermaid
// 12.3 reads no string of markdown in a block diagram, and no empty label.
func (b *blockDiagram) quoted(at int) (string, error) {
	rest := b.src[b.pos:]
	if strings.HasPrefix(rest, "\"`") {
		return "", b.errorAt(at, "a label of markdown, which Mermaid does not read in a block diagram")
	}
	if !strings.HasPrefix(rest, `"`) {
		return "", b.errorAt(at, "a label that is not in double quotes")
	}
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 {
		return "", b.errorAt(at, "a label with no end quote")
	}
	if end == 0 {
		return "", b.errorAt(at, "an empty label")
	}
	b.pos += end + 2
	return rest[1 : end+1], nil
}

// shape reads the label and the end of a shape after its opening
// delimiter.
func (b *blockDiagram) shape(t blockToken) (blockToken, error) {
	label, err := b.quoted(t.pos)
	if err != nil {
		return t, err
	}
	rest := b.src[b.pos:]
	for _, cl := range blockCloses {
		if strings.HasPrefix(rest, cl) {
			b.pos += len(cl)
			t.kind, t.label, t.close = btShape, label, cl
			return t, nil
		}
	}
	return t, b.errorAt(t.pos, "the shape %s%q has no end", t.open, label)
}

// arrowShape reads a block arrow after <[: its label, and its directions
// in parentheses after ]>.
func (b *blockDiagram) arrowShape(t blockToken) (blockToken, error) {
	label, err := b.quoted(t.pos)
	if err != nil {
		return t, err
	}
	m := blockArrowEndRE.FindString(b.src[b.pos:])
	if m == "" {
		return t, b.errorAt(t.pos, "the block arrow %q has no directions", label)
	}
	b.pos += len(m)
	for {
		rest := b.src[b.pos:]
		if d := blockDirRE.FindStringSubmatch(rest); d != nil {
			b.pos += len(d[0])
			t.dirs = append(t.dirs, d[1])
			continue
		}
		if blockDirEndRE.MatchString(rest) && len(t.dirs) > 0 {
			b.pos++
			t.kind, t.label, t.open, t.close = btShape, label, "<[", "]>"
			return t, nil
		}
		return t, b.errorAt(t.pos, "the directions of the block arrow %q", label)
	}
}

// labeledLink reads the label of a link in double quotes and the rest of
// the link, as in a link with a label from A to B.
func (b *blockDiagram) labeledLink(t blockToken) (blockToken, error) {
	for b.pos < len(b.src) && strings.IndexByte(" \t\n\r\f\v", b.src[b.pos]) >= 0 {
		b.pos++
	}
	label, err := b.quoted(t.pos)
	if err != nil {
		return t, err
	}
	for b.pos < len(b.src) && strings.IndexByte(" \t\n\r\f\v", b.src[b.pos]) >= 0 {
		b.pos++
	}
	m := blockLinkRE.FindString(b.src[b.pos:])
	if m == "" {
		return t, b.errorAt(t.pos, "the link with the label %q has no end", label)
	}
	b.pos += len(m)
	t.kind, t.text, t.label = btLink, m, label
	return t, nil
}

// blockRef is a block that a statement names: its id, and the shape, the
// label and the width that the statement gives it.
type blockRef struct {
	id, label, shape string
	labeled          bool
	dirs             []string
	span             int
	pos              int
}

// document reads the statements of a grid up to its end, or up to the end
// of the source for the diagram, and returns how many it read.
func (b *blockDiagram) document(parent *blockNode, nested bool) (int, error) {
	n := 0
	for {
		t, err := b.next()
		if err != nil {
			return n, err
		}
		b.c.line = b.lineOf(t.pos)
		switch t.kind {
		case btEOF:
			if nested {
				return n, b.errorAt(t.pos, "the block %s has no end", parent.id)
			}
			return n, nil
		case btEnd:
			if !nested {
				return n, b.errorAt(t.pos, "an end with no block")
			}
			if n == 0 {
				return n, b.errorAt(t.pos, "the block %s is empty", parent.id)
			}
			return n, nil
		case btColumns:
			parent.columns = t.n
			if t.n == 0 {
				return n, b.errorAt(t.pos, "columns 0")
			}
		case btSpace:
			for range t.n {
				parent.kids = append(parent.kids, &blockNode{kind: blockSpace, span: 1, line: b.c.line})
			}
		case btKey:
			b.anon++
			blk := &blockNode{id: fmt.Sprintf("block-%d", b.anon), kind: blockComposite, columns: -1, span: 1, line: b.c.line}
			parent.kids = append(parent.kids, blk)
			if _, err := b.document(blk, true); err != nil {
				return n, err
			}
		case btIDBlock:
			if err := b.composite(parent, t); err != nil {
				return n, err
			}
		case btClassDef:
			st, err := b.css(t.label)
			if err != nil {
				return n, err
			}
			if b.classDefs[t.text] == nil {
				b.classDefs[t.text] = map[string]string{}
			}
			maps.Copy(b.classDefs[t.text], st)
		case btClass:
			b.classes = append(b.classes, blockStyle{ids: t.ids, text: t.text, line: b.c.line})
		case btStyle:
			b.styles = append(b.styles, blockStyle{ids: t.ids, text: t.text, line: b.c.line})
		case btID:
			if err := b.statement(parent, t); err != nil {
				return n, err
			}
		case btShape:
			return n, b.errorAt(t.pos, "the shape %s%q%s with no id before it", t.open, t.label, t.close)
		case btLink:
			return n, b.errorAt(t.pos, "the link %q with no block before it", t.text)
		case btSize:
			return n, b.errorAt(t.pos, "the width :%d with no block before it", t.n)
		}
		n++
	}
}

// ref reads a block that a statement names after its id: its shape and
// its width.
func (b *blockDiagram) ref(t blockToken) (blockRef, error) {
	r := blockRef{id: t.text, span: 1, pos: t.pos}
	nt, err := b.peek()
	if err != nil {
		return r, err
	}
	if nt.kind == btShape {
		_, _ = b.next()
		r.label, r.labeled = labelText(html.UnescapeString(nt.label)), true
		r.dirs = nt.dirs
		if nt.open == "<[" {
			r.shape = blockArrow
		} else if s, ok := blockShapes[nt.open+nt.close]; ok {
			r.shape = s
		} else {
			r.shape = "rect"
		}
		if nt, err = b.peek(); err != nil {
			return r, err
		}
	}
	if nt.kind == btSize {
		_, _ = b.next()
		r.span = nt.n
		if r.span < 1 {
			return r, b.errorAt(nt.pos, "the width :%d of %s", nt.n, r.id)
		}
	}
	return r, nil
}

// statement reads a block and the links after it: a, a:2, a --> b, or a
// link with a label.
func (b *blockDiagram) statement(parent *blockNode, t blockToken) error {
	r, err := b.ref(t)
	if err != nil {
		return err
	}
	nt, err := b.peek()
	if err != nil {
		return err
	}
	if nt.kind != btLink {
		return b.use(parent, r, true)
	}
	// Mermaid 12.3 ignores the width of a block with a link after it.
	if err := b.use(parent, r, false); err != nil {
		return err
	}
	from := r.id
	for nt.kind == btLink {
		_, _ = b.next()
		line := b.lineOf(nt.pos)
		to, err := b.next()
		if err != nil {
			return err
		}
		if to.kind != btID {
			return b.errorAt(nt.pos, "the link %q has no block after it", nt.text)
		}
		tr, err := b.ref(to)
		if err != nil {
			return err
		}
		if tr.span != 1 {
			return b.errorAt(to.pos, "a width after the block %s at the end of a link", tr.id)
		}
		if err := b.use(parent, tr, false); err != nil {
			return err
		}
		b.edges = append(b.edges, blockLink(from, tr.id, nt.text, nt.label, line))
		from = tr.id
		if nt, err = b.peek(); err != nil {
			return err
		}
	}
	return nil
}

// blockLink returns the edge of a link, by the marks at its ends: > or <
// for an arrow, o for a circle and x for a cross. == draws it thick, and a
// dot dotted.
func blockLink(a, b, link, label string, line int) *blockEdge {
	e := &blockEdge{a: a, b: b, label: labelText(html.UnescapeString(label)), head: "none", tail: "none", line: line}
	end := func(c byte) string {
		switch c {
		case '>', '<':
			return "normal"
		case 'o':
			return "circle"
		case 'x':
			return "cross"
		}
		return "none"
	}
	e.head, e.tail = end(link[len(link)-1]), end(link[0])
	e.thick = strings.Contains(link, "==")
	e.dotted = strings.Contains(link, ".-")
	return e
}

// use adds a block that a statement names to a grid, or updates the block
// with the id: a shape and a label replace those of the block. sized says
// whether the width of the statement counts.
func (b *blockDiagram) use(parent *blockNode, r blockRef, sized bool) error {
	if old, ok := b.byID[r.id]; ok {
		if old.kind == blockComposite && r.shape != "" {
			if err := b.c.add("the shape of %s, which is a composite block", r.id); err != nil {
				return err
			}
			return nil
		}
		if r.shape != "" {
			old.shape, old.dirs = r.shape, dirSet(r.dirs)
		}
		if r.labeled {
			old.label = r.label
		}
		return nil
	}
	n := &blockNode{id: r.id, label: r.label, shape: r.shape, kind: blockLeaf, span: 1, dirs: dirSet(r.dirs), line: b.c.line}
	if !r.labeled {
		n.label = r.id
	}
	if n.shape == "" {
		n.shape = "rect"
	}
	if sized {
		n.span = r.span
	}
	b.byID[r.id] = n
	parent.kids = append(parent.kids, n)
	return nil
}

// dirSet returns the directions of a block arrow as a set: x is right and
// left, and y is up and down.
func dirSet(dirs []string) map[string]bool {
	if len(dirs) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, d := range dirs {
		switch d {
		case "x":
			set["right"], set["left"] = true, true
		case "y":
			set["up"], set["down"] = true, true
		default:
			set[d] = true
		}
	}
	return set
}

// composite reads a composite block with an id, block:id, block:id:3 or
// block:id["Label"], and the statements of its grid up to end.
func (b *blockDiagram) composite(parent *blockNode, t blockToken) error {
	it, err := b.next()
	if err != nil {
		return err
	}
	if it.kind != btID {
		return b.errorAt(t.pos, "block: with no id after it")
	}
	r, err := b.ref(it)
	if err != nil {
		return err
	}
	if r.shape != "" && r.shape != "rect" {
		if err := b.c.add("the shape of the composite block %s", r.id); err != nil {
			return err
		}
	}
	if nt, err := b.peek(); err != nil {
		return err
	} else if nt.kind == btLink {
		return b.errorAt(nt.pos, "a link after block:%s", r.id)
	}
	if _, ok := b.byID[r.id]; ok {
		return b.errorAt(t.pos, "a second block with the id %s", r.id)
	}
	blk := &blockNode{id: r.id, label: r.label, kind: blockComposite, columns: -1, span: r.span, line: b.lineOf(t.pos)}
	b.byID[r.id] = blk
	parent.kids = append(parent.kids, blk)
	_, err = b.document(blk, true)
	return err
}

// css reads a style of a classDef or of a style statement. A property that
// zuzu does not draw, and a part with no colon, which Mermaid ignores, are
// issues.
func (b *blockDiagram) css(s string) (map[string]string, error) {
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if p := strings.TrimSpace(part); p != "" && !strings.Contains(p, ":") {
			if err := b.c.add("the style %q, which has no colon", p); err != nil {
				return nil, err
			}
		}
	}
	return css(s, b.c.add)
}

// check applies the class and the style statements, and checks the ids of
// the edges.
func (b *blockDiagram) check() error {
	c := b.c
	for _, st := range b.classes {
		c.line = st.line
		for _, id := range st.ids {
			n, ok := b.byID[id]
			if !ok {
				if err := c.add("the class of %s, which is not a block", id); err != nil {
					return err
				}
				continue
			}
			if _, ok := b.classDefs[st.text]; !ok {
				if err := c.add("the class %s, which no classDef names", st.text); err != nil {
					return err
				}
			}
			n.classes = append(n.classes, st.text)
		}
	}
	for _, st := range b.styles {
		c.line = st.line
		if len(st.ids) > 1 {
			return c.errorf("a style of more than one block, %s", strings.Join(st.ids, ","))
		}
		n, ok := b.byID[st.ids[0]]
		if !ok {
			return c.errorf("the style of %s, which is not a block", st.ids[0])
		}
		own, err := b.css(st.text)
		if err != nil {
			return err
		}
		n.own = own
	}
	for _, e := range b.edges {
		c.line = e.line
		if e.a == e.b {
			if err := c.add("the edge from %s to itself", e.a); err != nil {
				return err
			}
		}
	}
	return nil
}
