package mermaid

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// tvKeyword is the keyword of a tree view.
const tvKeyword = "treeView-beta"

// tvNode is an item of a tree view: a file, or a directory, whose name
// ends in a slash in the source.
type tvNode struct {
	name  string
	dir   bool
	level int
	// class is the class of :::class, and highlight draws a band behind
	// the item.
	class string
	// icon is the icon of icon(name), or none, or empty when the source
	// gives no icon.
	icon string
	desc string

	children []*tvNode
}

// treeView is a tree view as the compiler reads it.
type treeView struct {
	c *chart
	// root is the directory / that holds the items of the source at the
	// top, as in Mermaid, which draws it as the first row.
	root     *tvNode
	title    string
	hasTitle bool

	// The settings of treeView, in the pixels of Mermaid.
	rowIndent, padX, padY, thickness float64
	showIcons                        bool
	iconPack                         string
	fileIcons, extIcons              map[string]string

	// The theme variables of treeView: the size of a label in the pixels
	// of Mermaid, and the colors that replace those of the theme.
	labelSize                                                       float64
	labelColor, lineColor, iconColor, descColor, highBg, highStroke string
}

// tvToken is a kind of token of the lexer of a tree view.
type tvToken int

// The tokens of a tree view, in the order in which the lexer of Mermaid
// 12.3 tries them. The first that matches wins.
const (
	tvKey tvToken = iota
	tvAccDescr
	tvAccTitle
	tvTitle
	tvClass
	tvIcon
	tvDesc
	tvIndent
	tvQuoted
	tvComment
	tvNewline
	tvBare
)

// tvTok is a token of a tree view: its kind, its text, and the number of
// its line in the source.
type tvTok struct {
	kind tvToken
	text string
	line int
}

// The patterns of the lines of a tree view drawn with the characters of
// boxes, as Mermaid 12.3 reads them before the parse.
var (
	// tvCommentRE matches a comment that Mermaid drops from a source
	// before the parse, with the blank lines before it.
	tvCommentRE = regexp.MustCompile(`(?m)^\s*%%[^{\n][^\n]*\n?`)
	// tvMetaRE matches a line of a title, an accTitle or an accDescr.
	tvMetaRE = regexp.MustCompile(`^\s*(title[\t ]|accTitle[\t ]*:|accDescr[\t ]*[:{])`)
	// tvBoxRE matches a line that holds a character of a box.
	tvBoxRE = regexp.MustCompile(`[─━│┃└┗├┣]`)
	// tvDecorRE matches a line of only spaces and vertical lines.
	tvDecorRE = regexp.MustCompile(`^[\s│┃]+$`)
	// tvBoxOnlyRE matches a line of only spaces and characters of boxes.
	tvBoxOnlyRE = regexp.MustCompile(`^[\s─━│┃└┗├┣]+$`)
	// tvSpacesRE matches a run of two spaces or tabs or more.
	tvSpacesRE = regexp.MustCompile(`[\t ]{2,}`)
)

// treeViewChart compiles a tree view: a tree of files and directories,
// one row each, indented by its depth, with lines that join each item to
// its parent, into a Vega spec. The compiler lays the tree out in Go, as
// Mermaid 12.3 does, and the spec holds rows of rules, paths, rects and
// text.
func treeViewChart(c *chart) (map[string]any, bool, error) {
	t := &treeView{
		c: c, root: &tvNode{name: "/", dir: true, level: -1},
		rowIndent: 10, padX: 5, padY: 5, thickness: 1, labelSize: 16,
	}
	if err := t.settings(); err != nil {
		return nil, false, err
	}
	src, lines, err := t.source()
	if err != nil {
		return nil, false, err
	}
	toks, err := t.lex(src, lines)
	if err != nil {
		return nil, false, err
	}
	if err := t.parse(toks); err != nil {
		return nil, false, err
	}
	spec := newTreeViewLayout(t).spec()
	if t.hasTitle && t.title != "" {
		spec["title"] = map[string]any{"text": t.title, "anchor": "start"}
	}
	return spec, false, nil
}

