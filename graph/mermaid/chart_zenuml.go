package mermaid

import (
	"regexp"
	"strconv"
	"strings"
)

// zStarterID is the participant that sends the messages of a ZenUML
// diagram that name no sender, when the diagram names no starter. ZenUML
// draws it as an actor with no label.
const zStarterID = "_STARTER_"

// zKinds are the types of a participant of ZenUML that zuzu draws as a
// kind of participant of a sequence diagram.
var zKinds = map[string]string{
	"actor": "actor", "database": "database", "boundary": "boundary", "control": "control", "entity": "entity",
}

// zIcons are the types of a participant that ZenUML draws as an icon of a
// cloud, which zuzu does not draw.
var zIcons = map[string]bool{
	"ec2": true, "lambda": true, "azurefunction": true, "sqs": true, "sns": true, "iam": true,
}

// zenuml is the walk of a ZenUML diagram into a sequence diagram.
type zenuml struct {
	c *chart
	s *sequence
	// starter is the participant that the diagram names with @Starter, or
	// empty.
	starter string
	// noSender reports whether a message has no sender, so that the
	// diagram draws zStarterID.
	noSender bool
	// groups holds the index of the box of each group.
	groups map[string]int
	// decl holds the participants that the head declares, by name.
	decl map[string]*zPart
}

// zenumlChart compiles a ZenUML diagram (D31). It reads the grammar of
// ZenUML, and translates its messages, its fragments and its participants
// into the model of a sequence diagram, which lays it out.
//
// ZenUML reads no setting of the config, so each setting is an issue, as
// in every chart, and the layout takes the defaults of a sequence diagram.
func zenumlChart(c *chart) (map[string]any, bool, error) {
	if err := c.unreadSettings(); err != nil {
		return nil, false, err
	}
	lines := c.d.Lines
	if c.d.Header != "" {
		// ZenUML reads the rest of the first line as code.
		lines = append([]Line{{Text: c.d.Header, N: c.d.N}}, lines...)
	}
	p := &zParser{c: c, lx: zLex(lines)}
	p.toks = p.lx.toks
	prog, err := p.prog()
	if err != nil {
		return nil, false, err
	}
	z := &zenuml{
		c: c, s: &sequence{c: c, byID: map[string]*seqParticipant{}, active: map[string]int{}},
		starter: prog.starter, groups: map[string]int{}, decl: map[string]*zPart{},
	}
	if err := z.declare(prog); err != nil {
		return nil, false, err
	}
	origin := prog.starter
	known := origin != ""
	if !known {
		origin = zStarterID
	}
	if err := z.walk(prog.stats, origin, origin, known, ""); err != nil {
		return nil, false, err
	}
	if _, ok := z.s.byID[zStarterID]; !ok && (z.noSender || len(z.s.participants) == 0) {
		st := &seqParticipant{id: zStarterID, kind: "actor", box: -1, created: -1, destroyed: -1, destroyAt: -1}
		z.s.byID[zStarterID] = st
		z.s.participants = append([]*seqParticipant{st}, z.s.participants...)
	}
	cfg := newSeqConfig(map[string]any{}, c.d.Kind)
	// ZenUML draws the heads at the top only.
	cfg.mirror = false
	spec := newSeqLayout(z.s, cfg).spec()
	if prog.hasTitle && prog.title != "" {
		spec["title"] = map[string]any{"text": prog.title, "anchor": "start"}
	}
	if desc := describe(c.d); desc != "" {
		spec["description"] = desc
	}
	// ZenUML draws its own title, and not the title of the front matter,
	// which Mermaid reads and draws nothing for (D32).
	c.d.Title = ""
	return spec, false, nil
}

// declare adds the participants and the groups of the head, and the
// starter.
func (z *zenuml) declare(prog *zProg) error {
	for i := range prog.parts {
		d := &prog.parts[i]
		z.c.line = d.line
		name := d.name
		if name == "" {
			// ZenUML names a participant with no name so.
			name = "Missing `Participant`"
		}
		if old := z.decl[name]; old != nil {
			// ZenUML keeps the first value of each field of a participant.
			for _, f := range [][2]*string{
				{&old.label, &d.label}, {&old.typ, &d.typ}, {&old.stereotype, &d.stereotype},
				{&old.emoji, &d.emoji}, {&old.color, &d.color}, {&old.group, &d.group},
			} {
				if *f[0] == "" {
					*f[0] = *f[1]
				}
			}
		} else {
			z.decl[name] = d
		}
		z.add(name)
	}
	for _, p := range z.s.participants {
		if err := z.style(p, z.decl[p.id]); err != nil {
			return err
		}
	}
	if prog.starter != "" {
		z.add(prog.starter)
	}
	return nil
}

