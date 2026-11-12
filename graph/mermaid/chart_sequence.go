package mermaid

import (
	"encoding/json/v2"
	"html"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/zuzu/internal/color"
)

// seqEnd is the mark at one end of a message.
type seqEnd int

// The marks at the end of a message.
const (
	endNone seqEnd = iota
	endFilled
	endOpen
	endCross
	endHalfTop
	endHalfBottom
	endStickTop
	endStickBottom
)

// seqArrow is an arrow of a message: its token, whether its line is
// dotted, and the marks at its head, where the receiver is, and at its
// tail, where the sender is.
type seqArrow struct {
	token      string
	dotted     bool
	head, tail seqEnd
}

// seqArrows are the arrows of a message, the longest token first, so that
// the first token that matches is the right one.
var seqArrows = []seqArrow{
	{"<<-->>", true, endFilled, endFilled},
	{"<<->>", false, endFilled, endFilled},
	{"-->>", true, endFilled, endNone},
	{`--|\`, true, endHalfTop, endNone},
	{"--|/", true, endHalfBottom, endNone},
	{`--\\`, true, endStickTop, endNone},
	{"--//", true, endStickBottom, endNone},
	{"/|--", true, endNone, endHalfTop},
	{`\|--`, true, endNone, endHalfBottom},
	{"//--", true, endNone, endStickTop},
	{`\\--`, true, endNone, endStickBottom},
	{"->>", false, endFilled, endNone},
	{"-->", true, endNone, endNone},
	{"--x", true, endCross, endNone},
	{"--)", true, endOpen, endNone},
	{`-|\`, false, endHalfTop, endNone},
	{"-|/", false, endHalfBottom, endNone},
	{`-\\`, false, endStickTop, endNone},
	{"-//", false, endStickBottom, endNone},
	{"/|-", false, endNone, endHalfTop},
	{`\|-`, false, endNone, endHalfBottom},
	{"//-", false, endNone, endStickTop},
	{`\\-`, false, endNone, endStickBottom},
	{"->", false, endNone, endNone},
	{"-x", false, endCross, endNone},
	{"-)", false, endOpen, endNone},
}

// seqTypes are the kinds of participant that zuzu draws.
var seqTypes = []string{"participant", "actor", "boundary", "control", "entity", "database", "collections", "queue"}

// seqParticipant is a participant of a sequence diagram.
type seqParticipant struct {
	id, kind string
	label    seqText
	// box is the index of the box of the participant, or -1.
	box int
	// created and destroyed are the indexes of the events of the messages
	// that create and destroy the participant, or -1.
	created, destroyed int
	// destroyAt is the index of the event after destroy, or -1. The
	// participant is destroyed only when that event is a message that names
	// it, as in Mermaid.
	destroyAt int
	// used reports whether an event names the participant.
	used bool
}

// seqText is a text of the source: its lines, and its wrap, which is nil
// when the text follows the setting of the diagram.
type seqText struct {
	lines []string
	wrap  *bool
}

// seqEvent is a statement of a sequence diagram, in the order of the
// source.
type seqEvent struct {
	kind string
	line int
	text seqText
	// from and to are the ends of a message, the participants of a note
	// or of an activation in from, and the participants that a fragment of
	// ZenUML starts at and spans.
	from, to string
	arrow    seqArrow
	// activate and deactivate are the shorthands + and - of a message.
	activate, deactivate bool
	// centralFrom and centralTo are the central connections () of a
	// message.
	centralFrom, centralTo bool
	// place is left, right or over, for a note.
	place string
	// block is the keyword of a block, such as loop, for its start, its
	// sections and its end.
	block string
	// fill is the color of a rect.
	fill string
	// start and step are the numbers of autonumber, and visible whether it
	// is on.
	start, step float64
	visible     bool
	// number is the number that ZenUML gives a message or a fragment,
	// which its badge shows in place of the number of autonumber.
	number string
	// comment holds the lines of a comment of ZenUML, which draw over a
	// message or a fragment.
	comment []string
}

// seqBox is a box of participants: its title and its color.
type seqBox struct {
	title seqText
	fill  string
}

// sequence is a sequence diagram as the compiler reads it.
type sequence struct {
	c        *chart
	title    string
	hasTitle bool
	// acc holds the texts of accTitle and accDescr, which join into the
	// description of the spec.
	acc          []string
	participants []*seqParticipant
	byID         map[string]*seqParticipant
	boxes        []seqBox
	events       []seqEvent
	// entities are the texts of the entity codes of the source, which a
	// placeholder holds until the text is drawn.
	entities []string
	// pendingCreate and pendingDestroy are the participants whose next
	// message creates or destroys them. A create goes first: the message
	// after it clears only the create, as in Mermaid.
	pendingCreate, pendingDestroy string
	// active counts the activations of each participant, to find a
	// deactivation of a participant that is not active.
	active map[string]int
	// blocks is the stack of the keywords of the open blocks.
	blocks []string
	// math holds each text with math of KaTeX and its line, for the
	// report. Mermaid draws the math, and zuzu draws the source.
	math []Line
}

var (
	// seqEntity matches an entity code, such as `#9829;` or `#infin;`.
	seqEntity = regexp.MustCompile(`#([0-9]+|[A-Za-z][A-Za-z0-9]*);`)
	// seqPlaceholder matches the placeholder of an entity code.
	seqPlaceholder = regexp.MustCompile("([0-9]+)")
	// seqBreak matches a line break in a text.
	seqBreak = regexp.MustCompile(`(?i)<br\s*/?>`)
	// seqDeclare matches a participant or an actor, created or not.
	seqDeclare = regexp.MustCompile(`(?i)^(create\s+)?(participant|actor)\s+(.+)$`)
	// seqAlias matches an id and its alias.
	seqAlias = regexp.MustCompile(`(?i)^(\S+?)\s+as\s+(.*)$`)
	// seqAfterMeta matches the alias after the metadata of a participant.
	seqAfterMeta = regexp.MustCompile(`(?i)^as(?:\s+(.*))?$`)
	// seqNote matches a note: its place, its participants and its text.
	seqNote = regexp.MustCompile(`(?i)^note\s+(left\s+of|right\s+of|over)\s+([^:]+?)\s*:(.*)$`)
	// seqMenu matches a menu of a participant: links, link, properties and
	// details.
	seqMenu = regexp.MustCompile(`(?i)^(links|link|properties|details)\s+([^:]+?)\s*:(.*)$`)
	// seqBlock matches the start of a block and its text.
	seqBlock = regexp.MustCompile(`(?i)^(loop|rect|opt|alt|par_over|par|critical|break)(?:\s+(.*))?$`)
	// seqSection matches a section of a block and its text.
	seqSection = regexp.MustCompile(`(?i)^(else|and|option)(?:\s+(.*))?$`)
	// seqKeyword matches a statement of one keyword and an argument.
	seqKeyword = regexp.MustCompile(`(?i)^(activate|deactivate|destroy|autonumber|box|title)(?:\s+(.*))?$`)
	// seqBoxColor matches the color at the start of the text of a box.
	seqBoxColor = regexp.MustCompile(`^((?:rgba?|hsla?)\s*\(.*?\)|\w*)(.*)$`)
)

// sectionOf names the keyword of the sections of each block that has
// them.
var sectionOf = map[string]string{"alt": "else", "par": "and", "par_over": "and", "critical": "option"}

// sequenceChart compiles a sequence diagram: participants with their
// lifelines, and messages, notes, activations and blocks down the page,
// into a Vega spec (D31). The compiler lays out the diagram in Go, and the
// spec holds rows of rects, rules, paths, symbols and text.
func sequenceChart(c *chart) (map[string]any, bool, error) {
	cfg, err := c.sequenceSettings()
	if err != nil {
		return nil, false, err
	}
	if c.d.Header != "" {
		c.line = c.d.N
		return nil, false, c.errorf("the text %q after %s", c.d.Header, c.d.Kind)
	}
	s := &sequence{c: c, byID: map[string]*seqParticipant{}, active: map[string]int{}}
	if err := s.parse(); err != nil {
		return nil, false, err
	}
	l := newSeqLayout(s, cfg)
	spec := l.spec()
	if s.hasTitle {
		spec["title"] = map[string]any{"text": s.title, "anchor": "start"}
	}
	if desc := describe(c.d, s.acc...); desc != "" {
		spec["description"] = desc
	}
	return spec, false, nil
}

// parse reads the body of a sequence diagram.
func (s *sequence) parse() error {
	lines := s.c.d.Lines
	for i := 0; i < len(lines); i++ {
		s.c.line = lines[i].N
		if accRE.MatchString(lines[i].Text) {
			text, n, ok := ganttAccText(lines, i)
			if !ok {
				return s.c.errorf("an accDescr block with no end brace")
			}
			i = n
			s.acc = append(s.acc, text)
			continue
		}
		for _, st := range s.statements(lines[i].Text) {
			if err := s.statement(st, lines, &i); err != nil {
				return err
			}
		}
	}
	s.c.line = s.c.d.N
	if len(lines) > 0 {
		s.c.line = lines[len(lines)-1].N
	}
	switch {
	case len(s.blocks) > 0:
		return s.c.errorf("a %s with no end", s.blocks[len(s.blocks)-1])
	case s.pendingCreate != "":
		return s.c.errorf("the participant %s is created and no message follows to it", s.pendingCreate)
	case len(s.participants) == 0:
		return s.c.errorf("a sequence diagram with no participant")
	}
	// Mermaid 12.3 draws the math of KaTeX in each text of a sequence
	// diagram, whatever htmlLabels is.
	for _, m := range s.math {
		s.c.line = m.N
		if err := s.c.add(mathIssue, m.Text); err != nil {
			return err
		}
	}
	return nil
}

// statements splits a line into its statements. An entity code becomes a
// placeholder first, so that `#59;` is not the end of a statement. A # that
// is not an entity code starts a comment to the end of the line, and a
// semicolon ends a statement.
func (s *sequence) statements(line string) []string {
	line = seqEntity.ReplaceAllStringFunc(line, func(m string) string {
		name := m[1 : len(m)-1]
		var v string
		if name[0] >= '0' && name[0] <= '9' {
			v = html.UnescapeString("&#" + name + ";")
		} else {
			v = html.UnescapeString("&" + name + ";")
			if v == "&"+name+";" {
				v = m
			}
		}
		s.entities = append(s.entities, v)
		return "" + strconv.Itoa(len(s.entities)-1) + ""
	})
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
	}
	var out []string
	for st := range strings.SplitSeq(line, ";") {
		if st = strings.TrimSpace(st); st != "" {
			out = append(out, st)
		}
	}
	return out
}