// settings reads the settings of treeView and its theme variables. Each
// other setting is an issue.
func (t *treeView) settings() error {
	c := t.c
	read := []string{
		"treeView.rowIndent", "treeView.paddingX", "treeView.paddingY", "treeView.lineThickness",
		"treeView.showIcons", "treeView.defaultIconPack", "treeView.filenameIcons", "treeView.extensionIcons",
	}
	vars := []string{"labelFontSize", "labelColor", "lineColor", "iconColor", "descriptionColor", "highlightBg", "highlightStroke"}
	for _, v := range vars {
		read = append(read, "themeVariables.treeView."+v)
	}
	if err := c.unreadSettings(read...); err != nil {
		return err
	}
	sec, _ := c.d.Config["treeView"].(map[string]any)
	for _, s := range []struct {
		key string
		v   *float64
	}{{"rowIndent", &t.rowIndent}, {"paddingX", &t.padX}, {"paddingY", &t.padY}, {"lineThickness", &t.thickness}} {
		v, ok := sec[s.key]
		if !ok {
			continue
		}
		n, isNum := v.(float64)
		if !isNum || n < 0 || math.IsInf(n, 0) {
			if err := c.add("the setting treeView.%s %v, which is not a number of 0 or more", s.key, v); err != nil {
				return err
			}
			continue
		}
		*s.v = n
	}
	if v, ok := sec["showIcons"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			if err := c.add("the setting treeView.showIcons %v, which is not true or false", v); err != nil {
				return err
			}
		}
		t.showIcons = b
	}
	if v, ok := sec["defaultIconPack"]; ok {
		s, isText := v.(string)
		if !isText {
			if err := c.add("the setting treeView.defaultIconPack %v, which is not a text", v); err != nil {
				return err
			}
		}
		t.iconPack = s
	}
	var err error
	if t.fileIcons, err = t.iconMap(sec, "filenameIcons"); err != nil {
		return err
	}
	if t.extIcons, err = t.iconMap(sec, "extensionIcons"); err != nil {
		return err
	}
	tv, _ := c.d.Config["themeVariables"].(map[string]any)
	tv, _ = tv["treeView"].(map[string]any)
	if v, ok := tv["labelFontSize"]; ok {
		n, ok := configNumber(tv, "labelFontSize")
		if !ok || n <= 0 {
			if err := c.add("the theme variable treeView.labelFontSize %v, which is not a size", v); err != nil {
				return err
			}
		} else {
			t.labelSize = n
		}
	}
	for _, s := range []struct {
		key string
		v   *string
	}{
		{"labelColor", &t.labelColor}, {"lineColor", &t.lineColor}, {"iconColor", &t.iconColor},
		{"descriptionColor", &t.descColor}, {"highlightBg", &t.highBg}, {"highlightStroke", &t.highStroke},
	} {
		v, ok := tv[s.key]
		if !ok {
			continue
		}
		col, isText := v.(string)
		if !isText {
			if err := c.add("the theme variable treeView.%s %v, which is not a color", s.key, v); err != nil {
				return err
			}
			continue
		}
		*s.v = strings.TrimSpace(col)
	}
	return nil
}

// iconMap reads a map of names to icons of the settings of treeView, such
// as filenameIcons.
func (t *treeView) iconMap(sec map[string]any, key string) (map[string]string, error) {
	v, ok := sec[key]
	if !ok {
		return nil, nil
	}
	m, isMap := v.(map[string]any)
	if !isMap {
		return nil, t.c.add("the setting treeView.%s, which is not a map", key)
	}
	out := map[string]string{}
	for k, v := range m {
		s, isText := v.(string)
		if !isText {
			if err := t.c.add("the icon %v of treeView.%s.%s, which is not a text", v, key, k); err != nil {
				return nil, err
			}
			continue
		}
		out[k] = s
	}
	return out, nil
}

// source returns the source of a tree view as the parser of Mermaid 12.3
// reads it, with the number of each of its lines in the source. Mermaid
// drops the comments on lines of their own first. A tree drawn with the
// characters of boxes, such as ├── and │, then turns into a tree of
// indents.
func (t *treeView) source() (string, []int, error) {
	d := t.c.d
	text := strings.Join(d.body, "\n")
	// Mermaid drops each comment and the blank lines before it. zuzu keeps
	// a new line in their place, so that each line keeps its number.
	text = tvCommentRE.ReplaceAllStringFunc(text, func(m string) string {
		return strings.Repeat("\n", strings.Count(m, "\n"))
	})
	text = strings.TrimLeft(text, " \t")
	lines := strings.Split(text, "\n")
	nums := make([]int, len(lines))
	for i := range lines {
		nums[i] = d.N + i
	}
	if strings.TrimSpace(lines[0]) != tvKeyword {
		return text, nums, nil
	}
	return t.boxes(lines, nums)
}

