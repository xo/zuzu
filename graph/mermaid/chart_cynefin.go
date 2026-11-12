package mermaid

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// The domains of a Cynefin diagram, in the order in which Mermaid 12.3
// draws their items.
const (
	cyComplex     = "complex"
	cyComplicated = "complicated"
	cyChaotic     = "chaotic"
	cyClear       = "clear"
	cyConfusion   = "confusion"
)

// cyDomains are the domains of a Cynefin diagram in the order in which
// Mermaid draws their items.
var cyDomains = []string{cyComplex, cyComplicated, cyChaotic, cyClear, cyConfusion}

// cyTransition is a transition of a Cynefin diagram: an arrow from one
// domain to another, with a label or none.
type cyTransition struct {
	from, to, label string
}

// cynefin is a Cynefin diagram as the compiler reads it.
type cynefin struct {
	c *chart
	// items are the labels of the items of each domain. A domain that the
	// source names again takes the items of its last block, as in Mermaid.
	items       map[string][]string
	transitions []cyTransition
	// title is the title of the body, which wins over the title of the
	// front matter when hasTitle is true.
	title    string
	hasTitle bool
	cfg      cyConfig
}

// cyConfig holds the settings of a Cynefin diagram, with the defaults of
// Mermaid 12.3.
type cyConfig struct {
	width, height float64
	// padding is the space around the diagram. zuzu draws its own margin
	// unless the source sets cynefin.padding.
	padding    float64
	hasPadding bool
	// descriptions is cynefin.showDomainDescriptions: the model and the
	// practice under the name of each domain.
	descriptions bool
	// amplitude is cynefin.boundaryAmplitude, how far the boundaries
	// between the domains wave.
	amplitude float64
	// seed is cynefin.seed, which seeds the waves of the boundaries.
	seed float64
	// colors are the theme variables of themeVariables.cynefin that the
	// source gives, by name, such as complexBg.
	colors map[string]string
}

// cyColorVars are the theme variables of themeVariables.cynefin that zuzu
// reads: the color of each domain, of the boundaries, of the boundary
// between clear and chaotic, of the arrows and of the texts. Each other
// variable of the section, such as a size, is an issue.
var cyColorVars = []string{
	"complexBg", "complicatedBg", "clearBg", "chaoticBg", "confusionBg",
	"boundaryColor", "cliffColor", "arrowColor", "textColor", "labelColor",
}

// cyTitleRE matches a title of a Cynefin diagram: the word title, alone or
// with a space and a text.
var cyTitleRE = regexp.MustCompile(`^title(?:[\t ]|$)`)

// cyToken is a token of the body of a Cynefin diagram.
type cyToken struct {
	// kind is a domain, a string, the arrow -->, the colon or the end of
	// a line.
	kind cyKind
	text string
	line int
}

// cyKind is the kind of a token of a Cynefin diagram.
type cyKind int

// The kinds of the tokens of a Cynefin diagram.
const (
	cyDomain cyKind = iota
	cyString
	cyArrow
	cyColon
	cyEnd
)

// cynefinChart compiles a Cynefin diagram: the four domains clear,
// complicated, complex and chaotic around a center of confusion, the items
// of each domain, and the transitions between them, into a Vega spec
// (D31). The compiler lays the diagram out in Go, as Mermaid 12.3 lays it
// out, and the spec holds rows of rects, paths and text.
func cynefinChart(c *chart) (map[string]any, bool, error) {
	y := &cynefin{c: c, items: map[string][]string{}}
	if err := y.settings(); err != nil {
		return nil, false, err
	}
	if err := y.parse(); err != nil {
		return nil, false, err
	}
	if y.hasTitle {
		// The title of the body wins over the title of the front matter,
		// and an empty title draws none, as in Mermaid.
		c.d.Title = y.title
	}
	return newCynefinLayout(y).spec(), false, nil
}

