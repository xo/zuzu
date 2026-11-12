package mermaid

import (
	"regexp"
	"strings"

	"github.com/xo/zuzu/scene"
)

// kbNode is a node of a kanban as the source gives it: a column or a card.
type kbNode struct {
	label, ticket, assigned, priority string
	indent, line                      int
	column                            bool
}

// kbColumn is a column of a kanban with its cards.
type kbColumn struct {
	*kbNode

	cards []*kbNode
}

// kanban is a kanban as the compiler reads it.
type kanban struct {
	c *chart
	// width is kanban.sectionWidth, the width of a column in the pixels of
	// Mermaid, and base is kanban.ticketBaseUrl.
	width float64
	base  string

	columns []*kbColumn
	nodes   []*kbNode
	// shallow is the label of the last node with less indent than the
	// first node. The next node is an error, as in Mermaid.
	shallow    string
	hasShallow bool
}

// The tokens of a kanban that Mermaid 12.3 reads.
var (
	// kbKeywordRE matches the keyword kanban, in any case, which only the
	// first line can hold.
	kbKeywordRE = regexp.MustCompile(`(?i)^kanban\b`)
	// kbIconRE matches an icon after a node, ::icon(fa fa-book).
	kbIconRE = regexp.MustCompile(`^::icon\(([^)]+)\)$`)
)

// The tokens that start and end the text of a node, in the order in which
// the lexer of Mermaid 12.3 tries them.
var (
	kbStarts = []string{"-)", "(-", "))", ")", "((", "{{", "(", "["}
	kbEnds   = []string{"))", ")", "]", "}}", "(-", "-)", "((", "("}
)

// kbPriorities are the colors of the line that Mermaid draws at the left of
// a card by its priority. Mermaid draws no line for Medium or for another
// value.
var kbPriorities = map[string]string{
	"Very High": "red",
	"High":      "orange",
	"Low":       "blue",
	"Very Low":  "lightblue",
}

// kanbanChart compiles a kanban: its columns from left to right, each with
// its cards from top to bottom, into a Vega spec (D31). The compiler lays
// the board out in Go, with the sizes of Mermaid 12.3.
func kanbanChart(c *chart) (map[string]any, bool, error) {
	k := &kanban{c: c, width: 200}
	if err := k.settings(); err != nil {
		return nil, false, err
	}
	if err := k.parse(); err != nil {
		return nil, false, err
	}
	if len(k.columns) == 0 {
		// Mermaid draws nothing for a kanban with no node.
		return emptyChart("", ""), false, nil
	}
	l := newKanbanLayout(k)
	if l.err != nil {
		return nil, false, l.err
	}
	return l.spec(), false, nil
}

// settings reads the config of a kanban: kanban.sectionWidth and
// kanban.ticketBaseUrl. Mermaid 12.3 reads kanban.padding and
// mindmap.maxNodeWidth and draws nothing for them, so they are no issue
// (D32). Each other setting is an issue.
func (k *kanban) settings() error {
	c := k.c
	if err := c.unreadSettings("kanban.sectionWidth", "kanban.ticketBaseUrl", "kanban.padding", "mindmap.maxNodeWidth"); err != nil {
		return err
	}
	sec, _ := c.d.Config["kanban"].(map[string]any)
	if v, ok := sec["sectionWidth"]; ok {
		n, isNum := configNumber(sec, "sectionWidth")
		if !isNum || n <= 0 {
			if err := c.add("the setting kanban.sectionWidth %v, which is not a positive number", v); err != nil {
				return err
			}
		} else {
			k.width = n
		}
	}
	if v, ok := sec["ticketBaseUrl"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return c.add("the setting kanban.ticketBaseUrl %v, which is not a text", v)
		}
		k.base = s
	}
	return nil
}