// boxes turns a tree drawn with the characters of boxes into a tree of
// indents, as Mermaid 12.3 does before the parse. Each ├ or └ at a column
// sets the depth of its item, by the column of the first such character
// after the start of a line. A line of only │ drops. A tree of indents,
// with no character of a box, stays as it is.
func (t *treeView) boxes(lines []string, nums []int) (string, []int, error) {
	content := func(l string) bool {
		s := strings.TrimSpace(l)
		return s != "" && !strings.HasPrefix(s, "%%") && !tvMetaRE.MatchString(l) && !tvDecorRE.MatchString(l)
	}
	boxed, width := false, 0
	for _, l := range lines[1:] {
		if !content(l) {
			continue
		}
		l = strings.ReplaceAll(l, "\t", "    ")
		if !tvBoxRE.MatchString(l) {
			continue
		}
		boxed = true
		if col := tvBranchCol(l); col > 0 && width == 0 {
			width = col
		}
	}
	if !boxed {
		return strings.Join(lines, "\n"), nums, nil
	}
	if width == 0 {
		width = 4
	}
	out, outNums := []string{lines[0]}, []int{nums[0]}
	for i, l := range lines[1:] {
		n := nums[i+1]
		s := strings.TrimSpace(l)
		norm := strings.ReplaceAll(l, "\t", "    ")
		switch {
		case s == "", strings.HasPrefix(s, "%%"), tvMetaRE.MatchString(l):
		case tvDecorRE.MatchString(l):
			continue
		case tvBranchCol(norm) >= 0:
			col := tvBranchCol(norm)
			depth := int(math.Floor(float64(col)/float64(width)+0.5)) + 1
			rest := []rune(norm)
			p := 0
			for k := 0; p < len(rest); p++ {
				if k == col {
					break
				}
				k += utf16Len(rest[p])
			}
			p++
			for p < len(rest) && (rest[p] == '─' || rest[p] == '━') {
				p++
			}
			for p < len(rest) && rest[p] == ' ' {
				p++
			}
			item := strings.TrimRight(string(rest[p:]), " \t\n\r\v\f")
			if item == "" {
				t.c.line = n
				return "", nil, t.c.errorf("a branch of a box with no name after it")
			}
			l = strings.Repeat("    ", depth) + item
		case tvBoxOnlyRE.MatchString(norm):
			continue
		case tvBoxRE.MatchString(norm):
		case strings.TrimLeft(norm, " \t\n\r\v\f") != norm:
			t.c.line = n
			return "", nil, t.c.errorf("an indent with no ├── or └── in a tree drawn with boxes")
		}
		out = append(out, l)
		outNums = append(outNums, n)
	}
	return strings.Join(out, "\n"), outNums, nil
}

// tvBranchCol returns the column of the first ├, ┣, └ or ┗ of a line, in
// the units of UTF-16 that Mermaid counts, or -1.
func tvBranchCol(l string) int {
	col := 0
	for _, r := range l {
		switch r {
		case '├', '┣', '└', '┗':
			return col
		}
		col += utf16Len(r)
	}
	return -1
}

// utf16Len returns the number of units of UTF-16 of a rune.
func utf16Len(r rune) int {
	if r > 0xFFFF {
		return 2
	}
	return 1
}

// lex splits the source of a tree view into tokens, as the lexer of
// Mermaid 12.3 does: at each place, the first kind of token that matches
// wins. A comment and a new line drop. A character that starts no token is
// an error, as in Mermaid.
func (t *treeView) lex(src string, nums []int) ([]tvTok, error) {
	var toks []tvTok
	line := 0
	for i := 0; i < len(src); {
		kind, n := tvMatch(src, i)
		if n == 0 {
			t.c.line = nums[line]
			r, _ := utf8.DecodeRuneInString(src[i:])
			return nil, t.c.errorf("the character %q, which starts no part of a tree view", r)
		}
		if kind != tvComment && kind != tvNewline {
			toks = append(toks, tvTok{kind: kind, text: src[i : i+n], line: nums[line]})
		}
		line += strings.Count(src[i:i+n], "\n")
		i += n
	}
	return toks, nil
}