// text reads a text of the source: the prefix wrap: or nowrap:, the
// entity codes and the line breaks.
func (s *sequence) text(t string) seqText {
	t = strings.TrimSpace(t)
	var out seqText
	for _, p := range []struct {
		prefix string
		wrap   bool
	}{{"wrap:", true}, {"nowrap:", false}, {":wrap:", true}, {":nowrap:", false}} {
		if rest, ok := strings.CutPrefix(t, p.prefix); ok {
			w := p.wrap
			out.wrap = &w
			t = strings.TrimSpace(rest)
			break
		}
	}
	t = s.decode(t)
	if hasMath(t) {
		s.math = append(s.math, Line{Text: t, N: s.c.line})
	}
	for _, l := range seqBreak.Split(t, -1) {
		out.lines = append(out.lines, strings.TrimSpace(l))
	}
	if len(out.lines) == 1 && out.lines[0] == "" {
		out.lines = nil
	}
	return out
}

// decode puts the text of each entity code back in place of its
// placeholder.
func (s *sequence) decode(t string) string {
	return seqPlaceholder.ReplaceAllStringFunc(t, func(m string) string {
		n, _ := strconv.Atoi(m[len("") : len(m)-len("")])
		return s.entities[n]
	})
}

// statement reads one statement. A box reads the lines up to its end.
func (s *sequence) statement(st string, lines []Line, i *int) error {
	if m := seqDeclare.FindStringSubmatch(st); m != nil {
		return s.declare(m[1] != "", strings.ToLower(m[2]), m[3], -1)
	}
	if m := seqNote.FindStringSubmatch(st); m != nil {
		return s.note(m[1], m[2], m[3])
	}
	if m := seqMenu.FindStringSubmatch(st); m != nil {
		s.participant(strings.TrimSpace(m[2]))
		return s.c.add("the %s of the participant %s, which zuzu does not draw", strings.ToLower(m[1]), strings.TrimSpace(m[2]))
	}
	if m := seqBlock.FindStringSubmatch(st); m != nil {
		return s.blockStart(strings.ToLower(m[1]), m[2])
	}
	if m := seqSection.FindStringSubmatch(st); m != nil {
		return s.section(strings.ToLower(m[1]), m[2])
	}
	if strings.EqualFold(st, "end") {
		if len(s.blocks) == 0 {
			return s.c.errorf("an end with no block")
		}
		b := s.blocks[len(s.blocks)-1]
		s.blocks = s.blocks[:len(s.blocks)-1]
		s.events = append(s.events, seqEvent{kind: "end", line: s.c.line, block: b})
		return nil
	}
	if rest, ok := strings.CutPrefix(st, "title:"); ok {
		s.title, s.hasTitle = s.decode(strings.TrimSpace(rest)), true
		return nil
	}
	if m := seqKeyword.FindStringSubmatch(st); m != nil {
		return s.keyword(strings.ToLower(m[1]), strings.TrimSpace(m[2]), lines, i)
	}
	return s.message(st)
}

