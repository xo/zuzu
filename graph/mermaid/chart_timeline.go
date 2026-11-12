package mermaid

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tlPeriod is a period of a timeline: its text and the texts of its
// events, as the source gives them.
type tlPeriod struct {
	text   string
	events []string
}

// tlGroup is a section of a timeline and its periods. The periods before
// the first section form a group with no header.
type tlGroup struct {
	name    string
	header  bool
	periods []*tlPeriod
}

// timeline is a timeline as the compiler reads it.
type timeline struct {
	c *chart
	// dir is LR or TD.
	dir      string
	title    string
	hasTitle bool
	// multicolor is false when timeline.disableMulticolor is true.
	multicolor bool
	groups     []*tlGroup
	// last is the last period, which takes the next event.
	last *tlPeriod
}

// The tokens of a timeline that Mermaid 12.3 reads at the start of a
// statement, in any case.
var (
	// tlDirRE matches the direction after the keyword timeline.
	tlDirRE = regexp.MustCompile(`(?i)^(LR|TD)\b`)
	// tlKeywordRE matches the keyword timeline, which only the first line
	// can hold.
	tlKeywordRE = regexp.MustCompile(`(?i)^timeline\b`)
	// tlTitleRE matches a title: the word title, a space and a text.
	tlTitleRE = regexp.MustCompile(`(?i)^title\s.`)
	// tlSectionRE matches a section: the word section, a space and a name
	// up to a colon.
	tlSectionRE = regexp.MustCompile(`(?i)^section\s[^:]+`)
	// tlAccRE matches accTitle and accDescr in any case.
	tlAccRE = regexp.MustCompile(`(?i)^acc(title\s*:|descr\s*[:{])`)
	// tlEntityRE matches an entity as Mermaid reads it before the parse,
	// such as `#quot;` or `#35;`. Its # does not start a comment.
	tlEntityRE = regexp.MustCompile(`#\w+;`)
)

// tlHash stands for the # of an entity while a line is read, so that it
// does not start a comment.
const tlHash = "\x00"

// timelineChart compiles a timeline: its periods from left to right, or
// from top to bottom, each with its events, under the headers of their
// sections, into a Vega spec (D31). The compiler lays the timeline out in
// Go, and the spec holds rows of rects, rules, paths and text.
func timelineChart(c *chart) (map[string]any, bool, error) {
	t := &timeline{c: c, dir: "LR", multicolor: true}
	if err := t.settings(); err != nil {
		return nil, false, err
	}
	if err := t.parse(); err != nil {
		return nil, false, err
	}
	if len(t.groups) == 0 {
		// Mermaid draws the title and the axis of a timeline with no
		// period and no section.
		return emptyChart(t.title, ""), false, nil
	}
	spec := newTimelineLayout(t).spec()
	if t.hasTitle {
		spec["title"] = map[string]any{"text": t.title, "anchor": "start"}
	}
	return spec, false, nil
}

// settings reads the config of a timeline. zuzu reads
// timeline.disableMulticolor, and each other setting is an issue.
func (t *timeline) settings() error {
	if err := t.c.unreadSettings("timeline.disableMulticolor"); err != nil {
		return err
	}
	sec, _ := t.c.d.Config["timeline"].(map[string]any)
	v, ok := sec["disableMulticolor"]
	if !ok {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return t.c.add("the setting timeline.disableMulticolor %v, which is not true or false", v)
	}
	t.multicolor = !b
	return nil
}