// tvMatch returns the kind of the token at a place of a source and its
// length, or a length of 0.
func tvMatch(s string, i int) (tvToken, int) {
	if n := tvMatchKeyword(s, i); n > 0 {
		// A name that is longer than the keyword wins over it.
		if b := tvMatchBare(s, i); b > n {
			return tvBare, b
		}
		return tvKey, n
	}
	for _, m := range []struct {
		kind  tvToken
		match func(string, int) int
	}{
		{tvAccDescr, tvMatchAccDescr}, {tvAccTitle, tvMatchAccTitle}, {tvTitle, tvMatchTitle},
		{tvClass, tvMatchClass}, {tvIcon, tvMatchIcon}, {tvDesc, tvMatchDesc}, {tvIndent, tvMatchIndent},
		{tvQuoted, tvMatchQuoted}, {tvComment, tvMatchComment}, {tvNewline, tvMatchNewline}, {tvBare, tvMatchBare},
	} {
		if n := m.match(s, i); n > 0 {
			return m.kind, n
		}
	}
	return 0, 0
}

// tvSpaces returns the end of the run of spaces and tabs at a place.
func tvSpaces(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// tvLineEnd returns the place of the end of the line at a place.
func tvLineEnd(s string, i int) int {
	if k := strings.IndexAny(s[i:], "\n\r"); k >= 0 {
		return i + k
	}
	return len(s)
}

// tvLetter reports whether a byte is a letter or _, which starts the name
// of a class.
func tvLetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

// tvWord reports whether a byte is a letter, a digit, _ or -.
func tvWord(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-'
}

// tvMatchKeyword matches the keyword treeView-beta, before a space, the
// end, or %%.
func tvMatchKeyword(s string, i int) int {
	if !strings.HasPrefix(s[i:], tvKeyword) {
		return 0
	}
	j := i + len(tvKeyword)
	if j == len(s) || strings.HasPrefix(s[j:], "%%") || strings.ContainsRune(" \t\n\r\v\f", rune(s[j])) {
		return len(tvKeyword)
	}
	return 0
}

// tvRestOfLine returns the end of a text of a title or an accTitle that
// starts at a place: before the first %% of the line, or at its end.
func tvRestOfLine(s string, i int) int {
	end := tvLineEnd(s, i)
	if k := strings.Index(s[i:end], "%%"); k >= 0 {
		return i + k
	}
	return end
}

// tvMatchAccDescr matches accDescr: a text up to %% or the end of the
// line, or a block in braces.
func tvMatchAccDescr(s string, i int) int {
	j := tvSpaces(s, i)
	if !strings.HasPrefix(s[j:], "accDescr") {
		return 0
	}
	j += len("accDescr")
	if k := tvSpaces(s, j); k < len(s) && s[k] == ':' {
		return tvRestOfLine(s, k+1) - i
	}
	k := j
	for k < len(s) && strings.IndexByte(" \t\n\r\v\f", s[k]) >= 0 {
		k++
	}
	if k < len(s) && s[k] == '{' {
		if e := strings.IndexByte(s[k:], '}'); e >= 0 {
			return k + e + 1 - i
		}
	}
	return 0
}

// tvMatchAccTitle matches accTitle: a text up to %% or the end of the
// line.
func tvMatchAccTitle(s string, i int) int {
	j := tvSpaces(s, i)
	if !strings.HasPrefix(s[j:], "accTitle") {
		return 0
	}
	j = tvSpaces(s, j+len("accTitle"))
	if j >= len(s) || s[j] != ':' {
		return 0
	}
	return tvRestOfLine(s, j+1) - i
}

// tvMatchTitle matches the word title and the text after a space, up to
// %% or the end of the line. The word alone matches too, so that the
// title of titles.txt is "title", as in Mermaid.
func tvMatchTitle(s string, i int) int {
	j := tvSpaces(s, i)
	if !strings.HasPrefix(s[j:], "title") {
		return 0
	}
	j += len("title")
	if j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		return tvRestOfLine(s, j+1) - i
	}
	return j - i
}

// tvMatchClass matches :::class after a space.
func tvMatchClass(s string, i int) int {
	j := tvSpaces(s, i)
	if j == i || !strings.HasPrefix(s[j:], ":::") {
		return 0
	}
	j = tvSpaces(s, j+3)
	if j >= len(s) || !tvLetter(s[j]) {
		return 0
	}
	for j < len(s) && tvWord(s[j]) {
		j++
	}
	return j - i
}