// keyword reads a statement of a keyword: activate, deactivate, destroy,
// autonumber, box and title.
func (s *sequence) keyword(word, arg string, lines []Line, i *int) error {
	switch word {
	case "title":
		s.title, s.hasTitle = s.decode(arg), true
		return nil
	case "autonumber":
		return s.autonumber(arg)
	case "box":
		return s.box(arg, lines, i)
	}
	if arg == "" {
		return s.c.errorf("%s with no participant", word)
	}
	p := s.participant(arg)
	p.used = true
	switch word {
	case "activate":
		s.active[p.id]++
		s.events = append(s.events, seqEvent{kind: "activate", line: s.c.line, from: p.id})
	case "deactivate":
		if s.active[p.id] < 1 {
			return s.c.errorf("a deactivation of %s, which is not active", p.id)
		}
		s.active[p.id]--
		s.events = append(s.events, seqEvent{kind: "deactivate", line: s.c.line, from: p.id})
	case "destroy":
		if p.destroyed >= 0 {
			return s.c.errorf("the participant %s is destroyed twice", p.id)
		}
		s.pendingDestroy, p.destroyAt = p.id, len(s.events)
	}
	return nil
}

// autonumber reads autonumber, with off, or with a start and a step.
func (s *sequence) autonumber(arg string) error {
	e := seqEvent{kind: "autonumber", line: s.c.line, visible: true}
	f := strings.Fields(arg)
	switch {
	case len(f) == 1 && strings.EqualFold(f[0], "off"):
		e.visible = false
	case len(f) <= 2:
		for k, v := range f {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil || n < 0 {
				return s.c.errorf("the number %q of autonumber", v)
			}
			if k == 0 {
				e.start = n
			} else {
				e.step = n
			}
		}
	default:
		return s.c.errorf("autonumber %s", arg)
	}
	s.events = append(s.events, e)
	return nil
}