// parse reads the header and the statements of a timeline.
func (t *timeline) parse() error {
	c := t.c
	c.line = c.d.N
	head := strings.TrimSpace(c.d.Header)
	if m := tlDirRE.FindString(head); m != "" {
		t.dir = strings.ToUpper(m)
		head = strings.TrimSpace(head[len(m):])
	}
	// The text after the keyword and the direction holds statements, as
	// in Mermaid, so timeline TB is a period TB.
	lines := make([]Line, len(c.d.Lines))
	copy(lines, c.d.Lines)
	if head != "" {
		lines = append([]Line{{Text: head, N: c.d.N}}, lines...)
	}
	for i := 0; i < len(lines); i++ {
		c.line = lines[i].N
		// Mermaid reads accTitle and accDescr in any case.
		if m := tlAccRE.FindString(lines[i].Text); m != "" {
			name := "accTitle"
			if strings.EqualFold(m[:8], "accdescr") {
				name = "accDescr"
			}
			lines[i].Text = name + lines[i].Text[8:]
		}
		n, ok, err := c.accessibility(lines, i)
		if err != nil {
			return err
		}
		if ok {
			i = n
			continue
		}
		next, err := t.statements(lines, i)
		if err != nil {
			return err
		}
		i = next
	}
	return nil
}

// statements reads the statements of the line i, as the lexer of Mermaid
// 12.3 reads them, and returns the last line that it read. A line holds a
// title, a section, a period or an event first, and then events. A colon
// at the end of a line takes the next line as its event.
func (t *timeline) statements(lines []Line, i int) (int, error) {
	c := t.c
	s := tlMask(lines[i].Text)
	switch {
	case strings.HasPrefix(s, "%") && !strings.HasPrefix(s, "%{"), strings.HasPrefix(s, "#"):
		// A % or a # starts a comment that runs to the end of the line.
		return i, nil
	case tlKeywordRE.MatchString(s):
		return i, c.errorf("the keyword timeline after the first line")
	case tlTitleRE.MatchString(s):
		t.title, t.hasTitle = tlText(s[6:]), true
		return i, nil
	}
	if m := tlSectionRE.FindString(s); m != "" {
		t.groups = append(t.groups, &tlGroup{name: tlText(m[8:]), header: true})
		s = s[len(m):]
	} else if end := strings.IndexAny(s, "#:"); end != 0 {
		// A period runs up to a colon or a #.
		if end < 0 {
			end = len(s)
		}
		p := &tlPeriod{text: tlText(s[:end])}
		if len(t.groups) == 0 {
			t.groups = append(t.groups, &tlGroup{})
		}
		g := t.groups[len(t.groups)-1]
		g.periods = append(g.periods, p)
		t.last = p
		s = s[end:]
	}
	for {
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
		switch {
		case s == "", strings.HasPrefix(s, "#"):
			return i, nil
		case s == ":" && i+1 < len(lines):
			// The space after the colon is the end of the line, and the
			// event is the next line.
			i++
			c.line = lines[i].N
			s = ":\n" + tlMask(lines[i].Text)
		case !strings.HasPrefix(s, ":"):
			return i, c.errorf("the text %q", tlUnmask(s))
		}
		r, size := utf8.DecodeRuneInString(s[1:])
		if size == 0 {
			return i, c.errorf("an event with no text at the end of the source")
		}
		if !unicode.IsSpace(r) {
			return i, c.errorf("a colon with no space after it in %q", tlUnmask(s))
		}
		// An event runs up to a colon with a space after it, or up to the
		// end of the line.
		body := s[1+size:]
		end := len(body)
		for k := range len(body) {
			if body[k] != ':' {
				continue
			}
			r, _ := utf8.DecodeRuneInString(body[k+1:])
			if k+1 == len(body) || unicode.IsSpace(r) {
				end = k
				break
			}
		}
		if end == 0 {
			return i, c.errorf("an event with no text in %q", tlUnmask(s))
		}
		if t.last == nil {
			return i, c.errorf("an event before the first period")
		}
		t.last.events = append(t.last.events, tlText(body[:end]))
		s = body[end:]
	}
}

// tlMask hides the # of each entity of a line, so that it does not start a
// comment.
func tlMask(s string) string {
	return tlEntityRE.ReplaceAllStringFunc(s, func(m string) string { return tlHash + m[1:] })
}

// tlUnmask gives back the # of each entity of a text that tlMask hid.
func tlUnmask(s string) string {
	return strings.ReplaceAll(s, tlHash, "#")
}

// tlText returns the text of a label of a timeline: br as a new line, and
// the entities read.
func tlText(s string) string {
	return labelText(tlUnmask(s))
}
