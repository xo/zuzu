package mermaid

import (
	"regexp"
)

// ishNode is a node of an ishikawa diagram: the problem at the head, a
// cause, or a cause of a cause. text is the text as the source gives it.
type ishNode struct {
	text string
	kids []*ishNode
}

// ishikawa is an ishikawa diagram as the compiler reads it.
type ishikawa struct {
	c *chart
	// root is the problem, the first statement. It is nil when the
	// diagram has no statement.
	root *ishNode
	// fontSize is the setting fontSize, 16 by default. Mermaid wraps the
	// problem and sets the least length of a bone by it.
	fontSize float64
	// padding is ishikawa.diagramPadding, 20 by default.
	padding float64
}

// ishKeywordRE matches the keyword ishikawa, which the lexer of Mermaid 12.3
// reads in any case at the start of a statement.
var ishKeywordRE = regexp.MustCompile(`(?i)^ishikawa\b`)

// ishikawaChart compiles an ishikawa diagram: a problem at the head of a
// spine, with its causes as bones off the spine and the causes of each
// cause as smaller bones, into a Vega spec (D31). The compiler lays the
// diagram out in Go, as Mermaid 12.3 does, and the spec holds rows of
// rules, symbols, paths, rects and text.
func ishikawaChart(c *chart) (map[string]any, bool, error) {
	k := &ishikawa{c: c, fontSize: 16, padding: 20}
	if err := k.settings(); err != nil {
		return nil, false, err
	}
	if err := k.parse(); err != nil {
		return nil, false, err
	}
	if k.root == nil {
		// Mermaid draws nothing for a diagram with no statement.
		return emptyChart("", ""), false, nil
	}
	return newIshLayout(k).spec(), false, nil
}

// settings reads the config of an ishikawa diagram. zuzu reads fontSize
// and ishikawa.diagramPadding, and each other setting is an issue.
func (k *ishikawa) settings() error {
	c := k.c
	if err := c.unreadSettings("fontSize", "ishikawa.diagramPadding"); err != nil {
		return err
	}
	if _, ok := c.d.Config["fontSize"]; ok {
		v, ok := configNumber(c.d.Config, "fontSize")
		if !ok || v <= 0 {
			if err := c.add("the setting fontSize %v, which is not a positive number", c.d.Config["fontSize"]); err != nil {
				return err
			}
		} else {
			k.fontSize = v
		}
	}
	sec, _ := c.d.Config["ishikawa"].(map[string]any)
	if _, ok := sec["diagramPadding"]; ok {
		v, ok := configNumber(sec, "diagramPadding")
		if !ok || v < 0 {
			return c.add("the setting ishikawa.diagramPadding %v, which is not a number of 0 or more", sec["diagramPadding"])
		}
		k.padding = v
	}
	return nil
}

// parse reads the statements of an ishikawa diagram, as the parser of
// Mermaid 12.3 reads them. Each statement is a line of text. The first is
// the problem. The indent of the second is the indent of a cause, and each
// statement belongs to the last statement before it with a smaller indent.
// A statement with an indent smaller than that of a cause is a cause too.
func (k *ishikawa) parse() error {
	c := k.c
	type entry struct {
		level int
		node  *ishNode
	}
	var stack []entry
	base, hasBase := 0, false
	add := func(indent int, s string) error {
		if ishKeywordRE.MatchString(s) {
			return c.errorf("the keyword ishikawa after the first line")
		}
		n := &ishNode{text: s}
		if k.root == nil {
			k.root = n
			stack = []entry{{0, n}}
			return nil
		}
		if !hasBase {
			base, hasBase = indent, true
		}
		level := max(indent-base+1, 1)
		for len(stack) > 1 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		p := stack[len(stack)-1].node
		p.kids = append(p.kids, n)
		stack = append(stack, entry{level, n})
		return nil
	}
	// A text after the keyword is the first statement, as in Mermaid.
	if c.d.Header != "" {
		c.line = c.d.N
		if err := add(1, c.d.Header); err != nil {
			return err
		}
	}
	for _, l := range c.d.Lines {
		c.line = l.N
		if err := add(l.Indent, l.Text); err != nil {
			return err
		}
	}
	c.line = 0
	return nil
}