// box reads a box: its color and its title, then its participants up to
// its end.
func (s *sequence) box(arg string, lines []Line, i *int) error {
	b := seqBox{}
	m := seqBoxColor.FindStringSubmatch(arg)
	fill, title := m[1], m[2]
	if _, ok := color.Parse(fill); !ok || fill == "" {
		fill, title = "", arg
	}
	if strings.EqualFold(fill, "transparent") {
		fill = ""
	}
	b.fill = fill
	// Mermaid draws the title of a box as text, with no math, as zuzu
	// does, so its math is no issue.
	kept := len(s.math)
	b.title = s.text(title)
	s.math = s.math[:kept]
	s.boxes = append(s.boxes, b)
	n := len(s.boxes) - 1
	for *i++; *i < len(lines); *i++ {
		s.c.line = lines[*i].N
		for _, st := range s.statements(lines[*i].Text) {
			if strings.EqualFold(st, "end") {
				return nil
			}
			m := seqDeclare.FindStringSubmatch(st)
			if m == nil {
				return s.c.errorf("the statement %q in a box, which holds only participants", st)
			}
			if err := s.declare(m[1] != "", strings.ToLower(m[2]), m[3], n); err != nil {
				return err
			}
		}
	}
	return s.c.errorf("a box with no end")
}