// tvMatchIcon matches icon(name) or icon(pack:name) after a space.
func tvMatchIcon(s string, i int) int {
	j := tvSpaces(s, i)
	if j == i || !strings.HasPrefix(s[j:], "icon(") {
		return 0
	}
	j += len("icon(")
	for j < len(s) && tvWord(s[j]) {
		j++
	}
	if j < len(s) && s[j] == ':' {
		k := j + 1
		for k < len(s) && tvWord(s[k]) {
			k++
		}
		if k > j+1 {
			j = k
		}
	}
	if j >= len(s) || s[j] != ')' {
		return 0
	}
	return j + 1 - i
}

// tvMatchDesc matches ## and a description to the end of the line, after
// a space.
func tvMatchDesc(s string, i int) int {
	j := tvSpaces(s, i)
	if j == i || !strings.HasPrefix(s[j:], "##") {
		return 0
	}
	return tvLineEnd(s, j) - i
}

// tvMatchIndent matches a run of spaces and tabs.
func tvMatchIndent(s string, i int) int {
	return tvSpaces(s, i) - i
}

// tvMatchQuoted matches a name in double or single quotes, which can hold
// a new line.
func tvMatchQuoted(s string, i int) int {
	if s[i] != '"' && s[i] != '\'' {
		return 0
	}
	if k := strings.IndexByte(s[i+1:], s[i]); k >= 0 {
		return k + 2
	}
	return 0
}

// tvMatchComment matches a comment from %% to the end of the line.
func tvMatchComment(s string, i int) int {
	if !strings.HasPrefix(s[i:], "%%") {
		return 0
	}
	if k := strings.IndexByte(s[i:], '\n'); k >= 0 {
		return k
	}
	return len(s) - i
}

// tvMatchNewline matches a new line.
func tvMatchNewline(s string, i int) int {
	switch {
	case strings.HasPrefix(s[i:], "\r\n"):
		return 2
	case s[i] == '\n':
		return 1
	}
	return 0
}

// tvAnnotationAt reports whether an annotation starts at a place: a run
// of spaces, then :::class, icon( or ##.
func tvAnnotationAt(s string, i int) bool {
	j := tvSpaces(s, i)
	if j == i {
		return false
	}
	r := s[j:]
	if strings.HasPrefix(r, "icon(") || strings.HasPrefix(r, "##") {
		return true
	}
	if !strings.HasPrefix(r, ":::") {
		return false
	}
	k := tvSpaces(s, j+3)
	return k < len(s) && tvLetter(s[k])
}

// tvMatchBare matches a name with no quotes: the rest of the line up to
// an annotation. It cannot start with a space, a quote, :::, icon( or ##.
func tvMatchBare(s string, i int) int {
	r := s[i:]
	if strings.HasPrefix(r, ":::") || strings.HasPrefix(r, "icon(") || strings.HasPrefix(r, "##") {
		return 0
	}
	if strings.IndexByte(" \t\n\r\"'", s[i]) >= 0 {
		return 0
	}
	j := i + 1
	for j < len(s) && s[j] != '\n' && s[j] != '\r' && !tvAnnotationAt(s, j) {
		j++
	}
	return j - i
}

// tvCommonText returns the text of a title, an accTitle or an accDescr on
// one line, as Mermaid reads it: trimmed, with each run of spaces as one.
func tvCommonText(s string) string {
	return tvSpacesRE.ReplaceAllString(strings.TrimSpace(s), " ")
}