// settings reads the config of a Cynefin diagram: cynefin.width,
// cynefin.height, cynefin.padding, cynefin.showDomainDescriptions,
// cynefin.boundaryAmplitude and cynefin.seed, and the colors of
// themeVariables.cynefin. Each other setting is an issue.
func (y *cynefin) settings() error {
	c := y.c
	y.cfg = cyConfig{width: 800, height: 600, descriptions: true, amplitude: 8, colors: map[string]string{}}
	read := []string{"cynefin.width", "cynefin.height", "cynefin.padding", "cynefin.showDomainDescriptions", "cynefin.boundaryAmplitude", "cynefin.seed"}
	for _, k := range cyColorVars {
		read = append(read, "themeVariables.cynefin."+k)
	}
	if err := c.unreadSettings(read...); err != nil {
		return err
	}
	line := c.line
	c.line = 0
	defer func() { c.line = line }()
	sec, _ := c.d.Config["cynefin"].(map[string]any)
	for _, k := range slices.Sorted(maps.Keys(sec)) {
		v := sec[k]
		var err error
		switch k {
		case "width", "height", "padding", "boundaryAmplitude", "seed":
			n, ok := configNumber(sec, k)
			switch {
			case !ok, k != "seed" && n < 0, (k == "width" || k == "height") && n == 0:
				err = c.add("the setting cynefin.%s: %v", k, v)
			case k == "width":
				y.cfg.width = n
			case k == "height":
				y.cfg.height = n
			case k == "padding":
				y.cfg.padding, y.cfg.hasPadding = n, true
			case k == "boundaryAmplitude":
				y.cfg.amplitude = n
			default:
				y.cfg.seed = n
			}
		case "showDomainDescriptions":
			b, ok := v.(bool)
			if !ok {
				err = c.add("the setting cynefin.showDomainDescriptions: %v", v)
			}
			y.cfg.descriptions = b || !ok
		}
		if err != nil {
			return err
		}
	}
	for _, k := range cyColorVars {
		if s := str(c.d.Config, "themeVariables", "cynefin", k); s != "" {
			y.cfg.colors[k] = s
		}
	}
	return nil
}

// parse reads the body of a Cynefin diagram as the grammar of Mermaid 12.3
// reads it. A statement is a title, an accTitle, an accDescr, a domain
// and the strings of its items, or a transition: a domain, -->, a domain,
// and a colon and a string for its label, up to the end of the line.
func (y *cynefin) parse() error {
	c := y.c
	lines := make([]Line, 0, len(c.d.Lines)+1)
	if head := strings.TrimSpace(c.d.Header); head != "" {
		// The text after the keyword holds statements, as in Mermaid.
		lines = append(lines, Line{Text: head, N: c.d.N})
	}
	lines = append(lines, c.d.Lines...)
	var toks []cyToken
	// flush reads the tokens of the lines before a title or an
	// accessibility statement, which ends a block of items.
	flush := func() error {
		err := y.statements(toks)
		toks = toks[:0]
		return err
	}
	for i := 0; i < len(lines); i++ {
		c.line = lines[i].N
		if n, ok, err := c.accessibility(lines, i); ok || err != nil {
			if err != nil {
				return err
			}
			if err := flush(); err != nil {
				return err
			}
			c.line = lines[i].N
			i = n
			continue
		}
		if s := lines[i].Text; cyTitleRE.MatchString(s) {
			if err := flush(); err != nil {
				return err
			}
			c.line = lines[i].N
			// Mermaid trims the title and joins its runs of spaces.
			y.title, y.hasTitle = strings.Join(strings.FieldsFunc(s[len("title"):], func(r rune) bool { return r == ' ' || r == '\t' }), " "), true
			y.title = decodeEntities(y.title)
			continue
		}
		lt, err := y.tokens(lines[i])
		if err != nil {
			return err
		}
		toks = append(toks, lt...)
	}
	return flush()
}