// declare reads a participant or an actor: its id, its metadata in @{} and
// its alias after as.
func (s *sequence) declare(create bool, kind, rest string, box int) error {
	rest = strings.TrimSpace(rest)
	var id, alias, meta string
	hasAlias := false
	if at := strings.Index(rest, "@{"); at >= 0 {
		end := strings.LastIndex(rest, "}")
		if end < at {
			return s.c.errorf("the metadata of %q has no end brace", rest)
		}
		id, meta = strings.TrimSpace(rest[:at]), rest[at+1:end+1]
		after := strings.TrimSpace(rest[end+1:])
		if after != "" {
			m := seqAfterMeta.FindStringSubmatch(after)
			if m == nil {
				return s.c.errorf("the text %q after the metadata of %s", after, id)
			}
			alias, hasAlias = m[1], true
		}
	} else if m := seqAlias.FindStringSubmatch(rest); m != nil {
		id, alias, hasAlias = m[1], m[2], true
	} else {
		id = rest
	}
	if id == "" || strings.ContainsAny(id, "<>:,;@") {
		return s.c.errorf("the participant %q", rest)
	}
	if hasAlias && strings.TrimSpace(alias) == "" {
		return s.c.errorf("the participant %s with as and no alias", id)
	}
	if meta != "" {
		var err error
		if kind, alias, hasAlias, err = s.metadata(id, meta, kind, alias, hasAlias); err != nil {
			return err
		}
	}
	old := s.byID[id]
	if create {
		if old != nil {
			return s.c.errorf("the participant %s is created and it exists", id)
		}
		if s.pendingCreate != "" {
			return s.c.errorf("the participant %s is created and no message follows to it", s.pendingCreate)
		}
		s.pendingCreate = id
	}
	if old != nil && old.box >= 0 && box >= 0 && old.box != box {
		return s.c.errorf("the participant %s is in two boxes", id)
	}
	p := s.participant(id)
	if box >= 0 {
		p.box = box
	}
	if old != nil && !hasAlias {
		// As in Mermaid, a participant that is named again with no alias
		// keeps its kind.
		return nil
	}
	p.kind = kind
	if hasAlias {
		p.label = s.text(alias)
	}
	return nil
}

// metadata reads the metadata of a participant, @{"type": "queue"}, and
// returns its kind and its alias. An alias after as wins over the alias of
// the metadata.
func (s *sequence) metadata(id, meta, kind, alias string, hasAlias bool) (string, string, bool, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(relaxedJSON(meta)), &m); err != nil {
		return "", "", false, s.c.errorf("the metadata %s of %s", meta, id)
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v, _ := m[k].(string)
		switch k {
		case "type":
			if !slices.Contains(seqTypes, v) {
				if err := s.c.add("the type %q of the participant %s, drawn as a participant", v, id); err != nil {
					return "", "", false, err
				}
				v = "participant"
			}
			kind = v
		case "alias":
			if !hasAlias {
				alias, hasAlias = v, true
			}
		default:
			if err := s.c.add("the key %s of the metadata of the participant %s", k, id); err != nil {
				return "", "", false, err
			}
		}
	}
	return kind, alias, hasAlias, nil
}

// participant returns the participant of an id, and adds it if it is new.
func (s *sequence) participant(id string) *seqParticipant {
	if p := s.byID[id]; p != nil {
		return p
	}
	p := &seqParticipant{id: id, kind: "participant", box: -1, created: -1, destroyed: -1, destroyAt: -1}
	p.label = seqText{lines: []string{s.decode(id)}}
	s.byID[id] = p
	s.participants = append(s.participants, p)
	return p
}

// note reads a note: its place, one or two participants and its text.
func (s *sequence) note(place, who, text string) error {
	e := seqEvent{kind: "note", line: s.c.line, text: s.text(text)}
	switch strings.ToLower(strings.Join(strings.Fields(place), " ")) {
	case "left of":
		e.place = "left"
	case "right of":
		e.place = "right"
	default:
		e.place = "over"
	}
	parts := strings.Split(who, ",")
	if len(parts) > 2 || e.place != "over" && len(parts) > 1 {
		return s.c.errorf("the participants %q of a note", who)
	}
	for k, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return s.c.errorf("the participants %q of a note", who)
		}
		s.participant(p).used = true
		if k == 0 {
			e.from, e.to = p, p
		} else {
			e.to = p
		}
	}
	s.events = append(s.events, e)
	return nil
}