// parse reads the tokens of a tree view, as the grammar of Mermaid 12.3
// does: the keyword, then titles, accTitles and accDescrs, then items.
// An item is an indent, a name, and annotations in any order. Each other
// order of the tokens is an error, as in Mermaid.
func (t *treeView) parse(toks []tvTok) error {
	c := t.c
	if len(toks) == 0 || toks[0].kind != tvKey {
		if len(toks) > 0 {
			c.line = toks[0].line
		}
		// The keyword reads as part of a longer name when text follows it
		// on its line, even a space.
		return c.errorf("text after the keyword %s on its line", tvKeyword)
	}
	i := 1
	var accTitle, accDescr string
	var accOrder []*string
	for ; i < len(toks); i++ {
		tok := toks[i]
		switch tok.kind {
		case tvTitle:
			_, rest, _ := strings.Cut(tok.text, "title")
			t.title, t.hasTitle = tvCommonText(rest), true
			continue
		case tvAccTitle:
			_, rest, _ := strings.Cut(tok.text, ":")
			accTitle = tvCommonText(rest)
			accOrder = appendOnce(accOrder, &accTitle)
			continue
		case tvAccDescr:
			accDescr = tvAccDescrText(tok.text)
			accOrder = appendOnce(accOrder, &accDescr)
			continue
		}
		break
	}
	for _, p := range accOrder {
		c.acc = append(c.acc, *p)
	}
	stack := []*tvNode{t.root}
	for i < len(toks) {
		level := 0
		if toks[i].kind == tvIndent {
			level = len(toks[i].text)
			i++
		}
		if i >= len(toks) {
			c.line = toks[i-1].line
			return c.errorf("an indent with no name after it at the end of the source")
		}
		tok := toks[i]
		c.line = tok.line
		var name string
		switch tok.kind {
		case tvQuoted:
			name = tok.text[1 : len(tok.text)-1]
		case tvBare:
			name = strings.TrimRight(tok.text, " \t")
		case tvIndent:
			return c.errorf("spaces after a name or an annotation, or a line of only spaces")
		default:
			return c.errorf("the text %q where the name of an item goes", strings.TrimSpace(tok.text))
		}
		n := &tvNode{name: name, level: level}
		n.name, n.dir = strings.CutSuffix(name, "/")
		for i++; i < len(toks); i++ {
			text := strings.TrimSpace(toks[i].text)
			switch toks[i].kind {
			case tvClass:
				n.class = strings.TrimSpace(text[3:])
				continue
			case tvIcon:
				n.icon = text[5 : len(text)-1]
				if n.icon == "" {
					n.icon = "none"
				}
				continue
			case tvDesc:
				n.desc = strings.TrimSpace(text[2:])
				continue
			}
			break
		}
		for level <= stack[len(stack)-1].level {
			stack = stack[:len(stack)-1]
		}
		top := stack[len(stack)-1]
		top.children = append(top.children, n)
		stack = append(stack, n)
	}
	return nil
}

// appendOnce appends a pointer to a list that does not hold it.
func appendOnce(list []*string, p *string) []*string {
	if slices.Contains(list, p) {
		return list
	}
	return append(list, p)
}

// tvAccDescrText returns the text of an accDescr, as Mermaid reads it: a
// text on one line, or the lines of a block in braces, each trimmed.
func tvAccDescrText(tok string) string {
	j := strings.Index(tok, "accDescr") + len("accDescr")
	rest := tok[j:]
	if k := tvSpaces(rest, 0); k < len(rest) && rest[k] == ':' {
		return tvCommonText(rest[k+1:])
	}
	open := strings.IndexByte(rest, '{')
	body := rest[open+1 : len(rest)-1]
	lines := strings.Split(body, "\n")
	var out []string
	for _, l := range lines {
		l = tvSpacesRE.ReplaceAllString(strings.TrimSpace(l), " ")
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// iconOf returns the icon of an item, as Mermaid 12.3 picks it: the icon
// of icon(name), or with treeView.showIcons the icon of its name or of
// the extension of its name, or the icon of a file or of a directory. It
// is empty for no icon. A name with no pack takes the pack
// mermaid-treeview, which holds the icons folder and file, or the pack of
// treeView.defaultIconPack.
func (t *treeView) iconOf(n *tvNode) string {
	if n.icon == "none" {
		return ""
	}
	if n.icon != "" {
		return t.qualify(n.icon)
	}
	if !t.showIcons {
		return ""
	}
	if !n.dir {
		ic := t.fileIcons[n.name]
		if ic == "" {
			if dot := strings.LastIndexByte(n.name, '.'); dot > 0 {
				ext := strings.ToLower(n.name[dot:])
				var ok bool
				if ic, ok = t.extIcons[ext]; !ok {
					ic = t.extIcons[ext[1:]]
				}
			}
		}
		if ic == "none" {
			return ""
		}
		if ic != "" {
			return t.qualify(ic)
		}
		return tvIconPack + ":file"
	}
	return tvIconPack + ":folder"
}

// tvIconPack is the name of the pack of the icons of a tree view in
// Mermaid, which holds folder and file.
const tvIconPack = "mermaid-treeview"

// qualify returns the name of an icon with its pack.
func (t *treeView) qualify(ic string) string {
	if strings.Contains(ic, ":") {
		return ic
	}
	if ic == "folder" || ic == "file" || t.iconPack == "" {
		return tvIconPack + ":" + ic
	}
	return t.iconPack + ":" + ic
}