// tokens splits a line of a Cynefin diagram into its tokens, with the end
// of the line last.
func (y *cynefin) tokens(l Line) ([]cyToken, error) {
	c := y.c
	var out []cyToken
	s := l.Text
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			return append(out, cyToken{kind: cyEnd, line: l.N}), nil
		}
		switch {
		case strings.HasPrefix(s, "-->"):
			out = append(out, cyToken{kind: cyArrow, line: l.N})
			s = s[3:]
		case s[0] == ':':
			out = append(out, cyToken{kind: cyColon, line: l.N})
			s = s[1:]
		case s[0] == '"' || s[0] == '\'':
			text, n, ok := cyQuoted(s)
			if !ok {
				return nil, c.errorf("a string with no end quote in %q", l.Text)
			}
			out = append(out, cyToken{kind: cyString, text: text, line: l.N})
			s = s[n:]
		default:
			n := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' })
			if n < 0 {
				n = len(s)
			}
			w := s[:n]
			if !slices.Contains(cyDomains, w) {
				if w == "" {
					w = s[:1]
				}
				return nil, c.errorf("the text %q in a Cynefin diagram, which is not a domain, a string or a transition", w)
			}
			out = append(out, cyToken{kind: cyDomain, text: w, line: l.N})
			s = s[n:]
		}
	}
}

// cyQuoted reads a string in double or single quotes at the start of s,
// with the escapes of a backslash. It returns its text and its length in
// s. A new line, a tab or another space that an escape gives draws as a
// space, as in the SVG of Mermaid, and an entity, such as `#quot;`, draws as
// its character.
func cyQuoted(s string) (string, int, bool) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch ch := s[i]; {
		case ch == q:
			return decodeEntities(b.String()), i + 1, true
		case ch == '\\' && i+1 < len(s):
			i++
			switch s[i] {
			case 'b', 'f', 'n', 'r', 't', 'v', '0':
				b.WriteByte(' ')
			default:
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(ch)
		}
	}
	return "", 0, false
}

// statements reads the statements of a run of tokens: a domain and the
// strings of its items, or a transition up to the end of its line.
func (y *cynefin) statements(toks []cyToken) error {
	c := y.c
	cur := ""
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		c.line = t.line
		switch t.kind {
		case cyEnd:
		case cyString:
			if cur == "" {
				return c.errorf("the item %q outside a domain", t.text)
			}
			y.items[cur] = append(y.items[cur], t.text)
		case cyDomain:
			if i+1 < len(toks) && toks[i+1].kind == cyArrow {
				n, err := y.transition(toks, i)
				if err != nil {
					return err
				}
				i, cur = n, ""
				continue
			}
			// A block of a domain replaces the items of a block before it.
			cur = t.text
			y.items[cur] = []string{}
		case cyArrow:
			return c.errorf("a transition with no domain before -->")
		default:
			return c.errorf("a colon outside a transition")
		}
	}
	return nil
}

// transition reads a transition that starts at the token i, and returns
// the number of the token at its end. A transition from a domain to
// itself draws nothing, as in Mermaid.
func (y *cynefin) transition(toks []cyToken, i int) (int, error) {
	c := y.c
	from := toks[i].text
	if i+2 >= len(toks) || toks[i+2].kind != cyDomain {
		return i, c.errorf("a transition from %s with no domain after -->", from)
	}
	t := cyTransition{from: from, to: toks[i+2].text}
	n := i + 3
	if toks[n].kind == cyColon {
		if toks[n+1].kind != cyString {
			return i, c.errorf("the label of the transition from %s to %s, which is not a string", t.from, t.to)
		}
		t.label = toks[n+1].text
		n += 2
	}
	if toks[n].kind != cyEnd {
		return i, c.errorf("the text after the transition from %s to %s", t.from, t.to)
	}
	if t.from != t.to {
		y.transitions = append(y.transitions, t)
	}
	return n, nil
}