// blockStart reads the start of a block, such as loop Every minute.
func (s *sequence) blockStart(word, text string) error {
	e := seqEvent{kind: "start", line: s.c.line, block: word}
	if word == "rect" {
		e.fill = strings.TrimSpace(text)
		if _, ok := color.Parse(e.fill); !ok {
			if err := s.c.add("the color %q of a rect, drawn as a panel", e.fill); err != nil {
				return err
			}
			e.fill = ""
		}
	} else {
		e.text = s.text(text)
	}
	s.blocks = append(s.blocks, word)
	s.events = append(s.events, e)
	return nil
}

// section reads a section of a block: else, and or option.
func (s *sequence) section(word, text string) error {
	if len(s.blocks) == 0 || sectionOf[s.blocks[len(s.blocks)-1]] != word {
		return s.c.errorf("%s outside of its block", word)
	}
	s.events = append(s.events, seqEvent{kind: "section", line: s.c.line, block: word, text: s.text(text)})
	return nil
}

// message reads a message: a sender, an arrow, a receiver and a text.
func (s *sequence) message(st string) error {
	from, rest, arrow, ok := cutArrow(st)
	if !ok {
		return s.c.errorf("the statement %q", st)
	}
	e := seqEvent{kind: "message", line: s.c.line, arrow: arrow}
	from = strings.TrimSpace(from)
	if f, ok := strings.CutSuffix(from, "()"); ok {
		from, e.centralFrom = strings.TrimSpace(f), true
	}
	rest = strings.TrimLeft(rest, " \t")
	switch {
	case strings.HasPrefix(rest, "+"):
		e.activate, rest = true, rest[1:]
	case strings.HasPrefix(rest, "-"):
		e.deactivate, rest = true, rest[1:]
	}
	rest = strings.TrimLeft(rest, " \t")
	if r, ok := strings.CutPrefix(rest, "()"); ok {
		rest, e.centralTo = r, true
	}
	to, text, ok := strings.Cut(rest, ":")
	to = strings.TrimSpace(to)
	if !ok {
		return s.c.errorf("the message %q has no colon and no text", st)
	}
	if from == "" || to == "" || strings.ContainsAny(from+to, "<>,") {
		return s.c.errorf("the message %q", st)
	}
	e.from, e.to, e.text = from, to, s.text(text)
	if err := s.lifecycle(e); err != nil {
		return err
	}
	fp, tp := s.participant(from), s.participant(to)
	fp.used, tp.used = true, true
	if e.activate {
		s.active[to]++
	}
	if e.deactivate {
		if s.active[from] < 1 {
			return s.c.errorf("a deactivation of %s, which is not active", from)
		}
		s.active[from]--
	}
	n := len(s.events)
	if s.pendingCreate != "" {
		tp.created, s.pendingCreate = n, ""
	} else {
		s.pendingDestroy = ""
	}
	for _, p := range []*seqParticipant{fp, tp} {
		if p.destroyAt == n {
			p.destroyed = n
		}
	}
	s.events = append(s.events, e)
	return nil
}

// lifecycle checks a message against a create or a destroy before it: a
// created participant receives the next message, and a destroyed one sends
// or receives it. As in Mermaid, a create goes before a destroy: a message
// that a create waits for does not check the destroy.
func (s *sequence) lifecycle(e seqEvent) error {
	if s.pendingCreate != "" && s.pendingCreate != e.to {
		return s.c.errorf("the participant %s is created and the next message does not go to it", s.pendingCreate)
	}
	if s.pendingCreate == "" && s.pendingDestroy != "" && s.pendingDestroy != e.to && s.pendingDestroy != e.from {
		return s.c.errorf("the participant %s is destroyed and the next message does not name it", s.pendingDestroy)
	}
	for _, id := range []string{e.from, e.to} {
		if p := s.byID[id]; p != nil && p.destroyed >= 0 {
			return s.c.errorf("a message of %s after it is destroyed", id)
		}
	}
	return nil
}

// cutArrow finds the first arrow in a statement, and returns the text
// before it, the text after it and the arrow.
func cutArrow(st string) (string, string, seqArrow, bool) {
	for i := range len(st) {
		for _, a := range seqArrows {
			if strings.HasPrefix(st[i:], a.token) {
				return st[:i], st[i+len(a.token):], a, true
			}
		}
	}
	return "", "", seqArrow{}, false
}