// parse reads the nodes of a kanban, one on each line, as the grammar of
// Mermaid 12.3 reads them. The first node and each node with its indent
// are columns. Each other node is a card of the last column.
func (k *kanban) parse() error {
	c := k.c
	var lines []Line
	// A node can follow the keyword on its line, as in Mermaid.
	if h := strings.TrimSpace(c.d.Header); h != "" {
		lines = append(lines, Line{Text: h, N: c.d.N, Indent: 1})
	}
	lines = append(lines, c.d.Lines...)
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		c.line = l.N
		s := l.Text
		for {
			done, err := k.statement(s, l)
			if err != nil {
				return err
			}
			if done {
				break
			}
			// The text of a node, a string or the data of a node runs over
			// more lines, up to its end.
			if i+1 >= len(lines) {
				return c.errorf("a node with no end in %q", s)
			}
			i++
			s += "\n" + lines[i].Text
		}
	}
	c.line = 0
	return nil
}

// statement reads a statement of a kanban: a node, with its data in @{
// and }, an icon or a class. It reports false when the statement goes on
// in the next line.
func (k *kanban) statement(s string, l Line) (bool, error) {
	c := k.c
	switch {
	case strings.HasPrefix(s, ":::"):
		// The classes of a kanban are classes of CSS that the page of a
		// site gives. Mermaid draws nothing for them by itself, so zuzu
		// reads them with no issue (D32).
		if len(k.nodes) == 0 {
			return true, c.errorf("a class before the first node")
		}
		return true, nil
	case strings.HasPrefix(s, "::icon("):
		if !strings.Contains(s, ")") {
			return false, nil
		}
		if !kbIconRE.MatchString(s) {
			return true, c.errorf("the icon %q", s)
		}
		// Mermaid keeps the icon of a card and draws nothing for it, so
		// zuzu reads it with no issue (D32).
		if len(k.nodes) == 0 {
			return true, c.errorf("an icon before the first node")
		}
		return true, nil
	case kbKeywordRE.MatchString(s):
		return true, c.errorf("the keyword kanban after the first line")
	}
	n := &kbNode{indent: l.Indent, line: l.N}
	rest := s
	open := kbPrefix(rest, kbStarts)
	if open == "" {
		// The id of a node runs up to the start of its text or of its
		// data.
		end := strings.IndexAny(rest, "([\n){}@")
		if end < 0 {
			end = len(rest)
		}
		if end == 0 {
			return true, c.errorf("the node %q", s)
		}
		n.label, rest = strings.TrimSpace(rest[:end]), rest[end:]
		open = kbPrefix(rest, kbStarts)
	}
	if open != "" {
		label, after, done, err := k.text(rest[len(open):], s)
		if !done || err != nil {
			return done, err
		}
		n.label, rest = label, after
	}
	if strings.HasPrefix(rest, "@{") {
		end := kbDataEnd(rest)
		if end < 0 {
			return false, nil
		}
		if strings.Contains(rest[:end], "^") {
			return true, c.errorf("a ^ in the data of the node %q", s)
		}
		if err := k.data(n, rest[2:end]); err != nil {
			return true, err
		}
		rest = rest[end+1:]
	}
	if strings.TrimSpace(rest) != "" {
		return true, c.errorf("the text %q after a node", strings.TrimSpace(rest))
	}
	n.label = kbLabel(n.label)
	return true, k.add(n)
}

// text reads the text of a node after the token that starts it, and the
// token that ends it: a string in quotes, a string of markdown in quotes
// and backticks, or a text up to the end. It returns the text, the rest of
// the statement, and false when the text goes on in the next line.
func (k *kanban) text(rest, s string) (string, string, bool, error) {
	c := k.c
	var body string
	switch {
	case strings.HasPrefix(rest, "\"`"):
		j := strings.Index(rest[2:], "`\"")
		if j < 0 {
			if strings.Contains(rest[2:], `"`) {
				return "", "", true, c.errorf("a quote in the string of markdown of %q", s)
			}
			return "", "", false, nil
		}
		if strings.Contains(rest[2:2+j], `"`) {
			return "", "", true, c.errorf("a quote in the string of markdown of %q", s)
		}
		body, rest = rest[2:2+j], rest[2+j+2:]
	case strings.HasPrefix(rest, `"`):
		j := strings.Index(rest[1:], `"`)
		if j < 0 {
			return "", "", false, nil
		}
		body, rest = rest[1:1+j], rest[1+j+1:]
	default:
		if kbPrefix(rest, kbEnds) != "" {
			return "", "", true, c.errorf("a node with no text in %q", s)
		}
		j := strings.IndexAny(rest, ")](}")
		if j < 0 {
			return "", "", false, nil
		}
		if j == 0 {
			return "", "", true, c.errorf("the text of the node %q", s)
		}
		body, rest = rest[:j], rest[j:]
	}
	end := kbPrefix(rest, kbEnds)
	if end == "" {
		return "", "", true, c.errorf("the end of the node %q", s)
	}
	return body, rest[len(end):], true, nil
}