// style gives a participant the label, the kind and the box that the head
// declares.
func (z *zenuml) style(p *seqParticipant, d *zPart) error {
	z.c.line = d.line
	label := d.label
	if label == "" {
		label = p.id
	}
	p.label = seqText{lines: []string{label}}
	if d.stereotype != "" {
		p.label.lines = []string{"«" + d.stereotype + "»", label}
	}
	typ := strings.ToLower(d.typ)
	if k, ok := zKinds[typ]; ok {
		p.kind = k
	} else if zIcons[typ] {
		if err := z.c.add("the icon %s of the participant %s, drawn as a participant", d.typ, p.id); err != nil {
			return err
		}
	}
	if d.group != "" {
		b, ok := z.groups[d.group]
		if !ok {
			b = len(z.s.boxes)
			z.groups[d.group] = b
			z.s.boxes = append(z.s.boxes, seqBox{title: seqText{lines: []string{d.group}}})
		}
		p.box = b
	}
	if d.color != "" {
		if err := z.c.add("the color %s of the participant %s", d.color, p.id); err != nil {
			return err
		}
	}
	if d.emoji != "" {
		if err := z.c.add("the emoji %s of the participant %s", d.emoji, p.id); err != nil {
			return err
		}
	}
	return nil
}

// add adds a participant by its name, if it is new, with its name as its
// label.
func (z *zenuml) add(name string) *seqParticipant {
	if p := z.s.byID[name]; p != nil {
		return p
	}
	p := &seqParticipant{id: name, kind: "participant", box: -1, created: -1, destroyed: -1, destroyAt: -1}
	p.label = seqText{lines: []string{name}}
	z.s.byID[name] = p
	z.s.participants = append(z.s.participants, p)
	return p
}

// event appends an event of the sequence and returns its index.
func (z *zenuml) event(e seqEvent) int {
	z.s.events = append(z.s.events, e)
	return len(z.s.events) - 1
}

// The arrows of ZenUML: a sync message has a filled head, an async message
// an open head, and a creation and a return a dashed line and an open
// head.
var (
	zSyncArrow   = seqArrow{token: "->>", head: endFilled}
	zAsyncArrow  = seqArrow{token: "-)", head: endOpen}
	zDashedArrow = seqArrow{token: "--)", dotted: true, head: endOpen}
)

// walk translates the statements of a block into events. origin is the
// participant that sends a message that names no sender, and known reports
// whether ZenUML knows it, or draws zStarterID for it. caller is the
// participant that a return goes back to, and parent the number of the
// block.
func (z *zenuml) walk(stats []*zStat, origin, caller string, known bool, parent string) error {
	return z.walkFrom(stats, origin, caller, known, parent, 0)
}

// walkFrom walks statements whose numbers start after offset.
func (z *zenuml) walkFrom(stats []*zStat, origin, caller string, known bool, parent string, offset int) error {
	for k, st := range stats {
		number := strconv.Itoa(offset + k + 1)
		if parent != "" {
			number = parent + "." + number
		}
		z.c.line = st.line
		comment, err := z.comment(st.comment)
		if err != nil {
			return err
		}
		if err := z.stat(st, origin, caller, known, number, comment); err != nil {
			return err
		}
	}
	return nil
}