// data reads the data of a node in @{ and }, a map of YAML: label,
// assigned, ticket and priority draw. Mermaid keeps icon and each other
// key and draws nothing for them, so zuzu reads them with no issue (D32).
// A shape with a capital letter or an underscore is an error, as in
// Mermaid, and Mermaid draws each other shape as a card.
func (k *kanban) data(n *kbNode, body string) error {
	m := agentMap(body)
	if s := m["shape"]; s != strings.ToLower(s) || strings.Contains(s, "_") {
		return k.c.errorf("the shape %s, which is not in lower case", s)
	}
	if v := m["label"]; v != "" {
		n.label = v
	}
	if v := m["assigned"]; v != "" {
		n.assigned = labelText(v)
	}
	if v := m["ticket"]; v != "" {
		n.ticket = labelText(v)
	}
	n.priority = m["priority"]
	return nil
}

// add adds a node to the kanban. The first node and each node with the
// indent of the first are columns, and each other node is a card of the
// last column. A node after a node with less indent than the first is an
// error, as in Mermaid.
func (k *kanban) add(n *kbNode) error {
	if k.hasShallow {
		return k.c.errorf("a card with no column: the node %q has less indent than the first node", k.shallow)
	}
	k.nodes = append(k.nodes, n)
	if len(k.columns) == 0 || n.indent == k.columns[0].indent {
		n.column = true
		k.columns = append(k.columns, &kbColumn{kbNode: n})
		return nil
	}
	if n.indent < k.columns[0].indent {
		k.shallow, k.hasShallow = n.label, true
	}
	col := k.columns[len(k.columns)-1]
	col.cards = append(col.cards, n)
	return nil
}

// link returns the link of the ticket of a card: kanban.ticketBaseUrl with
// the ticket in place of #TICKET#. A card with no ticket, or a kanban with
// no ticketBaseUrl, has no link. A link whose scheme zuzu does not write is
// an issue (D32).
func (k *kanban) link(n *kbNode) (string, error) {
	if n.ticket == "" || k.base == "" {
		return "", nil
	}
	href := strings.Replace(k.base, "#TICKET#", n.ticket, 1)
	if !scene.LinkAllowed(href) {
		k.c.line = n.line
		defer func() { k.c.line = 0 }()
		return "", k.c.add("the link %s of the ticket %s, whose scheme zuzu does not write", href, n.ticket)
	}
	return href, nil
}

// kbPrefix returns the first token of a list that starts s, or "".
func kbPrefix(s string, tokens []string) string {
	for _, t := range tokens {
		if strings.HasPrefix(s, t) {
			return t
		}
	}
	return ""
}

// kbDataEnd returns the index of the brace that ends the data of a node
// that starts with @{, or -1. A brace in double quotes does not end it.
func kbDataEnd(s string) int {
	quoted := false
	for i := 2; i < len(s); i++ {
		switch s[i] {
		case '"':
			quoted = !quoted
		case '}':
			if !quoted {
				return i
			}
		}
	}
	return -1
}

// kbLabel returns the text of a label of a kanban: br as a new line, the
// entities read, and each line of a text over more lines trimmed, as in
// Mermaid.
func kbLabel(s string) string {
	parts := strings.Split(labelText(s), "\n")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, "\n")
}