// stat translates one statement into events.
func (z *zenuml) stat(st *zStat, origin, caller string, known bool, number string, comment []string) error {
	from, to := st.from, st.to
	if from != "" {
		z.add(from)
	}
	if to != "" && st.kind != "creation" {
		z.add(to)
	}
	if from == "" {
		from = origin
		if !known && st.kind != "divider" && st.kind != "fragment" {
			z.noSender = true
		}
	}
	switch st.kind {
	case "sync", "creation":
		arrow := zSyncArrow
		if st.kind == "creation" {
			arrow = zDashedArrow
			if st.assignee != "" {
				to = st.assignee + ":" + st.to
			}
			z.add(to)
		}
		if to == "" {
			to = origin
		}
		// A sync message activates its receiver at its line. A creation
		// activates the new participant below its head.
		n := z.event(seqEvent{
			kind: "message", line: st.line, from: from, to: to, arrow: arrow, activate: st.kind == "sync",
			text: seqText{lines: zLines(st.label)}, number: number, comment: comment,
		})
		if st.kind == "creation" {
			if p := z.s.byID[to]; p.created < 0 && !p.used {
				p.created = n
			}
			z.event(seqEvent{kind: "activate", line: st.line, from: to})
		}
		z.use(from, to)
		if err := z.walk(st.stats, to, from, true, number); err != nil {
			return err
		}
		if st.assignee != "" && from != to {
			z.event(seqEvent{
				kind: "message", line: st.line, from: to, to: from, arrow: zDashedArrow,
				text: seqText{lines: []string{st.assignee}}, number: number + "." + strconv.Itoa(len(st.stats)+1),
			})
		}
		z.event(seqEvent{kind: "deactivate", line: st.line, from: to})
	case "async", "return":
		arrow := zAsyncArrow
		if st.kind == "return" {
			arrow = zDashedArrow
			if to == "" {
				to = caller
			}
		}
		if to == "" {
			to = origin
			if !known {
				to = from
			}
		}
		if st.kind == "return" && from == to {
			// ZenUML draws no number on a return to itself.
			number = ""
		}
		z.event(seqEvent{
			kind: "message", line: st.line, from: from, to: to, arrow: arrow,
			text: seqText{lines: zLines(st.label)}, number: number, comment: comment,
		})
		z.use(from, to)
	case "divider":
		z.event(seqEvent{kind: "divider", line: st.line, text: seqText{lines: zLines(st.label)}})
	case "fragment":
		return z.fragment(st, origin, caller, known, number, comment)
	}
	return nil
}

// use marks participants as used by an event.
func (z *zenuml) use(ids ...string) {
	for _, id := range ids {
		if id != zStarterID {
			z.add(id).used = true
		}
	}
}

// zBlocks are the words that the tab of each fragment of ZenUML shows.
var zBlocks = map[string]string{
	"alt": "alt", "loop": "loop", "opt": "opt", "par": "par", "critical": "critical",
	"section": "section", "tcf": "try", "ref": "ref",
}

// fragment translates a fragment into a block of the sequence: its start,
// its sections and its end.
func (z *zenuml) fragment(st *zStat, origin, caller string, known bool, number string, comment []string) error {
	e := seqEvent{kind: "start", line: st.line, block: zBlocks[st.frag], number: number, comment: comment, from: origin}
	if len(st.sections) > 0 {
		e.text = seqText{lines: zLines(st.sections[0].label)}
	}
	if st.frag == "tcf" {
		e.text = seqText{}
	}
	if st.frag == "ref" && len(st.refs) > 0 {
		for _, r := range st.refs {
			z.add(r)
		}
		e.from, e.to = st.refs[0], st.refs[len(st.refs)-1]
	}
	z.event(e)
	offset := 0
	for k, sec := range st.sections {
		if k > 0 {
			z.event(seqEvent{kind: "section", line: st.line, text: seqText{lines: zLines(sec.label)}})
		}
		if err := z.walkFrom(sec.stats, origin, caller, known, number, offset); err != nil {
			return err
		}
		offset += len(sec.stats)
	}
	z.event(seqEvent{kind: "end", line: st.line, block: e.block})
	return nil
}

// zLines returns a label as the lines of a text, none for an empty label.
func zLines(s string) []string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return []string{s}
}

var (
	// zStyleRE matches the styles at the start of the last line of a
	// comment, such as [red] or <bold>, which ZenUML applies to the
	// comment and the message.
	zStyleRE = regexp.MustCompile(`^\s*((?:<[^>]*>|\([^)]*\)|\[[^\]]*\]|\s)+)`)
	// zMarkRE matches the marks of Markdown that zuzu takes out of a
	// comment: bold, italic with underscores, and code.
	zMarkRE = regexp.MustCompile("\\*\\*|__|`")
)

// comment returns the lines of a comment that ZenUML draws over a
// statement. A style of the comment and its Markdown are issues, and the
// text draws plain.
func (z *zenuml) comment(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var lines []string
	for k, l := range raw {
		if k == len(raw)-1 {
			if m := zStyleRE.FindString(l); strings.TrimSpace(m) != "" {
				if err := z.c.add("the style %s of a comment", strings.TrimSpace(m)); err != nil {
					return nil, err
				}
				l = l[len(m):]
			}
		}
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	text := strings.Join(lines, " ")
	if zMarkRE.MatchString(text) {
		if err := z.c.add("the Markdown of the comment %q, drawn as plain text", text); err != nil {
			return nil, err
		}
		for k := range lines {
			lines[k] = zMarkRE.ReplaceAllString(lines[k], "")
		}
	}
	return lines, nil
}
